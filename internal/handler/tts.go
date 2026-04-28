package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/speech"
)

const (
	// ttsMaxBodyBytes limits the request body size for TTS endpoint (1MB).
	ttsMaxBodyBytes = 1 << 20

	// ttsSummarizeTimeout is the timeout for the summarization step.
	ttsSummarizeTimeout = 15 * time.Second

	// ttsSynthesizeTimeout is the timeout for the TTS synthesis step.
	ttsSynthesizeTimeout = 30 * time.Second
)

// speechProvider is the global speech provider instance.
var speechProvider speech.SpeechProvider = speech.NewMiniMaxProvider()

// SetSpeechProvider replaces the global speech provider.
// Must be called before the HTTP server starts; not goroutine-safe.
func SetSpeechProvider(p speech.SpeechProvider) {
	speechProvider = p
}

// ttsGenerateRequest is the request body for POST /api/tts/generate.
type ttsGenerateRequest struct {
	Text string `json:"text"`
}

// ttsGenerateResponse is the response body for POST /api/tts/generate.
type ttsGenerateResponse struct {
	AudioPath        string `json:"audioPath"`
	Summary          string `json:"summary,omitempty"`
	SummarizeFailed  bool   `json:"summarizeFailed,omitempty"`
}

// TTSGenerate handles POST /api/tts/generate.
// It summarizes the input text, synthesizes speech, and returns the audio file path.
func TTSGenerate(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, int64(ttsMaxBodyBytes))

	var req ttsGenerateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Text == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "text is required")
		return
	}

	if speech.MaxTextRunes > 0 && len([]rune(req.Text)) > speech.MaxTextRunes {
		model.WriteErrorf(w, http.StatusBadRequest, fmt.Sprintf("文本过长，最多支持%d字符", speech.MaxTextRunes))
		return
	}

	// Compute cache key from text content
	hash := sha256.Sum256([]byte(req.Text))
	cacheKey := hex.EncodeToString(hash[:])[:speech.CacheKeyHexLen]
	relAudioPath := filepath.Join(".clawbench", "generated", "tts", cacheKey+".mp3")

	// Validate the output path (defense-in-depth)
	absAudioPath, ok := validateAndResolvePath(w, projectPath, relAudioPath)
	if !ok {
		return
	}

	// Check cache: if audio file already exists, return immediately
	// Also check for a cached summary file alongside the audio
	if info, err := os.Stat(absAudioPath); err == nil && info.Size() > 0 {
		slog.Info("tts cache hit",
			slog.String("cache_key", cacheKey),
			slog.String("path", relAudioPath),
		)
		// Try to read cached summary
		summaryPath := absAudioPath + ".summary.txt"
		cachedSummary, _ := os.ReadFile(summaryPath)
		writeJSON(w, http.StatusOK, ttsGenerateResponse{
			AudioPath: relAudioPath,
			Summary:   string(cachedSummary),
		})
		return
	}

	// Step 1: Summarize the text for voice output (handler controls deadline)
	summarizeCtx, summarizeCancel := context.WithTimeout(r.Context(), ttsSummarizeTimeout)
	defer summarizeCancel()

	summary, err := speechProvider.Summarize(summarizeCtx, req.Text)
	summarizeFailed := false
	if err != nil {
		// Log warning but fall back to original text
		slog.Warn("tts summarize failed, using original text",
			slog.String("error", err.Error()),
		)
		summary = req.Text
		summarizeFailed = true
	}

	// Strip any markdown from the summary before synthesis and display
	summary = speech.StripMarkdown(summary)

	slog.Info("tts summarize completed",
		slog.String("cache_key", cacheKey),
		slog.Int("original_len", len([]rune(req.Text))),
		slog.Int("summary_len", len([]rune(summary))),
	)

	// Cache the summary alongside the audio for future cache hits
	summaryPath := absAudioPath + ".summary.txt"
	if writeErr := os.WriteFile(summaryPath, []byte(summary), 0644); writeErr != nil {
		slog.Warn("tts failed to cache summary",
			slog.String("error", writeErr.Error()),
		)
	}

	// Step 2: Synthesize speech from the summary (handler controls deadline)
	synthesizeCtx, synthesizeCancel := context.WithTimeout(r.Context(), ttsSynthesizeTimeout)
	defer synthesizeCancel()

	if err := speechProvider.Synthesize(synthesizeCtx, summary, absAudioPath); err != nil {
		slog.Error("tts synthesize failed",
			slog.String("error", err.Error()),
			slog.String("cache_key", cacheKey),
		)
		// Don't leak internal error details to the client
		model.WriteErrorf(w, http.StatusInternalServerError, "语音生成失败，请稍后重试")
		return
	}

	slog.Info("tts generate completed",
		slog.String("cache_key", cacheKey),
		slog.String("path", relAudioPath),
	)

	writeJSON(w, http.StatusOK, ttsGenerateResponse{
		AudioPath: relAudioPath,
		Summary:   summary,
		SummarizeFailed: summarizeFailed,
	})
}

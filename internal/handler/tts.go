package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/service"
	"clawbench/internal/speech"
)

const (
	// ttsMaxBodyBytes limits the request body size for TTS endpoint (1MB).
	ttsMaxBodyBytes = 1 << 20

	// ttsSummarizeTimeout is the timeout for the summarization step.
	ttsSummarizeTimeout = 60 * time.Second

	// ttsSynthesizeTimeout is the timeout for the TTS synthesis step.
	ttsSynthesizeTimeout = 120 * time.Second
)

// speechProvider is the global speech provider instance.
var speechProvider speech.SpeechProvider = speech.NewEdgeTTSProvider()

// SetSpeechProvider replaces the global speech provider.
// Must be called before the HTTP server starts; not goroutine-safe.
func SetSpeechProvider(p speech.SpeechProvider) {
	speechProvider = p
}

// summarizer is the global text summarizer instance.
var summarizer speech.Summarizer = speech.NewSimpleSummarizer()

// SetSummarizer replaces the global text summarizer.
// Must be called before the HTTP server starts; not goroutine-safe.
func SetSummarizer(s speech.Summarizer) {
	summarizer = s
}

// ttsGenerateRequest is the request body for POST /api/tts/generate.
type ttsGenerateRequest struct {
	Text     string `json:"text"`
	Language string `json:"language"` // language code, e.g. "zh", "en"; defaults to "zh" if empty
}

// ttsSSEEvent is an SSE event sent during TTS generation.
type ttsSSEEvent struct {
	Type             string `json:"type"`                       // "phase" or "result"
	Phase            string `json:"phase,omitempty"`             // "summarizing", "synthesizing"
	AudioPath        string `json:"audioPath,omitempty"`
	Summary          string `json:"summary,omitempty"`
	SummarizeFailed  bool   `json:"summarizeFailed,omitempty"`
	SynthesizeFailed bool   `json:"synthesizeFailed,omitempty"`
	SynthesizeError  string `json:"synthesizeError,omitempty"`
}

// ttsWriteSSE writes a single SSE event and flushes.
func ttsWriteSSE(w http.ResponseWriter, event ttsSSEEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// TTSGenerate handles POST /api/tts/generate.
// It streams SSE events to report progress: summarizing → synthesizing → result.
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

	// Default language to "zh" if not provided
	if req.Language == "" {
		req.Language = "zh"
	}

	if speech.MaxTextRunes > 0 && len([]rune(req.Text)) > speech.MaxTextRunes {
		model.WriteErrorf(w, http.StatusBadRequest, fmt.Sprintf("文本过长，最多支持%d字符", speech.MaxTextRunes))
		return
	}

	// Compute cache key from text content
	hash := sha256.Sum256([]byte(req.Text))
	cacheKey := hex.EncodeToString(hash[:])[:speech.CacheKeyHexLen]

	// Determine audio file extension based on TTS engine
	audioExt := ".mp3"
	if _, ok := speechProvider.(*speech.PiperProvider); ok {
		audioExt = ".wav"
	}
	if _, ok := speechProvider.(*speech.KokoroProvider); ok {
		audioExt = ".wav"
	}
	if _, ok := speechProvider.(*speech.MossNanoProvider); ok {
		audioExt = ".wav"
	}
	relAudioPath := filepath.Join(".clawbench", "generated", "tts", cacheKey+audioExt)

	// Validate the output path (defense-in-depth)
	absAudioPath, ok := validateAndResolvePath(w, projectPath, relAudioPath)
	if !ok {
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Check cache: if audio file already exists, return immediately
	if info, err := os.Stat(absAudioPath); err == nil && info.Size() > 0 {
		slog.Info("tts cache hit",
			slog.String("cache_key", cacheKey),
			slog.String("path", relAudioPath),
		)
		// Always send both phase events for consistent UX, even on cache hits.
		ttsWriteSSE(w, ttsSSEEvent{Type: "phase", Phase: "summarizing"})
		// Small delay so the frontend can render "摘要中" before we send "合成中".
		// On cache hits all events fire instantly; without a gap the user never
		// sees the intermediate labels.
		time.Sleep(100 * time.Millisecond)
		ttsWriteSSE(w, ttsSSEEvent{Type: "phase", Phase: "synthesizing"})
		time.Sleep(100 * time.Millisecond)
		// Try DB first, fall back to file cache
		summary, _, found := service.GetTTSSummary(cacheKey)
		if !found {
			cachedSummary, _ := os.ReadFile(absAudioPath + ".summary.txt")
			summary = string(cachedSummary)
		}
		// Don't forward summarizeFailed on cache hits — the audio exists,
		// so the user doesn't need to know that summarization failed on a
		// previous attempt.  Showing "摘要失败" every time is misleading.
		ttsWriteSSE(w, ttsSSEEvent{
			Type:      "result",
			AudioPath: relAudioPath,
			Summary:   summary,
		})
		return
	}

	// Check DB for cached summary (previous summarize succeeded but synthesize failed)
	var summary string
	var summarizeFailed bool
	cachedSummary, cachedFailed, found := service.GetTTSSummary(cacheKey)
	if found && cachedSummary != "" {
		slog.Info("tts summary cache hit, skipping summarization",
			slog.String("cache_key", cacheKey),
		)
		// Still send "summarizing" phase for consistent UX even though we skip the actual work.
		ttsWriteSSE(w, ttsSSEEvent{Type: "phase", Phase: "summarizing"})
		time.Sleep(100 * time.Millisecond)
		summary = cachedSummary
		summarizeFailed = cachedFailed
	} else {
		// Always send "summarizing" phase for consistent UX.
		ttsWriteSSE(w, ttsSSEEvent{Type: "phase", Phase: "summarizing"})

		summarizeCtx, summarizeCancel := context.WithTimeout(r.Context(), ttsSummarizeTimeout)
		defer summarizeCancel()

		var err error
		summary, err = summarizer.Summarize(summarizeCtx, req.Text, req.Language)
		if err != nil {
			slog.Warn("tts summarize failed, using stripped original text",
				slog.String("error", err.Error()),
			)
			summary = speech.StripMarkdown(req.Text)
			summarizeFailed = true
		}

		slog.Info("tts summarize completed",
			slog.String("cache_key", cacheKey),
			slog.Int("original_len", len([]rune(req.Text))),
			slog.Int("summary_len", len([]rune(summary))),
		)

		// Save summary to database (independent of audio generation result)
		if err := service.SaveTTSSummary(cacheKey, summary, summarizeFailed); err != nil {
			slog.Warn("tts failed to cache summary to DB",
				slog.String("error", err.Error()),
			)
		}
	}

	// Phase 2: Synthesize
	ttsWriteSSE(w, ttsSSEEvent{Type: "phase", Phase: "synthesizing"})

	synthesizeCtx, synthesizeCancel := context.WithTimeout(r.Context(), ttsSynthesizeTimeout)
	defer synthesizeCancel()

	if err := speechProvider.Synthesize(synthesizeCtx, summary, absAudioPath, req.Language); err != nil {
		slog.Error("tts synthesize failed",
			slog.String("error", err.Error()),
			slog.String("cache_key", cacheKey),
		)
		ttsWriteSSE(w, ttsSSEEvent{
			Type:             "result",
			SynthesizeFailed: true,
			SynthesizeError:  "语音合成失败，请稍后重试",
			Summary:          summary,
			SummarizeFailed:  summarizeFailed,
		})
		return
	}

	slog.Info("tts generate completed",
		slog.String("cache_key", cacheKey),
		slog.String("path", relAudioPath),
	)

	ttsWriteSSE(w, ttsSSEEvent{
		Type:            "result",
		AudioPath:       relAudioPath,
		Summary:         summary,
		SummarizeFailed: summarizeFailed,
	})
}

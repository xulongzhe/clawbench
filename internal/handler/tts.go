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
	"strings"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/service"
	"clawbench/internal/speech"
	"clawbench/internal/summarize"
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
var summarizer summarize.Summarizer = summarize.NewSimple()

// SetSummarizer replaces the global text summarizer.
// Must be called before the HTTP server starts; not goroutine-safe.
func SetSummarizer(s summarize.Summarizer) {
	summarizer = s
}

// ttsGenerateRequest is the request body for POST /api/tts/generate.
type ttsGenerateRequest struct {
	Text      string `json:"text"`
	Language  string `json:"language"`  // language code, e.g. "zh", "en"; defaults to "zh" if empty
	MessageID int64  `json:"messageId"`  // chat_history.id for TTS summary caching
}

// TTSGenerate handles POST /api/tts/generate.
// It validates input, checks cache, and either returns cached audio immediately
// or starts an async TTS job and returns a jobId for SSE streaming.
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
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "TextRequired")
		return
	}

	// Default language to "zh" if not provided
	if req.Language == "" {
		req.Language = "zh"
	}

	if summarize.MaxTextRunes > 0 && len([]rune(req.Text)) > summarize.MaxTextRunes {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "TextTooLong", map[string]any{"MaxChars": summarize.MaxTextRunes})
		return
	}

	// Compute cache key from text content
	hash := sha256.Sum256([]byte(req.Text))
	cacheKey := hex.EncodeToString(hash[:])[:summarize.CacheKeyHexLen]

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
	absAudioPath, ok := validateAndResolvePath(w, r, projectPath, relAudioPath)
	if !ok {
		return
	}

	// Check cache: if audio file already exists, return immediately as JSON
	if info, err := os.Stat(absAudioPath); err == nil && info.Size() > 0 {
		slog.Info("tts cache hit",
			slog.String("cache_key", cacheKey),
			slog.String("path", relAudioPath),
		)
		// Try DB for cached summary
		var summary string
		if req.MessageID > 0 {
			summary, _ = service.GetTTSSummaryByMessageID(req.MessageID)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"cached":    true,
			"audioPath": relAudioPath,
			"summary":   summary,
		})
		return
	}

	// Cache miss — start async TTS job
	ctx, cancel := context.WithCancel(context.Background())
	service.RegisterTTSJob(cacheKey, cancel)

	// Start background goroutine to perform summarize + synthesize
	go func() {
		defer service.UnregisterTTSJob(cacheKey)
		defer service.CloseTTSJobDone(cacheKey)
		defer cancel()

		// Phase 1: Summarize
		var summary string
		cachedSummary, found := service.GetTTSSummaryByMessageID(req.MessageID)
		if found && cachedSummary != "" && req.MessageID > 0 {
			slog.Info("tts summary cache hit, skipping summarization",
				slog.String("cache_key", cacheKey),
			)
			summary = cachedSummary
		} else {
			service.SendTTSEvent(cacheKey, service.TTSEvent{Type: "phase", Phase: "summarizing"})

			summarizeCtx, summarizeCancel := context.WithTimeout(ctx, ttsSummarizeTimeout)
			var err error
			summary, err = summarizer.Summarize(summarizeCtx, req.Text, req.Language)
			summarizeCancel()
			if err != nil {
				slog.Warn("tts summarize failed",
					slog.String("error", err.Error()),
				)
				service.SendTTSEvent(cacheKey, service.TTSEvent{
					Type:             "result",
					SynthesizeFailed: true,
					SynthesizeError:  T(r, "SummarizeFailed"),
				})
				return
			}

			slog.Info("tts summarize completed",
				slog.String("cache_key", cacheKey),
				slog.Int("original_len", len([]rune(req.Text))),
				slog.Int("summary_len", len([]rune(summary))),
			)

			// Save summary to database
			if req.MessageID > 0 {
				if err := service.SaveTTSSummaryByMessageID(req.MessageID, summary); err != nil {
					slog.Warn("tts failed to cache summary to DB",
						slog.String("error", err.Error()),
					)
				}
			}
		}

		// Phase 2: Synthesize
		service.SendTTSEvent(cacheKey, service.TTSEvent{Type: "phase", Phase: "synthesizing"})

		synthesizeCtx, synthesizeCancel := context.WithTimeout(ctx, ttsSynthesizeTimeout)
		err := speechProvider.Synthesize(synthesizeCtx, summary, absAudioPath, req.Language)
		synthesizeCancel()
		if err != nil {
			slog.Error("tts synthesize failed",
				slog.String("error", err.Error()),
				slog.String("cache_key", cacheKey),
			)
		service.SendTTSEvent(cacheKey, service.TTSEvent{
			Type:             "result",
			SynthesizeFailed: true,
			SynthesizeError:  T(r, "SynthesizeFailed"),
			Summary:          summary,
		})
			return
		}

		slog.Info("tts generate completed",
			slog.String("cache_key", cacheKey),
			slog.String("path", relAudioPath),
		)

		// Evict oldest cached files if over the limit
		service.EvictTTSCache(projectPath, model.TTSMaxCacheFiles)

		service.SendTTSEvent(cacheKey, service.TTSEvent{
			Type:      "result",
			AudioPath: relAudioPath,
			Summary:   summary,
		})
	}()

	// Return jobId so the frontend can connect via EventSource
	writeJSON(w, http.StatusOK, map[string]any{
		"jobId": cacheKey,
	})
}

// TTSStream handles GET /api/tts/stream/{jobId}.
// It streams SSE events for a TTS job using typed event format,
// compatible with browser EventSource API.
func TTSStream(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	_, ok := requireProject(w, r)
	if !ok {
		return
	}

	// Extract jobId from URL path: /api/tts/stream/{jobId}
	jobID := strings.TrimPrefix(r.URL.Path, "/api/tts/stream/")
	if jobID == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "JobIdRequired")
		return
	}

	job, ok := service.GetTTSJob(jobID)
	if !ok {
		writeLocalizedErrorf(w, r, http.StatusNotFound, "JobNotFound")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, canFlush := w.(http.Flusher)

	// Send cached events that may have already been produced before we connected.
	// Read from channel until it's empty, then enter the live streaming loop.
	for {
		select {
		case event, ok := <-job.StreamCh:
			if !ok {
				// Channel closed — job finished
				return
			}
			writeTTSSSE(w, event, canFlush, flusher)
			// If this is a result event, we're done
			if event.Type == "result" {
				return
			}
		default:
			// No cached events — enter live streaming
			goto liveStream
		}
	}

liveStream:
	for {
		select {
		case event, ok := <-job.StreamCh:
			if !ok {
				// Channel closed — job finished
				return
			}
			writeTTSSSE(w, event, canFlush, flusher)
			if event.Type == "result" {
				return
			}
		case <-r.Context().Done():
			slog.Info("tts sse client disconnected, cancelling job",
				slog.String("job_id", jobID),
			)
			service.CancelTTSJob(jobID)
			return
		}
	}
}

// writeTTSSSE writes a single TTS event as a typed SSE message and flushes.
func writeTTSSSE(w http.ResponseWriter, event service.TTSEvent, canFlush bool, flusher http.Flusher) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
	if canFlush {
		flusher.Flush()
	}
}

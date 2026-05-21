package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawbench/internal/service"
	"clawbench/internal/speech"
	"clawbench/internal/summarize"

	"github.com/stretchr/testify/assert"
)

// mockSummarizer is a test double for summarize.Summarizer.
type mockSummarizer struct {
	result       string
	err          error
	called       bool
	lastText     string
	lastLanguage string
}

func (m *mockSummarizer) Summarize(ctx context.Context, text string, language string) (string, error) {
	m.called = true
	m.lastText = text
	m.lastLanguage = language
	if m.err != nil {
		return "", m.err
	}
	if m.result != "" {
		return m.result, nil
	}
	return "summary of: " + text, nil
}

// mockSpeechProvider is a test double for speech.SpeechProvider.
type mockSpeechProvider struct {
	synthesizeErr    error
	synthesizeCalled bool
	lastSynthText    string
	lastSynthLang    string
	// synthesizeBlock, if non-nil, is closed before Synthesize returns.
	// Set before starting the job to keep the goroutine alive until the
	// stream endpoint has connected (avoids race where the goroutine
	// finishes and unregisters the job before TTSStream can look it up).
	synthesizeBlock chan struct{}
}

func (m *mockSpeechProvider) Synthesize(ctx context.Context, text string, outputPath string, language string) error {
	m.synthesizeCalled = true
	m.lastSynthText = text
	m.lastSynthLang = language
	if m.synthesizeErr != nil {
		return m.synthesizeErr
	}
	// Create a dummy audio file at outputPath
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, []byte("fake audio data"), 0644); err != nil {
		return err
	}
	// Block until the test releases us (if configured)
	if m.synthesizeBlock != nil {
		select {
		case <-m.synthesizeBlock:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// setupTTSTest sets up a test environment with mock provider and summarizer.
// Returns the test env and a teardown function.
func setupTTSTest(t *testing.T, mockProvider *mockSpeechProvider, mockSum *mockSummarizer) (*testEnv, func()) {
	t.Helper()

	// Use the shared test environment (inits DB, project dir, etc.)
	env, envTeardown := setupTestEnv(t)

	// Save and replace the global speech provider and summarizer
	origProvider := speechProvider
	origSummarizer := summarizer
	speechProvider = mockProvider
	summarizer = mockSum

	teardown := func() {
		speechProvider = origProvider
		summarizer = origSummarizer
		envTeardown()
	}

	return env, teardown
}

// --- TTSGenerate: method validation ---

func TestTTSGenerate_MethodNotAllowed(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{}
	_, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/tts/generate", nil)
	req = withProjectCookie(req, t.TempDir())
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.False(t, mockSum.called)
}

// --- TTSGenerate: empty text ---

func TestTTSGenerate_EmptyText(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": ""})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, mockSum.called)
}

// --- TTSGenerate: text too long ---

func TestTTSGenerate_TextTooLong(t *testing.T) {
	// Temporarily set MaxTextRunes to enforce a limit
	origMaxTextRunes := summarize.MaxTextRunes
	summarize.MaxTextRunes = 100
	defer func() { summarize.MaxTextRunes = origMaxTextRunes }()

	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	longText := strings.Repeat("x", 101)
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": longText})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, mockSum.called)
}

// --- TTSGenerate: successful generation returns jobId ---

func TestTTSGenerate_Success(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{result: "这是核心结论"}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	text := "这是一段较长的AI回复内容，需要被总结为语音。包含了详细的分析和代码示例，需要提取核心要点。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Response should be JSON with jobId
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp, "jobId")
	assert.NotEmpty(t, resp["jobId"])

	// Wait for the background goroutine to complete
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:summarize.CacheKeyHexLen]
	job, ok := service.GetTTSJob(cacheKey)
	if ok {
		select {
		case <-job.Done:
		case <-time.After(5 * time.Second):
			t.Fatal("TTS job did not complete in time")
		}
	}

	// Verify the mock was called
	assert.True(t, mockSum.called)
	assert.True(t, mockProvider.synthesizeCalled)
}

// --- TTSGenerate: summarize failure returns error, does not synthesize ---

func TestTTSGenerate_SummarizeFailure(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{err: context.DeadlineExceeded}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	text := "这是一段需要总结的长文本内容，但由于摘要失败会直接报错。内容足够长以触发摘要流程。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for the background goroutine to complete
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:summarize.CacheKeyHexLen]
	job, ok := service.GetTTSJob(cacheKey)
	if ok {
		select {
		case <-job.Done:
		case <-time.After(5 * time.Second):
			t.Fatal("TTS job did not complete in time")
		}
	}

	// Summarizer was called, but synthesize should NOT be called
	assert.True(t, mockSum.called)
	assert.False(t, mockProvider.synthesizeCalled)
}

// --- TTSGenerate: synthesize failure returns error via SSE stream ---

func TestTTSGenerate_SynthesizeFailure(t *testing.T) {
	mockProvider := &mockSpeechProvider{synthesizeErr: context.DeadlineExceeded}
	mockSum := &mockSummarizer{result: "总结文本"}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	text := "测试语音合成失败的场景。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp, "jobId")

	// Wait for job to complete
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:summarize.CacheKeyHexLen]
	job, ok := service.GetTTSJob(cacheKey)
	if ok {
		select {
		case <-job.Done:
		case <-time.After(5 * time.Second):
			t.Fatal("TTS job did not complete in time")
		}
	}

	assert.True(t, mockSum.called)
	assert.True(t, mockProvider.synthesizeCalled)
}

// --- TTSGenerate: cache hit returns JSON directly ---

func TestTTSGenerate_CacheHit(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	text := "这段文本会被缓存。"
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:summarize.CacheKeyHexLen]
	relAudioPath := filepath.Join(".clawbench", "generated", "tts", cacheKey+".mp3")
	absAudioPath := filepath.Join(env.ProjectDir, relAudioPath)

	// Pre-create the cached file
	if err := os.MkdirAll(filepath.Dir(absAudioPath), 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}
	if err := os.WriteFile(absAudioPath, []byte("cached audio"), 0644); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Cache hit returns JSON directly (not SSE)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp["cached"])
	assert.Equal(t, relAudioPath, resp["audioPath"])

	// Provider should NOT be called on cache hit
	assert.False(t, mockSum.called)
	assert.False(t, mockProvider.synthesizeCalled)
}

// --- TTSGenerate: invalid JSON body ---

func TestTTSGenerate_InvalidJSON(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/tts/generate", strings.NewReader(`{invalid json}`))
	req.Header.Set("Content-Type", "application/json")
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, mockSum.called)
}

// --- TTSGenerate: cache key is deterministic ---

func TestTTSGenerate_CacheKeyDeterministic(t *testing.T) {
	text := "同样的文本应该产生同样的缓存键"
	hash1 := sha256.Sum256([]byte(text))
	key1 := hex.EncodeToString(hash1[:])[:summarize.CacheKeyHexLen]

	hash2 := sha256.Sum256([]byte(text))
	key2 := hex.EncodeToString(hash2[:])[:summarize.CacheKeyHexLen]

	assert.Equal(t, key1, key2)
	assert.Len(t, key1, summarize.CacheKeyHexLen)
}

// --- SetSpeechProvider and SetSummarizer ---

func TestSetSpeechProvider(t *testing.T) {
	origProvider := speechProvider
	defer func() { speechProvider = origProvider }()

	mock := &mockSpeechProvider{}
	SetSpeechProvider(mock)

	// Verify the global was replaced
	assert.Equal(t, speechProvider, mock)
}

func TestSetSummarizer(t *testing.T) {
	origSum := summarizer
	defer func() { summarizer = origSum }()

	mock := &mockSummarizer{}
	SetSummarizer(mock)

	// Verify the global was replaced
	assert.Equal(t, summarizer, mock)
}

// --- ensure mockSummarizer satisfies Summarizer interface ---
var _ summarize.Summarizer = (*mockSummarizer)(nil)

// --- ensure mockSpeechProvider satisfies SpeechProvider interface ---
var _ speech.SpeechProvider = (*mockSpeechProvider)(nil)

// --- TTSGenerate: language propagation ---

func TestTTSGenerate_LanguageDefaultToZh(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{result: "核心结论"}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	// Send request WITHOUT language field
	text := "这是一段较长的AI回复内容，需要被总结为语音。包含了详细的分析和代码示例，需要提取核心要点。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for job to complete
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:summarize.CacheKeyHexLen]
	if job, ok := service.GetTTSJob(cacheKey); ok {
		select {
		case <-job.Done:
		case <-time.After(5 * time.Second):
			t.Fatal("TTS job did not complete in time")
		}
	}

	// Language should default to "zh"
	assert.Equal(t, "zh", mockSum.lastLanguage)
	assert.Equal(t, "zh", mockProvider.lastSynthLang)
}

func TestTTSGenerate_LanguageEn(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{result: "Core conclusion"}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	// Send request WITH language="en"
	text := "This is a long AI response that contains detailed analysis and code examples, requiring core point extraction."
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text, "language": "en"})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for job to complete
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:summarize.CacheKeyHexLen]
	if job, ok := service.GetTTSJob(cacheKey); ok {
		select {
		case <-job.Done:
		case <-time.After(5 * time.Second):
			t.Fatal("TTS job did not complete in time")
		}
	}

	assert.Equal(t, "en", mockSum.lastLanguage)
	assert.Equal(t, "en", mockProvider.lastSynthLang)
}

func TestTTSGenerate_LanguageJa(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{result: "核心の結論"}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	text := "これは長いAIの応答内容であり、音声に要約する必要があります。詳細な分析とコード例が含まれています。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text, "language": "ja"})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for job to complete
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:summarize.CacheKeyHexLen]
	if job, ok := service.GetTTSJob(cacheKey); ok {
		select {
		case <-job.Done:
		case <-time.After(5 * time.Second):
			t.Fatal("TTS job did not complete in time")
		}
	}

	assert.Equal(t, "ja", mockSum.lastLanguage)
	assert.Equal(t, "ja", mockProvider.lastSynthLang)
}

// --- TTSStream: SSE streaming via EventSource-compatible format ---

func TestTTSStream_Success(t *testing.T) {
	block := make(chan struct{})
	mockProvider := &mockSpeechProvider{synthesizeBlock: block}
	mockSum := &mockSummarizer{result: "这是核心结论"}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	text := "这是用于测试SSE流的文本内容，包含足够的文字以触发摘要流程。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var genResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &genResp)
	jobID := genResp["jobId"].(string)

	// Wait for the summarizing phase to be sent (ensures the goroutine is running)
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:summarize.CacheKeyHexLen]
	job, ok := service.GetTTSJob(cacheKey)
	if !ok {
		t.Fatal("TTS job not found after TTSGenerate returned")
	}

	// Connect to stream endpoint while the job is still alive.
	// The mock synthesize blocks on `block`, so the goroutine won't finish
	// and unregister the job before we connect.
	streamReq := httptest.NewRequest(http.MethodGet, "/api/tts/stream/"+jobID, nil)
	streamReq = withProjectCookie(streamReq, env.ProjectDir)
	streamW := httptest.NewRecorder()

	// Release the mock so the job can finish after we've connected.
	close(block)

	TTSStream(streamW, streamReq)

	// Wait for the job to complete so events are fully flushed.
	select {
	case <-job.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("TTS job did not complete in time")
	}

	body := streamW.Body.String()
	assert.Contains(t, body, "event: phase")
	assert.Contains(t, body, "summarizing")
	assert.Contains(t, body, "synthesizing")
	assert.Contains(t, body, "event: result")
}

func TestTTSStream_JobNotFound(t *testing.T) {
	_, teardown := setupTTSTest(t, &mockSpeechProvider{}, &mockSummarizer{})
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/tts/stream/nonexistent", nil)
	req = withProjectCookie(req, t.TempDir())
	w := httptest.NewRecorder()

	TTSStream(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTTSStream_MissingJobID(t *testing.T) {
	_, teardown := setupTTSTest(t, &mockSpeechProvider{}, &mockSummarizer{})
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/tts/stream/", nil)
	req = withProjectCookie(req, t.TempDir())
	w := httptest.NewRecorder()

	TTSStream(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

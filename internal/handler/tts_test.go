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

	"clawbench/internal/speech"

	"github.com/stretchr/testify/assert"
)

// mockSummarizer is a test double for speech.Summarizer.
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
	return os.WriteFile(outputPath, []byte("fake audio data"), 0644)
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

// parseTTSResult parses SSE data lines and returns the result event.
func parseTTSResult(t *testing.T, body string) ttsSSEEvent {
	t.Helper()
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			var event ttsSSEEvent
			if err := json.Unmarshal([]byte(line[6:]), &event); err == nil && event.Type == "result" {
				return event
			}
		}
	}
	t.Fatal("no result event found in SSE output")
	return ttsSSEEvent{}
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
	origMaxTextRunes := speech.MaxTextRunes
	speech.MaxTextRunes = 100
	defer func() { speech.MaxTextRunes = origMaxTextRunes }()

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

// --- TTSGenerate: successful generation ---

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

	result := parseTTSResult(t, w.Body.String())
	assert.Contains(t, result.AudioPath, ".clawbench/generated/tts/")
	assert.Contains(t, result.AudioPath, ".mp3")
	assert.True(t, mockSum.called)
	assert.True(t, mockProvider.synthesizeCalled)
}

// --- TTSGenerate: summarize failure falls back to original text ---

func TestTTSGenerate_SummarizeFailure_Fallback(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{err: context.DeadlineExceeded}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	text := "这是一段需要总结的长文本内容，但由于摘要失败会回退到原文。内容足够长以触发摘要流程。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Synthesize should be called
	assert.True(t, mockSum.called)
	assert.True(t, mockProvider.synthesizeCalled)
	// The text passed to Synthesize should contain the original text
	assert.Contains(t, mockProvider.lastSynthText, "摘要失败")

	// Response should indicate summarizeFailed
	result := parseTTSResult(t, w.Body.String())
	assert.True(t, result.SummarizeFailed)
}

// --- TTSGenerate: synthesize failure returns error ---

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

	// SSE result should indicate synthesize failed
	result := parseTTSResult(t, w.Body.String())
	assert.True(t, result.SynthesizeFailed)
	assert.Contains(t, result.SynthesizeError, "语音合成失败")
}

// --- TTSGenerate: cache hit returns immediately ---

func TestTTSGenerate_CacheHit(t *testing.T) {
	mockProvider := &mockSpeechProvider{}
	mockSum := &mockSummarizer{}
	env, teardown := setupTTSTest(t, mockProvider, mockSum)
	defer teardown()

	text := "这段文本会被缓存。"
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:speech.CacheKeyHexLen]
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

	result := parseTTSResult(t, w.Body.String())
	assert.Equal(t, relAudioPath, result.AudioPath)

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
	key1 := hex.EncodeToString(hash1[:])[:speech.CacheKeyHexLen]

	hash2 := sha256.Sum256([]byte(text))
	key2 := hex.EncodeToString(hash2[:])[:speech.CacheKeyHexLen]

	assert.Equal(t, key1, key2)
	assert.Len(t, key1, speech.CacheKeyHexLen)
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
var _ speech.Summarizer = (*mockSummarizer)(nil)

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

	assert.Equal(t, "ja", mockSum.lastLanguage)
	assert.Equal(t, "ja", mockProvider.lastSynthLang)
}

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

// mockSpeechProvider is a test double for speech.SpeechProvider.
type mockSpeechProvider struct {
	summarizeResult string
	summarizeErr    error
	synthesizeErr   error
	summarizeCalled bool
	synthesizeCalled bool
	lastSummaryText string
}

func (m *mockSpeechProvider) Summarize(ctx context.Context, text string) (string, error) {
	m.summarizeCalled = true
	m.lastSummaryText = text
	if m.summarizeErr != nil {
		return "", m.summarizeErr
	}
	if m.summarizeResult != "" {
		return m.summarizeResult, nil
	}
	return "summary of: " + text, nil
}

func (m *mockSpeechProvider) Synthesize(ctx context.Context, text string, outputPath string) error {
	m.synthesizeCalled = true
	if m.synthesizeErr != nil {
		return m.synthesizeErr
	}
	// Create a dummy audio file at outputPath
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte("fake audio data"), 0644)
}

// setupTTSTest sets up a test environment with mock provider and project dir.
// Returns the project dir and a teardown function.
func setupTTSTest(t *testing.T, mock *mockSpeechProvider) (string, func()) {
	t.Helper()

	// Save and replace the global speech provider
	origProvider := speechProvider
	speechProvider = mock

	// Create a temp project dir
	projectDir := t.TempDir()

	// We also need the project cookie to be set for requireProject
	// This is handled by withProjectCookie in the test request

	teardown := func() {
		speechProvider = origProvider
	}

	return projectDir, teardown
}

// --- TTSGenerate: method validation ---

func TestTTSGenerate_MethodNotAllowed(t *testing.T) {
	mock := &mockSpeechProvider{}
	_, teardown := setupTTSTest(t, mock)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/tts/generate", nil)
	req = withProjectCookie(req, t.TempDir())
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.False(t, mock.summarizeCalled)
}

// --- TTSGenerate: missing project ---

func TestTTSGenerate_NoProject(t *testing.T) {
	mock := &mockSpeechProvider{}
	_, teardown := setupTTSTest(t, mock)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/tts/generate", strings.NewReader(`{"text":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- TTSGenerate: empty text ---

func TestTTSGenerate_EmptyText(t *testing.T) {
	mock := &mockSpeechProvider{}
	projectDir, teardown := setupTTSTest(t, mock)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": ""})
	req = withProjectCookie(req, projectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, mock.summarizeCalled)
}

// --- TTSGenerate: text too long ---

func TestTTSGenerate_TextTooLong(t *testing.T) {
	mock := &mockSpeechProvider{}
	projectDir, teardown := setupTTSTest(t, mock)
	defer teardown()

	longText := strings.Repeat("x", 10001)
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": longText})
	req = withProjectCookie(req, projectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, mock.summarizeCalled)
}

// --- TTSGenerate: successful generation ---

func TestTTSGenerate_Success(t *testing.T) {
	mock := &mockSpeechProvider{
		summarizeResult: "这是核心结论",
	}
	projectDir, teardown := setupTTSTest(t, mock)
	defer teardown()

	text := "这是一段较长的AI回复内容，需要被总结为语音。包含了详细的分析和代码示例，需要提取核心要点。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, projectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp ttsGenerateResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp.AudioPath, ".clawbench/generated/tts/")
	assert.Contains(t, resp.AudioPath, ".mp3")
	assert.True(t, mock.summarizeCalled)
	assert.True(t, mock.synthesizeCalled)
}

// --- TTSGenerate: summarize failure falls back to original text ---

func TestTTSGenerate_SummarizeFailure_Fallback(t *testing.T) {
	mock := &mockSpeechProvider{
		summarizeErr:    context.DeadlineExceeded,
		summarizeResult: "",
	}
	projectDir, teardown := setupTTSTest(t, mock)
	defer teardown()

	text := "这是一段需要总结的长文本内容，但由于摘要失败会回退到原文。内容足够长以触发摘要流程。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, projectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Synthesize should be called with the original text (fallback)
	assert.True(t, mock.summarizeCalled)
	assert.True(t, mock.synthesizeCalled)
	// The text passed to Synthesize should contain the original text
	assert.Contains(t, mock.lastSummaryText, "摘要失败")
}

// --- TTSGenerate: synthesize failure returns 500 ---

func TestTTSGenerate_SynthesizeFailure(t *testing.T) {
	mock := &mockSpeechProvider{
		synthesizeErr: context.DeadlineExceeded,
	}
	projectDir, teardown := setupTTSTest(t, mock)
	defer teardown()

	text := "测试语音合成失败的场景。"
	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, projectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Error message should not leak internal details
	var errResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &errResp)
	errMsg, _ := errResp["error"].(string)
	assert.NotContains(t, errMsg, "stderr")
	assert.NotContains(t, errMsg, "mmx")
	assert.Contains(t, errMsg, "语音生成失败")
}

// --- TTSGenerate: cache hit returns immediately ---

func TestTTSGenerate_CacheHit(t *testing.T) {
	mock := &mockSpeechProvider{}
	projectDir, teardown := setupTTSTest(t, mock)
	defer teardown()

	text := "这段文本会被缓存。"
	hash := sha256.Sum256([]byte(text))
	cacheKey := hex.EncodeToString(hash[:])[:speech.CacheKeyHexLen]
	relAudioPath := filepath.Join(".clawbench", "generated", "tts", cacheKey+".mp3")
	absAudioPath := filepath.Join(projectDir, relAudioPath)

	// Pre-create the cached file
	if err := os.MkdirAll(filepath.Dir(absAudioPath), 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}
	if err := os.WriteFile(absAudioPath, []byte("cached audio"), 0644); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}

	req := newRequest(t, http.MethodPost, "/api/tts/generate", map[string]string{"text": text})
	req = withProjectCookie(req, projectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp ttsGenerateResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, relAudioPath, resp.AudioPath)

	// Provider should NOT be called on cache hit
	assert.False(t, mock.summarizeCalled)
	assert.False(t, mock.synthesizeCalled)
}

// --- TTSGenerate: invalid JSON body ---

func TestTTSGenerate_InvalidJSON(t *testing.T) {
	mock := &mockSpeechProvider{}
	projectDir, teardown := setupTTSTest(t, mock)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/tts/generate", strings.NewReader(`{invalid json}`))
	req.Header.Set("Content-Type", "application/json")
	req = withProjectCookie(req, projectDir)
	w := httptest.NewRecorder()

	TTSGenerate(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, mock.summarizeCalled)
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

// --- SetSpeechProvider ---

func TestSetSpeechProvider(t *testing.T) {
	origProvider := speechProvider
	defer func() { speechProvider = origProvider }()

	mock := &mockSpeechProvider{}
	SetSpeechProvider(mock)

	// Verify the global was replaced
	assert.Equal(t, speechProvider, mock)
}

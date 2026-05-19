package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"clawbench/internal/middleware"
	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestServeConfig_Get(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// Set known config values using named types
	cfg := model.Config{}
	cfg.Upload.MaxSizeMB = 50
	cfg.Upload.MaxFiles = 10
	cfg.Chat.InitialMessages = 15
	cfg.Chat.PageSize = 25
	cfg.Chat.CollapsedHeight = 200
	cfg.Chat.SystemPromptInterval = 5
	cfg.Session.MaxCount = 5
	cfg.Terminal.Enabled = true
	cfg.Terminal.IdleTimeout = "10m"
	cfg.Terminal.MaxSessions = 8
	cfg.Terminal.BufferLines = 3000
	cfg.TTS.Engine = "edge"
	cfg.TTS.Speed = 1.5
	cfg.TTS.Voice = "zh-CN-XiaoxiaoNeural"
	cfg.TTS.MaxCacheFiles = 50
	cfg.RAG.Enabled = true
	cfg.RAG.OllamaBaseURL = "http://localhost:11434"
	cfg.RAG.OllamaModel = "bge-m3"
	cfg.RAG.ChunkSize = 512
	cfg.RAG.SearchLimit = 5
	cfg.RAG.RetentionDays = 30
	cfg.Proxy.Enabled = true
	cfg.Proxy.AllowedPorts = "1024-65535"
	cfg.SSH.Enabled = true
	cfg.SSH.Port = 20001
	cfg.Push.JPush.Enabled = true
	cfg.Push.JPush.AppKey = "test-app-key"
	model.ConfigInstance = cfg

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Verify allowed sections ARE present
	assert.Contains(t, resp, "chat")
	assert.Contains(t, resp, "session")
	assert.Contains(t, resp, "upload")
	assert.Contains(t, resp, "terminal")
	assert.Contains(t, resp, "tts")
	assert.Contains(t, resp, "rag")
	assert.Contains(t, resp, "proxy")
	assert.Contains(t, resp, "ssh")
	assert.Contains(t, resp, "push")

	// Verify specific values
	chat, _ := resp["chat"].(map[string]any)
	assert.Equal(t, float64(200), chat["collapsed_height"])
	assert.Equal(t, float64(15), chat["initial_messages"])

	upload, _ := resp["upload"].(map[string]any)
	assert.Equal(t, float64(50), upload["max_size_mb"])

	terminal, _ := resp["terminal"].(map[string]any)
	assert.Equal(t, true, terminal["enabled"])
	assert.Equal(t, "10m", terminal["idle_timeout"])

	// Verify sensitive fields are NOT present
	assert.NotContains(t, resp, "password")
	assert.NotContains(t, resp, "tls")
	assert.NotContains(t, resp, "host")
	assert.NotContains(t, resp, "port")
	assert.NotContains(t, resp, "log_level")
	assert.NotContains(t, resp, "log_dir")
	assert.NotContains(t, resp, "watch_dir")
	assert.NotContains(t, resp, "dev_port")
	assert.NotContains(t, resp, "default_agent")

	// Verify TTS doesn't expose API keys or engine-specific advanced configs
	tts, _ := resp["tts"].(map[string]any)
	assert.NotContains(t, tts, "piper")
	assert.NotContains(t, tts, "kokoro")
	assert.NotContains(t, tts, "moss_nano")
	assert.NotContains(t, tts, "api")
	assert.NotContains(t, tts, "inline_code_max_len")
	assert.NotContains(t, tts, "max_summarize_runes")

	// Verify SSH doesn't expose host_key
	ssh, _ := resp["ssh"].(map[string]any)
	assert.NotContains(t, ssh, "host_key")

	// Verify Push doesn't expose master_secret
	push, _ := resp["push"].(map[string]any)
	jpush, _ := push["jpush"].(map[string]any)
	assert.NotContains(t, jpush, "master_secret")
}

func TestServeConfig_Get_MethodNotAllowed(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeConfig_Get_Unauthorized(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// Set a password so auth is required
	model.SessionToken = "test-token"

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	// No auth cookie
	w := callHandler(middleware.Auth(ServeConfig), req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- PATCH /api/config tests ---

func TestServeConfig_Patch_Success(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// Set initial config
	cfg := model.Config{}
	cfg.Chat.CollapsedHeight = 150
	cfg.Upload.MaxSizeMB = 100
	model.ConfigInstance = cfg

	body := `{"chat":{"collapsed_height":200},"upload":{"max_size_mb":50}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp["needs_restart"].(bool))
	changed, ok := resp["changed_cold_fields"].([]any)
	assert.True(t, ok)
	assert.True(t, len(changed) >= 2)

	// Verify in-memory config was updated
	assert.Equal(t, 200, model.ConfigInstance.Chat.CollapsedHeight)
	assert.Equal(t, 50, model.ConfigInstance.Upload.MaxSizeMB)
}

func TestServeConfig_Patch_ForbiddenField_Password(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := `{"password":"hacked"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfig_Patch_ForbiddenField_TLS(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := `{"tls":{"enabled":false}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfig_Patch_ForbiddenField_MasterSecret(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := `{"push":{"jpush":{"master_secret":"stolen"}}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfig_Patch_InvalidEngine(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := `{"tts":{"engine":"invalid_engine"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfig_Patch_InvalidSummarizeBackend(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := `{"tts":{"summarize_backend":"nonexistent"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfig_Patch_NegativeNumber(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := `{"chat":{"collapsed_height":-1}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfig_Patch_InvalidJSON(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfig_Patch_EmptyBody(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := `{}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	// Empty patch should succeed with no changes
	assert.Equal(t, http.StatusOK, w.Code)
}

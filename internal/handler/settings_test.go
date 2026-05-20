package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawbench/internal/middleware"
	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeConfig_Get(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

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
	cfg.Tasks.SummarizeBackend = "simple"
	cfg.Tasks.SummarizeModel = ""
	model.ConfigInstance = cfg

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Verify version is present
	assert.Contains(t, resp, "version")
	assert.NotEmpty(t, resp["version"])

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
	assert.Contains(t, resp, "tasks")

	// Verify specific values
	chat, _ := resp["chat"].(map[string]any)
	assert.Equal(t, float64(200), chat["collapsed_height"])
	assert.Equal(t, float64(15), chat["initial_messages"])

	upload, _ := resp["upload"].(map[string]any)
	assert.Equal(t, float64(50), upload["max_size_mb"])

	terminal, _ := resp["terminal"].(map[string]any)
	assert.Equal(t, true, terminal["enabled"])
	assert.Equal(t, "10m", terminal["idle_timeout"])

	// Verify tasks section
	tasks, _ := resp["tasks"].(map[string]any)
	assert.Equal(t, "simple", tasks["summarize_backend"])

	// When engine=edge, engine-specific sub-configs should NOT be present
	tts, _ := resp["tts"].(map[string]any)
	assert.NotContains(t, tts, "piper")
	assert.NotContains(t, tts, "kokoro")
	assert.NotContains(t, tts, "moss_nano")
	// When summarize_backend != "api", api sub-config should NOT be present
	assert.NotContains(t, tts, "api")
	// Internal fields should never be present
	assert.NotContains(t, tts, "inline_code_max_len")
	assert.NotContains(t, tts, "max_summarize_runes")

	// Verify sensitive fields are NOT present
	assert.NotContains(t, resp, "password")
	assert.NotContains(t, resp, "tls")
	assert.NotContains(t, resp, "host")
	assert.NotContains(t, resp, "port")
	assert.NotContains(t, resp, "log_level")
	assert.NotContains(t, resp, "log_dir")
	assert.NotContains(t, resp, "watch_dir")
	assert.NotContains(t, resp, "dev_port")

	// Verify SSH doesn't expose host_key
	ssh, _ := resp["ssh"].(map[string]any)
	assert.NotContains(t, ssh, "host_key")

	// Verify Push doesn't expose master_secret
	push, _ := resp["push"].(map[string]any)
	jpush, _ := push["jpush"].(map[string]any)
	assert.NotContains(t, jpush, "master_secret")
}

func TestServeConfig_Get_ConditionalPiperSubConfig(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.Engine = "piper"
	cfg.TTS.Piper.ModelPath = "/path/to/model.onnx"
	cfg.TTS.Piper.NoiseScale = 0.667
	cfg.TTS.Piper.LengthScale = 1.0
	cfg.TTS.Piper.SentenceSilence = 0.2
	model.ConfigInstance = cfg

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	tts, _ := resp["tts"].(map[string]any)
	assert.Contains(t, tts, "piper")
	// Kokoro/MossNano should not be present
	assert.NotContains(t, tts, "kokoro")
	assert.NotContains(t, tts, "moss_nano")

	piper, _ := tts["piper"].(map[string]any)
	assert.Equal(t, "/path/to/model.onnx", piper["model_path"])
	assert.Equal(t, 0.667, piper["noise_scale"])
}

func TestServeConfig_Get_ConditionalKokoroSubConfig(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.Engine = "kokoro"
	cfg.TTS.Kokoro.ModelPath = "/path/to/kokoro.onnx"
	cfg.TTS.Kokoro.VoicesPath = "/path/to/voices.bin"
	cfg.TTS.Kokoro.Lang = "cmn"
	model.ConfigInstance = cfg

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	tts, _ := resp["tts"].(map[string]any)
	assert.Contains(t, tts, "kokoro")
	assert.NotContains(t, tts, "piper")
	assert.NotContains(t, tts, "moss_nano")

	kokoro, _ := tts["kokoro"].(map[string]any)
	assert.Equal(t, "cmn", kokoro["lang"])
}

func TestServeConfig_Get_ConditionalMossNanoSubConfig(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.Engine = "moss-nano"
	cfg.TTS.MossNano.ModelDir = "/path/to/models"
	cfg.TTS.MossNano.Voice = "Junhao"
	cfg.TTS.MossNano.Backend = "onnx"
	model.ConfigInstance = cfg

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	tts, _ := resp["tts"].(map[string]any)
	assert.Contains(t, tts, "moss_nano")
	assert.NotContains(t, tts, "piper")
	assert.NotContains(t, tts, "kokoro")

	mossNano, _ := tts["moss_nano"].(map[string]any)
	assert.Equal(t, "onnx", mossNano["backend"])
	assert.Equal(t, "Junhao", mossNano["voice"])
}

func TestServeConfig_Get_ConditionalAPISubConfig(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.Engine = "edge"
	cfg.TTS.SummarizeBackend = "api"
	cfg.TTS.API.BaseURL = "https://api.openai.com/v1/chat/completions"
	cfg.TTS.API.Key = "sk-1234567890abcdefghijklmnopqrstuvwxyz"
	cfg.TTS.API.Format = "openai"
	cfg.TTS.API.Model = "gpt-4o-mini"
	model.ConfigInstance = cfg

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	tts, _ := resp["tts"].(map[string]any)
	assert.Contains(t, tts, "api")

	api, _ := tts["api"].(map[string]any)
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", api["base_url"])
	// API key must be masked
	assert.Contains(t, api["key"], "***")
	assert.NotEqual(t, "sk-1234567890abcdefghijklmnopqrstuvwxyz", api["key"])
	// Verify mask format: first 4 + *** + last 3
	assert.Equal(t, "sk-1***xyz", api["key"])
	assert.Equal(t, "openai", api["format"])
	assert.Equal(t, "gpt-4o-mini", api["model"])
}

func TestServeConfig_Get_APIMaskShortKey(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.SummarizeBackend = "api"
	cfg.TTS.API.Key = "short"
	model.ConfigInstance = cfg

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	tts, _ := resp["tts"].(map[string]any)
	api, _ := tts["api"].(map[string]any)
	assert.Equal(t, "****", api["key"])
}

func TestServeConfig_Get_APIMaskEmptyKey(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.SummarizeBackend = "api"
	cfg.TTS.API.Key = ""
	model.ConfigInstance = cfg

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	tts, _ := resp["tts"].(map[string]any)
	api, _ := tts["api"].(map[string]any)
	assert.Equal(t, "", api["key"])
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

	model.SessionToken = "test-token"

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	w := callHandler(middleware.Auth(ServeConfig), req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- PATCH /api/config tests ---

func TestServeConfig_Patch_Success(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

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

	assert.Equal(t, 200, model.ConfigInstance.Chat.CollapsedHeight)
	assert.Equal(t, 50, model.ConfigInstance.Upload.MaxSizeMB)
}

func TestServeConfig_Patch_PiperSubConfig(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.Engine = "piper"
	cfg.TTS.Piper.ModelPath = "/path/to/model.onnx" // required when engine=piper
	model.ConfigInstance = cfg

	body := `{"tts":{"piper":{"noise_scale":0.5,"length_scale":1.2,"sentence_silence":0.3}}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0.5, model.ConfigInstance.TTS.Piper.NoiseScale)
	assert.Equal(t, 1.2, model.ConfigInstance.TTS.Piper.LengthScale)
	assert.Equal(t, 0.3, model.ConfigInstance.TTS.Piper.SentenceSilence)
}

func TestServeConfig_Patch_APISubConfig(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	model.ConfigInstance = cfg

	body := `{"tts":{"api":{"base_url":"https://api.example.com/v1/chat","format":"openai","model":"gpt-4o-mini"}}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://api.example.com/v1/chat", model.ConfigInstance.TTS.API.BaseURL)
	assert.Equal(t, "openai", model.ConfigInstance.TTS.API.Format)
	assert.Equal(t, "gpt-4o-mini", model.ConfigInstance.TTS.API.Model)
}

func TestServeConfig_Patch_APIKeyMasked(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	model.ConfigInstance = cfg

	// PATCH with masked key containing *** should be rejected
	body := `{"tts":{"api":{"key":"sk-1***xyz"}}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfig_Patch_APIKeyFull(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	model.ConfigInstance = cfg

	body := `{"tts":{"api":{"key":"sk-1234567890abcdef"}}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "sk-1234567890abcdef", model.ConfigInstance.TTS.API.Key)
}

func TestServeConfig_Patch_TasksSection(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	model.ConfigInstance = cfg

	body := `{"tasks":{"summarize_backend":"codebuddy","summarize_model":"codebuddy-latest"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "codebuddy", model.ConfigInstance.Tasks.SummarizeBackend)
	assert.Equal(t, "codebuddy-latest", model.ConfigInstance.Tasks.SummarizeModel)
}

func TestServeConfig_Patch_MossNanoInvalidBackend(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	model.ConfigInstance = cfg

	body := `{"tts":{"moss_nano":{"backend":"invalid"}}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfig_Patch_APIInvalidFormat(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	model.ConfigInstance = cfg

	body := `{"tts":{"api":{"format":"invalid"}}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
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

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- POST /api/config/restart tests ---

func TestServeConfigRestart_Success(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	restartCh := make(chan struct{}, 1)
	SetRestartFunc(func() {
		restartCh <- struct{}{}
	})

	req := httptest.NewRequest(http.MethodPost, "/api/config/restart", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigRestart, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "restarting", resp["status"])

	select {
	case <-restartCh:
	case <-time.After(5 * time.Second):
		t.Fatal("restart function was not called within timeout")
	}
}

func TestServeConfigRestart_MethodNotAllowed(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/config/restart", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigRestart, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeConfig_Get_DefaultAgent(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.DefaultAgent = "codebuddy"
	model.ConfigInstance = cfg

	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.Contains(t, resp, "default_agent")
	assert.Equal(t, "codebuddy", resp["default_agent"])
}

func TestServeConfig_Patch_DefaultAgent(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.DefaultAgent = "codebuddy"
	model.ConfigInstance = cfg

	body := `{"default_agent":"claude"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "claude", model.ConfigInstance.DefaultAgent)
	assert.Equal(t, "claude", model.DefaultAgentID)
}

func TestServeConfig_Patch_SummarizeAPIWithoutBaseURL(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// Set up config with empty api.base_url
	cfg := model.Config{}
	cfg.TTS.SummarizeBackend = "simple" // current value is not "api"
	cfg.TTS.API.BaseURL = ""           // no base URL configured
	model.ConfigInstance = cfg

	// Try to patch summarize_backend to "api" without providing base_url
	body := `{"tts":{"summarize_backend":"api"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "base_url is required")
}

func TestServeConfig_Patch_SummarizeAPIWithBaseURL(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// Set up config with base_url already configured
	cfg := model.Config{}
	cfg.TTS.SummarizeBackend = "simple"
	cfg.TTS.API.BaseURL = "https://api.openai.com/v1"
	model.ConfigInstance = cfg

	// Patch summarize_backend to "api" — should succeed because base_url exists
	body := `{"tts":{"summarize_backend":"api"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServeConfig_Patch_TasksAPIWithoutBaseURL(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.Tasks.SummarizeBackend = "simple"
	cfg.TTS.API.BaseURL = ""
	model.ConfigInstance = cfg

	body := `{"tasks":{"summarize_backend":"api"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "base_url is required")
}

func TestServeConfig_Patch_PiperEngineWithoutModelPath(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.Engine = "edge"
	cfg.TTS.Piper.ModelPath = ""
	model.ConfigInstance = cfg

	body := `{"tts":{"engine":"piper"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "piper.model_path is required")
}

func TestServeConfig_Patch_KokoroEngineWithoutPaths(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.Engine = "edge"
	model.ConfigInstance = cfg

	body := `{"tts":{"engine":"kokoro"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "kokoro.model_path is required")
}

func TestServeConfig_Patch_MossNanoEngineWithoutModelDir(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	cfg.TTS.Engine = "edge"
	model.ConfigInstance = cfg

	body := `{"tts":{"engine":"moss-nano"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "moss_nano.model_dir is required")
}

func TestServeConfig_Patch_InvalidDefaultAgent(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// Set up agents so we can validate
	agentsDir := filepath.Join(t.TempDir(), "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "test.yaml"), []byte("id: test\nname: Test\nbackend: test\n"), 0644))
	require.NoError(t, model.LoadAgents(agentsDir))
	defer func() { model.Agents = nil; model.AgentList = nil }()

	cfg := model.Config{}
	model.ConfigInstance = cfg

	body := `{"default_agent":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

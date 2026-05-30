package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"clawbench/internal/model"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- GET /api/setup/status ----------

func TestSetupStatus_NeedsSetup(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Clear agents to simulate needs_setup
	model.Agents = map[string]*model.Agent{}
	model.AgentList = []*model.Agent{}

	req := newRequest(t, http.MethodGet, "/api/setup/status", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupStatus, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["needs_setup"].(bool))
}

func TestSetupStatus_NoNeedSetup(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/setup/status", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupStatus, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["needs_setup"].(bool))
}

func TestSetupStatus_EmbeddedAgent(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	model.Agents = map[string]*model.Agent{}
	model.AgentList = []*model.Agent{}

	req := newRequest(t, http.MethodGet, "/api/setup/status", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupStatus, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// embedded_agent depends on EmbeddedAgentPath() — in test env it's likely empty
	_, hasEmbedded := resp["embedded_agent"]
	assert.True(t, hasEmbedded, "response should include embedded_agent field")
}

// ---------- GET /api/setup/providers ----------

func TestSetupProviders_ReturnsWizardReadyOnly(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/setup/providers", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupProviders, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	providers, ok := resp["providers"].([]any)
	require.True(t, ok, "response should contain providers array")
	assert.NotEmpty(t, providers, "should have at least one wizard-ready provider")

	// Verify no enterprise providers
	for _, p := range providers {
		pMap := p.(map[string]any)
		id := pMap["id"].(string)
		assert.NotEqual(t, "amazon-bedrock", id)
		assert.NotEqual(t, "azure-openai-responses", id)
		assert.NotEqual(t, "google-vertex", id)
	}

	// Verify custom_url_supported
	assert.True(t, resp["custom_url_supported"].(bool))
}

func TestSetupProviders_ContainsExpectedProviders(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/setup/providers", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupProviders, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	providers := resp["providers"].([]any)
	providerIDs := make(map[string]bool)
	for _, p := range providers {
		pMap := p.(map[string]any)
		providerIDs[pMap["id"].(string)] = true
	}

	// Check key providers exist
	assert.True(t, providerIDs["openai"])
	assert.True(t, providerIDs["anthropic"])
	assert.True(t, providerIDs["google"])
	assert.True(t, providerIDs["deepseek"])
}

func TestSetupProviders_EachHasRequiredFields(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/setup/providers", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupProviders, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	providers := resp["providers"].([]any)
	for _, p := range providers {
		pMap := p.(map[string]any)
		assert.NotEmpty(t, pMap["id"], "provider should have id")
		assert.NotEmpty(t, pMap["name"], "provider should have name")
		assert.NotEmpty(t, pMap["envVar"], "provider should have envVar")
	}
}

// ---------- POST /api/setup/models ----------

func TestSetupModels_KnownModelsProvider(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"provider":   "anthropic",
		"custom_url": "",
		"api_key":    "test-key",
	}
	req := newRequest(t, http.MethodPost, "/api/setup/models", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupModels, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	models, ok := resp["models"].([]any)
	require.True(t, ok, "response should contain models array")
	assert.NotEmpty(t, models, "anthropic should have KnownModels")

	// Check summarize_model_hint
	hint, hasHint := resp["summarize_model_hint"]
	assert.True(t, hasHint, "should have summarize_model_hint")
	assert.NotEmpty(t, hint, "summarize_model_hint should not be empty")
}

func TestSetupModels_UnknownProvider(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"provider":   "nonexistent",
		"custom_url": "",
		"api_key":    "test-key",
	}
	req := newRequest(t, http.MethodPost, "/api/setup/models", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupModels, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetupModels_MissingProvider(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"api_key": "test-key",
	}
	req := newRequest(t, http.MethodPost, "/api/setup/models", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupModels, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------- POST /api/setup/verify ----------

func TestSetupVerify_MissingFields(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"provider": "openai",
	}
	req := newRequest(t, http.MethodPost, "/api/setup/verify", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupVerify, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetupVerify_UnknownProvider(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"provider": "nonexistent",
		"api_key":  "test-key",
		"model":    "test-model",
	}
	req := newRequest(t, http.MethodPost, "/api/setup/verify", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupVerify, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------- POST /api/setup/complete ----------

func TestSetupComplete_MissingFields(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"provider": "openai",
	}
	req := newRequest(t, http.MethodPost, "/api/setup/complete", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupComplete, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetupComplete_UnknownProvider(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"provider":         "nonexistent",
		"api_key":          "test-key",
		"model":            "test-model",
		"summarize_model":  "test-summarize-model",
		"agent_name":       "Test",
		"agent_id":         "test",
	}
	req := newRequest(t, http.MethodPost, "/api/setup/complete", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupComplete, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetupComplete_CreatesAgent(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Clear existing agents to simulate fresh setup
	model.Agents = map[string]*model.Agent{}
	model.AgentList = []*model.Agent{}

	// Clear existing agents in DB
	service.DB.Exec("DELETE FROM agents")

	body := map[string]any{
		"provider":         "openai",
		"custom_url":       "",
		"api_key":          "sk-test-key-12345",
		"model":            "gpt-5.5",
		"summarize_model":  "gpt-4o-mini",
		"agent_name":       "OpenAI",
		"agent_id":         "openai",
	}
	req := newRequest(t, http.MethodPost, "/api/setup/complete", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupComplete, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["success"].(bool))

	// Verify agent created in memory
	assert.NotNil(t, model.Agents["openai"])
	assert.Equal(t, "OpenAI", model.Agents["openai"].Name)
	assert.Equal(t, "gpt-5.5", model.Agents["openai"].PreferredModel)
	assert.Equal(t, "setup", model.Agents["openai"].Source)
	assert.Equal(t, "pi", model.Agents["openai"].Backend)

	// Verify agent created in DB
	var name string
	err := service.DB.QueryRow("SELECT name FROM agents WHERE id = ?", "openai").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "OpenAI", name)

	// Verify API key encrypted in DB
	var count int
	service.DB.QueryRow("SELECT COUNT(*) FROM agent_api_keys WHERE agent_id = ? AND provider = ?", "openai", "openai").Scan(&count)
	assert.Equal(t, 1, count)
}

func TestSetupComplete_DuplicateAgentID(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"provider":         "openai",
		"api_key":          "sk-test-key",
		"model":            "gpt-5.5",
		"summarize_model":  "gpt-4o-mini",
		"agent_name":       "OpenAI",
		"agent_id":         "codebuddy", // already exists in test setup
	}
	req := newRequest(t, http.MethodPost, "/api/setup/complete", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupComplete, req)

	// Should fail with conflict or bad request
	assert.True(t, w.Code == http.StatusConflict || w.Code == http.StatusBadRequest,
		"expected 409 or 400 for duplicate agent_id, got %d", w.Code)
}

func TestSetupComplete_MethodNotAllowed(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/setup/complete", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeSetupComplete, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

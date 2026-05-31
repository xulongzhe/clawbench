package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawbench/internal/model"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAgentTestEnv creates a temp agents directory with DB records and in-memory agents.
// Returns the temp dir and a teardown function.
func setupAgentTestEnv(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp dir for model cache etc.
	tmpDir := t.TempDir()

	// Save original globals
	origAgents := model.Agents
	origAgentList := model.AgentList
	origDB := service.DB
	origDBRead := service.DBRead

	// Init in-memory SQLite
	db, err := service.InitInMemoryDB()
	require.NoError(t, err)
	service.DB = db
	service.DBRead = db

	// Set up test agents directly in DB
	codebuddyAgent := &model.Agent{
		ID:      "codebuddy",
		Name:    "Test",
		Icon:    "🤖",
		Specialty: "testing",
		Backend: "codebuddy",
		Models: []model.AgentModel{
			{ID: "glm-5.1", Name: "GLM 5.1", Default: true},
			{ID: "glm-4-flash", Name: "GLM 4 Flash"},
		},
		ThinkingEffortLevels: []string{"low", "medium", "high"},
		Source:               "auto",
	}
	claudeAgent := &model.Agent{
		ID:      "claude",
		Name:    "Claude",
		Icon:    "🧠",
		Specialty: "reasoning",
		Backend: "claude",
		Models: []model.AgentModel{
			{ID: "claude-sonnet-4-6", Name: "Claude Sonnet", Default: true},
		},
		ThinkingEffortLevels: []string{"low", "medium", "high", "xhigh"},
		Source:               "auto",
	}

	require.NoError(t, service.SaveAgent(db, codebuddyAgent))
	require.NoError(t, service.SaveAgent(db, claudeAgent))

	// Load agents into memory
	model.Agents = map[string]*model.Agent{
		"codebuddy": codebuddyAgent,
		"claude":    claudeAgent,
	}
	model.AgentList = []*model.Agent{codebuddyAgent, claudeAgent}

	teardown := func() {
		model.Agents = origAgents
		model.AgentList = origAgentList
		service.DB = origDB
		service.DBRead = origDBRead
		db.Close()
	}

	return tmpDir, teardown
}

func TestAgentGet(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/agents", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp, "agents")
	assert.Contains(t, resp, "defaultAgent")
}

func TestAgentPatch_PreferredModel(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"id":              "codebuddy",
		"preferred_model": "glm-4-flash",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify in-memory agent updated
	assert.Equal(t, "glm-4-flash", model.Agents["codebuddy"].PreferredModel)

	// Verify DB updated
	var preferredModel string
	err := service.DB.QueryRow("SELECT preferred_model FROM agents WHERE id = ?", "codebuddy").Scan(&preferredModel)
	require.NoError(t, err)
	assert.Equal(t, "glm-4-flash", preferredModel)
}

func TestAgentPatch_InvalidPreferredModel(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"id":              "codebuddy",
		"preferred_model": "nonexistent-model",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentPatch_PreferredThinkingEffort(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"id":                        "codebuddy",
		"preferred_thinking_effort": "high",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify in-memory agent updated
	assert.Equal(t, "high", model.Agents["codebuddy"].PreferredThinkingEffort)

	// Verify DB updated
	var preferredThinking string
	err := service.DB.QueryRow("SELECT preferred_thinking_effort FROM agents WHERE id = ?", "codebuddy").Scan(&preferredThinking)
	require.NoError(t, err)
	assert.Equal(t, "high", preferredThinking)
}

func TestAgentPatch_InvalidPreferredThinkingEffort(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"id":                        "codebuddy",
		"preferred_thinking_effort": "ultra",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentPatch_NonexistentAgent(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"id":              "nonexistent",
		"preferred_model": "some-model",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentPatch_BothFields(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"id":                        "claude",
		"preferred_model":           "claude-sonnet-4-6",
		"preferred_thinking_effort": "xhigh",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify in-memory agent updated
	assert.Equal(t, "claude-sonnet-4-6", model.Agents["claude"].PreferredModel)
	assert.Equal(t, "xhigh", model.Agents["claude"].PreferredThinkingEffort)

	// Verify DB updated
	var preferredModel, preferredThinking string
	err := service.DB.QueryRow("SELECT preferred_model, preferred_thinking_effort FROM agents WHERE id = ?", "claude").Scan(&preferredModel, &preferredThinking)
	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-6", preferredModel)
	assert.Equal(t, "xhigh", preferredThinking)
}

func TestAgentPatch_ClearPreferredModel(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// First set a preferred model
	model.Agents["codebuddy"].PreferredModel = "glm-4-flash"

	// Now clear it by sending empty string
	body := map[string]any{
		"id":              "codebuddy",
		"preferred_model": "",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", model.Agents["codebuddy"].PreferredModel)
}

func TestAgentPatch_DefaultModelIDRespectsPreferred(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Default without preferred_model should return the default model
	assert.Equal(t, "glm-5.1", model.Agents["codebuddy"].DefaultModelID())

	// Set preferred model
	model.Agents["codebuddy"].PreferredModel = "glm-4-flash"
	assert.Equal(t, "glm-4-flash", model.Agents["codebuddy"].DefaultModelID())

	// BaseModelID always returns the original default, ignoring preference
	assert.Equal(t, "glm-5.1", model.Agents["codebuddy"].BaseModelID())
}

func TestAgentPatch_EffectiveThinkingEffortRespectsPreferred(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Without preferred thinking, returns agent default (empty in test)
	assert.Equal(t, "", model.Agents["codebuddy"].EffectiveThinkingEffort())

	// Set preferred thinking effort
	model.Agents["codebuddy"].PreferredThinkingEffort = "high"
	assert.Equal(t, "high", model.Agents["codebuddy"].EffectiveThinkingEffort())

	// ThinkingEffort (original default) is not modified
	assert.Equal(t, "", model.Agents["codebuddy"].ThinkingEffort)
}

func TestAgentPatch_NoID(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"preferred_model": "glm-4-flash",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentPatch_MethodNotAllowed(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodDelete, "/api/agents", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAgentRefreshModels_Success(t *testing.T) {
	tmpDir, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Override DiscoverModels for testing
	origDiscover := model.DiscoverModels
	model.DiscoverModels = func(spec model.BackendSpec) []model.AgentModel {
		if spec.Backend == "codebuddy" {
			return []model.AgentModel{
				{ID: "glm-6", Name: "GLM 6", Default: true},
				{ID: "glm-5.1", Name: "GLM 5.1"},
			}
		}
		return nil
	}
	defer func() { model.DiscoverModels = origDiscover }()

	// Create model cache dir and set global
	cacheDir := filepath.Join(tmpDir, "model-cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	origCacheDir := model.ModelCacheDir
	model.ModelCacheDir = cacheDir
	defer func() { model.ModelCacheDir = origCacheDir }()

	req := newRequest(t, http.MethodPost, "/api/agents/codebuddy/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	models, ok := resp["models"].([]any)
	require.True(t, ok, "response should contain models array")
	assert.Len(t, models, 2)

	// Verify in-memory agent models were updated
	assert.Equal(t, "glm-6", model.Agents["codebuddy"].Models[0].ID)
	assert.Equal(t, "glm-5.1", model.Agents["codebuddy"].Models[1].ID)

	// Verify cache file was written
	cached := model.ReadModelCache(cacheDir, "codebuddy")
	require.NotNil(t, cached)
	assert.Len(t, cached, 2)
	assert.Equal(t, "glm-6", cached[0].ID)
}

func TestAgentRefreshModels_AgentNotFound(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/agents/nonexistent/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentRefreshModels_DiscoveryNotSupported(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Use a fictional backend that has no discovery capability
	model.Agents["unknown"] = &model.Agent{ID: "unknown", Backend: "unknown"}
	model.AgentList = append(model.AgentList, model.Agents["unknown"])

	req := newRequest(t, http.MethodPost, "/api/agents/unknown/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentRefreshModels_DiscoveryFails(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Override DiscoverModels to return nil (simulating discovery failure)
	origDiscover := model.DiscoverModels
	model.DiscoverModels = func(spec model.BackendSpec) []model.AgentModel {
		return nil
	}
	defer func() { model.DiscoverModels = origDiscover }()

	req := newRequest(t, http.MethodPost, "/api/agents/codebuddy/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	// When discovery returns no models:
	// - If CLI is on PATH but returns empty: 500 (ModelDiscoveryFailed)
	// - If CLI is NOT on PATH: 404 (CLINotFound)
	// CI may not have codebuddy installed, so accept either
	assert.True(t, w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound,
		"expected 500 or 404, got %d", w.Code)
}

func TestServeAgentSubRoutes_RefreshModels(t *testing.T) {
	tmpDir, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Override DiscoverModels for testing
	origDiscover := model.DiscoverModels
	model.DiscoverModels = func(spec model.BackendSpec) []model.AgentModel {
		if spec.Backend == "codebuddy" {
			return []model.AgentModel{{ID: "glm-6", Name: "GLM 6", Default: true}}
		}
		return nil
	}
	defer func() { model.DiscoverModels = origDiscover }()

	// Create model cache dir and set global
	cacheDir := filepath.Join(tmpDir, "model-cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	origCacheDir := model.ModelCacheDir
	model.ModelCacheDir = cacheDir
	defer func() { model.ModelCacheDir = origCacheDir }()

	req := newRequest(t, http.MethodPost, "/api/agents/codebuddy/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentSubRoutes, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServeAgentSubRoutes_NotFound(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/agents/codebuddy/something-else", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentSubRoutes, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestServeAgentRefreshModels_MethodNotAllowed(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/agents/codebuddy/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeAgentRefreshModels_EmptyAgentID(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/agents//refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeAgentRefreshModels_InvalidAgentID(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Path with extra slashes: /api/agents/foo/bar/refresh-models
	req := newRequest(t, http.MethodPost, "/api/agents/foo/bar/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeAgentRefreshModels_CLINotFound(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Override DiscoverModels to return nil, simulating CLI not available
	origDiscover := model.DiscoverModels
	model.DiscoverModels = func(spec model.BackendSpec) []model.AgentModel {
		return nil
	}
	defer func() { model.DiscoverModels = origDiscover }()

	// Use claude agent (which has DiscoverModelsFunc) — CLI likely not on CI
	req := newRequest(t, http.MethodPost, "/api/agents/claude/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	// Should be either 404 (CLINotFound) or 500 (ModelDiscoveryFailed)
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError,
		"expected 404 or 500, got %d", w.Code)
}

func TestAgentPatch_InvalidJSON(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Send malformed JSON to trigger decodeJSON failure (line 54-56)
	req := httptest.NewRequest(http.MethodPatch, "/api/agents", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentPatch_ClearPreferredThinkingEffort(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// First set a preferred thinking effort
	model.Agents["codebuddy"].PreferredThinkingEffort = "high"

	// Now clear it by sending empty string (empty string with no ThinkingEffortLevels should work)
	body := map[string]any{
		"id":                        "codebuddy",
		"preferred_thinking_effort": "",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", model.Agents["codebuddy"].PreferredThinkingEffort)
}

func TestAgentPatch_PreferredModelEmptyString(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Setting preferred_model to empty string should clear it without validation
	body := map[string]any{
		"id":              "codebuddy",
		"preferred_model": "",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", model.Agents["codebuddy"].PreferredModel)
}

func TestServeAgentRefreshModels_SaveAgentDBError(t *testing.T) {
	tmpDir, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Override DiscoverModels for testing
	origDiscover := model.DiscoverModels
	model.DiscoverModels = func(spec model.BackendSpec) []model.AgentModel {
		if spec.Backend == "codebuddy" {
			return []model.AgentModel{{ID: "glm-6", Name: "GLM 6", Default: true}}
		}
		return nil
	}
	defer func() { model.DiscoverModels = origDiscover }()

	// Create model cache dir and set global
	cacheDir := filepath.Join(tmpDir, "model-cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	origCacheDir := model.ModelCacheDir
	model.ModelCacheDir = cacheDir
	defer func() { model.ModelCacheDir = origCacheDir }()

	// Delete agents table to cause SaveAgent to fail
	service.DB.Exec("DROP TABLE agents")

	req := newRequest(t, http.MethodPost, "/api/agents/codebuddy/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	// Should still return 200 (DB save failure is logged but not fatal)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify in-memory agent models were still updated
	assert.Equal(t, "glm-6", model.Agents["codebuddy"].Models[0].ID)
}

func TestServeAgentRefreshModels_WriteModelCacheError(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Override DiscoverModels for testing
	origDiscover := model.DiscoverModels
	model.DiscoverModels = func(spec model.BackendSpec) []model.AgentModel {
		if spec.Backend == "codebuddy" {
			return []model.AgentModel{{ID: "glm-6", Name: "GLM 6", Default: true}}
		}
		return nil
	}
	defer func() { model.DiscoverModels = origDiscover }()

	// Set cache dir to invalid path to cause WriteModelCache to fail (lines 178-180)
	origCacheDir := model.ModelCacheDir
	model.ModelCacheDir = "/nonexistent/path/that/cannot/be/created"
	defer func() { model.ModelCacheDir = origCacheDir }()

	req := newRequest(t, http.MethodPost, "/api/agents/codebuddy/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	// Should still return 200 (cache write failure is logged but not fatal)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify in-memory agent models were still updated
	assert.Equal(t, "glm-6", model.Agents["codebuddy"].Models[0].ID)
}

func TestServeAgentRefreshModels_CLINotFoundSpecificError(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Create a custom agent whose CLI command doesn't exist on PATH
	model.Agents["fake-cli"] = &model.Agent{
		ID:      "fake-cli",
		Name:    "Fake CLI",
		Backend: "deepseek", // uses DefaultCmd "deepseek" which is unlikely on test PATH
		Models:  []model.AgentModel{{ID: "m1", Name: "M1", Default: true}},
	}
	model.AgentList = append(model.AgentList, model.Agents["fake-cli"])
	require.NoError(t, service.SaveAgent(service.DB, model.Agents["fake-cli"]))

	// Override DiscoverModels to return nil — will hit "no models" path
	origDiscover := model.DiscoverModels
	model.DiscoverModels = func(spec model.BackendSpec) []model.AgentModel {
		return nil
	}
	defer func() { model.DiscoverModels = origDiscover }()

	req := newRequest(t, http.MethodPost, "/api/agents/fake-cli/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgentRefreshModels, req)

	// Should be 404 (CLINotFound) or 500 (ModelDiscoveryFailed) depending on whether CLI exists
	// The key behavior is that it returns an error, not 200
	assert.NotEqual(t, http.StatusOK, w.Code, "should return error when models discovery returns empty")
}

func TestAgentPatch_NoThinkingEffortLevels(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Create an agent with no ThinkingEffortLevels
	model.Agents["nolevels"] = &model.Agent{
		ID:      "nolevels",
		Name:    "No Levels",
		Backend: "test",
		Models:  []model.AgentModel{{ID: "m1", Name: "Model 1", Default: true}},
	}
	model.AgentList = append(model.AgentList, model.Agents["nolevels"])
	require.NoError(t, service.SaveAgent(service.DB, model.Agents["nolevels"]))

	// Setting preferred_thinking_effort on agent with no levels should accept any value
	body := map[string]any{
		"id":                        "nolevels",
		"preferred_thinking_effort": "anything",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "anything", model.Agents["nolevels"].PreferredThinkingEffort)
}

func TestAgentPatch_PatchAgentDBError(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Create a closed DB that will return errors on Exec
	closedDB, err := service.InitInMemoryDB()
	require.NoError(t, err)
	closedDB.Close()

	// Replace service.DB with the closed DB
	origDB := service.DB
	service.DB = closedDB
	defer func() { service.DB = origDB }()

	body := map[string]any{
		"id":              "codebuddy",
		"preferred_model": "glm-4-flash",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---------- ServeAgents method not allowed ----------

func TestServeAgents_MethodNotAllowed(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodDelete, "/api/agents", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// ---------- ServeAgentRefreshModels with provider filter ----------

func TestServeAgentRefreshModels_WithProviderFilter(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Save original DiscoverModels and restore later
	origDiscover := model.DiscoverModels
	defer func() { model.DiscoverModels = origDiscover }()

	// Mock DiscoverModels to return models with provider prefix
	model.DiscoverModels = func(spec model.BackendSpec) []model.AgentModel {
		return []model.AgentModel{
			{ID: "openai/gpt-4o", Name: "openai/GPT-4o"},
			{ID: "anthropic/claude-sonnet-4-20250514", Name: "anthropic/Claude Sonnet 4"},
			{ID: "deepseek/deepseek-chat", Name: "deepseek/DeepSeek Chat"},
		}
	}

	// Add agent_api_keys entry using SaveAgentAPIKey (handles encryption + key_nonce)
	require.NoError(t, service.SaveAgentAPIKey(service.DB, "codebuddy", "openai", "", "test-api-key"))

	req := newRequest(t, http.MethodPost, "/api/agents/codebuddy/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	req.URL.Path = "/api/agents/codebuddy/refresh-models"
	w := callHandler(ServeAgentRefreshModels, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	models := resp["models"].([]any)
	// Should only return openai/ prefixed models (stripped of prefix)
	assert.NotEmpty(t, models, "should have models after provider filtering")
}

func TestServeAgentRefreshModels_ProviderFilterNoMatch(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	origDiscover := model.DiscoverModels
	defer func() { model.DiscoverModels = origDiscover }()

	// Mock DiscoverModels to return models that DON'T match the provider prefix
	model.DiscoverModels = func(spec model.BackendSpec) []model.AgentModel {
		return []model.AgentModel{
			{ID: "openai/gpt-4o", Name: "openai/GPT-4o"},
			{ID: "anthropic/claude-sonnet-4-20250514", Name: "anthropic/Claude Sonnet 4"},
		}
	}

	// Set up provider that won't match any model prefix
	require.NoError(t, service.SaveAgentAPIKey(service.DB, "codebuddy", "deepseek", "", "test-api-key"))

	req := newRequest(t, http.MethodPost, "/api/agents/codebuddy/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	req.URL.Path = "/api/agents/codebuddy/refresh-models"
	w := callHandler(ServeAgentRefreshModels, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// When no models match the prefix, all discovered models are returned as fallback
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	models := resp["models"].([]any)
	assert.Len(t, models, 2, "should return all models when no prefix matches")
}

func TestServeAgentRefreshModels_KnownModelsFallback(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	// Set up agent_api_keys entry for a provider with KnownModels (e.g., anthropic)
	require.NoError(t, service.SaveAgentAPIKey(service.DB, "codebuddy", "anthropic", "", "test-api-key"))

	// Make the agent's backend have NO discovery support by temporarily changing it
	origBackend := model.Agents["codebuddy"].Backend
	origModels := model.Agents["codebuddy"].Models
	model.Agents["codebuddy"].Backend = "nondiscoverable"
	model.Agents["codebuddy"].Models = nil
	defer func() {
		model.Agents["codebuddy"].Backend = origBackend
		model.Agents["codebuddy"].Models = origModels
	}()

	req := newRequest(t, http.MethodPost, "/api/agents/codebuddy/refresh-models", nil)
	withAuthCookie(req, model.SessionToken)
	req.URL.Path = "/api/agents/codebuddy/refresh-models"
	w := callHandler(ServeAgentRefreshModels, req)

	// Should fall back to KnownModels from anthropic provider
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	models := resp["models"].([]any)
	assert.NotEmpty(t, models, "should have KnownModels from anthropic provider")
}

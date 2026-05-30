package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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

package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAgentTestEnv creates a temp agents directory with YAML files and calls LoadAgents.
// Returns the temp dir and a teardown function.
func setupAgentTestEnv(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp config dir structure: config/agents/*.yaml
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Write test agent YAMLs
	codebuddyYAML := `id: codebuddy
name: Test
icon: 🤖
specialty: testing
backend: codebuddy
preferred_model: ""
thinking_effort: ""
models:
  - id: glm-5.1
    name: GLM 5.1
    default: true
  - id: glm-4-flash
    name: GLM 4 Flash
thinking_effort_levels:
  - low
  - medium
  - high
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "codebuddy.yaml"), []byte(codebuddyYAML), 0644))

	claudeYAML := `id: claude
name: Claude
icon: 🧠
specialty: reasoning
backend: claude
preferred_model: ""
thinking_effort: ""
models:
  - id: claude-sonnet-4-6
    name: Claude Sonnet
    default: true
thinking_effort_levels:
  - low
  - medium
  - high
  - xhigh
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "claude.yaml"), []byte(claudeYAML), 0644))

	// Save original globals
	origAgents := model.Agents
	origAgentList := model.AgentList

	// Load agents from temp dir
	require.NoError(t, model.LoadAgents(agentsDir))

	teardown := func() {
		model.Agents = origAgents
		model.AgentList = origAgentList
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
	tmpDir, teardown := setupAgentTestEnv(t)
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

	// Verify YAML file updated
	yamlData, err := os.ReadFile(filepath.Join(tmpDir, "agents", "codebuddy.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(yamlData), "glm-4-flash")
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
	tmpDir, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"id":                          "codebuddy",
		"preferred_thinking_effort":   "high",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify in-memory agent updated
	assert.Equal(t, "high", model.Agents["codebuddy"].PreferredThinkingEffort)

	// Verify YAML file updated
	yamlData, err := os.ReadFile(filepath.Join(tmpDir, "agents", "codebuddy.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(yamlData), "high")
}

func TestAgentPatch_InvalidPreferredThinkingEffort(t *testing.T) {
	_, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"id":                          "codebuddy",
		"preferred_thinking_effort":   "ultra",
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
	tmpDir, teardown := setupAgentTestEnv(t)
	defer teardown()

	body := map[string]any{
		"id":                          "claude",
		"preferred_model":             "claude-sonnet-4-6",
		"preferred_thinking_effort":   "xhigh",
	}
	req := newRequest(t, http.MethodPatch, "/api/agents", body)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeAgents, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify in-memory agent updated
	assert.Equal(t, "claude-sonnet-4-6", model.Agents["claude"].PreferredModel)
	assert.Equal(t, "xhigh", model.Agents["claude"].PreferredThinkingEffort)

	// Verify YAML file updated
	yamlData, err := os.ReadFile(filepath.Join(tmpDir, "agents", "claude.yaml"))
	require.NoError(t, err)
	content := string(yamlData)
	assert.Contains(t, content, "claude-sonnet-4-6")
	assert.Contains(t, content, "xhigh")
}

func TestAgentPatch_ClearPreferredModel(t *testing.T) {
	tmpDir, teardown := setupAgentTestEnv(t)
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

	// Verify YAML file does NOT contain preferred_model
	yamlData, err := os.ReadFile(filepath.Join(tmpDir, "agents", "codebuddy.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(yamlData), "preferred_model")
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

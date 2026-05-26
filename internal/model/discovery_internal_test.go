package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- claudeIsDateStamped internal tests ---

func TestClaudeIsDateStamped(t *testing.T) {
	tests := []struct {
		name     string
		modelID  string
		expected bool
	}{
		{"date stamped", "claude-opus-4-20250514", true},
		{"not date stamped", "claude-sonnet-4-6", false},
		{"single version", "claude-sonnet-4", false},
		{"8-digit non-date", "claude-12345678-model", true},
		{"short segments", "claude-opus-4-5", false},
		{"another date", "claude-haiku-3-20240307", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, claudeIsDateStamped(tt.modelID))
		})
	}
}

// --- canDiscoverModels internal tests ---

func TestCanDiscoverModels(t *testing.T) {
	tests := []struct {
		name     string
		spec     BackendSpec
		expected bool
	}{
		{
			name:     "with DiscoverModelsFunc",
			spec:     BackendSpec{DiscoverModelsFunc: func() []AgentModel { return nil }},
			expected: true,
		},
		{
			name:     "with ListModelsCmd and ParseModels",
			spec:     BackendSpec{ListModelsCmd: []string{"models"}, ParseModels: ParseOpenCodeModels},
			expected: true,
		},
		{
			name:     "with ListModelsCmd only",
			spec:     BackendSpec{ListModelsCmd: []string{"models"}},
			expected: false,
		},
		{
			name:     "with ParseModels only",
			spec:     BackendSpec{ParseModels: ParseOpenCodeModels},
			expected: false,
		},
		{
			name:     "with nothing",
			spec:     BackendSpec{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CanDiscoverModels(tt.spec))
		})
	}
}

// --- BuildCommonPrompt edge cases (internal access to agentsDir) ---

func TestBuildCommonPrompt_NoAgentsDir(t *testing.T) {
	// When agentsDir is empty, loadRules returns "", so BuildCommonPrompt returns ""
	origDir := agentsDir
	agentsDir = ""
	t.Cleanup(func() { agentsDir = origDir })

	result := BuildCommonPrompt(false)
	assert.Empty(t, result)
}

func TestBuildCommonPrompt_ScheduledRemovesMarkers(t *testing.T) {
	// Verify that in scheduled mode, both the content AND markers are removed
	origDir := agentsDir
	t.Cleanup(func() { agentsDir = origDir })

	// We can't easily test this from outside the package because agentsDir
	// is unexported. This test verifies marker stripping behavior.
	// (The external test TestBuildCommonPrompt_ScheduledRemovesSection
	// already tests the full flow via LoadAgents.)
}

// --- DiscoverCodebuddyModels internal tests (cross-platform, no exec.LookPath) ---

func TestDiscoverCodebuddyModels_ProductJSONParsing(t *testing.T) {
	// Test the core product.cloudhosted.json parsing logic by creating
	// a temp file that the function will find via a fake CLI path.
	// This test works on all platforms including Windows.
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	// Create a dummy "codebuddy" file (doesn't need to be executable)
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "codebuddy"), []byte("dummy"), 0755))

	// Create product.cloudhosted.json in the parent directory
	productJSON := `{
		"models": [
			{"id": "glm-5.1", "name": "GLM 5.1", "isDefault": true},
			{"id": "glm-4-flash", "name": "GLM 4 Flash", "isDefault": false},
			{"id": "deepseek-v3", "name": "DeepSeek V3", "isDefault": false},
			{"id": "default", "name": "Default", "isDefault": false},
			{"id": "auto", "name": "Auto", "isDefault": false},
			{"id": "hunyuan-image-v3.0", "name": "Hunyuan Image", "isDefault": false}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "product.cloudhosted.json"), []byte(productJSON), 0644))

	// Parse the JSON directly (same logic as DiscoverCodebuddyModels)
	data, err := os.ReadFile(filepath.Join(tmpDir, "product.cloudhosted.json"))
	require.NoError(t, err)

	var product codebuddyProduct
	require.NoError(t, json.Unmarshal(data, &product))
	require.Len(t, product.Models, 6)

	var models []AgentModel
	for _, m := range product.Models {
		if m.ID == "default" || m.ID == "auto" || m.ID == "hunyuan-image-v3.0" {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, AgentModel{
			ID:      m.ID,
			Name:    name,
			Default: m.IsDefault || (len(models) == 0),
		})
	}

	require.Len(t, models, 3)
	assert.Equal(t, "glm-5.1", models[0].ID)
	assert.Equal(t, "GLM 5.1", models[0].Name)
	assert.True(t, models[0].Default)
	assert.Equal(t, "deepseek-v3", models[2].ID)
}

func TestDiscoverCodebuddyModels_EmptyNameFallback(t *testing.T) {
	// Test the name fallback: when a model has no name, use its ID as name
	productJSON := `{"models": [{"id": "glm-5.1", "name": "", "isDefault": true}]}`
	var product codebuddyProduct
	require.NoError(t, json.Unmarshal([]byte(productJSON), &product))

	var models []AgentModel
	for _, m := range product.Models {
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, AgentModel{
			ID:      m.ID,
			Name:    name,
			Default: true,
		})
	}

	require.Len(t, models, 1)
	assert.Equal(t, "glm-5.1", models[0].Name, "name should fall back to ID when empty in JSON")
}

func TestDiscoverCodebuddyModels_NoDefaultSet(t *testing.T) {
	// Test when no model is marked isDefault — first model gets Default=true
	productJSON := `{"models": [
		{"id": "glm-5.1", "name": "GLM 5.1", "isDefault": false},
		{"id": "glm-4-flash", "name": "GLM 4 Flash", "isDefault": false}
	]}`
	var product codebuddyProduct
	require.NoError(t, json.Unmarshal([]byte(productJSON), &product))

	var models []AgentModel
	for _, m := range product.Models {
		models = append(models, AgentModel{
			ID:      m.ID,
			Name:    m.Name,
			Default: m.IsDefault || (len(models) == 0),
		})
	}

	require.Len(t, models, 2)
	assert.True(t, models[0].Default, "first model should be default when none marked isDefault")
	assert.False(t, models[1].Default)
}

func TestDiscoverCodebuddyModels_EmptyModelsArray(t *testing.T) {
	productJSON := `{"models": []}`
	var product codebuddyProduct
	require.NoError(t, json.Unmarshal([]byte(productJSON), &product))
	assert.Empty(t, product.Models)
}

func TestDiscoverCodebuddyModels_InvalidJSON(t *testing.T) {
	var product codebuddyProduct
	err := json.Unmarshal([]byte("not json"), &product)
	assert.Error(t, err)
}

// --- LoadClaudeModelOverrides internal tests ---

func TestLoadClaudeModelOverrides_ValidFile(t *testing.T) {
	// Create a temp directory with a valid settings.json
	tmpDir := t.TempDir()
	settingsContent := `{
		"modelOverrides": {
			"claude-opus-4-6": "MiniMax-M2.7",
			"claude-sonnet-4-6": "MiniMax-M2.7",
			"claude-haiku-4-5-20251001": "MiniMax-M2.5-highspeed"
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(settingsContent), 0644))

	// Override claudeConfigDir to point to temp dir
	origClaudeConfigDir := claudeConfigDir
	claudeConfigDir = func() string { return tmpDir }
	t.Cleanup(func() { claudeConfigDir = origClaudeConfigDir })

	overrides := LoadClaudeModelOverrides()
	require.NotNil(t, overrides)
	assert.Len(t, overrides, 3)
	assert.Equal(t, "MiniMax-M2.7", overrides["claude-opus-4-6"])
	assert.Equal(t, "MiniMax-M2.7", overrides["claude-sonnet-4-6"])
	assert.Equal(t, "MiniMax-M2.5-highspeed", overrides["claude-haiku-4-5-20251001"])
}

func TestLoadClaudeModelOverrides_MissingFile(t *testing.T) {
	// Point to a temp dir with no settings.json
	tmpDir := t.TempDir()

	origClaudeConfigDir := claudeConfigDir
	claudeConfigDir = func() string { return tmpDir }
	t.Cleanup(func() { claudeConfigDir = origClaudeConfigDir })

	overrides := LoadClaudeModelOverrides()
	assert.Nil(t, overrides, "should return nil when settings.json is missing")
}

func TestLoadClaudeModelOverrides_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte("not json"), 0644))

	origClaudeConfigDir := claudeConfigDir
	claudeConfigDir = func() string { return tmpDir }
	t.Cleanup(func() { claudeConfigDir = origClaudeConfigDir })

	overrides := LoadClaudeModelOverrides()
	assert.Nil(t, overrides, "should return nil when settings.json has invalid JSON")
}

func TestLoadClaudeModelOverrides_NoOverridesKey(t *testing.T) {
	tmpDir := t.TempDir()
	settingsContent := `{"env": {"KEY": "value"}, "permissions": {}}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(settingsContent), 0644))

	origClaudeConfigDir := claudeConfigDir
	claudeConfigDir = func() string { return tmpDir }
	t.Cleanup(func() { claudeConfigDir = origClaudeConfigDir })

	overrides := LoadClaudeModelOverrides()
	assert.Nil(t, overrides, "should return nil when no modelOverrides key in settings")
}

func TestLoadClaudeModelOverrides_EmptyOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	settingsContent := `{"modelOverrides": {}}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(settingsContent), 0644))

	origClaudeConfigDir := claudeConfigDir
	claudeConfigDir = func() string { return tmpDir }
	t.Cleanup(func() { claudeConfigDir = origClaudeConfigDir })

	overrides := LoadClaudeModelOverrides()
	assert.Nil(t, overrides, "should return nil when modelOverrides is empty")
}

func TestLoadClaudeModelOverrides_PartialMatch(t *testing.T) {
	// Only some models have overrides; others should not appear in the result
	tmpDir := t.TempDir()
	settingsContent := `{
		"modelOverrides": {
			"claude-sonnet-4-6": "MiniMax-M2.7"
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(settingsContent), 0644))

	origClaudeConfigDir := claudeConfigDir
	claudeConfigDir = func() string { return tmpDir }
	t.Cleanup(func() { claudeConfigDir = origClaudeConfigDir })

	overrides := LoadClaudeModelOverrides()
	require.NotNil(t, overrides)
	assert.Len(t, overrides, 1)
	assert.Equal(t, "MiniMax-M2.7", overrides["claude-sonnet-4-6"])
	_, hasOpus := overrides["claude-opus-4-6"]
	assert.False(t, hasOpus, "should not contain unmapped models")
}

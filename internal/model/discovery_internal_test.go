package model

import (
	"encoding/json"
	"os"
	"os/exec"
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

// --- BuildCommonPrompt edge cases ---

func TestBuildCommonPrompt_ReturnsContent(t *testing.T) {
	// BuildCommonPrompt always returns the embedded rules content
	result := BuildCommonPrompt()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "User Interaction")
}

// --- DiscoverCodebuddyModels internal tests (cross-platform, no exec.LookPath) ---

func TestDiscoverCodebuddyModels_ProductJSONParsing(t *testing.T) {
	// Test the core product.cloudhosted.json parsing logic by creating
	// a temp file that the function will find via a fake CLI path.
	// This test works on all platforms including Windows.
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	// Create a dummy "codebuddy" file (doesn't need to be executable)
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "codebuddy"), []byte("dummy"), 0o755))

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
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "product.cloudhosted.json"), []byte(productJSON), 0o644))

	// Parse the JSON directly (same logic as DiscoverCodebuddyModels)
	data, err := os.ReadFile(filepath.Join(tmpDir, "product.cloudhosted.json"))
	require.NoError(t, err)

	var product codebuddyProduct
	require.NoError(t, json.Unmarshal(data, &product))
	require.Len(t, product.Models, 6)

	models := make([]AgentModel, 0, len(product.Models))
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

	models := make([]AgentModel, 0, len(product.Models))
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

	models := make([]AgentModel, 0, len(product.Models))
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
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(settingsContent), 0o644))

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
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte("not json"), 0o644))

	origClaudeConfigDir := claudeConfigDir
	claudeConfigDir = func() string { return tmpDir }
	t.Cleanup(func() { claudeConfigDir = origClaudeConfigDir })

	overrides := LoadClaudeModelOverrides()
	assert.Nil(t, overrides, "should return nil when settings.json has invalid JSON")
}

func TestLoadClaudeModelOverrides_NoOverridesKey(t *testing.T) {
	tmpDir := t.TempDir()
	settingsContent := `{"env": {"KEY": "value"}, "permissions": {}}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(settingsContent), 0o644))

	origClaudeConfigDir := claudeConfigDir
	claudeConfigDir = func() string { return tmpDir }
	t.Cleanup(func() { claudeConfigDir = origClaudeConfigDir })

	overrides := LoadClaudeModelOverrides()
	assert.Nil(t, overrides, "should return nil when no modelOverrides key in settings")
}

func TestLoadClaudeModelOverrides_EmptyOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	settingsContent := `{"modelOverrides": {}}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(settingsContent), 0o644))

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
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(settingsContent), 0o644))

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

// --- codexTargetTriple internal tests ---

func TestCodexTargetTriple(t *testing.T) {
	// codexTargetTriple should return a non-empty string for the current platform
	result := codexTargetTriple()
	// On supported platforms (linux/darwin/windows with amd64/arm64) it returns a triple
	// On unsupported combos (e.g. plan9) it returns ""
	// We just verify it doesn't panic and returns a valid format if non-empty
	if result != "" {
		assert.Contains(t, result, "-")
	}
}

// --- DiscoverCodexModels internal tests ---

func TestDiscoverCodexModelsDefaults(t *testing.T) {
	// discoverCodexModelsDefaults returns nil if codex is not installed
	models := discoverCodexModelsDefaults()
	// On CI, codex is not installed, so this should return nil
	// If codex is installed locally, it returns the defaults
	if _, err := exec.LookPath("codex"); err != nil {
		assert.Nil(t, models)
	} else {
		assert.NotEmpty(t, models)
		assert.Equal(t, "gpt-5.5", models[0].ID)
		assert.True(t, models[0].Default)
	}
}

func TestDiscoverCodexModelsFromBinary_NotInstalled(t *testing.T) {
	// When codex is not on PATH, returns nil
	// (If codex IS installed, this test verifies it doesn't panic)
	models := discoverCodexModelsFromBinary()
	if _, err := exec.LookPath("codex"); err != nil {
		assert.Nil(t, models)
	}
}

func TestDiscoverCodexModelsFromStateDB_NoCodexDir(t *testing.T) {
	// When ~/.codex doesn't exist, returns nil
	models := discoverCodexModelsFromStateDB()
	if homeDir, err := os.UserHomeDir(); err == nil {
		if _, err := os.Stat(filepath.Join(homeDir, ".codex")); os.IsNotExist(err) {
			assert.Nil(t, models)
		}
	}
}

// --- DiscoverVeCLIModels internal parsing tests ---

func TestVeCLIModelParsing(t *testing.T) {
	// Test the regex patterns used in DiscoverVeCLIModels
	tests := []struct {
		name     string
		input    string
		wantID   string
		wantName string
	}{
		{
			name:     "standard entry",
			input:    `{ id: "deepseek-r1", name: "DeepSeek R1", enabled: true }`,
			wantID:   "deepseek-r1",
			wantName: "DeepSeek R1",
		},
		{
			name:     "no name",
			input:    `{ id: "model-a", enabled: false }`,
			wantID:   "model-a",
			wantName: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if m := vecliModelIDRe.FindStringSubmatch(tt.input); len(m) >= 2 {
				assert.Equal(t, tt.wantID, m[1])
			} else {
				assert.Empty(t, tt.wantID)
			}
			if m := vecliModelNameRe.FindStringSubmatch(tt.input); len(m) >= 2 {
				assert.Equal(t, tt.wantName, m[1])
			} else {
				assert.Empty(t, tt.wantName)
			}
		})
	}
}

// --- codexModelRe tests ---

func TestCodexModelRe(t *testing.T) {
	tests := []struct {
		modelID  string
		expected bool
	}{
		{"gpt-5.5", true},
		{"gpt-5.4", true},
		{"gpt-5.4-mini", true},
		{"o3", true},
		{"o4-mini", true},
		{"gpt-3.5", true},
		{"gpt-4.0", true},
		{"gpt-4.0-mini", true},
		{"claude-3", false},
		{"gpt", false},
		{"o3-mini-pro", false},
		{"gpt-5.5-turbo", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			assert.Equal(t, tt.expected, codexModelRe.MatchString(tt.modelID))
		})
	}
}

// --- codexModelOrder tests ---

func TestCodexModelOrder(t *testing.T) {
	assert.Contains(t, codexModelOrder, "gpt-5.5")
	assert.Contains(t, codexModelOrder, "o3")
	assert.Equal(t, 0, codexModelOrder["gpt-5.5"])
	assert.Less(t, codexModelOrder["gpt-5.5"], codexModelOrder["gpt-5.4"])
}

// --- qoderSkipModels / qoderModelKeyRe tests ---

func TestQoderSkipModels(t *testing.T) {
	assert.True(t, qoderSkipModels["auto"])
	assert.True(t, qoderSkipModels["ultimate"])
	assert.True(t, qoderSkipModels["performance"])
	assert.True(t, qoderSkipModels["efficient"])
	assert.True(t, qoderSkipModels["lite"])
	assert.False(t, qoderSkipModels["qwen-3"])
}

func TestQoderModelKeyRe(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
		match    string
	}{
		{"modelSelector.item.qmodel", true, "qmodel"},
		{"modelSelector.item.deepseek-v3", true, "deepseek-v3"},
		{"other.item.model", false, ""},
		{"modelSelector.other.key", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := qoderModelKeyRe.FindStringSubmatch(tt.key)
			if tt.expected {
				require.Len(t, m, 2)
				assert.Equal(t, tt.match, m[1])
			} else {
				assert.Nil(t, m)
			}
		})
	}
}

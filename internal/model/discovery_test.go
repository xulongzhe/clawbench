package model_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// --- Test 1: BackendRegistry ---

func TestBackendRegistry_ContainsAllBackends(t *testing.T) {
	expectedIDs := []string{"claude", "codebuddy", "opencode", "gemini", "codex", "qoder", "vecli", "deepseek"}
	assert.Len(t, model.BackendRegistry, len(expectedIDs))

	seen := make(map[string]bool)
	for _, spec := range model.BackendRegistry {
		seen[spec.ID] = true
	}
	for _, id := range expectedIDs {
		assert.True(t, seen[id], "missing backend: %s", id)
	}
}

func TestBackendRegistry_FieldsPopulated(t *testing.T) {
	for _, spec := range model.BackendRegistry {
		assert.NotEmpty(t, spec.ID, "ID should not be empty")
		assert.NotEmpty(t, spec.Backend, "Backend should not be empty for %s", spec.ID)
		assert.NotEmpty(t, spec.DefaultCmd, "DefaultCmd should not be empty for %s", spec.ID)
		assert.NotEmpty(t, spec.Name, "Name should not be empty for %s", spec.ID)
		assert.NotEmpty(t, spec.Icon, "Icon should not be empty for %s", spec.ID)
		assert.NotEmpty(t, spec.Specialty, "Specialty should not be empty for %s", spec.ID)
	}
}

func TestBackendRegistry_SpecificValues(t *testing.T) {
	specs := make(map[string]model.BackendSpec)
	for _, s := range model.BackendRegistry {
		specs[s.ID] = s
	}

	assert.Equal(t, "claude", specs["claude"].DefaultCmd)
	assert.Equal(t, "codebuddy", specs["codebuddy"].DefaultCmd)
	assert.Equal(t, "opencode", specs["opencode"].DefaultCmd)
	assert.Equal(t, "gemini", specs["gemini"].DefaultCmd)
	assert.Equal(t, "codex", specs["codex"].DefaultCmd)
	assert.Equal(t, "qodercli", specs["qoder"].DefaultCmd)
	assert.Equal(t, "vecli", specs["vecli"].DefaultCmd)
	assert.Equal(t, "deepseek", specs["deepseek"].DefaultCmd)
}

// --- Test 2: generateAgentYAML ---

func TestGenerateAgentYAML_Format(t *testing.T) {
	spec := model.BackendSpec{
		ID:         "claude",
		Backend:    "claude",
		DefaultCmd: "claude",
		Name:       "Claude",
		Icon:       "🤖",
		Specialty:  "代码编写与推理",
	}

	data, err := model.GenerateAgentYAML(spec, nil)
	require.NoError(t, err)

	// Verify it's valid YAML and parses back to Agent struct
	var agent model.Agent
	err = yaml.Unmarshal(data, &agent)
	require.NoError(t, err)

	assert.Equal(t, "claude", agent.ID)
	assert.Equal(t, "Claude", agent.Name)
	assert.Equal(t, "🤖", agent.Icon)
	assert.Equal(t, "代码编写与推理", agent.Specialty)
	assert.Equal(t, "claude", agent.Backend)
	assert.Empty(t, agent.Models)
	assert.Empty(t, agent.SystemPrompt)
	assert.Empty(t, agent.Command)
}

func TestGenerateAgentYAML_ContainsRequiredFields(t *testing.T) {
	spec := model.BackendSpec{
		ID:         "test",
		Backend:    "test",
		DefaultCmd: "test",
		Name:       "Test",
		Icon:       "T",
		Specialty:  "Testing",
	}

	data, err := model.GenerateAgentYAML(spec, nil)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "id: test")
	assert.Contains(t, content, "name: Test")
	assert.Contains(t, content, "backend: test")
	assert.Contains(t, content, "models: []")
	assert.Contains(t, content, "system_prompt: \"\"")
}

// --- Test 3: checkCLIExists ---

func TestCheckCLIExists_ExistingCommand(t *testing.T) {
	// "ls" exists on all platforms
	assert.True(t, model.CheckCLIExists("ls"))
}

func TestCheckCLIExists_NonExistingCommand(t *testing.T) {
	assert.False(t, model.CheckCLIExists("definitely_not_a_real_command_xyz_12345"))
}

func TestCheckCLIExists_EmptyCommand(t *testing.T) {
	assert.False(t, model.CheckCLIExists(""))
}

// --- Test 4: DiscoverAgents ---

func TestDiscoverAgents_CreatesDirAndYAMLs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agents")
	// dir does not exist yet

	err := model.DiscoverAgents(dir)
	require.NoError(t, err)

	// Directory should now exist
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Each YAML file (if any) should be parseable as an Agent.
	// Note: in CI environments no AI CLIs may be installed, so
	// we cannot assert yamlCount > 0; we only validate structure.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)

		var agent model.Agent
		err = yaml.Unmarshal(data, &agent)
		require.NoError(t, err, "YAML file %s should be parseable", e.Name())
		assert.NotEmpty(t, agent.ID)
		assert.NotEmpty(t, agent.Backend)
	}
}

func TestDiscoverAgents_GeneratedYAMLsLoadable(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	dir := filepath.Join(t.TempDir(), "agents")
	require.NoError(t, os.MkdirAll(dir, 0755))

	// Pre-generate a known agent YAML so the test does not depend
	// on any AI CLI being installed on the system.
	spec := model.BackendSpec{
		ID:         "test-loadable",
		Backend:    "claude",
		DefaultCmd: "nonexistent_cli_for_test",
		Name:       "Test Loadable",
		Icon:       "🧪",
		Specialty:  "Testing",
	}
	data, err := model.GenerateAgentYAML(spec, nil)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test-loadable.yaml"), data, 0644))

	// LoadAgents should successfully load the generated YAMLs
	err = model.LoadAgents(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, model.AgentList)
}

func TestDiscoverAgents_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Create an existing agent YAML
	existingYAML := `id: my-custom-agent
name: My Custom Agent
icon: 🎯
specialty: Custom
backend: codebuddy
models: []
system_prompt: "I am custom"
`
	err := os.WriteFile(filepath.Join(agentsDir, "my-custom-agent.yaml"), []byte(existingYAML), 0644)
	require.NoError(t, err)

	err = model.DiscoverAgents(agentsDir)
	require.NoError(t, err)

	// Existing file should be preserved
	data, err := os.ReadFile(filepath.Join(agentsDir, "my-custom-agent.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "My Custom Agent")
}

// --- Test 5: Model list parsers ---

func TestParseCodebuddyModels_RealOutput(t *testing.T) {
	// Real output from: codebuddy --help | grep "Currently supported"
	output := `  --model <model>                                  Model for the current session. Please provide the model ID. Currently supported: (glm-4.7, glm-4.6, deepseek-v3-2-volc, deepseek-v3-0324, minimax-m2.5, minimax-m2.7, kimi-k2.5, glm-5.0, glm-5.1, glm-4.6v, deepseek-v3-1-lkeap, deepseek-v3-0324-lkeap, hunyuan-2.0-instruct)
  --text-to-image-model <model>                    Model for text-to-image generation`

	models := model.ParseCodebuddyModels(output)
	require.Len(t, models, 13, "should parse all 13 model IDs")

	assert.Equal(t, "glm-4.7", models[0].ID)
	assert.True(t, models[0].Default, "first model should be default")
	assert.Equal(t, "hunyuan-2.0-instruct", models[12].ID)
	assert.False(t, models[12].Default)

	// Name should equal ID for codebuddy models
	assert.Equal(t, models[0].ID, models[0].Name)
}

func TestParseCodebuddyModels_EmptyOutput(t *testing.T) {
	models := model.ParseCodebuddyModels("no models here")
	assert.Nil(t, models)
}

func TestParseCodebuddyModels_PartialOutput(t *testing.T) {
	output := `Currently supported: (glm-4.7, glm-4.6)`
	models := model.ParseCodebuddyModels(output)
	require.Len(t, models, 2)
	assert.Equal(t, "glm-4.7", models[0].ID)
	assert.True(t, models[0].Default)
	assert.Equal(t, "glm-4.6", models[1].ID)
	assert.False(t, models[1].Default)
}

func TestParseDeepSeekModels_RealOutput(t *testing.T) {
	// Real output from: deepseek models
	output := `Available models (default: deepseek-v4-pro)
  deepseek-v4-flash (deepseek)
* deepseek-v4-pro (deepseek)
  deepseek-ai/deepseek-v4-pro (nvidia-nim)
  deepseek-ai/deepseek-v4-flash (nvidia-nim)
  gpt-4.1 (openai)
  gpt-4.1-mini (openai)
  deepseek/deepseek-v4-pro (openrouter)
  deepseek/deepseek-v4-flash (openrouter)
  deepseek-coder:1.3b (ollama)
`

	models := model.ParseDeepSeekModels(output)
	require.Len(t, models, 2, "should only include deepseek provider models, not third-party")

	assert.Equal(t, "deepseek-v4-flash", models[0].ID)
	assert.False(t, models[0].Default, "flash is not the default")
	assert.Equal(t, "deepseek-v4-pro", models[1].ID)
	assert.True(t, models[1].Default, "pro is the default (marked with *)")
}

func TestParseDeepSeekModels_EmptyOutput(t *testing.T) {
	models := model.ParseDeepSeekModels("no models here")
	assert.Nil(t, models)
}

func TestParseDeepSeekModels_NoDefaultMarker(t *testing.T) {
	// If the header doesn't have a default and no * marker,
	// the first model should still be marked as default (fallback convention)
	output := `  deepseek-v4-flash (deepseek)
  deepseek-v4-pro (deepseek)
`
	models := model.ParseDeepSeekModels(output)
	require.Len(t, models, 2)
	assert.True(t, models[0].Default, "first model should be default as fallback")
	assert.False(t, models[1].Default)
}

func TestParseDeepSeekModels_DefaultFromHeader(t *testing.T) {
	// Default is in header but no * marker on any line
	output := `Available models (default: deepseek-v4-pro)
  deepseek-v4-flash (deepseek)
  deepseek-v4-pro (deepseek)
`
	models := model.ParseDeepSeekModels(output)
	require.Len(t, models, 2)
	assert.False(t, models[0].Default)
	assert.True(t, models[1].Default, "should match default from header")
}

func TestParseOpenCodeModels_RealOutput(t *testing.T) {
	// Real output from: opencode models (truncated for test)
	output := `opencode/minimax-m2.5-free
opencode/nemotron-3-super-free
minimax/MiniMax-M2.5
minimax/MiniMax-M2.7
anthropic/claude-sonnet-4-6
`

	models := model.ParseOpenCodeModels(output)
	require.Len(t, models, 5)

	// First model should be default
	assert.Equal(t, "opencode/minimax-m2.5-free", models[0].ID)
	assert.Equal(t, "minimax-m2.5-free", models[0].Name, "Name should be model part after /")
	assert.True(t, models[0].Default, "first model should be default")

	// Provider/model format
	assert.Equal(t, "minimax/MiniMax-M2.5", models[2].ID)
	assert.Equal(t, "MiniMax-M2.5", models[2].Name)

	assert.Equal(t, "anthropic/claude-sonnet-4-6", models[4].ID)
	assert.Equal(t, "claude-sonnet-4-6", models[4].Name)
}

func TestParseOpenCodeModels_EmptyOutput(t *testing.T) {
	models := model.ParseOpenCodeModels("")
	assert.Nil(t, models)
}

func TestParseOpenCodeModels_InvalidLines(t *testing.T) {
	output := `minimax/MiniMax-M2.5
not-a-valid-line
anthropic/claude-sonnet-4-6

`
	models := model.ParseOpenCodeModels(output)
	require.Len(t, models, 2)
	assert.Equal(t, "minimax/MiniMax-M2.5", models[0].ID)
	assert.Equal(t, "anthropic/claude-sonnet-4-6", models[1].ID)
}

func TestParseOpenCodeModels_SingleModel(t *testing.T) {
	output := `opencode/minimax-m2.5-free`
	models := model.ParseOpenCodeModels(output)
	require.Len(t, models, 1)
	assert.Equal(t, "opencode/minimax-m2.5-free", models[0].ID)
	assert.True(t, models[0].Default)
}

// --- Test 6: BackendRegistry model discovery config ---

func TestBackendRegistry_ModelDiscoveryConfig(t *testing.T) {
	specs := make(map[string]model.BackendSpec)
	for _, s := range model.BackendRegistry {
		specs[s.ID] = s
	}

	// codebuddy should have model discovery
	assert.NotEmpty(t, specs["codebuddy"].ListModelsCmd, "codebuddy should have ListModelsCmd")
	assert.NotNil(t, specs["codebuddy"].ParseModels, "codebuddy should have ParseModels")

	// opencode should have model discovery
	assert.NotEmpty(t, specs["opencode"].ListModelsCmd, "opencode should have ListModelsCmd")
	assert.NotNil(t, specs["opencode"].ParseModels, "opencode should have ParseModels")

	// deepseek should have model discovery
	assert.NotEmpty(t, specs["deepseek"].ListModelsCmd, "deepseek should have ListModelsCmd")
	assert.NotNil(t, specs["deepseek"].ParseModels, "deepseek should have ParseModels")

	// claude, gemini, codex, qoder, vecli should NOT have model discovery
	for _, id := range []string{"claude", "gemini", "codex", "qoder", "vecli"} {
		assert.Empty(t, specs[id].ListModelsCmd, "%s should not have ListModelsCmd", id)
		assert.Nil(t, specs[id].ParseModels, "%s should not have ParseModels", id)
	}
}

// --- Test 7: DiscoverModels ---

func TestDiscoverModels_NoSupport(t *testing.T) {
	spec := model.BackendSpec{
		ID:         "claude",
		DefaultCmd: "claude",
	}
	models := model.DiscoverModels(spec)
	assert.Nil(t, models, "should return nil when no model discovery support")
}

func TestDiscoverModels_NonexistentCLI(t *testing.T) {
	spec := model.BackendSpec{
		ID:            "test",
		DefaultCmd:    "definitely_not_a_real_command_xyz_12345",
		ListModelsCmd: []string{"models"},
		ParseModels:   model.ParseOpenCodeModels,
	}
	models := model.DiscoverModels(spec)
	assert.Nil(t, models, "should return nil when CLI doesn't exist")
}

func TestDiscoverModels_WithRealCLI(t *testing.T) {
	// This test uses opencode if available; skip if not installed
	if !model.CheckCLIExists("opencode") {
		t.Skip("opencode not installed, skipping integration test")
	}

	spec := model.BackendSpec{
		ID:            "opencode",
		DefaultCmd:    "opencode",
		ListModelsCmd: []string{"models"},
		ParseModels:   model.ParseOpenCodeModels,
	}
	models := model.DiscoverModels(spec)
	assert.NotEmpty(t, models, "opencode should return at least one model")

	// First model should be default
	assert.True(t, models[0].Default, "first model should be default")
	// All models should have non-empty IDs
	for _, m := range models {
		assert.NotEmpty(t, m.ID)
		assert.NotEmpty(t, m.Name)
	}
}

func TestGenerateAgentYAML_WithNilModels(t *testing.T) {
	spec := model.BackendSpec{
		ID:         "test-nil-models",
		Backend:    "test",
		DefaultCmd: "nonexistent",
		Name:       "Test",
		Icon:       "T",
		Specialty:  "Testing",
	}

	data, err := model.GenerateAgentYAML(spec, nil)
	require.NoError(t, err)

	var agent model.Agent
	err = yaml.Unmarshal(data, &agent)
	require.NoError(t, err)
	assert.Empty(t, agent.Models, "nil models should result in empty model list")
}

func TestGenerateAgentYAML_WithModels(t *testing.T) {
	spec := model.BackendSpec{
		ID:         "test-with-models",
		Backend:    "test",
		DefaultCmd: "nonexistent",
		Name:       "Test",
		Icon:       "T",
		Specialty:  "Testing",
	}
	models := []model.AgentModel{
		{ID: "model-a", Name: "Model A", Default: true},
		{ID: "model-b", Name: "Model B", Default: false},
	}

	data, err := model.GenerateAgentYAML(spec, models)
	require.NoError(t, err)

	var agent model.Agent
	err = yaml.Unmarshal(data, &agent)
	require.NoError(t, err)
	require.Len(t, agent.Models, 2)
	assert.Equal(t, "model-a", agent.Models[0].ID)
	assert.True(t, agent.Models[0].Default)
	assert.Equal(t, "model-b", agent.Models[1].ID)
	assert.False(t, agent.Models[1].Default)
}

func TestDiscoverModels_WithEchoCLI(t *testing.T) {
	// Test the full DiscoverModels flow using "echo" as a CLI that always exists.
	spec := model.BackendSpec{
		ID:            "mock-agent",
		Backend:       "mock",
		DefaultCmd:    "echo", // always available, will succeed
		Name:          "Mock",
		Icon:          "🧪",
		Specialty:     "Testing",
		ListModelsCmd: []string{"model-a, model-b"}, // echo will output this
		ParseModels: func(s string) []model.AgentModel {
			return []model.AgentModel{
				{ID: "mock-a", Name: "Mock A", Default: true},
				{ID: "mock-b", Name: "Mock B", Default: false},
			}
		},
	}

	models := model.DiscoverModels(spec)
	require.Len(t, models, 2)
	assert.Equal(t, "mock-a", models[0].ID)
	assert.True(t, models[0].Default)
	assert.Equal(t, "mock-b", models[1].ID)
	assert.False(t, models[1].Default)
}

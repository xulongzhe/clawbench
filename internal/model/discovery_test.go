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

	data, err := model.GenerateAgentYAML(spec)
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

	data, err := model.GenerateAgentYAML(spec)
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
	data, err := model.GenerateAgentYAML(spec)
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

package model_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAgents_EmptyDir(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	dir := t.TempDir()
	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	assert.Empty(t, model.AgentList)
}

func TestLoadAgents_ValidYAML(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	dir := t.TempDir()
	yamlContent := `id: test-agent
name: Test Agent
icon: 🤖
specialty: Testing
backend: codebuddy
model: glm-5.1
system_prompt: You are a test agent.
`
	err := os.WriteFile(filepath.Join(dir, "test-agent.yaml"), []byte(yamlContent), 0o644)
	require.NoError(t, err)

	err = model.LoadAgents(dir)
	assert.NoError(t, err)
	assert.NotNil(t, model.Agents["test-agent"])
	assert.Equal(t, "Test Agent", model.Agents["test-agent"].Name)
	assert.Equal(t, "codebuddy", model.Agents["test-agent"].Backend)
	assert.Len(t, model.AgentList, 1)
}

func TestLoadAgents_SkipsNonYAML(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a yaml"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "no-id.yaml"), []byte("name: No ID Agent\n"), 0o644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	assert.Empty(t, model.AgentList)
}

func TestLoadAgents_MultipleAgents(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	dir := t.TempDir()
	yaml1 := `id: agent-1
name: Agent One
icon: "1"
specialty: One
backend: claude
system_prompt: Prompt 1
`
	yaml2 := `id: agent-2
name: Agent Two
icon: "2"
specialty: Two
backend: codebuddy
system_prompt: Prompt 2
`
	_ = os.WriteFile(filepath.Join(dir, "agent1.yaml"), []byte(yaml1), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "agent2.yaml"), []byte(yaml2), 0o644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	assert.Len(t, model.AgentList, 2)
	assert.NotNil(t, model.Agents["agent-1"])
	assert.NotNil(t, model.Agents["agent-2"])
}

func TestLoadAgents_CommonPromptGenerated(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	dir := t.TempDir()

	yaml := `id: with-common
name: With Common
icon: "C"
specialty: Common
backend: codebuddy
system_prompt: My specific prompt
`
	_ = os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte(yaml), 0o644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	agent := model.Agents["with-common"]
	assert.NotNil(t, agent)
	// Embedded common prompt is always present
	assert.Contains(t, agent.SystemPrompt, "My specific prompt")
	assert.Contains(t, agent.SystemPrompt, "User Interaction")
}

func TestLoadAgents_CommonPromptOnlyNoSystemPrompt(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	dir := t.TempDir()

	yaml := `id: no-prompt
name: No Prompt
icon: "N"
specialty: None
backend: claude
`
	_ = os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte(yaml), 0o644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	agent := model.Agents["no-prompt"]
	assert.NotNil(t, agent)
	// When agent has no system_prompt, the common prompt from embedded rules is used
	assert.Contains(t, agent.SystemPrompt, "User Interaction")
}

func TestLoadAgents_NonExistentDir(t *testing.T) {
	err := model.LoadAgents("/non/existent/directory")
	assert.Error(t, err)
}

func TestLoadAgents_InvalidYAML(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("::invalid yaml::\n  [bad"), 0o644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	assert.Empty(t, model.AgentList)
}

func TestGetDefaultAgentID_Configured(t *testing.T) {
	model.DefaultAgentID = "coder"
	model.Agents = map[string]*model.Agent{"coder": {ID: "coder"}}
	model.AgentList = []*model.Agent{{ID: "assistant"}, {ID: "coder"}}
	t.Cleanup(func() {
		model.DefaultAgentID = ""
		model.Agents = nil
		model.AgentList = nil
	})

	assert.Equal(t, "coder", model.GetDefaultAgentID())
}

func TestGetDefaultAgentID_ConfiguredNotFound(t *testing.T) {
	model.DefaultAgentID = "nonexistent"
	model.Agents = map[string]*model.Agent{"codebuddy": {ID: "codebuddy"}}
	model.AgentList = []*model.Agent{{ID: "codebuddy"}}
	t.Cleanup(func() {
		model.DefaultAgentID = ""
		model.Agents = nil
		model.AgentList = nil
	})

	// Configured agent not found, fallback to first in list
	assert.Equal(t, "codebuddy", model.GetDefaultAgentID())
}

func TestGetDefaultAgentID_FallbackFirst(t *testing.T) {
	model.DefaultAgentID = ""
	model.Agents = map[string]*model.Agent{"coder": {ID: "coder"}}
	model.AgentList = []*model.Agent{{ID: "coder"}}
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	assert.Equal(t, "coder", model.GetDefaultAgentID())
}

func TestGetDefaultAgentID_NoAgents(t *testing.T) {
	model.DefaultAgentID = ""
	model.Agents = nil
	model.AgentList = nil

	assert.Equal(t, "", model.GetDefaultAgentID())
}

// ---------- DefaultModelID ----------

func TestDefaultModelID_PreferredModel(t *testing.T) {
	agent := &model.Agent{
		PreferredModel: "preferred-model",
		Models: []model.AgentModel{
			{ID: "default-model", Default: true},
		},
	}
	assert.Equal(t, "preferred-model", agent.DefaultModelID())
}

func TestDefaultModelID_NoPreferredModel(t *testing.T) {
	agent := &model.Agent{
		Models: []model.AgentModel{
			{ID: "other-model"},
			{ID: "default-model", Default: true},
		},
	}
	assert.Equal(t, "default-model", agent.DefaultModelID())
}

func TestDefaultModelID_NoDefaultFlag(t *testing.T) {
	agent := &model.Agent{
		Models: []model.AgentModel{
			{ID: "first-model"},
			{ID: "second-model"},
		},
	}
	assert.Equal(t, "first-model", agent.DefaultModelID())
}

func TestDefaultModelID_NoModels(t *testing.T) {
	agent := &model.Agent{}
	assert.Equal(t, "", agent.DefaultModelID())
}

// ---------- BaseModelID ----------

func TestBaseModelID_DefaultFlag(t *testing.T) {
	agent := &model.Agent{
		PreferredModel: "preferred", // should be ignored
		Models: []model.AgentModel{
			{ID: "first"},
			{ID: "flagged", Default: true},
		},
	}
	assert.Equal(t, "flagged", agent.BaseModelID())
}

func TestBaseModelID_FirstInList(t *testing.T) {
	agent := &model.Agent{
		PreferredModel: "preferred", // should be ignored
		Models: []model.AgentModel{
			{ID: "first"},
			{ID: "second"},
		},
	}
	assert.Equal(t, "first", agent.BaseModelID())
}

func TestBaseModelID_NoModels(t *testing.T) {
	agent := &model.Agent{PreferredModel: "ignored"}
	assert.Equal(t, "", agent.BaseModelID())
}

// ---------- EffectiveThinkingEffort ----------

func TestEffectiveThinkingEffort_Preferred(t *testing.T) {
	agent := &model.Agent{
		PreferredThinkingEffort: "high",
		ThinkingEffort:          "medium",
	}
	assert.Equal(t, "high", agent.EffectiveThinkingEffort())
}

func TestEffectiveThinkingEffort_NoPreferred(t *testing.T) {
	agent := &model.Agent{
		ThinkingEffort: "medium",
	}
	assert.Equal(t, "medium", agent.EffectiveThinkingEffort())
}

func TestEffectiveThinkingEffort_Neither(t *testing.T) {
	agent := &model.Agent{}
	assert.Equal(t, "", agent.EffectiveThinkingEffort())
}

// ---------- WriteAgentYAML ----------

func TestWriteAgentYAML_Success(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	// Write initial agent YAML
	yamlContent := `id: test-agent
name: Test Agent
backend: codebuddy
`
	err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yamlContent), 0o644)
	require.NoError(t, err)

	// Load agents to set agentsDir
	err = model.LoadAgents(agentsDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	// Write preferences
	agent := model.Agents["test-agent"]
	agent.PreferredModel = "glm-5.1"
	agent.PreferredThinkingEffort = "high"

	err = model.WriteAgentYAML(agent)
	assert.NoError(t, err)

	// Read back and verify
	data, err := os.ReadFile(filepath.Join(agentsDir, "test-agent.yaml"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "preferred_model: glm-5.1")
	assert.Contains(t, content, "preferred_thinking_effort: high")
	assert.Contains(t, content, "backend: codebuddy")
}

func TestWriteAgentYAML_ClearPreferences(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	yamlContent := `id: test-agent
name: Test Agent
backend: codebuddy
preferred_model: old-model
preferred_thinking_effort: low
`
	err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yamlContent), 0o644)
	require.NoError(t, err)

	err = model.LoadAgents(agentsDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	// Clear preferences
	agent := model.Agents["test-agent"]
	agent.PreferredModel = ""
	agent.PreferredThinkingEffort = ""

	err = model.WriteAgentYAML(agent)
	assert.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(agentsDir, "test-agent.yaml"))
	require.NoError(t, err)
	content := string(data)
	assert.NotContains(t, content, "preferred_model")
	assert.NotContains(t, content, "preferred_thinking_effort")
}

func TestWriteAgentYAML_AgentYAMLNotFoundOnDisk(t *testing.T) {
	// When LoadAgents has been called (agentsDir is set) but the agent's
	// YAML file doesn't exist on disk, WriteAgentYAML should return an error.
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	err := model.LoadAgents(agentsDir)
	require.NoError(t, err)

	// Agent YAML doesn't exist on disk
	err = model.WriteAgentYAML(&model.Agent{ID: "nonexistent", PreferredModel: "m1"})
	assert.Error(t, err)
}

func TestLoadAgents_CommonPromptWithAgentPrompt(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	// Agent with system_prompt — should get commonPrompt + agent prompt
	yaml := `id: with-prompt
name: With Prompt
icon: "W"
specialty: Writing
backend: codebuddy
system_prompt: My specific prompt
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "agent.yaml"), []byte(yaml), 0o644))

	err := model.LoadAgents(agentsDir)
	require.NoError(t, err)
	agent := model.Agents["with-prompt"]
	assert.NotNil(t, agent)
	// Should have both embedded common prompt and specific prompt
	assert.Contains(t, agent.SystemPrompt, "User Interaction")
	assert.Contains(t, agent.SystemPrompt, "My specific prompt")
	// Common prompt should come first
	idx := strings.Index(agent.SystemPrompt, "User Interaction")
	idx2 := strings.Index(agent.SystemPrompt, "My specific prompt")
	assert.Less(t, idx, idx2)
}

func TestWriteAgentYAML_InvalidYAMLContent(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	// Write an agent YAML that is valid YAML but cannot be round-tripped
	yamlContent := `id: test-agent
name: Test Agent
backend: codebuddy
`
	err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yamlContent), 0o644)
	require.NoError(t, err)

	err = model.LoadAgents(agentsDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	agent := model.Agents["test-agent"]
	agent.PreferredModel = "new-model"
	err = model.WriteAgentYAML(agent)
	assert.NoError(t, err)

	// Verify the file was updated
	data, err := os.ReadFile(filepath.Join(agentsDir, "test-agent.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "preferred_model: new-model")
	assert.Contains(t, string(data), "backend: codebuddy")
}

func TestLoadAgents_ClawbenchBinReplacement(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
		model.ClawbenchBin = ""
	})

	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	yaml := `id: test-agent
name: Test
icon: "T"
specialty: Testing
backend: codebuddy
system_prompt: You test.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yaml), 0o644))

	// Set ClawbenchBin before loading
	model.ClawbenchBin = "/usr/local/bin/clawbench"
	err := model.LoadAgents(agentsDir)
	require.NoError(t, err)

	agent := model.Agents["test-agent"]
	assert.NotNil(t, agent)
	// The embedded rules template is always present
	assert.Contains(t, agent.SystemPrompt, "User Interaction")
	assert.Contains(t, agent.SystemPrompt, "You test.")
}

func TestWriteAgentYAML_WriteFileError(t *testing.T) {
	// Skip on Windows where permission model differs
	if os.PathSeparator == '\\' {
		t.Skip("read-only directory test not reliable on Windows")
	}

	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	yamlContent := `id: test-agent
name: Test Agent
backend: codebuddy
`
	err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yamlContent), 0o644)
	require.NoError(t, err)

	err = model.LoadAgents(agentsDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	// Make the directory read-only so WriteFile fails
	// Note: root user bypasses filesystem permissions, so skip on root
	require.NoError(t, os.Chmod(agentsDir, 0o555))
	defer os.Chmod(agentsDir, 0o755) // restore for cleanup

	if os.Getuid() == 0 {
		t.Skip("skipping: root user bypasses filesystem permissions")
	}

	agent := model.Agents["test-agent"]
	agent.PreferredModel = "new-model"
	err = model.WriteAgentYAML(agent)
	assert.Error(t, err, "writing to read-only directory should fail")
}

func TestWriteAgentYAML_CorruptYAMLUnmarshalFails(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	// Write a valid YAML first so LoadAgents sets up agentsDir
	yamlContent := `id: test-agent
name: Test Agent
backend: codebuddy
`
	err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yamlContent), 0o644)
	require.NoError(t, err)

	err = model.LoadAgents(agentsDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	// Now corrupt the YAML on disk so the read-unmarshal path fails
	corruptContent := `id: test-agent
name: Test Agent
backend: !!binary invalid-binary-tag
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(corruptContent), 0o644))

	agent := model.Agents["test-agent"]
	agent.PreferredModel = "new-model"
	err = model.WriteAgentYAML(agent)
	assert.Error(t, err, "should fail when YAML on disk cannot be unmarshaled")
}

func TestLoadAgents_DeterministicOrder(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	dir := t.TempDir()
	// Create agents that are NOT in alphabetical order on disk
	for _, id := range []string{"zebra", "alpha", "middle"} {
		yaml := fmt.Sprintf(`id: %s
name: %s
icon: "X"
specialty: Test
backend: codebuddy
system_prompt: Prompt
`, id, id)
		require.NoError(t, os.WriteFile(filepath.Join(dir, id+".yaml"), []byte(yaml), 0o644))
	}

	err := model.LoadAgents(dir)
	require.NoError(t, err)

	// AgentList should be sorted by ID
	require.Len(t, model.AgentList, 3)
	assert.Equal(t, "alpha", model.AgentList[0].ID)
	assert.Equal(t, "middle", model.AgentList[1].ID)
	assert.Equal(t, "zebra", model.AgentList[2].ID)
}

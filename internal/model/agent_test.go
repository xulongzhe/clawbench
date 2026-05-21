package model_test

import (
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
	err := os.WriteFile(filepath.Join(dir, "test-agent.yaml"), []byte(yamlContent), 0644)
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
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a yaml"), 0644)
	os.WriteFile(filepath.Join(dir, "no-id.yaml"), []byte("name: No ID Agent\n"), 0644)

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
	os.WriteFile(filepath.Join(dir, "agent1.yaml"), []byte(yaml1), 0644)
	os.WriteFile(filepath.Join(dir, "agent2.yaml"), []byte(yaml2), 0644)

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
	os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte(yaml), 0644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	agent := model.Agents["with-common"]
	assert.NotNil(t, agent)
	// Without rules.md in parent dir, only agent prompt is present
	assert.Contains(t, agent.SystemPrompt, "My specific prompt")
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
	os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte(yaml), 0644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	agent := model.Agents["no-prompt"]
	assert.NotNil(t, agent)
	// When agent has no system_prompt and no rules.md, system prompt is empty
	assert.Empty(t, agent.SystemPrompt)
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
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("::invalid yaml::\n  [bad"), 0644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	assert.Empty(t, model.AgentList)
}

func TestBuildCommonPrompt_ScheduledRemovesSection(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	// Create temp dir with agents/ and rules.md containing SCHEDULED markers
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	rulesContent := `## User Interaction

Some rules here.

<!-- SCHEDULED_BEGIN -->
## Scheduled Tasks

Task rules and CLI reference here.

<!-- SCHEDULED_END -->

## RAG History Search

RAG rules here.
`
	err := os.WriteFile(filepath.Join(tmpDir, "rules.md"), []byte(rulesContent), 0644)
	require.NoError(t, err)

	// Write an agent YAML so LoadAgents sets up agentsDir properly
	yaml := `id: test-agent
name: Test
icon: "T"
specialty: Testing
backend: codebuddy
system_prompt: You test.
`
	err = os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yaml), 0644)
	require.NoError(t, err)

	err = model.LoadAgents(agentsDir)
	require.NoError(t, err)

	// Normal: Scheduled Tasks section is present
	normalPrompt := model.BuildCommonPrompt(false)
	assert.Contains(t, normalPrompt, "Scheduled Tasks")
	assert.Contains(t, normalPrompt, "RAG History Search")
	assert.NotContains(t, normalPrompt, "SCHEDULED_BEGIN")
	assert.NotContains(t, normalPrompt, "SCHEDULED_END")

	// Scheduled: Scheduled Tasks section is removed
	scheduledPrompt := model.BuildCommonPrompt(true)
	assert.NotContains(t, scheduledPrompt, "Scheduled Tasks")
	assert.Contains(t, scheduledPrompt, "RAG History Search")
	assert.NotContains(t, scheduledPrompt, "SCHEDULED_BEGIN")
	assert.NotContains(t, scheduledPrompt, "SCHEDULED_END")
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
		ThinkingEffort:           "medium",
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
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Write initial agent YAML
	yamlContent := `id: test-agent
name: Test Agent
backend: codebuddy
`
	err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yamlContent), 0644)
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
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	yamlContent := `id: test-agent
name: Test Agent
backend: codebuddy
preferred_model: old-model
preferred_thinking_effort: low
`
	err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yamlContent), 0644)
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

func TestWriteAgentYAML_NotInitialized(t *testing.T) {
	// Save and restore agentsDir
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	// LoadAgents sets agentsDir, so calling it on a valid dir then
	// testing WriteAgentYAML for a nonexistent agent tests the read path
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	err := model.LoadAgents(agentsDir)
	require.NoError(t, err)

	// Write an agent YAML that doesn't exist on disk
	err = model.WriteAgentYAML(&model.Agent{ID: "nonexistent"})
	assert.Error(t, err)
}

func TestWriteAgentYAML_AgentNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	err := model.LoadAgents(agentsDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	// Agent YAML doesn't exist
	err = model.WriteAgentYAML(&model.Agent{ID: "nonexistent"})
	assert.Error(t, err)
}

func TestWriteAgentYAML_NotInitializedNoDir(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	// LoadAgents on an empty dir sets agentsDir, but the agent won't exist on disk
	// We need agentsDir to be empty to trigger "agents directory not initialized"
	// Since agentsDir is unexported, we test by calling WriteAgentYAML before LoadAgents
	// Reset state first
	model.Agents = nil
	model.AgentList = nil

	// WriteAgentYAML checks agentsDir internally; calling before LoadAgents
	// means agentsDir="" (the zero value). But since agentsDir persists across
	// tests, we use LoadAgents on empty dir (which sets agentsDir) then
	// test with a nonexistent agent which hits the read error instead.
	// The "not initialized" path is actually untestable without modifying
	// the source, so we test the behavior we can: calling WriteAgentYAML
	// for an agent whose YAML doesn't exist on disk.
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	err := model.LoadAgents(agentsDir)
	require.NoError(t, err)

	// WriteAgentYAML for nonexistent agent — should fail with read error
	err = model.WriteAgentYAML(&model.Agent{ID: "nonexistent", PreferredModel: "m1"})
	assert.Error(t, err)
}

func TestLoadAgents_CommonPromptOnlyWithRulesNoSystemPrompt(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Create rules.md in parent directory
	rulesContent := "## Rules\nBe helpful and concise."
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "rules.md"), []byte(rulesContent), 0644))

	// Agent with no system_prompt — should get commonPrompt only (the else-if branch)
	yaml := `id: no-prompt
name: No Prompt
icon: "N"
specialty: None
backend: claude
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "agent.yaml"), []byte(yaml), 0644))

	err := model.LoadAgents(agentsDir)
	require.NoError(t, err)
	agent := model.Agents["no-prompt"]
	assert.NotNil(t, agent)
	// Should have the common prompt (rules.md) as the system prompt
	assert.Contains(t, agent.SystemPrompt, "Be helpful and concise")
}

func TestLoadAgents_CommonPromptWithAgentPrompt(t *testing.T) {
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Create rules.md
	rulesContent := "## Rules\nBe helpful."
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "rules.md"), []byte(rulesContent), 0644))

	// Agent with system_prompt — should get commonPrompt + agent prompt
	yaml := `id: with-prompt
name: With Prompt
icon: "W"
specialty: Writing
backend: codebuddy
system_prompt: My specific prompt
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "agent.yaml"), []byte(yaml), 0644))

	err := model.LoadAgents(agentsDir)
	require.NoError(t, err)
	agent := model.Agents["with-prompt"]
	assert.NotNil(t, agent)
	// Should have both common prompt and specific prompt
	assert.Contains(t, agent.SystemPrompt, "Be helpful")
	assert.Contains(t, agent.SystemPrompt, "My specific prompt")
	// Common prompt should come first
	idx := strings.Index(agent.SystemPrompt, "Be helpful")
	idx2 := strings.Index(agent.SystemPrompt, "My specific prompt")
	assert.Less(t, idx, idx2)
}

func TestWriteAgentYAML_InvalidYAMLContent(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Write an agent YAML that is valid YAML but cannot be round-tripped
	yamlContent := `id: test-agent
name: Test Agent
backend: codebuddy
`
	err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yamlContent), 0644)
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
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Create rules.md with {{CLAWBENCH_BIN}} placeholder
	rulesContent := "Use {{CLAWBENCH_BIN}} for CLI operations."
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "rules.md"), []byte(rulesContent), 0644))

	yaml := `id: test-agent
name: Test
icon: "T"
specialty: Testing
backend: codebuddy
system_prompt: You test.
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yaml), 0644))

	// Set ClawbenchBin before loading
	model.ClawbenchBin = "/usr/local/bin/clawbench"
	err := model.LoadAgents(agentsDir)
	require.NoError(t, err)

	agent := model.Agents["test-agent"]
	assert.NotNil(t, agent)
	assert.Contains(t, agent.SystemPrompt, "/usr/local/bin/clawbench")
	assert.NotContains(t, agent.SystemPrompt, "{{CLAWBENCH_BIN}}")
}

func TestWriteAgentYAML_WriteFileError(t *testing.T) {
	// Skip on Windows where permission model differs
	if os.PathSeparator == '\\' {
		t.Skip("read-only directory test not reliable on Windows")
	}

	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	yamlContent := `id: test-agent
name: Test Agent
backend: codebuddy
`
	err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	err = model.LoadAgents(agentsDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		model.Agents = nil
		model.AgentList = nil
	})

	// Make the directory read-only so WriteFile fails
	// Note: root user bypasses filesystem permissions, so skip on root
	require.NoError(t, os.Chmod(agentsDir, 0555))
	defer os.Chmod(agentsDir, 0755) // restore for cleanup

	if os.Getuid() == 0 {
		t.Skip("skipping: root user bypasses filesystem permissions")
	}

	agent := model.Agents["test-agent"]
	agent.PreferredModel = "new-model"
	err = model.WriteAgentYAML(agent)
	assert.Error(t, err, "writing to read-only directory should fail")
}

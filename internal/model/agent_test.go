package model_test

import (
	"os"
	"path/filepath"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAgents_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	assert.Empty(t, model.AgentList)
}

func TestLoadAgents_ValidYAML(t *testing.T) {
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
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a yaml"), 0644)
	os.WriteFile(filepath.Join(dir, "no-id.yaml"), []byte("name: No ID Agent\n"), 0644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	assert.Empty(t, model.AgentList)
}

func TestLoadAgents_MultipleAgents(t *testing.T) {
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

func TestLoadAgents_CommonPrompt(t *testing.T) {
	dir := t.TempDir()
	commonPrompt := "This is the common prompt for all agents."
	os.WriteFile(filepath.Join(dir, "common_prompt.md"), []byte(commonPrompt), 0644)

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
	assert.Contains(t, agent.SystemPrompt, "This is the common prompt for all agents.")
	assert.Contains(t, agent.SystemPrompt, "My specific prompt")
}

func TestLoadAgents_CommonPromptOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "common_prompt.md"), []byte("Common only"), 0644)
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
	assert.Equal(t, "Common only", agent.SystemPrompt)
}

func TestLoadAgents_AvailableAgentsPlaceholder(t *testing.T) {
	dir := t.TempDir()
	yaml1 := "id: assistant\nname: Assistant\nicon: \"A\"\nspecialty: General\nbackend: codebuddy\nsystem_prompt: \"You are assistant. Available agents: {{AVAILABLE_AGENTS}}\"\n"
	yaml2 := `id: coder
name: Coder
icon: "C"
specialty: Code
backend: claude
system_prompt: You are coder.
`
	os.WriteFile(filepath.Join(dir, "assistant.yaml"), []byte(yaml1), 0644)
	os.WriteFile(filepath.Join(dir, "coder.yaml"), []byte(yaml2), 0644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	agent := model.Agents["assistant"]
	require.NotNil(t, agent)
	assert.NotContains(t, agent.SystemPrompt, "{{AVAILABLE_AGENTS}}")
	assert.Contains(t, agent.SystemPrompt, "coder")
}

func TestLoadAgents_NonExistentDir(t *testing.T) {
	err := model.LoadAgents("/non/existent/directory")
	assert.Error(t, err)
}

func TestLoadAgents_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("::invalid yaml::\n  [bad"), 0644)

	err := model.LoadAgents(dir)
	assert.NoError(t, err)
	assert.Empty(t, model.AgentList)
}

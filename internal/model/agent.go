package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Agent represents an AI agent with its own system prompt, backend, and model.
type Agent struct {
	ID           string `yaml:"id" json:"id"`
	Name         string `yaml:"name" json:"name"`
	Icon         string `yaml:"icon" json:"icon"`
	Specialty    string `yaml:"specialty" json:"specialty"`
	Backend      string `yaml:"backend" json:"backend"`
	Model        string `yaml:"model" json:"model"`
	Command      string `yaml:"command" json:"command"`           // optional: custom command path for the AI backend CLI
	SystemPrompt string `yaml:"system_prompt" json:"systemPrompt"`
}

var (
	Agents    map[string]*Agent // indexed by ID
	AgentList []*Agent          // ordered list for API responses
)

// GetDefaultAgentID returns the default agent ID for new sessions.
// Priority: configured DefaultAgentID > first agent in AgentList > empty string.
func GetDefaultAgentID() string {
	if DefaultAgentID != "" {
		if _, ok := Agents[DefaultAgentID]; ok {
			return DefaultAgentID
		}
	}
	if len(AgentList) > 0 {
		return AgentList[0].ID
	}
	return ""
}

// LoadAgents reads all YAML files from the given directory and registers them as agents.
// If no agents are found, a default agent is created from existing global config.
// It also loads the common prompt from common_prompt.md and prepends it to each agent's system prompt.
func LoadAgents(dir string) error {
	Agents = make(map[string]*Agent)
	AgentList = nil

	// Load common prompt shared by all agents
	commonPrompt := loadCommonPrompt(dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var agent Agent
		if err := yaml.Unmarshal(data, &agent); err != nil {
			continue
		}
		if agent.ID == "" {
			continue
		}

		// Prepend common prompt to agent's role-specific system prompt
		if commonPrompt != "" && agent.SystemPrompt != "" {
			agent.SystemPrompt = commonPrompt + "\n\n" + agent.SystemPrompt
		} else if commonPrompt != "" {
			agent.SystemPrompt = commonPrompt
		}

		Agents[agent.ID] = &agent
		AgentList = append(AgentList, &agent)
	}

	// Build available agent list for {{AVAILABLE_AGENTS}} placeholder
	var agentLines []string
	for _, a := range AgentList {
	if a.ID == DefaultAgentID {
		continue
	}
		agentLines = append(agentLines, fmt.Sprintf("    - %s：%s", a.ID, a.Specialty))
	}
	agentListReplacement := strings.Join(agentLines, "\n")

	// Inject available agent list into all agents' system prompts
	for _, agent := range Agents {
		if strings.Contains(agent.SystemPrompt, "{{AVAILABLE_AGENTS}}") {
			agent.SystemPrompt = strings.Replace(agent.SystemPrompt, "{{AVAILABLE_AGENTS}}", agentListReplacement, 1)
		}
	}

	return nil
}

// loadCommonPrompt reads the common prompt file from the agents directory.
// Returns empty string if the file does not exist or cannot be read.
func loadCommonPrompt(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "common_prompt.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

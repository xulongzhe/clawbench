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
	SystemPrompt string `yaml:"system_prompt" json:"systemPrompt"`
}

var (
	Agents    map[string]*Agent // indexed by ID
	AgentList []*Agent          // ordered list for API responses
)

// LoadAgents reads all YAML files from the given directory and registers them as agents.
// If no agents are found, a default agent is created from existing global config.
func LoadAgents(dir string) error {
	Agents = make(map[string]*Agent)
	AgentList = nil

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
		Agents[agent.ID] = &agent
		AgentList = append(AgentList, &agent)
	}

	// Inject available agent list into assistant's system prompt
	if assistant, ok := Agents["assistant"]; ok {
		var agentLines []string
		for _, a := range AgentList {
			if a.ID == "assistant" {
				continue
			}
			agentLines = append(agentLines, fmt.Sprintf("    - %s：%s", a.ID, a.Specialty))
		}
		replacement := strings.Join(agentLines, "\n")
		assistant.SystemPrompt = strings.Replace(assistant.SystemPrompt, "{{AVAILABLE_AGENTS}}", replacement, 1)
	}

	return nil
}

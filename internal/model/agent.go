package model

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentModel represents a model option for an agent.
type AgentModel struct {
	ID      string `yaml:"id" json:"id"`
	Name    string `yaml:"name" json:"name"`
	Default bool   `yaml:"default" json:"default"`
}

// Agent represents an AI agent with its own system prompt, backend, and models.
type Agent struct {
	ID           string       `yaml:"id" json:"id"`
	Name         string       `yaml:"name" json:"name"`
	Icon         string       `yaml:"icon" json:"icon"`
	Specialty    string       `yaml:"specialty" json:"specialty"`
	Backend      string       `yaml:"backend" json:"backend"`
	Models       []AgentModel `yaml:"models,omitempty" json:"models"`
	Command                      string   `yaml:"command,omitempty" json:"command"`                                       // optional: custom command path for the AI backend CLI
	ThinkingEffort               string   `yaml:"thinking_effort,omitempty" json:"thinkingEffort"`                       // agent's default thinking effort (from YAML); not modified by user preference
	ThinkingEffortLevels         []string `yaml:"thinking_effort_levels,omitempty" json:"thinkingEffortLevels"`         // valid levels for this backend, e.g. ["low","medium","high","xhigh"]
	PreferredModel               string   `yaml:"preferred_model,omitempty" json:"preferredModel"`                       // user's preferred model; empty = use BaseModelID()
	PreferredThinkingEffort      string   `yaml:"preferred_thinking_effort,omitempty" json:"preferredThinkingEffort"`   // user's preferred thinking effort; empty = use ThinkingEffort
	SystemPrompt                 string   `yaml:"system_prompt,omitempty" json:"systemPrompt"`

	// ModelsAutoDetected indicates whether Models were filled by auto-discovery
	// (from cache) rather than user-defined in YAML. Used by AsyncRefreshModelCache
	// to know which agents should have their models updated.
	ModelsAutoDetected bool `yaml:"-" json:"-"`

	// CanRefreshModels indicates whether this agent supports model refresh via the API.
	// Computed from BackendRegistry at load time based on whether the backend spec
	// has model discovery capability (DiscoverModelsFunc or ListModelsCmd+ParseModels).
	CanRefreshModels bool `yaml:"-" json:"canRefreshModels"`

	// Source indicates how the agent was created: "auto" (CLI detected), "setup" (wizard), "manual" (user).
	Source string `yaml:"-" json:"source"`

	// SortOrder determines display order in agent list; lower values first.
	SortOrder int `yaml:"-" json:"sortOrder"`
}

// DefaultModelID returns the default model ID for this agent.
// Priority: PreferredModel (user preference) > first model with Default:true > first model in list > empty string.
func (a *Agent) DefaultModelID() string {
	if a.PreferredModel != "" {
		return a.PreferredModel
	}
	return a.BaseModelID()
}

// BaseModelID returns the base default model ID without considering user preference.
// Used by scheduled tasks which should always use the agent's original default model.
// Priority: first model with Default:true > first model in list > empty string.
func (a *Agent) BaseModelID() string {
	for _, m := range a.Models {
		if m.Default {
			return m.ID
		}
	}
	if len(a.Models) > 0 {
		return a.Models[0].ID
	}
	return ""
}

// EffectiveThinkingEffort returns the thinking effort for interactive sessions.
// Priority: PreferredThinkingEffort (user preference) > ThinkingEffort (agent default).
func (a *Agent) EffectiveThinkingEffort() string {
	if a.PreferredThinkingEffort != "" {
		return a.PreferredThinkingEffort
	}
	return a.ThinkingEffort
}

var (
	Agents        map[string]*Agent // indexed by ID
	AgentList     []*Agent          // ordered list for API responses
	ClawbenchBin  string            // absolute path to clawbench binary for {{CLAWBENCH_BIN}} replacement
	ModelCacheDir string            // model cache directory, set by main.go at startup
	ConfigDir     string            // config directory containing rules.md; set at startup, replaces agentsDir for loadRules
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
// It loads rules.md (mandatory injection), builds a common prompt,
// and prepends it to each agent's system prompt.
func LoadAgents(dir string) error {
	Agents = make(map[string]*Agent)
	AgentList = nil
	// Set ConfigDir to the parent of the agents directory (where rules.md lives)
	// This is kept for backward compatibility during migration; will be removed after YAML loading is removed.
	ConfigDir = filepath.Dir(dir)

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

	// Sort AgentList by ID for deterministic ordering (filesystem iteration order is not guaranteed)
	sort.Slice(AgentList, func(i, j int) bool {
		return AgentList[i].ID < AgentList[j].ID
	})

	// Build common prompt from rules.md (always fully injected)
	commonPrompt := BuildCommonPrompt(false)

	// Prepend common prompt to each agent's system prompt
	for _, agent := range Agents {
		if commonPrompt != "" && agent.SystemPrompt != "" {
			agent.SystemPrompt = commonPrompt + "\n\n" + agent.SystemPrompt
		} else if commonPrompt != "" {
			agent.SystemPrompt = commonPrompt
		}
	}

	return nil
}

// WriteAgentYAML writes the agent's user-editable fields back to its YAML file.
// It uses atomic write (tmp + rename) to avoid partial writes.
// Only preferred_model and thinking_effort are written; all other fields are preserved as-is.
// DEPRECATED: This function will be removed once agent persistence moves fully to DB.
// It is kept temporarily for backward compatibility during migration.
func WriteAgentYAML(agent *Agent) error {
	if ConfigDir == "" {
		return fmt.Errorf("config directory not initialized")
	}
	yamlPath := filepath.Join(ConfigDir, "agents", agent.ID+".yaml")

	// Read existing YAML to preserve all fields
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return fmt.Errorf("read agent YAML %s: %w", yamlPath, err)
	}

	// Parse into generic map to preserve unknown fields
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse agent YAML %s: %w", yamlPath, err)
	}

	// Patch only the user-editable fields
	if agent.PreferredModel != "" {
		raw["preferred_model"] = agent.PreferredModel
	} else {
		delete(raw, "preferred_model")
	}
	if agent.PreferredThinkingEffort != "" {
		raw["preferred_thinking_effort"] = agent.PreferredThinkingEffort
	} else {
		delete(raw, "preferred_thinking_effort")
	}

	// Marshal back to YAML
	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal agent YAML %s: %w", yamlPath, err)
	}

	// Atomic write: tmp + rename
	tmpPath := yamlPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0644); err != nil {
		return fmt.Errorf("write temp agent YAML %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, yamlPath); err != nil {
		return fmt.Errorf("rename temp agent YAML: %w", err)
	}

	return nil
}

// scheduledBlockRe matches the <!-- SCHEDULED_BEGIN --> ... <!-- SCHEDULED_END --> block in rules.md.
var scheduledBlockRe = regexp.MustCompile(`(?s)\n*<!-- SCHEDULED_BEGIN -->\n(.*?)\n<!-- SCHEDULED_END -->\n*`)

// scheduledMarkerRe matches just the SCHEDULED_BEGIN/END comment lines (without the content between them).
var scheduledMarkerRe = regexp.MustCompile(`\n*<!-- SCHEDULED_(BEGIN|END) -->\n*`)

// BuildCommonPrompt generates the shared system prompt prepended to all agents.
// It loads rules.md (mandatory rules, always fully injected).
// When scheduled is true, the section wrapped in <!-- SCHEDULED_BEGIN/END --> markers
// is removed to prevent the AI from discovering scheduled task capability during
// a scheduled execution (anti-recursion).
// In both modes, the HTML comment markers themselves are stripped from the output.
func BuildCommonPrompt(scheduled bool) string {
	rules := loadRules(ConfigDir)
	if rules == "" {
		return ""
	}

	if scheduled {
		// Remove the entire SCHEDULED block (markers + content)
		rules = scheduledBlockRe.ReplaceAllString(rules, "\n\n")
	} else {
		// Keep the content but strip the HTML comment markers
		rules = scheduledMarkerRe.ReplaceAllString(rules, "\n\n")
	}
	rules = strings.TrimSpace(rules)

	return rules
}

// loadRules reads config/rules.md from the given config directory,
// replaces placeholders ({{PORT}}, {{AVAILABLE_AGENTS}}), and returns the content.
func loadRules(configDir string) string {
	data, err := os.ReadFile(filepath.Join(configDir, "rules.md"))
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))

	// Replace {{CLAWBENCH_BIN}} with absolute path to clawbench binary
	if ClawbenchBin != "" {
		content = strings.ReplaceAll(content, "{{CLAWBENCH_BIN}}", ClawbenchBin)
	}

	// Replace {{AVAILABLE_AGENTS}}
	var agentLines []string
	for _, a := range AgentList {
		agentLines = append(agentLines, fmt.Sprintf("    - %s: %s", a.ID, a.Specialty))
	}
	content = strings.ReplaceAll(content, "{{AVAILABLE_AGENTS}}", strings.Join(agentLines, "\n"))

	// Note: {{PROJECT_PATH}} is NOT replaced here — it is replaced per-request
	// in buildChatRequest() and scheduler executeTask() with the actual project
	// path from the cookie/database, not a static root path.

	return content
}

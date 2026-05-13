package model

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// BackendSpec defines a known AI backend for auto-discovery.
type BackendSpec struct {
	ID            string                    // agent id, e.g. "claude"
	Backend       string                    // backend type, e.g. "claude"
	DefaultCmd    string                    // command to detect on PATH, e.g. "claude"
	Name          string                    // display name, e.g. "Claude"
	Icon          string                    // emoji icon, e.g. "🤖"
	Specialty     string                    // short description, e.g. "代码编写与推理"
	ListModelsCmd []string                  // optional: args to list models, e.g. ["models"]; empty = not supported
	ParseModels   func(string) []AgentModel // optional: parse command stdout into AgentModel list; nil = not supported
}

// BackendRegistry lists all known AI backends for auto-discovery.
// When no agent configs exist, each entry is checked: if DefaultCmd
// is found on PATH, a YAML config is generated for that backend.
// For backends with ListModelsCmd+ParseModels, model lists are auto-discovered too.
var BackendRegistry = []BackendSpec{
	{ID: "claude", Backend: "claude", DefaultCmd: "claude", Name: "Claude", Icon: "🤖", Specialty: "代码编写与推理"},
	{ID: "codebuddy", Backend: "codebuddy", DefaultCmd: "codebuddy", Name: "Codebuddy", Icon: "🐛", Specialty: "全栈开发助手",
		ListModelsCmd: []string{"--help"}, ParseModels: ParseCodebuddyModels},
	{ID: "opencode", Backend: "opencode", DefaultCmd: "opencode", Name: "OpenCode", Icon: "📟", Specialty: "终端编码工具",
		ListModelsCmd: []string{"models"}, ParseModels: ParseOpenCodeModels},
	{ID: "gemini", Backend: "gemini", DefaultCmd: "gemini", Name: "Gemini", Icon: "💎", Specialty: "多模态推理"},
	{ID: "codex", Backend: "codex", DefaultCmd: "codex", Name: "Codex", Icon: "🐙", Specialty: "OpenAI 编码代理"},
	{ID: "qoder", Backend: "qoder", DefaultCmd: "qodercli", Name: "Qoder", Icon: "⚡", Specialty: "AI 编码助手"},
	{ID: "vecli", Backend: "vecli", DefaultCmd: "vecli", Name: "VeCLI", Icon: "🌿", Specialty: "字节跳动 AI 助手"},
	{ID: "deepseek", Backend: "deepseek", DefaultCmd: "deepseek", Name: "DeepSeek", Icon: "🔍", Specialty: "DeepSeek 推理与编码",
		ListModelsCmd: []string{"models"}, ParseModels: ParseDeepSeekModels},
}

// CheckCLIExists runs `cmd --version` with a 5-second timeout.
// Returns true if the command exits with code 0.
func CheckCLIExists(cmd string) bool {
	if cmd == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, cmd, "--version").Run()
	return err == nil
}

// DiscoverModels runs the CLI's model-list command and returns parsed models.
// Returns nil if the CLI doesn't support model listing or if the command fails.
// Errors are logged but not propagated — model discovery is best-effort.
func DiscoverModels(spec BackendSpec) []AgentModel {
	if len(spec.ListModelsCmd) == 0 || spec.ParseModels == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, spec.DefaultCmd, spec.ListModelsCmd...)
	out, err := cmd.Output()
	if err != nil {
		slog.Debug("model discovery command failed", "cmd", spec.DefaultCmd, "args", spec.ListModelsCmd, "error", err)
		return nil
	}

	models := spec.ParseModels(string(out))
	if len(models) == 0 {
		slog.Debug("model discovery returned no models", "cmd", spec.DefaultCmd)
		return nil
	}

	slog.Info("model discovery succeeded", "backend", spec.ID, "models", len(models))
	return models
}

// GenerateAgentYAML creates a YAML config for the given backend spec with the provided models.
// System prompt and command are left empty. This is a pure function — no subprocess execution.
func GenerateAgentYAML(spec BackendSpec, models []AgentModel) ([]byte, error) {
	agent := Agent{
		ID:           spec.ID,
		Name:         spec.Name,
		Icon:         spec.Icon,
		Specialty:    spec.Specialty,
		Backend:      spec.Backend,
		Models:       models,
		SystemPrompt: "",
	}
	return yaml.Marshal(agent)
}

// DiscoverAgents scans the system for installed AI CLI tools and generates
// agent YAML configs in the given directory. It only runs when no agent
// configs exist (one-time generation).
//
// For each backend in BackendRegistry, it runs `{DefaultCmd} --version`
// concurrently. If the command succeeds, it writes a YAML file.
// Existing files are not overwritten.
func DiscoverAgents(dir string) error {
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create agents directory: %w", err)
	}

	// Check all CLIs concurrently
	type result struct {
		spec   BackendSpec
		exists bool
	}
	results := make([]result, len(BackendRegistry))
	var wg sync.WaitGroup
	for i, spec := range BackendRegistry {
		wg.Add(1)
		go func(i int, spec BackendSpec) {
			defer wg.Done()
			results[i] = result{spec: spec, exists: CheckCLIExists(spec.DefaultCmd)}
		}(i, spec)
	}
	wg.Wait()

	generated := 0
	skipped := 0

	for _, r := range results {
		yamlPath := filepath.Join(dir, r.spec.ID+".yaml")

		// Don't overwrite existing files
		if _, err := os.Stat(yamlPath); err == nil {
			continue
		}

		if !r.exists {
			skipped++
			continue
		}

		data, err := GenerateAgentYAML(r.spec, DiscoverModels(r.spec))
		if err != nil {
			skipped++
			continue
		}

		if err := os.WriteFile(yamlPath, data, 0644); err != nil {
			skipped++
			continue
		}

		generated++
	}

	return nil
}

// --- Model list parsers ---

// codebuddyModelRe extracts model IDs from codebuddy --help output.
// Format: "Currently supported: (glm-4.7, glm-4.6, ...)"
var codebuddyModelRe = regexp.MustCompile(`Currently supported: \(([^)]+)\)`)

// ParseCodebuddyModels parses codebuddy --help output to extract model IDs.
// Output format: "... --model <model>  Model for the current session. ... Currently supported: (glm-4.7, glm-4.6, ...)"
func ParseCodebuddyModels(output string) []AgentModel {
	matches := codebuddyModelRe.FindStringSubmatch(output)
	if len(matches) < 2 {
		return nil
	}

	parts := strings.Split(matches[1], ",")
	var models []AgentModel
	for i, p := range parts {
		id := strings.TrimSpace(p)
		if id == "" {
			continue
		}
		models = append(models, AgentModel{
			ID:      id,
			Name:    id,
			Default: i == 0,
		})
	}
	return models
}

// deepseekModelLineRe matches lines like "  deepseek-v4-flash (deepseek)" or "* deepseek-v4-pro (deepseek)"
var deepseekModelLineRe = regexp.MustCompile(`^(\*?)\s*(\S+)\s+\((\S+)\)`)

// deepseekDefaultRe extracts the default model from the header line.
var deepseekDefaultRe = regexp.MustCompile(`Available models \(default:\s*(\S+)\)`)

// ParseDeepSeekModels parses deepseek models output.
// Output format:
//
//	Available models (default: deepseek-v4-pro)
//	  deepseek-v4-flash (deepseek)
//	* deepseek-v4-pro (deepseek)
//
// Only models from the "deepseek" provider are included (other providers are third-party replicas).
func ParseDeepSeekModels(output string) []AgentModel {
	// Extract default model name from header
	var defaultModel string
	if m := deepseekDefaultRe.FindStringSubmatch(output); len(m) >= 2 {
		defaultModel = m[1]
	}

	var models []AgentModel
	for _, line := range strings.Split(output, "\n") {
		m := deepseekModelLineRe.FindStringSubmatch(line)
		if len(m) < 4 {
			continue
		}
		isDefault := m[1] == "*" || m[2] == defaultModel || (defaultModel == "" && len(models) == 0)
		modelID := m[2]
		provider := m[3]

		// Only include the native deepseek provider
		if !strings.EqualFold(provider, "deepseek") {
			continue
		}

		models = append(models, AgentModel{
			ID:      modelID,
			Name:    modelID,
			Default: isDefault,
		})
	}
	return models
}

// opencodeModelLineRe matches lines like "minimax/MiniMax-M2.5" or "opencode/minimax-m2.5-free"
var opencodeModelLineRe = regexp.MustCompile(`^(\S+)/(\S+)$`)

// ParseOpenCodeModels parses opencode models output.
// Output format: one "provider/model" per line, e.g.:
//
//	opencode/minimax-m2.5-free
//	minimax/MiniMax-M2.5
//	anthropic/claude-sonnet-4-6
//
// The first model is marked as default.
func ParseOpenCodeModels(output string) []AgentModel {
	var models []AgentModel
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := opencodeModelLineRe.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}

		models = append(models, AgentModel{
			ID:      line, // full "provider/model" as ID (opencode uses this format)
			Name:    m[2], // model name part only for display
			Default: len(models) == 0,
		})
	}
	return models
}

package model

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// BackendSpec defines a known AI backend for auto-discovery.
type BackendSpec struct {
	ID                   string                    // agent id, e.g. "claude"
	Backend              string                    // backend type, e.g. "claude"
	DefaultCmd           string                    // command to detect on PATH, e.g. "claude"
	Name                 string                    // display name, e.g. "Claude"
	Icon                 string                    // emoji icon, e.g. "🤖"
	Specialty            string                    // short description, e.g. "代码编写与推理"
	ListModelsCmd        []string                  // optional: args to list models, e.g. ["models"]; empty = not supported
	ParseModels          func(string) []AgentModel // optional: parse command stdout into AgentModel list; nil = not supported
	DiscoverModelsFunc   func() []AgentModel       // optional: custom model discovery function (e.g. binary strings scan); takes priority over ListModelsCmd
	ThinkingEffortLevels []string                  // supported thinking effort levels, e.g. ["low","medium","high"]; nil = not supported
}

// BackendRegistry lists all known AI backends for auto-discovery.
// When no agent configs exist, each entry is checked: if DefaultCmd
// is found on PATH, a YAML config is generated for that backend.
// For backends with ListModelsCmd+ParseModels, model lists are auto-discovered too.
var BackendRegistry = []BackendSpec{
	{ID: "claude", Backend: "claude", DefaultCmd: "claude", Name: "Claude", Icon: "🤖", Specialty: "代码编写与推理",
		DiscoverModelsFunc: DiscoverClaudeModels,
		ThinkingEffortLevels: []string{"low", "medium", "high", "xhigh", "max"}},
	{ID: "codebuddy", Backend: "codebuddy", DefaultCmd: "codebuddy", Name: "Codebuddy", Icon: "🐛", Specialty: "全栈开发助手",
		ListModelsCmd: []string{"--help"}, ParseModels: ParseCodebuddyModels,
		ThinkingEffortLevels: []string{"low", "medium", "high", "xhigh"}},
	{ID: "opencode", Backend: "opencode", DefaultCmd: "opencode", Name: "OpenCode", Icon: "📟", Specialty: "终端编码工具",
		ListModelsCmd: []string{"models"}, ParseModels: ParseOpenCodeModels,
		ThinkingEffortLevels: []string{"minimal", "high", "max"}},
	{ID: "gemini", Backend: "gemini", DefaultCmd: "gemini", Name: "Gemini", Icon: "💎", Specialty: "多模态推理"},
	{ID: "codex", Backend: "codex", DefaultCmd: "codex", Name: "Codex", Icon: "🐙", Specialty: "OpenAI 编码代理",
		ThinkingEffortLevels: []string{"low", "medium", "high"}},
	{ID: "qoder", Backend: "qoder", DefaultCmd: "qodercli", Name: "Qoder", Icon: "⚡", Specialty: "AI 编码助手"},
	{ID: "vecli", Backend: "vecli", DefaultCmd: "vecli", Name: "VeCLI", Icon: "🌿", Specialty: "字节跳动 AI 助手"},
	{ID: "deepseek", Backend: "deepseek", DefaultCmd: "deepseek", Name: "DeepSeek", Icon: "🔍", Specialty: "DeepSeek 推理与编码",
		ListModelsCmd: []string{"models"}, ParseModels: ParseDeepSeekModels},
	{ID: "pi", Backend: "pi", DefaultCmd: "pi", Name: "Pi", Icon: "🥧", Specialty: "极简编程智能体",
		ListModelsCmd: []string{"--list-models"}, ParseModels: ParsePiModels,
		ThinkingEffortLevels: []string{"off", "minimal", "low", "medium", "high", "xhigh"}},
}

// CheckCLIExists checks whether a CLI command is available on the system.
// It first tries `cmd --version` with a 5-second timeout.
// If that fails, it falls back to exec.LookPath — some CLIs (especially Node.js ones)
// may return non-zero exit codes for --version when run without a TTY or in certain
// environments, but the binary itself is still present and functional.
func CheckCLIExists(cmd string) bool {
	if cmd == "" {
		return false
	}

	// Primary check: run `cmd --version`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, cmd, "--version").Run()
	if err == nil {
		return true
	}

	// Fallback: check if the binary exists on PATH
	// This handles cases where --version fails (non-zero exit, timeout, etc.)
	// but the CLI is actually installed and usable for its primary function.
	if _, lookupErr := exec.LookPath(cmd); lookupErr == nil {
		slog.Warn("CLI --version failed but binary found on PATH, keeping agent",
			"cmd", cmd, "version_error", err)
		return true
	}

	slog.Warn("CLI not found on PATH",
		"cmd", cmd, "version_error", err)
	return false
}

// DiscoverModels runs the CLI's model-list command and returns parsed models.
// Returns nil if the CLI doesn't support model listing or if the command fails.
// Errors are logged but not propagated — model discovery is best-effort.
// This is a variable so it can be overridden in tests.
var DiscoverModels = discoverModels

func discoverModels(spec BackendSpec) []AgentModel {
	// Custom discovery function takes priority (e.g. binary strings scan for claude)
	if spec.DiscoverModelsFunc != nil {
		models := spec.DiscoverModelsFunc()
		if len(models) > 0 {
			slog.Info("model discovery succeeded", "backend", spec.ID, "models", len(models))
		}
		return models
	}

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

// GenerateAgentYAML creates a minimal YAML config for the given backend spec.
// Only id, name, icon, specialty, and backend are written.
// Models, thinking_effort_levels, and system_prompt are NOT written —
// they are filled at runtime from auto-discovery and BackendRegistry.
func GenerateAgentYAML(spec BackendSpec) ([]byte, error) {
	agent := Agent{
		ID:        spec.ID,
		Name:      spec.Name,
		Icon:      spec.Icon,
		Specialty: spec.Specialty,
		Backend:   spec.Backend,
	}
	return yaml.Marshal(agent)
}

// FindSpecByBackend returns the BackendSpec for the given backend type, or nil.
func FindSpecByBackend(backend string) *BackendSpec {
	for i := range BackendRegistry {
		if BackendRegistry[i].Backend == backend {
			return &BackendRegistry[i]
		}
	}
	return nil
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

		data, err := GenerateAgentYAML(r.spec)
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

// SyncDiscoverAgents is called on every startup (not just first-run).
// It does three things:
// 1. Detects all installed CLIs from BackendRegistry.
// 2. Generates minimal YAML for newly found backends (no overwrite).
// 3. Returns a set of backend types whose CLI is currently present.
func SyncDiscoverAgents(dir string) map[string]bool {
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("failed to create agents directory", "dir", dir, "error", err)
		return nil
	}

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

	present := make(map[string]bool)
	for _, r := range results {
		if r.exists {
			present[r.spec.Backend] = true
		}

		yamlPath := filepath.Join(dir, r.spec.ID+".yaml")

		// Don't overwrite existing files
		if _, err := os.Stat(yamlPath); err == nil {
			continue
		}

		if !r.exists {
			continue
		}

		// New CLI found + no YAML → generate minimal config
		data, err := GenerateAgentYAML(r.spec)
		if err != nil {
			slog.Warn("failed to generate agent YAML", "backend", r.spec.ID, "error", err)
			continue
		}
		if err := os.WriteFile(yamlPath, data, 0644); err != nil {
			slog.Warn("failed to write agent YAML", "path", yamlPath, "error", err)
			continue
		}
		slog.Info("auto-generated agent config", "backend", r.spec.ID, "path", yamlPath)
	}

	return present
}

// CanDiscoverModels returns true if the spec supports model discovery
// via either DiscoverModelsFunc or ListModelsCmd+ParseModels.
func CanDiscoverModels(spec BackendSpec) bool {
	return spec.DiscoverModelsFunc != nil || (len(spec.ListModelsCmd) > 0 && spec.ParseModels != nil)
}

// SyncDiscoverModels runs DiscoverModels for all backends that support it
// and writes results to the model cache. This is called synchronously
// on first startup (when cache is empty).
func SyncDiscoverModels(cacheDir string) {
	for _, spec := range BackendRegistry {
		if !CanDiscoverModels(spec) {
			continue
		}
		models := DiscoverModels(spec)
		if len(models) == 0 {
			continue
		}
		if err := WriteModelCache(cacheDir, spec.Backend, models); err != nil {
			slog.Warn("failed to write model cache", "backend", spec.Backend, "error", err)
		} else {
			slog.Info("cached discovered models", "backend", spec.Backend, "count", len(models))
		}
	}
}

// AsyncRefreshModelCache runs DiscoverModels in a goroutine for all backends
// and updates the model cache + in-memory Agent data. Call this after startup
// is complete — it does not block.
func AsyncRefreshModelCache(cacheDir string) {
	go func() {
		for _, spec := range BackendRegistry {
			if !CanDiscoverModels(spec) {
				continue
			}
			models := DiscoverModels(spec)
			if len(models) == 0 {
				continue
			}
			if err := WriteModelCache(cacheDir, spec.Backend, models); err != nil {
				slog.Warn("failed to refresh model cache", "backend", spec.Backend, "error", err)
				continue
			}
			slog.Info("refreshed model cache", "backend", spec.Backend, "count", len(models))

			// Update in-memory agents whose models were auto-detected (not user-defined)
			for _, agent := range AgentList {
				if agent.Backend == spec.Backend && agent.ModelsAutoDetected {
					agent.Models = models
				}
			}
		}
	}()
}

// --- Model list parsers ---

// MergeDiscoveredData fills models and thinking_effort_levels for loaded agents.
// - Models: uses user-defined models if present; otherwise reads from model cache.
// - ThinkingEffortLevels: always from BackendRegistry by backend type (YAML values ignored).
// - Present map: if provided, agents whose backend is not in present are soft-removed
//   (removed from AgentList/Agents map, but YAML file is preserved).
func MergeDiscoveredData(cacheDir string, present ...map[string]bool) {
	var presentMap map[string]bool
	if len(present) > 0 {
		presentMap = present[0]
	}

	// Soft-remove agents whose CLI is not present
	if presentMap != nil {
		var keep []*Agent
		for _, agent := range AgentList {
			if !presentMap[agent.Backend] {
				slog.Info("soft-removing agent (CLI not found)", "id", agent.ID, "backend", agent.Backend)
				delete(Agents, agent.ID)
				continue
			}
			keep = append(keep, agent)
		}
		AgentList = keep
	}

	// Fill models and thinking effort levels
	for _, agent := range AgentList {
		spec := FindSpecByBackend(agent.Backend)

		// ThinkingEffortLevels: always from Registry (ignore YAML values)
		if spec != nil {
			agent.ThinkingEffortLevels = spec.ThinkingEffortLevels
		}

		// Models: user-defined takes priority; otherwise use cache
		if len(agent.Models) == 0 {
			cached := ReadModelCache(cacheDir, agent.Backend)
			if len(cached) > 0 {
				agent.Models = cached
				agent.ModelsAutoDetected = true
			}
		}
	}
}

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

// claudeModelRe matches Claude model IDs like "claude-sonnet-4-6" or "claude-opus-4-5" from strings output.
// Requires exactly two version segments (major-minor), excludes:
// - date-stamped like "claude-opus-4-20250514" (8-digit date suffix)
// - short aliases like "claude-sonnet-4" (points to latest snapshot)
var claudeModelRe = regexp.MustCompile(`^claude-(sonnet|opus|haiku)-\d+-\d+$`)

// claudeModelOrder defines the preferred display order: sonnet first (default), then opus, then haiku.
var claudeModelOrder = map[string]int{"sonnet": 0, "opus": 1, "haiku": 2}

// claudeModelNames maps model ID prefixes to human-readable names.
var claudeModelNames = map[string]string{
	"sonnet": "Sonnet",
	"opus":   "Opus",
	"haiku":  "Haiku",
}

// claudeIsDateStamped returns true if the model ID contains an 8-digit date segment
// like "claude-opus-4-20250514", which are snapshot aliases we want to skip.
func claudeIsDateStamped(modelID string) bool {
	for _, seg := range strings.Split(modelID, "-") {
		if len(seg) == 8 {
			return true
		}
	}
	return false
}

// DiscoverClaudeModels discovers Claude model IDs by scanning the claude binary
// with `strings`. Claude CLI does not have a --list-models command, so we extract
// model IDs from the binary which contains hardcoded model name patterns.
func DiscoverClaudeModels() []AgentModel {
	// Find the claude binary path
	path, err := exec.LookPath("claude")
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "strings", path).Output()
	if err != nil {
		slog.Debug("claude model discovery: strings command failed", "error", err)
		return nil
	}

	// Extract unique model IDs matching the pattern
	seen := make(map[string]bool)
	var models []AgentModel
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !claudeModelRe.MatchString(line) || seen[line] {
			continue
		}
		// Skip date-stamped versions like claude-opus-4-20250514
		if claudeIsDateStamped(line) {
			continue
		}
		seen[line] = true

		// Generate human-readable name: claude-sonnet-4-6 → "Claude Sonnet 4.6"
		parts := strings.SplitN(line, "-", 3) // ["claude", "sonnet", "4-6"]
		name := line
		if len(parts) == 3 {
			if family, ok := claudeModelNames[parts[1]]; ok {
				version := strings.ReplaceAll(parts[2], "-", ".")
				name = "Claude " + family + " " + version
			}
		}

		models = append(models, AgentModel{
			ID:   line,
			Name: name,
		})
	}

	// Sort: sonnet first, then opus, then haiku; within each family, newest first
	sort.Slice(models, func(i, j int) bool {
		familyI := strings.SplitN(models[i].ID, "-", 3)
		familyJ := strings.SplitN(models[j].ID, "-", 3)
		if len(familyI) >= 2 && len(familyJ) >= 2 {
			orderI, okI := claudeModelOrder[familyI[1]]
			orderJ, okJ := claudeModelOrder[familyJ[1]]
			if okI && okJ && orderI != orderJ {
				return orderI < orderJ
			}
		}
		// Same family: sort by ID descending (newest first)
		return models[i].ID > models[j].ID
	})

	// Mark first model as default
	if len(models) > 0 {
		models[0].Default = true
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
// The Name field includes the provider prefix for disambiguation,
// since different providers may offer models with identical names.
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
			Name:    m[1] + "/" + m[2], // include provider in display name for disambiguation
			Default: len(models) == 0,
		})
	}
	return models
}

// piModelLineRe matches lines from `pi --list-models` tabular output.
// Format: "provider        model                       context  max-out  thinking  images"
// We match any line with at least 2 whitespace-separated fields where the first
// doesn't look like a header.
var piModelLineRe = regexp.MustCompile(`^(\S+)\s+(\S+)`)

// ParsePiModels parses the output of `pi --list-models` into a list of AgentModel.
// Output format:
//
//	provider        model                       context  max-out  thinking  images
//	anthropic       claude-sonnet-4-6           1M       64K      yes       yes
//	openai          gpt-4o                      128K     4.1K     no        yes
//
// Models are prefixed with provider for disambiguation (e.g., "anthropic/claude-sonnet-4-6").
func ParsePiModels(output string) []AgentModel {
	var models []AgentModel
	for _, line := range strings.Split(output, "\n") {
		m := piModelLineRe.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		provider := m[1]
		modelID := m[2]
		// Skip header line
		if provider == "provider" || modelID == "model" {
			continue
		}
		fullID := provider + "/" + modelID
		models = append(models, AgentModel{
			ID:      fullID,
			Name:    modelID,
			Default: len(models) == 0,
		})
	}
	return models
}

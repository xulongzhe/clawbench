package model

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"runtime"
	"strings"
	"sync"
	"time"

	"clawbench/internal/platform"

	"gopkg.in/yaml.v3"
)

// BackendSpec defines a known AI backend for auto-discovery.
type BackendSpec struct {
	ID                   string                    // agent id, e.g. "claude"
	Backend              string                    // backend type, e.g. "claude"
	DefaultCmd           string                    // command to detect on PATH, e.g. "claude"
	NoCLI                bool                      // if true, this backend has no CLI (e.g. mock); always considered "present"
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
		DiscoverModelsFunc: DiscoverCodebuddyModels,
		ThinkingEffortLevels: []string{"low", "medium", "high", "xhigh"}},
	{ID: "opencode", Backend: "opencode", DefaultCmd: "opencode", Name: "OpenCode", Icon: "📟", Specialty: "终端编码工具",
		ListModelsCmd: []string{"models"}, ParseModels: ParseOpenCodeModels,
		ThinkingEffortLevels: []string{"minimal", "high", "max"}},
	{ID: "gemini", Backend: "gemini", DefaultCmd: "gemini", Name: "Gemini", Icon: "💎", Specialty: "多模态推理",
		DiscoverModelsFunc: DiscoverGeminiModels},
	{ID: "codex", Backend: "codex", DefaultCmd: "codex", Name: "Codex", Icon: "🐙", Specialty: "OpenAI 编码代理",
		DiscoverModelsFunc: DiscoverCodexModels,
		ThinkingEffortLevels: []string{"low", "medium", "high"}},
	{ID: "qoder", Backend: "qoder", DefaultCmd: "qodercli", Name: "Qoder", Icon: "⚡", Specialty: "AI 编码助手",
		DiscoverModelsFunc: DiscoverQoderModels},
	{ID: "vecli", Backend: "vecli", DefaultCmd: "vecli", Name: "VeCLI", Icon: "🌿", Specialty: "字节跳动 AI 助手",
		DiscoverModelsFunc: DiscoverVeCLIModels},
	{ID: "deepseek", Backend: "deepseek", DefaultCmd: "deepseek", Name: "DeepSeek", Icon: "🔍", Specialty: "DeepSeek 推理与编码",
		ListModelsCmd: []string{"models"}, ParseModels: ParseDeepSeekModels},
	{ID: "pi", Backend: "pi", DefaultCmd: "pi", Name: "Pi", Icon: "🥧", Specialty: "极简编程智能体",
		DiscoverModelsFunc: DiscoverPiModels,
		ThinkingEffortLevels: []string{"off", "minimal", "low", "medium", "high", "xhigh"}},
	{ID: "mock", Backend: "mock", NoCLI: true, Name: "Mock Agent", Icon: "🧪", Specialty: "E2E Testing"},
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

// CheckCLIExistsErr returns an error describing why the CLI is not available,
// or nil if the CLI is available. This is used for more specific error reporting.
func CheckCLIExistsErr(cmd string) error {
	if cmd == "" {
		return fmt.Errorf("empty command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, cmd, "--version").Run()
	if err == nil {
		return nil
	}

	_, lookupErr := exec.LookPath(cmd)
	if lookupErr == nil {
		// Binary exists but --version failed — CLI is still available
		return nil
	}

	return fmt.Errorf("CLI %q not found on PATH: %w", cmd, lookupErr)
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
			exists := spec.NoCLI || CheckCLIExists(spec.DefaultCmd)
			results[i] = result{spec: spec, exists: exists}
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
			exists := spec.NoCLI || CheckCLIExists(spec.DefaultCmd)
			results[i] = result{spec: spec, exists: exists}
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

	// Fill models, thinking effort levels, and CanRefreshModels
	for _, agent := range AgentList {
		spec := FindSpecByBackend(agent.Backend)

		// ThinkingEffortLevels: always from Registry (ignore YAML values)
		if spec != nil {
			agent.ThinkingEffortLevels = spec.ThinkingEffortLevels
			agent.CanRefreshModels = CanDiscoverModels(*spec)
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

// codebuddyProductFile is the JSON file in the codebuddy installation that contains
// the authoritative model list with names, capabilities, and default status.
const codebuddyProductFile = "product.cloudhosted.json"

// codebuddyProductModel represents a model entry in codebuddy's product JSON.
type codebuddyProductModel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

// codebuddyProduct represents the top-level structure of codebuddy's product JSON.
type codebuddyProduct struct {
	Models []codebuddyProductModel `json:"models"`
}

// DiscoverCodebuddyModels discovers Codebuddy models by reading the product.cloudhosted.json
// file from the CLI installation directory. This JSON file contains the authoritative model
// list with proper names and default status, making it far more reliable than --help output
// (which launches a TUI that hangs without a TTY) or JS bundle scanning (which is fragile).
func DiscoverCodebuddyModels() []AgentModel {
	// Find the codebuddy binary path
	path, err := exec.LookPath("codebuddy")
	if err != nil {
		return nil
	}

	// Resolve symlink to find the actual installation directory
	// Path is typically: .../node_modules/@tencent-ai/codebuddy-code/bin/codebuddy
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		realPath = path
	}

	// The product JSON is at .../codebuddy-code/product.cloudhosted.json
	// From .../bin/codebuddy: Dir → .../bin, Dir again → .../codebuddy-code
	pkgDir := filepath.Dir(filepath.Dir(realPath))
	productPath := filepath.Join(pkgDir, codebuddyProductFile)

	data, err := os.ReadFile(productPath)
	if err != nil {
		slog.Debug("codebuddy model discovery: product JSON not found", "path", productPath, "error", err)
		return nil
	}

	var product codebuddyProduct
	if err := json.Unmarshal(data, &product); err != nil {
		slog.Debug("codebuddy model discovery: failed to parse product JSON", "error", err)
		return nil
	}

	if len(product.Models) == 0 {
		slog.Debug("codebuddy model discovery: no models in product JSON")
		return nil
	}

	var models []AgentModel
	for _, m := range product.Models {
		// Skip pseudo-models like "default" and "auto" — these are selectors, not real model IDs
		if m.ID == "default" || m.ID == "auto" {
			continue
		}
		// Skip non-LLM models (e.g. text-to-image)
		if m.ID == "hunyuan-image-v3.0" {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, AgentModel{
			ID:      m.ID,
			Name:    name,
			Default: m.IsDefault || (len(models) == 0 && m.ID != "default" && m.ID != "auto"),
		})
	}

	if len(models) == 0 {
		return nil
	}

	// First non-skipped model gets Default=true if none was marked isDefault
	if !models[0].Default {
		models[0].Default = true
	}

	slog.Info("codebuddy model discovery succeeded", "models", len(models))
	return models
}

// codebuddyModelRe extracts model IDs from codebuddy --help output (legacy, kept for ParseCodebuddyModels).
// Format: "Currently supported: (glm-4.7, glm-4.6, ...)"
var codebuddyModelRe = regexp.MustCompile(`Currently supported: \(([^)]+)\)`)

// ParseCodebuddyModels parses codebuddy --help output to extract model IDs.
// Output format: "... --model <model>  Model for the current session. ... Currently supported: (glm-4.7, glm-4.6, ...)"
// Deprecated: codebuddy --help launches a TUI that hangs without a TTY; use DiscoverCodebuddyModels instead.
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

// claudeConfigDir returns the Claude config directory (~/.claude/).
// Overridable for testing (same pattern as DiscoverModels variable).
var claudeConfigDir = platform.ClaudeConfigDir

// LoadClaudeModelOverrides reads ~/.claude/settings.json and returns the
// modelOverrides map if present. Returns nil on any error (missing file,
// invalid JSON, no overrides key) — graceful degradation.
func LoadClaudeModelOverrides() map[string]string {
	path := filepath.Join(claudeConfigDir(), "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Debug("claude model overrides: settings.json not found", "path", path, "error", err)
		return nil
	}
	var cfg struct {
		ModelOverrides map[string]string `json:"modelOverrides"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		slog.Debug("claude model overrides: invalid JSON", "path", path, "error", err)
		return nil
	}
	if len(cfg.ModelOverrides) == 0 {
		return nil
	}
	return cfg.ModelOverrides
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

	// Apply model name overrides from ~/.claude/settings.json
	// When modelOverrides maps a Claude model ID to another name (e.g. "MiniMax-M2.7"),
	// we replace the display name so the user sees which underlying model is actually used.
	// The model ID is NOT changed — CLI invocation always uses the original Claude model ID.
	if overrides := LoadClaudeModelOverrides(); len(overrides) > 0 {
		for i := range models {
			if name, ok := overrides[models[i].ID]; ok {
				slog.Debug("claude model override applied", "id", models[i].ID, "name", name)
				models[i].Name = name
			}
		}
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
// The Name field includes the provider prefix for disambiguation (e.g., "deepseek/deepseek-v4-pro"),
// consistent with Pi and OpenCode model naming.
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

		fullID := provider + "/" + modelID
		models = append(models, AgentModel{
			ID:      fullID,
			Name:    fullID,
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
			Name:    fullID,
			Default: len(models) == 0,
		})
	}
	return models
}

// DiscoverPiModels discovers Pi model IDs by running `pi --list-models` and parsing the output.
// Pi outputs the model table to stderr (not stdout), so we must capture both streams.
func DiscoverPiModels() []AgentModel {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pi", "--list-models")
	// Pi outputs the model table to stderr; use CombinedOutput to capture both.
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("pi model discovery: command failed", "error", err)
		return nil
	}

	models := ParsePiModels(string(out))
	if len(models) == 0 {
		slog.Debug("pi model discovery: no models parsed")
		return nil
	}

	slog.Info("pi model discovery succeeded", "models", len(models))
	return models
}

// --- Gemini model discovery ---

// geminiModelDefRe matches model definition keys in the Gemini CLI JS bundle.
// Format: "gemini-X.Y-ZZZ": { ... isVisible: true ... }
var geminiModelDefRe = regexp.MustCompile(`"(gemini-\d+(?:\.\d+)?(?:-[\w-]+))":\s*\{`)

// geminiIsVisibleRe checks whether isVisible: true appears within a model definition block.
var geminiIsVisibleRe = regexp.MustCompile(`isVisible:\s*true`)

// geminiModelOrder defines the display order for Gemini models: pro first, then flash, then flash-lite.
var geminiModelOrder = map[string]int{"pro": 0, "flash": 1, "flash-lite": 2}

// geminiModelFamilyOrder defines the order for model families: gemini-3.x first, then gemini-2.5.x.
var geminiModelFamilyOrder = map[string]int{"gemini-3": 0, "gemini-2.5": 1}

// geminiTierRe extracts the tier value from a model definition block.
var geminiTierRe = regexp.MustCompile(`tier:\s*"([^"]+)"`)

// geminiFamilyRe extracts the family value from a model definition block.
var geminiFamilyRe = regexp.MustCompile(`family:\s*"([^"]+)"`)

// DiscoverGeminiModels discovers Gemini model IDs by scanning the JS bundle files
// in the Gemini CLI npm package directory. The model definitions are embedded in
// chunk-*.js files with isVisible: true/false markers.
func DiscoverGeminiModels() []AgentModel {
	path, err := exec.LookPath("gemini")
	if err != nil {
		return nil
	}

	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		realPath = path
	}

	// Navigate to the bundle directory: .../node_modules/@google/gemini-cli/bundle/
	bundleDir := filepath.Dir(realPath)
	if filepath.Base(bundleDir) != "bundle" {
		slog.Debug("gemini model discovery: unexpected path layout", "path", realPath)
		return nil
	}

	entries, err := os.ReadDir(bundleDir)
	if err != nil {
		slog.Debug("gemini model discovery: cannot read bundle directory", "dir", bundleDir, "error", err)
		return nil
	}

	type modelEntry struct {
		id     string
		tier   string
		family string
	}

	seen := make(map[string]bool)
	var found []modelEntry

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "chunk-") || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(bundleDir, entry.Name()))
		if err != nil {
			continue
		}

		content := string(data)
		matches := geminiModelDefRe.FindAllStringSubmatchIndex(content, -1)
		for _, match := range matches {
			if len(match) < 4 {
				continue
			}
			modelID := content[match[2]:match[3]]

			// Skip aliases (auto-gemini-*, single-word aliases)
			if strings.HasPrefix(modelID, "auto-gemini-") {
				continue
			}
			// Skip customtools and base variants
			if strings.HasSuffix(modelID, "-customtools") || strings.HasSuffix(modelID, "-base") {
				continue
			}
			if seen[modelID] {
				continue
			}

			// Check for isVisible: true within ~500 chars after the opening brace
			braceStart := match[1]
			lookEnd := braceStart + 500
			if lookEnd > len(content) {
				lookEnd = len(content)
			}
			block := content[braceStart:lookEnd]

			if !geminiIsVisibleRe.MatchString(block) {
				continue
			}

			seen[modelID] = true

			tier := ""
			family := ""
			if m := geminiTierRe.FindStringSubmatch(block); len(m) >= 2 {
				tier = m[1]
			}
			if m := geminiFamilyRe.FindStringSubmatch(block); len(m) >= 2 {
				family = m[1]
			}

			found = append(found, modelEntry{id: modelID, tier: tier, family: family})
		}
	}

	if len(found) == 0 {
		return nil
	}

	sort.Slice(found, func(i, j int) bool {
		fi, fj := found[i].family, found[j].family
		oi, oj := geminiModelFamilyOrder[fi], geminiModelFamilyOrder[fj]
		if oi != oj {
			return oi < oj
		}
		ti, tj := found[i].tier, found[j].tier
		tiOrder, tiOk := geminiModelOrder[ti]
		tjOrder, tjOk := geminiModelOrder[tj]
		if tiOk && tjOk && tiOrder != tjOrder {
			return tiOrder < tjOrder
		}
		return found[i].id > found[j].id
	})

	var models []AgentModel
	for i, e := range found {
		models = append(models, AgentModel{
			ID:      e.id,
			Name:    e.id,
			Default: i == 0,
		})
	}

	slog.Info("gemini model discovery succeeded", "models", len(models))
	return models
}

// --- Codex model discovery ---

// codexModelRe matches OpenAI model IDs in the Codex binary strings output.
var codexModelRe = regexp.MustCompile(`^(gpt-\d+\.\d+(-mini)?|o[34](-mini)?)$`)

// codexModelOrder defines the preferred display order for Codex models.
var codexModelOrder = map[string]int{
	"gpt-5.5":      0,
	"gpt-5.4":      1,
	"gpt-5.4-mini": 2,
	"o3":            3,
	"o4-mini":       4,
}

// codexTargetTriple returns the Rust target triple for the current platform.
func codexTargetTriple() string {
	arch := runtime.GOARCH
	switch runtime.GOOS {
	case "linux", "android":
		switch arch {
		case "amd64":
			return "x86_64-unknown-linux-musl"
		case "arm64":
			return "aarch64-unknown-linux-musl"
		}
	case "darwin":
		switch arch {
		case "amd64":
			return "x86_64-apple-darwin"
		case "arm64":
			return "aarch64-apple-darwin"
		}
	case "windows":
		switch arch {
		case "amd64":
			return "x86_64-pc-windows-msvc"
		case "arm64":
			return "aarch64-pc-windows-msvc"
		}
	}
	return ""
}

// DiscoverCodexModels discovers Codex model IDs using multiple strategies:
// 1. Run `strings` on the embedded Rust binary (works for unstripped binaries)
// 2. Read model info from the Codex state SQLite database (~/.codex/state_*.sqlite)
// 3. Fall back to hardcoded defaults based on the installed Codex version
func DiscoverCodexModels() []AgentModel {
	// Strategy 1: Try strings on the Rust binary
	if models := discoverCodexModelsFromBinary(); len(models) > 0 {
		return models
	}

	// Strategy 2: Read from Codex state SQLite database
	if models := discoverCodexModelsFromStateDB(); len(models) > 0 {
		return models
	}

	// Strategy 3: Hardcoded defaults for the current generation of Codex models
	// The Codex Rust binary is stripped, so strings extraction often fails.
	// We provide known model IDs based on the Codex version.
	return discoverCodexModelsDefaults()
}

// discoverCodexModelsFromBinary tries to extract model IDs by running `strings`
// on the Codex Rust binary. This works for unstripped or debug binaries.
func discoverCodexModelsFromBinary() []AgentModel {
	path, err := exec.LookPath("codex")
	if err != nil {
		return nil
	}

	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		realPath = path
	}

	// Navigate to the package directory: .../node_modules/@openai/codex/
	pkgDir := filepath.Dir(filepath.Dir(realPath))
	vendorDir := filepath.Join(pkgDir, "vendor")

	targetTriple := codexTargetTriple()
	if targetTriple == "" {
		return nil
	}

	binaryName := "codex"
	if runtime.GOOS == "windows" {
		binaryName = "codex.exe"
	}
	binaryPath := filepath.Join(vendorDir, targetTriple, "codex", binaryName)

	if _, err := os.Stat(binaryPath); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "strings", binaryPath).Output()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var models []AgentModel
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !codexModelRe.MatchString(line) || seen[line] {
			continue
		}
		seen[line] = true
		models = append(models, AgentModel{
			ID:   line,
			Name: line,
		})
	}

	if len(models) == 0 {
		return nil
	}

	sort.Slice(models, func(i, j int) bool {
		oi, okI := codexModelOrder[models[i].ID]
		oj, okJ := codexModelOrder[models[j].ID]
		if okI && okJ {
			return oi < oj
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return models[i].ID < models[j].ID
	})

	models[0].Default = true
	slog.Info("codex model discovery (strings) succeeded", "models", len(models))
	return models
}

// discoverCodexModelsFromStateDB reads model info from the Codex state SQLite database.
// The state database stores the model catalog that Codex fetched from OpenAI's API.
func discoverCodexModelsFromStateDB() []AgentModel {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	// Find the state SQLite database (e.g., state_5.sqlite)
	codexDir := filepath.Join(homeDir, ".codex")
	entries, err := os.ReadDir(codexDir)
	if err != nil {
		return nil
	}

	var dbPath string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "state_") && strings.HasSuffix(e.Name(), ".sqlite") {
			dbPath = filepath.Join(codexDir, e.Name())
			break
		}
	}

	if dbPath == "" {
		return nil
	}

	// Try to read models from the database
	// Codex stores model info in a "models" table or similar
	// Since we can't import C/sqlite3 directly, we use the codex CLI itself
	// to query models. But codex has no model listing command, so we skip this.
	return nil
}

// codexDefaultModels lists the known default models for the current Codex version.
// These are updated manually based on OpenAI's model catalog.
// When the strings approach or state DB approach works, those take priority.
var codexDefaultModels = []AgentModel{
	{ID: "gpt-5.5", Name: "GPT-5.5", Default: true},
	{ID: "gpt-5.4", Name: "GPT-5.4", Default: false},
	{ID: "gpt-5.4-mini", Name: "GPT-5.4 Mini", Default: false},
}

// discoverCodexModelsDefaults returns hardcoded default models for Codex.
// This is the fallback when neither binary strings nor state DB extraction works.
func discoverCodexModelsDefaults() []AgentModel {
	// Only return defaults if codex is actually installed
	if _, err := exec.LookPath("codex"); err != nil {
		return nil
	}

	models := make([]AgentModel, len(codexDefaultModels))
	copy(models, codexDefaultModels)
	slog.Info("codex model discovery: using hardcoded defaults", "models", len(models))
	return models
}

// --- Qoder model discovery ---

// qoderSkipModels are model IDs in the dynamic-texts.json that are tier-based
// selectors or routing aliases, not actual models.
var qoderSkipModels = map[string]bool{
	"auto":        true,
	"ultimate":    true,
	"performance": true,
	"efficient":   true,
	"lite":        true,
}

// qoderModelKeyRe matches keys like "modelSelector.item.qmodel" in the dynamic-texts JSON.
var qoderModelKeyRe = regexp.MustCompile(`^modelSelector\.item\.(.+)$`)

// DiscoverQoderModels discovers Qoder model IDs by reading the cached model catalog
// from ~/.qoder/.auth/dynamic-texts.json.
func DiscoverQoderModels() []AgentModel {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Debug("qoder model discovery: cannot determine home directory", "error", err)
		return nil
	}

	jsonPath := filepath.Join(homeDir, ".qoder", ".auth", "dynamic-texts.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		slog.Debug("qoder model discovery: dynamic-texts.json not found", "path", jsonPath, "error", err)
		return nil
	}

	var raw struct {
		Texts map[string]interface{} `json:"texts"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Debug("qoder model discovery: failed to parse JSON", "error", err)
		return nil
	}

	if len(raw.Texts) == 0 {
		slog.Debug("qoder model discovery: empty texts in JSON")
		return nil
	}

	type modelInfo struct {
		id   string
		name string
	}
	var modelEntries []modelInfo

	for key, val := range raw.Texts {
		m := qoderModelKeyRe.FindStringSubmatch(key)
		if len(m) < 2 {
			continue
		}
		modelID := m[1]

		// Skip description/markdown suffixes
		if strings.HasSuffix(modelID, ".description") || strings.HasSuffix(modelID, ".markdownDescription") {
			continue
		}

		// Skip known tier/alias IDs
		if qoderSkipModels[modelID] {
			continue
		}

		// Skip experts-* entries
		if strings.HasPrefix(modelID, "experts-") {
			continue
		}

		// Skip quest-* entries
		if strings.HasPrefix(modelID, "quest-") {
			continue
		}

		// Skip internal preview/dogfooding models
		if strings.HasSuffix(modelID, "_preview") {
			continue
		}

		// Skip keys with dots in the remaining part (metadata like "lite.description.quest")
		if strings.Contains(modelID, ".") {
			continue
		}

		name := modelID
		if strVal, ok := val.(string); ok && strVal != "" {
			name = strVal
		}

		modelEntries = append(modelEntries, modelInfo{id: modelID, name: name})
	}

	if len(modelEntries) == 0 {
		return nil
	}

	var models []AgentModel
	for i, e := range modelEntries {
		models = append(models, AgentModel{
			ID:      e.id,
			Name:    e.name,
			Default: i == 0,
		})
	}

	slog.Info("qoder model discovery succeeded", "models", len(models))
	return models
}

// --- VeCLI model discovery ---

// vecliModelIDRe matches id: "xxx" in MODEL_REGISTRY entries.
var vecliModelIDRe = regexp.MustCompile(`id:\s*"([^"]+)"`)

// vecliModelNameRe matches name: "xxx" in MODEL_REGISTRY entries.
var vecliModelNameRe = regexp.MustCompile(`name:\s*"([^"]+)"`)

// DiscoverVeCLIModels discovers VeCLI model IDs by parsing the MODEL_REGISTRY array
// embedded in the VeCLI JS bundle. All models are included regardless of enabled status
// (users can still select disabled models via -m flag; enabled only controls the CLI's default UI).
func DiscoverVeCLIModels() []AgentModel {
	path, err := exec.LookPath("vecli")
	if err != nil {
		return nil
	}

	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		realPath = path
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		slog.Debug("vecli model discovery: cannot read bundle file", "path", realPath, "error", err)
		return nil
	}

	content := string(data)

	registryStart := strings.Index(content, "MODEL_REGISTRY = [")
	if registryStart == -1 {
		slog.Debug("vecli model discovery: MODEL_REGISTRY not found in bundle")
		return nil
	}

	registryEnd := strings.Index(content[registryStart:], "];")
	if registryEnd == -1 {
		slog.Debug("vecli model discovery: MODEL_REGISTRY closing bracket not found")
		return nil
	}
	registrySection := content[registryStart : registryStart+registryEnd+2]

	type vecliEntry struct {
		id   string
		name string
	}

	var entries []vecliEntry
	entryStart := strings.Index(registrySection, "{")
	for entryStart != -1 {
		depth := 0
		i := entryStart
		for ; i < len(registrySection); i++ {
			if registrySection[i] == '{' {
				depth++
			} else if registrySection[i] == '}' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if i >= len(registrySection) {
			break
		}

		block := registrySection[entryStart : i+1]

		var id, name string
		if m := vecliModelIDRe.FindStringSubmatch(block); len(m) >= 2 {
			id = m[1]
		}
		if m := vecliModelNameRe.FindStringSubmatch(block); len(m) >= 2 {
			name = m[1]
		}

		if id != "" {
			entries = append(entries, vecliEntry{id: id, name: name})
		}

		remaining := registrySection[i+1:]
		nextEntry := strings.Index(remaining, "{")
		if nextEntry == -1 {
			break
		}
		entryStart = i + 1 + nextEntry
	}

	if len(entries) == 0 {
		return nil
	}

	var models []AgentModel
	for i, e := range entries {
		name := e.name
		if name == "" {
			name = e.id
		}
		models = append(models, AgentModel{
			ID:      e.id,
			Name:    name,
			Default: i == 0,
		})
	}

	slog.Info("vecli model discovery succeeded", "models", len(models))
	return models
}

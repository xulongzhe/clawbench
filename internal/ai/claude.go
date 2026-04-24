package ai

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// ClaudeBackend implements AIBackend for Claude CLI
type ClaudeBackend struct{}

// modelOverrides stores the mapping from display model names to actual model names
var (
	modelOverrides     map[string]string
	modelOverridesOnce sync.Once
)

// loadModelOverrides loads the model overrides from Claude settings.json
func loadModelOverrides() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("failed to get home dir for loading model overrides", "error", err)
		return
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		slog.Debug("failed to read Claude settings.json", "error", err)
		return
	}

	var settings struct {
		ModelOverrides map[string]string `json:"modelOverrides"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		slog.Warn("failed to parse Claude settings.json", "error", err)
		return
	}

	modelOverrides = settings.ModelOverrides
	slog.Info("loaded model overrides from Claude settings", "count", len(modelOverrides))
}

// getActualModel returns the actual model name based on modelOverrides mapping
func getActualModel(displayModel string) string {
	modelOverridesOnce.Do(loadModelOverrides)

	if modelOverrides == nil {
		return ""
	}
	return modelOverrides[displayModel]
}

// GetActualModel returns the actual model name based on modelOverrides mapping (exported)
func GetActualModel(displayModel string) string {
	actual := getActualModel(displayModel)
	if actual != "" {
		return actual
	}
	return displayModel
}

// Name returns the backend identifier
func (c *ClaudeBackend) Name() string {
	return "claude"
}

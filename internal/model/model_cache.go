package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// modelCacheEntry is the on-disk format for the model cache.
type modelCacheEntry struct {
	UpdatedAt string       `json:"updated_at"`
	Models    []AgentModel `json:"models"`
}

// ReadModelCache reads the cached model list for a backend type.
// Returns nil if cache doesn't exist, is corrupt, or has no models.
func ReadModelCache(dir, backend string) []AgentModel {
	path := filepath.Join(dir, backend+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entry modelCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	if len(entry.Models) == 0 {
		return nil
	}
	return entry.Models
}

// WriteModelCache writes the model list for a backend type to cache.
// Does not write if models is empty/nil.
func WriteModelCache(dir, backend string, models []AgentModel) error {
	if len(models) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entry := modelCacheEntry{
		UpdatedAt: time.Now().Format(time.RFC3339),
		Models:    models,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, backend+".json"), data, 0o644)
}

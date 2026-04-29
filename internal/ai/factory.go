package ai

import (
	"fmt"
)

// NewBackend creates a backend instance based on the backend type
func NewBackend(backendType string) (AIBackend, error) {
	switch backendType {
	case "claude":
		return &ExitPlanModeBackend{inner: claudeBackend}, nil
	case "codebuddy":
		return &ExitPlanModeBackend{inner: codebuddyBackend}, nil
	case "opencode":
		return opencodeBackend, nil
	case "gemini":
		return geminiBackend, nil
	case "codex":
		return &CodexBackend{}, nil
	default:
		return nil, fmt.Errorf("unsupported backend type: %s (supported: claude, codebuddy, opencode, gemini, codex)", backendType)
	}
}

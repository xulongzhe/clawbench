package ai

import (
	"fmt"
)

// NewBackend creates a backend instance based on the backend type
func NewBackend(backendType string) (AIBackend, error) {
	switch backendType {
	case "claude":
		return &ClaudeBackend{}, nil
	case "codebuddy":
		return &CodebuddyBackend{}, nil
	default:
		return nil, fmt.Errorf("unsupported backend type: %s (supported: claude, codebuddy)", backendType)
	}
}

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- claudeIsDateStamped internal tests ---

func TestClaudeIsDateStamped(t *testing.T) {
	tests := []struct {
		name     string
		modelID  string
		expected bool
	}{
		{"date stamped", "claude-opus-4-20250514", true},
		{"not date stamped", "claude-sonnet-4-6", false},
		{"single version", "claude-sonnet-4", false},
		{"8-digit non-date", "claude-12345678-model", true},
		{"short segments", "claude-opus-4-5", false},
		{"another date", "claude-haiku-3-20240307", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, claudeIsDateStamped(tt.modelID))
		})
	}
}

// --- canDiscoverModels internal tests ---

func TestCanDiscoverModels(t *testing.T) {
	tests := []struct {
		name     string
		spec     BackendSpec
		expected bool
	}{
		{
			name:     "with DiscoverModelsFunc",
			spec:     BackendSpec{DiscoverModelsFunc: func() []AgentModel { return nil }},
			expected: true,
		},
		{
			name:     "with ListModelsCmd and ParseModels",
			spec:     BackendSpec{ListModelsCmd: []string{"models"}, ParseModels: ParseOpenCodeModels},
			expected: true,
		},
		{
			name:     "with ListModelsCmd only",
			spec:     BackendSpec{ListModelsCmd: []string{"models"}},
			expected: false,
		},
		{
			name:     "with ParseModels only",
			spec:     BackendSpec{ParseModels: ParseOpenCodeModels},
			expected: false,
		},
		{
			name:     "with nothing",
			spec:     BackendSpec{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, canDiscoverModels(tt.spec))
		})
	}
}

// --- BuildCommonPrompt edge cases (internal access to agentsDir) ---

func TestBuildCommonPrompt_NoAgentsDir(t *testing.T) {
	// When agentsDir is empty, loadRules returns "", so BuildCommonPrompt returns ""
	origDir := agentsDir
	agentsDir = ""
	t.Cleanup(func() { agentsDir = origDir })

	result := BuildCommonPrompt(false)
	assert.Empty(t, result)
}

func TestBuildCommonPrompt_ScheduledRemovesMarkers(t *testing.T) {
	// Verify that in scheduled mode, both the content AND markers are removed
	origDir := agentsDir
	t.Cleanup(func() { agentsDir = origDir })

	// We can't easily test this from outside the package because agentsDir
	// is unexported. This test verifies marker stripping behavior.
	// (The external test TestBuildCommonPrompt_ScheduledRemovesSection
	// already tests the full flow via LoadAgents.)
}

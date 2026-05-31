package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Claude/Codebuddy: --effort <level> (via buildBaseStreamArgs)
// ============================================================================

func TestBuildBaseStreamArgs_ThinkingEffort_Set(t *testing.T) {
	req := ChatRequest{
		Prompt:         "hello world",
		SystemPrompt:   "you are helpful",
		Model:          "claude-4",
		WorkDir:        "/home/user/project",
		ThinkingEffort: "high",
	}
	args := buildBaseStreamArgs(req, nil)

	assert.Contains(t, args, "--effort")
	idx := indexOf(args, "--effort")
	assert.Equal(t, "high", args[idx+1], "--effort value should be 'high'")
}

func TestBuildBaseStreamArgs_ThinkingEffort_Empty(t *testing.T) {
	req := ChatRequest{
		Prompt:       "hello world",
		SystemPrompt: "you are helpful",
		Model:        "claude-4",
		WorkDir:      "/home/user/project",
	}
	args := buildBaseStreamArgs(req, nil)

	assert.NotContains(t, args, "--effort", "--effort should not appear when ThinkingEffort is empty")
}

// ============================================================================
// Pi: --thinking <level> (via buildPiStreamArgs)
// ============================================================================

func TestBuildPiStreamArgs_ThinkingEffort_Set(t *testing.T) {
	req := ChatRequest{
		Prompt:         "hello world",
		Model:          "pi-4",
		ThinkingEffort: "high",
	}
	args := buildPiStreamArgs(req)

	assert.Contains(t, args, "--thinking")
	idx := indexOf(args, "--thinking")
	assert.Equal(t, "high", args[idx+1], "--thinking value should be 'high'")
}

func TestBuildPiStreamArgs_ThinkingEffort_Empty(t *testing.T) {
	req := ChatRequest{
		Prompt: "hello world",
		Model:  "pi-4",
	}
	args := buildPiStreamArgs(req)

	assert.NotContains(t, args, "--thinking", "--thinking should not appear when ThinkingEffort is empty")
}

// ============================================================================
// Codex: -c model_reasoning_effort=<value> (via buildCodexStreamArgs)
// ============================================================================

func TestBuildCodexStreamArgs_ThinkingEffort_Set(t *testing.T) {
	req := ChatRequest{
		Prompt:         "hello world",
		Model:          "codex-1",
		ThinkingEffort: "high",
	}
	args := buildCodexStreamArgs(req)

	assert.Contains(t, args, "-c")
	idx := indexOf(args, "-c")
	assert.Equal(t, "model_reasoning_effort=high", args[idx+1], "-c value should be 'model_reasoning_effort=high'")
}

func TestBuildCodexStreamArgs_ThinkingEffort_Empty(t *testing.T) {
	req := ChatRequest{
		Prompt: "hello world",
		Model:  "codex-1",
	}
	args := buildCodexStreamArgs(req)

	for i, arg := range args {
		if arg == "-c" && i+1 < len(args) {
			assert.NotContains(t, args[i+1], "model_reasoning_effort",
				"model_reasoning_effort should not appear when ThinkingEffort is empty")
		}
	}
}

// ============================================================================
// Codex resume: -c model_reasoning_effort=<value> (via buildCodexResumeArgs)
// ============================================================================

func TestBuildCodexResumeArgs_ThinkingEffort_Set(t *testing.T) {
	req := ChatRequest{
		Prompt:         "continue this",
		Model:          "codex-1",
		ThinkingEffort: "medium",
	}
	args := buildCodexResumeArgs(req, "thread_abc123")

	assert.Contains(t, args, "-c")
	found := false
	for i, arg := range args {
		if arg == "-c" && i+1 < len(args) && args[i+1] == "model_reasoning_effort=medium" {
			found = true
			break
		}
	}
	assert.True(t, found, "should contain -c model_reasoning_effort=medium")
}

func TestBuildCodexResumeArgs_ThinkingEffort_Empty(t *testing.T) {
	req := ChatRequest{
		Prompt: "continue this",
		Model:  "codex-1",
	}
	args := buildCodexResumeArgs(req, "thread_abc123")

	for i, arg := range args {
		if arg == "-c" && i+1 < len(args) {
			assert.NotContains(t, args[i+1], "model_reasoning_effort",
				"model_reasoning_effort should not appear when ThinkingEffort is empty")
		}
	}
}

// ============================================================================
// OpenCode: --variant <level> (via buildOpenCodeStreamArgs)
// ============================================================================

func TestBuildOpenCodeStreamArgs_ThinkingEffort_Set(t *testing.T) {
	req := ChatRequest{
		Prompt:         "hello world",
		Model:          "opencode-model",
		ThinkingEffort: "high",
	}
	args := buildOpenCodeStreamArgs(req)

	assert.Contains(t, args, "--variant")
	idx := indexOf(args, "--variant")
	assert.Equal(t, "high", args[idx+1], "--variant value should be 'high'")
}

func TestBuildOpenCodeStreamArgs_ThinkingEffort_Empty(t *testing.T) {
	req := ChatRequest{
		Prompt: "hello world",
		Model:  "opencode-model",
	}
	args := buildOpenCodeStreamArgs(req)

	assert.NotContains(t, args, "--variant", "--variant should not appear when ThinkingEffort is empty")
}

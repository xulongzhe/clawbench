package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- CLIBackend Name ---

func TestCLIBackend_Name(t *testing.T) {
	b := &CLIBackend{name: "test-backend"}
	assert.Equal(t, "test-backend", b.Name())
}

// --- CLIBackend ExecuteStream ---

func TestCLIBackend_ExecuteStream_CommandFailure(t *testing.T) {
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "nonexistent-cli-command-12345",
		buildArgs:      func(req ChatRequest) []string { return []string{} },
		newParser:      func() LineParser { return &StreamParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-session",
		WorkDir:   t.TempDir(),
	})
	// Command doesn't exist, so Start should fail
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test stream: failed to start command")
}

func TestCLIBackend_ExecuteStream_ContextCancellation(t *testing.T) {
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "sleep", // will be cancelled
		buildArgs:      func(req ChatRequest) []string { return []string{"300"} },
		newParser:      func() LineParser { return &StreamParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-session",
		WorkDir:   t.TempDir(),
	})
	// Context already cancelled — command start may or may not fail
	// depending on scheduling, but it should not hang
	if err != nil {
		assert.Contains(t, err.Error(), "test stream:")
	}
}

// --- CLIBackend filterLine helpers ---

func TestDefaultFilterEmpty(t *testing.T) {
	f := defaultFilterEmpty()

	line, ok := f("")
	assert.False(t, ok)

	line, ok = f("hello")
	assert.True(t, ok)
	assert.Equal(t, "hello", line)
}

func TestFilterSkipNonJSON(t *testing.T) {
	f := filterSkipNonJSON()

	line, ok := f("")
	assert.False(t, ok)

	line, ok = f("not json")
	assert.False(t, ok)

	line, ok = f(`{"type":"content"}`)
	assert.True(t, ok)
	assert.Equal(t, `{"type":"content"}`, line)
}

// --- Factory returns CLIBackend instances ---

func TestNewBackend_ReturnsCLIBackend(t *testing.T) {
	// Verify that the backends returned by the factory implement AIBackend
	// and have the correct Name
	for _, name := range []string{"claude", "codebuddy", "opencode", "gemini"} {
		backend, err := NewBackend(name)
		assert.NoError(t, err, "NewBackend(%q) should not error", name)
		assert.Equal(t, name, backend.Name(), "backend name should match")
	}
}

func TestNewBackend_CodexIsNotCLIBackend(t *testing.T) {
	// Codex still uses its own struct
	backend, err := NewBackend("codex")
	assert.NoError(t, err)
	assert.Equal(t, "codex", backend.Name())
	// Verify it's NOT a *CLIBackend
	_, ok := backend.(*CLIBackend)
	assert.False(t, ok, "codex should NOT be a CLIBackend")
}

func TestNewBackend_ClaudeIsCLIBackend(t *testing.T) {
	backend, err := NewBackend("claude")
	assert.NoError(t, err)
	_, ok := backend.(*CLIBackend)
	assert.True(t, ok, "claude should be a CLIBackend")
}

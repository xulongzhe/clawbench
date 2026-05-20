package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	// Cancel before calling ExecuteStream — the command start should fail
	cancel()

	_, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-session",
		WorkDir:   t.TempDir(),
	})
	// With context already cancelled, either the command fails to start
	// or starts and is immediately killed. Either way, an error is expected.
	assert.Error(t, err, "pre-cancelled context should produce an error")
}

// --- CLIBackend filterLine helpers ---

func TestFilterSkipNonJSON(t *testing.T) {
	f := filterSkipNonJSON()

	_, ok := f("")
	assert.False(t, ok)

	_, ok = f("not json")
	assert.False(t, ok)

	line, ok := f(`{"type":"content"}`)
	assert.True(t, ok)
	assert.Equal(t, `{"type":"content"}`, line)
}


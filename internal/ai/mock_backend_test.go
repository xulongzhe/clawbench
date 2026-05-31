package ai

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockAIBackend_Name(t *testing.T) {
	m := NewMockAIBackend()
	assert.Equal(t, "mock", m.Name())
}

func TestMockAIBackend_ExecuteStream(t *testing.T) {
	m := NewMockAIBackend()
	assert.Equal(t, 0, m.CallCount())

	ch, err := m.ExecuteStream(context.Background(), ChatRequest{Prompt: "hello"})
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, 1, m.CallCount())

	// Collect all events
	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Should have content events, metadata, and done
	assert.GreaterOrEqual(t, len(events), 3, "expected at least content + metadata + done events")

	// First events should be content
	var contentParts []string
	for _, ev := range events {
		if ev.Type == "content" {
			contentParts = append(contentParts, ev.Content)
		}
	}
	combined := ""
	for _, p := range contentParts {
		combined += p
	}
	assert.Contains(t, combined, "mock assistant")

	// Check metadata event
	var metaEvent *StreamEvent
	for i := range events {
		if events[i].Type == "metadata" {
			metaEvent = &events[i]
			break
		}
	}
	require.NotNil(t, metaEvent, "expected a metadata event")
	assert.Equal(t, "mock-model", metaEvent.Meta.Model)
	assert.Equal(t, "end_turn", metaEvent.Meta.StopReason)

	// Last event should be done
	assert.Equal(t, "done", events[len(events)-1].Type)
}

func TestMockAIBackend_ExecuteStream_Cancel(t *testing.T) {
	m := NewMockAIBackend()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := m.ExecuteStream(ctx, ChatRequest{Prompt: "hello"})
	require.NoError(t, err)

	// Cancel context after a short delay (while streaming is still in progress)
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Should receive events and eventually close
	var gotWarning bool
	for ev := range ch {
		if ev.Type == "warning" {
			gotWarning = true
			assert.Equal(t, ReasonContextCancel, ev.Reason)
		}
	}
	assert.True(t, gotWarning, "expected a warning event on cancellation")
}

func TestMockAIBackend_CallCount(t *testing.T) {
	m := NewMockAIBackend()

	for range 3 {
		_, err := m.ExecuteStream(context.Background(), ChatRequest{Prompt: "test"})
		require.NoError(t, err)
	}
	assert.Equal(t, 3, m.CallCount())
}

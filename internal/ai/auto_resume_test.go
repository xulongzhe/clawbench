package ai

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockBackend implements AIBackend for testing.
type MockBackend struct {
	name      string
	streams   []MockStream
	callCount int
	mu        sync.Mutex
}

// MockStream defines the events and optional error for a single ExecuteStream call.
type MockStream struct {
	events []StreamEvent
	err    error
}

func (m *MockBackend) Name() string {
	return m.name
}

func (m *MockBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	m.mu.Lock()
	idx := m.callCount
	m.callCount++
	m.mu.Unlock()

	if idx >= len(m.streams) {
		ch := make(chan StreamEvent)
		close(ch)
		return ch, nil
	}

	stream := m.streams[idx]
	if stream.err != nil {
		return nil, stream.err
	}

	ch := make(chan StreamEvent, len(stream.events)+1)
	for _, e := range stream.events {
		ch <- e
	}
	close(ch)
	return ch, nil
}

// --- Tests ---

func TestExitPlanMode_TransparentPassThrough(t *testing.T) {
	mock := &MockBackend{
		name: "test",
		streams: []MockStream{
			{
				events: []StreamEvent{
					{Type: "content", Content: "hello "},
					{Type: "content", Content: "world"},
					{Type: "done"},
				},
			},
		},
	}

	wrapper := &ExitPlanModeBackend{inner: mock}
	ctx := context.Background()

	ch, err := wrapper.ExecuteStream(ctx, ChatRequest{SessionID: "test"})
	assert.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	assert.Equal(t, 2, len(events)) // "hello ", "world" — "done" closes channel
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "hello ", events[0].Content)
	assert.Equal(t, "content", events[1].Type)
	assert.Equal(t, "world", events[1].Content)
}

func TestExitPlanMode_EndsStreamOnDetection(t *testing.T) {
	mock := &MockBackend{
		name: "test",
		streams: []MockStream{
			{
				events: []StreamEvent{
					{Type: "content", Content: "planning..."},
					{Type: "tool_use", Tool: &ToolCall{Name: "ExitPlanMode", ID: "1", Done: true}},
				},
			},
		},
	}

	wrapper := &ExitPlanModeBackend{inner: mock}
	ctx := context.Background()

	ch, err := wrapper.ExecuteStream(ctx, ChatRequest{
		SessionID: "test",
		WorkDir:   "/tmp",
	})
	assert.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	// Expected: content, tool_use(ExitPlanMode), done
	assert.Equal(t, 3, len(events))
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "planning...", events[0].Content)
	assert.Equal(t, "tool_use", events[1].Type)
	assert.Equal(t, "ExitPlanMode", events[1].Tool.Name)
	assert.Equal(t, "done", events[2].Type)

	// Should only call backend once (no resume)
	assert.Equal(t, 1, mock.callCount)
}

func TestExitPlanMode_OuterCancelDuringStream(t *testing.T) {
	blockedCh := make(chan StreamEvent)
	customBackend := &blockingBackend{
		name: "test",
		ch:   blockedCh,
	}

	wrapper := &ExitPlanModeBackend{inner: customBackend}
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := wrapper.ExecuteStream(ctx, ChatRequest{SessionID: "test"})
	assert.NoError(t, err)

	cancel()

	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after outer cancel")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: outer channel did not close after cancel")
	}
}

func TestExitPlanMode_RawOutputPreserved(t *testing.T) {
	mock := &MockBackend{
		name: "test",
		streams: []MockStream{
			{
				events: []StreamEvent{
					{Type: "content", Content: "planning..."},
					{Type: "tool_use", Tool: &ToolCall{Name: "ExitPlanMode", ID: "1", Done: true}},
					{Type: "raw_output", RawOutput: "raw-data"},
				},
			},
		},
	}

	wrapper := &ExitPlanModeBackend{inner: mock}
	ctx := context.Background()

	ch, err := wrapper.ExecuteStream(ctx, ChatRequest{SessionID: "test"})
	assert.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	// raw_output should be preserved
	hasRaw := false
	for _, e := range events {
		if e.Type == "raw_output" && e.RawOutput == "raw-data" {
			hasRaw = true
		}
	}
	assert.True(t, hasRaw, "raw_output should be forwarded")
}

// --- Helpers ---

// blockingBackend returns a channel that never sends events (for cancel tests).
type blockingBackend struct {
	name string
	ch   chan StreamEvent
}

func (b *blockingBackend) Name() string { return b.name }

func (b *blockingBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	outCh := make(chan StreamEvent)
	go func() {
		defer close(outCh)
		select {
		case <-ctx.Done():
		case e, ok := <-b.ch:
			if ok {
				outCh <- e
			}
		}
	}()
	return outCh, nil
}

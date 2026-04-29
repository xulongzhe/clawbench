package ai

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockBackend implements AIBackend for testing.
// Each call to ExecuteStream returns events from the next MockStream entry.
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
		// No more streams configured — return empty closed channel
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

func TestAutoResume_TransparentPassThrough(t *testing.T) {
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

	wrapper := &AutoResumeBackend{inner: mock}
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

func TestAutoResume_ExitPlanModeDetection(t *testing.T) {
	mock := &MockBackend{
		name: "test",
		streams: []MockStream{
			// First stream: contains ExitPlanMode
			{
				events: []StreamEvent{
					{Type: "content", Content: "planning..."},
					{Type: "tool_use", Tool: &ToolCall{Name: "ExitPlanMode", ID: "1", Done: true}},
				},
			},
			// Second stream (resume)
			{
				events: []StreamEvent{
					{Type: "content", Content: "continuing..."},
					{Type: "done"},
				},
			},
		},
	}

	wrapper := &AutoResumeBackend{inner: mock}
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

	// Expected: content, tool_use(ExitPlanMode), resume_split, content(resume)
	assert.Equal(t, 4, len(events))
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "planning...", events[0].Content)
	assert.Equal(t, "tool_use", events[1].Type)
	assert.Equal(t, "ExitPlanMode", events[1].Tool.Name)
	assert.Equal(t, "resume_split", events[2].Type)
	assert.Equal(t, "content", events[3].Type)
	assert.Equal(t, "continuing...", events[3].Content)

	// Verify mock was called twice (original + resume)
	assert.Equal(t, 2, mock.callCount)
}

func TestAutoResume_OuterCancelDuringFirstStream(t *testing.T) {
	// Use a custom backend that returns a blocked channel
	blockedCh := make(chan StreamEvent)
	customBackend := &blockingBackend{
		name: "test",
		ch:   blockedCh,
	}

	wrapper := &AutoResumeBackend{inner: customBackend}
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := wrapper.ExecuteStream(ctx, ChatRequest{SessionID: "test"})
	assert.NoError(t, err)

	// Cancel the outer context
	cancel()

	// The outer channel should close promptly
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after outer cancel")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: outer channel did not close after cancel")
	}
}

func TestAutoResume_OuterCancelDuringResume(t *testing.T) {
	// Use a two-phase backend: first stream has ExitPlanMode, second stream blocks
	firstDone := make(chan struct{})
	backend := &twoPhaseBackend{
		firstEvents: []StreamEvent{
			{Type: "content", Content: "planning..."},
			{Type: "tool_use", Tool: &ToolCall{Name: "ExitPlanMode", ID: "1", Done: true}},
		},
		secondBlocked: true,
		firstDone:     firstDone,
	}

	wrapper := &AutoResumeBackend{inner: backend}
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := wrapper.ExecuteStream(ctx, ChatRequest{SessionID: "test"})
	assert.NoError(t, err)

	// Drain Phase 1 events (content, tool_use, resume_split) from the channel.
	// Once resume_split is received, the resume stream is active and we can cancel.
	gotResumeSplit := false
	for !gotResumeSplit {
		event, ok := <-ch
		if !ok {
			t.Fatal("channel closed unexpectedly before cancel")
		}
		if event.Type == "resume_split" {
			gotResumeSplit = true
		}
	}

	// Now cancel the outer context during the resume phase
	cancel()

	// The outer channel should close promptly
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after outer cancel during resume")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: outer channel did not close after cancel during resume")
	}
}

func TestAutoResume_ResumeStreamFailure(t *testing.T) {
	mock := &MockBackend{
		name: "test",
		streams: []MockStream{
			// First stream: ExitPlanMode
			{
				events: []StreamEvent{
					{Type: "content", Content: "planning..."},
					{Type: "tool_use", Tool: &ToolCall{Name: "ExitPlanMode", ID: "1", Done: true}},
				},
			},
			// Second stream: error
			{err: context.DeadlineExceeded},
		},
	}

	wrapper := &AutoResumeBackend{inner: mock}
	ctx := context.Background()

	ch, err := wrapper.ExecuteStream(ctx, ChatRequest{SessionID: "test"})
	assert.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	// Expected: content, tool_use, resume_split, done (graceful degradation)
	assert.Equal(t, 4, len(events))
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "tool_use", events[1].Type)
	assert.Equal(t, "resume_split", events[2].Type)
	assert.Equal(t, "done", events[3].Type) // fallback done event
}

func TestAutoResume_RawOutputHandling(t *testing.T) {
	mock := &MockBackend{
		name: "test",
		streams: []MockStream{
			// First stream: ExitPlanMode with raw_output
			{
				events: []StreamEvent{
					{Type: "content", Content: "planning..."},
					{Type: "tool_use", Tool: &ToolCall{Name: "ExitPlanMode", ID: "1", Done: true}},
					{Type: "raw_output", RawOutput: "first-raw"},
				},
			},
			// Second stream: has raw_output that should be suppressed
			{
				events: []StreamEvent{
					{Type: "raw_output", RawOutput: "second-raw"},
					{Type: "content", Content: "continued"},
					{Type: "done"},
				},
			},
		},
	}

	wrapper := &AutoResumeBackend{inner: mock}
	ctx := context.Background()

	ch, err := wrapper.ExecuteStream(ctx, ChatRequest{SessionID: "test"})
	assert.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	// Only first stream's raw_output should be present
	hasFirstRaw := false
	hasSecondRaw := false
	for _, e := range events {
		if e.Type == "raw_output" {
			if e.RawOutput == "first-raw" {
				hasFirstRaw = true
			}
			if e.RawOutput == "second-raw" {
				hasSecondRaw = true
			}
		}
	}
	assert.True(t, hasFirstRaw, "first stream raw_output should be forwarded")
	assert.False(t, hasSecondRaw, "second stream raw_output should be suppressed")
}

func TestAutoResume_NoNestedExitPlanMode(t *testing.T) {
	mock := &MockBackend{
		name: "test",
		streams: []MockStream{
			// First stream: ExitPlanMode
			{
				events: []StreamEvent{
					{Type: "content", Content: "planning..."},
					{Type: "tool_use", Tool: &ToolCall{Name: "ExitPlanMode", ID: "1", Done: true}},
				},
			},
			// Resume stream: also has ExitPlanMode (should be forwarded, not trigger another resume)
			{
				events: []StreamEvent{
					{Type: "content", Content: "planning again..."},
					{Type: "tool_use", Tool: &ToolCall{Name: "ExitPlanMode", ID: "2", Done: true}},
					{Type: "done"},
				},
			},
		},
	}

	wrapper := &AutoResumeBackend{inner: mock}
	ctx := context.Background()

	ch, err := wrapper.ExecuteStream(ctx, ChatRequest{SessionID: "test"})
	assert.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	// Should only have 2 calls (no nested resume), and second ExitPlanMode
	// should be forwarded as a normal event
	assert.Equal(t, 2, mock.callCount, "should not trigger nested resume")

	// Find second ExitPlanMode tool_use
	var secondExitPlanMode *StreamEvent
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == "tool_use" && events[i].Tool != nil && events[i].Tool.Name == "ExitPlanMode" && events[i].Tool.ID == "2" {
			secondExitPlanMode = &events[i]
			break
		}
	}
	assert.NotNil(t, secondExitPlanMode, "second ExitPlanMode should be forwarded as-is")
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

// twoPhaseBackend returns a first stream with specific events (e.g. ExitPlanMode),
// and a second stream that blocks until context cancellation.
type twoPhaseBackend struct {
	name          string
	firstEvents   []StreamEvent
	secondBlocked bool
	firstDone     chan struct{} // closed after first ExecuteStream returns its events
	callCount     int
	mu            sync.Mutex
}

func (b *twoPhaseBackend) Name() string { return b.name }

func (b *twoPhaseBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	b.mu.Lock()
	idx := b.callCount
	b.callCount++
	b.mu.Unlock()

	outCh := make(chan StreamEvent)

	if idx == 0 {
		// First stream: send events and close
		go func() {
			defer close(outCh)
			for _, e := range b.firstEvents {
				outCh <- e
			}
			close(b.firstDone)
		}()
	} else if b.secondBlocked {
		// Second stream: block until context cancelled
		go func() {
			defer close(outCh)
			<-ctx.Done()
		}()
	} else {
		close(outCh)
	}

	return outCh, nil
}

package ai

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MockAIBackend implements AIBackend for E2E testing.
// It returns configurable canned stream events with a small delay
// to simulate real AI CLI streaming behavior.
type MockAIBackend struct {
	mu        sync.Mutex
	callCount int
}

// NewMockAIBackend creates a new MockAIBackend instance.
func NewMockAIBackend() *MockAIBackend {
	return &MockAIBackend{}
}

// Name returns the backend identifier.
func (m *MockAIBackend) Name() string { return "mock" }

// CallCount returns the number of times ExecuteStream has been called (thread-safe).
func (m *MockAIBackend) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// ExecuteStream simulates an AI backend streaming response.
// It sends content words one by one with small delays, followed by
// metadata and a done event. This mimics real SSE streaming behavior.
func (m *MockAIBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	ch := make(chan StreamEvent, streamChanSize)

	go func() {
		defer close(ch)

		// Simulate streaming: send content in chunks with delays
		response := "Hello! I am a mock assistant. How can I help you today?"
		words := strings.Fields(response)

	for i, word := range words {
		sep := " "
		if i == 0 {
			sep = ""
		}

		// Use select to block on both the delay and context cancellation,
		// enabling instant cancellation (like real backends using cmd.Process.Kill).
		select {
		case <-ctx.Done():
			ch <- StreamEvent{Type: "warning", Content: "mock stream cancelled", Reason: ReasonContextCancel}
			return
		case <-time.After(50 * time.Millisecond):
			// Simulate streaming pace with instant cancel detection
		}

		ch <- StreamEvent{Type: "content", Content: sep + word}
	}

		// Send metadata
		ch <- StreamEvent{
			Type: "metadata",
			Meta: &Metadata{
				Model:        "mock-model",
				InputTokens:  10,
				OutputTokens: len(response) / 4,
				DurationMs:   500,
				StopReason:   "end_turn",
			},
		}

		ch <- StreamEvent{Type: "done"}
	}()

	return ch, nil
}

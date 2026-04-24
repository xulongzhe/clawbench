package service

import (
	"testing"

	"clawbench/internal/ai"
)

func TestRegisterAndGetSessionStream(t *testing.T) {
	cleanupStreams()
	sessionID := "test-session-1"

	ch := RegisterSessionStream(sessionID)

	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	// Should be able to get the same channel
	gotCh, ok := GetSessionStream(sessionID)
	if !ok {
		t.Fatal("expected to find stream for session")
	}

	// Write to original and read from got
	go func() {
		ch <- ai.StreamEvent{Type: "content", Content: "hello"}
	}()

	event := <-gotCh
	if event.Type != "content" || event.Content != "hello" {
		t.Errorf("unexpected event: %+v", event)
	}

	// Cleanup
	UnregisterSessionStream(sessionID)

	// Should no longer be available
	_, ok = GetSessionStream(sessionID)
	if ok {
		t.Fatal("expected stream to be unregistered")
	}
}

func TestGetSessionStream_NotFound(t *testing.T) {
	cleanupStreams()
	_, ok := GetSessionStream("nonexistent")
	if ok {
		t.Fatal("expected not to find stream for nonexistent session")
	}
}

func TestUnregisterSessionStream_ClosesChannel(t *testing.T) {
	cleanupStreams()
	sessionID := "test-session-close"

	ch := RegisterSessionStream(sessionID)
	UnregisterSessionStream(sessionID)

	// Reading from closed channel should return zero value with ok=false
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed")
	}
}

func cleanupStreams() {
	sessionStreams.Range(func(key, _ interface{}) bool {
		sessionStreams.Delete(key)
		return true
	})
}

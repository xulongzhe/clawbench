package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"clawbench/internal/ai"

	"github.com/stretchr/testify/assert"
)

// --- RegisterSessionCancel / UnregisterSessionCancel ---

func TestRegisterSessionCancel(t *testing.T) {
	cleanupCancels()
	defer cleanupCancels()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	RegisterSessionCancel("session-cancel-1", cancel)

	// Cancel should be stored; loading and calling it should cancel the context
	val, ok := sessionCancels.Load("session-cancel-1")
	assert.True(t, ok)
	loadedCancel, ok := val.(context.CancelFunc)
	assert.True(t, ok)
	assert.NotNil(t, loadedCancel)
}

func TestUnregisterSessionCancel(t *testing.T) {
	cleanupCancels()
	defer cleanupCancels()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	RegisterSessionCancel("session-cancel-2", cancel)
	UnregisterSessionCancel("session-cancel-2")

	_, ok := sessionCancels.Load("session-cancel-2")
	assert.False(t, ok)
}

func TestUnregisterSessionCancel_Idempotent(t *testing.T) {
	cleanupCancels()

	// Should not panic when deleting nonexistent key
	assert.NotPanics(t, func() {
		UnregisterSessionCancel("nonexistent")
	})
}

// --- GetAndClearCancelReason ---

func TestGetAndClearCancelReason_UserReason(t *testing.T) {
	cleanupCancelReasons()
	defer cleanupCancelReasons()

	sessionCancelReasons.Store("session-reason-1", "user")

	reason := GetAndClearCancelReason("session-reason-1")
	assert.Equal(t, "user", reason)

	// Should be cleared after first call
	reason2 := GetAndClearCancelReason("session-reason-1")
	assert.Equal(t, "", reason2)
}

func TestGetAndClearCancelReason_DisconnectReason(t *testing.T) {
	cleanupCancelReasons()
	defer cleanupCancelReasons()

	sessionCancelReasons.Store("session-reason-2", "disconnect")

	reason := GetAndClearCancelReason("session-reason-2")
	assert.Equal(t, "disconnect", reason)
}

func TestGetAndClearCancelReason_NoReason(t *testing.T) {
	cleanupCancelReasons()

	reason := GetAndClearCancelReason("nonexistent")
	assert.Equal(t, "", reason)
}

// --- CancelSession ---

func TestCancelSession_WithCancelFunc(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	ctx, cancel := context.WithCancel(context.Background())
	RegisterSessionCancel("session-cancel-3", cancel)
	SetSessionRunning("session-cancel-3", true)
	RegisterSessionStream("session-cancel-3")

	result := CancelSession("session-cancel-3")
	assert.True(t, result)

	// Context should be cancelled
	assert.Error(t, ctx.Err())

	// Session should no longer be running
	assert.False(t, IsSessionRunning("session-cancel-3"))

	// Cancel reason should be "user"
	reason := GetAndClearCancelReason("session-cancel-3")
	assert.Equal(t, "user", reason)

	// Cancel func should be removed
	_, ok := sessionCancels.Load("session-cancel-3")
	assert.False(t, ok)
}

func TestCancelSession_NotRunning_NoCancelFunc(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	// Session not running and no cancel func - idempotent success
	result := CancelSession("session-idle")
	assert.True(t, result)
}

func TestCancelSession_Running_NoCancelFunc(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	SetSessionRunning("session-stuck", true)

	// Running session with no cancel func - can't cancel
	result := CancelSession("session-stuck")
	assert.False(t, result)
}

func TestCancelSession_SendsCancelledEvent(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	_, cancel := context.WithCancel(context.Background())
	RegisterSessionCancel("session-event", cancel)
	SetSessionRunning("session-event", true)
	ch := RegisterSessionStream("session-event")

	result := CancelSession("session-event")
	assert.True(t, result)

	// Should receive a cancelled event
	select {
	case event := <-ch:
		assert.Equal(t, "cancelled", event.Type)
	case <-time.After(time.Second):
		t.Fatal("expected cancelled event on stream channel")
	}
}

// --- ForceCancelSession ---

func TestForceCancelSession(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	ctx, cancel := context.WithCancel(context.Background())
	RegisterSessionCancel("session-force", cancel)
	SetSessionRunning("session-force", true)

	ForceCancelSession("session-force")

	// Context should be cancelled
	assert.Error(t, ctx.Err())

	// Cancel reason should be "disconnect"
	reason := GetAndClearCancelReason("session-force")
	assert.Equal(t, "disconnect", reason)

	// Cancel func should be removed
	_, ok := sessionCancels.Load("session-force")
	assert.False(t, ok)
}

func TestForceCancelSession_NotFound(t *testing.T) {
	cleanupAllSessionState()

	// Should not panic on nonexistent session
	assert.NotPanics(t, func() {
		ForceCancelSession("nonexistent")
	})
}

// --- SendSessionEvent ---

func TestSendSessionEvent_Success(t *testing.T) {
	cleanupStreams()
	defer cleanupStreams()

	ch := RegisterSessionStream("session-event-test")

	event := ai.StreamEvent{Type: "content", Content: "hello"}
	sent := SendSessionEvent("session-event-test", event)
	assert.True(t, sent)

	received := <-ch
	assert.Equal(t, "content", received.Type)
	assert.Equal(t, "hello", received.Content)
}

func TestSendSessionEvent_SessionNotFound(t *testing.T) {
	cleanupStreams()

	sent := SendSessionEvent("nonexistent", ai.StreamEvent{Type: "content"})
	assert.False(t, sent)
}

func TestSendSessionEvent_FullChannel(t *testing.T) {
	cleanupStreams()
	defer cleanupStreams()

	RegisterSessionStream("session-full")

	// Fill the channel buffer (capacity is 256)
	for i := 0; i < 256; i++ {
		sent := SendSessionEvent("session-full", ai.StreamEvent{Type: "content", Content: "x"})
		assert.True(t, sent)
	}

	// Next send should fail (non-blocking)
	sent := SendSessionEvent("session-full", ai.StreamEvent{Type: "done"})
	assert.False(t, sent, "SendSessionEvent should return false when channel is full")
}

// --- TrySetSessionRunning ---

func TestTrySetSessionRunning_Success(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	result := TrySetSessionRunning("session-try-1")
	assert.True(t, result)
	assert.True(t, IsSessionRunning("session-try-1"))
}

func TestTrySetSessionRunning_AlreadyRunning(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	result1 := TrySetSessionRunning("session-try-2")
	assert.True(t, result1)

	result2 := TrySetSessionRunning("session-try-2")
	assert.False(t, result2, "Second TrySetSessionRunning should return false")
}

func TestTrySetSessionRunning_DifferentSessions(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	result1 := TrySetSessionRunning("session-a")
	assert.True(t, result1)
	assert.True(t, IsSessionRunning("session-a"))

	result2 := TrySetSessionRunning("session-b")
	assert.True(t, result2)
	assert.True(t, IsSessionRunning("session-b"))

	// Both should be running independently
	assert.True(t, IsSessionRunning("session-a"))
}

func TestTrySetSessionRunning_FailedTryDoesNotAffectExisting(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	// First TrySet succeeds
	assert.True(t, TrySetSessionRunning("session-x"))
	// Second TrySet on same ID fails
	assert.False(t, TrySetSessionRunning("session-x"))
	// But session is still marked as running
	assert.True(t, IsSessionRunning("session-x"))
}

func TestSetSessionRunning_TrySetMixedSequence(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	// Start via SetSessionRunning
	SetSessionRunning("session-mix", true)
	assert.True(t, IsSessionRunning("session-mix"))

	// TrySetSessionRunning on already-running session should fail
	assert.False(t, TrySetSessionRunning("session-mix"))

	// Stop via SetSessionRunning
	SetSessionRunning("session-mix", false)
	assert.False(t, IsSessionRunning("session-mix"))

	// Now TrySetSessionRunning should succeed
	assert.True(t, TrySetSessionRunning("session-mix"))
	assert.True(t, IsSessionRunning("session-mix"))
}

func TestTrySetSessionRunning_Concurrent(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	// Multiple goroutines try to set the same session as running.
	// Exactly one should succeed.
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if TrySetSessionRunning("session-concurrent-try") {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, 1, successCount, "Exactly one TrySetSessionRunning should succeed")
	assert.True(t, IsSessionRunning("session-concurrent-try"))
}

func TestSetSessionRunning_FalseRemovesKey(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	SetSessionRunning("session-rm", true)
	assert.True(t, IsSessionRunning("session-rm"))

	SetSessionRunning("session-rm", false)
	assert.False(t, IsSessionRunning("session-rm"))
}

// --- Concurrent access tests ---

func TestSendSessionEvent_ConcurrentAccess(t *testing.T) {
	cleanupStreams()
	defer cleanupStreams()

	RegisterSessionStream("session-concurrent")

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Send 50 events concurrently (buffer is 64, so most should succeed)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sent := SendSessionEvent("session-concurrent", ai.StreamEvent{Type: "content"})
			if sent {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, 50, successCount, "All 50 events should be sent (buffer is 64)")
}

// --- Helpers ---

func cleanupCancels() {
	sessionCancels.Range(func(key, _ interface{}) bool {
		sessionCancels.Delete(key)
		return true
	})
}

func cleanupCancelReasons() {
	sessionCancelReasons.Range(func(key, _ interface{}) bool {
		sessionCancelReasons.Delete(key)
		return true
	})
}

func cleanupActiveSessions() {
	activeMu.Lock()
	defer activeMu.Unlock()
	activeSessions = make(map[string]bool)
}

func cleanupAllSessionState() {
	cleanupActiveSessions()
	cleanupCancels()
	cleanupCancelReasons()
	cleanupStreams()
}

package service

import (
	"context"
	"log/slog"
	"sync"

	"clawbench/internal/ai"
	"clawbench/internal/ws"
)

// Active session tracking - keyed by sessionID
var (
	activeSessions = make(map[string]bool)
	activeMu      sync.Mutex
)

// Session stream channel management for SSE streaming
var sessionStreams sync.Map // map[string]chan ai.StreamEvent

// Session cancel functions for aborting AI responses
var sessionCancels sync.Map         // map[string]context.CancelFunc
var sessionCancelReasons sync.Map   // map[string]string — "user", "disconnect"

// emitSessionEvent broadcasts a session_update event to connected clients.
func emitSessionEvent(sessionID, status string, hasNewMessages bool) {
	mgr := ws.GetManager()
	if mgr == nil {
		return
	}
	mgr.BroadcastEvent(ws.ServerMessage{
		Type:  "event",
		ID:    ws.GenerateEventID(),
		Event: "session_update",
		Data: &ws.SessionUpdateData{
			SessionID:      sessionID,
			Status:         status,
			HasNewMessages: hasNewMessages,
		},
	})
}

// IsSessionRunning checks if a session is currently running.
func IsSessionRunning(sessionID string) bool {
	activeMu.Lock()
	defer activeMu.Unlock()
	return activeSessions[sessionID]
}

// SetSessionRunning sets the running state for a session.
// If skipEvent is true, the session_update event is suppressed (used by CancelSession
// which emits its own "cancelled" event and should not also emit "completed").
func SetSessionRunning(sessionID string, running bool, skipEvent ...bool) {
	activeMu.Lock()
	if running {
		activeSessions[sessionID] = true
	} else {
		delete(activeSessions, sessionID)
	}
	activeMu.Unlock()

	// Emit event unless caller explicitly skips (e.g. CancelSession sends its own event)
	if len(skipEvent) == 0 || !skipEvent[0] {
		if !running {
			emitSessionEvent(sessionID, "completed", true)
		} else {
			emitSessionEvent(sessionID, "running", false)
		}
	}
}

// TrySetSessionRunning atomically checks and sets running state.
// Returns true if session was successfully marked as running (was not running before).
// Returns false if session was already running.
func TrySetSessionRunning(sessionID string) bool {
	activeMu.Lock()
	defer activeMu.Unlock()

	if activeSessions[sessionID] {
		return false
	}
	activeSessions[sessionID] = true
	return true
}

// RegisterSessionCancel stores the cancel function for a session
func RegisterSessionCancel(sessionID string, cancel context.CancelFunc) {
	sessionCancels.Store(sessionID, cancel)
}

// UnregisterSessionCancel removes the cancel function for a session
func UnregisterSessionCancel(sessionID string) {
	sessionCancels.Delete(sessionID)
}

// GetAndClearCancelReason returns the reason for the most recent cancellation of a session.
// Returns "user" for user-initiated cancel, "disconnect" for SSE client disconnect.
// Returns "" if no reason was recorded (e.g. timeout or no cancel).
func GetAndClearCancelReason(sessionID string) string {
	val, ok := sessionCancelReasons.LoadAndDelete(sessionID)
	if !ok {
		return ""
	}
	return val.(string)
}

// CancelSession cancels an ongoing AI stream for a session.
// Returns true if session was found and cancelled, or if session is already not running (idempotent).
func CancelSession(sessionID string) bool {
	// Load and delete the cancel function
	val, ok := sessionCancels.LoadAndDelete(sessionID)
	if !ok {
		// If session is not in running state, consider it already cancelled (idempotent)
		if !IsSessionRunning(sessionID) {
			return true
		}
		return false
	}
	cancel, ok := val.(context.CancelFunc)
	if !ok {
		return false
	}

	// Cancel the context first (kills CLI subprocess), which causes the goroutine
	// to stop producing events and drain the channel, making room for the cancelled event.
	sessionCancelReasons.Store(sessionID, "user")
	ClearQueue(sessionID)
	cancel()
	emitSessionEvent(sessionID, "cancelled", false)

	// Send cancelled event to SSE stream after cancelling context (non-blocking)
	if streamVal, ok := sessionStreams.Load(sessionID); ok {
		if ch, ok := streamVal.(chan ai.StreamEvent); ok {
			select {
			case ch <- ai.StreamEvent{Type: "cancelled"}:
			default:
				// Channel full — SSE handler will detect session not running via checkSSE loop
			}
		}
	}

	// Mark session as not running (skip completed event — we already sent "cancelled")
	SetSessionRunning(sessionID, false, true)

	return true
}

// ForceCancelSession cancels the AI context for a session without sending SSE events.
// Used when the SSE client has disconnected and we want to stop the AI goroutine
// to prevent zombie processes.
func ForceCancelSession(sessionID string) {
	val, ok := sessionCancels.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	sessionCancelReasons.Store(sessionID, "disconnect")
	ClearQueue(sessionID)
	if cancel, ok := val.(context.CancelFunc); ok {
		cancel()
	}
}

// RegisterSessionStream creates and registers a stream channel for a session
func RegisterSessionStream(sessionID string) chan ai.StreamEvent {
	ch := make(chan ai.StreamEvent, 256)
	sessionStreams.Store(sessionID, ch)
	return ch
}

// GetSessionStream returns the stream channel for a session
func GetSessionStream(sessionID string) (<-chan ai.StreamEvent, bool) {
	val, ok := sessionStreams.Load(sessionID)
	if !ok {
		return nil, false
	}
	ch, ok := val.(chan ai.StreamEvent)
	if !ok {
		return nil, false
	}
	return ch, true
}

// UnregisterSessionStream removes and closes the stream channel for a session
func UnregisterSessionStream(sessionID string) {
	if val, ok := sessionStreams.LoadAndDelete(sessionID); ok {
		if ch, ok := val.(chan ai.StreamEvent); ok {
			close(ch)
		}
	}
}

// SendSessionEvent sends an event to the session stream channel (non-blocking).
// Returns true if the event was sent successfully.
func SendSessionEvent(sessionID string, event ai.StreamEvent) bool {
	if streamVal, ok := sessionStreams.Load(sessionID); ok {
		if ch, ok := streamVal.(chan ai.StreamEvent); ok {
			select {
			case ch <- event:
				return true
			default:
				slog.Warn("session stream channel full, dropping event",
					slog.String("session_id", sessionID),
					slog.String("event_type", event.Type),
				)
			}
		}
	}
	return false
}

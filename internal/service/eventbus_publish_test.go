package service

import (
	"database/sql"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const publishTestSchema = `
CREATE TABLE IF NOT EXISTS chat_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_path TEXT NOT NULL,
	role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
	content TEXT NOT NULL,
	files TEXT,
	session_id TEXT,
	backend TEXT NOT NULL DEFAULT 'claude',
	streaming INTEGER NOT NULL DEFAULT 0,
	indexed INTEGER NOT NULL DEFAULT 0,
	deleted INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS chat_sessions (
	id TEXT PRIMARY KEY,
	project_path TEXT NOT NULL,
	backend TEXT NOT NULL,
	title TEXT NOT NULL,
	agent_id TEXT DEFAULT '',
	agent_source TEXT DEFAULT 'default',
	model TEXT DEFAULT '',
	session_type TEXT NOT NULL DEFAULT 'chat',
	external_session_id TEXT DEFAULT '',
	thinking_effort TEXT DEFAULT '',
	deleted INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	last_read_at DATETIME,
	UNIQUE(project_path, backend, id)
);
`

// setupPublishTestDB creates an in-memory SQLite database for publish tests.
func setupPublishTestDB(t *testing.T) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	if _, err := db.Exec(publishTestSchema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	// Store original DB and restore on cleanup
	origDB := DB
	DB = db
	t.Cleanup(func() {
		db.Close()
		DB = origDB
	})
}

// setupIsolatedEventBus replaces GlobalEventBus with an isolated instance
// and returns a cleanup function to restore the original.
func setupIsolatedEventBus() (bus *EventBus, cleanup func()) {
	original := GlobalEventBus
	isolated := NewEventBus(eventBusMaxClients)
	GlobalEventBus = isolated
	return isolated, func() { GlobalEventBus = original }
}

// subscribeForTest subscribes to the bus with a predictable client ID
// and returns the channel and a cleanup function.
func subscribeForTest(bus *EventBus, t *testing.T) (<-chan SystemEvent, func()) {
	t.Helper()
	ch, ok := bus.Subscribe("test-sub-" + t.Name())
	if !ok {
		t.Fatal("failed to subscribe to event bus")
	}
	return ch, func() { bus.Unsubscribe("test-sub-" + t.Name()) }
}

// waitForEvent waits up to 2 seconds for an event of the given type.
// Returns the event and whether it was found.
func waitForEvent(ch <-chan SystemEvent, eventType string) (SystemEvent, bool) {
	timeout := time.After(2 * time.Second)
	for {
		select {
		case event := <-ch:
			if event.Type == eventType {
				return event, true
			}
			// Skip unrelated events
		case <-timeout:
			return SystemEvent{}, false
		}
	}
}

// --- AddChatMessage tests ---

func TestPublish_MessageNewOnUserMessage(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()
	setupPublishTestDB(t)

	// Create a session first
	sessionID, err := CreateSession("/tmp/test-publish", "codebuddy", "Test", "codebuddy", "", "default", "chat")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	// AddChatMessage with streaming=false should publish message_new
	_, err = AddChatMessage("/tmp/test-publish", "codebuddy", sessionID, "user", "hello", nil, false, "")
	if err != nil {
		t.Fatalf("AddChatMessage failed: %v", err)
	}

	event, found := waitForEvent(ch, "message_new")
	if !found {
		t.Fatal("message_new event not received")
	}
	if event.Payload["sessionId"] != sessionID {
		t.Errorf("expected sessionId=%s, got %v", sessionID, event.Payload["sessionId"])
	}
	if event.Payload["role"] != "user" {
		t.Errorf("expected role=user, got %v", event.Payload["role"])
	}
	if event.Payload["messageId"] == nil {
		t.Error("expected messageId to be set")
	}
}

func TestPublish_NoMessageNewForStreaming(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()
	setupPublishTestDB(t)

	// Create a session first
	sessionID, err := CreateSession("/tmp/test-publish", "codebuddy", "Test", "codebuddy", "", "default", "chat")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	// AddChatMessage with streaming=true should NOT publish message_new
	_, err = AddChatMessage("/tmp/test-publish", "codebuddy", sessionID, "assistant", "{}", nil, true, "")
	if err != nil {
		t.Fatalf("AddChatMessage failed: %v", err)
	}

	// Give a brief window for any potential event
	select {
	case event := <-ch:
		t.Errorf("unexpected event received for streaming message: %s", event.Type)
	case <-time.After(100 * time.Millisecond):
		// Correct: no event should be published for streaming messages
	}
}

// --- Direct event publish tests (verify payload format) ---

func TestPublish_TaskUpdateCreate(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type:    "task_update",
		Payload: map[string]any{"taskId": int64(1), "action": "create", "status": "active"},
	})

	event, found := waitForEvent(ch, "task_update")
	if !found {
		t.Fatal("task_update event not received")
	}
	if event.Payload["action"] != "create" {
		t.Errorf("expected action=create, got %v", event.Payload["action"])
	}
	if event.Payload["status"] != "active" {
		t.Errorf("expected status=active, got %v", event.Payload["status"])
	}
}

func TestPublish_TaskUpdatePause(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type:    "task_update",
		Payload: map[string]any{"taskId": int64(1), "action": "pause", "status": "paused"},
	})

	event, found := waitForEvent(ch, "task_update")
	if !found {
		t.Fatal("task_update event not received")
	}
	if event.Payload["action"] != "pause" {
		t.Errorf("expected action=pause, got %v", event.Payload["action"])
	}
}

func TestPublish_TaskUpdateDelete(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type:    "task_update",
		Payload: map[string]any{"taskId": int64(1), "action": "delete"},
	})

	event, found := waitForEvent(ch, "task_update")
	if !found {
		t.Fatal("task_update event not received")
	}
	if event.Payload["action"] != "delete" {
		t.Errorf("expected action=delete, got %v", event.Payload["action"])
	}
}

func TestPublish_TaskExecUpdateRunning(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type:    "task_exec_update",
		Payload: map[string]any{"taskId": int64(1), "execId": "exec-1", "status": "running", "triggerType": "auto"},
	})

	event, found := waitForEvent(ch, "task_exec_update")
	if !found {
		t.Fatal("task_exec_update event not received")
	}
	if event.Payload["status"] != "running" {
		t.Errorf("expected status=running, got %v", event.Payload["status"])
	}
	if event.Payload["triggerType"] != "auto" {
		t.Errorf("expected triggerType=auto, got %v", event.Payload["triggerType"])
	}
}

func TestPublish_TaskExecUpdateCompleted(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type:    "task_exec_update",
		Payload: map[string]any{"taskId": int64(2), "execId": "exec-2", "status": "completed"},
	})

	event, found := waitForEvent(ch, "task_exec_update")
	if !found {
		t.Fatal("task_exec_update event not received")
	}
	if event.Payload["status"] != "completed" {
		t.Errorf("expected status=completed, got %v", event.Payload["status"])
	}
}

func TestPublish_TaskExecUpdateCancelled(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type:    "task_exec_update",
		Payload: map[string]any{"taskId": int64(3), "execId": "exec-3", "status": "cancelled"},
	})

	event, found := waitForEvent(ch, "task_exec_update")
	if !found {
		t.Fatal("task_exec_update event not received")
	}
	if event.Payload["status"] != "cancelled" {
		t.Errorf("expected status=cancelled, got %v", event.Payload["status"])
	}
}

func TestPublish_TaskExecUpdateFailed(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type:    "task_exec_update",
		Payload: map[string]any{"taskId": int64(4), "execId": "exec-4", "status": "failed"},
	})

	event, found := waitForEvent(ch, "task_exec_update")
	if !found {
		t.Fatal("task_exec_update event not received")
	}
	if event.Payload["status"] != "failed" {
		t.Errorf("expected status=failed, got %v", event.Payload["status"])
	}
}

// --- SSH tunnel_status tests ---

func TestPublish_TunnelStatusConnected(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type: "tunnel_status",
		Payload: map[string]any{
			"connected":      true,
			"clientCount":   1,
			"activeChannels": 0,
		},
	})

	event, found := waitForEvent(ch, "tunnel_status")
	if !found {
		t.Fatal("tunnel_status event not received")
	}
	if event.Payload["connected"] != true {
		t.Errorf("expected connected=true, got %v", event.Payload["connected"])
	}
	if event.Payload["clientCount"] != 1 {
		t.Errorf("expected clientCount=1, got %v", event.Payload["clientCount"])
	}
}

func TestPublish_TunnelStatusDisconnected(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type: "tunnel_status",
		Payload: map[string]any{
			"connected":      false,
			"clientCount":   0,
			"activeChannels": 0,
		},
	})

	event, found := waitForEvent(ch, "tunnel_status")
	if !found {
		t.Fatal("tunnel_status event not received")
	}
	if event.Payload["connected"] != false {
		t.Errorf("expected connected=false, got %v", event.Payload["connected"])
	}
}

// --- Session lifecycle events ---

func TestPublish_SessionStart(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	// Simulate session_start as published in handler/chat.go
	GlobalEventBus.Publish(SystemEvent{
		Type:    "session_start",
		Payload: map[string]any{"sessionId": "s-123", "agentId": "codebuddy"},
	})

	event, found := waitForEvent(ch, "session_start")
	if !found {
		t.Fatal("session_start event not received")
	}
	if event.Payload["sessionId"] != "s-123" {
		t.Errorf("expected sessionId=s-123, got %v", event.Payload["sessionId"])
	}
	if event.Payload["agentId"] != "codebuddy" {
		t.Errorf("expected agentId=codebuddy, got %v", event.Payload["agentId"])
	}
}

func TestPublish_SessionCompleteDone(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type:    "session_complete",
		Payload: map[string]any{"sessionId": "s-123", "agentId": "codebuddy", "reason": "done"},
	})

	event, found := waitForEvent(ch, "session_complete")
	if !found {
		t.Fatal("session_complete event not received")
	}
	if event.Payload["reason"] != "done" {
		t.Errorf("expected reason=done, got %v", event.Payload["reason"])
	}
}

func TestPublish_SessionCompleteUserCancel(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	GlobalEventBus.Publish(SystemEvent{
		Type:    "session_complete",
		Payload: map[string]any{"sessionId": "s-456", "agentId": "claude", "reason": "user_cancel"},
	})

	event, found := waitForEvent(ch, "session_complete")
	if !found {
		t.Fatal("session_complete event not received")
	}
	if event.Payload["reason"] != "user_cancel" {
		t.Errorf("expected reason=user_cancel, got %v", event.Payload["reason"])
	}
}

// --- Integration: multiple events in sequence ---

func TestPublish_MultipleEventsInSequence(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	// Publish a sequence of events as they would occur in real usage
	GlobalEventBus.Publish(SystemEvent{Type: "task_update", Payload: map[string]any{"taskId": int64(1), "action": "create", "status": "active"}})
	GlobalEventBus.Publish(SystemEvent{Type: "task_exec_update", Payload: map[string]any{"taskId": int64(1), "execId": "exec-1", "status": "running"}})
	GlobalEventBus.Publish(SystemEvent{Type: "task_exec_update", Payload: map[string]any{"taskId": int64(1), "execId": "exec-1", "status": "completed"}})

	expected := []struct {
		eventType string
		status    string
	}{
		{"task_update", "active"},
		{"task_exec_update", "running"},
		{"task_exec_update", "completed"},
	}

	for i, exp := range expected {
		event, found := waitForEvent(ch, exp.eventType)
		if !found {
			t.Fatalf("event %d (%s) not received", i, exp.eventType)
		}
		if event.Payload["status"] != exp.status {
			t.Errorf("event %d: expected status=%s, got %v", i, exp.status, event.Payload["status"])
		}
	}
}

// --- Concurrent publish stress test ---

func TestPublish_ConcurrentStress(t *testing.T) {
	bus, cleanup := setupIsolatedEventBus()
	defer cleanup()

	ch, unsub := subscribeForTest(bus, t)
	defer unsub()

	var wg sync.WaitGroup
	const goroutines = 10
	const eventsPerGoroutine = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				GlobalEventBus.Publish(SystemEvent{
					Type:    "stress_test",
					Payload: map[string]any{"goroutine": id, "seq": j},
				})
			}
		}(i)
	}

	wg.Wait()

	// Drain and count — at least some events should arrive
	received := 0
drain:
	for {
		select {
		case <-ch:
			received++
		default:
			break drain
		}
	}

	if received == 0 {
		t.Error("should have received at least some events under stress")
	}
}

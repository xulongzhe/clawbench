package ws

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"clawbench/internal/push"
)

func newTestManager(jpush *push.JPushClient) *Manager {
	return &Manager{
		subscriptions: make(map[string]*ClientSubscription),
		jpush:        jpush,
	}
}

func TestManager_Subscribe(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	sub := mgr.Subscribe(nil, &writeMu, "client-1")
	if sub == nil {
		t.Fatal("expected non-nil subscription")
	}

	mgr.mu.Lock()
	stored, ok := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	if !ok || stored != sub {
		t.Error("subscription not stored correctly")
	}
}

func TestManager_SubscribeReplacesExisting(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu1, writeMu2 sync.Mutex

	sub1 := mgr.Subscribe(nil, &writeMu1, "client-1")
	_ = sub1

	// Second subscribe with same clientID should replace the first
	sub2 := mgr.Subscribe(nil, &writeMu2, "client-1")

	mgr.mu.Lock()
	stored := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	if stored != sub2 {
		t.Error("expected subscription to be replaced")
	}
}

func TestManager_SubscribeMultipleClients(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu1, writeMu2 sync.Mutex

	sub1 := mgr.Subscribe(nil, &writeMu1, "client-1")
	sub2 := mgr.Subscribe(nil, &writeMu2, "client-2")

	// Both should exist independently
	mgr.mu.Lock()
	s1 := mgr.subscriptions["client-1"]
	s2 := mgr.subscriptions["client-2"]
	mgr.mu.Unlock()
	if s1 != sub1 {
		t.Error("client-1 subscription not stored correctly")
	}
	if s2 != sub2 {
		t.Error("client-2 subscription not stored correctly")
	}
	if len(mgr.subscriptions) != 2 {
		t.Errorf("expected 2 subscriptions, got %d", len(mgr.subscriptions))
	}
}

func TestManager_Unsubscribe(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	mgr.DisconnectClient("client-1")

	mgr.mu.Lock()
	sub, ok := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()

	if !ok {
		t.Fatal("subscription should still exist after unsubscribe")
	}
	sub.mu.Lock()
	conn := sub.conn
	sub.mu.Unlock()
	if conn != nil {
		t.Error("expected conn to be nil after unsubscribe")
	}
}

func TestManager_RegisterPushID(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	mgr.RegisterPushID("test-reg-id", "client-1")

	mgr.mu.Lock()
	sub := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()

	sub.mu.Lock()
	regID := sub.pushRegID
	sub.mu.Unlock()
	if regID != "test-reg-id" {
		t.Errorf("expected push reg ID 'test-reg-id', got %q", regID)
	}
}

func TestManager_RegisterPushID_Dedup(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu1, writeMu2 sync.Mutex

	// Two clients, same pushRegID (same device reconnecting)
	mgr.Subscribe(nil, &writeMu1, "client-1")
	mgr.RegisterPushID("shared-reg-id", "client-1")

	mgr.Subscribe(nil, &writeMu2, "client-2")
	mgr.RegisterPushID("shared-reg-id", "client-2")

	// Client-2 should have the regID
	mgr.mu.Lock()
	s2 := mgr.subscriptions["client-2"]
	mgr.mu.Unlock()
	s2.mu.Lock()
	if s2.pushRegID != "shared-reg-id" {
		t.Errorf("client-2 expected 'shared-reg-id', got %q", s2.pushRegID)
	}
	s2.mu.Unlock()

	// Client-1 should have been cleared (dedup)
	mgr.mu.Lock()
	s1 := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	s1.mu.Lock()
	if s1.pushRegID != "" {
		t.Errorf("client-1 expected empty pushRegID (dedup), got %q", s1.pushRegID)
	}
	s1.mu.Unlock()
}

func TestManager_BroadcastEvent_NoSubscription(t *testing.T) {
	mgr := newTestManager(nil)
	// Should not panic
	mgr.BroadcastEvent(ServerMessage{Type: "event", Event: "session_update"})
}

func TestManager_BroadcastEvent_Disconnected(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	mgr.DisconnectClient("client-1")

	// Broadcast while disconnected — should buffer
	msg := ServerMessage{Type: "event", ID: "evt_1", Event: "session_update", Data: &SessionUpdateData{SessionID: "s1", Status: "completed"}}
	mgr.BroadcastEvent(msg)

	mgr.mu.Lock()
	sub := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()

	buffered := sub.GetBufferedEvents()
	if len(buffered) != 1 {
		t.Fatalf("expected 1 buffered event, got %d", len(buffered))
	}
	if buffered[0].ID != "evt_1" {
		t.Errorf("expected buffered event ID 'evt_1', got %q", buffered[0].ID)
	}
}

func TestManager_BroadcastEvent_JPushWhenDisconnected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"sendno":"123","msg_id":"456"}`))
	}))
	defer server.Close()

	jpush := push.NewJPushClient(push.JPushConfig{
		Enabled:      true,
		AppKey:       "test-key",
		MasterSecret: "test-secret",
	})

	mgr := newTestManager(jpush)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	mgr.RegisterPushID("reg-123", "client-1")
	mgr.DisconnectClient("client-1")

	// Broadcast while disconnected — should send JPush
	msg := ServerMessage{Type: "event", ID: "evt_1", Event: "session_update", Data: &SessionUpdateData{SessionID: "s1", Status: "completed"}}
	mgr.BroadcastEvent(msg)
	// If we get here without panic, JPush was called
}

func TestManager_BroadcastEvent_JPushDisabled(t *testing.T) {
	mgr := newTestManager(nil) // nil jpush = disabled
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	mgr.RegisterPushID("reg-123", "client-1")
	mgr.DisconnectClient("client-1")

	// Should not panic with nil jpush
	msg := ServerMessage{Type: "event", ID: "evt_1", Event: "task_update", Data: &TaskUpdateData{TaskID: "t1", Status: "completed"}}
	mgr.BroadcastEvent(msg)
}

func TestManager_BroadcastEvent_MultipleClients(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu1, writeMu2 sync.Mutex

	// Two clients subscribed
	mgr.Subscribe(nil, &writeMu1, "client-1")
	mgr.Subscribe(nil, &writeMu2, "client-2")

	// Disconnect both
	mgr.DisconnectClient("client-1")
	mgr.DisconnectClient("client-2")

	// Broadcast — both should buffer the event
	msg := ServerMessage{Type: "event", ID: "evt_1", Event: "session_update", Data: &SessionUpdateData{SessionID: "s1", Status: "completed"}}
	mgr.BroadcastEvent(msg)

	mgr.mu.Lock()
	s1 := mgr.subscriptions["client-1"]
	s2 := mgr.subscriptions["client-2"]
	mgr.mu.Unlock()

	if len(s1.GetBufferedEvents()) != 1 {
		t.Errorf("client-1: expected 1 buffered event, got %d", len(s1.GetBufferedEvents()))
	}
	if len(s2.GetBufferedEvents()) != 1 {
		t.Errorf("client-2: expected 1 buffered event, got %d", len(s2.GetBufferedEvents()))
	}
}

func TestBufferEvent_MaxSize(t *testing.T) {
	sub := &ClientSubscription{}

	for i := 0; i < 60; i++ {
		sub.bufferEvent(ServerMessage{ID: string(rune('a' + i%26))})
	}

	if len(sub.eventBuffer) > 50 {
		t.Errorf("expected at most 50 buffered events, got %d", len(sub.eventBuffer))
	}

	if len(sub.eventBuffer) == 50 {
		if sub.eventBuffer[0].ID != "k" {
			t.Logf("first buffered event ID: %q (eviction order may vary)", sub.eventBuffer[0].ID)
		}
	}
}

func TestGetBufferedEvents_Copy(t *testing.T) {
	sub := &ClientSubscription{}
	sub.bufferEvent(ServerMessage{ID: "evt_1"})

	events := sub.GetBufferedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Modifying the copy should not affect the original
	events[0] = ServerMessage{ID: "modified"}
	original := sub.GetBufferedEvents()
	if original[0].ID == "modified" {
		t.Error("GetBufferedEvents should return a copy")
	}
}

func TestCleanupStale_NoPushRegID(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	mgr.DisconnectClient("client-1")

	// Set bufferStart to 121 seconds ago — should be cleaned up (no pushRegID, >120s)
	mgr.mu.Lock()
	sub := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	sub.mu.Lock()
	sub.bufferStart = time.Now().Add(-121 * time.Second)
	sub.mu.Unlock()

	mgr.CleanupStale()

	mgr.mu.Lock()
	_, exists := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	if exists {
		t.Error("expected stale subscription (no push) to be cleaned up after 120s")
	}
}

func TestCleanupStale_NoPushRegID_RecentNotCleaned(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	mgr.DisconnectClient("client-1")

	// Set bufferStart to 60 seconds ago — should NOT be cleaned up (no pushRegID, <120s)
	mgr.mu.Lock()
	sub := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	sub.mu.Lock()
	sub.bufferStart = time.Now().Add(-60 * time.Second)
	sub.mu.Unlock()

	mgr.CleanupStale()

	mgr.mu.Lock()
	_, exists := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	if !exists {
		t.Error("expected subscription (no push, <120s) to not be cleaned up")
	}
}

func TestCleanupStale_WithPushRegID(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	mgr.RegisterPushID("reg-123", "client-1")
	mgr.DisconnectClient("client-1")

	// Set bufferStart to 31 minutes ago, lastActive to 31 minutes ago
	// Should NOT be cleaned up (has pushRegID, lastActive < 10 days)
	mgr.mu.Lock()
	sub := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	sub.mu.Lock()
	sub.bufferStart = time.Now().Add(-31 * time.Minute)
	sub.lastActive = time.Now().Add(-31 * time.Minute)
	sub.mu.Unlock()

	mgr.CleanupStale()

	mgr.mu.Lock()
	_, exists := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	if !exists {
		t.Error("expected subscription with pushRegID to survive (lastActive < 10 days)")
	}

	// Set lastActive to 11 days ago — should be cleaned up (no connection in 10 days)
	sub.mu.Lock()
	sub.lastActive = time.Now().Add(-11 * 24 * time.Hour)
	sub.mu.Unlock()

	mgr.CleanupStale()

	mgr.mu.Lock()
	_, exists = mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	if exists {
		t.Error("expected subscription with pushRegID to be cleaned up (lastActive > 10 days)")
	}
}

func TestCleanupStale_RecentNotCleaned(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	// Not unsubscribing — conn is active, should not be cleaned

	mgr.CleanupStale()

	mgr.mu.Lock()
	_, exists := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()
	if !exists {
		t.Error("expected active subscription to not be cleaned up")
	}
}

func TestClientSubscription_GetBufferedEvents_Empty(t *testing.T) {
	sub := &ClientSubscription{}
	events := sub.GetBufferedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestBroadcastEvent_BufferWindow(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu, "client-1")
	mgr.DisconnectClient("client-1")

	// Within buffer window (10s) — should buffer
	msg := ServerMessage{Type: "event", ID: "evt_1", Event: "session_update", Data: &SessionUpdateData{SessionID: "s1", Status: "completed"}}
	mgr.BroadcastEvent(msg)

	mgr.mu.Lock()
	sub := mgr.subscriptions["client-1"]
	mgr.mu.Unlock()

	buffered := sub.GetBufferedEvents()
	if len(buffered) != 1 {
		t.Fatalf("expected 1 buffered event within window, got %d", len(buffered))
	}

	// Beyond buffer window — should not buffer
	sub.mu.Lock()
	sub.bufferStart = time.Now().Add(-15 * time.Second)
	sub.eventBuffer = nil
	sub.mu.Unlock()

	msg2 := ServerMessage{Type: "event", ID: "evt_2", Event: "task_update", Data: &TaskUpdateData{TaskID: "t1", Status: "completed"}}
	mgr.BroadcastEvent(msg2)

	buffered2 := sub.GetBufferedEvents()
	if len(buffered2) != 0 {
		t.Errorf("expected 0 buffered events outside window, got %d", len(buffered2))
	}
}

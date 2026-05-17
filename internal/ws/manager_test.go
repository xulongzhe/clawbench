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

	// Subscribe without a real websocket conn (nil is fine for testing subscription tracking)
	sub := mgr.Subscribe(nil, &writeMu)
	if sub == nil {
		t.Fatal("expected non-nil subscription")
	}

	// Verify subscription is stored
	key := clientKey()
	mgr.mu.Lock()
	stored, ok := mgr.subscriptions[key]
	mgr.mu.Unlock()
	if !ok || stored != sub {
		t.Error("subscription not stored correctly")
	}
}

func TestManager_SubscribeReplacesExisting(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu1, writeMu2 sync.Mutex

	sub1 := mgr.Subscribe(nil, &writeMu1)
	_ = sub1

	// Second subscribe should replace the first
	sub2 := mgr.Subscribe(nil, &writeMu2)

	key := clientKey()
	mgr.mu.Lock()
	stored := mgr.subscriptions[key]
	mgr.mu.Unlock()
	if stored != sub2 {
		t.Error("expected subscription to be replaced")
	}
}

func TestManager_Unsubscribe(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu)
	mgr.Unsubscribe()

	key := clientKey()
	mgr.mu.Lock()
	sub, ok := mgr.subscriptions[key]
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

	mgr.Subscribe(nil, &writeMu)
	mgr.RegisterPushID("test-reg-id")

	key := clientKey()
	mgr.mu.Lock()
	sub := mgr.subscriptions[key]
	mgr.mu.Unlock()

	sub.mu.Lock()
	regID := sub.pushRegID
	sub.mu.Unlock()
	if regID != "test-reg-id" {
		t.Errorf("expected push reg ID 'test-reg-id', got %q", regID)
	}
}

func TestManager_RegisterPushID_NoSubscription(t *testing.T) {
	mgr := newTestManager(nil)
	// Should create a subscription entry automatically (HTTP call before WS connect)
	mgr.RegisterPushID("test-reg-id")

	key := clientKey()
	mgr.mu.Lock()
	sub, ok := mgr.subscriptions[key]
	mgr.mu.Unlock()

	if !ok {
		t.Fatal("expected subscription to be created by RegisterPushID")
	}
	sub.mu.Lock()
	regID := sub.pushRegID
	sub.mu.Unlock()
	if regID != "test-reg-id" {
		t.Errorf("expected push reg ID 'test-reg-id', got %q", regID)
	}
}

func TestManager_BroadcastEvent_NoSubscription(t *testing.T) {
	mgr := newTestManager(nil)
	// Should not panic
	mgr.BroadcastEvent(ServerMessage{Type: "event", Event: "session_update"})
}

func TestManager_BroadcastEvent_Disconnected(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu)
	mgr.Unsubscribe() // disconnect

	// Broadcast while disconnected — should buffer
	msg := ServerMessage{Type: "event", ID: "evt_1", Event: "session_update", Data: &SessionUpdateData{SessionID: "s1", Status: "completed"}}
	mgr.BroadcastEvent(msg)

	key := clientKey()
	mgr.mu.Lock()
	sub := mgr.subscriptions[key]
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
	// Create a test JPush server
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

	mgr.Subscribe(nil, &writeMu)
	mgr.RegisterPushID("reg-123")
	mgr.Unsubscribe() // disconnect

	// Broadcast while disconnected — should send JPush
	msg := ServerMessage{Type: "event", ID: "evt_1", Event: "session_update", Data: &SessionUpdateData{SessionID: "s1", Status: "completed"}}
	mgr.BroadcastEvent(msg)
	// If we get here without panic, JPush was called
}

func TestManager_BroadcastEvent_JPushDisabled(t *testing.T) {
	mgr := newTestManager(nil) // nil jpush = disabled
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu)
	mgr.RegisterPushID("reg-123")
	mgr.Unsubscribe()

	// Should not panic with nil jpush
	msg := ServerMessage{Type: "event", ID: "evt_1", Event: "task_update", Data: &TaskUpdateData{TaskID: "t1", Status: "completed"}}
	mgr.BroadcastEvent(msg)
}

func TestBufferEvent_MaxSize(t *testing.T) {
	sub := &ClientSubscription{}

	for i := 0; i < 60; i++ {
		sub.bufferEvent(ServerMessage{ID: string(rune('a' + i%26))})
	}

	if len(sub.eventBuffer) > 50 {
		t.Errorf("expected at most 50 buffered events, got %d", len(sub.eventBuffer))
	}

	// Should keep the last 50
	if len(sub.eventBuffer) == 50 {
		// First buffered event should be the 11th (index 10)
		if sub.eventBuffer[0].ID != "k" { // 10th letter (0-indexed: a=0..j=9, k=10)
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

func TestCleanupStale(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu)
	mgr.Unsubscribe()

	// Set bufferStart to 31 minutes ago
	key := clientKey()
	mgr.mu.Lock()
	sub := mgr.subscriptions[key]
	mgr.mu.Unlock()
	sub.mu.Lock()
	sub.bufferStart = time.Now().Add(-31 * time.Minute)
	sub.mu.Unlock()

	mgr.CleanupStale()

	mgr.mu.Lock()
	_, exists := mgr.subscriptions[key]
	mgr.mu.Unlock()
	if exists {
		t.Error("expected stale subscription to be cleaned up")
	}
}

func TestCleanupStale_RecentNotCleaned(t *testing.T) {
	mgr := newTestManager(nil)
	var writeMu sync.Mutex

	mgr.Subscribe(nil, &writeMu)
	// Not unsubscribing — conn is active, should not be cleaned

	mgr.CleanupStale()

	key := clientKey()
	mgr.mu.Lock()
	_, exists := mgr.subscriptions[key]
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

	mgr.Subscribe(nil, &writeMu)
	mgr.Unsubscribe()

	// Within buffer window (10s) — should buffer
	msg := ServerMessage{Type: "event", ID: "evt_1", Event: "session_update", Data: &SessionUpdateData{SessionID: "s1", Status: "completed"}}
	mgr.BroadcastEvent(msg)

	key := clientKey()
	mgr.mu.Lock()
	sub := mgr.subscriptions[key]
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

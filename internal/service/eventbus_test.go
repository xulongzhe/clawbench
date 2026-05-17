package service

import (
	"sync"
	"testing"
	"time"
)

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	ch, ok := bus.Subscribe("client-1")
	if !ok {
		t.Fatal("Subscribe should succeed")
	}
	defer bus.Unsubscribe("client-1")

	event := SystemEvent{Type: "session_start", Payload: map[string]any{"sessionId": "s1"}}
	bus.Publish(event)

	select {
	case got := <-ch:
		if got.Type != "session_start" {
			t.Errorf("expected type session_start, got %s", got.Type)
		}
		if got.Payload["sessionId"] != "s1" {
			t.Errorf("expected sessionId=s1, got %v", got.Payload["sessionId"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventBus_UnsubscribeClosesChannel(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	ch, ok := bus.Subscribe("client-2")
	if !ok {
		t.Fatal("Subscribe should succeed")
	}

	bus.Unsubscribe("client-2")

	// Channel should be closed — reading returns zero value with ok=false
	_, ok2 := <-ch
	if ok2 {
		t.Error("channel should be closed after Unsubscribe")
	}
}

func TestEventBus_UnsubscribeIdempotent(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	bus.Subscribe("client-3")

	// Unsubscribe twice should not panic
	bus.Unsubscribe("client-3")
	bus.Unsubscribe("client-3")
}

func TestEventBus_PublishToMultipleClients(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	ch1, _ := bus.Subscribe("c1")
	defer bus.Unsubscribe("c1")
	ch2, _ := bus.Subscribe("c2")
	defer bus.Unsubscribe("c2")

	event := SystemEvent{Type: "test", Payload: nil}
	bus.Publish(event)

	// Both clients should receive the event
	for i, ch := range []<-chan SystemEvent{ch1, ch2} {
		select {
		case <-ch:
			// OK
		case <-time.After(time.Second):
			t.Errorf("client %d timed out", i)
		}
	}
}

func TestEventBus_DropWhenFull(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	ch, _ := bus.Subscribe("full-client")
	defer bus.Unsubscribe("full-client")

	// Fill the channel buffer completely
	for i := 0; i < eventBusChannelBuf; i++ {
		bus.Publish(SystemEvent{Type: "fill", Payload: map[string]any{"i": i}})
	}

	// One more publish should not block (non-blocking send drops the event)
	bus.Publish(SystemEvent{Type: "overflow", Payload: nil})

	// Channel should have exactly eventBusChannelBuf events (overflow dropped)
	if len(ch) != eventBusChannelBuf {
		t.Errorf("expected %d events in channel, got %d", eventBusChannelBuf, len(ch))
	}
}

func TestEventBus_MaxClientsLimit(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: 3,
	}

	// Subscribe 3 clients (the limit)
	for i := 0; i < 3; i++ {
		_, ok := bus.Subscribe(string(rune('a' + i)))
		if !ok {
			t.Fatalf("subscribe %d should succeed", i)
		}
	}

	// 4th should fail
	_, ok := bus.Subscribe("d")
	if ok {
		t.Error("subscribe beyond maxClients should fail")
	}
}

func TestEventBus_ClientCount(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	if count := bus.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}

	bus.Subscribe("c1")
	bus.Subscribe("c2")
	if count := bus.ClientCount(); count != 2 {
		t.Errorf("expected 2 clients, got %d", count)
	}

	bus.Unsubscribe("c1")
	if count := bus.ClientCount(); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}
}

func TestEventBus_ConcurrentPublish(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	ch, _ := bus.Subscribe("concurrent")
	defer bus.Unsubscribe("concurrent")

	var wg sync.WaitGroup
	const publishers = 10
	const eventsPerPublisher = 100

	for i := 0; i < publishers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerPublisher; j++ {
				bus.Publish(SystemEvent{
					Type:    "concurrent",
					Payload: map[string]any{"pub": id, "seq": j},
				})
			}
		}(i)
	}

	wg.Wait()

	// Drain and count received events (some may be dropped if channel is full)
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
		t.Error("should have received at least some events")
	}
	// We don't assert exact count because drops are expected under load
}

func TestEventBus_PublishAfterUnsubscribe(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	bus.Subscribe("temp")
	bus.Unsubscribe("temp")

	// Publishing after unsubscribe should not panic
	bus.Publish(SystemEvent{Type: "after_unsubscribe", Payload: nil})
}

func TestEventBus_GlobalEventBusInitialized(t *testing.T) {
	// GlobalEventBus should be usable without initialization
	if GlobalEventBus == nil {
		t.Fatal("GlobalEventBus should be initialized")
	}
	if GlobalEventBus.maxClients != eventBusMaxClients {
		t.Errorf("expected maxClients=%d, got %d", eventBusMaxClients, GlobalEventBus.maxClients)
	}
}

func TestEventBus_DuplicateClientIDOverwrites(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	oldCh, ok1 := bus.Subscribe("dup")
	if !ok1 {
		t.Fatal("first subscribe should succeed")
	}

	// Subscribe with same clientID — overwrites the previous channel
	ch2, ok2 := bus.Subscribe("dup")
	if !ok2 {
		t.Fatal("second subscribe should succeed")
	}

	// ClientCount should be 1 (not 2) — replacement doesn't increment
	if count := bus.ClientCount(); count != 1 {
		t.Errorf("expected 1 client after duplicate subscribe, got %d", count)
	}

	// Old channel should be closed (signals old subscriber to stop)
	_, ok3 := <-oldCh
	if ok3 {
		t.Error("old channel should be closed after duplicate subscribe")
	}

	// New channel should receive events
	bus.Publish(SystemEvent{Type: "test", Payload: nil})

	select {
	case <-ch2:
		// OK
	case <-time.After(time.Second):
		t.Error("new channel should receive event")
	}

	// Clean up
	bus.Unsubscribe("dup")
}

func TestEventBus_PublishNilPayload(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	ch, _ := bus.Subscribe("nil-payload")
	defer bus.Unsubscribe("nil-payload")

	bus.Publish(SystemEvent{Type: "nil_test", Payload: nil})

	select {
	case got := <-ch:
		if got.Type != "nil_test" {
			t.Errorf("expected type nil_test, got %s", got.Type)
		}
		if got.Payload != nil {
			t.Errorf("expected nil payload, got %v", got.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event with nil payload")
	}
}

func TestEventBus_EmptyClientID(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	ch, ok := bus.Subscribe("")
	if !ok {
		t.Fatal("subscribe with empty clientID should succeed")
	}
	defer bus.Unsubscribe("")

	bus.Publish(SystemEvent{Type: "empty_id_test", Payload: nil})

	select {
	case <-ch:
		// OK
	case <-time.After(time.Second):
		t.Error("empty clientID subscriber should receive event")
	}
}

func TestEventBus_NewEventBusZeroMaxClients(t *testing.T) {
	bus := NewEventBus(0)

	_, ok := bus.Subscribe("any")
	if ok {
		t.Error("subscribe should fail when maxClients=0")
	}
	if count := bus.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}
}

func TestEventBus_UnsubscribeNonExistentWithOtherClients(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	bus.Subscribe("real")
	defer bus.Unsubscribe("real")

	// Unsubscribe a non-existent clientID
	bus.Unsubscribe("ghost")

	// Existing client should still be there
	if count := bus.ClientCount(); count != 1 {
		t.Errorf("expected 1 client after unsubscribing ghost, got %d", count)
	}
}

func TestEventBus_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	bus := NewEventBus(eventBusMaxClients)

	var wg sync.WaitGroup
	const goroutines = 20

	// Concurrently subscribe and unsubscribe
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clientID := string(rune('a' + id))
			ch, ok := bus.Subscribe(clientID)
			if ok {
				// Read at least one event if possible
				time.Sleep(time.Millisecond)
				bus.Unsubscribe(clientID)
				_ = ch
			}
		}(i)
	}

	// Concurrent publishers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				bus.Publish(SystemEvent{Type: "concurrent_sub", Payload: map[string]any{"pub": id}})
			}
		}(i)
	}

	wg.Wait()

	// After all goroutines finish, client count should be 0
	if count := bus.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after all unsubscribes, got %d", count)
	}
}

func TestEventBus_ClientCountConsistency(t *testing.T) {
	bus := &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: eventBusMaxClients,
	}

	// Subscribe 5 clients
	for i := 0; i < 5; i++ {
		bus.Subscribe(string(rune('a' + i)))
	}

	if count := bus.ClientCount(); count != 5 {
		t.Errorf("expected 5 clients, got %d", count)
	}

	// Unsubscribe 2
	bus.Unsubscribe("a")
	bus.Unsubscribe("b")

	if count := bus.ClientCount(); count != 3 {
		t.Errorf("expected 3 clients, got %d", count)
	}

	// Subscribe one more
	bus.Subscribe("z")

	if count := bus.ClientCount(); count != 4 {
		t.Errorf("expected 4 clients, got %d", count)
	}
}

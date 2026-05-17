package service

import (
	"sync"
	"sync/atomic"
)

// SystemEvent represents a lightweight state-change notification.
// Payloads are minimal — clients fetch full data via REST if needed.
type SystemEvent struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// EventBus is a simple fan-out pub/sub for SystemEvent.
// Uses mutex + map of buffered channels (same pattern as FileWatcher).
type EventBus struct {
	mu          sync.RWMutex
	clients     map[string]chan SystemEvent
	maxClients  int
	clientCount atomic.Int32
}

const (
	eventBusChannelBuf = 256 // match sessionStreams buffer size
	eventBusMaxClients  = 20  // soft limit to prevent resource exhaustion
)

// GlobalEventBus is the singleton event bus instance.
var GlobalEventBus = NewEventBus(eventBusMaxClients)

// NewEventBus creates an EventBus with the given max client limit.
func NewEventBus(maxClients int) *EventBus {
	return &EventBus{
		clients:    make(map[string]chan SystemEvent),
		maxClients: maxClients,
	}
}

// Subscribe registers a client and returns a buffered channel for receiving events.
// Returns false if the maximum number of subscribers has been reached.
// If the clientID already exists, the old channel is closed and replaced (no count increment).
func (b *EventBus) Subscribe(clientID string) (<-chan SystemEvent, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.clients) >= b.maxClients {
		// Allow re-subscribe with same ID (doesn't increase count)
		if _, exists := b.clients[clientID]; !exists {
			return nil, false
		}
	}

	if oldCh, exists := b.clients[clientID]; exists {
		// Close old channel to signal the old subscriber to stop reading
		close(oldCh)
		// Don't increment count — we're replacing, not adding
	} else {
		b.clientCount.Add(1)
	}

	ch := make(chan SystemEvent, eventBusChannelBuf)
	b.clients[clientID] = ch
	return ch, true
}

// Unsubscribe removes a client and closes its channel.
// This signals the SSE handler's select loop to exit (ok = false on receive).
func (b *EventBus) Unsubscribe(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.clients[clientID]; ok {
		delete(b.clients, clientID)
		b.clientCount.Add(-1)
		close(ch)
	}
}

// Publish sends an event to all subscribed clients (non-blocking).
// Drops the event if a client's channel is full — full-state sync on
// reconnect guarantees eventual consistency.
func (b *EventBus) Publish(event SystemEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.clients {
		select {
		case ch <- event:
		default:
			// Channel full — client will catch up via full-state sync
		}
	}
}

// ClientCount returns the current number of subscribers (for monitoring).
func (b *EventBus) ClientCount() int {
	return int(b.clientCount.Load())
}

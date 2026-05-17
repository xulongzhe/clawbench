package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	"clawbench/internal/push"
)

// ClientSubscription tracks a single client's WS connection and push state.
type ClientSubscription struct {
	mu          sync.Mutex
	conn        *websocket.Conn
	WriteMu     *sync.Mutex // shared with EventsHandler for serialized writes
	pushRegID   string
	lastActive  time.Time
	eventBuffer []ServerMessage
	bufferStart time.Time
}

// Manager manages all client subscriptions.
type Manager struct {
	mu            sync.Mutex
	subscriptions map[string]*ClientSubscription // keyed by auth identity
	jpush         *push.JPushClient
}

var defaultManager *Manager

func InitManager(jpushClient *push.JPushClient) {
	defaultManager = &Manager{
		subscriptions: make(map[string]*ClientSubscription),
		jpush:        jpushClient,
	}
}

func GetManager() *Manager {
	return defaultManager
}

// clientKey returns a unique key for the authenticated client.
// Since ClawBench is single-user, we use a fixed key.
func clientKey() string {
	return "default"
}

// Subscribe registers a new WS connection for the client.
func (m *Manager) Subscribe(conn *websocket.Conn, writeMu *sync.Mutex) *ClientSubscription {
	key := clientKey()
	m.mu.Lock()
	defer m.mu.Unlock()

	sub, ok := m.subscriptions[key]
	if !ok {
		sub = &ClientSubscription{}
		m.subscriptions[key] = sub
	}

	sub.mu.Lock()
	// Close existing connection if any
	if sub.conn != nil {
		sub.conn.Close(websocket.StatusNormalClosure, "replaced")
	}
	sub.conn = conn
	sub.WriteMu = writeMu
	sub.lastActive = time.Now()
	sub.eventBuffer = nil
	sub.bufferStart = time.Time{}
	sub.mu.Unlock()

	slog.Debug("ws: client subscribed")
	return sub
}

// Unsubscribe handles WS disconnection.
func (m *Manager) Unsubscribe() {
	key := clientKey()
	m.mu.Lock()
	defer m.mu.Unlock()

	sub, ok := m.subscriptions[key]
	if !ok {
		return
	}

	sub.mu.Lock()
	sub.conn = nil
	sub.WriteMu = nil
	sub.bufferStart = time.Now() // start buffer window
	sub.mu.Unlock()

	slog.Debug("ws: client unsubscribed")
}

// RegisterPushID stores the JPush registration ID for fallback push notifications.
// Called via HTTP POST /api/push/register (login-level lifecycle, not per-WS-connection).
// Creates a subscription entry if one doesn't exist yet (e.g., HTTP call before WS connect).
func (m *Manager) RegisterPushID(regID string) {
	key := clientKey()
	m.mu.Lock()
	defer m.mu.Unlock()

	sub, ok := m.subscriptions[key]
	if !ok {
		sub = &ClientSubscription{}
		m.subscriptions[key] = sub
	}
	sub.mu.Lock()
	sub.pushRegID = regID
	sub.mu.Unlock()

	slog.Debug("ws: registered push ID", "reg_id", regID)
}

// BroadcastEvent sends an event to the connected client, or buffers it / sends JPush.
func (m *Manager) BroadcastEvent(msg ServerMessage) {
	key := clientKey()
	m.mu.Lock()
	sub, ok := m.subscriptions[key]
	if !ok {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	sub.mu.Lock()
	conn := sub.conn
	writeMu := sub.WriteMu
	pushRegID := sub.pushRegID

	if conn != nil && writeMu != nil {
		// Client is connected — send via WS (serialized with writeMu)
		data, err := json.Marshal(msg)
		if err != nil {
			slog.Error("ws: marshal event", "error", err)
			sub.mu.Unlock()
			return
		}
		writeMu.Lock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
			slog.Warn("ws: failed to send event, client may be disconnected", "error", err)
		}
		cancel()
		writeMu.Unlock()
		// Buffer event for reconnect replay
		sub.bufferEvent(msg)
		sub.mu.Unlock()
		return
	}

	// Client is disconnected — check buffer window and send JPush
	if sub.bufferStart.IsZero() || time.Since(sub.bufferStart) < 10*time.Second {
		// Within buffer window — buffer the event in case client reconnects soon
		sub.bufferEvent(msg)
	}

	// Send JPush if we have a registration ID
	if pushRegID != "" && m.jpush != nil && m.jpush.Enabled() {
		sub.mu.Unlock() // unlock before potentially slow network call
		extras := map[string]string{"event_type": msg.Event}
		// Extract session_id or task_id from data
		switch d := msg.Data.(type) {
		case *SessionUpdateData:
			extras["session_id"] = d.SessionID
		case *TaskUpdateData:
			extras["task_id"] = d.TaskID
		}
		title := "AI任务完成"
		alert := "AI会话已结束"
		if msg.Event == "task_update" {
			alert = "计划任务已完成"
		}
		if err := m.jpush.SendNotification(pushRegID, title, alert, extras); err != nil {
			slog.Warn("ws: jpush notification failed", "error", err)
		}
		return
	}

	sub.mu.Unlock()
}

// GetBufferedEvents returns buffered events for replay on reconnect.
func (s *ClientSubscription) GetBufferedEvents() []ServerMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]ServerMessage, len(s.eventBuffer))
	copy(result, s.eventBuffer)
	return result
}

// bufferEvent appends an event to the replay buffer, keeping at most 50 events.
func (s *ClientSubscription) bufferEvent(msg ServerMessage) {
	s.eventBuffer = append(s.eventBuffer, msg)
	if len(s.eventBuffer) > 50 {
		s.eventBuffer = s.eventBuffer[len(s.eventBuffer)-50:]
	}
}

// CleanupStale removes subscriptions disconnected for over 30 minutes.
func (m *Manager) CleanupStale() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, sub := range m.subscriptions {
		sub.mu.Lock()
		if sub.conn == nil && !sub.bufferStart.IsZero() && time.Since(sub.bufferStart) > 30*time.Minute {
			delete(m.subscriptions, key)
			slog.Debug("ws: cleaned up stale subscription", "key", key)
		}
		sub.mu.Unlock()
	}
}

// GenerateEventID creates a unique event ID.
func GenerateEventID() string {
	return fmt.Sprintf("evt_%d_%d", time.Now().UnixMilli(), time.Now().Nanosecond()%1000)
}

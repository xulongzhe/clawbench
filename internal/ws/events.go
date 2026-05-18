package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// EventsHandler handles the /api/ai/events/ws WebSocket endpoint.
// Auth is handled by middleware.Auth before this function is called.
func EventsHandler(w http.ResponseWriter, r *http.Request) {
	mgr := GetManager()
	if mgr == nil {
		http.Error(w, "events not initialized", http.StatusServiceUnavailable)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{
			"http://" + r.Host,
			"https://" + r.Host,
			"http://localhost:*",
			"https://localhost:*",
			"http://127.0.0.1:*",
			"https://127.0.0.1:*",
		},
	})
	if err != nil {
		slog.Error("ws: accept failed", "error", err)
		return
	}

	var writeMu sync.Mutex
	sub := mgr.Subscribe(conn, &writeMu)
	defer mgr.Unsubscribe()

	// Replay buffered events on reconnect
	buffered := sub.GetBufferedEvents()
	if len(buffered) > 0 {
		slog.Debug("ws: replaying buffered events", "count", len(buffered))
		for _, msg := range buffered {
			data, _ := json.Marshal(msg)
			writeMu.Lock()
			ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			conn.Write(ctx2, websocket.MessageText, data)
			cancel()
			writeMu.Unlock()
		}
	}

	// Ping ticker
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Ping goroutine
	go func() {
		for range pingTicker.C {
			writeMu.Lock()
			pingData, _ := json.Marshal(ServerMessage{Type: "ping"})
			ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := conn.Write(ctx2, websocket.MessageText, pingData)
			cancel()
			writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	// Read client messages (blocks until disconnect)
	ctx := context.Background()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			slog.Debug("ws: client disconnected", "error", err)
			break
		}

		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Warn("ws: invalid client message", "error", err)
			continue
		}

		switch msg.Type {
		case "ack":
			slog.Debug("ws: ack received", "id", msg.ID)
		case "pong":
			// Connection alive
		default:
			slog.Warn("ws: unknown client message type", "type", msg.Type)
		}
	}

	conn.Close(websocket.StatusNormalClosure, "handler exiting")
}

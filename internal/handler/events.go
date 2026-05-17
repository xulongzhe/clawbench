package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"clawbench/internal/service"
)

const eventsHeartbeatSec = 15

// newEventsClientID generates a random client ID for system events SSE connections.
func newEventsClientID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// SystemEventsSSE handles GET /api/events — SSE stream for system state changes.
// Pushes lightweight events (session/task/tunnel state changes) to connected clients.
// Clients use full-state REST sync on reconnect to catch up on missed events.
func SystemEventsSSE(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	_, ok := requireProject(w, r)
	if !ok {
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Prevent token leakage via Referrer header (review issue #2)
	w.Header().Set("Referrer-Policy", "no-referrer")

	flusher, canFlush := w.(http.Flusher)

	// Subscribe to event bus
	clientID := newEventsClientID()
	pushCh, ok := service.GlobalEventBus.Subscribe(clientID)
	if !ok {
		// Max subscribers reached
		writeLocalizedErrorf(w, r, http.StatusTooManyRequests, "TooManyConnections")
		return
	}
	// CRITICAL: defer Unsubscribe to prevent channel leak on handler exit
	// (review issue #3 — matches FileWatchSSE pattern at file_watch.go:84)
	defer service.GlobalEventBus.Unsubscribe(clientID)

	// Send connected event with clientId
	connectedData, _ := json.Marshal(map[string]string{"clientId": clientID})
	fmt.Fprintf(w, "event: connected\ndata: %s\n\n", connectedData)
	if canFlush {
		flusher.Flush()
	}

	// Heartbeat ticker — keeps connection alive through proxies/mobile networks
	heartbeat := time.NewTicker(eventsHeartbeatSec * time.Second)
	defer heartbeat.Stop()

	slog.Debug("system events SSE connected", slog.String("clientId", clientID))

	for {
		select {
		case event, ok := <-pushCh:
			if !ok {
				// Channel closed by Unsubscribe — clean exit
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				slog.Warn("failed to marshal system event", slog.String("err", err.Error()))
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if canFlush {
				flusher.Flush()
			}

		case <-heartbeat.C:
			// SSE comment lines keep the TCP connection alive through
			// reverse proxies and mobile networks (review issue #14)
			fmt.Fprintf(w, ": heartbeat %d\n\n", time.Now().UnixMilli())
			if canFlush {
				flusher.Flush()
			}

		case <-r.Context().Done():
			slog.Debug("system events SSE disconnected", slog.String("clientId", clientID))
			return
		}
	}
}

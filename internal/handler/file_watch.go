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

const watchHeartbeatSec = 30

// fileWatchUpdateRequest is the body for PUT /api/file/watch/update
type fileWatchUpdateRequest struct {
	ClientID string `json:"clientId"`
	DirPath  string `json:"dir"`
	FilePath string `json:"file"`
}

// newWatchClientID generates a random client ID for file watch SSE connections.
func newWatchClientID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// FileWatchSSE handles GET /api/file/watch — SSE stream for file change notifications.
// Query params: dir (required, relative path), file (optional, relative path).
func FileWatchSSE(w http.ResponseWriter, r *http.Request) { //nolint:gocyclo // SSE with filesystem watcher lifecycle
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	fw := service.GlobalFileWatcher
	if fw == nil {
		writeLocalizedErrorf(w, r, http.StatusServiceUnavailable, "FileWatcherNotAvailable")
		return
	}

	dirRel := r.URL.Query().Get("dir")
	fileRel := r.URL.Query().Get("file")

	// Resolve and validate paths
	// Empty dirRel means the project root — watch the project path itself
	var dirAbs, fileAbs string
	if dirRel == "" {
		dirAbs = projectPath
	} else {
		abs, ok := validateAndResolvePath(w, r, projectPath, dirRel)
		if !ok {
			return
		}
		dirAbs = abs
	}
	if fileRel != "" {
		abs, ok := validateAndResolvePath(w, r, projectPath, fileRel)
		if !ok {
			return
		}
		fileAbs = abs
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, canFlush := w.(http.Flusher)

	// Register client
	clientID := newWatchClientID()
	pushCh := fw.RegisterClient(clientID)
	defer fw.UnregisterClient(clientID)

	// Set initial watch paths
	fw.UpdateWatch(clientID, dirAbs, fileAbs)

	// Send connected event with clientID
	data, _ := json.Marshal(map[string]string{"clientId": clientID})
	_, _ = fmt.Fprintf(w, "event: connected\ndata: %s\n\n", data)
	if canFlush {
		flusher.Flush()
	}

	// Heartbeat ticker
	heartbeat := time.NewTicker(watchHeartbeatSec * time.Second)
	defer heartbeat.Stop()

	slog.Debug(
		"file watch SSE connected",
		slog.String("clientId", clientID),
		slog.String("dir", dirRel),
		slog.String("file", fileRel),
	)

	for {
		select {
		case event, ok := <-pushCh:
			if !ok {
				// Channel closed — watcher shutting down
				return
			}
			data, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if canFlush {
				flusher.Flush()
			}

		case <-heartbeat.C:
			_, _ = fmt.Fprintf(w, "event: heartbeat\ndata: {}\n\n")
			if canFlush {
				flusher.Flush()
			}

		case <-r.Context().Done():
			slog.Debug(
				"file watch SSE disconnected",
				slog.String("clientId", clientID),
			)
			return
		}
	}
}

// FileWatchUpdate handles PUT /api/file/watch/update — update watched paths.
func FileWatchUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPut) {
		return
	}

	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	slog.Debug(
		"file watch update received",
		slog.String("projectPath", projectPath),
	)

	fw := service.GlobalFileWatcher
	if fw == nil {
		writeLocalizedErrorf(w, r, http.StatusServiceUnavailable, "FileWatcherNotAvailable")
		return
	}

	var req fileWatchUpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.ClientID == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "ClientIdRequired")
		return
	}

	// Resolve and validate paths
	// Empty dirPath means the project root — watch the project path itself
	var dirAbs, fileAbs string
	if req.DirPath == "" {
		dirAbs = projectPath
	} else {
		abs, ok := validateAndResolvePath(w, r, projectPath, req.DirPath)
		if !ok {
			return
		}
		dirAbs = abs
	}
	if req.FilePath != "" {
		abs, ok := validateAndResolvePath(w, r, projectPath, req.FilePath)
		if !ok {
			return
		}
		fileAbs = abs
	}

	fw.UpdateWatch(req.ClientID, dirAbs, fileAbs)

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

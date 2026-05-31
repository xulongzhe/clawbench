package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clawbench/internal/model"
)

// androidLogMu protects concurrent writes to the android log file.
var androidLogMu sync.Mutex

// AndroidLogEntry represents a single log entry from the Android app.
type AndroidLogEntry struct {
	Level string `json:"level"` // D, I, W, E
	Tag   string `json:"tag"`
	Msg   string `json:"msg"`
	Ts    int64  `json:"ts"` // epoch millis
}

// androidLogRequest is the request body for POST /api/android-log.
type androidLogRequest struct {
	Entries []AndroidLogEntry `json:"entries"`
}

// androidLogFilePath returns the path to the android log file.
func androidLogFilePath() string {
	return filepath.Join(model.ConfigInstance.LogDir, "android.log")
}

// ServeAndroidLog handles POST /api/android-log.
// It receives batched log entries from the Android app and appends them
// to .clawbench/logs/android.log in a human-readable format.
func ServeAndroidLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
		return
	}

	var req androidLogRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if len(req.Entries) == 0 {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRequestBody")
		return
	}

	// Cap at 200 entries per request
	if len(req.Entries) > 200 {
		req.Entries = req.Entries[:200]
	}

	// Format entries (one line per entry; escape newlines in messages)
	lines := make([]byte, 0, len(req.Entries)*128)
	for _, e := range req.Entries {
		t := time.UnixMilli(e.Ts)
		msg := strings.ReplaceAll(e.Msg, "\n", "\\n")
		line := fmt.Sprintf(
			"%s %s/%s: %s\n",
			t.Format("2006-01-02T15:04:05.000"),
			e.Level,
			e.Tag,
			msg,
		)
		lines = append(lines, line...)
	}

	// Append to file (mutex-protected)
	androidLogMu.Lock()
	defer androidLogMu.Unlock()

	path := androidLogFilePath()
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("create log dir: %w", err)))
		return
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644) //nolint:gosec // log file, not security-sensitive
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("open android log: %w", err)))
		return
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(lines); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("write android log: %w", err)))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"written": len(req.Entries)})
}

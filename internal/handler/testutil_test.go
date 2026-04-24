package handler

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"clawbench/internal/middleware"
	"clawbench/internal/model"
	"clawbench/internal/service"

	_ "modernc.org/sqlite"
)

// testEnv holds test environment state for setup/teardown.
type testEnv struct {
	ProjectDir string
	WatchDir   string
	OrigToken  string
	OrigWatch  string
	OrigDB     *sql.DB
	OrigDev    bool
}

// setupTestEnv creates a temporary project directory, initializes an in-memory DB,
// and saves/restores global state. Returns env and teardown function.
func setupTestEnv(t *testing.T) (*testEnv, func()) {
	t.Helper()

	// Create temp directories
	projectDir := t.TempDir()
	watchDir := t.TempDir()

	// Save original globals
	origToken := model.SessionToken
	origWatch := model.WatchDir
	origDB := service.DB
	origDev := model.DevMode

	// Set test globals
	model.SessionToken = ""
	model.WatchDir = watchDir
	model.DevMode = false

	// Init in-memory SQLite
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_path TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
			content TEXT NOT NULL,
			file_path TEXT,
			files TEXT,
			session_id TEXT,
			backend TEXT NOT NULL DEFAULT 'claude',
			streaming INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS chat_sessions (
			id TEXT PRIMARY KEY,
			project_path TEXT NOT NULL,
			backend TEXT NOT NULL,
			title TEXT NOT NULL,
			agent_id TEXT DEFAULT '',
			model TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_read_at DATETIME,
			UNIQUE(project_path, backend, id)
		);
		CREATE TABLE IF NOT EXISTS recent_projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_path TEXT UNIQUE NOT NULL,
			accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id TEXT PRIMARY KEY,
			project_path TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			cron_expr TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			session_id TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			repeat_mode TEXT NOT NULL DEFAULT 'always',
			max_runs INTEGER DEFAULT 0,
			last_run_at DATETIME,
			next_run_at DATETIME,
			run_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS task_executions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL,
			message_id INTEGER NOT NULL REFERENCES chat_history(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_executions_task ON task_executions(task_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_history_session ON chat_history(project_path, backend, session_id, created_at);
		CREATE INDEX IF NOT EXISTS idx_sessions_project_backend ON chat_sessions(project_path, backend);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	service.DB = db

	env := &testEnv{
		ProjectDir: projectDir,
		WatchDir:   watchDir,
		OrigToken:  origToken,
		OrigWatch:  origWatch,
		OrigDB:     origDB,
		OrigDev:    origDev,
	}

	teardown := func() {
		model.SessionToken = origToken
		model.WatchDir = origWatch
		model.DevMode = origDev
		service.DB = origDB
		db.Close()
	}

	return env, teardown
}

// newRequest creates an HTTP request with optional body.
func newRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// withProjectCookie adds the clawbench_project cookie to the request.
func withProjectCookie(req *http.Request, projectPath string) *http.Request {
	req.AddCookie(&http.Cookie{
		Name:  "clawbench_project",
		Value: url.QueryEscape(projectPath),
	})
	return req
}

// withAuthCookie adds the clawbench_session cookie to the request.
func withAuthCookie(req *http.Request, token string) *http.Request {
	req.AddCookie(&http.Cookie{
		Name:  model.SessionCookie,
		Value: token,
	})
	return req
}

// withSessionCookie adds the chat_session_id cookie to the request.
func withSessionCookie(req *http.Request, sessionID string) *http.Request {
	req.AddCookie(&http.Cookie{
		Name:  "chat_session_id",
		Value: sessionID,
	})
	return req
}

// hashPassword computes the SHA-256 hash with salt, matching the auth logic.
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password + "clawbench-salt"))
	return hex.EncodeToString(hash[:])
}

// decodeJSON decodes the response body into target.
func decodeJSON(t *testing.T, body io.Reader, target interface{}) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(target); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
}

// callHandler calls a handler function with the given request and returns the response recorder.
func callHandler(handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

// callHandlerWithAuth calls a handler wrapped with Auth middleware.
func callHandlerWithAuth(handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	middleware.Auth(handler)(w, req)
	return w
}

// createTestFile creates a file with the given content in the project directory.
func createTestFile(t *testing.T, projectDir, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(projectDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

// assertOK asserts that the response status code is 200.
func assertOK(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// assertStatus asserts the response status code.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("expected status %d, got %d; body: %s", expected, w.Code, w.Body.String())
	}
}

// assertJSONField asserts a specific field value in the JSON response.
func assertJSONField(t *testing.T, w *httptest.ResponseRecorder, field string, expected interface{}) {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v; body: %s", err, w.Body.String())
	}
	val, ok := result[field]
	if !ok {
		t.Errorf("field %q not found in response; body: %s", field, w.Body.String())
		return
	}
	if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", expected) {
		t.Errorf("field %q: expected %v, got %v", field, expected, val)
	}
}

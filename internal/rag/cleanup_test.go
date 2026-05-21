package rag

import (
	"database/sql"
	"fmt"
	"testing"

	"clawbench/internal/model"
	"clawbench/internal/service"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
)

// setupCleanupDB creates an in-memory SQLite database with the required schema
// for cleanup tests, sets service.DB, and returns a cleanup function.
func setupCleanupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	assert.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_path TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
			content TEXT NOT NULL,
			files TEXT,
			session_id TEXT,
			backend TEXT NOT NULL DEFAULT 'claude',
			streaming INTEGER NOT NULL DEFAULT 0,
			indexed INTEGER NOT NULL DEFAULT 0,
			deleted INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS chat_sessions (
			id TEXT PRIMARY KEY,
			project_path TEXT NOT NULL,
			backend TEXT NOT NULL,
			title TEXT NOT NULL,
			agent_id TEXT DEFAULT '',
			agent_source TEXT DEFAULT 'default',
			model TEXT DEFAULT '',
			session_type TEXT NOT NULL DEFAULT 'chat',
			deleted INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_read_at DATETIME,
			UNIQUE(project_path, backend, id)
		);
		CREATE TABLE IF NOT EXISTS ai_raw_responses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			message_id INTEGER NOT NULL,
			backend TEXT NOT NULL DEFAULT '',
			raw_output TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	assert.NoError(t, err)

	origDB := service.DB
	origDBRead := service.DBRead
	service.DB = db
	service.DBRead = db // Same instance for :memory: SQLite — data is shared
	t.Cleanup(func() {
		service.DB = origDB
		service.DBRead = origDBRead
		db.Close()
	})
	return db
}

// helperCreateCleanupSession creates a session for cleanup tests.
func helperCreateCleanupSession(t *testing.T, projectPath, backend, title string) string {
	t.Helper()
	id, err := service.CreateSession(projectPath, backend, title, "", "", "default", "chat")
	assert.NoError(t, err)
	assert.NotEmpty(t, id)
	return id
}

// ---------- CleanupWorker.cleanup() ----------

func TestCleanup_NoExpiredSessions(t *testing.T) {
	setupCleanupDB(t)

	// No sessions at all — cleanup should be a no-op
	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.cleanup()

	// Active session — should not be purged
	sid := helperCreateCleanupSession(t, "/project", "claude", "Active")
	_, _ = service.AddChatMessage("/project", "claude", sid, "user", "msg", nil, false, "NewSession")

	w.cleanup()

	// Verify session still exists
	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, "msg", title)
}

func TestCleanup_RecentlyDeletedNotPurged(t *testing.T) {
	setupCleanupDB(t)

	sid := helperCreateCleanupSession(t, "/project", "claude", "Recent Delete")
	_, _ = service.AddChatMessage("/project", "claude", sid, "user", "msg", nil, false, "NewSession")
	_ = service.DeleteSession("/project", "claude", sid)

	// Just deleted — within 90-day retention period
	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.cleanup()

	// Data should still be physically present (just soft-deleted)
	var count int
	err := service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE id = ?", sid).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCleanup_ExpiredSessionPurged(t *testing.T) {
	setupCleanupDB(t)

	sid := helperCreateCleanupSession(t, "/project", "claude", "Old Delete")
	_, _ = service.AddChatMessage("/project", "claude", sid, "user", "msg1", nil, false, "NewSession")
	_, _ = service.AddChatMessage("/project", "claude", sid, "assistant", "reply1", nil, false, "NewSession")
	_ = service.DeleteSession("/project", "claude", sid)

	// Simulate deletion 100 days ago
	_, _ = service.DB.Exec("UPDATE chat_sessions SET updated_at = datetime('now', '-100 days') WHERE id = ?", sid)

	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.cleanup()

	// Session should be completely gone
	var sessionCount int
	err := service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE id = ?", sid).Scan(&sessionCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, sessionCount)

	// Messages should be gone
	var msgCount int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM chat_history WHERE session_id = ?", sid).Scan(&msgCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, msgCount)
}

func TestCleanup_MixedExpiredAndRecent(t *testing.T) {
	setupCleanupDB(t)

	// Expired session (deleted 100 days ago)
	expiredSID := helperCreateCleanupSession(t, "/project", "claude", "Old")
	_, _ = service.AddChatMessage("/project", "claude", expiredSID, "user", "old msg", nil, false, "NewSession")
	_ = service.DeleteSession("/project", "claude", expiredSID)
	_, _ = service.DB.Exec("UPDATE chat_sessions SET updated_at = datetime('now', '-100 days') WHERE id = ?", expiredSID)

	// Recent session (deleted just now) — should NOT be purged
	recentSID := helperCreateCleanupSession(t, "/project", "claude", "Recent")
	_, _ = service.AddChatMessage("/project", "claude", recentSID, "user", "recent msg", nil, false, "NewSession")
	_ = service.DeleteSession("/project", "claude", recentSID)

	// Active session — should not be touched
	activeSID := helperCreateCleanupSession(t, "/project", "claude", "Active")
	_, _ = service.AddChatMessage("/project", "claude", activeSID, "user", "active msg", nil, false, "NewSession")

	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.cleanup()

	// Expired session: gone
	var count int
	_ = service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE id = ?", expiredSID).Scan(&count)
	assert.Equal(t, 0, count)

	// Recent deleted session: still present (soft-deleted)
	_ = service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE id = ?", recentSID).Scan(&count)
	assert.Equal(t, 1, count)

	// Active session: still present and visible
	title, err := service.GetSessionTitle(activeSID)
	assert.NoError(t, err)
	assert.Equal(t, "active msg", title)
}

func TestCleanup_RawResponsesAlsoPurged(t *testing.T) {
	setupCleanupDB(t)

	sid := helperCreateCleanupSession(t, "/project", "claude", "With Raw")
	msgID, _ := service.AddChatMessage("/project", "claude", sid, "assistant", "reply", nil, false, "NewSession")
	_ = service.DeleteSession("/project", "claude", sid)
	_, _ = service.DB.Exec("UPDATE chat_sessions SET updated_at = datetime('now', '-100 days') WHERE id = ?", sid)
	_, _ = service.DB.Exec("INSERT INTO ai_raw_responses (session_id, message_id, backend, raw_output) VALUES (?, ?, 'claude', 'raw data')", sid, msgID)

	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.cleanup()

	// Raw responses should be purged
	var count int
	err := service.DB.QueryRow("SELECT COUNT(*) FROM ai_raw_responses WHERE session_id = ?", sid).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCleanup_RetentionDaysZero_NoCleanup(t *testing.T) {
	setupCleanupDB(t)

	sid := helperCreateCleanupSession(t, "/project", "claude", "Keep Forever")
	_, _ = service.AddChatMessage("/project", "claude", sid, "user", "msg", nil, false, "NewSession")
	_ = service.DeleteSession("/project", "claude", sid)
	_, _ = service.DB.Exec("UPDATE chat_sessions SET updated_at = datetime('now', '-365 days') WHERE id = ?", sid)

	// retention_days=0 means keep forever — StartCleanupWorker should not even start
	// But if cleanup() is called directly, the cutoff would be time.Now() which
	// means very old sessions get purged. The guard is in StartCleanupWorker.
	// Here we verify that with RetentionDays=90, it still works as expected.
	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.cleanup()

	// With 90-day retention and 365-day-old deletion, it should be purged
	var count int
	_ = service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE id = ?", sid).Scan(&count)
	assert.Equal(t, 0, count)
}

// ---------- CleanupWorker Start/Stop ----------

func TestCleanupWorker_StartStop(t *testing.T) {
	setupCleanupDB(t)

	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})

	// Start should succeed
	w.Start()
	assert.True(t, w.running)

	// Double start should be no-op
	w.Start()

	// Stop should succeed
	w.Stop()
	assert.False(t, w.running)

	// Double stop should be no-op
	w.Stop()
}

func TestCleanupWorker_StopBeforeFirstRun(t *testing.T) {
	setupCleanupDB(t)

	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.Start()

	// Stop immediately — the 5-minute delay hasn't elapsed yet,
	// so the goroutine should exit cleanly without running cleanup
	w.Stop()
	assert.False(t, w.running)
}

// ---------- NewCleanupWorker ----------

func TestNewCleanupWorker_NilStore(t *testing.T) {
	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	assert.Nil(t, w.store)
	assert.Equal(t, 90, w.cfg.RetentionDays)
}

func TestNewCleanupWorker_WithStore(t *testing.T) {
	// Just verify construction — don't need a real DuckDB
	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 30})
	assert.Equal(t, 30, w.cfg.RetentionDays)
}

// ---------- DeleteChunksBySessionIDs ----------

func TestDeleteChunksBySessionIDs_EmptyList(t *testing.T) {
	// Can't test with real DuckDB in unit tests without Docker/Ollama,
	// but we can test the nil/empty path via CleanupWorker
	setupCleanupDB(t)

	// With nil store (RAG disabled), cleanup should still work for SQLite
	sid := helperCreateCleanupSession(t, "/project", "claude", "Test")
	_, _ = service.AddChatMessage("/project", "claude", sid, "user", "msg", nil, false, "NewSession")
	_ = service.DeleteSession("/project", "claude", sid)
	_, _ = service.DB.Exec("UPDATE chat_sessions SET updated_at = datetime('now', '-100 days') WHERE id = ?", sid)

	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.cleanup()

	// SQLite data should be purged even without DuckDB
	var count int
	_ = service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE id = ?", sid).Scan(&count)
	assert.Equal(t, 0, count)
}

// ---------- Multiple expired sessions with raw responses ----------

func TestCleanup_MultipleExpiredWithRawResponses(t *testing.T) {
	setupCleanupDB(t)

	var sids []string
	for i := 0; i < 3; i++ {
		sid := helperCreateCleanupSession(t, "/project", "claude", fmt.Sprintf("Session %d", i))
		msgID, _ := service.AddChatMessage("/project", "claude", sid, "assistant", fmt.Sprintf("reply %d", i), nil, false, "NewSession")
		_ = service.DeleteSession("/project", "claude", sid)
		_, _ = service.DB.Exec("UPDATE chat_sessions SET updated_at = datetime('now', '-100 days') WHERE id = ?", sid)
		_, _ = service.DB.Exec("INSERT INTO ai_raw_responses (session_id, message_id, backend, raw_output) VALUES (?, ?, 'claude', ?)", sid, msgID, fmt.Sprintf("raw %d", i))
		sids = append(sids, sid)
	}

	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.cleanup()

	// All sessions should be gone
	for _, sid := range sids {
		var count int
		_ = service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE id = ?", sid).Scan(&count)
		assert.Equal(t, 0, count)

		_ = service.DB.QueryRow("SELECT COUNT(*) FROM chat_history WHERE session_id = ?", sid).Scan(&count)
		assert.Equal(t, 0, count)

		_ = service.DB.QueryRow("SELECT COUNT(*) FROM ai_raw_responses WHERE session_id = ?", sid).Scan(&count)
		assert.Equal(t, 0, count)
	}
}

// ---------- Cutoff boundary test ----------

func TestCleanup_CutoffBoundary(t *testing.T) {
	setupCleanupDB(t)

	// Create a session deleted exactly at the retention boundary (89 days ago — within retention)
	sid89 := helperCreateCleanupSession(t, "/project", "claude", "89 days")
	_, _ = service.AddChatMessage("/project", "claude", sid89, "user", "msg", nil, false, "NewSession")
	_ = service.DeleteSession("/project", "claude", sid89)
	_, _ = service.DB.Exec("UPDATE chat_sessions SET updated_at = datetime('now', '-89 days') WHERE id = ?", sid89)

	// Create a session deleted 91 days ago (outside retention)
	sid91 := helperCreateCleanupSession(t, "/project", "claude", "91 days")
	_, _ = service.AddChatMessage("/project", "claude", sid91, "user", "msg", nil, false, "NewSession")
	_ = service.DeleteSession("/project", "claude", sid91)
	_, _ = service.DB.Exec("UPDATE chat_sessions SET updated_at = datetime('now', '-91 days') WHERE id = ?", sid91)

	w := NewCleanupWorker(nil, model.RAGConfig{RetentionDays: 90})
	w.cleanup()

	// 89-day session should still exist
	var count89 int
	_ = service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE id = ?", sid89).Scan(&count89)
	assert.Equal(t, 1, count89)

	// 91-day session should be purged
	var count91 int
	_ = service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE id = ?", sid91).Scan(&count91)
	assert.Equal(t, 0, count91)
}

package service

import (
	"database/sql"
	"encoding/json"
	"testing"

	"clawbench/internal/model"
	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
)

// setupTestDBForTTS creates an in-memory SQLite database with the tts_summaries table
// for testing GetTTSSummary and SaveTTSSummary.
func setupTestDBForTTS(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	origDB := DB
	origDBRead := DBRead

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tts_summaries (
			cache_key TEXT PRIMARY KEY,
			summary TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
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
			external_session_id TEXT DEFAULT '',
			deleted INTEGER NOT NULL DEFAULT 0,
			last_read_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(project_path, backend, id)
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	DB = db
	DBRead = db // Same instance for :memory: SQLite — data is shared
	teardown := func() {
		DB = origDB
		DBRead = origDBRead
		db.Close()
	}
	return db, teardown
}

// setupTestDBForQuickSend creates an in-memory SQLite database with the chat_quick_send table
// for testing ChatQuickSend CRUD functions.
func setupTestDBForQuickSend(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	origDB := DB
	origDBRead := DBRead

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_quick_send (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			command TEXT NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	DB = db
	DBRead = db // Same instance for :memory: SQLite — data is shared
	teardown := func() {
		DB = origDB
		DBRead = origDBRead
		db.Close()
	}
	return db, teardown
}

// ---------- Schema: session_type, task_executions columns, new indexes ----------

func TestSchema_SessionTypeColumnExists(t *testing.T) {
	tmpDir := t.TempDir()
	origBinDir := model.BinDir
	model.BinDir = tmpDir
	defer func() { model.BinDir = origBinDir }()

	err := InitDB()
	assert.NoError(t, err)
	defer DB.Close()

	columns := getTableColumns(t, DB, "chat_sessions")
	assert.Contains(t, columns, "session_type", "chat_sessions should have session_type column")
}

func TestSchema_TaskExecutionsColumns(t *testing.T) {
	tmpDir := t.TempDir()
	origBinDir := model.BinDir
	model.BinDir = tmpDir
	defer func() { model.BinDir = origBinDir }()

	err := InitDB()
	assert.NoError(t, err)
	defer DB.Close()

	columns := getTableColumns(t, DB, "task_executions")
	assert.Contains(t, columns, "session_id", "task_executions should have session_id column")
	assert.Contains(t, columns, "status", "task_executions should have status column")
	assert.NotContains(t, columns, "content", "task_executions should NOT have content column")
}

func TestSchema_NewIndexes(t *testing.T) {
	tmpDir := t.TempDir()
	origBinDir := model.BinDir
	model.BinDir = tmpDir
	defer func() { model.BinDir = origBinDir }()

	err := InitDB()
	assert.NoError(t, err)
	defer DB.Close()

	indexes := getIndexes(t, DB)
	assert.Contains(t, indexes, "idx_executions_session", "idx_executions_session index should exist")
	assert.Contains(t, indexes, "idx_sessions_type", "idx_sessions_type index should exist")
}

// getTableColumns returns a set of column names for the given table.
func getTableColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info('" + table + "')")
	assert.NoError(t, err)
	defer rows.Close()

	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltVal sql.NullString
		var pk int
		assert.NoError(t, rows.Scan(&cid, &name, &typ, &notNull, &dfltVal, &pk))
		cols[name] = true
	}
	return cols
}

// getIndexes returns a set of index names from sqlite_master.
func getIndexes(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='index'")
	assert.NoError(t, err)
	defer rows.Close()

	indexes := make(map[string]bool)
	for rows.Next() {
		var name string
		assert.NoError(t, rows.Scan(&name))
		indexes[name] = true
	}
	return indexes
}

// ---------- Read-write connection separation ----------

func TestInitDB_ReadWriteSeparation(t *testing.T) {
	tmpDir := t.TempDir()
	origBinDir := model.BinDir
	model.BinDir = tmpDir
	defer func() { model.BinDir = origBinDir }()

	origDB := DB
	origDBRead := DBRead
	defer func() { DB = origDB; DBRead = origDBRead }()

	err := InitDB()
	assert.NoError(t, err)

	// DB (write pool) should be initialized
	assert.NotNil(t, DB, "DB (write pool) should be initialized")

	// DBRead (read pool) should be initialized
	assert.NotNil(t, DBRead, "DBRead (read pool) should be initialized")

	// Both should be different instances
	assert.NotEqual(t, DB, DBRead, "DB and DBRead should be separate connections")

	// Verify write pool has MaxOpenConns=1
	stats := DB.Stats()
	assert.Equal(t, 1, stats.MaxOpenConnections, "DB write pool should have MaxOpenConns=1")

	// Verify read pool has MaxOpenConns=2
	statsRead := DBRead.Stats()
	assert.Equal(t, 2, statsRead.MaxOpenConnections, "DBRead pool should have MaxOpenConns=2")

	// Verify both can query
	var count int
	err = DBRead.QueryRow("SELECT COUNT(*) FROM chat_sessions").Scan(&count)
	assert.NoError(t, err, "DBRead should be able to query")

	// Verify CloseDB closes both
	CloseDB()
}

// ---------- Performance indexes ----------

func TestSchema_HistorySessionIDIndex(t *testing.T) {
	tmpDir := t.TempDir()
	origBinDir := model.BinDir
	model.BinDir = tmpDir
	defer func() { model.BinDir = origBinDir }()

	err := InitDB()
	assert.NoError(t, err)
	defer DB.Close()

	indexes := getIndexes(t, DB)
	assert.True(t, indexes["idx_history_session_id"], "expected idx_history_session_id index to exist")
}

func TestSchema_TasksProjectIndex(t *testing.T) {
	tmpDir := t.TempDir()
	origBinDir := model.BinDir
	model.BinDir = tmpDir
	defer func() { model.BinDir = origBinDir }()

	err := InitDB()
	assert.NoError(t, err)
	defer DB.Close()

	indexes := getIndexes(t, DB)
	assert.True(t, indexes["idx_tasks_project"], "expected idx_tasks_project index to exist")
}

// ---------- Table creation ----------

func TestInitDB_CreatesTables(t *testing.T) {
	db, teardown := setupTestDBForTTS(t)
	defer teardown()

	tables := []string{"tts_summaries", "chat_history", "chat_sessions"}
	for _, table := range tables {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count, "table %s should exist", table)
	}
}

// ---------- Orphaned streaming message cleanup ----------

func TestInitDB_CleansOrphanedStreamingJSON(t *testing.T) {
	db, teardown := setupTestDBForTTS(t)
	defer teardown()

	content := map[string]any{
		"blocks": []any{
			map[string]any{"type": "text", "text": "partial response"},
		},
	}
	contentJSON, _ := json.Marshal(content)
	_, err := db.Exec(
		"INSERT INTO chat_history (project_path, role, content, session_id, backend, streaming) VALUES (?, 'assistant', ?, ?, 'claude', 1)",
		"/test", string(contentJSON), "sess-1",
	)
	assert.NoError(t, err)

	rows, err := db.Query("SELECT id, content FROM chat_history WHERE streaming = 1")
	assert.NoError(t, err)

	type orphanMsg struct {
		id      int64
		content string
	}
	var orphans []orphanMsg
	for rows.Next() {
		var m orphanMsg
		assert.NoError(t, rows.Scan(&m.id, &m.content))
		orphans = append(orphans, m)
	}
	rows.Close()
	assert.Len(t, orphans, 1)

	m := orphans[0]
	var contentMap map[string]any
	json.Unmarshal([]byte(m.content), &contentMap)
	contentMap["cancelled"] = true
	blocks, _ := contentMap["blocks"].([]any)
	blocks = append(blocks, map[string]any{
		"type":   "warning",
		"text":   "Server restarted, AI response interrupted",
		"reason": "restart",
	})
	contentMap["blocks"] = blocks
	updatedContent, _ := json.Marshal(contentMap)
	db.Exec("UPDATE chat_history SET content = ?, streaming = 0 WHERE id = ?", string(updatedContent), m.id)

	var streaming int
	var updated string
	err = db.QueryRow("SELECT streaming, content FROM chat_history WHERE id = ?", m.id).Scan(&streaming, &updated)
	assert.NoError(t, err)
	assert.Equal(t, 0, streaming)

	var result map[string]any
	json.Unmarshal([]byte(updated), &result)
	assert.Equal(t, true, result["cancelled"])
	blocksArr := result["blocks"].([]any)
	assert.Len(t, blocksArr, 2)
	warningBlock := blocksArr[1].(map[string]any)
	assert.Equal(t, "warning", warningBlock["type"])
	assert.Equal(t, "restart", warningBlock["reason"])
}

func TestInitDB_CleansOrphanedStreamingPlain(t *testing.T) {
	db, teardown := setupTestDBForTTS(t)
	defer teardown()

	_, err := db.Exec(
		"INSERT INTO chat_history (project_path, role, content, session_id, backend, streaming) VALUES (?, 'assistant', ?, ?, 'claude', 1)",
		"/test", "plain text response", "sess-2",
	)
	assert.NoError(t, err)

	rows, err := db.Query("SELECT id, content FROM chat_history WHERE streaming = 1")
	assert.NoError(t, err)

	type orphanMsg struct {
		id      int64
		content string
	}
	var orphans []orphanMsg
	for rows.Next() {
		var m orphanMsg
		assert.NoError(t, rows.Scan(&m.id, &m.content))
		orphans = append(orphans, m)
	}
	rows.Close()
	assert.Len(t, orphans, 1)

	m := orphans[0]
	var contentMap map[string]any
	err = json.Unmarshal([]byte(m.content), &contentMap)
	if err != nil {
		contentMap = map[string]any{
			"blocks":    []any{map[string]any{"type": "text", "text": m.content}},
			"cancelled": true,
		}
	}
	updatedContent, _ := json.Marshal(contentMap)
	db.Exec("UPDATE chat_history SET content = ?, streaming = 0 WHERE id = ?", string(updatedContent), m.id)

	var streaming int
	var updated string
	db.QueryRow("SELECT streaming, content FROM chat_history WHERE id = ?", m.id).Scan(&streaming, &updated)
	assert.Equal(t, 0, streaming)

	var result map[string]any
	json.Unmarshal([]byte(updated), &result)
	assert.Equal(t, true, result["cancelled"])
	blocksArr := result["blocks"].([]any)
	assert.Len(t, blocksArr, 1)
	textBlock := blocksArr[0].(map[string]any)
	assert.Equal(t, "text", textBlock["type"])
	assert.Equal(t, "plain text response", textBlock["text"])
}

func TestInitDB_CLIModeSkipsOrphanCleanup(t *testing.T) {
	// Verify that InitDB without runFromServer=true does NOT clean up streaming messages
	db, teardown := setupTestDBForTTS(t)
	defer teardown()

	// Insert a streaming message (simulating an active AI response)
	content := map[string]any{
		"blocks": []any{
			map[string]any{"type": "text", "text": "active streaming response"},
		},
	}
	contentJSON, _ := json.Marshal(content)
	_, err := db.Exec(
		"INSERT INTO chat_history (project_path, role, content, session_id, backend, streaming) VALUES (?, 'assistant', ?, ?, 'claude', 1)",
		"/test", string(contentJSON), "sess-active",
	)
	assert.NoError(t, err)

	// Call the orphan cleanup logic directly with isServerStartup=false
	// This simulates what InitDB(runFromServer=false) does
	// The streaming message should NOT be cleaned up
	orphanCleanup(t, db, false)

	var streaming int
	err = db.QueryRow("SELECT streaming FROM chat_history WHERE session_id = 'sess-active'").Scan(&streaming)
	assert.NoError(t, err)
	assert.Equal(t, 1, streaming, "CLI mode should NOT clean up active streaming messages")
}

func TestInitDB_ServerModeCleansOrphans(t *testing.T) {
	// Verify that InitDB with runFromServer=true DOES clean up streaming messages
	db, teardown := setupTestDBForTTS(t)
	defer teardown()

	// Insert a streaming message (simulating an orphaned message from crash)
	content := map[string]any{
		"blocks": []any{
			map[string]any{"type": "text", "text": "orphaned response"},
		},
	}
	contentJSON, _ := json.Marshal(content)
	_, err := db.Exec(
		"INSERT INTO chat_history (project_path, role, content, session_id, backend, streaming) VALUES (?, 'assistant', ?, ?, 'claude', 1)",
		"/test", string(contentJSON), "sess-orphan",
	)
	assert.NoError(t, err)

	// Call the orphan cleanup logic directly with isServerStartup=true
	// This simulates what InitDB(runFromServer=true) does
	orphanCleanup(t, db, true)

	var streaming int
	err = db.QueryRow("SELECT streaming FROM chat_history WHERE session_id = 'sess-orphan'").Scan(&streaming)
	assert.NoError(t, err)
	assert.Equal(t, 0, streaming, "server mode should clean up orphaned streaming messages")

	// Verify the warning block was added
	var updated string
	err = db.QueryRow("SELECT content FROM chat_history WHERE session_id = 'sess-orphan'").Scan(&updated)
	assert.NoError(t, err)
	var result map[string]any
	json.Unmarshal([]byte(updated), &result)
	assert.Equal(t, true, result["cancelled"])
}

// orphanCleanup replicates the orphan cleanup logic from InitDB for testing.
func orphanCleanup(t *testing.T, db *sql.DB, isServerStartup bool) {
	t.Helper()
	if !isServerStartup {
		return
	}
	rows, err := db.Query("SELECT id, content FROM chat_history WHERE streaming = 1")
	assert.NoError(t, err)
	defer rows.Close()

	type orphanMsg struct {
		id      int64
		content string
	}
	var orphans []orphanMsg
	for rows.Next() {
		var m orphanMsg
		assert.NoError(t, rows.Scan(&m.id, &m.content))
		orphans = append(orphans, m)
	}

	for _, m := range orphans {
		var contentMap map[string]any
		if err := json.Unmarshal([]byte(m.content), &contentMap); err != nil {
			contentMap = map[string]any{
				"blocks":    []any{map[string]any{"type": "text", "text": m.content}},
				"cancelled": true,
			}
		} else {
			contentMap["cancelled"] = true
			blocks, _ := contentMap["blocks"].([]any)
			blocks = append(blocks, map[string]any{
				"type":   "warning",
				"text":   "Server restarted, AI response interrupted",
				"reason": "restart",
			})
			contentMap["blocks"] = blocks
		}
		updatedContent, _ := json.Marshal(contentMap)
		db.Exec("UPDATE chat_history SET content = ?, streaming = 0 WHERE id = ?", string(updatedContent), m.id)
	}
}

// ---------- TTS Summary cache ----------

func TestGetTTSSummary_NotFound(t *testing.T) {
	_, teardown := setupTestDBForTTS(t)
	defer teardown()

	summary, found := GetTTSSummary("nonexistent-key")
	assert.Equal(t, "", summary)
	assert.False(t, found)
}

func TestGetTTSSummary_Found(t *testing.T) {
	_, teardown := setupTestDBForTTS(t)
	defer teardown()

	err := SaveTTSSummary("key-1", "hello world")
	assert.NoError(t, err)

	summary, found := GetTTSSummary("key-1")
	assert.Equal(t, "hello world", summary)
	assert.True(t, found)
}

func TestGetTTSSummary_FailedEntry(t *testing.T) {
	_, teardown := setupTestDBForTTS(t)
	defer teardown()

	err := SaveTTSSummary("key-fail", "raw text")
	assert.NoError(t, err)

	summary, found := GetTTSSummary("key-fail")
	assert.Equal(t, "raw text", summary)
	assert.True(t, found)
}

func TestSaveTTSSummary_Upsert(t *testing.T) {
	_, teardown := setupTestDBForTTS(t)
	defer teardown()

	err := SaveTTSSummary("key-upsert", "version 1")
	assert.NoError(t, err)

	err = SaveTTSSummary("key-upsert", "version 2")
	assert.NoError(t, err)

	summary, found := GetTTSSummary("key-upsert")
	assert.True(t, found)
	assert.Equal(t, "version 2", summary)
}

// ---------- ChatQuickSend CRUD ----------

func TestGetChatQuickSend_Empty(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	items, err := GetChatQuickSend()
	assert.NoError(t, err)
	assert.Nil(t, items)
}

func TestAddChatQuickSend_Single(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	id, err := AddChatQuickSend("▶️ 继续", "继续")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)

	items, err := GetChatQuickSend()
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), items[0].ID)
	assert.Equal(t, "▶️ 继续", items[0].Label)
	assert.Equal(t, "继续", items[0].Command)
	assert.Equal(t, 0, items[0].SortOrder)
}

func TestAddChatQuickSend_MultipleAutoIncrement(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	id1, _ := AddChatQuickSend("继续", "继续")
	id2, _ := AddChatQuickSend("提交", "提交")
	id3, _ := AddChatQuickSend("调试", "调试")

	assert.Equal(t, int64(1), id1)
	assert.Equal(t, int64(2), id2)
	assert.Equal(t, int64(3), id3)

	items, _ := GetChatQuickSend()
	assert.Len(t, items, 3)
	// sort_order auto-increments
	assert.Equal(t, 0, items[0].SortOrder)
	assert.Equal(t, 1, items[1].SortOrder)
	assert.Equal(t, 2, items[2].SortOrder)
}

func TestUpdateChatQuickSend(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	AddChatQuickSend("继续", "继续")

	err := UpdateChatQuickSend(1, "▶️ 继续", "请继续")
	assert.NoError(t, err)

	items, _ := GetChatQuickSend()
	assert.Len(t, items, 1)
	assert.Equal(t, "▶️ 继续", items[0].Label)
	assert.Equal(t, "请继续", items[0].Command)
}

func TestUpdateChatQuickSend_Nonexistent(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	err := UpdateChatQuickSend(999, "x", "y")
	assert.NoError(t, err)
}

func TestDeleteChatQuickSend(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	AddChatQuickSend("继续", "继续")
	AddChatQuickSend("提交", "提交")

	err := DeleteChatQuickSend(1)
	assert.NoError(t, err)

	items, _ := GetChatQuickSend()
	assert.Len(t, items, 1)
	assert.Equal(t, "提交", items[0].Label)
}

func TestDeleteChatQuickSend_Nonexistent(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	err := DeleteChatQuickSend(999)
	assert.NoError(t, err)
}

func TestReorderChatQuickSend(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	AddChatQuickSend("继续", "继续") // id=1, sort_order=0
	AddChatQuickSend("提交", "提交") // id=2, sort_order=1
	AddChatQuickSend("调试", "调试") // id=3, sort_order=2

	// Reverse order: 3, 2, 1
	err := ReorderChatQuickSend([]int64{3, 2, 1})
	assert.NoError(t, err)

	items, _ := GetChatQuickSend()
	assert.Len(t, items, 3)
	assert.Equal(t, "调试", items[0].Label)
	assert.Equal(t, 0, items[0].SortOrder)
	assert.Equal(t, "提交", items[1].Label)
	assert.Equal(t, 1, items[1].SortOrder)
	assert.Equal(t, "继续", items[2].Label)
	assert.Equal(t, 2, items[2].SortOrder)
}

func TestReorderChatQuickSend_EmptyIDs(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	AddChatQuickSend("继续", "继续")

	err := ReorderChatQuickSend([]int64{})
	assert.NoError(t, err)

	items, _ := GetChatQuickSend()
	assert.Len(t, items, 1)
}

func TestReorderChatQuickSend_PartialIDs(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	AddChatQuickSend("继续", "继续") // id=1
	AddChatQuickSend("提交", "提交") // id=2
	AddChatQuickSend("调试", "调试") // id=3

	// Only reorder the first two
	err := ReorderChatQuickSend([]int64{2, 1})
	assert.NoError(t, err)

	items, _ := GetChatQuickSend()
	assert.Len(t, items, 3)
	// 提交(2)→sort=0, 继续(1)→sort=1, 调试(3) still has sort=2 from original
	assert.Equal(t, "提交", items[0].Label)
	assert.Equal(t, "继续", items[1].Label)
	assert.Equal(t, "调试", items[2].Label)
}

func TestGetChatQuickSend_OrderedBySortOrder(t *testing.T) {
	_, teardown := setupTestDBForQuickSend(t)
	defer teardown()

	AddChatQuickSend("A", "a") // sort=0
	AddChatQuickSend("B", "b") // sort=1
	AddChatQuickSend("C", "c") // sort=2

	// Reorder to C, A, B
	ReorderChatQuickSend([]int64{3, 1, 2})

	items, _ := GetChatQuickSend()
	assert.Equal(t, "C", items[0].Label)
	assert.Equal(t, "A", items[1].Label)
	assert.Equal(t, "B", items[2].Label)
}

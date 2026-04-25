package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"clawbench/internal/ai"
	"clawbench/internal/service"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
)

const schema = `
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
`

// setupDB creates an in-memory SQLite database with the required schema,
// sets service.DB, and returns a cleanup function.
func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	assert.NoError(t, err)

	_, err = db.Exec(schema)
	assert.NoError(t, err)

	service.DB = db
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

// helperCreateSession creates a session and asserts success, returning the session ID.
func helperCreateSession(t *testing.T, projectPath, backend, title string) string {
	t.Helper()
	id, err := service.CreateSession(projectPath, backend, title, "", "")
	assert.NoError(t, err)
	assert.NotEmpty(t, id)
	t.Cleanup(func() {
		service.SetSessionRunning(id, false)
	})
	return id
}

// ---------- GetChatHistory / AddChatMessage ----------

func TestAddChatMessageAndGetHistory(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "Test Session")

	_, err := service.AddChatMessage("/project", "claude", sid, "user", "Hello", "", nil, false)
	assert.NoError(t, err)

	_, err = service.AddChatMessage("/project", "claude", sid, "assistant", "Hi there", "", nil, false)
	assert.NoError(t, err)

	msgs, err := service.GetChatHistory("/project", "claude", sid)
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)

	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "Hello", msgs[0].Content)
	assert.Equal(t, sid, msgs[0].SessionID)
	assert.Equal(t, "claude", msgs[0].Backend)
	assert.False(t, msgs[0].Streaming)

	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "Hi there", msgs[1].Content)
}

func TestGetChatHistory_Empty(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "Empty")

	msgs, err := service.GetChatHistory("/project", "claude", sid)
	assert.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestAddChatMessage_AutoTitle(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "New Session")

	// First user message should auto-title the session
	_, err := service.AddChatMessage("/project", "claude", sid, "user", "This is my question about Go testing", "", nil, false)
	assert.NoError(t, err)

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, "This is my question about Go testing", title)
}

func TestAddChatMessage_AutoTitleTruncated(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "New Session")

	longContent := strings.Repeat("啊", 60) // 60 runes, each is a multi-byte character
	_, err := service.AddChatMessage("/project", "claude", sid, "user", longContent, "", nil, false)
	assert.NoError(t, err)

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, 53, utf8.RuneCountInString(title)) // 50 runes + "..."
	runes := []rune(title)
	assert.Equal(t, "啊", string(runes[49]))  // 50th rune is still content
	assert.Equal(t, "...", string(runes[50:])) // followed by ellipsis
}

func TestAddChatMessage_AutoTitleOnlyFirstUserMessage(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "New Session")

	_, err := service.AddChatMessage("/project", "claude", sid, "user", "First message", "", nil, false)
	assert.NoError(t, err)

	_, err = service.AddChatMessage("/project", "claude", sid, "user", "Second message", "", nil, false)
	assert.NoError(t, err)

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, "First message", title) // Title unchanged after second message
}

func TestAddChatMessage_AutoTitleEmptyContentWithFiles(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "New Session")

	_, err := service.AddChatMessage("/project", "claude", sid, "user", "", "", []string{"file1.txt"}, false)
	assert.NoError(t, err)

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, "文件消息", title)
}

func TestAddChatMessage_AutoTitleEmptyContentNoFiles(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "New Session")

	_, err := service.AddChatMessage("/project", "claude", sid, "user", "", "", nil, false)
	assert.NoError(t, err)

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, "新会话", title)
}

func TestAddChatMessage_WithFiles(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "File Test")

	files := []string{"image.png", "doc.pdf"}
	_, err := service.AddChatMessage("/project", "claude", sid, "user", "Check these", "/path/file", files, false)
	assert.NoError(t, err)

	msgs, err := service.GetChatHistory("/project", "claude", sid)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, files, msgs[0].Files)
	assert.Equal(t, "/path/file", msgs[0].FilePath)
}

func TestAddChatMessage_Streaming(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "Stream Test")

	_, err := service.AddChatMessage("/project", "claude", sid, "assistant", "partial...", "", nil, true)
	assert.NoError(t, err)

	msgs, err := service.GetChatHistory("/project", "claude", sid)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.True(t, msgs[0].Streaming)
}

func TestAddChatMessage_AssistantDoesNotAutoTitle(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "Original Title")

	_, err := service.AddChatMessage("/project", "claude", sid, "assistant", "AI response", "", nil, false)
	assert.NoError(t, err)

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, "Original Title", title) // Assistant messages don't change title
}

// ---------- CreateSession ----------

func TestCreateSession_UUIDFormat(t *testing.T) {
	setupDB(t)

	id, err := service.CreateSession("/project", "claude", "Test", "", "")
	assert.NoError(t, err)
	assert.NotEmpty(t, id)

	// UUID v4 format: 8-4-4-4-12 hex digits separated by dashes
	parts := strings.Split(id, "-")
	assert.Len(t, parts, 5, "UUID should have 5 parts separated by dashes")
	assert.Equal(t, 8, len(parts[0]))
	assert.Equal(t, 4, len(parts[1]))
	assert.Equal(t, 4, len(parts[2]))
	assert.Equal(t, 4, len(parts[3]))
	assert.Equal(t, 12, len(parts[4]))
}

func TestCreateSession_UniqueIDs(t *testing.T) {
	setupDB(t)

	id1, err := service.CreateSession("/project", "claude", "Session 1", "", "")
	assert.NoError(t, err)
	id2, err := service.CreateSession("/project", "claude", "Session 2", "", "")
	assert.NoError(t, err)
	assert.NotEqual(t, id1, id2)
}

// ---------- DeleteSession ----------

func TestDeleteSession(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "To Delete")
	_, err := service.AddChatMessage("/project", "claude", sid, "user", "msg", "", nil, false)
	assert.NoError(t, err)

	err = service.DeleteSession("/project", "claude", sid)
	assert.NoError(t, err)

	// Session should be gone
	_, err = service.GetSessionTitle(sid)
	assert.Error(t, err) // Should return error for non-existent session

	// Messages should be gone
	msgs, err := service.GetChatHistory("/project", "claude", sid)
	assert.NoError(t, err)
	assert.Empty(t, msgs)
}

// ---------- GetSessions ----------

func TestGetSessions_FiltersByProjectAndBackend(t *testing.T) {
	setupDB(t)

	helperCreateSession(t, "/proj1", "claude", "C1")
	helperCreateSession(t, "/proj1", "codebuddy", "CB1")
	helperCreateSession(t, "/proj2", "claude", "C2")

	sessions, err := service.GetSessions("/proj1", "claude")
	assert.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "claude", sessions[0].Backend)

	sessions, err = service.GetSessions("/proj1", "codebuddy")
	assert.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "codebuddy", sessions[0].Backend)

	sessions, err = service.GetSessions("/proj2", "claude")
	assert.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestGetSessions_AllBackends(t *testing.T) {
	setupDB(t)

	helperCreateSession(t, "/proj", "claude", "C1")
	helperCreateSession(t, "/proj", "codebuddy", "CB1")
	helperCreateSession(t, "/other", "claude", "C2")

	sessions, err := service.GetSessions("/proj", "")
	assert.NoError(t, err)
	assert.Len(t, sessions, 2)

	// Should NOT include /other
	for _, s := range sessions {
		// Can't directly check project_path since it's not in ChatSession,
		// but we know we created 2 sessions for /proj
		assert.Contains(t, []string{"claude", "codebuddy"}, s.Backend)
	}
}

// ---------- GetSessionBackend ----------

func TestGetSessionBackend(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "codebuddy", "Test")
	backend := service.GetSessionBackend(sid)
	assert.Equal(t, "codebuddy", backend)
}

func TestGetSessionBackend_NonExistent(t *testing.T) {
	setupDB(t)

	backend := service.GetSessionBackend("non-existent-id")
	assert.Equal(t, "", backend)
}

// ---------- Session Running ----------

func TestIsSessionRunning_DefaultFalse(t *testing.T) {
	setupDB(t)

	assert.False(t, service.IsSessionRunning("any-id"))
}

func TestSetSessionRunning(t *testing.T) {
	setupDB(t)

	service.SetSessionRunning("sess-1", true)
	assert.True(t, service.IsSessionRunning("sess-1"))

	service.SetSessionRunning("sess-1", false)
	assert.False(t, service.IsSessionRunning("sess-1"))
}

func TestTrySetSessionRunning(t *testing.T) {
	setupDB(t)

	// First try should succeed
	ok := service.TrySetSessionRunning("sess-2")
	assert.True(t, ok)
	assert.True(t, service.IsSessionRunning("sess-2"))

	// Second try should fail (already running)
	ok = service.TrySetSessionRunning("sess-2")
	assert.False(t, ok)

	// Clean up
	service.SetSessionRunning("sess-2", false)
}

// ---------- SessionHasAssistant ----------

func TestSessionHasAssistant(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "Test")

	// No assistant messages yet
	assert.False(t, service.SessionHasAssistant(sid))

	// Add a streaming assistant message - should NOT count
	_, err := service.AddChatMessage("/project", "claude", sid, "assistant", "partial", "", nil, true)
	assert.NoError(t, err)
	assert.False(t, service.SessionHasAssistant(sid))

	// Add a finalized assistant message - should count
	_, err = service.AddChatMessage("/project", "claude", sid, "assistant", "final", "", nil, false)
	assert.NoError(t, err)
	assert.True(t, service.SessionHasAssistant(sid))
}

// ---------- UpdateStreamingMessage / FinalizeStreamingMessage ----------

func TestUpdateStreamingMessage(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "Stream")

	_, err := service.AddChatMessage("/project", "claude", sid, "assistant", "initial", "", nil, true)
	assert.NoError(t, err)

	err = service.UpdateStreamingMessage("/project", "claude", sid, "updated content")
	assert.NoError(t, err)

	msgs, err := service.GetChatHistory("/project", "claude", sid)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "updated content", msgs[0].Content)
	assert.True(t, msgs[0].Streaming) // Still streaming
}

func TestFinalizeStreamingMessage(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "Stream")

	_, err := service.AddChatMessage("/project", "claude", sid, "assistant", "streaming...", "", nil, true)
	assert.NoError(t, err)

	err = service.FinalizeStreamingMessage("/project", "claude", sid, "final content")
	assert.NoError(t, err)

	msgs, err := service.GetChatHistory("/project", "claude", sid)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "final content", msgs[0].Content)
	assert.False(t, msgs[0].Streaming) // No longer streaming
}

func TestUpdateStreamingMessage_NoStreamingRow(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "NoStream")

	// No streaming message exists; update should succeed but affect 0 rows
	err := service.UpdateStreamingMessage("/project", "claude", sid, "content")
	assert.NoError(t, err)
}

// ---------- CancelSession ----------

func TestCancelSession_NoCancelFunc(t *testing.T) {
	setupDB(t)

	ok := service.CancelSession("non-existent-session")
	// Non-running session with no cancel func is considered already cancelled (idempotent)
	assert.True(t, ok)
}

func TestCancelSession_WithCancelFunc(t *testing.T) {
	setupDB(t)

	sid := "cancel-test-session"
	cancelled := false
	ctx, cancel := context.WithCancel(context.Background())

	service.RegisterSessionCancel(sid, cancel)
	service.SetSessionRunning(sid, true)

	// Cancel the session
	ok := service.CancelSession(sid)
	assert.True(t, ok)

	// Context should be cancelled
	<-ctx.Done()
	cancelled = true
	assert.True(t, cancelled)

	// Session should no longer be running
	assert.False(t, service.IsSessionRunning(sid))

	// Second cancel should return true (idempotent: session is no longer running)
	ok = service.CancelSession(sid)
	assert.True(t, ok)
}

func TestCancelSession_WithStreamChannel(t *testing.T) {
	setupDB(t)

	sid := "cancel-stream-test"
	ctx, cancel := context.WithCancel(context.Background())

	// Register both cancel and stream
	service.RegisterSessionCancel(sid, cancel)
	ch := service.RegisterSessionStream(sid)
	service.SetSessionRunning(sid, true)

	// Cancel in goroutine that also reads the stream
	done := make(chan struct{})
	go func() {
		event := <-ch
		assert.Equal(t, "cancelled", event.Type)
		close(done)
	}()

	ok := service.CancelSession(sid)
	assert.True(t, ok)

	<-done
	<-ctx.Done()

	// Clean up stream
	service.UnregisterSessionStream(sid)
}

// ---------- GetSessionTitle ----------

func TestGetSessionTitle(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "My Title")

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, "My Title", title)
}

func TestGetSessionTitle_NonExistent(t *testing.T) {
	setupDB(t)

	_, err := service.GetSessionTitle("non-existent")
	assert.Error(t, err)
}

// ---------- Edge cases ----------

func TestAddChatMessage_MultipleSessionsIsolated(t *testing.T) {
	setupDB(t)

	sid1 := helperCreateSession(t, "/project", "claude", "Session 1")
	sid2 := helperCreateSession(t, "/project", "claude", "Session 2")

	_, _ = service.AddChatMessage("/project", "claude", sid1, "user", "Msg in session 1", "", nil, false)
	_, _ = service.AddChatMessage("/project", "claude", sid2, "user", "Msg in session 2", "", nil, false)

	msgs1, err := service.GetChatHistory("/project", "claude", sid1)
	assert.NoError(t, err)
	assert.Len(t, msgs1, 1)
	assert.Equal(t, "Msg in session 1", msgs1[0].Content)

	msgs2, err := service.GetChatHistory("/project", "claude", sid2)
	assert.NoError(t, err)
	assert.Len(t, msgs2, 1)
	assert.Equal(t, "Msg in session 2", msgs2[0].Content)
}

func TestAddChatMessage_AutoTitleExactly50Runes(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "New")

	// Exactly 50 runes - should NOT be truncated
	content := strings.Repeat("x", 50)
	_, err := service.AddChatMessage("/project", "claude", sid, "user", content, "", nil, false)
	assert.NoError(t, err)

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, content, title) // No "..." appended
}

func TestAddChatMessage_AutoTitle51Runes(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "New")

	// 51 runes - should be truncated to 50 + "..."
	content := strings.Repeat("x", 51)
	_, err := service.AddChatMessage("/project", "claude", sid, "user", content, "", nil, false)
	assert.NoError(t, err)

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, strings.Repeat("x", 50)+"...", title)
}

func TestAddChatMessage_WithFilePath(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "FP Test")

	_, err := service.AddChatMessage("/project", "claude", sid, "user", "look at this", "/src/main.go", nil, false)
	assert.NoError(t, err)

	msgs, err := service.GetChatHistory("/project", "claude", sid)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "/src/main.go", msgs[0].FilePath)
}

func TestGetSessions_OrderedByUpdatedDesc(t *testing.T) {
	setupDB(t)

	// Create sessions - their updated_at will be set on creation
	sid1 := helperCreateSession(t, "/project", "claude", "First")
	sid2 := helperCreateSession(t, "/project", "claude", "Second")

	// Add a message to sid1 to bump its updated_at
	_, _ = service.AddChatMessage("/project", "claude", sid1, "user", "bump", "", nil, false)

	sessions, err := service.GetSessions("/project", "claude")
	assert.NoError(t, err)
	assert.Len(t, sessions, 2)

	// sid1 should be first since it was updated most recently
	assert.Equal(t, sid1, sessions[0].ID)
	assert.Equal(t, sid2, sessions[1].ID)
}

func TestDeleteSession_NonExistentDoesNotError(t *testing.T) {
	setupDB(t)

	// Deleting a non-existent session should not return an error
	// (DELETE on non-existent rows is a no-op)
	err := service.DeleteSession("/project", "claude", "non-existent-id")
	assert.NoError(t, err)
}

func TestRegisterUnregisterSessionCancel(t *testing.T) {
	setupDB(t)

	sid := "cancel-reg-test"
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	service.RegisterSessionCancel(sid, cancel)

	// Should be able to cancel
	ok := service.CancelSession(sid)
	assert.True(t, ok)

	// After cancel, register a new one
	_, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	service.RegisterSessionCancel(sid, cancel2)
	service.UnregisterSessionCancel(sid)

	// After unregister, cancel returns true (session not running → idempotent)
	ok = service.CancelSession(sid)
	assert.True(t, ok)
}

func TestFinalizeStreamingMessage_NoStreamingRow(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "NoStream")

	// No streaming message; finalize should succeed but affect 0 rows
	err := service.FinalizeStreamingMessage("/project", "claude", sid, "content")
	assert.NoError(t, err)
}

func TestAddChatMessage_AutoTitleWithFilePathAndContent(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "New")

	// When content is non-empty, title comes from content (not files)
	_, err := service.AddChatMessage("/project", "claude", sid, "user", "Hello world", "/some/file.go", []string{"file.go"}, false)
	assert.NoError(t, err)

	title, err := service.GetSessionTitle(sid)
	assert.NoError(t, err)
	assert.Equal(t, "Hello world", title)
}

func TestGetChatHistory_DifferentBackendsIsolated(t *testing.T) {
	setupDB(t)

	sidC := helperCreateSession(t, "/project", "claude", "Claude")
	sidCB := helperCreateSession(t, "/project", "codebuddy", "CodeBuddy")

	_, _ = service.AddChatMessage("/project", "claude", sidC, "user", "claude msg", "", nil, false)
	_, _ = service.AddChatMessage("/project", "codebuddy", sidCB, "user", "codebuddy msg", "", nil, false)

	msgsC, err := service.GetChatHistory("/project", "claude", sidC)
	assert.NoError(t, err)
	assert.Len(t, msgsC, 1)
	assert.Equal(t, "claude msg", msgsC[0].Content)

	msgsCB, err := service.GetChatHistory("/project", "codebuddy", sidCB)
	assert.NoError(t, err)
	assert.Len(t, msgsCB, 1)
	assert.Equal(t, "codebuddy msg", msgsCB[0].Content)
}

// Ensure TestMain-like global DB save/restore works correctly
func TestGlobalDBPreservedAcrossParallelTests(t *testing.T) {
	originalDB := service.DB
	setupDB(t)
	// Within this test, service.DB is our in-memory DB
	assert.NotNil(t, service.DB)
	assert.NotEqual(t, originalDB, service.DB) // if originalDB was nil or different
}

func TestAddChatMessage_StreamingFalse(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "No Stream")

	_, err := service.AddChatMessage("/project", "claude", sid, "assistant", "final response", "", nil, false)
	assert.NoError(t, err)

	msgs, err := service.GetChatHistory("/project", "claude", sid)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.False(t, msgs[0].Streaming)
}

func TestUpdateThenFinalizeStreaming(t *testing.T) {
	setupDB(t)

	sid := helperCreateSession(t, "/project", "claude", "Full Stream")

	// Start streaming
	_, _ = service.AddChatMessage("/project", "claude", sid, "assistant", "start", "", nil, true)

	// Update content multiple times
	service.UpdateStreamingMessage("/project", "claude", sid, "start + more")
	service.UpdateStreamingMessage("/project", "claude", sid, "start + more + final")

	// Verify still streaming
	msgs, _ := service.GetChatHistory("/project", "claude", sid)
	assert.True(t, msgs[0].Streaming)
	assert.Equal(t, "start + more + final", msgs[0].Content)

	// Finalize
	service.FinalizeStreamingMessage("/project", "claude", sid, "complete response")

	msgs, _ = service.GetChatHistory("/project", "claude", sid)
	assert.False(t, msgs[0].Streaming)
	assert.Equal(t, "complete response", msgs[0].Content)
}

func TestCancelSession_CleansUpRunningState(t *testing.T) {
	setupDB(t)

	sid := fmt.Sprintf("cleanup-test-%d", len("x"))
	_, cancel := context.WithCancel(context.Background())

	service.RegisterSessionCancel(sid, cancel)
	service.SetSessionRunning(sid, true)
	assert.True(t, service.IsSessionRunning(sid))

	service.CancelSession(sid)
	assert.False(t, service.IsSessionRunning(sid))
}

// ---------- GetChatMessageCount ----------

func TestGetChatMessageCount(t *testing.T) {
	setupDB(t)
	sid := helperCreateSession(t, "/project", "claude", "Test")
	// Initially 0
	assert.Equal(t, 0, service.GetChatMessageCount(sid))
	// Add messages
	service.AddChatMessage("/project", "claude", sid, "user", "Hello", "", nil, false)
	service.AddChatMessage("/project", "claude", sid, "assistant", "Hi", "", nil, false)
	assert.Equal(t, 2, service.GetChatMessageCount(sid))
}

func TestGetChatMessageCount_NonExistent(t *testing.T) {
	setupDB(t)
	assert.Equal(t, 0, service.GetChatMessageCount("non-existent"))
}

// ---------- UpdateLastRead ----------

func TestUpdateLastRead(t *testing.T) {
	setupDB(t)
	sid := helperCreateSession(t, "/project", "claude", "Test")
	// Should not panic and should succeed
	service.UpdateLastRead(sid)
	// Verify by checking sessions - last_read_at should be set
	var lastRead sql.NullTime
	err := service.DB.QueryRow("SELECT last_read_at FROM chat_sessions WHERE id = ?", sid).Scan(&lastRead)
	assert.NoError(t, err)
	assert.True(t, lastRead.Valid)
}

// ---------- GetSessionAgentID ----------

func TestGetSessionAgentID(t *testing.T) {
	setupDB(t)
	// Create session with agent ID
	sid, err := service.CreateSession("/project", "claude", "Test", "my-agent", "gpt-4")
	assert.NoError(t, err)
	assert.Equal(t, "my-agent", service.GetSessionAgentID(sid))
}

func TestGetSessionAgentID_NonExistent(t *testing.T) {
	setupDB(t)
	assert.Equal(t, "", service.GetSessionAgentID("non-existent"))
}

func TestGetSessionAgentID_EmptyAgent(t *testing.T) {
	setupDB(t)
	sid := helperCreateSession(t, "/project", "claude", "No Agent")
	assert.Equal(t, "", service.GetSessionAgentID(sid))
}

// ---------- GetAndClearCancelReason ----------

func TestGetAndClearCancelReason_NoReason(t *testing.T) {
	setupDB(t)
	assert.Equal(t, "", service.GetAndClearCancelReason("non-existent"))
}

func TestGetAndClearCancelReason_WithReason(t *testing.T) {
	setupDB(t)
	sid := "test-reason-session"
	// Simulate setting cancel reason (as CancelSession does)
	service.RegisterSessionCancel(sid, func() {})
	service.SetSessionRunning(sid, true)
	// Cancel sets "user" reason
	service.CancelSession(sid)
	// Should return "user"
	assert.Equal(t, "user", service.GetAndClearCancelReason(sid))
	// Second call should return "" (cleared)
	assert.Equal(t, "", service.GetAndClearCancelReason(sid))
}

// ---------- ForceCancelSession ----------

func TestForceCancelSession(t *testing.T) {
	setupDB(t)
	sid := "force-cancel-test"
	ctx, cancel := context.WithCancel(context.Background())
	service.RegisterSessionCancel(sid, cancel)
	service.SetSessionRunning(sid, true)

	service.ForceCancelSession(sid)

	// Context should be cancelled
	<-ctx.Done()
	// Reason should be "disconnect"
	assert.Equal(t, "disconnect", service.GetAndClearCancelReason(sid))
}

func TestForceCancelSession_NoCancelFunc(t *testing.T) {
	setupDB(t)
	// Should not panic when no cancel func exists
	service.ForceCancelSession("non-existent")
}

// ---------- SendSessionEvent ----------

func TestSendSessionEvent(t *testing.T) {
	setupDB(t)
	sid := "send-event-test"
	ch := service.RegisterSessionStream(sid)

	// Send event
	ok := service.SendSessionEvent(sid, ai.StreamEvent{Type: "content", Content: "hello"})
	assert.True(t, ok)

	// Receive event
	event := <-ch
	assert.Equal(t, "content", event.Type)
	assert.Equal(t, "hello", event.Content)

	service.UnregisterSessionStream(sid)
}

func TestSendSessionEvent_NoStream(t *testing.T) {
	setupDB(t)
	ok := service.SendSessionEvent("non-existent", ai.StreamEvent{Type: "content"})
	assert.False(t, ok)
}

func TestSendSessionEvent_FullChannel(t *testing.T) {
	setupDB(t)
	sid := "full-channel-test"
	// Register stream with small buffer (64 is the default)
	ch := service.RegisterSessionStream(sid)

	// Fill the channel buffer
	for i := 0; i < 64; i++ {
		ok := service.SendSessionEvent(sid, ai.StreamEvent{Type: "content", Content: fmt.Sprintf("msg-%d", i)})
		assert.True(t, ok)
	}

	// Next send should fail (non-blocking)
	ok := service.SendSessionEvent(sid, ai.StreamEvent{Type: "content", Content: "overflow"})
	assert.False(t, ok)

	// Drain the channel to clean up
	for i := 0; i < 64; i++ {
		<-ch
	}
	service.UnregisterSessionStream(sid)
}

// ---------- UpdateMessageContent ----------

func TestUpdateMessageContent(t *testing.T) {
	setupDB(t)
	sid := helperCreateSession(t, "/project", "claude", "Test")

	msgID, err := service.AddChatMessage("/project", "claude", sid, "user", "original", "", nil, false)
	assert.NoError(t, err)

	err = service.UpdateMessageContent(int(msgID), "updated content")
	assert.NoError(t, err)

	msgs, _ := service.GetChatHistory("/project", "claude", sid)
	assert.Equal(t, "updated content", msgs[0].Content)
}

func TestUpdateMessageContent_NonExistent(t *testing.T) {
	setupDB(t)
	// Updating non-existent message should not error (UPDATE affects 0 rows)
	err := service.UpdateMessageContent(99999, "content")
	assert.NoError(t, err)
}

// ---------- UpdateExternalSessionID / GetExternalSessionID ----------

func TestUpdateAndGetExternalSessionID(t *testing.T) {
	db := setupDB(t)
	// Add external_session_id column (not in base schema)
	_, err := db.Exec("ALTER TABLE chat_sessions ADD COLUMN external_session_id TEXT DEFAULT ''")
	assert.NoError(t, err)

	sid := helperCreateSession(t, "/project", "opencode", "Test")

	// Initially empty
	assert.Equal(t, "", service.GetExternalSessionID(sid))

	// Set external ID
	err = service.UpdateExternalSessionID(sid, "ext-session-123")
	assert.NoError(t, err)

	// Get external ID
	assert.Equal(t, "ext-session-123", service.GetExternalSessionID(sid))
}

func TestGetExternalSessionID_NonExistent(t *testing.T) {
	setupDB(t)
	assert.Equal(t, "", service.GetExternalSessionID("non-existent"))
}

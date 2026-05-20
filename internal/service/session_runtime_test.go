package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"clawbench/internal/ai"
	"clawbench/internal/model"
	"clawbench/internal/ws"

	"github.com/stretchr/testify/assert"

	_ "modernc.org/sqlite"
)

// --- RegisterSessionCancel / UnregisterSessionCancel ---

func TestRegisterSessionCancel(t *testing.T) {
	cleanupCancels()
	defer cleanupCancels()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	RegisterSessionCancel("session-cancel-1", cancel)

	// Cancel should be stored; loading and calling it should cancel the context
	val, ok := sessionCancels.Load("session-cancel-1")
	assert.True(t, ok)
	loadedCancel, ok := val.(context.CancelFunc)
	assert.True(t, ok)
	assert.NotNil(t, loadedCancel)
}

func TestUnregisterSessionCancel(t *testing.T) {
	cleanupCancels()
	defer cleanupCancels()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	RegisterSessionCancel("session-cancel-2", cancel)
	UnregisterSessionCancel("session-cancel-2")

	_, ok := sessionCancels.Load("session-cancel-2")
	assert.False(t, ok)
}

func TestUnregisterSessionCancel_Idempotent(t *testing.T) {
	cleanupCancels()

	// Should not panic when deleting nonexistent key
	assert.NotPanics(t, func() {
		UnregisterSessionCancel("nonexistent")
	})
}

// --- GetAndClearCancelReason ---

func TestGetAndClearCancelReason_UserReason(t *testing.T) {
	cleanupCancelReasons()
	defer cleanupCancelReasons()

	sessionCancelReasons.Store("session-reason-1", "user")

	reason := GetAndClearCancelReason("session-reason-1")
	assert.Equal(t, "user", reason)

	// Should be cleared after first call
	reason2 := GetAndClearCancelReason("session-reason-1")
	assert.Equal(t, "", reason2)
}

func TestGetAndClearCancelReason_DisconnectReason(t *testing.T) {
	cleanupCancelReasons()
	defer cleanupCancelReasons()

	sessionCancelReasons.Store("session-reason-2", "disconnect")

	reason := GetAndClearCancelReason("session-reason-2")
	assert.Equal(t, "disconnect", reason)
}

func TestGetAndClearCancelReason_NoReason(t *testing.T) {
	cleanupCancelReasons()

	reason := GetAndClearCancelReason("nonexistent")
	assert.Equal(t, "", reason)
}

// --- CancelSession ---

func TestCancelSession_WithCancelFunc(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	ctx, cancel := context.WithCancel(context.Background())
	RegisterSessionCancel("session-cancel-3", cancel)
	SetSessionRunning("session-cancel-3", true)
	RegisterSessionStream("session-cancel-3")

	result := CancelSession("session-cancel-3")
	assert.True(t, result)

	// Context should be cancelled
	assert.Error(t, ctx.Err())

	// Session should no longer be running
	assert.False(t, IsSessionRunning("session-cancel-3"))

	// Cancel reason should be "user"
	reason := GetAndClearCancelReason("session-cancel-3")
	assert.Equal(t, "user", reason)

	// Cancel func should be removed
	_, ok := sessionCancels.Load("session-cancel-3")
	assert.False(t, ok)
}

func TestCancelSession_NotRunning_NoCancelFunc(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	// Session not running and no cancel func - idempotent success
	result := CancelSession("session-idle")
	assert.True(t, result)
}

func TestCancelSession_Running_NoCancelFunc(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	SetSessionRunning("session-stuck", true)

	// Running session with no cancel func - can't cancel
	result := CancelSession("session-stuck")
	assert.False(t, result)
}

func TestCancelSession_SendsCancelledEvent(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	_, cancel := context.WithCancel(context.Background())
	RegisterSessionCancel("session-event", cancel)
	SetSessionRunning("session-event", true)
	ch := RegisterSessionStream("session-event")

	result := CancelSession("session-event")
	assert.True(t, result)

	// Should receive a cancelled event
	select {
	case event := <-ch:
		assert.Equal(t, "cancelled", event.Type)
	case <-time.After(time.Second):
		t.Fatal("expected cancelled event on stream channel")
	}
}

// --- ForceCancelSession ---

func TestForceCancelSession(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	ctx, cancel := context.WithCancel(context.Background())
	RegisterSessionCancel("session-force", cancel)
	SetSessionRunning("session-force", true)

	ForceCancelSession("session-force")

	// Context should be cancelled
	assert.Error(t, ctx.Err())

	// Cancel reason should be "disconnect"
	reason := GetAndClearCancelReason("session-force")
	assert.Equal(t, "disconnect", reason)

	// Cancel func should be removed
	_, ok := sessionCancels.Load("session-force")
	assert.False(t, ok)
}

func TestForceCancelSession_NotFound(t *testing.T) {
	cleanupAllSessionState()

	// Should not panic on nonexistent session
	assert.NotPanics(t, func() {
		ForceCancelSession("nonexistent")
	})
}

// --- SendSessionEvent ---

func TestSendSessionEvent_Success(t *testing.T) {
	cleanupStreams()
	defer cleanupStreams()

	ch := RegisterSessionStream("session-event-test")

	event := ai.StreamEvent{Type: "content", Content: "hello"}
	sent := SendSessionEvent("session-event-test", event)
	assert.True(t, sent)

	received := <-ch
	assert.Equal(t, "content", received.Type)
	assert.Equal(t, "hello", received.Content)
}

func TestSendSessionEvent_SessionNotFound(t *testing.T) {
	cleanupStreams()

	sent := SendSessionEvent("nonexistent", ai.StreamEvent{Type: "content"})
	assert.False(t, sent)
}

func TestSendSessionEvent_FullChannel(t *testing.T) {
	cleanupStreams()
	defer cleanupStreams()

	RegisterSessionStream("session-full")

	// Fill the channel buffer (capacity is 256)
	for i := 0; i < 256; i++ {
		sent := SendSessionEvent("session-full", ai.StreamEvent{Type: "content", Content: "x"})
		assert.True(t, sent)
	}

	// Next send should fail (non-blocking)
	sent := SendSessionEvent("session-full", ai.StreamEvent{Type: "done"})
	assert.False(t, sent, "SendSessionEvent should return false when channel is full")
}

// --- TrySetSessionRunning ---

func TestTrySetSessionRunning_Success(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	result := TrySetSessionRunning("session-try-1")
	assert.True(t, result)
	assert.True(t, IsSessionRunning("session-try-1"))
}

func TestTrySetSessionRunning_AlreadyRunning(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	result1 := TrySetSessionRunning("session-try-2")
	assert.True(t, result1)

	result2 := TrySetSessionRunning("session-try-2")
	assert.False(t, result2, "Second TrySetSessionRunning should return false")
}

func TestTrySetSessionRunning_DifferentSessions(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	result1 := TrySetSessionRunning("session-a")
	assert.True(t, result1)
	assert.True(t, IsSessionRunning("session-a"))

	result2 := TrySetSessionRunning("session-b")
	assert.True(t, result2)
	assert.True(t, IsSessionRunning("session-b"))

	// Both should be running independently
	assert.True(t, IsSessionRunning("session-a"))
}

func TestTrySetSessionRunning_FailedTryDoesNotAffectExisting(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	// First TrySet succeeds
	assert.True(t, TrySetSessionRunning("session-x"))
	// Second TrySet on same ID fails
	assert.False(t, TrySetSessionRunning("session-x"))
	// But session is still marked as running
	assert.True(t, IsSessionRunning("session-x"))
}

func TestSetSessionRunning_TrySetMixedSequence(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	// Start via SetSessionRunning
	SetSessionRunning("session-mix", true)
	assert.True(t, IsSessionRunning("session-mix"))

	// TrySetSessionRunning on already-running session should fail
	assert.False(t, TrySetSessionRunning("session-mix"))

	// Stop via SetSessionRunning
	SetSessionRunning("session-mix", false)
	assert.False(t, IsSessionRunning("session-mix"))

	// Now TrySetSessionRunning should succeed
	assert.True(t, TrySetSessionRunning("session-mix"))
	assert.True(t, IsSessionRunning("session-mix"))
}

func TestTrySetSessionRunning_Concurrent(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	// Multiple goroutines try to set the same session as running.
	// Exactly one should succeed.
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if TrySetSessionRunning("session-concurrent-try") {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, 1, successCount, "Exactly one TrySetSessionRunning should succeed")
	assert.True(t, IsSessionRunning("session-concurrent-try"))
}

func TestSetSessionRunning_FalseRemovesKey(t *testing.T) {
	cleanupActiveSessions()
	defer cleanupActiveSessions()

	SetSessionRunning("session-rm", true)
	assert.True(t, IsSessionRunning("session-rm"))

	SetSessionRunning("session-rm", false)
	assert.False(t, IsSessionRunning("session-rm"))
}

// --- Concurrent access tests ---

func TestSendSessionEvent_ConcurrentAccess(t *testing.T) {
	cleanupStreams()
	defer cleanupStreams()

	RegisterSessionStream("session-concurrent")

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Send 50 events concurrently (buffer is 64, so most should succeed)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sent := SendSessionEvent("session-concurrent", ai.StreamEvent{Type: "content"})
			if sent {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, 50, successCount, "All 50 events should be sent (buffer is 64)")
}

// --- Helpers ---

func cleanupCancels() {
	sessionCancels.Range(func(key, _ interface{}) bool {
		sessionCancels.Delete(key)
		return true
	})
}

func cleanupCancelReasons() {
	sessionCancelReasons.Range(func(key, _ interface{}) bool {
		sessionCancelReasons.Delete(key)
		return true
	})
}

func cleanupActiveSessions() {
	activeMu.Lock()
	defer activeMu.Unlock()
	activeSessions = make(map[string]bool)
}

func cleanupAllSessionState() {
	cleanupActiveSessions()
	cleanupCancels()
	cleanupCancelReasons()
	cleanupStreams()
}

// --- getSessionResponsePreview tests ---

func setupChatTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS chat_history (
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
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func insertTestMessage(t *testing.T, db *sql.DB, sessionID, role, content string) {
	t.Helper()
	_, err := db.Exec("INSERT INTO chat_history (project_path, role, content, session_id, backend, streaming) VALUES (?, ?, ?, ?, 'claude', 0)",
		"/test", role, content, sessionID)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
}

func TestGetSessionResponsePreview_WithTextBlock(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	content := model.ContentBlock{Type: "text", Text: "你好，这是AI的回复内容"}
	blocks := map[string]any{"blocks": []model.ContentBlock{content}}
	contentJSON, _ := json.Marshal(blocks)
	insertTestMessage(t, db, "session-preview-1", "user", "问题")
	insertTestMessage(t, db, "session-preview-1", "assistant", string(contentJSON))

	result := getSessionResponsePreview("session-preview-1")
	assert.Equal(t, "你好，这是AI的回复内容", result)
}

func TestGetSessionResponsePreview_Truncation(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	// responsePreviewMaxRunes+1 runes — should be truncated
	longText := strings.Repeat("测", responsePreviewMaxRunes+1)
	content := model.ContentBlock{Type: "text", Text: longText}
	blocks := map[string]any{"blocks": []model.ContentBlock{content}}
	contentJSON, _ := json.Marshal(blocks)
	insertTestMessage(t, db, "session-preview-2", "user", "问题")
	insertTestMessage(t, db, "session-preview-2", "assistant", string(contentJSON))

	result := getSessionResponsePreview("session-preview-2")
	runes := []rune(longText)
	assert.Equal(t, string(runes[:responsePreviewMaxRunes])+"…", result)
	assert.Equal(t, responsePreviewMaxRunes+1, utf8.RuneCountInString(result)) // maxRunes + ellipsis
}

func TestGetSessionResponsePreview_NoAssistantMessage(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	insertTestMessage(t, db, "session-preview-3", "user", "只有用户消息")

	result := getSessionResponsePreview("session-preview-3")
	assert.Equal(t, "", result)
}

func TestGetSessionResponsePreview_NoMessages(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	result := getSessionResponsePreview("session-nonexistent")
	assert.Equal(t, "", result)
}

func TestGetSessionResponsePreview_SkipsToolUseBlocks(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	toolBlock := model.ContentBlock{Type: "tool_use", Name: "Read", ID: "tool-1"}
	textBlock := model.ContentBlock{Type: "text", Text: "工具执行后的文本"}
	blocks := map[string]any{"blocks": []model.ContentBlock{toolBlock, textBlock}}
	contentJSON, _ := json.Marshal(blocks)
	insertTestMessage(t, db, "session-preview-4", "user", "问题")
	insertTestMessage(t, db, "session-preview-4", "assistant", string(contentJSON))

	result := getSessionResponsePreview("session-preview-4")
	assert.Equal(t, "工具执行后的文本", result)
}

func TestGetSessionResponsePreview_UsesLastAssistantMessage(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	firstContent := model.ContentBlock{Type: "text", Text: "第一次回复"}
	firstBlocks := map[string]any{"blocks": []model.ContentBlock{firstContent}}
	firstJSON, _ := json.Marshal(firstBlocks)
	insertTestMessage(t, db, "session-preview-5", "user", "问题1")
	insertTestMessage(t, db, "session-preview-5", "assistant", string(firstJSON))

	secondContent := model.ContentBlock{Type: "text", Text: "第二次回复"}
	secondBlocks := map[string]any{"blocks": []model.ContentBlock{secondContent}}
	secondJSON, _ := json.Marshal(secondBlocks)
	insertTestMessage(t, db, "session-preview-5", "user", "问题2")
	insertTestMessage(t, db, "session-preview-5", "assistant", string(secondJSON))

	result := getSessionResponsePreview("session-preview-5")
	assert.Equal(t, "第二次回复", result)
}

func TestGetSessionResponsePreview_InvalidJSON(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	insertTestMessage(t, db, "session-preview-6", "user", "问题")
	insertTestMessage(t, db, "session-preview-6", "assistant", "not valid json {{{")

	result := getSessionResponsePreview("session-preview-6")
	assert.Equal(t, "", result)
}

func TestGetSessionResponsePreview_NoTextBlocks(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	toolBlock := model.ContentBlock{Type: "tool_use", Name: "Read", ID: "tool-1"}
	blocks := map[string]any{"blocks": []model.ContentBlock{toolBlock}}
	contentJSON, _ := json.Marshal(blocks)
	insertTestMessage(t, db, "session-preview-7", "user", "问题")
	insertTestMessage(t, db, "session-preview-7", "assistant", string(contentJSON))

	result := getSessionResponsePreview("session-preview-7")
	assert.Equal(t, "", result)
}

func TestGetSessionResponsePreview_ExactMaxRunes(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	// Exactly responsePreviewMaxRunes runes — should NOT be truncated
	exactText := strings.Repeat("一二三四", responsePreviewMaxRunes/4)
	content := model.ContentBlock{Type: "text", Text: exactText}
	blocks := map[string]any{"blocks": []model.ContentBlock{content}}
	contentJSON, _ := json.Marshal(blocks)
	insertTestMessage(t, db, "session-preview-8", "user", "问题")
	insertTestMessage(t, db, "session-preview-8", "assistant", string(contentJSON))

	result := getSessionResponsePreview("session-preview-8")
	assert.Equal(t, exactText, result)
	assert.Equal(t, responsePreviewMaxRunes, utf8.RuneCountInString(result))
}

func TestGetSessionResponsePreview_OneOverMaxRunes(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	// responsePreviewMaxRunes+1 runes — should be truncated to maxRunes + …
	longText := strings.Repeat("一二三四", responsePreviewMaxRunes/4) + "五"
	content := model.ContentBlock{Type: "text", Text: longText}
	blocks := map[string]any{"blocks": []model.ContentBlock{content}}
	contentJSON, _ := json.Marshal(blocks)
	insertTestMessage(t, db, "session-preview-9", "user", "问题")
	insertTestMessage(t, db, "session-preview-9", "assistant", string(contentJSON))

	result := getSessionResponsePreview("session-preview-9")
	assert.Equal(t, strings.Repeat("一二三四", responsePreviewMaxRunes/4)+"…", result)
}

// --- emitSessionEvent with response preview ---

func TestEmitSessionEvent_CompletedWithPreview(t *testing.T) {
	origDB := DB
	db := setupChatTestDB(t)
	DB = db
	defer func() { DB = origDB }()

	// Insert assistant message for preview
	content := model.ContentBlock{Type: "text", Text: "AI完成了任务"}
	blocks := map[string]any{"blocks": []model.ContentBlock{content}}
	contentJSON, _ := json.Marshal(blocks)
	insertTestMessage(t, db, "session-emit-1", "user", "问题")
	insertTestMessage(t, db, "session-emit-1", "assistant", string(contentJSON))

	// Set up ws manager and a subscriber to capture the event
	mgr := ws.NewManagerForTest(nil)
	ws.SetManagerForTest(mgr)
	defer ws.SetManagerForTest(nil)

	var writeMu sync.Mutex
	sub := mgr.Subscribe(nil, &writeMu, "test-client-emit")
	_ = sub

	EmitSessionEvent("session-emit-1", "completed", true)

	// Verify the buffered event has response_preview
	buffered := sub.GetBufferedEvents()
	if len(buffered) == 0 {
		t.Fatal("expected at least one buffered event")
	}
	data, ok := buffered[0].Data.(*ws.SessionUpdateData)
	if !ok {
		t.Fatal("expected SessionUpdateData")
	}
	assert.Equal(t, "completed", data.Status)
	assert.Equal(t, "session-emit-1", data.SessionID)
	assert.Equal(t, "AI完成了任务", data.ResponsePreview)
}

func TestEmitSessionEvent_RunningNoPreview(t *testing.T) {
	mgr := ws.NewManagerForTest(nil)
	ws.SetManagerForTest(mgr)
	defer ws.SetManagerForTest(nil)

	var writeMu sync.Mutex
	sub := mgr.Subscribe(nil, &writeMu, "test-client-emit2")
	_ = sub

	EmitSessionEvent("session-emit-2", "running", false)

	buffered := sub.GetBufferedEvents()
	if len(buffered) == 0 {
		t.Fatal("expected at least one buffered event")
	}
	data, ok := buffered[0].Data.(*ws.SessionUpdateData)
	if !ok {
		t.Fatal("expected SessionUpdateData")
	}
	assert.Equal(t, "running", data.Status)
	assert.Equal(t, "", data.ResponsePreview)
}

// --- GetSessionStream edge cases ---

func TestGetSessionStream_NotRegistered(t *testing.T) {
	cleanupStreams()

	ch, ok := GetSessionStream("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, ch)
}

func TestGetSessionStream_BadType(t *testing.T) {
	cleanupStreams()
	defer cleanupStreams()

	// Store a non-channel value to test type assertion failure
	sessionStreams.Store("bad-type", "not-a-channel")

	ch, ok := GetSessionStream("bad-type")
	assert.False(t, ok, "should return false for wrong type")
	assert.Nil(t, ch)
}

// --- emitSessionEvent with nil ws manager ---

func TestEmitSessionEvent_NilManager(t *testing.T) {
	ws.SetManagerForTest(nil)

	// Should not panic when ws manager is nil
	assert.NotPanics(t, func() {
		EmitSessionEvent("session-nil-mgr", "running", false)
	})
}

// --- CancelSession with bad cancel type ---

func TestCancelSession_BadCancelType(t *testing.T) {
	cleanupAllSessionState()
	defer cleanupAllSessionState()

	// Store a non-CancelFunc value
	sessionCancels.Store("session-bad-cancel", "not-a-cancel-func")
	SetSessionRunning("session-bad-cancel", true)

	result := CancelSession("session-bad-cancel")
	assert.False(t, result, "should return false when cancel func has wrong type")
}

// --- UnregisterSessionStream ---

func TestUnregisterSessionStream(t *testing.T) {
	cleanupStreams()
	defer cleanupStreams()

	ch := RegisterSessionStream("session-unreg")
	UnregisterSessionStream("session-unreg")

	// Channel should be closed
	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after unregister")
}

func TestUnregisterSessionStream_Nonexistent(t *testing.T) {
	cleanupStreams()

	// Should not panic
	assert.NotPanics(t, func() {
		UnregisterSessionStream("nonexistent")
	})
}

// --- SetSessionRunning with skipEvent ---

func TestSetSessionRunning_SkipEventTrue(t *testing.T) {
	cleanupActiveSessions()

	// Set running with skipEvent=true — should NOT emit event
	SetSessionRunning("session-skip", true, true)
	assert.True(t, IsSessionRunning("session-skip"))

	// Stop with skipEvent=true — should NOT emit completed event
	SetSessionRunning("session-skip", false, true)
	assert.False(t, IsSessionRunning("session-skip"))
}

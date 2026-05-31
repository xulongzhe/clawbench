package service

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"clawbench/internal/ai"
	"clawbench/internal/model"
	"clawbench/internal/summarize"
	"clawbench/internal/ws"

	"github.com/stretchr/testify/assert"
)

// --- AsyncSummarize tests ---

// setupTestDBForAsyncSummary creates an in-memory DB with summaries table
func setupTestDBForAsyncSummary(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	origDB := DB
	origDBRead := DBRead

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, _ = db.Exec("PRAGMA busy_timeout=5000")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS summaries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			target_type TEXT NOT NULL,
			target_id   INTEGER NOT NULL,
			summary     TEXT NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(target_type, target_id)
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	DB = db
	DBRead = db
	teardown := func() {
		DB = origDB
		DBRead = origDBRead
		db.Close()
	}
	return db, teardown
}

// mockAsyncSummarizerBackend is a mock AI backend for AsyncSummarize tests
type mockAsyncSummarizerBackend struct {
	streamCh   chan ai.StreamEvent
	executeErr error
}

func (m *mockAsyncSummarizerBackend) Name() string { return "mock-async" }

func (m *mockAsyncSummarizerBackend) ExecuteStream(ctx context.Context, req ai.ChatRequest) (<-chan ai.StreamEvent, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return m.streamCh, nil
}

func TestAsyncSummarize_ShortText(t *testing.T) {
	_, dbTeardown := setupTestDBForAsyncSummary(t)
	defer dbTeardown()

	origInstance := taskSummarizerInstance
	defer func() { taskSummarizerInstance = origInstance }()

	// Create a TaskSummarizer with mock backend
	ch := make(chan ai.StreamEvent, 1)
	ch <- ai.StreamEvent{Type: "done"}
	close(ch)
	mock := &mockAsyncSummarizerBackend{streamCh: ch}
	taskSummarizerInstance = &summarize.TaskSummarizer{Backend: mock}

	// Short text block — should save empty summary
	blocks := []model.ContentBlock{{Type: "text", Text: "短"}}

	var wg sync.WaitGroup
	wg.Add(1)
	origBroadcast := ws.GetManager()
	_ = origBroadcast // We don't test WS here, just DB

	AsyncSummarize("chat_message", 1, blocks, "/test", "session-1")

	// Wait for goroutine to complete
	time.Sleep(200 * time.Millisecond)

	summary, found := GetSummary("chat_message", 1)
	assert.True(t, found)
	assert.Equal(t, "", summary) // short text = empty summary
}

func TestAsyncSummarize_NormalText(t *testing.T) {
	_, dbTeardown := setupTestDBForAsyncSummary(t)
	defer dbTeardown()

	origInstance := taskSummarizerInstance
	defer func() { taskSummarizerInstance = origInstance }()

	// Create mock backend that returns a summary
	ch := make(chan ai.StreamEvent, 3)
	ch <- ai.StreamEvent{Type: "content", Content: "## 精简总结\n\n关键结论。"}
	ch <- ai.StreamEvent{Type: "done"}
	close(ch)
	mock := &mockAsyncSummarizerBackend{streamCh: ch}
	taskSummarizerInstance = &summarize.TaskSummarizer{Backend: mock}

	// Long text block
	longText := strings.Repeat("这是一段较长的AI回复内容。", 30)
	blocks := []model.ContentBlock{{Type: "text", Text: longText}}

	AsyncSummarize("chat_message", 2, blocks, "/test", "session-2")

	// Wait for goroutine to complete
	time.Sleep(200 * time.Millisecond)

	summary, found := GetSummary("chat_message", 2)
	assert.True(t, found)
	assert.Contains(t, summary, "精简总结")
}

func TestAsyncSummarize_NilSummarizer(t *testing.T) {
	_, dbTeardown := setupTestDBForAsyncSummary(t)
	defer dbTeardown()

	origInstance := taskSummarizerInstance
	defer func() { taskSummarizerInstance = origInstance }()

	// nil summarizer — should return immediately, no goroutine
	taskSummarizerInstance = nil

	blocks := []model.ContentBlock{{Type: "text", Text: "some text"}}

	// Should not panic or create goroutine
	AsyncSummarize("chat_message", 3, blocks, "/test", "session-3")

	time.Sleep(100 * time.Millisecond)

	// No summary should be saved
	_, found := GetSummary("chat_message", 3)
	assert.False(t, found)
}

func TestAsyncSummarize_BackendError(t *testing.T) {
	_, dbTeardown := setupTestDBForAsyncSummary(t)
	defer dbTeardown()

	origInstance := taskSummarizerInstance
	defer func() { taskSummarizerInstance = origInstance }()

	// Mock backend that returns error
	mock := &mockAsyncSummarizerBackend{executeErr: context.DeadlineExceeded}
	taskSummarizerInstance = &summarize.TaskSummarizer{Backend: mock}

	longText := strings.Repeat("这是一段较长的AI回复内容。", 30)
	blocks := []model.ContentBlock{{Type: "text", Text: longText}}

	AsyncSummarize("chat_message", 4, blocks, "/test", "session-4")

	time.Sleep(200 * time.Millisecond)

	// No summary saved on error
	_, found := GetSummary("chat_message", 4)
	assert.False(t, found)
}

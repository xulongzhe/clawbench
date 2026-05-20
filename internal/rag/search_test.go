package rag

import (
	"context"
	"database/sql"
	"testing"

	"clawbench/internal/service"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- Helpers ----------

// setupSearchDB creates an in-memory SQLite with sessions for search tests.
func setupSearchDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	_, err = db.Exec(`
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
	`)
	require.NoError(t, err)

	origDB := service.DB
	service.DB = db
	t.Cleanup(func() {
		service.DB = origDB
		db.Close()
	})
	return db
}

// ---------- RAGSearch ----------

func TestRAGSearch_EmptyQuery(t *testing.T) {
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: ""}, 5, 20)
	assert.NoError(t, err)
	assert.Empty(t, result.Results)
	assert.Equal(t, 0, result.Total)
}

func TestRAGSearch_DefaultLimit(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	// Insert some chunks and build FTS index
	insertTestChunks(t, store, 3)
	require.NoError(t, store.CreateFTSIndex())

	// Search with limit=0 should use defaultLimit
	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 0}, 5, 20)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Results), 5)
}

func TestRAGSearch_WithResults(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	insertTestChunks(t, store, 2)
	require.NoError(t, store.CreateFTSIndex())

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 10}, 5, 20)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Results)
	assert.Equal(t, len(result.Results), result.Total)
	// Should use hybrid or vector mode since Ollama is healthy
	assert.Contains(t, []SearchMode{SearchModeHybrid, SearchModeVector, SearchModeFTS}, result.Mode)
}

func TestRAGSearch_FTSOnly(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)

	// Insert chunks and build FTS index
	insertTestChunks(t, store, 3)
	require.NoError(t, store.CreateFTSIndex())

	// Use nil embedder (Ollama not available) — should fall back to FTS-only
	result, err := RAGSearch(context.Background(), store, nil, SearchParams{Query: "chunk", Limit: 5}, 5, 20)
	assert.NoError(t, err)
	assert.Equal(t, SearchModeFTS, result.Mode)
	assert.NotEmpty(t, result.Results)
}

func TestRAGSearch_NilStore(t *testing.T) {
	_, err := RAGSearch(context.Background(), nil, nil, SearchParams{Query: "test"}, 5, 20)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RAG not initialized")
}

func TestRAGSearch_NilStoreEmptyQuery(t *testing.T) {
	// Empty query should return before nil check
	result, err := RAGSearch(context.Background(), nil, nil, SearchParams{Query: ""}, 5, 20)
	assert.NoError(t, err)
	assert.Empty(t, result.Results)
}

func TestRAGSearch_SearchModeField(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	insertTestChunks(t, store, 3)
	require.NoError(t, store.CreateFTSIndex())

	// With healthy Ollama — should be hybrid
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "chunk", Limit: 5}, 5, 20)
	require.NoError(t, err)
	// With both FTS and Ollama available, should use hybrid
	assert.Equal(t, SearchModeHybrid, result.Mode)
}

// ---------- getSessionTitles ----------

func TestGetSessionTitles(t *testing.T) {
	setupSearchDB(t)

	// Create a session with known title
	sid, err := service.CreateSession("/test", "claude", "Test Session Title", "", "", "default", "chat")
	require.NoError(t, err)

	titles := getSessionTitles(map[string]bool{sid: true})
	assert.Equal(t, "Test Session Title", titles[sid])
}

func TestGetSessionTitles_MissingSession(t *testing.T) {
	setupSearchDB(t)

	titles := getSessionTitles(map[string]bool{"nonexistent": true})
	_, ok := titles["nonexistent"]
	assert.False(t, ok, "missing session should not appear in titles")
}

func TestGetSessionTitles_Empty(t *testing.T) {
	setupSearchDB(t)

	titles := getSessionTitles(map[string]bool{})
	assert.Empty(t, titles)
}

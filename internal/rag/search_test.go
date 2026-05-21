package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// ---------- RAGSearch ----------

func TestRAGSearch_EmptyQuery(t *testing.T) {
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: ""}, 5, 20)
	assert.NoError(t, err)
	assert.Empty(t, result.Results)
	assert.Equal(t, 0, result.Total)
}

func TestRAGSearch_DefaultLimit(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}
	embedder, cleanup := newHealthyMockEmbedder(t)
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
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	insertTestChunks(t, store, 2)
	require.NoError(t, store.CreateFTSIndex())

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 10}, 5, 20)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Results)
	assert.Equal(t, len(result.Results), result.Total)
	// Should use hybrid or vector mode since embedder is healthy
	assert.Contains(t, []SearchMode{SearchModeHybrid, SearchModeVector, SearchModeFTS}, result.Mode)
}

func TestRAGSearch_FTSOnly(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}
	SetEmbedderHealthy(false) // Ensure cached state doesn't interfere

	// Insert chunks and build FTS index
	insertTestChunks(t, store, 3)
	require.NoError(t, store.CreateFTSIndex())

	// Use nil embedder (embedder not available) — should fall back to FTS-only
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

	// With healthy embedder — should be hybrid
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "chunk", Limit: 5}, 5, 20)
	require.NoError(t, err)
	// With both FTS and embedder available, should use hybrid
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

// ---------- RAGSearch vector-only mode ----------

func TestRAGSearch_VectorOnlyMode(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)

	// Set embedder healthy but FTS not available
	SetEmbedderHealthy(true)
	t.Cleanup(func() { SetEmbedderHealthy(false) })
	store.ftsAvailable = false

	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	// Insert chunks with embeddings
	insertTestChunks(t, store, 2)

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 5}, 5, 20)
	require.NoError(t, err)
	assert.Equal(t, SearchModeVector, result.Mode)
}

// ---------- RAGSearch with no search available ----------

func TestRAGSearch_NoSearchAvailable(t *testing.T) {
	store := setupTestStore(t)
	store.ftsAvailable = false
	SetEmbedderHealthy(false)

	// nil embedder — no search available
	_, err := RAGSearch(context.Background(), store, nil, SearchParams{Query: "test"}, 5, 20)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no search available")
}

// ---------- RAGSearch with default poolSize ----------

func TestRAGSearch_DefaultPoolSize(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	insertTestChunks(t, store, 2)
	require.NoError(t, store.CreateFTSIndex())

	// poolSize=0 should use default of 20
	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 5}, 5, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Results)
}

// ---------- RAGSearch embedding fallback to FTS ----------

func TestRAGSearch_EmbeddingFailsFallsBackToFTS(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Server where embedding endpoint fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(openaiModelsResponse{Data: []openaiModelInfo{{ID: "bge-m3:latest"}}})
		} else if r.URL.Path == "/v1/embeddings" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	embedder := NewEmbeddingClient(server.URL, "bge-m3", "")
	embedder.HTTPClient = server.Client()
	SetEmbedderHealthy(true)
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	insertTestChunks(t, store, 3)
	require.NoError(t, store.CreateFTSIndex())

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "chunk", Limit: 5}, 5, 20)
	require.NoError(t, err)
	assert.Equal(t, SearchModeFTS, result.Mode, "should fall back to FTS when embedding fails")
}

// ---------- RAGSearch with cached embedder health ----------

func TestRAGSearch_CachedEmbedderHealth(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Set cached embedder health to true
	SetEmbedderHealthy(true)
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	insertTestChunks(t, store, 2)
	require.NoError(t, store.CreateFTSIndex())

	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 5}, 5, 20)
	require.NoError(t, err)
	// With cached healthy state, should use hybrid or vector mode
	assert.Contains(t, []SearchMode{SearchModeHybrid, SearchModeVector}, result.Mode)
}

// ---------- RAGSearch vector-only (no FTS) ----------

func TestRAGSearch_VectorOnlyNoFTS(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)
	SetEmbedderHealthy(true)
	t.Cleanup(func() { SetEmbedderHealthy(false) })
	store.ftsAvailable = false

	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	insertTestChunks(t, store, 2)

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 5}, 5, 20)
	require.NoError(t, err)
	assert.Equal(t, SearchModeVector, result.Mode)
}

// ---------- RAGSearch with vector-only embedding error ----------

func TestRAGSearch_VectorOnlyEmbeddingError(t *testing.T) {
	store := setupTestStore(t)
	SetEmbedderHealthy(true)
	t.Cleanup(func() { SetEmbedderHealthy(false) })
	store.ftsAvailable = false

	// Embedding will fail
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(openaiModelsResponse{Data: []openaiModelInfo{{ID: "bge-m3:latest"}}})
		} else if r.URL.Path == "/v1/embeddings" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	embedder := NewEmbeddingClient(server.URL, "bge-m3", "")
	embedder.HTTPClient = server.Client()

	insertTestChunks(t, store, 2)

	_, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 5}, 5, 20)
	assert.Error(t, err, "should error when embedding fails in vector-only mode")
}

// ---------- getSessionTitles batch failure fallback ----------

func TestGetSessionTitles_BatchFailureFallback(t *testing.T) {
	db := setupSearchDB(t)

	// Create a session with known title
	sid, err := service.CreateSession("/test", "claude", "Batch Fallback Title", "", "", "default", "chat")
	require.NoError(t, err)

	// Drop the chat_sessions table to make batch query fail
	db.Exec("DROP TABLE chat_sessions")

	// Should fall back gracefully (return empty map)
	titles := getSessionTitles(map[string]bool{sid: true})
	_, ok := titles[sid]
	assert.False(t, ok, "should handle batch failure gracefully")
}

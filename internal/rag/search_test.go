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

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: ""}, 5)
	assert.NoError(t, err)
	assert.Empty(t, result.Results)
	assert.Equal(t, 0, result.Total)
}

func TestRAGSearch_DefaultLimit(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)

	// Mock embedder that returns 1024-dim vectors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaEmbedResponse{Embedding: makeTestEmbedding(1024)}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()
	embedder := NewEmbeddingClient(server.URL, "bge-m3")
	embedder.HTTPClient = server.Client()

	// Insert some chunks
	insertTestChunks(t, store, 3)

	// Search with limit=0 should use defaultLimit
	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 0}, 5)
	assert.NoError(t, err)
	assert.LessOrEqual(t, len(result.Results), 5)
}

func TestRAGSearch_WithResults(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaEmbedResponse{Embedding: makeTestEmbedding(1024)}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()
	embedder := NewEmbeddingClient(server.URL, "bge-m3")
	embedder.HTTPClient = server.Client()

	insertTestChunks(t, store, 2)

	result, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test", Limit: 10}, 5)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.Results)
	assert.Equal(t, len(result.Results), result.Total)
}

func TestRAGSearch_EmbedError(t *testing.T) {
	setupSearchDB(t)
	store := setupTestStore(t)

	// Server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	embedder := NewEmbeddingClient(server.URL, "bge-m3")
	embedder.HTTPClient = server.Client()

	_, err := RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test"}, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embed query")
}

func TestRAGSearch_SearchError(t *testing.T) {
	// Use a store that's been closed — should error on search
	dir := t.TempDir()
	store, err := NewStore(dir + "/test.duckdb")
	require.NoError(t, err)
	insertTestChunks(t, store, 1)
	store.Close() // Close the store

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaEmbedResponse{Embedding: makeTestEmbedding(1024)}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()
	embedder := NewEmbeddingClient(server.URL, "bge-m3")
	embedder.HTTPClient = server.Client()

	_, err = RAGSearch(context.Background(), store, embedder, SearchParams{Query: "test"}, 5)
	assert.Error(t, err)
}

func TestRAGSearch_NilStore(t *testing.T) {
	_, err := RAGSearch(context.Background(), nil, nil, SearchParams{Query: "test"}, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RAG not initialized")
}

func TestRAGSearch_NilStoreEmptyQuery(t *testing.T) {
	// Empty query should return before nil check
	result, err := RAGSearch(context.Background(), nil, nil, SearchParams{Query: ""}, 5)
	assert.NoError(t, err)
	assert.Empty(t, result.Results)
}

// ---------- getSessionTitles ----------

func TestGetSessionTitles(t *testing.T) {
	setupSearchDB(t)

	// Create a session with known title
	sid, err := service.CreateSession("/test", "claude", "Test Session Title", "", "", "default")
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

package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/service"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- Helpers ----------

// setupIndexerDB creates an in-memory SQLite with the required schema
// for indexer tests and swaps service.DB.
func setupIndexerDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
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
			deleted INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_read_at DATETIME,
			UNIQUE(project_path, backend, id)
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

// newHealthyMockOllama creates a mock server that responds to both
// /api/tags (health) and /api/embeddings (embed).
func newHealthyMockOllama(t *testing.T) (*EmbeddingClient, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			resp := ollamaTagsResponse{
				Models: []ollamaModelInfo{{Name: "bge-m3:latest"}},
			}
			json.NewEncoder(w).Encode(resp)
		case "/api/embeddings":
			resp := ollamaEmbedResponse{Embedding: makeTestEmbedding(1024)}
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	client := NewEmbeddingClient(server.URL, "bge-m3")
	client.HTTPClient = server.Client()
	return client, server.Close
}

// addUnindexedMessage inserts an unindexed message into chat_history.
func addUnindexedMessage(t *testing.T, db *sql.DB, sessionID, role, content string) int64 {
	t.Helper()
	result, err := db.Exec(
		"INSERT INTO chat_history (project_path, role, content, session_id, backend, streaming, indexed) VALUES (?, ?, ?, ?, 'claude', 0, 0)",
		"/test", role, content, sessionID,
	)
	require.NoError(t, err)
	id, _ := result.LastInsertId()
	return id
}

// ---------- NewIndexer ----------

func TestNewIndexer_Construction(t *testing.T) {
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{
		PollInterval: "10s",
		BatchSize:    10,
		ChunkSize:    512,
		ChunkOverlap: 64,
	})

	assert.NotNil(t, idx)
	assert.False(t, idx.running)
}

// ---------- Start/Stop lifecycle ----------

func TestIndexer_StartStop(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{
		PollInterval: "24h", // long interval so indexBatch only runs once
		BatchSize:    10,
		ChunkSize:    512,
		ChunkOverlap: 64,
	})

	idx.Start()
	assert.True(t, idx.running)

	// Stop should complete
	idx.Stop()
	assert.False(t, idx.running)
}

func TestIndexer_DoubleStart(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{PollInterval: "24h"})

	idx.Start()
	idx.Start() // should be no-op
	assert.True(t, idx.running)

	idx.Stop()
}

func TestIndexer_DoubleStop(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{PollInterval: "24h"})

	// Stop before start — should be no-op
	idx.Stop()

	idx.Start()
	idx.Stop()
	idx.Stop() // should be no-op
	assert.False(t, idx.running)
}

// ---------- indexBatch ----------

func TestIndexer_indexBatch_OllamaNotReachable(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	// Use a client pointing to a non-existent server
	embedder := NewEmbeddingClient("http://127.0.0.1:1", "bge-m3")

	idx := NewIndexer(store, embedder, model.RAGConfig{PollInterval: "10s"})

	// indexBatch should return early when Ollama is unreachable
	idx.indexBatch()
	// No panic, no error — just a silent skip
}

func TestIndexer_indexBatch_ModelNotAvailable(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	// Mock server where model is not available
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaTagsResponse{
			Models: []ollamaModelInfo{{Name: "other-model:latest"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewEmbeddingClient(server.URL, "bge-m3")
	embedder.HTTPClient = server.Client()

	idx := NewIndexer(store, embedder, model.RAGConfig{PollInterval: "10s"})
	idx.indexBatch()

	// Should skip without indexing
	count, _ := store.ChunkCount()
	assert.Equal(t, 0, count, "should not index when model not available")
}

// ---------- indexMessage ----------

func TestIndexer_indexMessage_EmptyContent(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{ChunkSize: 512})

	// Message with only tool_use blocks (no text)
	msg := service.UnindexedMessage{
		ID:          1,
		Content:     `{"blocks":[{"type":"tool_use","name":"Read","id":"t1"}]}`,
		Role:        "assistant",
		SessionID:   "sess-1",
		ProjectPath: "/test",
		Backend:     "claude",
		CreatedAt:   time.Now(),
	}

	err := idx.indexMessage(context.Background(), msg)
	assert.NoError(t, err, "empty content message should be skipped without error")

	count, _ := store.ChunkCount()
	assert.Equal(t, 0, count, "should not index messages with no text content")
}

func TestIndexer_indexMessage_Success(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockOllama(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{ChunkSize: 512, ChunkOverlap: 64})

	msg := service.UnindexedMessage{
		ID:          1,
		Content:     "This is a test message with some content to index.",
		Role:        "user",
		SessionID:   "sess-1",
		ProjectPath: "/test",
		Backend:     "claude",
		CreatedAt:   time.Now(),
	}

	err := idx.indexMessage(context.Background(), msg)
	assert.NoError(t, err)

	count, _ := store.ChunkCount()
	assert.Equal(t, 1, count, "should have indexed 1 chunk")
}

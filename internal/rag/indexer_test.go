package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
			session_type TEXT NOT NULL DEFAULT 'chat',
			deleted INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_read_at DATETIME,
			UNIQUE(project_path, backend, id)
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

// newHealthyMockEmbedder creates a mock server that responds to both
// /v1/models (health) and /v1/embeddings (embed).
func newHealthyMockEmbedder(t *testing.T) (*EmbeddingClient, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			resp := openaiModelsResponse{
				Data: []openaiModelInfo{{ID: "bge-m3:latest"}},
			}
			json.NewEncoder(w).Encode(resp)
		case "/v1/embeddings":
			var req openaiEmbedRequest
			json.NewDecoder(r.Body).Decode(&req)
			data := make([]openaiEmbeddingData, len(req.Input))
			for i := range req.Input {
				data[i] = openaiEmbeddingData{Embedding: makeTestEmbedding(1024), Index: i}
			}
			resp := openaiEmbedResponse{Data: data}
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	client := NewEmbeddingClient(server.URL, "bge-m3", "")
	client.HTTPClient = server.Client()
	return client, server.Close
}

// ---------- NewIndexer ----------

func TestNewIndexer_Construction(t *testing.T) {
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
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
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{
		PollInterval: "24h", // long interval so indexBatch only runs once
		BatchSize:    10,
		ChunkSize:    512,
		ChunkOverlap: 64,
	})
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	idx.Start()
	assert.True(t, idx.running)

	// Stop should complete
	idx.Stop()
	assert.False(t, idx.running)
}

func TestIndexer_DoubleStart(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{PollInterval: "24h"})
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	idx.Start()
	idx.Start() // should be no-op
	assert.True(t, idx.running)

	idx.Stop()
}

func TestIndexer_DoubleStop(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{PollInterval: "24h"})
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	// Stop before start — should be no-op
	idx.Stop()

	idx.Start()
	idx.Stop()
	idx.Stop() // should be no-op
	assert.False(t, idx.running)
}

// ---------- indexBatch ----------

func TestIndexer_indexBatch_EmbedderNotReachable(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	// Use a client pointing to a non-existent server
	embedder := NewEmbeddingClient("http://127.0.0.1:1", "bge-m3", "")

	idx := NewIndexer(store, embedder, model.RAGConfig{PollInterval: "10s"})
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	// indexBatch should return early when embedder is unreachable
	idx.indexBatch()
	// No panic, no error — just a silent skip
}

func TestIndexer_indexBatch_ModelNotAvailable(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	// Mock server where model is not available
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiModelsResponse{
			Data: []openaiModelInfo{{ID: "other-model:latest"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewEmbeddingClient(server.URL, "bge-m3", "")
	embedder.HTTPClient = server.Client()

	idx := NewIndexer(store, embedder, model.RAGConfig{PollInterval: "10s"})
	t.Cleanup(func() { SetEmbedderHealthy(false) })
	idx.indexBatch()

	// Should skip without indexing
	count, _ := store.ChunkCount()
	assert.Equal(t, 0, count, "should not index when model not available")
}

// ---------- indexMessage ----------

func TestIndexer_indexMessage_EmptyContent(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
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
	embedder, cleanup := newHealthyMockEmbedder(t)
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

// ---------- checkEmbedderHealth ----------

func TestIndexer_checkEmbedderHealth_HealthyTransition(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{})
	assert.False(t, idx.embedderHealthy)
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	idx.checkEmbedderHealth(context.Background())
	assert.True(t, idx.embedderHealthy)
	assert.True(t, EmbedderHealthy())
}

func TestIndexer_checkEmbedderHealth_Error(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	// Server that returns 500 on /v1/models
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	embedder := NewEmbeddingClient(server.URL, "bge-m3", "")
	embedder.HTTPClient = server.Client()

	idx := NewIndexer(store, embedder, model.RAGConfig{})
	idx.embedderHealthy = true

	idx.checkEmbedderHealth(context.Background())
	assert.False(t, idx.embedderHealthy)
	assert.False(t, EmbedderHealthy())
}

func TestIndexer_checkEmbedderHealth_UnreachableBecomesReachable(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{})
	idx.embedderHealthy = false
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	idx.checkEmbedderHealth(context.Background())
	assert.True(t, idx.embedderHealthy, "should transition to healthy")
}

func TestIndexer_checkEmbedderHealth_ReachableBecomesUnreachable(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	// Unreachable server
	embedder := NewEmbeddingClient("http://127.0.0.1:1", "bge-m3", "")

	idx := NewIndexer(store, embedder, model.RAGConfig{})
	idx.embedderHealthy = true

	idx.checkEmbedderHealth(context.Background())
	assert.False(t, idx.embedderHealthy, "should transition to unhealthy")
}

func TestIndexer_checkEmbedderHealth_ModelNotAvailable(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiModelsResponse{
			Data: []openaiModelInfo{{ID: "other-model:latest"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewEmbeddingClient(server.URL, "bge-m3", "")
	embedder.HTTPClient = server.Client()

	idx := NewIndexer(store, embedder, model.RAGConfig{})
	idx.checkEmbedderHealth(context.Background())

	assert.False(t, idx.embedderHealthy)
	assert.True(t, idx.modelWarn, "should set modelWarn flag")
}

// ---------- indexNewMessages ----------

func TestIndexer_indexNewMessages_WithMessages(t *testing.T) {
	db := setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{
		ChunkSize:    512,
		ChunkOverlap: 64,
		BatchSize:    10,
	})
	idx.embedderHealthy = true

	// Insert unindexed messages into SQLite
	now := time.Now().Truncate(time.Millisecond)
	_, err := db.Exec(`INSERT INTO chat_history (project_path, role, content, session_id, backend, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"/test", "user", "Hello, this is a test message", "sess-1", "claude", now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO chat_history (project_path, role, content, session_id, backend, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"/test", "assistant", `{"blocks":[{"type":"text","text":"This is the response"}]}`, "sess-1", "claude", now)
	require.NoError(t, err)

	idx.indexNewMessages(context.Background())

	count, _ := store.ChunkCount()
	assert.Equal(t, 2, count, "should index both messages")

	// Verify messages are marked as indexed
	var indexedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM chat_history WHERE indexed = 1").Scan(&indexedCount)
	assert.NoError(t, err)
	assert.Equal(t, 2, indexedCount, "messages should be marked as indexed")
}

func TestIndexer_indexNewMessages_NoMessages(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{BatchSize: 10})
	idx.embedderHealthy = true

	// No messages — should not panic
	idx.indexNewMessages(context.Background())

	count, _ := store.ChunkCount()
	assert.Equal(t, 0, count)
}

func TestIndexer_indexNewMessages_EmptyContentSkipped(t *testing.T) {
	db := setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{ChunkSize: 512, BatchSize: 10})
	idx.embedderHealthy = true

	// Insert a message with only tool_use blocks (no text content)
	_, err := db.Exec(`INSERT INTO chat_history (project_path, role, content, session_id, backend, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"/test", "assistant", `{"blocks":[{"type":"tool_use","name":"Read","id":"t1"}]}`, "sess-1", "claude", time.Now())
	require.NoError(t, err)

	idx.indexNewMessages(context.Background())

	// The message should be marked indexed but no chunks should be stored
	var indexedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM chat_history WHERE indexed = 1").Scan(&indexedCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, indexedCount, "message should be marked indexed")

	count, _ := store.ChunkCount()
	assert.Equal(t, 0, count, "no chunks for text-less message")
}

func TestIndexer_indexNewMessages_WithoutEmbedder(t *testing.T) {
	db := setupIndexerDB(t)
	store := setupTestStore(t)

	embedder := NewEmbeddingClient("http://127.0.0.1:1", "bge-m3", "")

	idx := NewIndexer(store, embedder, model.RAGConfig{ChunkSize: 512, BatchSize: 10})
	idx.embedderHealthy = false // Embedder not healthy — text-only indexing

	// Insert a message
	_, err := db.Exec(`INSERT INTO chat_history (project_path, role, content, session_id, backend, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"/test", "user", "Text-only indexing test", "sess-1", "claude", time.Now())
	require.NoError(t, err)

	idx.indexNewMessages(context.Background())

	count, _ := store.ChunkCount()
	assert.Equal(t, 1, count, "should still index text-only when embedder is down")

	// Verify chunk has no embedding
	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 1, pending, "chunk should need embedding backfill")
}

// ---------- backfillEmbeddings ----------

func TestIndexer_backfillEmbeddings_Success(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{BatchSize: 10})
	idx.embedderHealthy = true

	// Insert a chunk without embedding (simulating text-only indexing)
	chunk := Chunk{
		SessionID:          "sess-1",
		MessageID:          1,
		ChunkText:          "needs backfill",
		ChunkTextSegmented: "needs backfill",
		ChunkIndex:         0,
		TokenCount:         3,
		Embedding:          nil,
		HasEmbedding:       false,
		ProjectPath:        "/test",
		Backend:            "claude",
		Role:               "assistant",
		CreatedAt:          time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 1, pending, "should have 1 pending before backfill")

	idx.backfillEmbeddings(context.Background())

	pending, _ = store.PendingEmbeddingCount()
	assert.Equal(t, 0, pending, "all embeddings should be backfilled")
}

func TestIndexer_backfillEmbeddings_NoPending(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{BatchSize: 10})
	idx.embedderHealthy = true

	// No pending embeddings — should be a no-op
	idx.backfillEmbeddings(context.Background())
}

func TestIndexer_backfillEmbeddings_EmbeddingFails(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	// Server that returns 500 on embeddings endpoint
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

	idx := NewIndexer(store, embedder, model.RAGConfig{BatchSize: 10})
	idx.embedderHealthy = true

	// Insert a chunk without embedding
	chunk := Chunk{
		SessionID:          "sess-1",
		MessageID:          1,
		ChunkText:          "needs backfill",
		ChunkTextSegmented: "needs backfill",
		ChunkIndex:         0,
		TokenCount:         3,
		Embedding:          nil,
		HasEmbedding:       false,
		ProjectPath:        "/test",
		Backend:            "claude",
		Role:               "assistant",
		CreatedAt:          time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	// backfill should not panic when embedding fails
	idx.backfillEmbeddings(context.Background())

	// Pending count should remain the same
	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 1, pending, "embedding should still be pending after failure")
}

// ---------- run (invalid poll interval) ----------

func TestIndexer_run_InvalidPollInterval(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{
		PollInterval: "not-a-duration",
		BatchSize:    10,
		ChunkSize:    512,
	})

	// Start and stop — should use 10s default and not panic
	idx.Start()
	assert.True(t, idx.running)
	idx.Stop()
	assert.False(t, idx.running)
}

// ---------- indexMessage with embedding failure ----------

func TestIndexer_indexMessage_EmbeddingFails(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	// Server where embedding endpoint returns 500
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

	idx := NewIndexer(store, embedder, model.RAGConfig{ChunkSize: 512})
	idx.embedderHealthy = true

	msg := service.UnindexedMessage{
		ID:          1,
		Content:     "Test message when embedding fails",
		Role:        "user",
		SessionID:   "sess-1",
		ProjectPath: "/test",
		Backend:     "claude",
		CreatedAt:   time.Now(),
	}

	err := idx.indexMessage(context.Background(), msg)
	assert.NoError(t, err, "should not error when embedding fails — stores text-only")

	count, _ := store.ChunkCount()
	assert.Equal(t, 1, count, "should store chunk even without embedding")

	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 1, pending, "chunk should need embedding backfill")
}

func TestIndexer_indexMessage_AssistantWithText(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{ChunkSize: 512})
	idx.embedderHealthy = true

	// Assistant message with text blocks
	msg := service.UnindexedMessage{
		ID:          1,
		Content:     `{"blocks":[{"type":"text","text":"Here is the answer."},{"type":"tool_use","name":"Read","id":"t1"}]}`,
		Role:        "assistant",
		SessionID:   "sess-1",
		ProjectPath: "/test",
		Backend:     "claude",
		CreatedAt:   time.Now(),
	}

	err := idx.indexMessage(context.Background(), msg)
	assert.NoError(t, err)

	count, _ := store.ChunkCount()
	assert.Equal(t, 1, count, "should index text from assistant message")
}

func TestIndexer_indexMessage_LargeMessage(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{ChunkSize: 20, ChunkOverlap: 5})
	idx.embedderHealthy = true

	// Generate a message that will produce many chunks
	longText := ""
	for i := 0; i < 100; i++ {
		longText += "This is sentence number that adds to the total content. "
	}

	msg := service.UnindexedMessage{
		ID:          1,
		Content:     longText,
		Role:        "user",
		SessionID:   "sess-1",
		ProjectPath: "/test",
		Backend:     "claude",
		CreatedAt:   time.Now(),
	}

	err := idx.indexMessage(context.Background(), msg)
	assert.NoError(t, err)

	count, _ := store.ChunkCount()
	assert.Greater(t, count, 0, "should index chunks from large message")
}

// ---------- backfillEmbeddings edge cases ----------

func TestIndexer_backfillEmbeddings_ZeroBatchSize(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{BatchSize: 0})
	idx.embedderHealthy = true

	// Insert a chunk without embedding
	chunk := Chunk{
		SessionID: "sess-1", MessageID: 1, ChunkText: "test backfill",
		ChunkTextSegmented: "test backfill", ChunkIndex: 0, TokenCount: 3,
		Embedding: nil, HasEmbedding: false,
		ProjectPath: "/test", Backend: "claude", Role: "assistant",
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	// Should use default batch size (10) when configured batch size is 0
	idx.backfillEmbeddings(context.Background())

	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 0, pending, "should backfill even with batch size 0 (uses default)")
}

func TestIndexer_backfillEmbeddings_LargeBatchSize(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	// Batch size > 50 should be capped at 50
	idx := NewIndexer(store, embedder, model.RAGConfig{BatchSize: 100})
	idx.embedderHealthy = true

	// Insert a single chunk without embedding
	chunk := Chunk{
		SessionID: "sess-1", MessageID: 1, ChunkText: "test cap",
		ChunkTextSegmented: "test cap", ChunkIndex: 0, TokenCount: 3,
		Embedding: nil, HasEmbedding: false,
		ProjectPath: "/test", Backend: "claude", Role: "assistant",
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	idx.backfillEmbeddings(context.Background())

	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 0, pending, "should backfill with capped batch size")
}

// ---------- checkEmbedderHealth dimension sync ----------

func TestIndexer_checkEmbedderHealth_DimensionSync(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{})
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	// First: make an embed call so the embedder auto-detects its dimension
	_, err := embedder.Embed(context.Background(), "test")
	require.NoError(t, err)
	require.Greater(t, embedder.Dim(), 0, "embedder should have auto-detected dimension")

	// Now check health — should sync dimension to store
	idx.checkEmbedderHealth(context.Background())
	assert.True(t, idx.embedderHealthy)
	assert.True(t, idx.dimensionSynced, "dimension should be synced after first healthy check")
	assert.Equal(t, embedder.Dim(), store.embeddingDim)
}

func TestIndexer_checkEmbedderHealth_DimensionSyncOnlyOnce(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{})
	t.Cleanup(func() { SetEmbedderHealthy(false) })

	// Make an embed call so the embedder auto-detects its dimension
	_, err := embedder.Embed(context.Background(), "test")
	require.NoError(t, err)

	// First check — should sync
	idx.checkEmbedderHealth(context.Background())
	assert.True(t, idx.dimensionSynced)
	origDim := store.embeddingDim

	// Change the store dimension manually — second check should NOT re-sync
	store.embeddingDim = 999
	idx.checkEmbedderHealth(context.Background())
	// dimensionSynced remains true, so the dim should not have been overwritten
	assert.Equal(t, 999, store.embeddingDim, "dimension should not be re-synced after first sync")
	assert.NotEqual(t, origDim, 999)
}

func TestIndexer_checkEmbedderHealth_ModelNotAvailableRepeated(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiModelsResponse{
			Data: []openaiModelInfo{{ID: "other-model:latest"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewEmbeddingClient(server.URL, "bge-m3", "")
	embedder.HTTPClient = server.Client()

	idx := NewIndexer(store, embedder, model.RAGConfig{})
	idx.checkEmbedderHealth(context.Background())
	assert.True(t, idx.modelWarn, "should set modelWarn on first not-available check")

	// Second call — modelWarn should already be true, should not log again
	idx.checkEmbedderHealth(context.Background())
	assert.True(t, idx.modelWarn, "modelWarn should remain true")
}

// ---------- indexMessage edge cases ----------

func TestIndexer_indexMessage_EmbeddingSuccessWithChunks(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{ChunkSize: 512})
	idx.embedderHealthy = true

	msg := service.UnindexedMessage{
		ID:          1,
		Content:     "This is a test message for embedding success.",
		Role:        "user",
		SessionID:   "sess-1",
		ProjectPath: "/test",
		Backend:     "claude",
		CreatedAt:   time.Now(),
	}

	err := idx.indexMessage(context.Background(), msg)
	assert.NoError(t, err)

	count, _ := store.ChunkCount()
	assert.Equal(t, 1, count)

	// Verify the chunk has embedding
	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 0, pending, "chunk should have embedding when embedder is healthy")
}

func TestIndexer_indexMessage_NoChunks(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)
	embedder, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	idx := NewIndexer(store, embedder, model.RAGConfig{ChunkSize: 512})
	idx.embedderHealthy = true

	// Very short content that might not produce chunks (depends on ChunkText behavior)
	msg := service.UnindexedMessage{
		ID:          1,
		Content:     "Hi",
		Role:        "user",
		SessionID:   "sess-1",
		ProjectPath: "/test",
		Backend:     "claude",
		CreatedAt:   time.Now(),
	}

	err := idx.indexMessage(context.Background(), msg)
	assert.NoError(t, err)
}

// ---------- backfillEmbeddings with nil embedding ----------

func TestIndexer_backfillEmbeddings_NilEmbedding(t *testing.T) {
	setupIndexerDB(t)
	store := setupTestStore(t)

	// Mock server that returns nil embedding for one item
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			json.NewEncoder(w).Encode(openaiModelsResponse{Data: []openaiModelInfo{{ID: "bge-m3:latest"}}})
		case "/v1/embeddings":
			var req openaiEmbedRequest
			json.NewDecoder(r.Body).Decode(&req)
			data := make([]openaiEmbeddingData, len(req.Input))
			for i := range req.Input {
				data[i] = openaiEmbeddingData{Embedding: makeTestEmbedding(1024), Index: i}
			}
			resp := openaiEmbedResponse{Data: data}
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	embedder := NewEmbeddingClient(server.URL, "bge-m3", "")
	embedder.HTTPClient = server.Client()

	idx := NewIndexer(store, embedder, model.RAGConfig{BatchSize: 10})
	idx.embedderHealthy = true

	// Insert two chunks without embedding
	for i := 0; i < 2; i++ {
		chunk := Chunk{
			SessionID: "sess-1", MessageID: int64(i + 1), ChunkText: fmt.Sprintf("text %d", i),
			ChunkTextSegmented: fmt.Sprintf("text %d", i), ChunkIndex: 0, TokenCount: 3,
			Embedding: nil, HasEmbedding: false,
			ProjectPath: "/test", Backend: "claude", Role: "assistant",
			CreatedAt: time.Now().Truncate(time.Millisecond),
		}
		err := store.InsertChunks([]Chunk{chunk})
		require.NoError(t, err)
	}

	idx.backfillEmbeddings(context.Background())

	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 0, pending, "all embeddings should be backfilled")
}

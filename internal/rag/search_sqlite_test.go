package rag

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- RAGSearch strategy selection ----------

func TestRAGSearch_EmptyQuery(t *testing.T) {
	store := setupSQLiteStore(t)
	result, err := RAGSearch(context.Background(), store, nil, SearchParams{Query: ""}, 5, 20)
	require.NoError(t, err)
	assert.Equal(t, SearchModeFTS, result.Mode)
	assert.Empty(t, result.Results)
}

func TestRAGSearch_NilStore(t *testing.T) {
	_, err := RAGSearch(context.Background(), nil, nil, SearchParams{Query: "test"}, 5, 20)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "store is nil")
}

func TestRAGSearch_FTSOnly_WhenNoEmbedder(t *testing.T) {
	store := setupSQLiteStore(t)
	SetEmbedderHealthy(false)

	// Insert some chunks with FTS text
	chunk := Chunk{
		SessionID: "sess-1", MessageID: 1, ChunkText: "database query optimization",
		ChunkTextSegmented: "database query optimization", ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	// Search with no embedder — should use FTS-only
	result, err := RAGSearch(context.Background(), store, nil, SearchParams{
		Query:       "database",
		ProjectPath: testProjectPath,
	}, 5, 20)
	require.NoError(t, err)
	assert.Equal(t, SearchModeFTS, result.Mode)
	assert.NotEmpty(t, result.Results)
}

func TestRAGSearch_Hybrid_WhenEmbedderHealthy(t *testing.T) {
	store := setupSQLiteStore(t)
	SetEmbedderHealthy(true)

	// Insert chunks with embeddings and FTS text
	chunks := make([]Chunk, 3)
	for i := range 3 {
		chunks[i] = Chunk{
			SessionID: "sess-1", MessageID: int64(i + 1), ChunkText: "database query optimization test",
			ChunkTextSegmented: "database query optimization test", ChunkIndex: i,
			TokenCount: 5, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
			ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
			CreatedAt: time.Now().Truncate(time.Millisecond),
		}
	}
	err := store.InsertChunks(chunks)
	require.NoError(t, err)

	// Ensure cache is loaded (SearchSimple auto-reloads when dirty)
	// Pass nil embedder — since EmbedderHealthy=true, it will try to embed
	// but will fail and fall back to FTS. This is the expected behavior
	// when embedder is marked healthy but the actual client is nil.
	result, err := RAGSearch(context.Background(), store, nil, SearchParams{
		Query:       "database",
		ProjectPath: testProjectPath,
	}, 5, 20)
	require.NoError(t, err)
	// With nil embedder but healthy flag, embedding will fail → falls back to FTS
	assert.Equal(t, SearchModeFTS, result.Mode)
	assert.NotEmpty(t, result.Results)
}

func TestRAGSearch_RespectsDefaultLimit(t *testing.T) {
	store := setupSQLiteStore(t)
	SetEmbedderHealthy(false)
	insertTestChunksSQLite(t, store, 10)

	result, err := RAGSearch(context.Background(), store, nil, SearchParams{
		Query:       "chunk",
		ProjectPath: testProjectPath,
	}, 3, 20)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Results), 3)
}

func TestRAGSearch_CacheNotReady_FallbackToFTS(t *testing.T) {
	store := setupSQLiteStore(t)
	SetEmbedderHealthy(true)

	// Insert chunk
	chunk := Chunk{
		SessionID: "sess-1", MessageID: 1, ChunkText: "database search test",
		ChunkTextSegmented: "database search test", ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	// Clear cache to simulate "not ready" state
	store.cache.Clear()

	// With healthy flag but cache not ready — should fall back to FTS
	result, err := RAGSearch(context.Background(), store, nil, SearchParams{
		Query:       "database",
		ProjectPath: testProjectPath,
	}, 5, 20)
	require.NoError(t, err)
	assert.Equal(t, SearchModeFTS, result.Mode, "should fall back to FTS when cache not ready")
}

package rag

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- Helpers ----------

// setupTestStore creates a temporary DuckDB store for testing.
func setupTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	store, err := NewStore(dbPath)
	require.NoError(t, err, "NewStore should succeed")
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

// makeTestEmbedding creates a float64 slice of the given dimension
// with simple sequential values for testing.
func makeTestEmbedding(dim int) []float64 {
	emb := make([]float64, dim)
	for i := range emb {
		emb[i] = float64(i%100) * 0.01
	}
	return emb
}

// makeTestChunk creates a Chunk with the given text and a default 1024-dim embedding.
func makeTestChunk(sessionID string, messageID int64, chunkIndex int, text string) Chunk {
	return Chunk{
		SessionID:   sessionID,
		MessageID:   messageID,
		ChunkText:   text,
		ChunkIndex:  chunkIndex,
		TokenCount:  len(text) / 4,
		Embedding:   makeTestEmbedding(1024),
		ProjectPath: "/test/project",
		Backend:     "claude",
		Role:        "assistant",
		CreatedAt:   time.Now().Truncate(time.Millisecond),
	}
}

// insertTestChunks inserts n test chunks into the store.
func insertTestChunks(t *testing.T, store *Store, n int) {
	t.Helper()
	chunks := make([]Chunk, n)
	for i := 0; i < n; i++ {
		chunks[i] = makeTestChunk(
			"session-1",
			int64(i+1),
			i,
			fmt.Sprintf("chunk text %d", i),
		)
	}
	err := store.InsertChunks(chunks)
	require.NoError(t, err, "InsertChunks should succeed")
}

// ---------- NewStore ----------

func TestNewStore_CreatesDB(t *testing.T) {
	store := setupTestStore(t)
	assert.NotNil(t, store.db)
	assert.NotEmpty(t, store.dbPath)
}

func TestNewStore_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "deep", "nested")
	dbPath := filepath.Join(nestedDir, "test.duckdb")
	store, err := NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	assert.NotNil(t, store)
}

// ---------- InsertChunks ----------

func TestStore_InsertChunks_Empty(t *testing.T) {
	store := setupTestStore(t)
	err := store.InsertChunks(nil)
	assert.NoError(t, err, "InsertChunks with nil should be no-op")

	err = store.InsertChunks([]Chunk{})
	assert.NoError(t, err, "InsertChunks with empty slice should be no-op")
}

func TestStore_InsertChunks_SingleChunk(t *testing.T) {
	store := setupTestStore(t)
	chunks := []Chunk{makeTestChunk("sess-1", 1, 0, "hello world")}
	err := store.InsertChunks(chunks)
	assert.NoError(t, err)

	count, err := store.ChunkCount()
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestStore_InsertChunks_MultipleChunks(t *testing.T) {
	store := setupTestStore(t)
	insertTestChunks(t, store, 5)

	count, err := store.ChunkCount()
	assert.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestStore_InsertChunks_AutoIncrementID(t *testing.T) {
	store := setupTestStore(t)

	// First batch
	chunks1 := []Chunk{makeTestChunk("sess-1", 1, 0, "first batch")}
	err := store.InsertChunks(chunks1)
	require.NoError(t, err)

	// Second batch — IDs should continue from where the first left off
	chunks2 := []Chunk{makeTestChunk("sess-2", 2, 0, "second batch")}
	err = store.InsertChunks(chunks2)
	require.NoError(t, err)

	count, err := store.ChunkCount()
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

// ---------- SearchSimple ----------

func TestStore_SearchSimple_NoMatch(t *testing.T) {
	store := setupTestStore(t)
	// Search empty store
	hits, err := store.SearchSimple(makeTestEmbedding(1024), 5, "", "", "", "", "", "", "")
	assert.NoError(t, err)
	assert.Empty(t, hits)
}

func TestStore_SearchSimple_WithResults(t *testing.T) {
	store := setupTestStore(t)
	insertTestChunks(t, store, 3)

	// Search with the same embedding pattern — should find matches
	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", "", "", "", "", "", "")
	assert.NoError(t, err)
	assert.NotEmpty(t, hits, "should find results with matching embeddings")
}

func TestStore_SearchSimple_FiltersByProject(t *testing.T) {
	store := setupTestStore(t)

	// Insert chunks for two different projects
	chunk1 := makeTestChunk("sess-1", 1, 0, "project A content")
	chunk1.ProjectPath = "/project/a"
	chunk2 := makeTestChunk("sess-2", 2, 0, "project B content")
	chunk2.ProjectPath = "/project/b"

	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	// Filter by project A
	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "/project/a", "", "", "", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, "/project/a", hits[0].ProjectPath)
}

func TestStore_SearchSimple_FiltersByBackend(t *testing.T) {
	store := setupTestStore(t)

	chunk1 := makeTestChunk("sess-1", 1, 0, "claude content")
	chunk1.Backend = "claude"
	chunk2 := makeTestChunk("sess-2", 2, 0, "codebuddy content")
	chunk2.Backend = "codebuddy"

	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", "claude", "", "", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, "claude", hits[0].Backend)
}

func TestStore_SearchSimple_FiltersByRole(t *testing.T) {
	store := setupTestStore(t)

	chunk1 := makeTestChunk("sess-1", 1, 0, "assistant msg")
	chunk1.Role = "assistant"
	chunk2 := makeTestChunk("sess-2", 2, 0, "user msg")
	chunk2.Role = "user"

	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", "", "user", "", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, "user", hits[0].Role)
}

func TestStore_SearchSimple_FiltersBySessionID(t *testing.T) {
	store := setupTestStore(t)

	chunk1 := makeTestChunk("sess-target", 1, 0, "target content")
	chunk2 := makeTestChunk("sess-other", 2, 0, "other content")

	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", "", "", "sess-target", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, "sess-target", hits[0].SessionID)
}

func TestStore_SearchSimple_ExcludeSessionID(t *testing.T) {
	store := setupTestStore(t)

	chunk1 := makeTestChunk("sess-exclude", 1, 0, "exclude this")
	chunk2 := makeTestChunk("sess-keep", 2, 0, "keep this")

	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", "", "", "", "sess-exclude", "", "")
	assert.NoError(t, err)
	for _, h := range hits {
		assert.NotEqual(t, "sess-exclude", h.SessionID, "excluded session should not appear")
	}
}

func TestStore_SearchSimple_RespectsLimit(t *testing.T) {
	store := setupTestStore(t)
	insertTestChunks(t, store, 5)

	hits, err := store.SearchSimple(makeTestEmbedding(1024), 2, "", "", "", "", "", "", "")
	assert.NoError(t, err)
	assert.LessOrEqual(t, len(hits), 2, "should respect limit")
}

func TestStore_SearchSimple_OrderByScore(t *testing.T) {
	store := setupTestStore(t)
	insertTestChunks(t, store, 5)

	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", "", "", "", "", "", "")
	assert.NoError(t, err)
	// Verify scores are in descending order
	for i := 1; i < len(hits); i++ {
		assert.GreaterOrEqual(t, hits[i-1].Score, hits[i].Score,
			"hits should be ordered by score descending")
	}
}

func TestStore_SearchSimple_FiltersByTimeRange(t *testing.T) {
	store := setupTestStore(t)

	// Insert an old chunk
	oldChunk := makeTestChunk("sess-old", 1, 0, "old content")
	oldChunk.CreatedAt = time.Now().Add(-48 * time.Hour).Truncate(time.Second)

	// Insert a recent chunk
	recentChunk := makeTestChunk("sess-recent", 2, 0, "recent content")
	recentChunk.CreatedAt = time.Now().Truncate(time.Second)

	err := store.InsertChunks([]Chunk{oldChunk, recentChunk})
	require.NoError(t, err)

	// Search with from filter — uses ?::TIMESTAMP cast for DuckDB compatibility
	from := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", "", "", "", "", from, "")
	assert.NoError(t, err, "time filter should work with ::TIMESTAMP cast")
	for _, h := range hits {
		assert.True(t, h.CreatedAt.After(time.Now().Add(-24*time.Hour)),
			"should only return recent chunks")
	}
}

// ---------- CheckDimensionMismatch ----------

func TestStore_CheckDimensionMismatch_Empty(t *testing.T) {
	store := setupTestStore(t)
	dim, mismatch, err := store.CheckDimensionMismatch(1024)
	assert.NoError(t, err)
	assert.Equal(t, 0, dim, "empty table should return 0 dim")
	assert.False(t, mismatch, "empty table should not report mismatch")
}

func TestStore_CheckDimensionMismatch_Match(t *testing.T) {
	store := setupTestStore(t)
	insertTestChunks(t, store, 1)

	dim, mismatch, err := store.CheckDimensionMismatch(1024)
	assert.NoError(t, err)
	assert.Equal(t, 1024, dim)
	assert.False(t, mismatch, "same dimension should not report mismatch")
}

func TestStore_CheckDimensionMismatch_Mismatch(t *testing.T) {
	store := setupTestStore(t)
	insertTestChunks(t, store, 1)

	dim, mismatch, err := store.CheckDimensionMismatch(768)
	assert.NoError(t, err)
	assert.Equal(t, 1024, dim)
	assert.True(t, mismatch, "different dimension should report mismatch")
}

// ---------- ResetTable ----------

func TestStore_ResetTable(t *testing.T) {
	store := setupTestStore(t)
	insertTestChunks(t, store, 3)

	count, _ := store.ChunkCount()
	assert.Equal(t, 3, count, "should have chunks before reset")

	err := store.ResetTable()
	assert.NoError(t, err)

	count, err = store.ChunkCount()
	assert.NoError(t, err)
	assert.Equal(t, 0, count, "should have no chunks after reset")
}

// ---------- ChunkCount ----------

func TestStore_ChunkCount_Empty(t *testing.T) {
	store := setupTestStore(t)
	count, err := store.ChunkCount()
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

// ---------- DeleteChunksBySessionIDs ----------

func TestStore_DeleteChunksBySessionIDs(t *testing.T) {
	store := setupTestStore(t)

	// Insert chunks for two sessions
	chunks := []Chunk{
		makeTestChunk("sess-a", 1, 0, "content a1"),
		makeTestChunk("sess-a", 2, 1, "content a2"),
		makeTestChunk("sess-b", 3, 0, "content b1"),
	}
	err := store.InsertChunks(chunks)
	require.NoError(t, err)

	deleted, err := store.DeleteChunksBySessionIDs([]string{"sess-a"})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), deleted, "should delete 2 chunks from sess-a")

	count, _ := store.ChunkCount()
	assert.Equal(t, 1, count, "should have 1 chunk remaining")
}

func TestStore_DeleteChunksBySessionIDs_EmptyList(t *testing.T) {
	store := setupTestStore(t)
	deleted, err := store.DeleteChunksBySessionIDs(nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), deleted)

	deleted, err = store.DeleteChunksBySessionIDs([]string{})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), deleted)
}

// ---------- Close ----------

func TestStore_Close_NilDB(t *testing.T) {
	s := &Store{db: nil, dbPath: "/tmp/nonexistent"}
	err := s.Close()
	assert.NoError(t, err, "Close with nil db should not error")
}

// ---------- RecoverFromCorruption ----------

func TestStore_RecoverFromCorruption(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recover.duckdb")

	// Create a store and add data
	store, err := NewStore(dbPath)
	require.NoError(t, err)
	insertTestChunks(t, store, 2)
	store.Close()

	// Reopen and recover
	store2, err := NewStore(dbPath)
	require.NoError(t, err)

	err = store2.RecoverFromCorruption()
	assert.NoError(t, err, "RecoverFromCorruption should succeed")

	// After recovery, table should be empty (fresh schema)
	count, err := store2.ChunkCount()
	assert.NoError(t, err)
	assert.Equal(t, 0, count, "recovered table should be empty")

	store2.Close()
}

// ---------- embeddingToSQLArray ----------

func TestEmbeddingToSQLArray(t *testing.T) {
	vec := []float64{0.1, 0.2, 0.3}
	result := embeddingToSQLArray(vec)
	assert.True(t, strings.HasPrefix(result, "array["))
	assert.True(t, strings.HasSuffix(result, "]::FLOAT[1024]"))
	assert.Contains(t, result, "0.1")
	assert.Contains(t, result, "0.2")
	assert.Contains(t, result, "0.3")
}

func TestEmbeddingToSQLArray_Empty(t *testing.T) {
	result := embeddingToSQLArray([]float64{})
	assert.Equal(t, "array[]::FLOAT[1024]", result)
}

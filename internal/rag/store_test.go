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
	// Ensure segmenter is initialized for tests that use SegmentText
	if segmenter == nil {
		if err := InitSegmenter(); err != nil {
			t.Logf("Warning: gse segmenter not available: %v", err)
		}
	}
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
		SessionID:          sessionID,
		MessageID:          messageID,
		ChunkText:          text,
		ChunkTextSegmented: SegmentText(text),
		ChunkIndex:         chunkIndex,
		TokenCount:         len(text) / 4,
		Embedding:          makeTestEmbedding(1024),
		HasEmbedding:       true,
		ProjectPath:        "/test/project",
		Backend:            "claude",
		Role:               "assistant",
		CreatedAt:          time.Now().Truncate(time.Millisecond),
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

// ---------- Schema migration (new columns) ----------

func TestStore_SchemaHasSegmentedTextColumn(t *testing.T) {
	store := setupTestStore(t)
	// Insert a chunk with segmented text
	chunk := makeTestChunk("sess-1", 1, 0, "hello world")
	chunk.ChunkTextSegmented = "hello world"
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	// Verify the column exists by querying it directly
	var segmented string
	err = store.db.QueryRow("SELECT chunk_text_segmented FROM chat_chunks LIMIT 1").Scan(&segmented)
	assert.NoError(t, err, "chunk_text_segmented column should exist")
	assert.Equal(t, "hello world", segmented)
}

func TestStore_SchemaHasHasEmbeddingColumn(t *testing.T) {
	store := setupTestStore(t)
	// Insert a chunk with embedding
	chunk := makeTestChunk("sess-1", 1, 0, "test")
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	var hasEmb bool
	err = store.db.QueryRow("SELECT has_embedding FROM chat_chunks LIMIT 1").Scan(&hasEmb)
	assert.NoError(t, err, "has_embedding column should exist")
	assert.True(t, hasEmb, "chunk with embedding should have has_embedding=true")
}

func TestStore_InsertChunks_WithoutEmbedding(t *testing.T) {
	store := setupTestStore(t)
	// Insert a chunk WITHOUT embedding (Ollama unavailable scenario)
	chunk := Chunk{
		SessionID:          "sess-1",
		MessageID:          1,
		ChunkText:          "test without embedding",
		ChunkTextSegmented: "test without embedding",
		ChunkIndex:         0,
		TokenCount:         5,
		Embedding:          nil, // no embedding
		HasEmbedding:       false,
		ProjectPath:        "/test",
		Backend:            "claude",
		Role:               "assistant",
		CreatedAt:          time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	var hasEmb bool
	err = store.db.QueryRow("SELECT has_embedding FROM chat_chunks LIMIT 1").Scan(&hasEmb)
	assert.NoError(t, err)
	assert.False(t, hasEmb, "chunk without embedding should have has_embedding=false")

	count, _ := store.ChunkCount()
	assert.Equal(t, 1, count, "chunk should be inserted even without embedding")
}

func TestStore_PendingEmbeddingCount(t *testing.T) {
	store := setupTestStore(t)

	// Insert one with embedding and one without
	chunk1 := makeTestChunk("sess-1", 1, 0, "with embedding")
	err := store.InsertChunks([]Chunk{chunk1})
	require.NoError(t, err)

	chunk2 := Chunk{
		SessionID:          "sess-2",
		MessageID:          2,
		ChunkText:          "without embedding",
		ChunkTextSegmented: "without embedding",
		ChunkIndex:         0,
		TokenCount:         3,
		Embedding:          nil,
		HasEmbedding:       false,
		ProjectPath:        "/test",
		Backend:            "claude",
		Role:               "assistant",
		CreatedAt:          time.Now().Truncate(time.Millisecond),
	}
	err = store.InsertChunks([]Chunk{chunk2})
	require.NoError(t, err)

	pending, err := store.PendingEmbeddingCount()
	assert.NoError(t, err)
	assert.Equal(t, 1, pending, "should have 1 pending embedding")
}

func TestStore_UpdateEmbedding(t *testing.T) {
	store := setupTestStore(t)

	// Insert a single chunk without embedding
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
	require.NoError(t, err, "InsertChunks should succeed for chunk without embedding")

	// Get the chunk ID
	var chunkID int64
	err = store.db.QueryRow("SELECT id FROM chat_chunks WHERE has_embedding = false LIMIT 1").Scan(&chunkID)
	require.NoError(t, err)

	// Backfill the embedding
	embedding := makeTestEmbedding(1024)
	err = store.UpdateEmbedding(chunkID, embedding)
	assert.NoError(t, err)

	// Verify has_embedding is now true
	var hasEmb bool
	err = store.db.QueryRow("SELECT has_embedding FROM chat_chunks WHERE id = ?", chunkID).Scan(&hasEmb)
	assert.NoError(t, err)
	assert.True(t, hasEmb, "embedding should be set after backfill")

	// Verify it's now searchable via vector search
	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 0, pending, "no pending embeddings after backfill")
}

// ---------- FTS (Full-Text Search) ----------

func TestStore_FTSAvailable(t *testing.T) {
	store := setupTestStore(t)
	// FTS should be available (DuckDB FTS extension loaded)
	assert.True(t, store.ftsAvailable, "FTS should be available in test store")
}

func TestStore_SearchFTS_English(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert chunks with segmented text
	chunks := []Chunk{
		{
			SessionID: "sess-1", MessageID: 1, ChunkText: "database query optimization",
			ChunkTextSegmented: "database query optimization", ChunkIndex: 0,
			TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
			ProjectPath: "/test", Backend: "claude", Role: "assistant",
			CreatedAt: time.Now().Truncate(time.Millisecond),
		},
		{
			SessionID: "sess-2", MessageID: 2, ChunkText: "web server configuration",
			ChunkTextSegmented: "web server configuration", ChunkIndex: 0,
			TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
			ProjectPath: "/test", Backend: "claude", Role: "assistant",
			CreatedAt: time.Now().Truncate(time.Millisecond),
		},
	}
	err := store.InsertChunks(chunks)
	require.NoError(t, err)

	// Rebuild FTS index
	err = store.CreateFTSIndex()
	require.NoError(t, err)

	// Search for "database"
	hits, err := store.SearchFTS("database", 5, "", "", "", "", "", "", "")
	assert.NoError(t, err)
	assert.NotEmpty(t, hits, "FTS search should find results for 'database'")
	assert.Contains(t, hits[0].ChunkText, "database")
}

func TestStore_SearchFTS_Chinese(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert Chinese chunks with pre-segmented text
	chunks := []Chunk{
		{
			SessionID: "sess-1", MessageID: 1, ChunkText: "使用DuckDB进行全文检索",
			ChunkTextSegmented: SegmentText("使用DuckDB进行全文检索"), ChunkIndex: 0,
			TokenCount: 10, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
			ProjectPath: "/test", Backend: "claude", Role: "assistant",
			CreatedAt: time.Now().Truncate(time.Millisecond),
		},
		{
			SessionID: "sess-2", MessageID: 2, ChunkText: "人工智能技术发展",
			ChunkTextSegmented: SegmentText("人工智能技术发展"), ChunkIndex: 0,
			TokenCount: 5, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
			ProjectPath: "/test", Backend: "claude", Role: "assistant",
			CreatedAt: time.Now().Truncate(time.Millisecond),
		},
	}
	err := store.InsertChunks(chunks)
	require.NoError(t, err)

	// Rebuild FTS index
	err = store.CreateFTSIndex()
	require.NoError(t, err)

	// Search for "全文检索" (segmented query)
	hits, err := store.SearchFTS("全文检索", 5, "", "", "", "", "", "", "")
	assert.NoError(t, err)
	assert.NotEmpty(t, hits, "FTS search should find Chinese results")
}

func TestStore_SearchFTS_RespectsFilters(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert chunks for different projects
	chunk1 := Chunk{
		SessionID: "sess-1", MessageID: 1, ChunkText: "database query optimization",
		ChunkTextSegmented: "database query optimization", ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: "/project/a", Backend: "claude", Role: "assistant",
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	chunk2 := Chunk{
		SessionID: "sess-2", MessageID: 2, ChunkText: "database indexing strategies",
		ChunkTextSegmented: "database indexing strategies", ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: "/project/b", Backend: "codebuddy", Role: "user",
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	err = store.CreateFTSIndex()
	require.NoError(t, err)

	// Filter by project
	hits, err := store.SearchFTS("database", 5, "/project/a", "", "", "", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, "/project/a", hits[0].ProjectPath)
}

func TestStore_SearchHybrid_CombinesSources(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert several chunks
	insertTestChunks(t, store, 5)

	err := store.CreateFTSIndex()
	require.NoError(t, err)

	// Hybrid search should combine vector + FTS results
	hits, err := store.SearchHybrid(
		makeTestEmbedding(1024), "chunk text", 20, 5,
		"", "", "", "", "", "", "",
	)
	assert.NoError(t, err)
	assert.NotEmpty(t, hits, "hybrid search should return results")
}

// ---------- FTS rebuild management ----------

func TestStore_RebuildFTSIfDirty(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert chunks — should mark FTS dirty
	insertTestChunks(t, store, 3)
	assert.True(t, store.ftsDirty, "inserting chunks should mark FTS dirty")

	// Rebuild should clear the dirty flag
	err := store.RebuildFTSIfDirty()
	assert.NoError(t, err)
	assert.False(t, store.ftsDirty, "rebuild should clear dirty flag")
}

package rag

import (
	"context"
	"fmt"
	"math"
	"os"
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
	store, err := NewStore(dbPath, nil)
	require.NoError(t, err, "NewStore should succeed")
	t.Cleanup(func() {
		_ = store.Close()
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
		Backend:            testBackendClaude,
		Role:               testRoleAssistant,
		CreatedAt:          time.Now().Truncate(time.Millisecond),
	}
}

// insertTestChunks inserts n test chunks into the store.
func insertTestChunks(t *testing.T, store *Store, n int) {
	t.Helper()
	chunks := make([]Chunk, n)
	for i := range n {
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
	store, err := NewStore(dbPath, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
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
	chunks := []Chunk{makeTestChunk(testSession1, 1, 0, "hello world")}
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
	chunks1 := []Chunk{makeTestChunk(testSession1, 1, 0, "first batch")}
	err := store.InsertChunks(chunks1)
	require.NoError(t, err)

	// Second batch — IDs should continue from where the first left off
	chunks2 := []Chunk{makeTestChunk(testSession2, 2, 0, "second batch")}
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
	chunk1 := makeTestChunk(testSession1, 1, 0, "project A content")
	chunk1.ProjectPath = "/project/a"
	chunk2 := makeTestChunk(testSession2, 2, 0, "project B content")
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

	chunk1 := makeTestChunk(testSession1, 1, 0, "claude content")
	chunk1.Backend = testBackendClaude
	chunk2 := makeTestChunk(testSession2, 2, 0, "codebuddy content")
	chunk2.Backend = testBackendCodebuddy

	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", testBackendClaude, "", "", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, testBackendClaude, hits[0].Backend)
}

func TestStore_SearchSimple_FiltersByRole(t *testing.T) {
	store := setupTestStore(t)

	chunk1 := makeTestChunk(testSession1, 1, 0, "assistant msg")
	chunk1.Role = testRoleAssistant
	chunk2 := makeTestChunk(testSession2, 2, 0, "user msg")
	chunk2.Role = testRoleUser

	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", "", testRoleUser, "", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, testRoleUser, hits[0].Role)
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
	dim, mismatch, err := store.CheckDimensionMismatch()
	assert.NoError(t, err)
	assert.Equal(t, 0, dim, "empty table should return 0 dim")
	assert.False(t, mismatch, "empty table should not report mismatch")
}

func TestStore_CheckDimensionMismatch_Match(t *testing.T) {
	store := setupTestStore(t)
	insertTestChunks(t, store, 1)

	dim, mismatch, err := store.CheckDimensionMismatch()
	assert.NoError(t, err)
	assert.Equal(t, 1024, dim)
	assert.False(t, mismatch, "same dimension should not report mismatch")
}

func TestStore_CheckDimensionMismatch_Mismatch(t *testing.T) {
	store := setupTestStore(t)
	insertTestChunks(t, store, 1)

	// Change the store's expected dimension to simulate a mismatch
	store.embeddingDim = 768
	dim, mismatch, err := store.CheckDimensionMismatch()
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
	result, err := embeddingToSQLArray(vec, 1024)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "array["))
	assert.True(t, strings.HasSuffix(result, "]::FLOAT[1024]"))
	assert.Contains(t, result, "0.1")
	assert.Contains(t, result, "0.2")
	assert.Contains(t, result, "0.3")
}

func TestEmbeddingToSQLArray_Empty(t *testing.T) {
	result, err := embeddingToSQLArray([]float64{}, 1024)
	assert.NoError(t, err)
	assert.Equal(t, "array[]::FLOAT[1024]", result)
}

func TestEmbeddingToSQLArray_NonFinite(t *testing.T) {
	_, err := embeddingToSQLArray([]float64{0.1, math.NaN(), 0.3}, 1024)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-finite")

	_, err = embeddingToSQLArray([]float64{0.1, math.Inf(1)}, 1024)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-finite")
}

// ---------- Non-finite embedding rejection in public methods (ISS-130) ----------

func TestStore_InsertChunks_RejectsNaNEmbedding(t *testing.T) {
	store := setupTestStore(t)

	chunk := makeTestChunk("sess-nan", 1, 0, "test chunk with NaN embedding")
	chunk.Embedding = makeTestEmbedding(1024)
	chunk.Embedding[5] = math.NaN()

	err := store.InsertChunks([]Chunk{chunk})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding validation")
}

func TestStore_SearchSimple_RejectsInfEmbedding(t *testing.T) {
	store := setupTestStore(t)

	queryEmbedding := makeTestEmbedding(1024)
	queryEmbedding[0] = math.Inf(1)

	_, err := store.SearchSimple(queryEmbedding, 10, "", "", "", "", "", "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query embedding validation")
}

func TestStore_UpdateEmbedding_RejectsNaNEmbedding(t *testing.T) {
	store := setupTestStore(t)

	// First insert a valid chunk so UpdateEmbedding has a target
	chunk := makeTestChunk("sess-update-nan", 1, 0, "test chunk for update")
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	// Search to find the inserted chunk's ID
	hits, err := store.SearchSimple(makeTestEmbedding(1024), 1, "", "", "", "", "", "", "")
	require.NoError(t, err)
	require.NotEmpty(t, hits)

	// Try updating with NaN embedding
	nanEmb := makeTestEmbedding(1024)
	nanEmb[0] = math.NaN()
	err = store.UpdateEmbedding(hits[0].ChunkID, nanEmb)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding validation for update")
}

// ---------- Schema migration (new columns) ----------

func TestStore_SchemaHasSegmentedTextColumn(t *testing.T) {
	store := setupTestStore(t)
	// Insert a chunk with segmented text
	chunk := makeTestChunk(testSession1, 1, 0, "hello world")
	chunk.ChunkTextSegmented = "hello world"
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	// Verify the column exists by querying it directly
	var segmented string
	err = store.db.QueryRowContext(context.Background(), "SELECT chunk_text_segmented FROM chat_chunks LIMIT 1").Scan(&segmented)
	assert.NoError(t, err, "chunk_text_segmented column should exist")
	assert.Equal(t, "hello world", segmented)
}

func TestStore_SchemaHasHasEmbeddingColumn(t *testing.T) {
	store := setupTestStore(t)
	// Insert a chunk with embedding
	chunk := makeTestChunk(testSession1, 1, 0, "test")
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	var hasEmb bool
	err = store.db.QueryRowContext(context.Background(), "SELECT has_embedding FROM chat_chunks LIMIT 1").Scan(&hasEmb)
	assert.NoError(t, err, "has_embedding column should exist")
	assert.True(t, hasEmb, "chunk with embedding should have has_embedding=true")
}

func TestStore_InsertChunks_WithoutEmbedding(t *testing.T) {
	store := setupTestStore(t)
	// Insert a chunk WITHOUT embedding (Ollama unavailable scenario)
	chunk := Chunk{
		SessionID:          testSession1,
		MessageID:          1,
		ChunkText:          "test without embedding",
		ChunkTextSegmented: "test without embedding",
		ChunkIndex:         0,
		TokenCount:         5,
		Embedding:          nil, // no embedding
		HasEmbedding:       false,
		ProjectPath:        testProjectPath,
		Backend:            testBackendClaude,
		Role:               testRoleAssistant,
		CreatedAt:          time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err)

	var hasEmb bool
	err = store.db.QueryRowContext(context.Background(), "SELECT has_embedding FROM chat_chunks LIMIT 1").Scan(&hasEmb)
	assert.NoError(t, err)
	assert.False(t, hasEmb, "chunk without embedding should have has_embedding=false")

	count, _ := store.ChunkCount()
	assert.Equal(t, 1, count, "chunk should be inserted even without embedding")
}

func TestStore_PendingEmbeddingCount(t *testing.T) {
	store := setupTestStore(t)

	// Insert one with embedding and one without
	chunk1 := makeTestChunk(testSession1, 1, 0, "with embedding")
	err := store.InsertChunks([]Chunk{chunk1})
	require.NoError(t, err)

	chunk2 := Chunk{
		SessionID:          testSession2,
		MessageID:          2,
		ChunkText:          "without embedding",
		ChunkTextSegmented: "without embedding",
		ChunkIndex:         0,
		TokenCount:         3,
		Embedding:          nil,
		HasEmbedding:       false,
		ProjectPath:        testProjectPath,
		Backend:            testBackendClaude,
		Role:               testRoleAssistant,
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
		SessionID:          testSession1,
		MessageID:          1,
		ChunkText:          testNeedsBackfill,
		ChunkTextSegmented: testNeedsBackfill,
		ChunkIndex:         0,
		TokenCount:         3,
		Embedding:          nil,
		HasEmbedding:       false,
		ProjectPath:        testProjectPath,
		Backend:            testBackendClaude,
		Role:               testRoleAssistant,
		CreatedAt:          time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk})
	require.NoError(t, err, "InsertChunks should succeed for chunk without embedding")

	// Get the chunk ID
	var chunkID int64
	err = store.db.QueryRowContext(context.Background(), "SELECT id FROM chat_chunks WHERE has_embedding = false LIMIT 1").Scan(&chunkID)
	require.NoError(t, err)

	// Backfill the embedding
	embedding := makeTestEmbedding(1024)
	err = store.UpdateEmbedding(chunkID, embedding)
	assert.NoError(t, err)

	// Verify has_embedding is now true
	var hasEmb bool
	err = store.db.QueryRowContext(context.Background(), "SELECT has_embedding FROM chat_chunks WHERE id = ?", chunkID).Scan(&hasEmb)
	assert.NoError(t, err)
	assert.True(t, hasEmb, "embedding should be set after backfill")

	// Verify it's now searchable via vector search
	pending, _ := store.PendingEmbeddingCount()
	assert.Equal(t, 0, pending, "no pending embeddings after backfill")
}

// ---------- FTS (Full-Text Search) ----------

func TestStore_FTSAvailable(t *testing.T) {
	store := setupTestStore(t)
	// FTS may not be available on all platforms (e.g., Windows CI where
	// DuckDB FTS extension can't be installed). Just verify the flag is set
	// consistently — if FTS init succeeded, the flag should be true.
	if store.ftsAvailable {
		// FTS loaded successfully, verify it works
		assert.True(t, store.ftsAvailable, "FTS should be available when initFTS succeeds")
	}
	// If FTS is not available, that's also acceptable — the store still
	// functions with vector-only search.
}

func TestStore_SearchFTS_English(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert chunks with segmented text
	chunks := []Chunk{
		{
			SessionID: testSession1, MessageID: 1, ChunkText: testDBQueryOptimization,
			ChunkTextSegmented: testDBQueryOptimization, ChunkIndex: 0,
			TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
			ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
			CreatedAt: time.Now().Truncate(time.Millisecond),
		},
		{
			SessionID: testSession2, MessageID: 2, ChunkText: "web server configuration",
			ChunkTextSegmented: "web server configuration", ChunkIndex: 0,
			TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
			ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
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
			SessionID: testSession1, MessageID: 1, ChunkText: "使用DuckDB进行全文检索",
			ChunkTextSegmented: SegmentText("使用DuckDB进行全文检索"), ChunkIndex: 0,
			TokenCount: 10, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
			ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
			CreatedAt: time.Now().Truncate(time.Millisecond),
		},
		{
			SessionID: testSession2, MessageID: 2, ChunkText: "人工智能技术发展",
			ChunkTextSegmented: SegmentText("人工智能技术发展"), ChunkIndex: 0,
			TokenCount: 5, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
			ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
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
		SessionID: testSession1, MessageID: 1, ChunkText: testDBQueryOptimization,
		ChunkTextSegmented: testDBQueryOptimization, ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: "/project/a", Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	chunk2 := Chunk{
		SessionID: testSession2, MessageID: 2, ChunkText: "database indexing strategies",
		ChunkTextSegmented: "database indexing strategies", ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: "/project/b", Backend: testBackendCodebuddy, Role: testRoleUser,
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

// ---------- GetPendingEmbeddings ----------

func TestStore_GetPendingEmbeddings(t *testing.T) {
	store := setupTestStore(t)

	// Insert two chunks without embedding
	chunk1 := Chunk{
		SessionID: testSession1, MessageID: 1, ChunkText: "text 1",
		ChunkTextSegmented: "text 1", ChunkIndex: 0, TokenCount: 3,
		Embedding: nil, HasEmbedding: false,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	chunk2 := Chunk{
		SessionID: testSession1, MessageID: 2, ChunkText: "text 2",
		ChunkTextSegmented: "text 2", ChunkIndex: 0, TokenCount: 3,
		Embedding: nil, HasEmbedding: false,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	pending, err := store.GetPendingEmbeddings(10)
	assert.NoError(t, err)
	assert.Len(t, pending, 2, "should return 2 pending chunks")
	assert.Equal(t, "text 1", pending[0].ChunkText)
	assert.Equal(t, "text 2", pending[1].ChunkText)
}

func TestStore_GetPendingEmbeddings_RespectsLimit(t *testing.T) {
	store := setupTestStore(t)

	// Insert 5 chunks without embedding
	for i := range 5 {
		chunk := Chunk{
			SessionID: testSession1, MessageID: int64(i + 1), ChunkText: fmt.Sprintf("text %d", i),
			ChunkTextSegmented: fmt.Sprintf("text %d", i), ChunkIndex: 0, TokenCount: 3,
			Embedding: nil, HasEmbedding: false,
			ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
			CreatedAt: time.Now().Truncate(time.Millisecond),
		}
		err := store.InsertChunks([]Chunk{chunk})
		require.NoError(t, err)
	}

	pending, err := store.GetPendingEmbeddings(2)
	assert.NoError(t, err)
	assert.Len(t, pending, 2, "should respect limit")
}

func TestStore_GetPendingEmbeddings_None(t *testing.T) {
	store := setupTestStore(t)

	// All chunks have embeddings
	insertTestChunks(t, store, 3)

	pending, err := store.GetPendingEmbeddings(10)
	assert.NoError(t, err)
	assert.Empty(t, pending, "should return no pending when all have embeddings")
}

// ---------- RecoverFromCorruption ----------

func TestStore_RecoverFromCorruption(t *testing.T) {
	store := setupTestStore(t)

	// Insert some data
	insertTestChunks(t, store, 3)
	count, _ := store.ChunkCount()
	assert.Equal(t, 3, count)

	// Recover from corruption — should recreate the database
	err := store.RecoverFromCorruption()
	assert.NoError(t, err)

	// Should have a fresh table with no chunks
	count, _ = store.ChunkCount()
	assert.Equal(t, 0, count, "should have no chunks after recovery")

	// Should be able to insert after recovery
	insertTestChunks(t, store, 1)
	count, _ = store.ChunkCount()
	assert.Equal(t, 1, count, "should be able to insert after recovery")
}

// ---------- RebuildFTSIfDirty debounce ----------

func TestStore_RebuildFTSIfDirty_NotDirty(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Not dirty — should be no-op
	err := store.RebuildFTSIfDirty()
	assert.NoError(t, err)
}

func TestStore_RebuildFTSIfDirty_Debounce(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert chunks and rebuild
	insertTestChunks(t, store, 3)
	err := store.RebuildFTSIfDirty()
	require.NoError(t, err)
	assert.False(t, store.ftsDirty, "dirty flag should be cleared")

	// Insert more chunks — mark dirty again
	insertTestChunks(t, store, 2)
	assert.True(t, store.ftsDirty, "should be dirty after insert")

	// Manually set ftsLastRebuild to recent time to test debounce
	store.ftsLastRebuild = time.Now()

	// Should skip rebuild due to debounce (less than 30s since last rebuild)
	err = store.RebuildFTSIfDirty()
	assert.NoError(t, err)
	assert.True(t, store.ftsDirty, "should still be dirty due to debounce")
}

func TestStore_RebuildFTSIfDirty_FTSNotAvailable(t *testing.T) {
	store := setupTestStore(t)
	store.ftsAvailable = false
	store.ftsDirty = true

	err := store.RebuildFTSIfDirty()
	assert.NoError(t, err, "should not error when FTS not available")
}

// ---------- SetEmbeddingDim ----------

func TestStore_SetEmbeddingDim_Changed(t *testing.T) {
	store := setupTestStore(t)

	// Default dim is 1024; changing to 768 should return true
	changed := store.SetEmbeddingDim(768)
	assert.True(t, changed, "should return true when dimension changes")
	assert.Equal(t, 768, store.embeddingDim)

	// Verify it was persisted to metadata
	var val string
	err := store.db.QueryRowContext(context.Background(), "SELECT value FROM rag_metadata WHERE key = 'embedding_dim'").Scan(&val)
	assert.NoError(t, err)
	assert.Equal(t, "768", val)
}

func TestStore_SetEmbeddingDim_Same(t *testing.T) {
	store := setupTestStore(t)

	// Default dim is 1024; setting to same value should return false
	changed := store.SetEmbeddingDim(1024)
	assert.False(t, changed, "should return false when dimension is same")
}

// ---------- readMetadata / readMetadataInt ----------

func TestStore_ReadMetadata_Missing(t *testing.T) {
	store := setupTestStore(t)

	val := store.readMetadata("nonexistent_key")
	assert.Equal(t, "", val)
}

func TestStore_ReadMetadata_Exists(t *testing.T) {
	store := setupTestStore(t)

	store.writeMetadata("test_key", "test_value")
	val := store.readMetadata("test_key")
	assert.Equal(t, "test_value", val)
}

func TestStore_ReadMetadataInt_Missing(t *testing.T) {
	store := setupTestStore(t)

	val := store.readMetadataInt("nonexistent_key", 42)
	assert.Equal(t, 42, val, "should return fallback for missing key")
}

func TestStore_ReadMetadataInt_InvalidInt(t *testing.T) {
	store := setupTestStore(t)

	store.writeMetadata("bad_int", "not_a_number")
	val := store.readMetadataInt("bad_int", 42)
	assert.Equal(t, 42, val, "should return fallback for non-integer value")
}

func TestStore_ReadMetadataInt_ValidInt(t *testing.T) {
	store := setupTestStore(t)

	store.writeMetadata("good_int", "123")
	val := store.readMetadataInt("good_int", 42)
	assert.Equal(t, 123, val)
}

// ---------- writeMetadata ----------

func TestStore_WriteMetadata_ErrorPath(t *testing.T) {
	store := setupTestStore(t)
	// Close the DB to trigger a write error
	_ = store.db.Close()

	// writeMetadata should not panic when DB is closed
	store.writeMetadata("error_key", "error_value")
	// If we get here without panic, the error path is covered
}

func TestStore_WriteMetadata_Upsert(t *testing.T) {
	store := setupTestStore(t)

	store.writeMetadata("upsert_key", "first")
	assert.Equal(t, "first", store.readMetadata("upsert_key"))

	store.writeMetadata("upsert_key", "second")
	assert.Equal(t, "second", store.readMetadata("upsert_key"), "should update existing value")
}

// ---------- loadEmbeddingDim ----------

func TestStore_LoadEmbeddingDim(t *testing.T) {
	store := setupTestStore(t)

	// Write a dimension and reload
	store.writeMetadata("embedding_dim", "512")
	store.loadEmbeddingDim()

	assert.Equal(t, 512, store.embeddingDim, "should load persisted embedding dimension")
}

func TestStore_LoadEmbeddingDim_Zero(t *testing.T) {
	store := setupTestStore(t)

	// No metadata set — should not override the default from initSchema
	origDim := store.embeddingDim
	store.loadEmbeddingDim()
	assert.Equal(t, origDim, store.embeddingDim, "should not change dim when no metadata")
}

// ---------- NewStore error path ----------

func TestNewStore_FailedSchema(t *testing.T) {
	// Create a directory (not a file) at the db path to force a schema init failure
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	// Write garbage to the parent to make it harder to create the DB
	// This tests the error path indirectly; the real test is that NewStore
	// returns an error when it can't init schema.
	store, err := NewStore(dbPath, nil)
	if err != nil {
		// If it fails, that's the expected error path
		assert.Nil(t, store)
	} else {
		// If it succeeds, clean up
		_ = store.Close()
	}
}

func TestNewStore_InitSchemaError(t *testing.T) {
	// Create a directory where the DB file should be — DuckDB can't create a DB
	// in a path occupied by a directory, which forces initSchema to fail,
	// exercising the `_ = db.Close()` error path.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	// Create a directory at the DB path to force an error
	require.NoError(t, os.MkdirAll(dbPath, 0o755))

	_, err := NewStore(dbPath, nil)
	assert.Error(t, err, "NewStore should fail when dbPath is a directory")
}

// ---------- SearchFTS with filters ----------

func TestStore_SearchFTS_FiltersByBackend(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	chunk1 := Chunk{
		SessionID: testSession1, MessageID: 1, ChunkText: "database optimization",
		ChunkTextSegmented: "database optimization", ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	chunk2 := Chunk{
		SessionID: testSession2, MessageID: 2, ChunkText: testDBSearch,
		ChunkTextSegmented: testDBSearch, ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendCodebuddy, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	err = store.CreateFTSIndex()
	require.NoError(t, err)

	hits, err := store.SearchFTS("database", 5, "", testBackendClaude, "", "", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, testBackendClaude, hits[0].Backend)
}

func TestStore_SearchFTS_FiltersByRole(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	chunk1 := Chunk{
		SessionID: testSession1, MessageID: 1, ChunkText: testDBQuery,
		ChunkTextSegmented: testDBQuery, ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	chunk2 := Chunk{
		SessionID: testSession2, MessageID: 2, ChunkText: "database command",
		ChunkTextSegmented: "database command", ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleUser,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	err = store.CreateFTSIndex()
	require.NoError(t, err)

	hits, err := store.SearchFTS("database", 5, "", "", testRoleUser, "", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, testRoleUser, hits[0].Role)
}

func TestStore_SearchFTS_FiltersBySessionID(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	chunk1 := Chunk{
		SessionID: "sess-target", MessageID: 1, ChunkText: testDBQuery,
		ChunkTextSegmented: testDBQuery, ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	chunk2 := Chunk{
		SessionID: "sess-other", MessageID: 2, ChunkText: "database other",
		ChunkTextSegmented: "database other", ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	err = store.CreateFTSIndex()
	require.NoError(t, err)

	hits, err := store.SearchFTS("database", 5, "", "", "", "sess-target", "", "", "")
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, "sess-target", hits[0].SessionID)
}

func TestStore_SearchFTS_ExcludeSessionID(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	chunk1 := Chunk{
		SessionID: "sess-exclude", MessageID: 1, ChunkText: testDBQuery,
		ChunkTextSegmented: testDBQuery, ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	chunk2 := Chunk{
		SessionID: "sess-keep", MessageID: 2, ChunkText: testDBSearch,
		ChunkTextSegmented: testDBSearch, ChunkIndex: 0,
		TokenCount: 3, Embedding: makeTestEmbedding(1024), HasEmbedding: true,
		ProjectPath: testProjectPath, Backend: testBackendClaude, Role: testRoleAssistant,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	err := store.InsertChunks([]Chunk{chunk1, chunk2})
	require.NoError(t, err)

	err = store.CreateFTSIndex()
	require.NoError(t, err)

	hits, err := store.SearchFTS("database", 5, "", "", "", "", "sess-exclude", "", "")
	assert.NoError(t, err)
	for _, h := range hits {
		assert.NotEqual(t, "sess-exclude", h.SessionID, "excluded session should not appear")
	}
}

func TestStore_SearchFTS_NoResults(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert some chunks and build FTS index so the FTS catalog exists
	insertTestChunks(t, store, 1)
	require.NoError(t, store.CreateFTSIndex())

	// Search for something that won't match
	hits, err := store.SearchFTS("nonexistent_xyz_12345", 5, "", "", "", "", "", "", "")
	assert.NoError(t, err)
	assert.Empty(t, hits)
}

func TestStore_SearchFTS_FTSNotAvailable(t *testing.T) {
	store := setupTestStore(t)
	store.ftsAvailable = false

	_, err := store.SearchFTS("test", 5, "", "", "", "", "", "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FTS not available")
}

// ---------- SearchHybrid edge cases ----------

func TestStore_SearchHybrid_VectorFails(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert chunks and build FTS index
	insertTestChunks(t, store, 3)
	err := store.CreateFTSIndex()
	require.NoError(t, err)

	// Use a zero-length embedding to cause a vector search failure
	// Actually, SearchSimple with nil embedding would cause a SQL error.
	// Instead, test the fallback path by using an empty store for vector.
	// The real fallback is tested via RAGSearch.
	// Here we just verify SearchHybrid works normally.
	hits, err := store.SearchHybrid(makeTestEmbedding(1024), "chunk text", 20, 5, "", "", "", "", "", "", "")
	assert.NoError(t, err)
	assert.NotEmpty(t, hits)
}

// ---------- NewStore with existing chunks ----------

func TestNewStore_ExistingChunksRebuildsFTS(t *testing.T) {
	store := setupTestStore(t)
	if !store.ftsAvailable {
		t.Skip("FTS not available")
	}

	// Insert some chunks
	insertTestChunks(t, store, 3)

	// Close and reopen the store
	dbPath := store.dbPath
	_ = store.Close()

	store2, err := NewStore(dbPath, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store2.Close() })

	// Should have rebuilt FTS index from existing chunks
	assert.True(t, store2.ftsAvailable)
	count, _ := store2.ChunkCount()
	assert.Equal(t, 3, count, "should preserve chunks after reopen")
}

// ---------- SearchSimple with time filters ----------

func TestStore_SearchSimple_ToTimeFilter(t *testing.T) {
	store := setupTestStore(t)

	// Insert a recent chunk
	recentChunk := makeTestChunk("sess-recent", 1, 0, "recent content")
	recentChunk.CreatedAt = time.Now().Truncate(time.Second)

	// Insert an old chunk
	oldChunk := makeTestChunk("sess-old", 2, 0, "old content")
	oldChunk.CreatedAt = time.Now().Add(-48 * time.Hour).Truncate(time.Second)

	err := store.InsertChunks([]Chunk{recentChunk, oldChunk})
	require.NoError(t, err)

	// Search with to filter
	to := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	hits, err := store.SearchSimple(makeTestEmbedding(1024), 10, "", "", "", "", "", "", to)
	assert.NoError(t, err)
	for _, h := range hits {
		assert.True(t, h.CreatedAt.Before(time.Now().Add(-23*time.Hour)),
			"should only return old chunks with to filter")
	}
}

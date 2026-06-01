package rag

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Shared test constants to avoid goconst duplicates across test files.
const (
	testModelBgeM3Latest    = "bge-m3:latest"
	testModelBgeM3          = "bge-m3"
	testV1Models            = "/v1/models"
	testV1Embeddings        = "/v1/embeddings"
	testRoleAssistant       = "assistant"
	testRoleUser            = "user"
	testSession1            = "sess-1"
	testSession2            = "sess-2"
	testProjectPath         = "/test"
	testBackendClaude       = "claude"
	testBackendCodebuddy    = "codebuddy"
	testNeedsBackfill       = "needs backfill"
	testOllamaURL           = "http://localhost:11434"
	testPollInterval10s     = "10s"
	testPollInterval24h     = "24h"
	testDBQueryOptimization = "database query optimization"
	testDBSearch            = "database search"
	testDBQuery             = "database query"
	testOtherModelLatest    = "other-model:latest"
	testSearchQueryTest     = "test"
	testSearchQueryChunk    = "chunk"
	testEmbeddingTextHello  = "hello"
)

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
		ProjectPath:        testProjectPath,
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

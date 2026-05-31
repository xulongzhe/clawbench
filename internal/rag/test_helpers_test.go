package rag

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

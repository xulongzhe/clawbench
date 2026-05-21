package rag

import (
	"context"
	"encoding/json"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTextFromContent_UserMessage(t *testing.T) {
	text := ExtractTextFromContent("Hello, how are you?", "user")
	assert.Equal(t, "Hello, how are you?", text)
}

func TestExtractTextFromContent_AssistantTextOnly(t *testing.T) {
	content := `{"blocks":[{"type":"text","text":"Here is the answer."}]}`
	text := ExtractTextFromContent(content, "assistant")
	assert.Equal(t, "Here is the answer.", text)
}

func TestExtractTextFromContent_AssistantMixedBlocks(t *testing.T) {
	content := `{"blocks":[
		{"type":"text","text":"Let me read that file."},
		{"type":"thinking","text":"I should check the config..."},
		{"type":"tool_use","name":"Read","id":"toolu_1","input":{"file_path":"/etc/config"},"done":true},
		{"type":"text","text":"The config shows XYZ."}
	]}`
	text := ExtractTextFromContent(content, "assistant")
	assert.Equal(t, "Let me read that file.\n\nThe config shows XYZ.", text)
}

func TestExtractTextFromContent_AssistantOnlyToolUse(t *testing.T) {
	content := `{"blocks":[
		{"type":"tool_use","name":"Write","id":"toolu_1","input":{"file_path":"/tmp/test","content":"hello"},"done":true}
	]}`
	text := ExtractTextFromContent(content, "assistant")
	assert.Equal(t, "", text)
}

func TestExtractTextFromContent_InvalidJSON(t *testing.T) {
	text := ExtractTextFromContent("not json at all", "assistant")
	assert.Equal(t, "not json at all", text)
}

func TestEstimateTokens_English(t *testing.T) {
	tokens := estimateTokens("Hello world, this is a test.")
	assert.Greater(t, tokens, 0)
	assert.Less(t, tokens, 20) // Rough check
}

func TestEstimateTokens_CJK(t *testing.T) {
	tokens := estimateTokens("这是一个中文测试")
	assert.Greater(t, tokens, 0)
}

func TestEstimateTokens_Mixed(t *testing.T) {
	tokens := estimateTokens("Hello 你好 world 世界")
	assert.Greater(t, tokens, 0)
}

func TestChunkText_ShortText(t *testing.T) {
	chunks := ChunkText("Hello world", 512, 64)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "Hello world", chunks[0].Text)
	assert.Equal(t, 0, chunks[0].Index)
}

func TestChunkText_EmptyText(t *testing.T) {
	chunks := ChunkText("", 512, 64)
	assert.Nil(t, chunks)
}

func TestChunkText_LongText(t *testing.T) {
	// Generate a long text that should be split
	longText := ""
	for i := 0; i < 100; i++ {
		longText += "This is sentence number " + string(rune('0'+i%10)) + " in the test. "
	}
	// Should produce multiple chunks with chunkSize=50
	chunks := ChunkText(longText, 50, 10)
	assert.Greater(t, len(chunks), 1)

	// Verify indices are sequential
	for i, c := range chunks {
		assert.Equal(t, i, c.Index)
	}
}

func TestChunkText_ParagraphBreak(t *testing.T) {
	text := "First paragraph content.\n\nSecond paragraph content.\n\nThird paragraph content."
	chunks := ChunkText(text, 10, 2)
	// Should prefer paragraph breaks
	assert.Greater(t, len(chunks), 0)
}

// ---------- Init / Shutdown lifecycle ----------

func TestInit_CreatesStoreAndEmbedder(t *testing.T) {
	origBinDir := model.BinDir
	origStore := GlobalStore
	origEmbedder := GlobalEmbedder
	t.Cleanup(func() {
		model.BinDir = origBinDir
		GlobalStore = origStore
		GlobalEmbedder = origEmbedder
	})

	model.BinDir = t.TempDir()

	cfg := model.RAGConfig{
		BaseURL:      "http://localhost:11434",
		Model:        "bge-m3",
		ChunkSize:    512,
		ChunkOverlap: 64,
	}

	err := Init(cfg)
	require.NoError(t, err)
	assert.NotNil(t, GlobalStore, "GlobalStore should be initialized")
	assert.NotNil(t, GlobalEmbedder, "GlobalEmbedder should be initialized")

	// Cleanup
	GlobalStore.Close()
	GlobalStore = nil
	GlobalEmbedder = nil
}

func TestInit_DimensionMismatchResetsTable(t *testing.T) {
	origBinDir := model.BinDir
	origStore := GlobalStore
	origEmbedder := GlobalEmbedder
	t.Cleanup(func() {
		model.BinDir = origBinDir
		GlobalStore = origStore
		GlobalEmbedder = origEmbedder
	})

	model.BinDir = t.TempDir()

	// First init creates store with 1024-dim chunks
	cfg := model.RAGConfig{
		BaseURL: "http://localhost:11434",
		Model:   "bge-m3",
	}
	err := Init(cfg)
	require.NoError(t, err)

	// Insert a chunk with 1024-dim embedding
	insertTestChunks(t, GlobalStore, 1)
	count, _ := GlobalStore.ChunkCount()
	assert.Equal(t, 1, count)

	GlobalStore.Close()
	GlobalStore = nil
	GlobalEmbedder = nil

	// Re-init — 1024-dim matches expected, no reset
	err = Init(cfg)
	require.NoError(t, err)
	count, _ = GlobalStore.ChunkCount()
	assert.Equal(t, 1, count, "matching dimension should not reset")

	GlobalStore.Close()
	GlobalStore = nil
	GlobalEmbedder = nil
}

func TestStartIndexer_NilStoreSkips(t *testing.T) {
	origIndexer := GlobalIndexer
	origStore := GlobalStore
	origEmbedder := GlobalEmbedder
	t.Cleanup(func() {
		GlobalIndexer = origIndexer
		GlobalStore = origStore
		GlobalEmbedder = origEmbedder
	})

	GlobalStore = nil
	GlobalEmbedder = nil

	// Should not panic when store/embedder are nil
	StartIndexer(model.RAGConfig{PollInterval: "10s"})
	assert.Nil(t, GlobalIndexer, "should not create indexer with nil store")
}

func TestShutdown_Idempotent(t *testing.T) {
	origStore := GlobalStore
	origEmbedder := GlobalEmbedder
	origIndexer := GlobalIndexer
	origCleanup := GlobalCleanupWorker
	t.Cleanup(func() {
		GlobalStore = origStore
		GlobalEmbedder = origEmbedder
		GlobalIndexer = origIndexer
		GlobalCleanupWorker = origCleanup
	})

	// Shutdown with nil singletons — should not panic
	GlobalCleanupWorker = nil
	GlobalIndexer = nil
	GlobalStore = nil
	GlobalEmbedder = nil

	Shutdown() // first call — no-op
	Shutdown() // second call — also no-op

	// Now with a real store
	origBinDir := model.BinDir
	t.Cleanup(func() {
		model.BinDir = origBinDir
	})
	model.BinDir = t.TempDir()

	err := Init(model.RAGConfig{
		BaseURL: "http://localhost:11434",
		Model:   "bge-m3",
	})
	require.NoError(t, err)

	Shutdown() // should close store
	assert.Nil(t, GlobalStore)
	assert.Nil(t, GlobalEmbedder)

	Shutdown() // idempotent — no panic
}

// ---------- SearchParams / SearchResult JSON ----------

func TestSearchParams_JSONRoundTrip(t *testing.T) {
	params := SearchParams{
		Query:            "test query",
		Limit:            10,
		ProjectPath:      "/project",
		Backend:          "claude",
		Role:             "assistant",
		SessionID:        "sess-1",
		ExcludeSessionID: "sess-2",
		FromTime:         "2024-01-01",
		ToTime:           "2024-12-31",
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	var decoded SearchParams
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, params, decoded)
}

func TestSearchResult_EmptyResultsNotNil(t *testing.T) {
	// SearchResult with nil Results slice should be valid
	result := &SearchResult{Results: nil, Total: 0}
	data, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"results":null`)

	// Unmarshal back
	var decoded SearchResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
}

// ---------- Mock Ollama for rag_test helpers ----------

func TestNewHealthyMockEmbedder(t *testing.T) {
	// Verify our test helper works correctly
	client, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err)
	assert.True(t, reachable)
	assert.True(t, modelAvailable)
}

func TestMockEmbedderEndpoint(t *testing.T) {
	client, cleanup := newHealthyMockEmbedder(t)
	defer cleanup()

	emb, err := client.Embed(context.Background(), "test")
	assert.NoError(t, err)
	assert.Len(t, emb, 1024)
}

// ---------- StartCleanupWorker ----------

func TestStartCleanupWorker_ZeroRetention(t *testing.T) {
	origCleanup := GlobalCleanupWorker
	t.Cleanup(func() {
		GlobalCleanupWorker = origCleanup
	})

	// Zero retention days should skip starting the worker
	StartCleanupWorker(model.RAGConfig{RetentionDays: 0})
	assert.Nil(t, GlobalCleanupWorker, "should not start cleanup worker with zero retention")
}

func TestStartCleanupWorker_WithRetention(t *testing.T) {
	origStore := GlobalStore
	origCleanup := GlobalCleanupWorker
	t.Cleanup(func() {
		if GlobalCleanupWorker != nil {
			GlobalCleanupWorker.Stop()
		}
		GlobalCleanupWorker = origCleanup
		GlobalStore = origStore
	})

	origBinDir := model.BinDir
	t.Cleanup(func() { model.BinDir = origBinDir })
	model.BinDir = t.TempDir()

	// Need a store for cleanup worker
	store, err := InitStore()
	require.NoError(t, err)
	GlobalStore = store

	StartCleanupWorker(model.RAGConfig{RetentionDays: 30})
	assert.NotNil(t, GlobalCleanupWorker, "should start cleanup worker with positive retention")

	GlobalCleanupWorker.Stop()
	GlobalStore.Close()
	GlobalStore = origStore
	GlobalCleanupWorker = nil
}

// ---------- Init with segmenter warning ----------

func TestInit_SegmenterWarningContinues(t *testing.T) {
	origBinDir := model.BinDir
	origStore := GlobalStore
	origEmbedder := GlobalEmbedder
	t.Cleanup(func() {
		model.BinDir = origBinDir
		GlobalStore = origStore
		GlobalEmbedder = origEmbedder
	})

	model.BinDir = t.TempDir()

	cfg := model.RAGConfig{
		BaseURL: "http://localhost:11434",
		Model:   "bge-m3",
	}

	// Init should succeed even if segmenter is not available
	err := Init(cfg)
	require.NoError(t, err)
	assert.NotNil(t, GlobalStore)

	GlobalStore.Close()
	GlobalStore = nil
	GlobalEmbedder = nil
}

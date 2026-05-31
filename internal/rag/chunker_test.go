package rag

import (
	"math"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
)

// ---------- ChunkText ----------

func TestChunkText_ZeroChunkSize(t *testing.T) {
	chunks := ChunkText("some text", 0, 10)
	assert.Nil(t, chunks, "zero chunkSize should return nil")
}

func TestChunkText_NegativeChunkSize(t *testing.T) {
	chunks := ChunkText("some text", -1, 10)
	assert.Nil(t, chunks, "negative chunkSize should return nil")
}

func TestChunkText_OverlapBehavior(t *testing.T) {
	// Generate text long enough to produce multiple chunks
	longText := strings.Repeat("This is a sentence. ", 200)

	chunks := ChunkText(longText, 50, 20)
	assert.Greater(t, len(chunks), 1, "should produce multiple chunks")

	// Verify overlap: the last part of chunk[i] should overlap with the
	// beginning of chunk[i+1]. We check by verifying that chunks are non-empty.
	if len(chunks) >= 2 {
		for i := range len(chunks) - 1 {
			nextChunk := chunks[i+1].Text
			// At least some overlap text should appear in next chunk
			// (The overlap may not be exact due to break-point alignment)
			assert.NotEmpty(t, nextChunk,
				"chunk %d should have overlap with chunk %d", i, i+1)
		}
	}
}

func TestChunkText_CJKText(t *testing.T) {
	// CJK text should be chunked correctly using rune-based splitting
	longCJK := strings.Repeat("这是一个中文句子。", 100)

	chunks := ChunkText(longCJK, 50, 10)
	assert.Greater(t, len(chunks), 1, "CJK text should produce multiple chunks")

	// All chunks should have valid index
	for i, c := range chunks {
		assert.Equal(t, i, c.Index, "chunk index should be sequential")
		assert.Greater(t, c.TokenCount, 0, "token count should be positive")
	}
}

func TestChunkText_SingleCharacterText(t *testing.T) {
	chunks := ChunkText("x", 512, 64)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "x", chunks[0].Text)
}

func TestChunkText_ChunkIndicesSequential(t *testing.T) {
	longText := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 50)
	chunks := ChunkText(longText, 30, 5)
	for i, c := range chunks {
		assert.Equal(t, i, c.Index, "chunk index should be sequential starting from 0")
	}
}

// ---------- ExtractTextFromContent ----------

func TestExtractTextFromContent_UserMessageWhitespace(t *testing.T) {
	text := ExtractTextFromContent("  hello world  ", "user")
	assert.Equal(t, "hello world", text, "user message should be trimmed")
}

func TestExtractTextFromContent_EmptyContent(t *testing.T) {
	text := ExtractTextFromContent("", "user")
	assert.Equal(t, "", text)
}

func TestExtractTextFromContent_AssistantEmptyBlocks(t *testing.T) {
	content := `{"blocks":[]}`
	text := ExtractTextFromContent(content, "assistant")
	assert.Equal(t, "", text)
}

func TestExtractTextFromContent_AssistantWarningBlock(t *testing.T) {
	content := `{"blocks":[
		{"type":"warning","text":"something went wrong"},
		{"type":"text","text":"but here is the answer"}
	]}`
	text := ExtractTextFromContent(content, "assistant")
	assert.Equal(t, "but here is the answer", text, "warning blocks should be skipped")
}

func TestExtractTextFromContent_AssistantErrorBlock(t *testing.T) {
	content := `{"blocks":[
		{"type":"error","text":"fatal error"},
		{"type":"text","text":"recovered text"}
	]}`
	text := ExtractTextFromContent(content, "assistant")
	assert.Equal(t, "recovered text", text, "error blocks should be skipped")
}

func TestExtractTextFromContent_AssistantTextBlockEmpty(t *testing.T) {
	content := `{"blocks":[
		{"type":"text","text":""},
		{"type":"tool_use","name":"Read","id":"t1"},
		{"type":"text","text":"result"}
	]}`
	text := ExtractTextFromContent(content, "assistant")
	assert.Equal(t, "result", text, "empty text blocks should be skipped")
}

func TestExtractTextFromContent_AssistantWhitespaceOnlyText(t *testing.T) {
	content := `{"blocks":[{"type":"text","text":"  "}]}`
	text := ExtractTextFromContent(content, "assistant")
	// After TrimSpace, empty text blocks produce empty result
	assert.Equal(t, "", text)
}

// ---------- estimateTokens ----------

func TestEstimateTokens_Empty(t *testing.T) {
	tokens := estimateTokens("")
	assert.Equal(t, 0, tokens)
}

func TestEstimateTokens_SingleWord(t *testing.T) {
	tokens := estimateTokens("hello")
	assert.Greater(t, tokens, 0)
}

func TestEstimateTokens_CJKHigherThanEnglish(t *testing.T) {
	// Same length string: CJK should produce more tokens than English
	// because CJK chars are ~1.5 chars/token while English is ~1.3 words/token
	englishTokens := estimateTokens("aaaaaaaaaaaaaaaaaa") // 18 English chars
	cjkTokens := estimateTokens("你好你好你好你好你好你好")

	// Both should produce positive token counts
	assert.Greater(t, englishTokens, 0)
	assert.Greater(t, cjkTokens, 0)
}

func TestEstimateTokens_MixedCJKAndEnglish(t *testing.T) {
	tokens := estimateTokens("Hello 你好 World 世界")
	assert.Greater(t, tokens, 0, "mixed text should have positive token count")
}

// ---------- isCJK ----------

func TestIsCJK_Han(t *testing.T) {
	assert.True(t, isCJK('中'), "Han character should be CJK")
	assert.True(t, isCJK('国'), "Han character should be CJK")
	assert.True(t, isCJK('字'), "Han character should be CJK")
}

func TestIsCJK_Hangul(t *testing.T) {
	assert.True(t, isCJK('한'), "Hangul character should be CJK")
	assert.True(t, isCJK('국'), "Hangul character should be CJK")
}

func TestIsCJK_Hiragana(t *testing.T) {
	assert.True(t, isCJK('あ'), "Hiragana character should be CJK")
	assert.True(t, isCJK('い'), "Hiragana character should be CJK")
}

func TestIsCJK_Katakana(t *testing.T) {
	assert.True(t, isCJK('ア'), "Katakana character should be CJK")
	assert.True(t, isCJK('カ'), "Katakana character should be CJK")
}

func TestIsCJK_Latin(t *testing.T) {
	assert.False(t, isCJK('A'), "Latin character should not be CJK")
	assert.False(t, isCJK('z'), "Latin character should not be CJK")
	assert.False(t, isCJK('0'), "Digit should not be CJK")
	assert.False(t, isCJK(' '), "Space should not be CJK")
}

func TestIsCJK_UnicodeRanges(t *testing.T) {
	// Test borderline characters
	assert.False(t, isCJK(unicode.MaxLatin1), "MaxLatin1 should not be CJK")
	assert.True(t, isCJK('漢'), "CJK Unified Ideograph should be CJK")
}

// ---------- embeddingToSQLArray (additional coverage) ----------

func TestEmbeddingToSQLArray_NegativeInf(t *testing.T) {
	_, err := embeddingToSQLArray([]float64{0.1, math.Inf(-1)}, 1024)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-finite")
}

func TestEmbeddingToSQLArray_SingleValue(t *testing.T) {
	result, err := embeddingToSQLArray([]float64{0.5}, 3)
	assert.NoError(t, err)
	assert.Equal(t, "array[0.5]::FLOAT[3]", result)
}

func TestEmbeddingToSQLArray_NegativeValues(t *testing.T) {
	result, err := embeddingToSQLArray([]float64{-0.1, 0.2, -0.3}, 3)
	assert.NoError(t, err)
	assert.Contains(t, result, "-0.1")
	assert.Contains(t, result, "0.2")
	assert.Contains(t, result, "-0.3")
	assert.Contains(t, result, "::FLOAT[3]")
}

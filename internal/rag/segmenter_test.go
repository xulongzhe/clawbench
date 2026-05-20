package rag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSegmentText_Chinese(t *testing.T) {
	err := InitSegmenter()
	if err != nil {
		t.Skip("gse segmenter not available:", err)
	}

	result := SegmentText("人工智能技术发展")
	// Should produce space-separated tokens, not the original unsegmented string
	assert.NotEqual(t, "人工智能技术发展", result, "segmented text should differ from original")
	assert.Contains(t, result, "人工", "should contain '人工' token")
	// Tokens should be space-separated
	assert.Contains(t, result, " ", "tokens should be space-separated")
}

func TestSegmentText_English(t *testing.T) {
	err := InitSegmenter()
	if err != nil {
		t.Skip("gse segmenter not available:", err)
	}

	result := SegmentText("hello world")
	// English text should pass through with spaces
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, "world")
}

func TestSegmentText_Mixed(t *testing.T) {
	err := InitSegmenter()
	if err != nil {
		t.Skip("gse segmenter not available:", err)
	}

	result := SegmentText("使用DuckDB进行全文检索")
	// Should segment Chinese portions
	assert.Contains(t, result, " ", "mixed text should have space-separated tokens")
	// English portion should be preserved (gse lowercases, so check lowercase)
	assert.Contains(t, result, "duckdb", "English token should be present (lowercased by gse)")
}

func TestSegmentText_NilSegmenter(t *testing.T) {
	// Reset segmenter to nil
	origSegmenter := segmenter
	segmenter = nil
	t.Cleanup(func() { segmenter = origSegmenter })

	result := SegmentText("人工智能技术发展")
	assert.Equal(t, "人工智能技术发展", result, "nil segmenter should return original text")
}

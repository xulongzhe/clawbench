package service

import (
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestExtractTextFromBlocks_Empty(t *testing.T) {
	assert.Equal(t, "", extractTextFromBlocks(nil))
	assert.Equal(t, "", extractTextFromBlocks([]model.ContentBlock{}))
}

func TestExtractTextFromBlocks_TextOnly(t *testing.T) {
	blocks := []model.ContentBlock{
		{Type: "text", Text: "Hello"},
		{Type: "text", Text: "World"},
	}
	assert.Equal(t, "Hello\n\nWorld", extractTextFromBlocks(blocks))
}

func TestExtractTextFromBlocks_MixedTypes(t *testing.T) {
	blocks := []model.ContentBlock{
		{Type: "text", Text: "First"},
		{Type: "tool_use", Text: "ignored"},
		{Type: "thinking", Text: "also ignored"},
		{Type: "text", Text: "Second"},
	}
	assert.Equal(t, "First\n\nSecond", extractTextFromBlocks(blocks))
}

func TestExtractTextFromBlocks_SkipsEmptyText(t *testing.T) {
	blocks := []model.ContentBlock{
		{Type: "text", Text: "First"},
		{Type: "text", Text: ""},
		{Type: "text", Text: "Second"},
	}
	assert.Equal(t, "First\n\nSecond", extractTextFromBlocks(blocks))
}

func TestExtractTextFromBlocks_SingleTextBlock(t *testing.T) {
	blocks := []model.ContentBlock{
		{Type: "text", Text: "Only one"},
	}
	assert.Equal(t, "Only one", extractTextFromBlocks(blocks))
}

func TestExtractTextFromBlocks_NoTextBlocks(t *testing.T) {
	blocks := []model.ContentBlock{
		{Type: "tool_use", Text: "tool"},
		{Type: "thinking", Text: "thought"},
	}
	assert.Equal(t, "", extractTextFromBlocks(blocks))
}

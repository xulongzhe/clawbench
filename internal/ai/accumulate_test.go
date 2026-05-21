package ai

import (
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestAccumulateBlock_Content(t *testing.T) {
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{Type: "content", Content: "Hello"})
	AccumulateBlock(&blocks, StreamEvent{Type: "content", Content: " world"})
	assert.Len(t, blocks, 1)
	assert.Equal(t, "Hello world", blocks[0].Text)
}

func TestAccumulateBlock_Thinking(t *testing.T) {
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{Type: "thinking", Content: "Think"})
	AccumulateBlock(&blocks, StreamEvent{Type: "thinking", Content: " more"})
	assert.Len(t, blocks, 1)
	assert.Equal(t, "Think more", blocks[0].Text)
}

func TestAccumulateBlock_ToolUseStart(t *testing.T) {
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Read", ID: "t1", Input: `{"file_path":"/a.go"}`, Done: false},
	})
	assert.Len(t, blocks, 1)
	assert.Equal(t, "tool_use", blocks[0].Type)
	assert.Equal(t, "Read", blocks[0].Name)
	assert.Equal(t, "t1", blocks[0].ID)
	assert.False(t, blocks[0].Done)
	assert.Equal(t, "/a.go", blocks[0].Input["file_path"])
}

func TestAccumulateBlock_ToolUseDone(t *testing.T) {
	blocks := []model.ContentBlock{}
	// Start event
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Read", ID: "t1", Input: `{"file_path":"/a.go"}`, Done: false},
	})
	// Done event (same ID, updates existing block)
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Read", ID: "t1", Input: `{"file_path":"/a.go"}`, Done: true},
	})
	assert.Len(t, blocks, 1)
	assert.True(t, blocks[0].Done)
}

func TestAccumulateBlock_ToolUseWithOutput(t *testing.T) {
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Bash", ID: "t2", Input: `{"command":"ls"}`, Done: true, Output: "file1.go\nfile2.go", Status: "success"},
	})
	assert.Len(t, blocks, 1)
	assert.Equal(t, "file1.go\nfile2.go", blocks[0].Output)
	assert.Equal(t, "success", blocks[0].Status)
}

func TestAccumulateBlock_ToolUseOutputUpdate(t *testing.T) {
	blocks := []model.ContentBlock{}
	// Start without output
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Bash", ID: "t2", Input: `{"command":"ls"}`, Done: false},
	})
	// Done event adds output/status
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Bash", ID: "t2", Done: true, Output: "output text", Status: "success"},
	})
	assert.Len(t, blocks, 1)
	assert.Equal(t, "output text", blocks[0].Output)
	assert.Equal(t, "success", blocks[0].Status)
}

func TestAccumulateBlock_ToolResultUpdatesExisting(t *testing.T) {
	blocks := []model.ContentBlock{}
	// First: tool_use without output
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Read", ID: "t3", Input: `{"file_path":"/a.go"}`, Done: true},
	})
	assert.Equal(t, "", blocks[0].Output)
	assert.Equal(t, "", blocks[0].Status)

	// Then: tool_result event fills in output/status
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_result",
		Tool: &ToolCall{ID: "t3", Output: "file contents here", Status: "success"},
	})
	assert.Len(t, blocks, 1) // No new block added
	assert.Equal(t, "file contents here", blocks[0].Output)
	assert.Equal(t, "success", blocks[0].Status)
}

func TestAccumulateBlock_ToolResultError(t *testing.T) {
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Bash", ID: "t4", Input: `{"command":"bad-cmd"}`, Done: true},
	})
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_result",
		Tool: &ToolCall{ID: "t4", Output: "command not found", Status: "error"},
	})
	assert.Equal(t, "command not found", blocks[0].Output)
	assert.Equal(t, "error", blocks[0].Status)
}

func TestAccumulateBlock_ToolResultNoMatch(t *testing.T) {
	blocks := []model.ContentBlock{}
	// tool_result for an ID that doesn't match any existing block — silently ignored
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_result",
		Tool: &ToolCall{ID: "nonexistent", Output: "output", Status: "success"},
	})
	assert.Len(t, blocks, 0)
}

func TestAccumulateBlock_ToolResultNilTool(t *testing.T) {
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{Type: "tool_result", Tool: nil})
	assert.Len(t, blocks, 0)
}

func TestAccumulateBlock_ToolUseNilTool(t *testing.T) {
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{Type: "tool_use", Tool: nil})
	assert.Len(t, blocks, 0)
}

func TestAccumulateBlock_Warning(t *testing.T) {
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{Type: "warning", Content: "slow response", Reason: "timeout"})
	assert.Len(t, blocks, 1)
	assert.Equal(t, "warning", blocks[0].Type)
	assert.Equal(t, "slow response", blocks[0].Text)
	assert.Equal(t, "timeout", blocks[0].Reason)
}

func TestAccumulateBlock_ContentAfterToolUse(t *testing.T) {
	// text after tool_use should NOT merge with text before tool_use
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{Type: "content", Content: "before"})
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Read", ID: "t5", Input: `{}`, Done: true},
	})
	AccumulateBlock(&blocks, StreamEvent{Type: "content", Content: "after"})
	assert.Len(t, blocks, 3)
	assert.Equal(t, "before", blocks[0].Text)
	assert.Equal(t, "tool_use", blocks[1].Type)
	assert.Equal(t, "after", blocks[2].Text)
}

func TestAccumulateBlock_MultipleToolResults(t *testing.T) {
	// Multiple tool_use blocks + tool_result events that match by ID
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{Type: "tool_use", Tool: &ToolCall{Name: "Read", ID: "t1", Input: `{}`, Done: true}})
	AccumulateBlock(&blocks, StreamEvent{Type: "tool_use", Tool: &ToolCall{Name: "Bash", ID: "t2", Input: `{}`, Done: true}})

	// tool_result for t2 arrives (out of order)
	AccumulateBlock(&blocks, StreamEvent{Type: "tool_result", Tool: &ToolCall{ID: "t2", Output: "bash output", Status: "success"}})
	// tool_result for t1
	AccumulateBlock(&blocks, StreamEvent{Type: "tool_result", Tool: &ToolCall{ID: "t1", Output: "read output", Status: "success"}})

	assert.Len(t, blocks, 2)
	assert.Equal(t, "read output", blocks[0].Output)
	assert.Equal(t, "success", blocks[0].Status)
	assert.Equal(t, "bash output", blocks[1].Output)
	assert.Equal(t, "success", blocks[1].Status)
}

func TestAccumulateBlock_ToolResultOverwritesEmptyOutput(t *testing.T) {
	// tool_use with empty output → tool_result fills it in
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Grep", ID: "t6", Input: `{"pattern":"TODO"}`, Done: true, Output: "", Status: ""},
	})
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_result",
		Tool: &ToolCall{ID: "t6", Output: "main.go:42: TODO fix this", Status: "success"},
	})
	assert.Equal(t, "main.go:42: TODO fix this", blocks[0].Output)
	assert.Equal(t, "success", blocks[0].Status)
}

func TestAccumulateBlock_ErrorEvent(t *testing.T) {
	// "error" event type creates a warning ContentBlock with Error and Reason
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{Type: "error", Error: "connection lost", Reason: "disconnect"})
	assert.Len(t, blocks, 1)
	assert.Equal(t, "warning", blocks[0].Type, "error event should produce a warning block")
	assert.Equal(t, "connection lost", blocks[0].Text)
	assert.Equal(t, "disconnect", blocks[0].Reason)
}

func TestAccumulateBlock_ToolUseMalformedJSON(t *testing.T) {
	// Malformed JSON input should result in an empty map, not a crash
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{
		Type: "tool_use",
		Tool: &ToolCall{Name: "Bash", ID: "t7", Input: `{invalid json`, Done: true},
	})
	assert.Len(t, blocks, 1)
	assert.Equal(t, "tool_use", blocks[0].Type)
	assert.NotNil(t, blocks[0].Input, "input should be non-nil even with malformed JSON")
	assert.Empty(t, blocks[0].Input, "malformed JSON should produce empty input map")
}

func TestAccumulateBlock_ThinkingAndContentInterleaved(t *testing.T) {
	// Thinking and content without tool_use boundaries should coalesce correctly
	blocks := []model.ContentBlock{}
	AccumulateBlock(&blocks, StreamEvent{Type: "thinking", Content: "think1"})
	AccumulateBlock(&blocks, StreamEvent{Type: "content", Content: "text1"})
	AccumulateBlock(&blocks, StreamEvent{Type: "thinking", Content: "think2"})
	AccumulateBlock(&blocks, StreamEvent{Type: "content", Content: "text2"})

	// Without tool_use boundaries, same-type blocks coalesce:
	// thinking: "think1" then coalesce "think2" into first thinking block
	// content: "text1" then coalesce "text2" into first content block
	// But they interleave, so: thinking block, content block, and
	// the second thinking should coalesce into the first thinking block,
	// and second content into first content block.
	assert.Len(t, blocks, 2, "should have thinking and content blocks")
	assert.Equal(t, "thinking", blocks[0].Type)
	assert.Equal(t, "think1think2", blocks[0].Text, "thinking deltas should coalesce across content blocks")
	assert.Equal(t, "text", blocks[1].Type)
	assert.Equal(t, "text1text2", blocks[1].Text, "content deltas should coalesce across thinking blocks")
}

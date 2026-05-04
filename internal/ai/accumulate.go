package ai

import (
	"encoding/json"

	"clawbench/internal/model"
)

// AccumulateBlock processes a single StreamEvent and updates the blocks slice.
// Both text and thinking events are coalesced into the most recent block of
// the same type; tool_use events are deduplicated by ID.
//
// When AI models (e.g. GLM-5.1) interleave thinking_delta and text_delta events,
// the last block may not be the same type as the incoming event. Instead of only
// checking the last block, we search backward for the most recent block of the
// same type and merge into it. However, tool_use blocks act as natural boundaries —
// text/thinking after a tool_use should not be merged with text/thinking before it.
// This prevents a single thinking or text block from being fragmented into many
// tiny blocks when events alternate, while preserving the semantic separation
// around tool calls.
func AccumulateBlock(blocks *[]model.ContentBlock, event StreamEvent) {
	// findLastBlockOfType searches backward for the most recent block of the
	// given type, but stops at tool_use boundaries (they are natural separators).
	findLastBlockOfType := func(typ string) (int, bool) {
		for i := len(*blocks) - 1; i >= 0; i-- {
			if (*blocks)[i].Type == typ {
				return i, true
			}
			// tool_use blocks are natural boundaries — don't merge across them
			if (*blocks)[i].Type == "tool_use" {
				return -1, false
			}
		}
		return -1, false
	}

	switch event.Type {
	case "content":
		// Note: <schedule-proposal> detection is done in finalize phase on
		// the fully accumulated text, not here on incremental deltas — the
		// tag spans multiple delta events and is never complete in any single one.
		// Coalesce incremental content deltas into the most recent text block.
		if idx, found := findLastBlockOfType("text"); found {
			(*blocks)[idx].Text += event.Content
		} else {
			*blocks = append(*blocks, model.ContentBlock{Type: "text", Text: event.Content})
		}
	case "thinking":
		// Coalesce incremental thinking deltas into the most recent thinking block.
		if idx, found := findLastBlockOfType("thinking"); found {
			(*blocks)[idx].Text += event.Content
		} else {
			*blocks = append(*blocks, model.ContentBlock{Type: "thinking", Text: event.Content})
		}
	case "tool_use":
		if event.Tool != nil {
			// Parse tool input JSON into map
			var input map[string]any
			if event.Tool.Input != "" {
				json.Unmarshal([]byte(event.Tool.Input), &input)
			}
			if input == nil {
				input = make(map[string]any)
			}
			// Find existing block by tool ID and update, or append new
			found := false
			for i := len(*blocks) - 1; i >= 0; i-- {
				if (*blocks)[i].Type == "tool_use" && (*blocks)[i].ID == event.Tool.ID {
					(*blocks)[i].Input = input
					(*blocks)[i].Done = event.Tool.Done
					found = true
					break
				}
			}
			if !found {
				*blocks = append(*blocks, model.ContentBlock{
					Type:  "tool_use",
					Name:  event.Tool.Name,
					ID:    event.Tool.ID,
					Input: input,
					Done:  event.Tool.Done,
				})
			}
		}
	case "warning":
		*blocks = append(*blocks, model.ContentBlock{Type: "warning", Text: event.Content, Reason: event.Reason})
	case "error":
		*blocks = append(*blocks, model.ContentBlock{Type: "warning", Text: event.Error, Reason: event.Reason})
	}
}

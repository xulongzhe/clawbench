package handler

import (
	"strings"
	"testing"

	"clawbench/internal/model"
)

func TestConvertAskQuestionBlocks_WrongCloseTag_StripsTagFromText(t *testing.T) {
	// Regression test: When Strategy 2 (wrong-close regex) matches a non-standard
	// closing tag instead of the standard </ask-question>, the <ask-question> content
	// must be stripped from the text block. Previously, only Strategy 3 (unclosed)
	// set matchStartIdx, so Strategy 2 matches left the tag in the text, causing
	// duplicate ask-question cards (one from frontend detectAskQuestion, one from
	// the tool_use block).
	blocks := []model.ContentBlock{
		{Type: "text", Text: "Here is my analysis.\n\n---\n\n<ask-question>\n{\"questions\":[{\"header\":\"Pick\",\"multiSelect\":false,\"options\":[{\"label\":\"A\",\"description\":\"Option A\"}],\"question\":\"Which one?\"}]}\n</ask-question>"},
	}

	result := convertAskQuestionBlocks(blocks)

	askQCount := 0
	textHasAskTag := false
	for _, b := range result {
		if b.Type == "tool_use" && b.Name == "AskUserQuestion" {
			askQCount++
		}
		if b.Type == "text" && strings.Contains(b.Text, "<ask-question") {
			textHasAskTag = true
		}
	}

	if askQCount != 1 {
		t.Errorf("expected 1 AskUserQuestion tool_use block, got %d", askQCount)
	}
	if textHasAskTag {
		t.Error("text block should NOT contain <ask-question> tag - it must be stripped to avoid duplicate cards")
	}
}

func TestConvertAskQuestionBlocks_IDUsesUUID(t *testing.T) {
	// Verify that the tool_use block ID uses UUID format ("ask-" + UUID)
	// instead of the old format ("ask-" + unixNano%1000000).
	blocks := []model.ContentBlock{
		{Type: "text", Text: "<ask-question>\n{\"questions\":[{\"header\":\"Pick\",\"multiSelect\":false,\"options\":[{\"label\":\"A\",\"description\":\"Option A\"}],\"question\":\"Which one?\"}]}\n</ask-question>"},
	}

	result := convertAskQuestionBlocks(blocks)

	for _, b := range result {
		if b.Type == "tool_use" && b.Name == "AskUserQuestion" {
			// ID should start with "ask-" followed by a UUID (8-4-4-4-12 format)
			if !strings.HasPrefix(b.ID, "ask-") {
				t.Errorf("expected ID to start with 'ask-', got %q", b.ID)
			}
			uuidPart := strings.TrimPrefix(b.ID, "ask-")
			// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (36 chars with dashes)
			if len(uuidPart) != 36 {
				t.Errorf("expected UUID part to be 36 chars, got %d (ID=%q)", len(uuidPart), b.ID)
			}
			// Check for dashes at expected positions
			for i, c := range uuidPart {
				switch i {
				case 8, 13, 18, 23:
					if c != '-' {
						t.Errorf("expected dash at position %d in UUID, got %c (ID=%q)", i, c, b.ID)
					}
				default:
					if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
						t.Errorf("expected hex digit at position %d in UUID, got %c (ID=%q)", i, c, b.ID)
					}
				}
			}
			return
		}
	}
	t.Error("expected to find an AskUserQuestion tool_use block")
}

func TestConvertAskQuestionBlocks_IDsAreUnique(t *testing.T) {
	// Generate multiple blocks and verify that IDs are unique (UUID-based)
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		blocks := []model.ContentBlock{
			{Type: "text", Text: "<ask-question>\n{\"questions\":[{\"header\":\"Pick\",\"multiSelect\":false,\"options\":[{\"label\":\"A\",\"description\":\"Option A\"}],\"question\":\"Which one?\"}]}\n</ask-question>"},
		}

		result := convertAskQuestionBlocks(blocks)
		for _, b := range result {
			if b.Type == "tool_use" && b.Name == "AskUserQuestion" {
				if ids[b.ID] {
					t.Errorf("duplicate ID generated: %q", b.ID)
				}
				ids[b.ID] = true
			}
		}
	}
	if len(ids) != 10 {
		t.Errorf("expected 10 unique IDs, got %d", len(ids))
	}
}

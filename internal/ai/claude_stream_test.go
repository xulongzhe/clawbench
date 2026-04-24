package ai

import (
	"encoding/json"
	"testing"
)

// parseLine is a test helper that creates a StreamParser, parses one line, and returns the events.
func parseLine(line string) []StreamEvent {
	ch := make(chan StreamEvent, 64)
	parser := &StreamParser{}
	parser.ParseLine(line, ch)
	close(ch)

	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

// parseLines is a test helper that feeds multiple lines through a single StreamParser.
func parseLines(lines []string) []StreamEvent {
	ch := make(chan StreamEvent, 64)
	parser := &StreamParser{}
	for _, line := range lines {
		if line != "" {
			parser.ParseLine(line, ch)
		}
	}
	close(ch)

	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestStreamParser_AssistantContent(t *testing.T) {
	msg := ClaudeStreamMessage{
		Type: "assistant",
		Message: &ClaudeStreamMessageBody{
			Content: []ClaudeContentBlock{
				{Type: "text", Text: "Hello, world!"},
			},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal test message: %v", err)
	}

	events := parseLine(string(data))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "content" {
		t.Errorf("expected event type 'content', got %q", events[0].Type)
	}
	if events[0].Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %q", events[0].Content)
	}
}

func TestStreamParser_SystemEventsSkipped(t *testing.T) {
	lines := []string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"system","subtype":"start"}`,
	}

	events := parseLines(lines)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for system messages, got %d: %+v", len(events), events)
	}
}

func TestStreamParser_MultipleContentBlocks(t *testing.T) {
	msg := ClaudeStreamMessage{
		Type: "assistant",
		Message: &ClaudeStreamMessageBody{
			Content: []ClaudeContentBlock{
				{Type: "text", Text: "First block"},
				{Type: "tool_use", ID: "tool1", Name: "Read"},
				{Type: "text", Text: "Second block"},
			},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal test message: %v", err)
	}

	events := parseLine(string(data))

	// Should produce: 2 content events + 1 tool_use, in block order
	if len(events) != 3 {
		t.Fatalf("expected 3 events (2 content + 1 tool_use), got %d", len(events))
	}
	// First: first text
	if events[0].Type != "content" || events[0].Content != "First block" {
		t.Errorf("expected first content 'First block', got type=%q content=%q", events[0].Type, events[0].Content)
	}
	// Second: tool_use
	if events[1].Type != "tool_use" {
		t.Errorf("expected second event type 'tool_use', got %q", events[1].Type)
	}
	if events[1].Tool == nil || events[1].Tool.Name != "Read" {
		t.Errorf("expected tool name 'Read', got %v", events[1].Tool)
	}
	// Third: second text
	if events[2].Type != "content" || events[2].Content != "Second block" {
		t.Errorf("expected second content 'Second block', got type=%q content=%q", events[2].Type, events[2].Content)
	}
}

func TestStreamParser_ResultMetadata(t *testing.T) {
	msg := ClaudeStreamMessage{
		Type:         "result",
		Subtype:      "success",
		DurationMs:   5000,
		TotalCostUSD: 0.05,
		SessionID:    "sess-123",
		IsError:      false,
		Usage: &ClaudeStreamUsage{
			InputTokens:  100,
			OutputTokens: 200,
		},
		ModelUsage: map[string]ClaudeStreamModelUsage{
			"claude-3-5-sonnet": {InputTokens: 100, OutputTokens: 200},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal test message: %v", err)
	}

	events := parseLine(string(data))

	// Result should produce: metadata event + done event
	if len(events) != 2 {
		t.Fatalf("expected 2 events (metadata + done), got %d", len(events))
	}

	// First event: metadata
	if events[0].Type != "metadata" {
		t.Errorf("expected first event type 'metadata', got %q", events[0].Type)
	}
	if events[0].Meta == nil {
		t.Fatal("expected metadata to be non-nil")
	}
	if events[0].Meta.SessionID != "sess-123" {
		t.Errorf("expected session ID 'sess-123', got %q", events[0].Meta.SessionID)
	}
	if events[0].Meta.DurationMs != 5000 {
		t.Errorf("expected duration 5000, got %d", events[0].Meta.DurationMs)
	}
	if events[0].Meta.CostUSD != 0.05 {
		t.Errorf("expected cost 0.05, got %f", events[0].Meta.CostUSD)
	}
	if events[0].Meta.InputTokens != 100 {
		t.Errorf("expected input tokens 100, got %d", events[0].Meta.InputTokens)
	}
	if events[0].Meta.OutputTokens != 200 {
		t.Errorf("expected output tokens 200, got %d", events[0].Meta.OutputTokens)
	}

	// Second event: done
	if events[1].Type != "done" {
		t.Errorf("expected second event type 'done', got %q", events[1].Type)
	}
}

func TestStreamParser_ResultWithIsError(t *testing.T) {
	msg := ClaudeStreamMessage{
		Type:      "result",
		IsError:   true,
		Result:    "something went wrong",
		SessionID: "sess-err",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	events := parseLine(string(data))

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Type != "warning" || events[0].Content != "something went wrong" {
		t.Errorf("expected warning event with error message, got %v", events[0])
	}
	if events[1].Meta == nil || !events[1].Meta.IsError {
		t.Fatalf("expected IsError=true in metadata")
	}
	if events[1].Meta.ErrorMessage != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got %q", events[1].Meta.ErrorMessage)
	}
	if events[2].Type != "done" {
		t.Errorf("expected third event type 'done', got %q", events[2].Type)
	}
}

func TestStreamParser_ThinkingDelta(t *testing.T) {
	lines := []string{
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me think..."}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":" about this"}}}`,
	}

	events := parseLines(lines)

	if len(events) != 2 {
		t.Fatalf("expected 2 thinking events, got %d", len(events))
	}
	if events[0].Type != "thinking" {
		t.Errorf("expected first event type 'thinking', got %q", events[0].Type)
	}
	if events[0].Content != "Let me think..." {
		t.Errorf("expected first thinking content 'Let me think...', got %q", events[0].Content)
	}
	if events[1].Content != " about this" {
		t.Errorf("expected second thinking content ' about this', got %q", events[1].Content)
	}
}

func TestStreamParser_ThinkingDedup(t *testing.T) {
	// Simulate: thinking_delta (incremental) then assistant message with thinking block (should be skipped)
	lines := []string{
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"My thought"}}}`,
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"My thought"},{"type":"text","text":"Hello"}]}}`,
	}

	events := parseLines(lines)

	// Should get: 1 thinking (from delta) + 1 content (from assistant text)
	// The assistant thinking block should be skipped due to receivedPartialThinking
	var thinkingEvents, contentEvents int
	for _, ev := range events {
		switch ev.Type {
		case "thinking":
			thinkingEvents++
		case "content":
			contentEvents++
		}
	}
	if thinkingEvents != 1 {
		t.Errorf("expected 1 thinking event, got %d", thinkingEvents)
	}
	if contentEvents != 1 {
		t.Errorf("expected 1 content event, got %d", contentEvents)
	}
}

func TestStreamParser_ClaudeThinkingBlock(t *testing.T) {
	// Claude without --include-partial-messages: thinking comes in assistant message
	msg := ClaudeStreamMessage{
		Type: "assistant",
		Message: &ClaudeStreamMessageBody{
			Content: []ClaudeContentBlock{
				{Type: "thinking", Thinking: "Deep analysis here..."},
				{Type: "text", Text: "The answer is 42."},
			},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	events := parseLine(string(data))

	// Should produce: 1 thinking + 1 content
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "thinking" {
		t.Errorf("expected first event type 'thinking', got %q", events[0].Type)
	}
	if events[0].Content != "Deep analysis here..." {
		t.Errorf("expected thinking content 'Deep analysis here...', got %q", events[0].Content)
	}
	if events[1].Type != "content" || events[1].Content != "The answer is 42." {
		t.Errorf("expected content 'The answer is 42.', got %q", events[1].Content)
	}
}

// --- New tests below ---

func TestStreamParser_TextDedup(t *testing.T) {
	// When text_delta events are received, the full assistant text should be skipped
	lines := []string{
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}]}}`,
	}

	events := parseLines(lines)

	// Should get 2 content events from deltas, the full assistant text should be skipped
	var contentText string
	var contentCount int
	for _, ev := range events {
		if ev.Type == "content" {
			contentCount++
			contentText += ev.Content
		}
	}
	if contentCount != 2 {
		t.Errorf("expected 2 content events (from deltas only), got %d", contentCount)
	}
	if contentText != "Hello world" {
		t.Errorf("expected combined content 'Hello world', got %q", contentText)
	}
}

func TestStreamParser_TextDedup_CodebuddySimpleFormat(t *testing.T) {
	// Codebuddy simple format (no Message, uses Text field) should also be skipped when receivedPartial
	lines := []string{
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Partial"}}}`,
		`{"type":"assistant","subtype":"text","text":"Full text"}`,
	}

	events := parseLines(lines)

	var contentText string
	for _, ev := range events {
		if ev.Type == "content" {
			contentText += ev.Content
		}
	}
	if contentText != "Partial" {
		t.Errorf("expected only partial content 'Partial', got %q", contentText)
	}
}

func TestStreamParser_ToolUseStartStop(t *testing.T) {
	// Test full tool_use lifecycle: content_block_start -> input_json_delta -> content_block_stop
	lines := []string{
		`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_123","name":"Read"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","text":"{\"file_"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","text":"path\":\"/src/main.go\"}"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":1}}`,
	}

	events := parseLines(lines)

	// Should get 2 tool_use events: start and stop
	// Note: both events share the same *ToolCall pointer, so after stop
	// sets Done=true, the start event's Tool also appears Done=true.
	if len(events) != 2 {
		t.Fatalf("expected 2 tool_use events, got %d", len(events))
	}

	// Both events are tool_use type
	if events[0].Type != "tool_use" || events[1].Type != "tool_use" {
		t.Errorf("expected both events to be tool_use, got %q and %q", events[0].Type, events[1].Type)
	}

	// The tool name and ID are set from content_block_start
	if events[0].Tool == nil {
		t.Fatal("expected Tool to be non-nil")
	}
	if events[0].Tool.Name != "Read" {
		t.Errorf("expected tool name 'Read', got %q", events[0].Tool.Name)
	}
	if events[0].Tool.ID != "toolu_123" {
		t.Errorf("expected tool ID 'toolu_123', got %q", events[0].Tool.ID)
	}

	// The final event (stop) should have the accumulated input and Done=true
	if !events[1].Tool.Done {
		t.Error("expected stop event Tool.Done to be true")
	}
	if events[1].Tool.Input != `{"file_path":"/src/main.go"}` {
		t.Errorf("expected accumulated input '{\"file_path\":\"/src/main.go\"}', got %q", events[1].Tool.Input)
	}
}

func TestStreamParser_InputJsonDeltaNoCurrentTool(t *testing.T) {
	// input_json_delta without a currentTool should be silently ignored
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","text":"{\"key\":\"val\"}"}}}`

	events := parseLine(line)

	if len(events) != 0 {
		t.Errorf("expected 0 events when no currentTool, got %d", len(events))
	}
}

func TestStreamParser_MessageStartModel(t *testing.T) {
	// Model name extracted from message_start should be used in result
	lines := []string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"model":"claude-3.5-sonnet"}}}`,
		`{"type":"result","session_id":"sess-1","duration_ms":1000}`,
	}

	events := parseLines(lines)

	// Find metadata event
	var metaEvent *StreamEvent
	for i := range events {
		if events[i].Type == "metadata" {
			metaEvent = &events[i]
			break
		}
	}
	if metaEvent == nil {
		t.Fatal("expected a metadata event")
	}
	if metaEvent.Meta.Model != "claude-3.5-sonnet" {
		t.Errorf("expected model 'claude-3.5-sonnet', got %q", metaEvent.Meta.Model)
	}
}

func TestStreamParser_MessageStartOverridesProviderData(t *testing.T) {
	// Model from message_start should take priority over providerData
	lines := []string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"model":"model-from-start"}}}`,
		`{"type":"result","session_id":"s1","providerData":{"model":"model-from-provider"}}`,
	}

	events := parseLines(lines)

	var metaEvent *StreamEvent
	for i := range events {
		if events[i].Type == "metadata" {
			metaEvent = &events[i]
			break
		}
	}
	if metaEvent == nil {
		t.Fatal("expected a metadata event")
	}
	if metaEvent.Meta.Model != "model-from-start" {
		t.Errorf("expected model from message_start, got %q", metaEvent.Meta.Model)
	}
}

func TestStreamParser_FileHistorySnapshotSkipped(t *testing.T) {
	line := `{"type":"file-history-snapshot","files":["a.go","b.go"]}`
	events := parseLine(line)

	if len(events) != 0 {
		t.Errorf("expected 0 events for file-history-snapshot, got %d", len(events))
	}
}

func TestStreamParser_UnparseableLine(t *testing.T) {
	events := parseLine("not json at all")
	if len(events) != 0 {
		t.Errorf("expected 0 events for unparseable line, got %d", len(events))
	}
}

func TestStreamParser_EmptyTextDelta(t *testing.T) {
	// Empty text in text_delta should be silently ignored
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":""}}}`
	events := parseLine(line)

	if len(events) != 0 {
		t.Errorf("expected 0 events for empty text_delta, got %d", len(events))
	}
}

func TestStreamParser_EmptyThinkingDelta(t *testing.T) {
	// Empty thinking in thinking_delta should be silently ignored
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":""}}}`
	events := parseLine(line)

	if len(events) != 0 {
		t.Errorf("expected 0 events for empty thinking_delta, got %d", len(events))
	}
}

func TestStreamParser_StreamEventWithNilEvent(t *testing.T) {
	// stream_event with nil Event field should be silently ignored
	line := `{"type":"stream_event"}`
	events := parseLine(line)

	if len(events) != 0 {
		t.Errorf("expected 0 events for stream_event with nil event, got %d", len(events))
	}
}

func TestStreamParser_StreamEventWithNilDelta(t *testing.T) {
	// content_block_delta with nil Delta should be silently ignored
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0}}`
	events := parseLine(line)

	if len(events) != 0 {
		t.Errorf("expected 0 events for delta with nil Delta, got %d", len(events))
	}
}

func TestStreamParser_ContentBlockStartNonToolUse(t *testing.T) {
	// content_block_start for "text" type should be silently ignored
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text"}}}`
	events := parseLine(line)

	if len(events) != 0 {
		t.Errorf("expected 0 events for text content_block_start, got %d", len(events))
	}
}

func TestStreamParser_FullCodebuddyFlow(t *testing.T) {
	// Simulate a complete Codebuddy streaming session with thinking + tool_use + text
	lines := []string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"model":"glm-4"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me read the file..."}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_001","name":"Read"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","text":"{\"file_path\":\"config.yaml\"}"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":1}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":2,"content_block":{"type":"text"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":2,"delta":{"type":"text_delta","text":"The port is "}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":2,"delta":{"type":"text_delta","text":"20000."}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":2}}`,
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Let me read the file..."},{"type":"tool_use","id":"toolu_001","name":"Read","input":{"file_path":"config.yaml"}},{"type":"text","text":"The port is 20000."}]}}`,
		`{"type":"result","session_id":"sess-1","duration_ms":3000,"total_cost_usd":0.01,"providerData":{"model":"glm-4","usage":{"inputTokens":50,"outputTokens":100}}}`,
	}

	events := parseLines(lines)

	// Expected events:
	// 1. thinking (from thinking_delta)
	// 2. tool_use start (from content_block_start)
	// 3. tool_use stop (from content_block_stop, with accumulated input)
	// 4. content "The port is " (from text_delta)
	// 5. content "20000." (from text_delta)
	// 6. tool_use (from assistant message, with full input - NOT deduplicated since assistant tool_use always emits)
	// 7. metadata
	// 8. done
	//
	// Note: assistant message thinking is skipped (receivedPartialThinking), text is skipped (receivedPartial)
	// Note: assistant tool_use IS emitted even with receivedPartial (this is by design - it carries the complete input)

	var thinkingCount, contentCount, toolUseCount, metadataCount, doneCount int
	for _, ev := range events {
		switch ev.Type {
		case "thinking":
			thinkingCount++
		case "content":
			contentCount++
		case "tool_use":
			toolUseCount++
		case "metadata":
			metadataCount++
		case "done":
			doneCount++
		}
	}

	if thinkingCount != 1 {
		t.Errorf("expected 1 thinking event, got %d", thinkingCount)
	}
	if contentCount != 2 {
		t.Errorf("expected 2 content events, got %d", contentCount)
	}
	// 2 from stream (start+stop) + 1 from assistant message = 3
	if toolUseCount != 3 {
		t.Errorf("expected 3 tool_use events, got %d", toolUseCount)
	}
	if metadataCount != 1 {
		t.Errorf("expected 1 metadata event, got %d", metadataCount)
	}
	if doneCount != 1 {
		t.Errorf("expected 1 done event, got %d", doneCount)
	}

	// Verify model from message_start
	var metaEvent *StreamEvent
	for i := range events {
		if events[i].Type == "metadata" {
			metaEvent = &events[i]
			break
		}
	}
	if metaEvent == nil || metaEvent.Meta.Model != "glm-4" {
		t.Errorf("expected model 'glm-4', got %v", metaEvent)
	}
}

func TestStreamParser_FullClaudeFlow(t *testing.T) {
	// Simulate a complete Claude streaming session (no --include-partial-messages)
	// Claude uses --verbose which outputs full assistant messages
	msg1 := ClaudeStreamMessage{
		Type: "assistant",
		Message: &ClaudeStreamMessageBody{
			Content: []ClaudeContentBlock{
				{Type: "thinking", Thinking: "Analyzing the code..."},
				{Type: "tool_use", ID: "toolu_abc", Name: "Bash", Input: json.RawMessage(`{"command":"ls -la"}`)},
			},
		},
	}
	msg2 := ClaudeStreamMessage{
		Type: "assistant",
		Message: &ClaudeStreamMessageBody{
			Content: []ClaudeContentBlock{
				{Type: "tool_use", ID: "toolu_abc", Name: "Bash", Input: json.RawMessage(`{"command":"ls -la"}`)},
				{Type: "text", Text: "Here are the files."},
			},
		},
	}
	msg3 := ClaudeStreamMessage{
		Type:      "result",
		SessionID: "sess-claude",
		DurationMs: 5000,
		Usage: &ClaudeStreamUsage{InputTokens: 200, OutputTokens: 150},
		ModelUsage: map[string]ClaudeStreamModelUsage{
			"claude-3.5-sonnet": {InputTokens: 200, OutputTokens: 150},
		},
	}

	var lines []string
	for _, msg := range []ClaudeStreamMessage{msg1, msg2, msg3} {
		data, _ := json.Marshal(msg)
		lines = append(lines, string(data))
	}

	events := parseLines(lines)

	// Expected:
	// From msg1: thinking + tool_use (Bash, done=true)
	// From msg2: tool_use (Bash, done=true) + content
	// From msg3: metadata + done
	var thinkingCount, contentCount, toolUseCount, metadataCount, doneCount int
	for _, ev := range events {
		switch ev.Type {
		case "thinking":
			thinkingCount++
		case "content":
			contentCount++
		case "tool_use":
			toolUseCount++
		case "metadata":
			metadataCount++
		case "done":
			doneCount++
		}
	}

	if thinkingCount != 1 {
		t.Errorf("expected 1 thinking event, got %d", thinkingCount)
	}
	if contentCount != 1 {
		t.Errorf("expected 1 content event, got %d", contentCount)
	}
	if toolUseCount != 2 {
		t.Errorf("expected 2 tool_use events, got %d", toolUseCount)
	}
	if metadataCount != 1 {
		t.Errorf("expected 1 metadata event, got %d", metadataCount)
	}
	if doneCount != 1 {
		t.Errorf("expected 1 done event, got %d", doneCount)
	}

	// Verify model from ModelUsage
	var metaEvent *StreamEvent
	for i := range events {
		if events[i].Type == "metadata" {
			metaEvent = &events[i]
			break
		}
	}
	if metaEvent == nil || metaEvent.Meta.Model != "claude-3.5-sonnet" {
		t.Errorf("expected model 'claude-3.5-sonnet', got %v", metaEvent)
	}
}

func TestStreamParser_ToolUseFromAssistantWithInput(t *testing.T) {
	// Tool use in assistant message should carry the full input JSON
	msg := ClaudeStreamMessage{
		Type: "assistant",
		Message: &ClaudeStreamMessageBody{
			Content: []ClaudeContentBlock{
				{Type: "tool_use", ID: "toolu_xyz", Name: "Edit", Input: json.RawMessage(`{"file_path":"/app/main.go","old_string":"hello","new_string":"world"}`)},
			},
		},
	}
	data, _ := json.Marshal(msg)

	events := parseLine(string(data))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "tool_use" {
		t.Fatalf("expected tool_use event, got %q", events[0].Type)
	}
	if events[0].Tool.Name != "Edit" {
		t.Errorf("expected tool name 'Edit', got %q", events[0].Tool.Name)
	}
	if events[0].Tool.ID != "toolu_xyz" {
		t.Errorf("expected tool ID 'toolu_xyz', got %q", events[0].Tool.ID)
	}
	if !events[0].Tool.Done {
		t.Error("expected tool to be done (from complete assistant message)")
	}
	// Input should be the raw JSON string
	expectedInput := `{"file_path":"/app/main.go","old_string":"hello","new_string":"world"}`
	if events[0].Tool.Input != expectedInput {
		t.Errorf("expected input %q, got %q", expectedInput, events[0].Tool.Input)
	}
}

func TestBuildClaudeStreamArgs_NewSession(t *testing.T) {
	args := buildClaudeStreamArgs(ChatRequest{
		Prompt:       "hello",
		SessionID:    "sess-1",
		WorkDir:      "/tmp",
		SystemPrompt: "be helpful",
		Resume:       false,
	})

	// Should use --session-id, prompt as last arg
	found := false
	for i, a := range args {
		if a == "--session-id" && i+1 < len(args) && args[i+1] == "sess-1" {
			found = true
		}
	}
	if !found {
		t.Error("expected --session-id sess-1 in args")
	}

	// Should NOT have --resume
	for _, a := range args {
		if a == "--resume" {
			t.Error("should not have --resume for new session")
		}
	}

	// Prompt should be the last argument
	if args[len(args)-1] != "hello" {
		t.Errorf("expected last arg to be prompt 'hello', got %q", args[len(args)-1])
	}
}

func TestBuildClaudeStreamArgs_ResumeSession(t *testing.T) {
	args := buildClaudeStreamArgs(ChatRequest{
		Prompt:       "follow-up question",
		SessionID:    "sess-1",
		WorkDir:      "/tmp",
		SystemPrompt: "be helpful",
		Resume:       true,
	})

	// Should use --resume
	found := false
	for i, a := range args {
		if a == "--resume" && i+1 < len(args) && args[i+1] == "sess-1" {
			found = true
		}
	}
	if !found {
		t.Error("expected --resume sess-1 in args")
	}

	// Should NOT have --session-id
	for _, a := range args {
		if a == "--session-id" {
			t.Error("should not have --session-id for resume session")
		}
	}

	// Prompt should NOT be in args (goes via stdin)
	for _, a := range args {
		if a == "follow-up question" {
			t.Error("prompt should not be in args for resume (goes via stdin)")
		}
	}
}

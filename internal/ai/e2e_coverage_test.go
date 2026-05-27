package ai

import (
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseVeCLISessionSummary tests ---

func TestParseVeCLISessionSummary_ValidJSON(t *testing.T) {
	raw := `{
		"sessionMetrics": {
			"models": {
				"gemini-2.5-pro": {
					"api": {"totalRequests": 5, "totalErrors": 1, "totalLatencyMs": 1200},
					"tokens": {"prompt": 800, "candidates": 200, "total": 1000, "cached": 50, "thoughts": 30, "tool": 10}
				}
			},
			"tools": {"totalCalls": 3, "totalSuccess": 2, "totalFail": 1, "totalDurationMs": 400},
			"files": {"totalLinesAdded": 20, "totalLinesRemoved": 5}
		}
	}`

	summary, err := parseVeCLISessionSummary([]byte(raw))
	require.NoError(t, err)
	require.NotNil(t, summary)

	models := summary.SessionMetrics.Models
	assert.Len(t, models, 1)
	m, ok := models["gemini-2.5-pro"]
	require.True(t, ok)
	assert.Equal(t, 800, m.Tokens.Prompt)
	assert.Equal(t, 200, m.Tokens.Candidates)
	assert.Equal(t, 1200, m.API.TotalLatencyMs)
}

func TestParseVeCLISessionSummary_InvalidJSON(t *testing.T) {
	_, err := parseVeCLISessionSummary([]byte("not json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse session-summary")
}

func TestParseVeCLISessionSummary_EmptyObject(t *testing.T) {
	summary, err := parseVeCLISessionSummary([]byte("{}"))
	require.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Empty(t, summary.SessionMetrics.Models)
}

func TestParseVeCLISessionSummary_MultipleModels(t *testing.T) {
	raw := `{
		"sessionMetrics": {
			"models": {
				"model-a": {
					"api": {"totalRequests": 1, "totalErrors": 0, "totalLatencyMs": 100},
					"tokens": {"prompt": 100, "candidates": 50, "total": 150, "cached": 0, "thoughts": 0, "tool": 0}
				},
				"model-b": {
					"api": {"totalRequests": 2, "totalErrors": 0, "totalLatencyMs": 200},
					"tokens": {"prompt": 200, "candidates": 100, "total": 300, "cached": 0, "thoughts": 0, "tool": 0}
				}
			},
			"tools": {"totalCalls": 0},
			"files": {}
		}
	}`

	summary, err := parseVeCLISessionSummary([]byte(raw))
	require.NoError(t, err)
	assert.Len(t, summary.SessionMetrics.Models, 2)
}

// --- VeCLISessionSummary.extractMetadata tests ---

func TestVeCLISessionSummary_ExtractMetadata_MatchingModel(t *testing.T) {
	raw := `{
		"sessionMetrics": {
			"models": {
				"gemini-2.5-pro": {
					"api": {"totalRequests": 1, "totalErrors": 0, "totalLatencyMs": 500},
					"tokens": {"prompt": 300, "candidates": 150, "total": 450, "cached": 0, "thoughts": 0, "tool": 0}
				},
				"other-model": {
					"api": {"totalRequests": 2, "totalErrors": 0, "totalLatencyMs": 800},
					"tokens": {"prompt": 500, "candidates": 250, "total": 750, "cached": 0, "thoughts": 0, "tool": 0}
				}
			},
			"tools": {"totalCalls": 0},
			"files": {}
		}
	}`

	var summary VeCLISessionSummary
	require.NoError(t, json.Unmarshal([]byte(raw), &summary))

	meta := summary.extractMetadata("gemini-2.5-pro")
	assert.Equal(t, "gemini-2.5-pro", meta.Model)
	assert.Equal(t, 300, meta.InputTokens)
	assert.Equal(t, 150, meta.OutputTokens)
	assert.Equal(t, 500, meta.DurationMs)
}

func TestVeCLISessionSummary_ExtractMetadata_FallbackToFirst(t *testing.T) {
	raw := `{
		"sessionMetrics": {
			"models": {
				"some-model": {
					"api": {"totalRequests": 1, "totalErrors": 0, "totalLatencyMs": 300},
					"tokens": {"prompt": 100, "candidates": 50, "total": 150, "cached": 0, "thoughts": 0, "tool": 0}
				}
			},
			"tools": {"totalCalls": 0},
			"files": {}
		}
	}`

	var summary VeCLISessionSummary
	require.NoError(t, json.Unmarshal([]byte(raw), &summary))

	// Request model doesn't match any entry — should fall back to first
	meta := summary.extractMetadata("nonexistent-model")
	assert.Equal(t, "some-model", meta.Model, "should fall back to first model entry")
	assert.Equal(t, 100, meta.InputTokens)
}

func TestVeCLISessionSummary_ExtractMetadata_EmptyModels(t *testing.T) {
	var summary VeCLISessionSummary
	require.NoError(t, json.Unmarshal([]byte(`{"sessionMetrics":{"models":{},"tools":{},"files":{}}}`), &summary))

	meta := summary.extractMetadata("fallback-model")
	assert.Equal(t, "fallback-model", meta.Model, "should use request model when no model entries exist")
	assert.Equal(t, 0, meta.InputTokens)
	assert.Equal(t, 0, meta.OutputTokens)
}

func TestVeCLISessionSummary_ExtractMetadata_NoModelsNoReqModel(t *testing.T) {
	var summary VeCLISessionSummary
	require.NoError(t, json.Unmarshal([]byte(`{"sessionMetrics":{"models":{},"tools":{},"files":{}}}`), &summary))

	meta := summary.extractMetadata("")
	assert.Equal(t, "", meta.Model, "should be empty when no models and no request model")
}

// --- VeCLIBackend.vecliPreStart tests ---

func TestVeCLIBackend_vecliPreStart(t *testing.T) {
	b := NewVeCLIBackend()
	cmd := &exec.Cmd{Args: []string{"vecli"}}
	req := ChatRequest{SessionID: "test-session-123"}

	b.vecliPreStart(cmd, req)

	// Should have appended --session-summary flag
	assert.Contains(t, cmd.Args, "--session-summary")
	idx := indexOfArg(cmd.Args, "--session-summary")
	require.GreaterOrEqual(t, idx, 0)
	assert.Contains(t, cmd.Args[idx+1], "test-session-123.json", "summary filename should include session ID")

	// Should have stored the path in summaryMap
	v, ok := b.summaryMap.Load("test-session-123")
	assert.True(t, ok, "summaryMap should contain session ID")
	assert.Contains(t, v.(string), "test-session-123.json")

	// Clean up
	b.summaryMap.LoadAndDelete("test-session-123")
}

// --- VeCLIBackend.Name test ---

func TestVeCLIBackend_Name(t *testing.T) {
	b := NewVeCLIBackend()
	assert.Equal(t, "vecli", b.Name())
}

// --- forwardEvent tests ---

func TestForwardEvent_ChannelFull(t *testing.T) {
	// Create a channel with buffer size 1 and fill it
	ch := make(chan StreamEvent, 1)
	ch <- StreamEvent{Type: "content", Content: "first"}

	// ForwardEvent should not block when channel is full
	// (it drops the event and logs a warning)
	forwardEvent(ch, StreamEvent{Type: "content", Content: "dropped"})

	// Only the first event should be in the channel
	assert.Equal(t, 1, len(ch))
	ev := <-ch
	assert.Equal(t, "first", ev.Content)
}

func TestForwardEvent_ChannelEmpty(t *testing.T) {
	ch := make(chan StreamEvent, 1)
	forwardEvent(ch, StreamEvent{Type: "content", Content: "hello"})

	assert.Equal(t, 1, len(ch))
	ev := <-ch
	assert.Equal(t, "hello", ev.Content)
}

// --- extractContentText tests ---

func TestExtractContentText_Empty(t *testing.T) {
	assert.Equal(t, "", extractContentText(nil))
	assert.Equal(t, "", extractContentText(json.RawMessage("")))
}

func TestExtractContentText_PlainString(t *testing.T) {
	result := extractContentText(json.RawMessage(`"hello world"`))
	assert.Equal(t, "hello world", result)
}

func TestExtractContentText_ArrayOfBlocks(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"line1"},{"type":"text","text":"line2"}]`)
	result := extractContentText(raw)
	assert.Equal(t, "line1\nline2", result)
}

func TestExtractContentText_ArrayWithNonTextBlocks(t *testing.T) {
	raw := json.RawMessage(`[{"type":"image","url":"x"},{"type":"text","text":"only-text"}]`)
	result := extractContentText(raw)
	// Non-text blocks are skipped but increment the index, so text at index 1 gets "\n" prefix
	assert.Equal(t, "\nonly-text", result)
}

func TestExtractContentText_SingleTextBlock(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"single"}]`)
	result := extractContentText(raw)
	assert.Equal(t, "single", result)
}

func TestExtractContentText_Fallback(t *testing.T) {
	// Invalid JSON — neither string nor array — falls back to raw string
	raw := json.RawMessage(`{invalid}`)
	result := extractContentText(raw)
	assert.Equal(t, "{invalid}", result)
}

func TestExtractContentText_ArrayWithEmptyText(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":""}]`)
	result := extractContentText(raw)
	assert.Equal(t, "", result)
}

// --- StreamParser additional tests ---

func TestStreamParser_GetCapturedSessionID(t *testing.T) {
	p := &StreamParser{}
	assert.Equal(t, "", p.GetCapturedSessionID())
}

func TestStreamParser_ParseLine_InvalidJSON(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	p.ParseLine("not json", ch)
	close(ch)
	// Should skip unparseable lines — no events emitted
	assert.Empty(t, collectEvents(ch))
}

func TestStreamParser_ParseLine_SystemMessage(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	p.ParseLine(`{"type":"system","subtype":"init"}`, ch)
	close(ch)
	assert.Empty(t, collectEvents(ch), "system messages should be skipped")
}

func TestStreamParser_ParseLine_UnknownType(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	p.ParseLine(`{"type":"custom_type","text":"hello"}`, ch)
	close(ch)
	assert.Empty(t, collectEvents(ch), "unknown types should be skipped")
}

func TestStreamParser_ParseLine_ResultWithModelUsage(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"result","session_id":"s1","duration_ms":100,"modelUsage":{"claude-3": {"inputTokens":50,"outputTokens":25}},"stop_reason":"end_turn"}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	// Should have metadata + done
	require.Len(t, events, 2)
	assert.Equal(t, "metadata", events[0].Type)
	assert.Equal(t, "claude-3", events[0].Meta.Model)
	assert.Equal(t, 50, events[0].Meta.InputTokens)
	assert.Equal(t, 25, events[0].Meta.OutputTokens)
	assert.Equal(t, "done", events[1].Type)
}

func TestStreamParser_ParseLine_ResultWithProviderData(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"result","session_id":"s2","duration_ms":200,"providerData":{"model":"gpt-4o","usage":{"inputTokens":80,"outputTokens":40}},"stop_reason":"end_turn"}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 2)
	assert.Equal(t, "metadata", events[0].Type)
	assert.Equal(t, "gpt-4o", events[0].Meta.Model)
	assert.Equal(t, 80, events[0].Meta.InputTokens)
	assert.Equal(t, 40, events[0].Meta.OutputTokens)
}

func TestStreamParser_ParseLine_ResultWithError(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"result","is_error":true,"result":"something went wrong","session_id":"s3","duration_ms":0,"stop_reason":"error"}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	// Should have warning + metadata + done
	require.GreaterOrEqual(t, len(events), 2, "should have at least metadata + done")
	// Find warning event
	var hasWarning bool
	for _, ev := range events {
		if ev.Type == "warning" {
			hasWarning = true
			assert.Equal(t, "something went wrong", ev.Content)
		}
	}
	assert.True(t, hasWarning, "error result should produce a warning event")
}

func TestStreamParser_ParseLine_ResultWithErrorsArray(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"result","is_error":true,"result":"","errors":["error1","error2"],"session_id":"s4","duration_ms":0}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	var hasWarning bool
	for _, ev := range events {
		if ev.Type == "warning" {
			hasWarning = true
			assert.Contains(t, ev.Content, "error1")
			assert.Contains(t, ev.Content, "error2")
		}
	}
	assert.True(t, hasWarning, "errors array should produce a warning event")
}

func TestStreamParser_ParseLine_StreamEventTextDelta(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "hello", events[0].Content)
	assert.True(t, p.receivedPartial, "receivedPartial should be set")
}

func TestStreamParser_ParseLine_StreamEventThinkingDelta(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hmm..."}}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "thinking", events[0].Type)
	assert.Equal(t, "hmm...", events[0].Content)
	assert.True(t, p.receivedPartialThinking, "receivedPartialThinking should be set")
}

func TestStreamParser_ParseLine_StreamEventToolUseStart(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","name":"Read","id":"tool-1"}}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "tool_use", events[0].Type)
	assert.Equal(t, "Read", events[0].Tool.Name)
	assert.Equal(t, "tool-1", events[0].Tool.ID)
	assert.False(t, events[0].Tool.Done, "initial tool_use should not be Done")
	assert.True(t, p.receivedPartialToolUse)
}

func TestStreamParser_ParseLine_StreamEventToolUseWithInput(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","name":"Bash","id":"tool-2","input":{"command":"ls"}}}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "Bash", events[0].Tool.Name)
	assert.Equal(t, `{"command":"ls"}`, events[0].Tool.Input)
}

func TestStreamParser_ParseLine_StreamEventToolUseEmptyInput(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","name":"Read","id":"tool-empty","input":{}}}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "", events[0].Tool.Input, "empty '{}' input should not be set")
	assert.True(t, p.emittedToolInputEmpty["tool-empty"], "should track empty input tool ID")
}

func TestStreamParser_ParseLine_StreamEventInputJsonDelta(t *testing.T) {
	p := &StreamParser{}
	p.activeTools = map[int]*ToolCall{
		0: {Name: "Bash", ID: "tool-delta"},
	}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"command\":"}}}`
	p.ParseLine(line, ch)
	close(ch)

	// input_json_delta accumulates into activeTools, no events emitted
	events := collectEvents(ch)
	assert.Empty(t, events)
	assert.Equal(t, `{"command":`, p.activeTools[0].Input)
}

func TestStreamParser_ParseLine_StreamEventContentBlockStop(t *testing.T) {
	p := &StreamParser{}
	p.activeTools = map[int]*ToolCall{
		0: {Name: "Bash", ID: "tool-stop", Input: `{"command":"ls"}`},
	}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "tool_use", events[0].Type)
	assert.True(t, events[0].Tool.Done, "content_block_stop should mark tool as Done")
	assert.Equal(t, "Bash", events[0].Tool.Name)
	// activeTools entry should be cleaned up
	_, exists := p.activeTools[0]
	assert.False(t, exists, "activeTools entry should be deleted after content_block_stop")
}

func TestStreamParser_ParseLine_StreamEventContentBlockStopUnknownIndex(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_stop","index":99}}`
	p.ParseLine(line, ch)
	close(ch)

	// Should not panic or emit events for unknown index
	events := collectEvents(ch)
	assert.Empty(t, events)
}

func TestStreamParser_ParseLine_StreamEventMessageStart(t *testing.T) {
	p := &StreamParser{}
	p.receivedPartial = true
	p.receivedPartialThinking = true
	p.receivedPartialToolUse = true

	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"message_start","message":{"model":"claude-3.5-sonnet"}}}`
	p.ParseLine(line, ch)
	close(ch)

	assert.Equal(t, "claude-3.5-sonnet", p.model)
	assert.False(t, p.receivedPartial, "message_start should reset receivedPartial")
	assert.False(t, p.receivedPartialThinking, "message_start should reset receivedPartialThinking")
	assert.False(t, p.receivedPartialToolUse, "message_start should reset receivedPartialToolUse")
}

func TestStreamParser_ParseLine_StreamEventMessageDeltaAndStop(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)

	p.ParseLine(`{"type":"stream_event","event":{"type":"message_delta"}}`, ch)
	p.ParseLine(`{"type":"stream_event","event":{"type":"message_stop"}}`, ch)
	close(ch)

	assert.Empty(t, collectEvents(ch), "message_delta and message_stop should not emit events")
}

func TestStreamParser_ParseLine_StreamEventNilEvent(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event"}`
	p.ParseLine(line, ch)
	close(ch)
	assert.Empty(t, collectEvents(ch), "stream_event with nil event should be skipped")
}

func TestStreamParser_ParseLine_StreamEventNilDelta(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_delta"}}`
	p.ParseLine(line, ch)
	close(ch)
	assert.Empty(t, collectEvents(ch), "content_block_delta with nil delta should be skipped")
}

func TestStreamParser_ParseLine_StreamEventTextDeltaEmpty(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":""}}}`
	p.ParseLine(line, ch)
	close(ch)
	assert.Empty(t, collectEvents(ch), "empty text_delta should not emit event")
	assert.False(t, p.receivedPartial, "empty text_delta should not set receivedPartial")
}

func TestStreamParser_ParseLine_StreamEventToolResultStart(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_result","tool_use_id":"tu-1","is_error":false}}}`
	p.ParseLine(line, ch)
	close(ch)

	// Should not emit event, just track the tool_result block
	events := collectEvents(ch)
	assert.Empty(t, events)
	assert.NotNil(t, p.activeToolResults)
	assert.Contains(t, p.activeToolResults, 1)
	assert.Equal(t, "tu-1", p.activeToolResults[1].ToolUseID)
	assert.False(t, p.activeToolResults[1].IsError)
}

func TestStreamParser_ParseLine_StreamEventToolResultWithContent(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_result","tool_use_id":"tu-content","content":"file contents here","is_error":false}}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	assert.Empty(t, events)
	assert.NotNil(t, p.activeToolResults)
	assert.Contains(t, p.activeToolResults, 1)
	assert.Equal(t, "file contents here", p.activeToolResults[1].Output.String())
}

func TestStreamParser_ParseLine_StreamEventToolResultIDFallback(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	// tool_use_id is empty, should fall back to ID field
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_result","id":"fallback-id","tool_use_id":"","is_error":false}}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	assert.Empty(t, events)
	assert.Equal(t, "fallback-id", p.activeToolResults[0].ToolUseID)
}

func TestStreamParser_ParseLine_StreamEventToolResultAccumulation(t *testing.T) {
	p := &StreamParser{}
	// First: start tool_result block
	ch1 := make(chan StreamEvent, 64)
	line1 := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_result","tool_use_id":"tr-1","is_error":false}}}`
	p.ParseLine(line1, ch1)
	close(ch1)
	collectEvents(ch1)

	// Second: text_delta for tool_result should be accumulated, not emitted as content
	ch2 := make(chan StreamEvent, 64)
	line2 := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"accumulated output"}}}`
	p.ParseLine(line2, ch2)
	close(ch2)
	events := collectEvents(ch2)
	assert.Empty(t, events, "text_delta in tool_result should not emit content event")
	assert.Equal(t, "accumulated output", p.activeToolResults[0].Output.String())

	// Third: content_block_stop finalizes the tool_result
	ch3 := make(chan StreamEvent, 64)
	line3 := `{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`
	p.ParseLine(line3, ch3)
	close(ch3)

	events = collectEvents(ch3)
	require.Len(t, events, 1)
	assert.Equal(t, "tool_result", events[0].Type)
	assert.Equal(t, "tr-1", events[0].Tool.ID)
	assert.Equal(t, "accumulated output", events[0].Tool.Output)
	assert.Equal(t, "success", events[0].Tool.Status)
}

func TestStreamParser_ParseLine_StreamEventToolResultError(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line1 := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_result","tool_use_id":"tr-err","is_error":true}}}`
	p.ParseLine(line1, ch)
	close(ch)
	collectEvents(ch)

	ch2 := make(chan StreamEvent, 64)
	line2 := `{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`
	p.ParseLine(line2, ch2)
	close(ch2)

	events := collectEvents(ch2)
	require.Len(t, events, 1)
	assert.Equal(t, "error", events[0].Tool.Status)
}

func TestStreamParser_ParseLine_AssistantToolResultBlock(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_result","tool_use_id":"tu-assistant","content":"result text","is_error":false}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "tool_result", events[0].Type)
	assert.Equal(t, "tu-assistant", events[0].Tool.ID)
	assert.Equal(t, "result text", events[0].Tool.Output)
	assert.Equal(t, "success", events[0].Tool.Status)
}

func TestStreamParser_ParseLine_AssistantToolResultError(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_result","tool_use_id":"tu-err","is_error":true,"content":"failed"}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "error", events[0].Tool.Status)
}

func TestStreamParser_ParseLine_AssistantToolResultIDFallback(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_result","id":"fallback-id","tool_use_id":"","content":"ok"}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "fallback-id", events[0].Tool.ID)
}

func TestStreamParser_ParseLine_UserToolResult(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu-user","content":"user output","is_error":false}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "tool_result", events[0].Type)
	assert.Equal(t, "tu-user", events[0].Tool.ID)
	assert.Equal(t, "user output", events[0].Tool.Output)
	assert.Equal(t, "success", events[0].Tool.Status)
}

func TestStreamParser_ParseLine_UserToolResultError(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu-user-err","is_error":true,"content":"failed"}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "error", events[0].Tool.Status)
}

func TestStreamParser_ParseLine_AssistantToolResultWithTextContent(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	// tool_result with array content (not plain string)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_result","tool_use_id":"tu-arr","content":[{"type":"text","text":"line1"},{"type":"text","text":"line2"}],"is_error":false}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "tool_result", events[0].Type)
	assert.Contains(t, events[0].Tool.Output, "line1")
}

func TestStreamParser_ParseLine_AssistantToolResultWithTextFallback(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	// tool_result with empty content but non-empty Text field
	line := `{"type":"assistant","message":{"content":[{"type":"tool_result","tool_use_id":"tu-text","content":"","text":"fallback text","is_error":false}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "fallback text", events[0].Tool.Output)
}

func TestStreamParser_ParseLine_CodebuddyText(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","subtype":"text","text":"hello from codebuddy"}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "hello from codebuddy", events[0].Content)
}

func TestStreamParser_ParseLine_CodebuddyTextAfterPartial(t *testing.T) {
	p := &StreamParser{}
	p.receivedPartial = true
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","subtype":"text","text":"duplicate"}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	assert.Empty(t, events, "codebuddy text after partial should be skipped")
}

func TestStreamParser_ParseLine_AssistantThinkingBlock(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"I should think about this..."}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "thinking", events[0].Type)
	assert.Equal(t, "I should think about this...", events[0].Content)
}

func TestStreamParser_ParseLine_AssistantThinkingBlockAfterPartial(t *testing.T) {
	p := &StreamParser{}
	p.receivedPartialThinking = true
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"duplicate"}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	assert.Empty(t, events, "thinking after partial thinking should be skipped")
}

func TestStreamParser_ParseLine_AssistantTextBlock(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"main response"}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "main response", events[0].Content)
}

func TestStreamParser_ParseLine_AssistantTextBlockAfterPartial(t *testing.T) {
	p := &StreamParser{}
	p.receivedPartial = true
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"duplicate"}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	assert.Empty(t, events, "text after partial should be skipped")
}

func TestStreamParser_ParseLine_AssistantToolUseBlock(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","id":"tu-1","input":{"command":"ls"}}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "tool_use", events[0].Type)
	assert.Equal(t, "Bash", events[0].Tool.Name)
	assert.Equal(t, "tu-1", events[0].Tool.ID)
	assert.Equal(t, `{"command":"ls"}`, events[0].Tool.Input)
	assert.True(t, events[0].Tool.Done)
}

func TestStreamParser_ParseLine_AssistantToolUseAfterPartialWithEmptyInput(t *testing.T) {
	p := &StreamParser{}
	p.receivedPartialToolUse = true
	p.emittedToolInputEmpty = map[string]bool{"tu-empty": true}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","id":"tu-empty","input":{"command":"ls -la"}}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 1)
	assert.Equal(t, "tool_use", events[0].Type)
	assert.Equal(t, `{"command":"ls -la"}`, events[0].Tool.Input, "should supplement empty input from assistant message")
}

func TestStreamParser_ParseLine_AssistantToolUseAfterPartialWithEmptySupplement(t *testing.T) {
	p := &StreamParser{}
	p.receivedPartialToolUse = true
	p.emittedToolInputEmpty = map[string]bool{"tu-skip": true}
	ch := make(chan StreamEvent, 64)
	// Input is empty object {} — string(block.Input) == "{}", should not supplement
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","id":"tu-skip","input":{}}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	assert.Empty(t, events, "empty '{}' input supplement should not emit event")
}

func TestStreamParser_ParseLine_AssistantToolUseAfterPartialNotTracked(t *testing.T) {
	p := &StreamParser{}
	p.receivedPartialToolUse = true
	ch := make(chan StreamEvent, 64)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","id":"not-tracked","input":{"file":"x"}}]}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	assert.Empty(t, events, "tool_use after partial (not tracked empty input) should be skipped")
}

func TestStreamParser_ParseLine_StreamEventToolUseReuseIndex(t *testing.T) {
	p := &StreamParser{}
	p.activeTools = map[int]*ToolCall{
		0: {Name: "OldTool", ID: "old-1", Input: "{}", Done: false},
	}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","name":"NewTool","id":"new-1"}}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	// Should auto-close old tool and emit new tool
	require.Len(t, events, 2)
	assert.Equal(t, "tool_use", events[0].Type)
	assert.Equal(t, "OldTool", events[0].Tool.Name)
	assert.True(t, events[0].Tool.Done, "old tool should be auto-closed")
	assert.Equal(t, "tool_use", events[1].Type)
	assert.Equal(t, "NewTool", events[1].Tool.Name)
}

func TestStreamParser_ParseLine_StreamEventToolUseContentBlockNil(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":0}}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	assert.Empty(t, events, "content_block_start with nil content_block should be skipped")
}

func TestStreamParser_ParseLine_FileHistorySnapshot(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"file-history-snapshot","files":["a.go","b.go"]}`
	p.ParseLine(line, ch)
	close(ch)
	assert.Empty(t, collectEvents(ch))
}

func TestStreamParser_ParseLine_ResultWithCapturedModel(t *testing.T) {
	p := &StreamParser{}
	p.model = "my-model"
	ch := make(chan StreamEvent, 64)
	line := `{"type":"result","session_id":"s5","duration_ms":100,"stop_reason":"end_turn"}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 2)
	assert.Equal(t, "my-model", events[0].Meta.Model, "should use model from message_start")
}

func TestStreamParser_ParseLine_ResultWithUsage(t *testing.T) {
	p := &StreamParser{}
	ch := make(chan StreamEvent, 64)
	line := `{"type":"result","session_id":"s6","duration_ms":50,"usage":{"input_tokens":100,"output_tokens":50},"stop_reason":"end_turn"}`
	p.ParseLine(line, ch)
	close(ch)

	events := collectEvents(ch)
	require.Len(t, events, 2)
	assert.Equal(t, "metadata", events[0].Type)
	assert.Equal(t, 100, events[0].Meta.InputTokens)
	assert.Equal(t, 50, events[0].Meta.OutputTokens)
}

// --- collectEvents helper ---

func collectEvents(ch chan StreamEvent) []StreamEvent {
	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

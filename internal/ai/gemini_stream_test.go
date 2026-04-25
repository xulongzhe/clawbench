package ai

import (
	"encoding/json"
	"testing"
)

func parseGeminiLine(line string) []StreamEvent {
	ch := make(chan StreamEvent, 64)
	parser := &GeminiStreamParser{}
	parser.ParseLine(line, ch)
	close(ch)
	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestGeminiStream_ParseLine_Init(t *testing.T) {
	line := `{"type":"init","timestamp":"2026-04-25T10:00:00.000Z","session_id":"ses_abc123","model":"gemini-3-pro-preview"}`
	events := parseGeminiLine(line)

	// Init events don't emit stream events, they just capture session/model
	if len(events) != 0 {
		t.Fatalf("expected 0 events for init, got %d", len(events))
	}
}

func TestGeminiStream_ParseLine_AssistantMessage(t *testing.T) {
	line := `{"type":"message","timestamp":"2026-04-25T10:00:01.000Z","role":"assistant","content":"Hello, world!","delta":true}`
	events := parseGeminiLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "content" {
		t.Errorf("expected content event, got %s", events[0].Type)
	}
	if events[0].Content != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", events[0].Content)
	}
}

func TestGeminiStream_ParseLine_UserMessage(t *testing.T) {
	line := `{"type":"message","timestamp":"2026-04-25T10:00:00.000Z","role":"user","content":"Say hello"}`
	events := parseGeminiLine(line)

	// User messages should be skipped (they echo back the input)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for user message, got %d", len(events))
	}
}

func TestGeminiStream_ParseLine_AssistantEmpty(t *testing.T) {
	line := `{"type":"message","timestamp":"2026-04-25T10:00:01.000Z","role":"assistant","content":"","delta":true}`
	events := parseGeminiLine(line)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for empty assistant message, got %d", len(events))
	}
}

func TestGeminiStream_ParseLine_ToolUse(t *testing.T) {
	line := `{"type":"tool_use","timestamp":"2026-04-25T10:00:02.000Z","tool_name":"read_file","tool_id":"call_123","parameters":{"filePath":"/tmp/test.go"}}`
	events := parseGeminiLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "tool_use" {
		t.Errorf("expected tool_use event, got %s", events[0].Type)
	}
	tool := events[0].Tool
	if tool == nil {
		t.Fatal("expected tool call, got nil")
	}
	if tool.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", tool.Name)
	}
	if tool.ID != "call_123" {
		t.Errorf("expected call ID 'call_123', got %q", tool.ID)
	}
	if !tool.Done {
		t.Error("expected Done=true for Gemini tool_use (full input in one event)")
	}
	// Verify input is preserved as JSON
	var input map[string]any
	if err := json.Unmarshal([]byte(tool.Input), &input); err != nil {
		t.Fatalf("failed to parse tool input: %v", err)
	}
	if input["filePath"] != "/tmp/test.go" {
		t.Errorf("unexpected input: %v", input)
	}
}

func TestGeminiStream_ParseLine_ToolUseEmptyParams(t *testing.T) {
	line := `{"type":"tool_use","timestamp":"2026-04-25T10:00:02.000Z","tool_name":"list_files","tool_id":"call_456"}`
	events := parseGeminiLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	tool := events[0].Tool
	if tool.Input != "{}" {
		t.Errorf("expected empty object for missing parameters, got %q", tool.Input)
	}
}

func TestGeminiStream_ParseLine_ToolResult(t *testing.T) {
	line := `{"type":"tool_result","timestamp":"2026-04-25T10:00:03.000Z","tool_id":"call_123","status":"success","output":"file content here"}`
	events := parseGeminiLine(line)

	// Tool results are informational — no stream event emitted
	if len(events) != 0 {
		t.Fatalf("expected 0 events for tool_result, got %d", len(events))
	}
}

func TestGeminiStream_ParseLine_ErrorWarning(t *testing.T) {
	line := `{"type":"error","timestamp":"2026-04-25T10:00:03.000Z","severity":"warning","message":"Loop detected, stopping execution"}`
	events := parseGeminiLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "warning" {
		t.Errorf("expected warning event, got %s", events[0].Type)
	}
	if events[0].Content != "Loop detected, stopping execution" {
		t.Errorf("unexpected content: %q", events[0].Content)
	}
}

func TestGeminiStream_ParseLine_ErrorError(t *testing.T) {
	line := `{"type":"error","timestamp":"2026-04-25T10:00:03.000Z","severity":"error","message":"Maximum session turns exceeded"}`
	events := parseGeminiLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "error" {
		t.Errorf("expected error event, got %s", events[0].Type)
	}
	if events[0].Error != "Maximum session turns exceeded" {
		t.Errorf("unexpected error: %q", events[0].Error)
	}
}

func TestGeminiStream_ParseLine_ResultSuccess(t *testing.T) {
	line := `{"type":"result","timestamp":"2026-04-25T10:00:05.000Z","status":"success","stats":{"total_tokens":500,"input_tokens":400,"output_tokens":100,"cached":0,"input":400,"duration_ms":3000,"tool_calls":2,"models":{"gemini-3-pro-preview":{"total_tokens":500,"input_tokens":400,"output_tokens":100,"cached":0,"input":400}}}}`
	events := parseGeminiLine(line)

	if len(events) != 2 {
		t.Fatalf("expected 2 events (metadata + done), got %d", len(events))
	}
	if events[0].Type != "metadata" {
		t.Errorf("expected metadata event first, got %s", events[0].Type)
	}
	meta := events[0].Meta
	if meta == nil {
		t.Fatal("expected metadata, got nil")
	}
	if meta.InputTokens != 400 {
		t.Errorf("expected input tokens 400, got %d", meta.InputTokens)
	}
	if meta.OutputTokens != 100 {
		t.Errorf("expected output tokens 100, got %d", meta.OutputTokens)
	}
	if meta.DurationMs != 3000 {
		t.Errorf("expected duration 3000ms, got %d", meta.DurationMs)
	}
	if meta.StopReason != "stop" {
		t.Errorf("expected stopReason 'stop', got %q", meta.StopReason)
	}
	if meta.IsError {
		t.Error("expected IsError=false for success result")
	}
	if events[1].Type != "done" {
		t.Errorf("expected done event second, got %s", events[1].Type)
	}
}

func TestGeminiStream_ParseLine_ResultError(t *testing.T) {
	line := `{"type":"result","timestamp":"2026-04-25T10:00:05.000Z","status":"error","error":{"type":"FatalAuthenticationError","message":"Authentication failed"},"stats":{"total_tokens":0,"input_tokens":0,"output_tokens":0,"cached":0,"input":0,"duration_ms":0,"tool_calls":0,"models":{}}}`
	events := parseGeminiLine(line)

	// Result with error: warning event + metadata + done
	if len(events) != 3 {
		t.Fatalf("expected 3 events (warning + metadata + done), got %d", len(events))
	}
	if events[0].Type != "warning" {
		t.Errorf("expected warning event first, got %s", events[0].Type)
	}
	if events[0].Content != "Authentication failed" {
		t.Errorf("unexpected warning content: %q", events[0].Content)
	}
	if events[1].Type != "metadata" {
		t.Errorf("expected metadata event second, got %s", events[1].Type)
	}
	if !events[1].Meta.IsError {
		t.Error("expected IsError=true for error result")
	}
}

func TestGeminiStream_ParseLine_UnparseableLine(t *testing.T) {
	events := parseGeminiLine("not json at all")
	if len(events) != 0 {
		t.Fatalf("expected 0 events for unparseable line, got %d", len(events))
	}
}

func TestGeminiStream_ParseLine_UnknownType(t *testing.T) {
	line := `{"type":"custom_event","timestamp":"2026-04-25T10:00:00.000Z"}`
	events := parseGeminiLine(line)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for unknown type, got %d", len(events))
	}
}

func TestGeminiStream_SessionIDCapture(t *testing.T) {
	parser := &GeminiStreamParser{}
	ch := make(chan StreamEvent, 64)

	// Init captures session ID and model
	line1 := `{"type":"init","timestamp":"2026-04-25T10:00:00.000Z","session_id":"ses_captured123","model":"gemini-3-pro-preview"}`
	parser.ParseLine(line1, ch)

	// Result uses the captured session ID in metadata
	line2 := `{"type":"result","timestamp":"2026-04-25T10:00:05.000Z","status":"success","stats":{"total_tokens":500,"input_tokens":400,"output_tokens":100,"cached":0,"input":400,"duration_ms":3000,"tool_calls":0,"models":{}}}`
	parser.ParseLine(line2, ch)
	close(ch)

	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Meta.SessionID != "ses_captured123" {
		t.Errorf("expected sessionID 'ses_captured123', got %q", events[0].Meta.SessionID)
	}
	if events[0].Meta.Model != "gemini-3-pro-preview" {
		t.Errorf("expected model 'gemini-3-pro-preview', got %q", events[0].Meta.Model)
	}
}

func TestGeminiStream_FullFlow(t *testing.T) {
	lines := []string{
		`{"type":"init","timestamp":"2026-04-25T10:00:00.000Z","session_id":"ses_full_flow","model":"gemini-3-pro-preview"}`,
		`{"type":"message","timestamp":"2026-04-25T10:00:00.500Z","role":"user","content":"Read the main.go file"}`,
		`{"type":"message","timestamp":"2026-04-25T10:00:01.000Z","role":"assistant","content":"I'll read","delta":true}`,
		`{"type":"message","timestamp":"2026-04-25T10:00:01.500Z","role":"assistant","content":" that file for you.","delta":true}`,
		`{"type":"tool_use","timestamp":"2026-04-25T10:00:02.000Z","tool_name":"read_file","tool_id":"call_001","parameters":{"filePath":"main.go"}}`,
		`{"type":"tool_result","timestamp":"2026-04-25T10:00:03.000Z","tool_id":"call_001","status":"success","output":"package main\n\nfunc main() {}"}`,
		`{"type":"message","timestamp":"2026-04-25T10:00:04.000Z","role":"assistant","content":"The file contains a simple main package.","delta":true}`,
		`{"type":"result","timestamp":"2026-04-25T10:00:05.000Z","status":"success","stats":{"total_tokens":1000,"input_tokens":800,"output_tokens":200,"cached":0,"input":800,"duration_ms":5000,"tool_calls":1,"models":{"gemini-3-pro-preview":{"total_tokens":1000,"input_tokens":800,"output_tokens":200,"cached":0,"input":800}}}}`,
	}

	ch := make(chan StreamEvent, 64)
	parser := &GeminiStreamParser{}
	for _, line := range lines {
		parser.ParseLine(line, ch)
	}
	close(ch)

	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Expected: content("I'll read"), content(" that file for you."), tool_use, content("The file..."), metadata, done
	if len(events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(events))
	}

	// Event 0: content
	if events[0].Type != "content" || events[0].Content != "I'll read" {
		t.Errorf("event 0: unexpected, got type=%s content=%q", events[0].Type, events[0].Content)
	}
	// Event 1: content
	if events[1].Type != "content" || events[1].Content != " that file for you." {
		t.Errorf("event 1: unexpected, got type=%s content=%q", events[1].Type, events[1].Content)
	}
	// Event 2: tool_use
	if events[2].Type != "tool_use" {
		t.Errorf("event 2: expected tool_use, got %s", events[2].Type)
	}
	if events[2].Tool.Name != "read_file" {
		t.Errorf("event 2: expected tool 'read_file', got %q", events[2].Tool.Name)
	}
	// Event 3: content
	if events[3].Type != "content" || events[3].Content != "The file contains a simple main package." {
		t.Errorf("event 3: unexpected, got type=%s content=%q", events[3].Type, events[3].Content)
	}
	// Event 4: metadata
	if events[4].Type != "metadata" {
		t.Errorf("event 4: expected metadata, got %s", events[4].Type)
	}
	if events[4].Meta.SessionID != "ses_full_flow" {
		t.Errorf("event 4: expected sessionID 'ses_full_flow', got %q", events[4].Meta.SessionID)
	}
	// Event 5: done
	if events[5].Type != "done" {
		t.Errorf("event 5: expected done, got %s", events[5].Type)
	}
}

func TestBuildGeminiStreamArgs_Basic(t *testing.T) {
	req := ChatRequest{
		Prompt:  "say hello",
		WorkDir: "/home/user/project",
		Model:   "gemini-3-pro-preview",
	}
	args := buildGeminiStreamArgs(req)

	expected := []string{"--prompt", "say hello", "--output-format", "stream-json", "--yolo", "--include-directories", "/home/user/project", "--model", "gemini-3-pro-preview"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, v := range expected {
		if args[i] != v {
			t.Errorf("arg %d: expected %q, got %q", i, v, args[i])
		}
	}
}

func TestBuildGeminiStreamArgs_ResumeSession(t *testing.T) {
	req := ChatRequest{
		Prompt:    "continue",
		SessionID: "ses_abc123",
		Resume:    true,
		WorkDir:   "/home/user/project",
	}
	args := buildGeminiStreamArgs(req)

	// Should contain --resume latest
	found := false
	for i, a := range args {
		if a == "--resume" && i+1 < len(args) && args[i+1] == "latest" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --resume latest in args when Resume=true")
	}
}

func TestBuildGeminiStreamArgs_NoResumeWithoutFlag(t *testing.T) {
	req := ChatRequest{
		Prompt:    "new session",
		SessionID: "some-id",
		Resume:    false,
	}
	args := buildGeminiStreamArgs(req)

	for _, a := range args {
		if a == "--resume" {
			t.Error("should not contain --resume when Resume=false")
		}
	}
}

func TestBuildGeminiStreamArgs_Minimal(t *testing.T) {
	req := ChatRequest{
		Prompt: "hello",
	}
	args := buildGeminiStreamArgs(req)

	expected := []string{"--prompt", "hello", "--output-format", "stream-json", "--yolo"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, v := range expected {
		if args[i] != v {
			t.Errorf("arg %d: expected %q, got %q", i, v, args[i])
		}
	}
}

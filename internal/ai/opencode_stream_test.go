package ai

import (
	"encoding/json"
	"testing"
)

func parseOpenCodeLine(line string) []StreamEvent {
	ch := make(chan StreamEvent, 64)
	parser := &OpenCodeStreamParser{}
	parser.ParseLine(line, ch)
	close(ch)
	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestOpenCodeStream_ParseLine_Text(t *testing.T) {
	line := `{"type":"text","timestamp":1777038590233,"sessionID":"ses_abc123","part":{"type":"text","text":"\n\nHello, world!"}}`
	events := parseOpenCodeLine(line)

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

func TestOpenCodeStream_ParseLine_TextNoPrefix(t *testing.T) {
	line := `{"type":"text","timestamp":1,"sessionID":"ses_abc","part":{"type":"text","text":"No prefix here"}}`
	events := parseOpenCodeLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Content != "No prefix here" {
		t.Errorf("expected 'No prefix here', got %q", events[0].Content)
	}
}

func TestOpenCodeStream_ParseLine_Reasoning(t *testing.T) {
	line := `{"type":"reasoning","timestamp":1,"sessionID":"ses_abc","part":{"type":"reasoning","text":"I need to think about this."}}`
	events := parseOpenCodeLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "thinking" {
		t.Errorf("expected thinking event, got %s", events[0].Type)
	}
	if events[0].Content != "I need to think about this." {
		t.Errorf("unexpected content: %q", events[0].Content)
	}
}

func TestOpenCodeStream_ParseLine_ToolUse(t *testing.T) {
	line := `{"type":"tool_use","timestamp":1,"sessionID":"ses_abc","part":{"type":"tool","tool":"read","callID":"call_123","state":{"status":"completed","input":{"filePath":"/tmp/test.go"},"output":"file content here"}}}`
	events := parseOpenCodeLine(line)

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
	if tool.Name != "read" {
		t.Errorf("expected tool name 'read', got %q", tool.Name)
	}
	if tool.ID != "call_123" {
		t.Errorf("expected call ID 'call_123', got %q", tool.ID)
	}
	if !tool.Done {
		t.Error("expected Done=true for completed tool")
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

func TestOpenCodeStream_ParseLine_ToolUseRunning(t *testing.T) {
	line := `{"type":"tool_use","timestamp":1,"sessionID":"ses_abc","part":{"type":"tool","tool":"bash","callID":"call_456","state":{"status":"running","input":{"command":"ls"}}}}`
	events := parseOpenCodeLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	tool := events[0].Tool
	if tool.Done {
		t.Error("expected Done=false for running tool")
	}
}

func TestOpenCodeStream_ParseLine_StepFinishStop(t *testing.T) {
	line := `{"type":"step_finish","timestamp":1,"sessionID":"ses_abc","part":{"reason":"stop","tokens":{"total":36690,"input":36635,"output":55,"reasoning":0},"cost":0}}`
	events := parseOpenCodeLine(line)

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
	if meta.SessionID != "ses_abc" {
		t.Errorf("expected sessionID 'ses_abc', got %q", meta.SessionID)
	}
	if meta.InputTokens != 36635 {
		t.Errorf("expected input tokens 36635, got %d", meta.InputTokens)
	}
	if meta.OutputTokens != 55 {
		t.Errorf("expected output tokens 55, got %d", meta.OutputTokens)
	}
	if meta.StopReason != "stop" {
		t.Errorf("expected stopReason 'stop', got %q", meta.StopReason)
	}
	if events[1].Type != "done" {
		t.Errorf("expected done event second, got %s", events[1].Type)
	}
}

func TestOpenCodeStream_ParseLine_StepFinishToolCalls(t *testing.T) {
	line := `{"type":"step_finish","timestamp":1,"sessionID":"ses_abc","part":{"reason":"tool-calls","tokens":{"total":36715,"input":326,"output":77},"cost":0}}`
	events := parseOpenCodeLine(line)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for tool-calls step_finish, got %d: %+v", len(events), events)
	}
}

func TestOpenCodeStream_ParseLine_StepStart(t *testing.T) {
	line := `{"type":"step_start","timestamp":1,"sessionID":"ses_abc","part":{"type":"step-start"}}`
	events := parseOpenCodeLine(line)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for step_start, got %d", len(events))
	}
}

func TestOpenCodeStream_ParseLine_EmptyText(t *testing.T) {
	line := `{"type":"text","timestamp":1,"sessionID":"ses_abc","part":{"type":"text","text":"\n\n"}}`
	events := parseOpenCodeLine(line)

	// After stripping \n\n prefix, text is empty → no event
	if len(events) != 0 {
		t.Fatalf("expected 0 events for empty text after prefix strip, got %d", len(events))
	}
}

func TestOpenCodeStream_ParseLine_UnparseableLine(t *testing.T) {
	events := parseOpenCodeLine("not json at all")
	if len(events) != 0 {
		t.Fatalf("expected 0 events for unparseable line, got %d", len(events))
	}
}

func TestOpenCodeStream_ParseLine_UnknownType(t *testing.T) {
	line := `{"type":"custom_event","timestamp":1,"sessionID":"ses_abc","part":{}}`
	events := parseOpenCodeLine(line)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for unknown type, got %d", len(events))
	}
}

func TestOpenCodeStream_SessionIDCapture(t *testing.T) {
	parser := &OpenCodeStreamParser{}
	ch := make(chan StreamEvent, 64)

	// First message captures session ID
	line1 := `{"type":"step_start","timestamp":1,"sessionID":"ses_captured123","part":{"type":"step-start"}}`
	parser.ParseLine(line1, ch)

	// Second message uses the captured session ID in metadata
	line2 := `{"type":"step_finish","timestamp":2,"part":{"reason":"stop","tokens":{"total":100,"input":80,"output":20},"cost":0}}`
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
		t.Errorf("expected sessionID 'ses_captured123' in metadata, got %q", events[0].Meta.SessionID)
	}
}

func TestOpenCodeStream_MultiStepFlow(t *testing.T) {
	lines := []string{
		`{"type":"step_start","timestamp":1,"sessionID":"ses_multi","part":{"type":"step-start"}}`,
		`{"type":"tool_use","timestamp":2,"sessionID":"ses_multi","part":{"type":"tool","tool":"read","callID":"call_1","state":{"status":"completed","input":{"filePath":"main.go"},"output":"package main"}}}`,
		`{"type":"step_finish","timestamp":3,"sessionID":"ses_multi","part":{"reason":"tool-calls","tokens":{"total":500,"input":400,"output":100},"cost":0}}`,
		`{"type":"step_start","timestamp":4,"sessionID":"ses_multi","part":{"type":"step-start"}}`,
		`{"type":"text","timestamp":5,"sessionID":"ses_multi","part":{"type":"text","text":"\n\nThe file uses package main."}}`,
		`{"type":"step_finish","timestamp":6,"sessionID":"ses_multi","part":{"reason":"stop","tokens":{"total":600,"input":500,"output":100},"cost":0}}`,
	}

	ch := make(chan StreamEvent, 64)
	parser := &OpenCodeStreamParser{}
	for _, line := range lines {
		parser.ParseLine(line, ch)
	}
	close(ch)

	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Expected: tool_use, content, metadata, done
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	// Event 1: tool_use
	if events[0].Type != "tool_use" {
		t.Errorf("event 0: expected tool_use, got %s", events[0].Type)
	}
	if events[0].Tool.Name != "read" {
		t.Errorf("event 0: expected tool name 'read', got %q", events[0].Tool.Name)
	}

	// Event 2: content
	if events[1].Type != "content" {
		t.Errorf("event 1: expected content, got %s", events[1].Type)
	}
	if events[1].Content != "The file uses package main." {
		t.Errorf("event 1: unexpected content %q", events[1].Content)
	}

	// Event 3: metadata
	if events[2].Type != "metadata" {
		t.Errorf("event 2: expected metadata, got %s", events[2].Type)
	}
	if events[2].Meta.SessionID != "ses_multi" {
		t.Errorf("event 2: expected sessionID 'ses_multi', got %q", events[2].Meta.SessionID)
	}

	// Event 4: done
	if events[3].Type != "done" {
		t.Errorf("event 3: expected done, got %s", events[3].Type)
	}
}

func TestBuildOpenCodeStreamArgs_NewSession(t *testing.T) {
	req := ChatRequest{
		Prompt:  "say hello",
		WorkDir: "/home/user/project",
		Model:   "minimax-cn-coding-plan/MiniMax-M2.7",
	}
	args := buildOpenCodeStreamArgs(req)

	expected := []string{"run", "say hello", "--format", "json", "--dangerously-skip-permissions", "--dir", "/home/user/project", "--model", "minimax-cn-coding-plan/MiniMax-M2.7"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, v := range expected {
		if args[i] != v {
			t.Errorf("arg %d: expected %q, got %q", i, v, args[i])
		}
	}

	// Should NOT contain --session (new session, Resume=false)
	for _, a := range args {
		if a == "--session" {
			t.Error("should not contain --session for new session")
		}
	}
}

func TestBuildOpenCodeStreamArgs_NewSessionWithClawBenchUUID(t *testing.T) {
	// When ClawBench sends a UUID session ID but Resume=false, should NOT pass --session
	req := ChatRequest{
		Prompt:    "hello",
		SessionID: "c8abd620-87e9-43d7-a031-fa20674667d0", // ClawBench UUID
		Resume:    false,
	}
	args := buildOpenCodeStreamArgs(req)

	for _, a := range args {
		if a == "--session" {
			t.Error("should not contain --session when Resume=false even with UUID SessionID")
		}
	}
}

func TestBuildOpenCodeStreamArgs_ResumeSession(t *testing.T) {
	req := ChatRequest{
		Prompt:    "continue",
		SessionID: "ses_abc123", // OpenCode session ID
		Resume:    true,
		WorkDir:   "/home/user/project",
	}
	args := buildOpenCodeStreamArgs(req)

	// Should contain --session ses_abc123 because Resume=true
	found := false
	for i, a := range args {
		if a == "--session" && i+1 < len(args) && args[i+1] == "ses_abc123" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --session ses_abc123 in args when Resume=true")
	}
}

func TestBuildOpenCodeStreamArgs_Minimal(t *testing.T) {
	req := ChatRequest{
		Prompt: "hello",
	}
	args := buildOpenCodeStreamArgs(req)

	// Minimal args: run, prompt, --format json, --thinking, --dangerously-skip-permissions
	expected := []string{"run", "hello", "--format", "json", "--dangerously-skip-permissions"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, v := range expected {
		if args[i] != v {
			t.Errorf("arg %d: expected %q, got %q", i, v, args[i])
		}
	}
}

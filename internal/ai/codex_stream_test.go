package ai

import (
	"strings"
	"testing"
)

func parseCodexLine(line string) []StreamEvent {
	ch := make(chan StreamEvent, 64)
	parser := &CodexStreamParser{}
	parser.ParseLine(line, ch)
	close(ch)
	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

// --- ParseLine tests ---

func TestCodexStream_ThreadStarted(t *testing.T) {
	line := `{"type":"thread.started","thread_id":"019dc744-1f6e-75d0-9877-99c8d2f134da"}`
	events := parseCodexLine(line)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for thread.started, got %d", len(events))
	}
}

func TestCodexStream_AgentMessageTextOnly(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Hello, world!"}}`
	events := parseCodexLine(line)

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

func TestCodexStream_AgentMessageWithThinking(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Let me think about this.\n\nHere is my answer."}}`
	events := parseCodexLine(line)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "thinking" {
		t.Errorf("expected thinking event, got %s", events[0].Type)
	}
	if events[0].Content != "Let me think about this." {
		t.Errorf("expected thinking content, got %q", events[0].Content)
	}
	if events[1].Type != "content" {
		t.Errorf("expected content event, got %s", events[1].Type)
	}
	if events[1].Content != "Here is my answer." {
		t.Errorf("expected content, got %q", events[1].Content)
	}
}

func TestCodexStream_AgentMessageOnlyThinking(t *testing.T) {
	// Text ends with \n\n but no content after it
	line := `{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Just thinking...\n\n"}}`
	events := parseCodexLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event (thinking only), got %d", len(events))
	}
	if events[0].Type != "thinking" {
		t.Errorf("expected thinking event, got %s", events[0].Type)
	}
	if events[0].Content != "Just thinking..." {
		t.Errorf("expected 'Just thinking...', got %q", events[0].Content)
	}
}

func TestCodexStream_AgentMessageEmpty(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":""}}`
	events := parseCodexLine(line)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for empty text, got %d", len(events))
	}
}

func TestCodexStream_AgentMessageMultipleParagraphs(t *testing.T) {
	// Only the first \n\n splits thinking from content
	line := `{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Thinking here.\n\nFirst paragraph.\n\nSecond paragraph."}}`
	events := parseCodexLine(line)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "thinking" || events[0].Content != "Thinking here." {
		t.Errorf("expected thinking, got type=%s content=%q", events[0].Type, events[0].Content)
	}
	if events[1].Type != "content" {
		t.Errorf("expected content event, got %s", events[1].Type)
	}
	// Content should include remaining \n\n
	if events[1].Content != "First paragraph.\n\nSecond paragraph." {
		t.Errorf("expected multi-paragraph content, got %q", events[1].Content)
	}
}

func TestCodexStream_CommandExecutionStarted(t *testing.T) {
	line := `{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"bash -lc 'ls -la'","aggregated_output":"","exit_code":null,"status":"in_progress"}}`
	events := parseCodexLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "tool_use" {
		t.Errorf("expected tool_use event, got %s", events[0].Type)
	}
	if events[0].Tool.Done {
		t.Error("expected tool to not be done")
	}
	if events[0].Tool.Name != "command_execution" {
		t.Errorf("expected tool name 'command_execution', got %q", events[0].Tool.Name)
	}
	if events[0].Tool.Input != "bash -lc 'ls -la'" {
		t.Errorf("expected command as input, got %q", events[0].Tool.Input)
	}
}

func TestCodexStream_CommandExecutionCompleted(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"bash -lc 'ls -la'","aggregated_output":"file1.txt\nfile2.txt","exit_code":0,"status":"completed"}}`
	events := parseCodexLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "tool_use" {
		t.Errorf("expected tool_use event, got %s", events[0].Type)
	}
	if !events[0].Tool.Done {
		t.Error("expected tool to be done")
	}
	if events[0].Tool.Name != "command_execution" {
		t.Errorf("expected tool name 'command_execution', got %q", events[0].Tool.Name)
	}
}

func TestCodexStream_CommandExecutionFailed(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_2","type":"command_execution","command":"bash -lc 'exit 1'","aggregated_output":"error: something went wrong","exit_code":1,"status":"completed"}}`
	events := parseCodexLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !events[0].Tool.Done {
		t.Error("expected tool to be done even on failure")
	}
	if !strings.Contains(events[0].Tool.Input, "exit 1") {
		t.Errorf("expected command in input, got %q", events[0].Tool.Input)
	}
	if !strings.Contains(events[0].Tool.Input, "error: something went wrong") {
		t.Errorf("expected output in input, got %q", events[0].Tool.Input)
	}
}

func TestCodexStream_CommandExecutionNoOutput(t *testing.T) {
	line := `{"type":"item.completed","item":{"id":"item_3","type":"command_execution","command":"bash -lc 'true'","aggregated_output":"","exit_code":0,"status":"completed"}}`
	events := parseCodexLine(line)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	// Input should just be the command when no output
	if events[0].Tool.Input != "bash -lc 'true'" {
		t.Errorf("expected command only, got %q", events[0].Tool.Input)
	}
}

func TestCodexStream_ItemCompletedNilItem(t *testing.T) {
	line := `{"type":"item.completed"}`
	events := parseCodexLine(line)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for nil item, got %d", len(events))
	}
}

func TestCodexStream_ItemStartedNotCommandExecution(t *testing.T) {
	// agent_message items in item.started should be ignored
	line := `{"type":"item.started","item":{"id":"item_0","type":"agent_message","text":""}}`
	events := parseCodexLine(line)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for non-command item.started, got %d", len(events))
	}
}

func TestCodexStream_TurnCompleted(t *testing.T) {
	line := `{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":200}}`
	events := parseCodexLine(line)

	if len(events) != 2 {
		t.Fatalf("expected 2 events (metadata + done), got %d", len(events))
	}
	if events[0].Type != "metadata" {
		t.Errorf("expected metadata event, got %s", events[0].Type)
	}
	if events[0].Meta.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", events[0].Meta.InputTokens)
	}
	if events[0].Meta.OutputTokens != 200 {
		t.Errorf("expected 200 output tokens, got %d", events[0].Meta.OutputTokens)
	}
	if events[1].Type != "done" {
		t.Errorf("expected done event, got %s", events[1].Type)
	}
}

func TestCodexStream_TurnCompletedNoUsage(t *testing.T) {
	line := `{"type":"turn.completed","usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0}}`
	events := parseCodexLine(line)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Meta.InputTokens != 0 {
		t.Errorf("expected 0 input tokens, got %d", events[0].Meta.InputTokens)
	}
}

func TestCodexStream_UnknownType(t *testing.T) {
	line := `{"type":"unknown_event","data":"something"}`
	events := parseCodexLine(line)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for unknown type, got %d", len(events))
	}
}

func TestCodexStream_UnparseableLine(t *testing.T) {
	line := `not json at all`
	events := parseCodexLine(line)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for unparseable line, got %d", len(events))
	}
}

func TestCodexStream_TurnStarted(t *testing.T) {
	line := `{"type":"turn.started"}`
	events := parseCodexLine(line)

	if len(events) != 0 {
		t.Fatalf("expected 0 events for turn.started, got %d", len(events))
	}
}

// --- Multi-event / stateful tests ---

func TestCodexStream_ThreadIDCapture(t *testing.T) {
	ch := make(chan StreamEvent, 64)
	parser := &CodexStreamParser{}

	threadLine := `{"type":"thread.started","thread_id":"019dc744-1f6e-75d0-9877-99c8d2f134da"}`
	parser.ParseLine(threadLine, ch)

	turnLine := `{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":20}}`
	parser.ParseLine(turnLine, ch)
	close(ch)

	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Meta.SessionID != "019dc744-1f6e-75d0-9877-99c8d2f134da" {
		t.Errorf("expected thread_id in metadata, got %q", events[0].Meta.SessionID)
	}
}

func TestCodexStream_FullFlow(t *testing.T) {
	ch := make(chan StreamEvent, 64)
	parser := &CodexStreamParser{}

	lines := []string{
		`{"type":"thread.started","thread_id":"019dc744-1f6e-75d0-9877-99c8d2f134da"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Hmm...\n\nLet me check the files."}}`,
		`{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"ls","aggregated_output":"","exit_code":null,"status":"in_progress"}}`,
		`{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"ls","aggregated_output":"a.go b.go","exit_code":0,"status":"completed"}}`,
		`{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"Here are the files."}}`,
		`{"type":"turn.completed","usage":{"input_tokens":500,"cached_input_tokens":0,"output_tokens":100}}`,
	}

	for _, line := range lines {
		parser.ParseLine(line, ch)
	}
	close(ch)

	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Expected: thinking, content, tool_use(started), tool_use(completed), content, metadata, done = 7
	if len(events) != 7 {
		t.Fatalf("expected 7 events in full flow, got %d", len(events))
	}

	// Verify order and types
	expectedTypes := []string{"thinking", "content", "tool_use", "tool_use", "content", "metadata", "done"}
	for i, ev := range events {
		if ev.Type != expectedTypes[i] {
			t.Errorf("event %d: expected type %s, got %s", i, expectedTypes[i], ev.Type)
		}
	}

	// Verify metadata carries thread_id
	if events[5].Meta.SessionID != "019dc744-1f6e-75d0-9877-99c8d2f134da" {
		t.Errorf("expected thread_id in metadata, got %q", events[5].Meta.SessionID)
	}
	if events[5].Meta.InputTokens != 500 {
		t.Errorf("expected 500 input tokens, got %d", events[5].Meta.InputTokens)
	}
}

// --- buildCodexStreamArgs tests ---

func TestBuildCodexStreamArgs_NewSession(t *testing.T) {
	req := ChatRequest{
		Prompt:    "hello world",
		WorkDir:   "/tmp/project",
		Model:     "codex-MiniMax-M2.7",
		SessionID: "test-session",
		Resume:    false,
	}
	args := buildCodexStreamArgs(req)

	if args[0] != "exec" {
		t.Errorf("expected first arg 'exec', got %q", args[0])
	}

	assertArg := func(name, value string) {
		for i, a := range args {
			if a == name && i+1 < len(args) && args[i+1] == value {
				return
			}
		}
		t.Errorf("expected %s %s in args", name, value)
	}

	assertArg("-C", "/tmp/project")
	assertArg("-m", "codex-MiniMax-M2.7")

	// Last arg should be prompt
	if args[len(args)-1] != "hello world" {
		t.Errorf("expected last arg to be prompt, got %q", args[len(args)-1])
	}
}

func TestBuildCodexStreamArgs_WithModel(t *testing.T) {
	req := ChatRequest{
		Prompt: "test",
		WorkDir: "/tmp",
		Model:  "MiniMax-M2.5-highspeed",
	}
	args := buildCodexStreamArgs(req)

	foundModel := false
	for i, a := range args {
		if a == "-m" && i+1 < len(args) && args[i+1] == "MiniMax-M2.5-highspeed" {
			foundModel = true
		}
	}
	if !foundModel {
		t.Error("expected -m MiniMax-M2.5-highspeed in args")
	}
}

func TestBuildCodexStreamArgs_NoModel(t *testing.T) {
	req := ChatRequest{
		Prompt:  "test",
		WorkDir: "/tmp",
		Model:   "",
	}
	args := buildCodexStreamArgs(req)

	for _, a := range args {
		if a == "-m" {
			t.Error("did not expect -m flag when no model specified")
		}
	}
}

func TestBuildCodexStreamArgs_NoWorkDir(t *testing.T) {
	req := ChatRequest{
		Prompt: "test",
	}
	args := buildCodexStreamArgs(req)

	for _, a := range args {
		if a == "-C" {
			t.Error("did not expect -C flag when no workdir specified")
		}
	}
}

// --- buildCodexResumeArgs tests ---

func TestBuildCodexResumeArgs(t *testing.T) {
	req := ChatRequest{
		Prompt: "continue this task",
	}
	args := buildCodexResumeArgs(req, "019dc744-1f6e-75d0-9877-99c8d2f134da")

	if args[0] != "exec" {
		t.Errorf("expected first arg 'exec', got %q", args[0])
	}
	if args[1] != "resume" {
		t.Errorf("expected second arg 'resume', got %q", args[1])
	}

	foundThreadID := false
	foundPrompt := false
	for _, arg := range args {
		if arg == "019dc744-1f6e-75d0-9877-99c8d2f134da" {
			foundThreadID = true
		}
		if arg == "continue this task" {
			foundPrompt = true
		}
	}
	if !foundThreadID {
		t.Error("expected thread_id in args")
	}
	if !foundPrompt {
		t.Error("expected prompt in args")
	}
}

func TestBuildCodexResumeArgs_NoModelOrWorkDir(t *testing.T) {
	req := ChatRequest{
		Prompt: "test",
	}
	args := buildCodexResumeArgs(req, "thread-123")

	// Resume args should NOT include -m or -C
	for _, a := range args {
		if a == "-m" || a == "-C" {
			t.Errorf("resume args should not contain %s", a)
		}
	}
}

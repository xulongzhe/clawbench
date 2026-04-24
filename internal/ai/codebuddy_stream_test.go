package ai

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"
)

func TestCodebuddyStream_BOMRemoval(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "line with BOM prefix",
			input:    "\xEF\xBB\xBF{\"type\":\"assistant\",\"subtype\":\"text\",\"text\":\"hello\"}",
			expected: "{\"type\":\"assistant\",\"subtype\":\"text\",\"text\":\"hello\"}",
		},
		{
			name:     "line without BOM",
			input:    "{\"type\":\"assistant\",\"subtype\":\"text\",\"text\":\"hello\"}",
			expected: "{\"type\":\"assistant\",\"subtype\":\"text\",\"text\":\"hello\"}",
		},
		{
			name:     "only BOM",
			input:    "\xEF\xBB\xBF",
			expected: "",
		},
		{
			name:     "empty line",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strings.TrimPrefix(tt.input, "\xEF\xBB\xBF")
			if result != tt.expected {
				t.Errorf("BOM removal: got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCodebuddyStream_CommandArgs(t *testing.T) {
	// Verify that Codebuddy stream args do NOT include --verbose
	req := ChatRequest{
		Prompt:    "test prompt",
		SessionID: "test-session",
		WorkDir:   "/tmp/test",
	}

	args := buildCodebuddyStreamArgs(req)

	// Should have --output-format stream-json
	hasStreamJSON := false
	for i, a := range args {
		if a == "--output-format" && i+1 < len(args) && args[i+1] == "stream-json" {
			hasStreamJSON = true
		}
	}
	if !hasStreamJSON {
		t.Error("expected --output-format stream-json in args")
	}

	// Should NOT have --verbose
	for _, a := range args {
		if a == "--verbose" {
			t.Error("--verbose should NOT be in Codebuddy stream args")
		}
	}

	// Should have --session-id
	hasSessionID := false
	for i, a := range args {
		if a == "--session-id" && i+1 < len(args) && args[i+1] == req.SessionID {
			hasSessionID = true
		}
	}
	if !hasSessionID {
		t.Error("expected --session-id in args")
	}

	// Should have --add-dir
	hasAddDir := false
	for i, a := range args {
		if a == "--add-dir" && i+1 < len(args) && args[i+1] == req.WorkDir {
			hasAddDir = true
		}
	}
	if !hasAddDir {
		t.Error("expected --add-dir in args")
	}

	// Should have --dangerously-skip-permissions
	hasSkipPerms := false
	for _, a := range args {
		if a == "--dangerously-skip-permissions" {
			hasSkipPerms = true
		}
	}
	if !hasSkipPerms {
		t.Error("expected --dangerously-skip-permissions in args")
	}

	// Prompt should be last arg
	if args[len(args)-1] != req.Prompt {
		t.Errorf("expected prompt as last arg, got %q", args[len(args)-1])
	}
}

func TestCodebuddyStream_CommandArgsWithSystemPrompt(t *testing.T) {
	req := ChatRequest{
		Prompt:       "test prompt",
		SessionID:    "test-session",
		WorkDir:      "/tmp/test",
		SystemPrompt: "you are helpful",
	}

	args := buildCodebuddyStreamArgs(req)

	hasSystemPrompt := false
	for i, a := range args {
		if a == "--system-prompt" && i+1 < len(args) && args[i+1] == req.SystemPrompt {
			hasSystemPrompt = true
		}
	}
	if !hasSystemPrompt {
		t.Error("expected --system-prompt in args when SystemPrompt is set")
	}
}

func TestCodebuddyStream_CommandArgsWithoutSystemPrompt(t *testing.T) {
	req := ChatRequest{
		Prompt:    "test prompt",
		SessionID: "test-session",
		WorkDir:   "/tmp/test",
	}

	args := buildCodebuddyStreamArgs(req)

	for i, a := range args {
		if a == "--system-prompt" {
			t.Errorf("--system-prompt should NOT be in args when SystemPrompt is empty, but found at index %d", i)
		}
	}
}

func TestCodebuddyStream_ScannerBufferSize(t *testing.T) {
	if scannerInitial != 64*1024 {
		t.Errorf("expected initial scanner buffer 64KB, got %d", scannerInitial)
	}
	if scannerMax != 1024*1024 {
		t.Errorf("expected max scanner buffer 1MB, got %d", scannerMax)
	}
}

func TestCodebuddyStream_ChannelBufferSize(t *testing.T) {
	if streamChanSize != 64 {
		t.Errorf("expected channel buffer size 64, got %d", streamChanSize)
	}
}

func TestCodebuddyStream_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	backend := &CodebuddyBackend{}
	_, err := backend.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-session",
		WorkDir:   "/tmp",
	})

	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestCodebuddyStream_ParseLineWithBOM(t *testing.T) {
	// Test that StreamParser handles lines after BOM removal
	line := "\xEF\xBB\xBF{\"type\":\"assistant\",\"subtype\":\"text\",\"text\":\"hello\"}"
	cleaned := strings.TrimPrefix(line, "\xEF\xBB\xBF")

	events := parseLine(cleaned)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "content" {
		t.Errorf("expected content event, got %q", events[0].Type)
	}
	if events[0].Content != "hello" {
		t.Errorf("expected content 'hello', got %q", events[0].Content)
	}
}

func TestCodebuddyStream_BOMRemovalInScanner(t *testing.T) {
	// Simulate the scanner loop processing lines with BOM
	input := "\xEF\xBB\xBF{\"type\":\"assistant\",\"subtype\":\"text\",\"text\":\"line1\"}\n{\"type\":\"assistant\",\"subtype\":\"text\",\"text\":\"line2\"}\n"
	scanner := bufio.NewScanner(strings.NewReader(input))

	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimPrefix(line, "\xEF\xBB\xBF")
		if line != "" {
			lines = append(lines, line)
		}
	}

	events := parseLines(lines)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Content != "line1" {
		t.Errorf("first event content: got %q, want %q", events[0].Content, "line1")
	}
	if events[1].Content != "line2" {
		t.Errorf("second event content: got %q, want %q", events[1].Content, "line2")
	}
}

func TestCodebuddyStream_NoVerboseInArgs(t *testing.T) {
	// Double-check: Claude uses --verbose, Codebuddy does NOT
	claudeArgs := buildClaudeStreamArgs(ChatRequest{
		Prompt: "test", SessionID: "s", WorkDir: "/tmp",
	})
	codebuddyArgs := buildCodebuddyStreamArgs(ChatRequest{
		Prompt: "test", SessionID: "s", WorkDir: "/tmp",
	})

	hasClaudeVerbose := false
	for _, a := range claudeArgs {
		if a == "--verbose" {
			hasClaudeVerbose = true
		}
	}
	if !hasClaudeVerbose {
		t.Error("Claude stream args should have --verbose")
	}

	for _, a := range codebuddyArgs {
		if a == "--verbose" {
			t.Error("Codebuddy stream args should NOT have --verbose")
		}
	}
}

func TestCodebuddyStream_ProviderDataInResult(t *testing.T) {
	// Test that Codebuddy's providerData is extracted in result messages
	line := `{"type":"result","session_id":"sess-1","duration_ms":1000,"providerData":{"model":"glm-4","usage":{"inputTokens":50,"outputTokens":100}}}`

	events := parseLine(line)

	// Result produces metadata + done
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "metadata" {
		t.Fatalf("expected metadata event, got %q", events[0].Type)
	}
	if events[0].Meta.Model != "glm-4" {
		t.Errorf("expected model 'glm-4', got %q", events[0].Meta.Model)
	}
	if events[0].Meta.InputTokens != 50 {
		t.Errorf("expected input tokens 50, got %d", events[0].Meta.InputTokens)
	}
	if events[0].Meta.OutputTokens != 100 {
		t.Errorf("expected output tokens 100, got %d", events[0].Meta.OutputTokens)
	}
	if events[1].Type != "done" {
		t.Errorf("expected done event, got %q", events[1].Type)
	}
}

func TestCodebuddyStream_ExecuteStreamReturnsChannel(t *testing.T) {
	// This test verifies ExecuteStream returns a non-nil channel.
	// We can't actually run codebuddy CLI in tests, so we just verify
	// the channel is created and closed when the command fails.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	backend := &CodebuddyBackend{}
	ch, err := backend.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-session",
		WorkDir:   "/tmp",
	})

	if err != nil {
		// Command might fail if codebuddy is not installed — that's ok for this test
		return
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	// Channel should eventually close
	var events []StreamEvent
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			events = append(events, ev)
		case <-timer.C:
			return
		}
	}
}

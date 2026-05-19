package ai

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseVeCLISessionSummary tests ---

func TestParseVeCLISessionSummary_ValidJSON(t *testing.T) {
	data := []byte(`{
		"sessionMetrics": {
			"models": {
				"deepseek-v3-1-terminus": {
					"api": {"totalRequests": 3, "totalErrors": 0, "totalLatencyMs": 549},
					"tokens": {"prompt": 400, "candidates": 100, "total": 500, "cached": 0, "thoughts": 0, "tool": 0}
				}
			},
			"tools": {"totalCalls": 2, "totalSuccess": 2, "totalFail": 0, "totalDurationMs": 300},
			"files": {"totalLinesAdded": 10, "totalLinesRemoved": 5}
		}
	}`)

	summary, err := parseVeCLISessionSummary(data)
	require.NoError(t, err)
	require.NotNil(t, summary)

	meta := summary.extractMetadata("deepseek-v3-1-terminus")
	assert.Equal(t, "deepseek-v3-1-terminus", meta.Model)
	assert.Equal(t, 400, meta.InputTokens)
	assert.Equal(t, 100, meta.OutputTokens)
	assert.Equal(t, 549, meta.DurationMs)
	assert.Equal(t, "stop", meta.StopReason)
}

func TestParseVeCLISessionSummary_InvalidJSON(t *testing.T) {
	_, err := parseVeCLISessionSummary([]byte("not valid json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vecli: failed to parse session-summary")
}

func TestParseVeCLISessionSummary_EmptyInput(t *testing.T) {
	_, err := parseVeCLISessionSummary([]byte{})
	assert.Error(t, err)
}

func TestParseVeCLISessionSummary_EmptyModels(t *testing.T) {
	data := []byte(`{"sessionMetrics":{"models":{},"tools":{},"files":{}}}`)
	summary, err := parseVeCLISessionSummary(data)
	require.NoError(t, err)

	meta := summary.extractMetadata("fallback-model")
	assert.Equal(t, "fallback-model", meta.Model)
	assert.Equal(t, 0, meta.InputTokens)
}

func TestParseVeCLISessionSummary_ModelFallback(t *testing.T) {
	data := []byte(`{"sessionMetrics":{"models":{"model-a":{"api":{"totalLatencyMs":100},"tokens":{"prompt":50,"candidates":25,"total":75}}},"tools":{},"files":{}}}`)
	summary, err := parseVeCLISessionSummary(data)
	require.NoError(t, err)

	// Request model matches
	meta := summary.extractMetadata("model-a")
	assert.Equal(t, "model-a", meta.Model)
	assert.Equal(t, 50, meta.InputTokens)
	assert.Equal(t, 25, meta.OutputTokens)
	assert.Equal(t, 100, meta.DurationMs)
}

func TestParseVeCLISessionSummary_FirstEntryFallback(t *testing.T) {
	data := []byte(`{"sessionMetrics":{"models":{"model-x":{"api":{"totalRequests":1,"totalErrors":0,"totalLatencyMs":200},"tokens":{"prompt":80,"candidates":40,"total":120,"cached":0,"thoughts":0,"tool":0}},"model-y":{"api":{"totalRequests":1,"totalErrors":0,"totalLatencyMs":50},"tokens":{"prompt":10,"candidates":5,"total":15,"cached":0,"thoughts":0,"tool":0}}},"tools":{"totalCalls":0},"files":{"totalLinesAdded":0,"totalLinesRemoved":0}}}`)
	summary, err := parseVeCLISessionSummary(data)
	require.NoError(t, err)

	// No matching model — should use first entry as fallback
	meta := summary.extractMetadata("model-z")
	assert.Equal(t, "model-x", meta.Model, "should fall back to first entry when no match")
	assert.Equal(t, 80, meta.InputTokens)
}

// --- ShouldInjectSystemPrompt tests ---

func TestShouldInjectSystemPrompt_EmptySystemPrompt(t *testing.T) {
	req := ChatRequest{SystemPrompt: ""}
	assert.False(t, req.ShouldInjectSystemPrompt(), "empty system prompt should not inject")
}

func TestShouldInjectSystemPrompt_NewSession(t *testing.T) {
	req := ChatRequest{
		SystemPrompt: "Be helpful",
		Resume:       false,
	}
	assert.True(t, req.ShouldInjectSystemPrompt(), "new session with system prompt should inject")
}

func TestShouldInjectSystemPrompt_ResumeWithZeroInterval(t *testing.T) {
	orig := model.ChatSystemPromptInterval
	model.ChatSystemPromptInterval = 0
	defer func() { model.ChatSystemPromptInterval = orig }()

	req := ChatRequest{
		SystemPrompt: "Be helpful",
		Resume:       true,
	}
	assert.False(t, req.ShouldInjectSystemPrompt(), "resume with zero interval should not inject")
}

func TestShouldInjectSystemPrompt_ResumeWithInterval(t *testing.T) {
	orig := model.ChatSystemPromptInterval
	model.ChatSystemPromptInterval = 5
	defer func() { model.ChatSystemPromptInterval = orig }()

	tests := []struct {
		name                string
		assistantMsgCount   int
		expected            bool
	}{
		{"count 0 — not at interval", 0, false},
		{"count 5 — at interval", 5, true},
		{"count 10 — at interval", 10, true},
		{"count 3 — not at interval", 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ChatRequest{
				SystemPrompt:          "Be helpful",
				Resume:                true,
				AssistantMessageCount: tt.assistantMsgCount,
			}
			assert.Equal(t, tt.expected, req.ShouldInjectSystemPrompt())
		})
	}
}

func TestShouldInjectSystemPrompt_ResumeWithoutSystemPrompt(t *testing.T) {
	req := ChatRequest{
		SystemPrompt: "",
		Resume:       true,
	}
	assert.False(t, req.ShouldInjectSystemPrompt(), "no system prompt means no injection regardless of resume")
}

// --- GetCapturedSessionID tests ---

func TestGeminiStreamParser_GetCapturedSessionID(t *testing.T) {
	p := &GeminiStreamParser{}
	assert.Equal(t, "", p.GetCapturedSessionID(), "GeminiStreamParser always returns empty string")
}

func TestStreamParser_GetCapturedSessionID(t *testing.T) {
	p := &StreamParser{}
	assert.Equal(t, "", p.GetCapturedSessionID(), "StreamParser always returns empty string")
}

// --- forwardEvent tests ---

func TestForwardEvent_ChannelWithSpace(t *testing.T) {
	ch := make(chan StreamEvent, 1)
	evt := StreamEvent{Type: "content", Content: "hello"}

	forwardEvent(ch, evt)

	select {
	case received := <-ch:
		assert.Equal(t, "content", received.Type)
		assert.Equal(t, "hello", received.Content)
	default:
		t.Fatal("expected event on channel")
	}
}

func TestForwardEvent_FullChannel(t *testing.T) {
	ch := make(chan StreamEvent, 1)
	ch <- StreamEvent{Type: "content", Content: "first"}

	// Channel is full — forwardEvent should not block
	forwardEvent(ch, StreamEvent{Type: "content", Content: "dropped"})

	// Only the first event should be on the channel
	select {
	case received := <-ch:
		assert.Equal(t, "first", received.Content)
	default:
		t.Fatal("expected first event on channel")
	}

	// Second event was dropped — channel is empty
	select {
	case <-ch:
		t.Fatal("second event should have been dropped")
	default:
		// expected
	}
}

// --- VeCLIBackend.vecliPreStart tests ---

func TestVeCLIPreStart(t *testing.T) {
	tmpDir := t.TempDir()
	origBinDir := model.BinDir
	model.BinDir = tmpDir
	defer func() { model.BinDir = origBinDir }()

	b := NewVeCLIBackend()
	cmd := exec.Command("echo", "test")
	req := ChatRequest{SessionID: "test-session-123"}

	b.vecliPreStart(cmd, req)

	// Verify --session-summary was appended to args
	found := false
	for i, a := range cmd.Args {
		if a == "--session-summary" && i+1 < len(cmd.Args) {
			assert.Contains(t, cmd.Args[i+1], "test-session-123.json")
			found = true
			break
		}
	}
	assert.True(t, found, "--session-summary should be appended to cmd.Args")

	// Verify summaryMap has the entry
	val, ok := b.summaryMap.Load("test-session-123")
	assert.True(t, ok, "summaryMap should contain session ID")
	assert.Contains(t, val.(string), "test-session-123.json")

	// Verify the directory was created
	summaryDir := tmpDir + "/.clawbench/vecli-summary"
	info, err := os.Stat(summaryDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// --- truncateToolOutput tests ---

func TestTruncateToolOutput_UnderLimit(t *testing.T) {
	input := "hello world"
	assert.Equal(t, input, truncateToolOutput(input))
}

func TestTruncateToolOutput_ExactlyAtLimit(t *testing.T) {
	input := strings.Repeat("a", 51200) // exactly 50KB
	result := truncateToolOutput(input)
	assert.Equal(t, input, result, "output at exactly 50KB should not be truncated")
}

func TestTruncateToolOutput_OverLimit(t *testing.T) {
	input := strings.Repeat("a", 60000) // 60KB
	result := truncateToolOutput(input)
	assert.Equal(t, 51200, len(strings.Split(result, "\n[truncated")[0]))
	assert.Contains(t, result, "[truncated: original 60000 bytes]")
}

func TestTruncateToolOutput_Empty(t *testing.T) {
	assert.Equal(t, "", truncateToolOutput(""))
}

// --- CLIBackend.ExecuteStream integration tests ---

func TestCLIBackend_ExecuteStream_WithEchoOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-dependent test, skipped on Windows")
	}
	// Use echo to simulate a CLI that outputs JSON lines
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "echo",
		buildArgs: func(req ChatRequest) []string {
			return []string{`{"type":"assistant","subtype":"text","text":"Hello from CLI"}`}
		},
		newParser: func() LineParser { return &StreamParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-echo",
		WorkDir:   t.TempDir(),
	})
	require.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	// Should get at least a raw_output event and content event
	hasContent := false
	for _, e := range events {
		if e.Type == "content" && e.Content == "Hello from CLI" {
			hasContent = true
		}
	}
	assert.True(t, hasContent, "should receive content event from echo output")
}

func TestCLIBackend_ExecuteStream_CustomCommand(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping flaky test in CI")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell-dependent test, skipped on Windows")
	}
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "nonexistent",
		buildArgs: func(req ChatRequest) []string {
			return []string{`{"type":"assistant","subtype":"text","text":"custom cmd"}`}
		},
		newParser: func() LineParser { return &StreamParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Override command via ChatRequest.Command
	ch, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-custom-cmd",
		WorkDir:   t.TempDir(),
		Command:   "echo",
	})
	require.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}
	assert.NotEmpty(t, events, "should receive events from custom command")
}

func TestCLIBackend_ExecuteStream_WithPreStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-dependent test, skipped on Windows")
	}
	preStartCalled := false
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "echo",
		buildArgs:      func(req ChatRequest) []string { return []string{"hello"} },
		newParser:      func() LineParser { return &StreamParser{} },
		preStart: func(cmd *exec.Cmd, req ChatRequest) {
			preStartCalled = true
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-prestart",
		WorkDir:   t.TempDir(),
	})
	require.NoError(t, err)
	for range ch {
	}
	assert.True(t, preStartCalled, "preStart hook should be called")
}

func TestCLIBackend_ExecuteStream_ScheduledExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-dependent test, skipped on Windows")
	}
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "echo",
		buildArgs:      func(req ChatRequest) []string { return []string{"output"} },
		newParser:      func() LineParser { return &StreamParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:             "test",
		SessionID:          "test-scheduled",
		WorkDir:            t.TempDir(),
		ScheduledExecution: true,
	})
	require.NoError(t, err)
	for range ch {
	}
}

func TestCLIBackend_ExecuteStream_WithFilterLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-dependent test, skipped on Windows")
	}
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "printf",
		buildArgs: func(req ChatRequest) []string {
			return []string{`non-json-line\n{"type":"assistant","subtype":"text","text":"filtered"}\n`}
		},
		newParser:  func() LineParser { return &StreamParser{} },
		filterLine: filterSkipNonJSON(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-filter",
		WorkDir:   t.TempDir(),
	})
	require.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	// The non-JSON line should be filtered out; only the JSON line should be parsed
	hasContent := false
	for _, e := range events {
		if e.Type == "content" && e.Content == "filtered" {
			hasContent = true
		}
	}
	assert.True(t, hasContent, "should receive content from filtered JSON line")
}

func TestCLIBackend_ExecuteStream_CommandExitsWithError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-dependent test, skipped on Windows")
	}
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "false", // exits with non-zero status
		buildArgs:      func(req ChatRequest) []string { return []string{} },
		newParser:      func() LineParser { return &StreamParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-exit-error",
		WorkDir:   t.TempDir(),
	})
	require.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	// Should get a warning about abnormal exit
	hasWarning := false
	for _, e := range events {
		if e.Type == "warning" {
			hasWarning = true
			break
		}
	}
	assert.True(t, hasWarning, "should receive warning for abnormal exit")
}

func TestCLIBackend_ExecuteStream_CommandWithStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-dependent test, skipped on Windows")
	}
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "bash",
		buildArgs:      func(req ChatRequest) []string { return []string{"-c", "echo error-msg >&2"} },
		newParser:      func() LineParser { return &StreamParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-stderr",
		WorkDir:   t.TempDir(),
	})
	require.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	// Should get a warning with stderr content
	hasStderrWarning := false
	for _, e := range events {
		if e.Type == "warning" && strings.Contains(e.Content, "error-msg") {
			hasStderrWarning = true
		}
	}
	assert.True(t, hasStderrWarning, "should receive warning with stderr content")
}

func TestCLIBackend_ExecuteStream_SessionCapture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-dependent test, skipped on Windows")
	}
	// Use a parser that returns a session ID for session_capture event testing
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "echo",
		buildArgs: func(req ChatRequest) []string {
			return []string{`{"type":"step_start","session_id":"ses_123"}`}
		},
		newParser: func() LineParser { return &sessionCapturingParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-capture",
		WorkDir:   t.TempDir(),
	})
	require.NoError(t, err)

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	hasCapture := false
	for _, e := range events {
		if e.Type == "session_capture" && e.Content == "ses_123" {
			hasCapture = true
		}
	}
	assert.True(t, hasCapture, "should receive session_capture event")
}

// sessionCapturingParser is a test LineParser that returns a session ID
type sessionCapturingParser struct {
	sessionID string
}

func (p *sessionCapturingParser) ParseLine(line string, ch chan<- StreamEvent) {
	// Parse the line looking for session_id
	if strings.Contains(line, `"session_id"`) {
		// Extract a simple session ID for testing
		p.sessionID = "ses_123"
	}
}

func (p *sessionCapturingParser) GetCapturedSessionID() string {
	return p.sessionID
}

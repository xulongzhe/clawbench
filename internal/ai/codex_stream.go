package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// CodexStreamMessage represents a single JSON line from `codex exec --json`
type CodexStreamMessage struct {
	Type     string          `json:"type"`
	ThreadID string          `json:"thread_id,omitempty"`
	Item     *CodexItem      `json:"item,omitempty"`
	Usage    *CodexUsage     `json:"usage,omitempty"`
}

// CodexItem represents an item in Codex stream output
type CodexItem struct {
	ID               string `json:"id"`
	Type             string `json:"type"`               // "agent_message" or "command_execution"
	Text             string `json:"text,omitempty"`     // agent_message
	Command          string `json:"command,omitempty"`  // command_execution
	AggregatedOutput string `json:"aggregated_output,omitempty"` // command_execution
	ExitCode         *int   `json:"exit_code,omitempty"`        // command_execution
	Status           string `json:"status,omitempty"`           // "in_progress" or "completed"
}

// CodexUsage represents token usage in a turn.completed event
type CodexUsage struct {
	InputTokens        int `json:"input_tokens"`
	CachedInputTokens  int `json:"cached_input_tokens"`
	OutputTokens       int `json:"output_tokens"`
}

// CodexStreamParser parses JSON Lines output from `codex exec --json`
type CodexStreamParser struct {
	threadID string // captured from thread.started event
}

// ParseLine parses a single JSON line from Codex's --json output and sends
// StreamEvent(s) to the provided channel.
func (p *CodexStreamParser) ParseLine(line string, ch chan<- StreamEvent) {
	var msg CodexStreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		slog.Debug("codex stream: skipping unparseable line", "line", line, "error", err)
		return
	}

	switch msg.Type {
	case "thread.started":
		if msg.ThreadID != "" {
			p.threadID = msg.ThreadID
		}

	case "item.completed":
		if msg.Item == nil {
			return
		}
		switch msg.Item.Type {
		case "agent_message":
			text := msg.Item.Text
			if text == "" {
				return
			}
			// Codex uses \n\n to separate thinking from content in agent_message.text.
			// The thinking section is wrapped in <think>...</think> tags or appears
			// before the first \n\n delimiter. We split on the first \n\n that
			// separates thinking from the actual response.
			if idx := strings.Index(text, "\n\n"); idx >= 0 {
				thinking := text[:idx]
				content := text[idx+2:]
				if thinking != "" {
					ch <- StreamEvent{Type: "thinking", Content: thinking}
				}
				if content != "" {
					ch <- StreamEvent{Type: "content", Content: content}
				}
			} else {
				// No separator — entire text is content
				ch <- StreamEvent{Type: "content", Content: text}
			}

		case "command_execution":
			// Emit tool_use event for completed command execution
			input := msg.Item.Command
			if msg.Item.AggregatedOutput != "" {
				input = fmt.Sprintf("%s\n\nOutput:\n%s", msg.Item.Command, msg.Item.AggregatedOutput)
			}
			ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
				Name:  "command_execution",
				ID:    msg.Item.ID,
				Input: input,
				Done:  true,
			}}
		}

	case "item.started":
		if msg.Item == nil {
			return
		}
		if msg.Item.Type == "command_execution" {
			ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
				Name:  "command_execution",
				ID:    msg.Item.ID,
				Input: msg.Item.Command,
				Done:  false,
			}}
		}

	case "turn.completed":
		meta := &Metadata{
			SessionID: p.threadID,
		}
		if msg.Usage != nil {
			meta.InputTokens = msg.Usage.InputTokens
			meta.OutputTokens = msg.Usage.OutputTokens
		}
		ch <- StreamEvent{Type: "metadata", Meta: meta}
		ch <- StreamEvent{Type: "done"}

	case "turn.started":
		// Structural event — no content

	default:
		slog.Debug("codex stream: skipping unknown message type", "type", msg.Type)
	}
}

// buildCodexStreamArgs constructs the CLI arguments for Codex streaming.
// The command field (e.g., "codex --profile m27") is split into binary + base args,
// then Codex-specific arguments are appended.
func buildCodexStreamArgs(req ChatRequest) []string {
	// Start with base command args from the command field
	var args []string

	// New session: codex exec --json ...
	args = append(args, "exec", "--json", "--dangerously-bypass-approvals-and-sandbox")

	// Working directory
	if req.WorkDir != "" {
		args = append(args, "-C", req.WorkDir)
	}

	// Model override
	if req.Model != "" {
		args = append(args, "-m", req.Model)
	}

	// Skip git repo check (allows running in non-git dirs)
	args = append(args, "--skip-git-repo-check")

	// Prompt is the last argument for new sessions
	args = append(args, req.Prompt)

	return args
}

// buildCodexResumeArgs constructs the CLI arguments for resuming a Codex session
func buildCodexResumeArgs(req ChatRequest, threadID string) []string {
	var args []string

	// Resume: codex exec resume --json <thread_id> <prompt>
	args = append(args, "exec", "resume", "--json", "--dangerously-bypass-approvals-and-sandbox")

	// Thread ID for resuming
	args = append(args, threadID)

	// Prompt for the resumed session
	args = append(args, req.Prompt)

	return args
}

// ExecuteStream runs the Codex CLI in streaming mode and returns a channel of events
func (c *CodexBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	// Parse command field: "codex --profile m27" -> binary="codex", baseArgs=["--profile","m27"]
	cmdBinary := "codex"
	var baseArgs []string
	if req.Command != "" {
		parts := strings.Fields(req.Command)
		if len(parts) > 0 {
			cmdBinary = parts[0]
			if len(parts) > 1 {
				baseArgs = parts[1:]
			}
		}
	}

	// Determine if we're resuming a session
	var codexArgs []string
	if req.Resume && req.SessionID != "" {
		// For resume, SessionID contains the Codex thread_id (stored as external_session_id)
		codexArgs = buildCodexResumeArgs(req, req.SessionID)
	} else {
		codexArgs = buildCodexStreamArgs(req)
	}

	// Combine: baseArgs + codexArgs
	fullArgs := append(baseArgs, codexArgs...)

	cmd := exec.CommandContext(ctx, cmdBinary, fullArgs...)
	cmd.Dir = req.WorkDir
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	slog.Info("executing ai stream command",
		slog.String("backend", "codex"),
		slog.String("work_dir", req.WorkDir),
		slog.String("session_id", req.SessionID),
		slog.String("prompt", req.Prompt),
		slog.Bool("resume", req.Resume),
		slog.Any("args", fullArgs),
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex stream: failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codex stream: failed to start command: %w", err)
	}

	ch := make(chan StreamEvent, streamChanSize)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(stdoutPipe)
		buf := make([]byte, scannerInitial)
		scanner.Buffer(buf, scannerMax)

		parser := &CodexStreamParser{}
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and non-JSON lines (e.g., ANSI codes, progress bars)
			if line == "" || !strings.HasPrefix(line, "{") {
				slog.Debug("codex stream: skipping non-JSON line", "line", line)
				continue
			}

			slog.Debug("codex stream: raw line", "session_id", req.SessionID, "line", line)
			parser.ParseLine(line, ch)

			// Check context after parsing
			select {
			case <-ctx.Done():
				slog.Warn("codex stream: context cancelled",
					slog.String("session_id", req.SessionID),
				)
				return
			default:
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case ch <- StreamEvent{Type: "warning", Content: fmt.Sprintf("AI 输出解析错误: %v", err)}:
			case <-ctx.Done():
			}
		}

		if err := cmd.Wait(); err != nil {
			if ctx.Err() != nil {
				slog.Warn("codex stream: command cancelled",
					slog.String("session_id", req.SessionID),
					slog.String("ctx_err", ctx.Err().Error()),
					slog.String("stderr", stderrBuf.String()),
				)
				return
			}
			stderr := stderrBuf.String()
			slog.Error("codex stream: command exited abnormally",
				slog.String("session_id", req.SessionID),
				slog.String("exit_error", err.Error()),
				slog.String("stderr", stderr),
			)
			warnMsg := "AI 后端异常退出"
			if stderr != "" {
				warnMsg = fmt.Sprintf("AI 后端异常退出\n%s", stderr)
			}
			select {
			case ch <- StreamEvent{Type: "warning", Content: warnMsg}:
			case <-ctx.Done():
			}
		} else if stderrBuf.Len() > 0 {
			stderr := stderrBuf.String()
			slog.Warn("codex stream: command succeeded with stderr output",
				slog.String("session_id", req.SessionID),
				slog.String("stderr", stderr),
			)
			select {
			case ch <- StreamEvent{Type: "warning", Content: stderr}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

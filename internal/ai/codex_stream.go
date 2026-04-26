package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// CodexStreamMessage represents a single JSON line from `codex exec --json`
type CodexStreamMessage struct {
	Type     string          `json:"type"`
	ThreadID string          `json:"thread_id,omitempty"`
	Message  string          `json:"message,omitempty"` // error message
	Error    *CodexError     `json:"error,omitempty"`   // turn.failed error
	Item     *CodexItem      `json:"item,omitempty"`
	Usage    *CodexUsage     `json:"usage,omitempty"`
}

type CodexError struct {
	Message string `json:"message"`
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

	case "error":
		if msg.Message != "" {
			ch <- StreamEvent{Type: "warning", Content: msg.Message}
		}

	case "turn.failed":
		errMsg := "AI 请求失败"
		if msg.Error != nil && msg.Error.Message != "" {
			errMsg = msg.Error.Message
		}
		ch <- StreamEvent{Type: "error", Error: errMsg}
		ch <- StreamEvent{Type: "done"}

	default:
		slog.Debug("codex stream: skipping unknown message type", "type", msg.Type)
	}
}

// buildCodexStreamArgs constructs the CLI arguments for Codex streaming.
// Note: returns args WITHOUT the "exec" prefix (it's added by ExecuteStream).
func buildCodexStreamArgs(req ChatRequest) []string {
	var args []string

	// New session: --json ...
	args = append(args, "--json", "--dangerously-bypass-approvals-and-sandbox")

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

// parseCodexResumeOutput parses plain text output from "codex exec resume".
// Output format:
//
//	OpenAI Codex v0.57.0 (research preview)
//	--------
//	workdir: ...
//	model: ...
//	--------
//	user
//	<prompt>
//	codex
//	<thinking block>
//	<response content>
//	exec
//	<command> in <dir> [succeeded|failed] in <time>:
//	<output>
//	codex
//	<thinking block>
//	<response content>
func parseCodexResumeOutput(scanner *bufio.Scanner, ch chan<- StreamEvent, sessionID string) {
	role := "" // current role: "codex" or "exec"
	inThinking := false
	var thinkingBuf strings.Builder
	var execOutput strings.Builder
	var execCommand string
	var execID string
	var execCounter int

	for scanner.Scan() {
		line := scanner.Text()

		// Handle ERROR lines from codex resume output
		if strings.HasPrefix(line, "ERROR:") {
			errMsg := strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
			if errMsg != "" {
				ch <- StreamEvent{Type: "error", Error: errMsg}
				return
			}
		}

		// Detect role markers
		if line == "codex" || line == "user" {
			// Flush any pending exec block
			if role == "exec" && execCommand != "" {
				ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
					Name:  "command_execution",
					ID:    execID,
					Input: execCommandCompleteJSON(execCommand, execOutput.String()),
					Done:  true,
				}}
				execCommand = ""
				execOutput.Reset()
			}
			role = line
			continue
		}

		if line == "exec" {
			// Flush any pending exec block
			if role == "exec" && execCommand != "" {
				ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
					Name:  "command_execution",
					ID:    execID,
					Input: execCommandCompleteJSON(execCommand, execOutput.String()),
					Done:  true,
				}}
				execCommand = ""
				execOutput.Reset()
			}
			role = "exec"
			continue
		}

		// Skip everything before first codex marker (header + user section)
		if role == "" || role == "user" {
			continue
		}

		// Handle codex role: thinking blocks and content
		if role == "codex" {
			// Track thinking blocks: <think> starts thinking, </think> ends thinking
			if strings.HasPrefix(line, "<think>") {
				inThinking = true
				thinkingBuf.Reset()
				// If there's text after <think> on the same line, capture it
				rest := strings.TrimPrefix(line, "<think>")
				if rest != "" {
					thinkingBuf.WriteString(rest)
				}
				continue
			}
			if strings.HasPrefix(line, "</think>") {
				if inThinking {
					inThinking = false
					if thinking := thinkingBuf.String(); thinking != "" {
						ch <- StreamEvent{Type: "thinking", Content: thinking}
					}
					thinkingBuf.Reset()
				}
				continue
			}
			if inThinking {
				if thinkingBuf.Len() > 0 {
					thinkingBuf.WriteByte('\n')
				}
				thinkingBuf.WriteString(line)
				continue
			}
			if line != "" {
				ch <- StreamEvent{Type: "content", Content: line + "\n"}
			}
			continue
		}

	// Handle exec role: command line and output
	if role == "exec" {
		if execCommand == "" {
			// First line is the command summary, e.g.:
			// "bash -c 'ls -1 /tmp | wc -l' in /tmp succeeded in 14ms:"
			execCommand = strings.TrimSuffix(line, ":")
			execID = fmt.Sprintf("exec-%d", execCounter)
			execCounter++
			ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
				Name:  "command_execution",
				ID:    execID,
				Input: execCommandSummaryJSON(execCommand),
				Done:  false,
			}}
		} else {
			if execOutput.Len() > 0 {
				execOutput.WriteByte('\n')
		}
			execOutput.WriteString(line)
		}
		continue
	}
	}

	// Flush any pending exec block at EOF
	if role == "exec" && execCommand != "" {
		ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
			Name:  "command_execution",
			ID:    execID,
			Input: execCommandCompleteJSON(execCommand, execOutput.String()),
			Done:  true,
		}}
	}
	// Also flush exec blocks when role changes (codex/exec marker)
	// These are handled inline above.

	// Flush remaining thinking
	if inThinking && thinkingBuf.Len() > 0 {
		ch <- StreamEvent{Type: "thinking", Content: thinkingBuf.String()}
	}

	// Send metadata and done events
	ch <- StreamEvent{Type: "metadata", Meta: &Metadata{SessionID: sessionID}}
	ch <- StreamEvent{Type: "done"}
}

// execCommandSummaryJSON returns a JSON string for a tool_use started event.
// The input contains just the command summary line.
func execCommandSummaryJSON(summary string) string {
	b, _ := json.Marshal(map[string]string{"command": summary})
	return string(b)
}

// execCommandCompleteJSON returns a JSON string for a tool_use completed event.
// The input contains the command summary and optional output.
func execCommandCompleteJSON(summary, output string) string {
	m := map[string]string{"command": summary}
	if output != "" {
		m["output"] = output
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// buildCodexResumeArgs constructs the CLI arguments for resuming a Codex session.
// "codex exec resume" does not support --json, so output is plain text.
// We pass -c config overrides to restore model/provider from the original session.
// Note: returns args WITHOUT the "exec" prefix (it's added by ExecuteStream).
func buildCodexResumeArgs(req ChatRequest, threadID string) []string {
	var args []string

	args = append(args, "resume")

	// Resume does not support --dangerously-bypass-approvals-and-sandbox,
	// use -c sandbox_permissions override instead.
	args = append(args, "-c", "sandbox_permissions=[\"disk-full-read-access\"]")

	// Restore model and provider via -c overrides (resume doesn't support -m/--profile)
	if req.Model != "" {
		args = append(args, "-c", fmt.Sprintf("model=%q", req.Model))
		args = append(args, "-c", "model_provider=minimax")
	}

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

	// Combine: "exec" + baseArgs (e.g. --profile m27) + codexArgs
	// --profile must come after "exec" (codex CLI quirk).
	fullArgs := make([]string, 0, 1+len(baseArgs)+len(codexArgs))
	fullArgs = append(fullArgs, "exec")
	fullArgs = append(fullArgs, baseArgs...)
	fullArgs = append(fullArgs, codexArgs...)

	cmd := exec.CommandContext(ctx, cmdBinary, fullArgs...)
	cmd.Dir = req.WorkDir
	cmd.Env = os.Environ() // inherit current environment (includes API keys)

	isResume := req.Resume && req.SessionID != ""

	// For resume mode, codex outputs the formatted transcript (with role markers
	// like "codex", "user", "exec") to stderr, and only the bare content to stdout.
	// We need the role markers for parsing, so we pipe stderr in resume mode.
	var stderrBuf bytes.Buffer
	var stderrPipe io.ReadCloser
	if isResume {
		var err error
		stderrPipe, err = cmd.StderrPipe()
		if err != nil {
			return nil, fmt.Errorf("codex stream: failed to create stderr pipe: %w", err)
		}
	} else {
		cmd.Stderr = &stderrBuf
	}

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

		if isResume {
			// Resume mode: "codex exec resume" outputs the formatted transcript
			// (with role markers like "codex", "user", "exec") to stderr.
			// We parse stderr to extract content, thinking, and tool_use events.
			scanner := bufio.NewScanner(stderrPipe)
			buf := make([]byte, scannerInitial)
			scanner.Buffer(buf, scannerMax)
			parseCodexResumeOutput(scanner, ch, req.SessionID)
		} else {
			// New session with --json: parse JSONL from stdout
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
		}

		if err := cmd.Wait(); err != nil {
			if ctx.Err() != nil {
				slog.Warn("codex stream: command cancelled",
					slog.String("session_id", req.SessionID),
					slog.String("ctx_err", ctx.Err().Error()),
				)
				return
			}
			slog.Error("codex stream: command exited abnormally",
				slog.String("session_id", req.SessionID),
				slog.String("exit_error", err.Error()),
			)
			select {
			case ch <- StreamEvent{Type: "warning", Content: "AI 后端异常退出"}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

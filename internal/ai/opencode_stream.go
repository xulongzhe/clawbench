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

// OpenCodeStreamMessage represents a single JSON line from `opencode run --format json`
type OpenCodeStreamMessage struct {
	Type      string          `json:"type"`       // "step_start", "text", "reasoning", "tool_use", "step_finish"
	Timestamp float64         `json:"timestamp"`
	SessionID string          `json:"sessionID"`
	Part      json.RawMessage `json:"part"`       // Varies by type — parse separately
}

// OpenCodeTextPart is the part for type="text" and type="reasoning" messages
type OpenCodeTextPart struct {
	Type string `json:"type"` // "text" or "reasoning"
	Text string `json:"text"`
}

// OpenCodeToolPart is the part for type="tool_use" messages
type OpenCodeToolPart struct {
	Type   string             `json:"type"`  // "tool"
	Tool   string             `json:"tool"`
	CallID string             `json:"callID"`
	State  *OpenCodeToolState `json:"state"`
}

// OpenCodeToolState holds tool execution status and I/O
type OpenCodeToolState struct {
	Status string          `json:"status"`  // "completed", "running"
	Input  json.RawMessage `json:"input"`
	Output string          `json:"output"`
}

// OpenCodeFinishPart is the part for type="step_finish" messages
type OpenCodeFinishPart struct {
	Reason string          `json:"reason"`  // "stop" or "tool-calls"
	Tokens *OpenCodeTokens `json:"tokens,omitempty"`
	Cost   float64         `json:"cost"`
}

// OpenCodeTokens holds token usage from step_finish
type OpenCodeTokens struct {
	Total     int `json:"total"`
	Input     int `json:"input"`
	Output    int `json:"output"`
	Reasoning int `json:"reasoning"`
}

// OpenCodeStreamParser parses JSON Lines output from `opencode run --format json`.
// It is separate from the shared StreamParser because OpenCode's format is
// fundamentally different (different top-level types, nesting in "part", multi-step lifecycle).
type OpenCodeStreamParser struct {
	sessionID string // captured from any message that has a sessionID
}

// ParseLine parses a single JSON line from OpenCode's stream-json output and sends
// StreamEvent(s) to the provided channel.
func (p *OpenCodeStreamParser) ParseLine(line string, ch chan<- StreamEvent) {
	var msg OpenCodeStreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		slog.Debug("opencode stream: skipping unparseable line", "line", line, "error", err)
		return
	}

	// Capture session ID from any message that has one
	if msg.SessionID != "" {
		p.sessionID = msg.SessionID
	}

	switch msg.Type {
	case "text":
		var part OpenCodeTextPart
		if err := json.Unmarshal(msg.Part, &part); err != nil {
			slog.Debug("opencode stream: skipping unparseable text part", "error", err)
			return
		}
		if part.Text == "" {
			return
		}
		// Strip the leading \n\n that OpenCode prepends to text responses
		text := strings.TrimPrefix(part.Text, "\n\n")
		if text != "" {
			ch <- StreamEvent{Type: "content", Content: text}
		}

	case "reasoning":
		var part OpenCodeTextPart // same structure as text
		if err := json.Unmarshal(msg.Part, &part); err != nil {
			slog.Debug("opencode stream: skipping unparseable reasoning part", "error", err)
			return
		}
		if part.Text == "" {
			return
		}
		text := strings.TrimPrefix(part.Text, "\n\n")
		if text != "" {
			ch <- StreamEvent{Type: "thinking", Content: text}
		}

	case "tool_use":
		var part OpenCodeToolPart
		if err := json.Unmarshal(msg.Part, &part); err != nil {
			slog.Debug("opencode stream: skipping unparseable tool_use part", "error", err)
			return
		}
		inputStr := "{}"
		if part.State != nil && len(part.State.Input) > 0 {
			inputStr = string(part.State.Input)
		}
		done := part.State != nil && part.State.Status == "completed"
		ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
			Name:  part.Tool,
			ID:    part.CallID,
			Input: inputStr,
			Done:  done,
		}}

	case "step_finish":
		var part OpenCodeFinishPart
		if err := json.Unmarshal(msg.Part, &part); err != nil {
			slog.Debug("opencode stream: skipping unparseable step_finish part", "error", err)
			return
		}
		if part.Reason == "stop" {
			meta := &Metadata{
				SessionID:  p.sessionID,
				StopReason: "stop",
				CostUSD:    part.Cost,
			}
			if part.Tokens != nil {
				meta.InputTokens = part.Tokens.Input
				meta.OutputTokens = part.Tokens.Output
			}
			ch <- StreamEvent{Type: "metadata", Meta: meta}
			ch <- StreamEvent{Type: "done"}
		}
		// reason="tool-calls" means more steps coming — no event

	case "step_start":
		// Structural — no event

	default:
		slog.Debug("opencode stream: skipping unknown message type", "type", msg.Type)
	}
}

// buildOpenCodeStreamArgs constructs the CLI arguments for OpenCode streaming
func buildOpenCodeStreamArgs(req ChatRequest) []string {
	args := []string{
		"run",
		req.Prompt,
		"--format", "json",
		"--dangerously-skip-permissions",
	}

	// Pass OpenCode session ID for continuing conversations.
	// Only pass --session when resuming an existing OpenCode session
	// (indicated by Resume=true and a ses_ prefixed session ID).
	// On first message, SessionID contains ClawBench's UUID which OpenCode
	// doesn't recognize — let OpenCode create its own session.
	if req.SessionID != "" && req.Resume {
		args = append(args, "--session", req.SessionID)
	}

	// Working directory
	if req.WorkDir != "" {
		args = append(args, "--dir", req.WorkDir)
	}

	// Model override (format: provider/model, e.g., "minimax-cn-coding-plan/MiniMax-M2.7")
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	return args
}

// ExecuteStream runs the OpenCode CLI in streaming mode and returns a channel of events
func (o *OpenCodeBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	args := buildOpenCodeStreamArgs(req)

	cmdName := req.Command
	if cmdName == "" {
		cmdName = "opencode"
	}
	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = req.WorkDir
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	slog.Info("executing ai stream command",
		slog.String("backend", "opencode"),
		slog.String("work_dir", req.WorkDir),
		slog.String("session_id", req.SessionID),
		slog.String("prompt", req.Prompt),
		slog.Any("args", args),
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("opencode stream: failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("opencode stream: failed to start command: %w", err)
	}

	ch := make(chan StreamEvent, streamChanSize)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(stdoutPipe)
		buf := make([]byte, scannerInitial)
		scanner.Buffer(buf, scannerMax)

		parser := &OpenCodeStreamParser{}
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and plugin prefix lines (e.g., "[opencode-mobile] v1.4.0")
			if line == "" || strings.HasPrefix(line, "[opencode-mobile]") {
				continue
			}
			// Skip other non-JSON prefix lines from plugins
			if !strings.HasPrefix(line, "{") {
				slog.Debug("opencode stream: skipping non-JSON line", "line", line)
				continue
			}

			slog.Debug("opencode stream: raw line", "session_id", req.SessionID, "line", line)
			parser.ParseLine(line, ch)

			// Check context after parsing
			select {
			case <-ctx.Done():
				slog.Warn("opencode stream: context cancelled",
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
				slog.Warn("opencode stream: command cancelled",
					slog.String("session_id", req.SessionID),
					slog.String("ctx_err", ctx.Err().Error()),
					slog.String("stderr", stderrBuf.String()),
				)
				return
			}
			stderr := stderrBuf.String()
			slog.Error("opencode stream: command exited abnormally",
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
			slog.Warn("opencode stream: command succeeded with stderr output",
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

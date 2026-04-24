package ai

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// buildCodebuddyStreamArgs constructs the CLI arguments for Codebuddy streaming
func buildCodebuddyStreamArgs(req ChatRequest) []string {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--session-id", req.SessionID,
		"--add-dir", req.WorkDir,
		"--dangerously-skip-permissions",
		"--disallowedTools", "CronCreate", "CronDelete", "CronList", "ToolSearch", "DeferExecuteTool",
	}

	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	// Pass model name: per-request override takes priority
	modelName := req.Model
	if modelName == "" {
		// No model specified, use default from agent configuration
		// This should have been set by the caller based on agent config
		modelName = "glm-5.1"
	}
	if modelName != "" {
		args = append(args, "--model", modelName)
	}

	// Prompt should be the last argument
	args = append(args, req.Prompt)

	return args
}

// ExecuteStream runs the Codebuddy CLI in streaming mode and returns a channel of events
func (c *CodebuddyBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	args := buildCodebuddyStreamArgs(req)

	cmd := exec.CommandContext(ctx, "codebuddy", args...)
	cmd.Dir = req.WorkDir
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	slog.Info("executing ai stream command",
		slog.String("backend", "codebuddy"),
		slog.String("work_dir", req.WorkDir),
		slog.String("session_id", req.SessionID),
		slog.String("prompt", req.Prompt),
		slog.Any("args", args),
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codebuddy stream: failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codebuddy stream: failed to start command: %w", err)
	}

	ch := make(chan StreamEvent, streamChanSize)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(stdoutPipe)
		buf := make([]byte, scannerInitial)
		scanner.Buffer(buf, scannerMax)

		parser := &StreamParser{}
		for scanner.Scan() {
			line := scanner.Text()

			// Codebuddy-specific: remove UTF-8 BOM prefix
			line = strings.TrimPrefix(line, "\xEF\xBB\xBF")

			if line == "" {
				continue
			}

			slog.Debug("codebuddy stream: raw line", "session_id", req.SessionID, "line", line)
			parser.ParseLine(line, ch)

			// Check context after parsing
			select {
			case <-ctx.Done():
				slog.Warn("codebuddy stream: context cancelled",
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
				slog.Warn("codebuddy stream: command cancelled",
					slog.String("session_id", req.SessionID),
					slog.String("ctx_err", ctx.Err().Error()),
					slog.String("stderr", stderrBuf.String()),
				)
				return
			}
			stderr := stderrBuf.String()
			slog.Error("codebuddy stream: command exited abnormally",
				slog.String("session_id", req.SessionID),
				slog.String("exit_error", err.Error()),
				slog.String("stderr", stderr),
			)
			// Build user-facing warning message
			warnMsg := "AI 后端异常退出"
			if stderr != "" {
				warnMsg = fmt.Sprintf("AI 后端异常退出\n%s", stderr)
			}
			select {
			case ch <- StreamEvent{Type: "warning", Content: warnMsg}:
			case <-ctx.Done():
			}
		} else if stderrBuf.Len() > 0 {
			// Command succeeded but stderr has output (warnings, diagnostics, etc.)
			stderr := stderrBuf.String()
			slog.Warn("codebuddy stream: command succeeded with stderr output",
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

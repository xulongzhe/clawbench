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

// CLIBackend is a generic AI backend that shells out to a CLI tool and streams
// JSON output. It implements the AIBackend interface via callbacks for
// backend-specific behavior.
type CLIBackend struct {
	name           string
	defaultCommand string
	buildArgs      func(req ChatRequest) []string
	newParser      func() LineParser
	filterLine     func(line string) (string, bool) // nil = skip empty lines only
	preStart       func(cmd *exec.Cmd, req ChatRequest) // optional, e.g. Claude stdin
}

// Name returns the backend identifier.
func (b *CLIBackend) Name() string {
	return b.name
}

// ExecuteStream runs the CLI backend in streaming mode and returns a channel of events.
func (b *CLIBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	args := b.buildArgs(req)

	cmdName := req.Command
	if cmdName == "" {
		cmdName = b.defaultCommand
	}
	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = req.WorkDir
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if b.preStart != nil {
		b.preStart(cmd, req)
	}

	slog.Info("executing ai stream command",
		slog.String("backend", b.name),
		slog.String("work_dir", req.WorkDir),
		slog.String("session_id", req.SessionID),
		slog.String("prompt", req.Prompt),
		slog.Any("args", args),
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%s stream: failed to create stdout pipe: %w", b.name, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s stream: failed to start command: %w", b.name, err)
	}

	ch := make(chan StreamEvent, streamChanSize)

	// Collect raw stdout lines for debugging/analysis
	var rawLines strings.Builder

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(stdoutPipe)
		buf := make([]byte, scannerInitial)
		scanner.Buffer(buf, scannerMax)

		parser := b.newParser()
		for scanner.Scan() {
			line := scanner.Text()

			// Filter lines based on backend-specific logic
			if b.filterLine != nil {
				filtered, ok := b.filterLine(line)
				if !ok {
					continue
				}
				line = filtered
			} else {
				// Default: skip empty lines
				if line == "" {
					continue
				}
			}

			// Collect raw line for debugging
			if rawLines.Len() > 0 {
				rawLines.WriteByte('\n')
			}
			rawLines.WriteString(line)

			// Check if this is the final "result" line — send raw_output
			// before parsing so the handler receives it before the "done" event.
			if strings.HasPrefix(line, `{"type":"result"`) {
				select {
				case ch <- StreamEvent{Type: "raw_output", RawOutput: rawLines.String()}:
				default:
				}
			}

			slog.Debug(b.name+" stream: raw line", "session_id", req.SessionID, "line", line)
			parser.ParseLine(line, ch)

			// Check context after parsing
			select {
			case <-ctx.Done():
				slog.Warn(b.name+" stream: context cancelled",
					slog.String("session_id", req.SessionID),
				)
				// Send raw output before returning so it's available for debugging
				if rawLines.Len() > 0 {
					select {
					case ch <- StreamEvent{Type: "raw_output", RawOutput: rawLines.String()}:
					default:
					}
				}
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
				slog.Warn(b.name+" stream: command cancelled",
					slog.String("session_id", req.SessionID),
					slog.String("ctx_err", ctx.Err().Error()),
					slog.String("stderr", stderrBuf.String()),
				)
				// Send raw output before returning
				if rawLines.Len() > 0 {
					select {
					case ch <- StreamEvent{Type: "raw_output", RawOutput: rawLines.String()}:
					default:
					}
				}
				return
			}
			stderr := stderrBuf.String()
			slog.Error(b.name+" stream: command exited abnormally",
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
			slog.Warn(b.name+" stream: command succeeded with stderr output",
				slog.String("session_id", req.SessionID),
				slog.String("stderr", stderr),
			)
			select {
			case ch <- StreamEvent{Type: "warning", Content: stderr}:
			case <-ctx.Done():
			}
		}

		// Send raw output event after all other events
		if rawLines.Len() > 0 {
			select {
			case ch <- StreamEvent{Type: "raw_output", RawOutput: rawLines.String()}:
			default:
			}
		}
	}()

	return ch, nil
}

// defaultFilterEmpty returns a filterLine that skips empty lines.
// This is the default behavior when filterLine is nil.
func defaultFilterEmpty() func(string) (string, bool) {
	return func(line string) (string, bool) {
		if line == "" {
			return "", false
		}
		return line, true
	}
}

// filterSkipNonJSON returns a filterLine that skips empty lines and lines
// that don't start with '{'.
func filterSkipNonJSON() func(string) (string, bool) {
	return func(line string) (string, bool) {
		if line == "" || !strings.HasPrefix(line, "{") {
			return "", false
		}
		return line, true
	}
}

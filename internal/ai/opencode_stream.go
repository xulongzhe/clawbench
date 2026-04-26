package ai

import (
	"encoding/json"
	"log/slog"
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

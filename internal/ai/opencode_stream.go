package ai

import (
	"encoding/json"
	"fmt"
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

// GetCapturedSessionID returns the OpenCode session ID (ses_xxx) captured from
// parsed stream messages. Available as soon as the first message (e.g., step_start) is parsed.
func (p *OpenCodeStreamParser) GetCapturedSessionID() string { return p.sessionID }

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
			// Normalize input field names from OpenCode's camelCase to canonical snake_case
			inputStr = normalizeOpenCodeInput(part.Tool, part.State.Input)
		}
		done := part.State != nil && part.State.Status == "completed"
		ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
			Name:  normalizeOpenCodeToolName(part.Tool),
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
	// Prompt: prepend system prompt if set.
	// OpenCode CLI has no --system-prompt flag, so injecting the system prompt
	// into the user prompt is the only way to pass it through.
	prompt := req.Prompt
	if req.SystemPrompt != "" {
		prompt = fmt.Sprintf("[System Instructions: %s]\n\n%s", req.SystemPrompt, prompt)
	}

	args := []string{
		"run",
		prompt,
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

// normalizeOpenCodeToolName maps OpenCode tool names to canonical names.
// OpenCode uses lowercase tool names (read, bash, edit, write).
func normalizeOpenCodeToolName(name string) string {
	switch name {
	case "read":
		return "Read"
	case "write":
		return "Write"
	case "edit":
		return "Edit"
	case "bash":
		return "Bash"
	case "glob":
		return "Glob"
	case "grep":
		return "Grep"
	case "ls":
		return "LS"
	case "webfetch":
		return "WebFetch"
	case "websearch":
		return "WebSearch"
	case "skill":
		return "Skill"
	case "task":
		return "Agent" // OpenCode's task tool is a sub-agent
	case "todowrite":
		return "TodoWrite"
	case "look_at":
		return "Read" // media inspection → Read
	default:
		return name
	}
}

// normalizeOpenCodeInput remaps OpenCode's camelCase input fields to canonical snake_case.
// OpenCode uses filePath instead of file_path, oldString instead of old_string, etc.
func normalizeOpenCodeInput(toolName string, rawInput json.RawMessage) string {
	var input map[string]any
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return string(rawInput) // fallback: return as-is
	}

	// Remap camelCase keys to canonical snake_case
	if v, ok := input["filePath"]; ok {
		delete(input, "filePath")
		input["file_path"] = v
	}
	if v, ok := input["oldString"]; ok {
		delete(input, "oldString")
		input["old_string"] = v
	}
	if v, ok := input["newString"]; ok {
		delete(input, "newString")
		input["new_string"] = v
	}

	normalized, err := json.Marshal(input)
	if err != nil {
		return string(rawInput)
	}
	return string(normalized)
}

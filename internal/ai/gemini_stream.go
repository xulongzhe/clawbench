package ai

import (
	"encoding/json"
	"log/slog"
)

// GeminiStreamMessage represents a single JSON line from `gemini --output-format stream-json`
// Fields are shared across event types — only relevant fields are populated per type.
type GeminiStreamMessage struct {
	Type      string `json:"type"`       // "init", "message", "tool_use", "tool_result", "error", "result"
	Timestamp string `json:"timestamp"`  // ISO 8601
	SessionID string `json:"session_id"` // from init event
	Model     string `json:"model"`      // from init event

	// message event fields
	Role    string `json:"role"`    // "user" | "assistant"
	Content string `json:"content"` // text content
	Delta   bool   `json:"delta"`   // true for incremental assistant messages

	// tool_use / tool_result shared field
	ToolID string `json:"tool_id"` // tool call ID

	// tool_use event fields
	ToolName   string          `json:"tool_name"`  // tool name
	Parameters json.RawMessage `json:"parameters"` // tool input parameters

	// tool_result / result shared field
	Status string `json:"status"` // "success" | "error"

	// tool_result event fields
	ToolOutput string `json:"output"` // display output

	// error event fields (also used by tool_result for error details)
	Severity string `json:"severity"` // "warning" | "error"
	Message  string `json:"message"`  // error message

	// result event fields
	Error  *GeminiResultError `json:"error"`  // only when status="error"
	Stats  *GeminiStreamStats `json:"stats"`
}

// GeminiResultError represents the error field in a result event
type GeminiResultError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GeminiStreamStats represents the stats field in a result event
type GeminiStreamStats struct {
	TotalTokens  int                          `json:"total_tokens"`
	InputTokens  int                          `json:"input_tokens"`
	OutputTokens int                          `json:"output_tokens"`
	Cached       int                          `json:"cached"`
	Input        int                          `json:"input"`
	DurationMs   int                          `json:"duration_ms"`
	ToolCalls    int                          `json:"tool_calls"`
	Models       map[string]GeminiModelTokens `json:"models"`
}

// GeminiModelTokens represents per-model token usage
type GeminiModelTokens struct {
	TotalTokens  int `json:"total_tokens"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Cached       int `json:"cached"`
	Input        int `json:"input"`
}

// GeminiStreamParser parses JSON Lines output from `gemini --output-format stream-json`
type GeminiStreamParser struct {
	sessionID string // captured from init event
	model     string // captured from init event
}

// ParseLine parses a single JSON line from Gemini's stream-json output and sends
// StreamEvent(s) to the provided channel.
func (p *GeminiStreamParser) ParseLine(line string, ch chan<- StreamEvent) {
	var msg GeminiStreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		slog.Debug("gemini stream: skipping unparseable line", "line", line, "error", err)
		return
	}

	switch msg.Type {
	case "init":
		// Capture session ID and model from init event
		if msg.SessionID != "" {
			p.sessionID = msg.SessionID
		}
		if msg.Model != "" {
			p.model = msg.Model
		}

	case "message":
		if msg.Role == "assistant" && msg.Content != "" {
			ch <- StreamEvent{Type: "content", Content: msg.Content}
		}
		// Skip user messages — they echo back the input prompt

	case "tool_use":
		inputStr := "{}"
		if len(msg.Parameters) > 0 {
			inputStr = string(msg.Parameters)
		}
		ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
			Name:  msg.ToolName,
			ID:    msg.ToolID,
			Input: inputStr,
			Done:  true, // Gemini sends full tool input in one event
		}}

	case "tool_result":
		// Tool results are informational — no event needed
		// The tool_use event already has the tool call details

	case "error":
		// Emit as warning for severity="warning", error for "error"
		if msg.Message != "" {
			if msg.Severity == "error" {
				ch <- StreamEvent{Type: "error", Error: msg.Message}
			} else {
				ch <- StreamEvent{Type: "warning", Content: msg.Message}
			}
		}

	case "result":
		meta := &Metadata{
			SessionID: p.sessionID,
			Model:     p.model,
		}
		if msg.Stats != nil {
			meta.InputTokens = msg.Stats.InputTokens
			meta.OutputTokens = msg.Stats.OutputTokens
			meta.DurationMs = msg.Stats.DurationMs
		}
		if msg.Status == "error" {
			meta.IsError = true
			if msg.Error != nil {
				meta.ErrorMessage = msg.Error.Message
			}
			if meta.ErrorMessage != "" {
				ch <- StreamEvent{Type: "warning", Content: meta.ErrorMessage}
			}
		} else {
			meta.StopReason = "stop"
		}
		ch <- StreamEvent{Type: "metadata", Meta: meta}
		ch <- StreamEvent{Type: "done"}

	default:
		slog.Debug("gemini stream: skipping unknown message type", "type", msg.Type)
	}
}

// buildGeminiStreamArgs constructs the CLI arguments for Gemini streaming
func buildGeminiStreamArgs(req ChatRequest) []string {
	args := []string{
		"--prompt", req.Prompt,
		"--output-format", "stream-json",
		"--yolo",
	}

	// Resume previous session
	if req.SessionID != "" && req.Resume {
		args = append(args, "--resume", "latest")
	}

	// Working directory — use --include-directories for additional dirs
	if req.WorkDir != "" {
		args = append(args, "--include-directories", req.WorkDir)
	}

	// Model override
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	return args
}


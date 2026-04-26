package ai

import (
	"encoding/json"
	"log/slog"
)

// LineParser is the interface for parsing individual JSON lines from CLI output.
type LineParser interface {
	ParseLine(line string, ch chan<- StreamEvent)
}

// ClaudeContentBlock represents a content block within a Claude stream message
type ClaudeContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
}

// ClaudeStreamMessageBody represents the message body within a Claude stream message
type ClaudeStreamMessageBody struct {
	Content []ClaudeContentBlock `json:"content"`
}

// ClaudeStreamUsage represents token usage in a stream message
type ClaudeStreamUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ClaudeStreamModelUsage represents per-model token usage
type ClaudeStreamModelUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// ClaudeStreamMessage represents a single message in the streaming JSON output
// from both Claude and Codebuddy CLIs (stream-json format).
type ClaudeStreamMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Text    string `json:"text"`

	// Nested message body for assistant messages (Claude verbose format)
	Message *ClaudeStreamMessageBody `json:"message,omitempty"`

	// Fields for result messages
	IsError      bool    `json:"is_error"`
	Result       string  `json:"result"`
	SessionID    string  `json:"session_id"`
	DurationMs   int     `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	StopReason   string  `json:"stop_reason"`

	// Usage fields (pointer so it can be nil)
	Usage *ClaudeStreamUsage `json:"usage,omitempty"`

	// Per-model usage (Claude-specific)
	ModelUsage map[string]ClaudeStreamModelUsage `json:"modelUsage,omitempty"`

	// Codebuddy-specific: providerData for model info
	ProviderData *struct {
		Model string `json:"model,omitempty"`
		Usage *struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"usage,omitempty"`
	} `json:"providerData,omitempty"`

	// stream_event fields (codebuddy --include-partial-messages)
	Event *StreamEventData `json:"event,omitempty"`
}

// StreamEventData represents the event field in a stream_event message
type StreamEventData struct {
	Type         string              `json:"type"`
	Index        int                 `json:"index,omitempty"`
	ContentBlock *StreamContentBlock `json:"content_block,omitempty"`
	Delta        *StreamDelta        `json:"delta,omitempty"`
	Message      *StreamMessageStart `json:"message,omitempty"`
}

// StreamContentBlock represents a content_block_start/stop payload
type StreamContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
	Name      string `json:"name,omitempty"`
	ID        string `json:"id,omitempty"`
}

// StreamDelta represents a content_block_delta payload
type StreamDelta struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

// StreamMessageStart represents the message field in a message_start event
type StreamMessageStart struct {
	Model string `json:"model"`
}

const (
	scannerInitial = 64 * 1024   // 64KB initial scanner buffer
	scannerMax     = 1024 * 1024 // 1MB max scanner buffer
	streamChanSize = 64          // channel buffer size
)

// StreamParser tracks state across stream lines to avoid duplicate content.
// It implements the LineParser interface and is used by both Claude and Codebuddy backends.
type StreamParser struct {
	// receivedPartial tracks whether we've seen stream_event content_block_delta,
	// so we can skip the full assistant message that follows
	receivedPartial bool
	// receivedPartialThinking tracks whether we've seen thinking_delta events,
	// so we can skip thinking blocks in the full assistant message
	receivedPartialThinking bool
	// model stores the model name extracted from message_start events
	model string
	// currentTool tracks the in-progress tool call
	currentTool *ToolCall
}

// ParseLine parses a single JSON line from stream-json output and sends
// StreamEvent(s) to the provided channel. This is the shared parser used by
// both Claude and Codebuddy streaming implementations.
func (p *StreamParser) ParseLine(line string, ch chan<- StreamEvent) {
	var msg ClaudeStreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		slog.Debug("stream: skipping unparseable line", "line", line, "error", err)
		return
	}

	switch msg.Type {
	case "assistant":
		// Claude verbose format: content blocks in msg.Message.Content
		if msg.Message != nil {
			for _, block := range msg.Message.Content {
				if block.Type == "tool_use" {
					// Emit tool_use event with full input from the complete message
					inputStr := string(block.Input)
					ch <- StreamEvent{Type: "tool_use", Tool: &ToolCall{
						Name:  block.Name,
						ID:    block.ID,
						Input: inputStr,
						Done:  true,
					}}
				} else if block.Type == "thinking" && block.Thinking != "" && !p.receivedPartialThinking {
					ch <- StreamEvent{Type: "thinking", Content: block.Thinking}
				} else if block.Type == "text" && block.Text != "" && !p.receivedPartial {
					ch <- StreamEvent{Type: "content", Content: block.Text}
				}
			}
			return
		}
		// If we already received partial content via stream_event, skip
		if p.receivedPartial {
			return
		}
		// Codebuddy format: simple text field
		if msg.Subtype == "text" && msg.Text != "" {
			ch <- StreamEvent{Type: "content", Content: msg.Text}
		}

	case "result":
		meta := &Metadata{
			SessionID:    msg.SessionID,
			DurationMs:   msg.DurationMs,
			CostUSD:      msg.TotalCostUSD,
			StopReason:   msg.StopReason,
			IsError:      msg.IsError,
		}
		if msg.Usage != nil {
			meta.InputTokens = msg.Usage.InputTokens
			meta.OutputTokens = msg.Usage.OutputTokens
		}
		// Use model from stream_event message_start if available
		if p.model != "" {
			meta.Model = p.model
		}
		// Claude-specific: extract model from ModelUsage
		for name, usage := range msg.ModelUsage {
			if meta.Model == "" {
				meta.Model = name
			}
			if meta.InputTokens == 0 && meta.OutputTokens == 0 {
				meta.InputTokens = usage.InputTokens
				meta.OutputTokens = usage.OutputTokens
			}
			break
		}
		// Codebuddy-specific: extract model from providerData
		if msg.ProviderData != nil {
			if meta.Model == "" {
				meta.Model = msg.ProviderData.Model
			}
			if msg.ProviderData.Usage != nil {
				if meta.InputTokens == 0 && meta.OutputTokens == 0 {
					meta.InputTokens = msg.ProviderData.Usage.InputTokens
					meta.OutputTokens = msg.ProviderData.Usage.OutputTokens
				}
			}
		}

		if msg.IsError {
			meta.ErrorMessage = msg.Result
			// Also emit warning event so error shows as warning block in chat message
			if msg.Result != "" {
				slog.Warn("stream: CLI returned is_error result", "result", msg.Result)
				ch <- StreamEvent{Type: "warning", Content: msg.Result}
			}
		}
		slog.Info("stream: emitting done event", "is_error", msg.IsError)
		ch <- StreamEvent{Type: "metadata", Meta: meta}
		ch <- StreamEvent{Type: "done"}

	case "system":
		// System messages (e.g., init, tool use) - skip

	case "stream_event":
		// Codebuddy --include-partial-messages: incremental token streaming
		if msg.Event == nil {
			return
		}
		switch msg.Event.Type {
		case "content_block_delta":
			if msg.Event.Delta == nil {
				return
			}
			switch msg.Event.Delta.Type {
			case "text_delta":
				if msg.Event.Delta.Text != "" {
					p.receivedPartial = true
					ch <- StreamEvent{Type: "content", Content: msg.Event.Delta.Text}
				}
			case "input_json_delta":
				// Accumulate tool input JSON
				if p.currentTool != nil {
					p.currentTool.Input += msg.Event.Delta.Text
				}
			case "thinking_delta":
				if msg.Event.Delta.Thinking != "" {
					p.receivedPartialThinking = true
					ch <- StreamEvent{Type: "thinking", Content: msg.Event.Delta.Thinking}
				}
			}
		case "content_block_start":
			if msg.Event.ContentBlock != nil && msg.Event.ContentBlock.Type == "tool_use" {
				p.currentTool = &ToolCall{
					Name: msg.Event.ContentBlock.Name,
					ID:   msg.Event.ContentBlock.ID,
				}
				ch <- StreamEvent{Type: "tool_use", Tool: p.currentTool}
			}
		case "content_block_stop":
			if p.currentTool != nil {
				p.currentTool.Done = true
				ch <- StreamEvent{Type: "tool_use", Tool: p.currentTool}
				p.currentTool = nil
			}
		case "message_start":
			// Extract model name from message_start
			if msg.Event.Message != nil && msg.Event.Message.Model != "" {
				p.model = msg.Event.Message.Model
			}
		case "message_delta", "message_stop":
			// Structural events - no content to emit
		}

	case "file-history-snapshot":
		// File history snapshot events - skip

	default:
		slog.Debug("stream: skipping unknown message type", "type", msg.Type)
	}
}

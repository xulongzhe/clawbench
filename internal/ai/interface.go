package ai

import (
	"context"
)

// ChatRequest represents a request to the AI backend
type ChatRequest struct {
	Prompt       string
	SessionID    string
	WorkDir      string
	SystemPrompt string
	Model        string // per-request model override (empty = use global default)
	Command      string // optional: custom command path for the AI backend CLI
	AgentID      string // agent ID for logging and persistence
	Resume       bool   // If true, resume an existing session instead of creating new
}

// Metadata contains additional information about the AI response
type Metadata struct {
	Model        string  `json:"model,omitempty"`
	InputTokens  int     `json:"inputTokens,omitempty"`
	OutputTokens int     `json:"outputTokens,omitempty"`
	DurationMs   int     `json:"durationMs,omitempty"`
	CostUSD      float64 `json:"costUsd,omitempty"`
	SessionID    string  `json:"sessionId,omitempty"`
	StopReason   string  `json:"stopReason,omitempty"`
	IsError      bool    `json:"isError,omitempty"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
}

// StreamEvent represents a single event in the streaming output
type StreamEvent struct {
	Type    string    // "content", "thinking", "metadata", "done", "error", "tool_use"
	Content string    // Incremental text (Type=content, Type=thinking)
	Meta    *Metadata // Metadata (Type=metadata)
	Error   string    // Error message (Type=error)
	Tool    *ToolCall // Tool call info (Type=tool_use)
}

// ToolCall represents a tool invocation by the AI
type ToolCall struct {
	Name   string // Tool name (e.g., "Read", "Bash", "Edit")
	ID     string // Tool call ID
	Input  string // Tool input (JSON string, accumulated incrementally)
	Done   bool   // Whether the tool call input is complete
}

// AIBackend defines the interface for AI backend implementations
type AIBackend interface {
	// Name returns the backend identifier (e.g., "claude", "codebuddy")
	Name() string

	// ExecuteStream runs the AI backend and returns a channel of streaming events
	ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}

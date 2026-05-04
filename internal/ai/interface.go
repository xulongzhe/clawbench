package ai

import (
	"context"

	"clawbench/internal/model"
)

// ChatRequest represents a request to the AI backend
type ChatRequest struct {
	Prompt             string
	SessionID          string
	WorkDir            string
	SystemPrompt       string
	Model              string // per-request model override (empty = use global default)
	Command            string // optional: custom command path for the AI backend CLI
	AgentID            string // agent ID for logging and persistence
	Resume             bool   // If true, resume an existing session instead of creating new
	ScheduledExecution bool   // If true, this is a scheduled task execution — block schedule-proposal creation
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

// Warning reason codes — used by frontend for i18n lookup and visual severity
const (
	ReasonDisconnect    = "disconnect"     // SSE client disconnected
	ReasonTimeout       = "timeout"        // AI response timeout
	ReasonUserCancel    = "user_cancel"    // User explicitly cancelled
	ReasonContextCancel = "context_cancel" // Context cancelled (generic interruption)
	ReasonEmpty         = "empty"          // AI returned no content
	ReasonParseError    = "parse_error"    // CLI output parsing error
	ReasonBackendExit   = "backend_exit"   // CLI process exited abnormally
	ReasonRequestFailed = "request_failed" // Codex turn.failed
	ReasonRestart       = "restart"        // Server restart, AI response interrupted
	ReasonPanic         = "panic"          // AI goroutine panicked
)

// StreamEvent represents a single event in the streaming output
type StreamEvent struct {
	Type       string          // "content", "thinking", "metadata", "done", "error", "tool_use", "raw_output", "resume_split", "queue_consume", "queue_update", "session_capture"
	Content    string          // Incremental text (Type=content, Type=thinking) or captured session ID (Type=session_capture)
	Reason     string          // Structured reason code for i18n (e.g. "disconnect", "timeout", "parse_error")
	Meta       *Metadata       // Metadata (Type=metadata)
	Error      string          // Error message (Type=error)
	Tool       *ToolCall       // Tool call info (Type=tool_use)
	RawOutput  string          // Raw stdout lines from AI backend (Type=raw_output)
	QueueEvent *QueueEventData // Queue data (Type=queue_consume, Type=queue_update)
}

// ToolCall represents a tool invocation by the AI.
// Each backend parser must normalize tool names and input field names
// to the canonical conventions before emitting ToolCall events:
//
//	Canonical tool names: Read, Write, Edit, Bash, Glob, Grep, LS, ...
//	Canonical input fields: file_path (not filePath), command, old_string, new_string, ...
type ToolCall struct {
	Name   string // Canonical tool name (e.g., "Read", "Bash", "Edit")
	ID     string // Tool call ID
	Input  string // Tool input (JSON string with canonical field names, accumulated incrementally)
	Done   bool   // Whether the tool call input is complete
}

// QueueEventData carries data for queue_consume and queue_update SSE events.
type QueueEventData struct {
	Text      string                `json:"text,omitempty"`
	FilePath  string                `json:"filePath,omitempty"`
	FilePaths []string              `json:"filePaths,omitempty"`
	Files     []string              `json:"files,omitempty"`
	Queue     []model.QueuedMessage `json:"queue,omitempty"`
}

// AIBackend defines the interface for AI backend implementations
type AIBackend interface {
	// Name returns the backend identifier (e.g., "claude", "codebuddy")
	Name() string

	// ExecuteStream runs the AI backend and returns a channel of streaming events
	ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}

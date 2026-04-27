package model

import "time"

// ChatMessage represents a single message in the chat history
type ChatMessage struct {
	ID        int64     `json:"id,omitempty"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	FilePath  string    `json:"filePath,omitempty"`
	Files     []string  `json:"files,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`
	Backend   string    `json:"backend,omitempty"`
	Streaming bool      `json:"streaming,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// ChatSession represents a chat session
type ChatSession struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Backend     string    `json:"backend"`
	AgentID     string    `json:"agentId,omitempty"`
	Model       string    `json:"model,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Running     bool      `json:"running,omitempty"`
	UnreadCount int       `json:"unreadCount,omitempty"`
	LastReadAt  *time.Time `json:"-"`
}

// ContentBlock represents a typed block within an assistant message's content.
// Stored as JSON in the chat_history.content column.
type ContentBlock struct {
	Type  string         `json:"type"`           // "thinking", "tool_use", "text"
	Text  string         `json:"text,omitempty"`  // thinking or text content
	Name  string         `json:"name,omitempty"`  // tool name (tool_use)
	ID    string         `json:"id,omitempty"`    // tool call ID (tool_use)
	Input map[string]any `json:"input,omitempty"` // tool input (tool_use)
	Done  bool           `json:"done,omitempty"`  // tool_use input complete (tool_use)
}

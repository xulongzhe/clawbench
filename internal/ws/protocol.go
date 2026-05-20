package ws

// ServerMessage is a message sent from server to client.
type ServerMessage struct {
	Type  string `json:"type"`            // "event", "ping"
	ID    string `json:"id,omitempty"`    // event ID for ack (e.g., "evt_1706000000_1")
	Event string `json:"event,omitempty"` // "session_update", "task_update", "queue_update"
	Data  any    `json:"data,omitempty"`
}

// ClientMessage is a message sent from client to server.
type ClientMessage struct {
	Type       string `json:"type"`                  // "ack", "pong", "register"
	ID         string `json:"id,omitempty"`          // ack target event ID
	PushRegID  string `json:"push_reg_id,omitempty"` // JPush registration ID (for "register" type)
}

// SessionUpdateData is the data payload for "session_update" events.
type SessionUpdateData struct {
	SessionID      string `json:"session_id"`
	Status         string `json:"status"`                    // "running", "completed", "cancelled"
	HasNewMessages bool   `json:"has_new_messages"`
	ResponsePreview string `json:"response_preview,omitempty"` // preview of AI's final reply (completed only)
	SessionTitle    string `json:"session_title,omitempty"`    // session title for push notification
}

// TaskUpdateData is the data payload for "task_update" events.
type TaskUpdateData struct {
	TaskID      string `json:"task_id"`
	Status      string `json:"status"`          // "running", "completed", "failed"
	ExecutionID string `json:"execution_id,omitempty"`
}

// QueueUpdateData is the data payload for "queue_update" events.
type QueueUpdateData struct {
	SessionID string `json:"session_id"`
	Count     int    `json:"count"`
}

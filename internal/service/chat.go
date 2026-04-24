package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"clawbench/internal/ai"
	"clawbench/internal/model"
)

// GetChatHistory retrieves all chat messages for a given project path, backend, and session.
func GetChatHistory(projectPath, backend, sessionID string) ([]model.ChatMessage, error) {
	messages := []model.ChatMessage{}
	rows, err := DB.Query(
		"SELECT id, role, content, file_path, files, backend, streaming, created_at FROM chat_history WHERE project_path = ? AND session_id = ? ORDER BY created_at ASC",
		projectPath, sessionID,
	)
	if err != nil {
		return messages, err
	}
	defer rows.Close()

	for rows.Next() {
		var msg model.ChatMessage
		var filesJSON sql.NullString
		var streaming int
		if err := rows.Scan(&msg.ID, &msg.Role, &msg.Content, &msg.FilePath, &filesJSON, &msg.Backend, &streaming, &msg.CreatedAt); err != nil {
			return nil, err
		}
		msg.Streaming = streaming != 0
		if filesJSON.Valid && filesJSON.String != "" {
			json.Unmarshal([]byte(filesJSON.String), &msg.Files)
		}
		msg.SessionID = sessionID
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// GetChatMessageCount returns the number of messages in a session.
func GetChatMessageCount(sessionID string) int {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM chat_history WHERE session_id = ?", sessionID).Scan(&count)
	return count
}

// AddChatMessage adds a message to the chat history for a given project path, backend, and session.
func AddChatMessage(projectPath, backend, sessionID, role, content, filePath string, files []string, streaming bool) (int64, error) {
	var filesJSON string
	if len(files) > 0 {
		data, _ := json.Marshal(files)
		filesJSON = string(data)
	}

	streamingInt := 0
	if streaming {
		streamingInt = 1
	}

	// Use transaction to ensure data consistency
	tx, err := DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO chat_history (project_path, backend, session_id, role, content, file_path, files, streaming) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		projectPath, backend, sessionID, role, content, filePath, filesJSON, streamingInt,
	)
	if err != nil {
		return 0, err
	}

	// Update session's updated_at timestamp
	_, err = tx.Exec("UPDATE chat_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", sessionID)
	if err != nil {
		return 0, err
	}

	// If this is the first user message, update session title
	if role == "user" {
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM chat_history WHERE session_id = ?", sessionID).Scan(&count)
		if err == nil && count == 1 {
		// Truncate title to 50 runes (characters), not bytes
		title := content
		if len(files) > 0 && title == "" {
			title = "文件消息"
		}
		if title == "" {
			title = "新会话"
		}
		runes := []rune(title)
		if len(runes) > 50 {
			title = string(runes[:50]) + "..."
		}
		_, err = tx.Exec("UPDATE chat_sessions SET title = ? WHERE id = ?", title, sessionID)
		if err != nil {
			return 0, err
		}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	messageID, _ := result.LastInsertId()
	return messageID, nil
}

// GetRecentProjects returns the most recent 10 project paths.
func GetRecentProjects() ([]string, error) {
	var paths []string
	rows, err := DB.Query("SELECT project_path FROM recent_projects ORDER BY accessed_at DESC LIMIT 10")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// AddRecentProject upserts a project path and prunes old entries beyond 10.
func AddRecentProject(projectPath string) error {
	_, err := DB.Exec(
		"INSERT INTO recent_projects (project_path, accessed_at) VALUES (?, CURRENT_TIMESTAMP) "+
			"ON CONFLICT(project_path) DO UPDATE SET accessed_at = CURRENT_TIMESTAMP",
		projectPath,
	)
	if err != nil {
		return err
	}
	_, err = DB.Exec(
		"DELETE FROM recent_projects WHERE id NOT IN (SELECT id FROM recent_projects ORDER BY accessed_at DESC LIMIT 10)",
	)
	return err
}

// generateSessionID generates a standard UUID v4 format session ID.
func generateSessionID() string {
	for i := 0; i < 10; i++ {
		b := make([]byte, 16)
		rand.Read(b)
		// Set version (4) and variant (2) bits according to UUID v4 spec
		b[6] = (b[6] & 0x0f) | 0x40 // Version 4
		b[8] = (b[8] & 0x3f) | 0x80 // Variant 2
		// Standard UUID format: 8-4-4-4-12 (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
		sessionID := fmt.Sprintf("%x-%x-%x-%x-%x",
			b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
		
		// Check for conflicts
		var exists bool
		err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM chat_sessions WHERE id = ?)", sessionID).Scan(&exists)
		if err != nil {
			slog.Warn("generateSessionID: DB check failed", slog.String("err", err.Error()))
			continue  // Continue to next attempt instead of returning
		}
		if !exists {
			return sessionID
		}
	}
	return ""  // Explicitly return empty after 10 attempts
}

// GetSessions retrieves chat sessions for a given project path.
// If backend is non-empty, filters by backend; otherwise returns all backends.
func GetSessions(projectPath, backend string) ([]model.ChatSession, error) {
	sessions := []model.ChatSession{}
	query := `SELECT s.id, s.title, s.backend, s.agent_id, s.model, s.created_at, s.updated_at, s.last_read_at,
		(SELECT COUNT(*) FROM chat_history h WHERE h.session_id = s.id AND h.role = 'assistant'
		 AND (s.last_read_at IS NULL OR h.created_at > s.last_read_at)) AS unread_count
		FROM chat_sessions s WHERE s.project_path = ?`
	args := []interface{}{projectPath}
	if backend != "" {
		query += " AND s.backend = ?"
		args = append(args, backend)
	}
	query += " ORDER BY s.updated_at DESC"

	rows, err := DB.Query(query, args...)
	if err != nil {
		return sessions, err
	}
	defer rows.Close()

	for rows.Next() {
		var s model.ChatSession
		var lastRead sql.NullTime
		if err := rows.Scan(&s.ID, &s.Title, &s.Backend, &s.AgentID, &s.Model, &s.CreatedAt, &s.UpdatedAt, &lastRead, &s.UnreadCount); err != nil {
			return nil, err
		}
		if lastRead.Valid {
			s.LastReadAt = &lastRead.Time
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// UpdateLastRead sets the last_read_at timestamp for a session to now.
func UpdateLastRead(sessionID string) {
	DB.Exec("UPDATE chat_sessions SET last_read_at = CURRENT_TIMESTAMP WHERE id = ?", sessionID)
}

// GetSessionBackend returns the backend of a session, or empty string if not found.
func GetSessionBackend(sessionID string) string {
	var backend string
	err := DB.QueryRow("SELECT backend FROM chat_sessions WHERE id = ?", sessionID).Scan(&backend)
	if err != nil {
		return ""
	}
	return backend
}

// CreateSession creates a new chat session and returns its ID.
func CreateSession(projectPath, backend, title, agentID, modelName string) (string, error) {
	sessionID := generateSessionID()
	if sessionID == "" {
		return "", fmt.Errorf("failed to generate unique session ID after 10 attempts")
	}
	_, err := DB.Exec(
		"INSERT INTO chat_sessions (id, project_path, backend, title, agent_id, model) VALUES (?, ?, ?, ?, ?, ?)",
		sessionID, projectPath, backend, title, agentID, modelName,
	)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

// DeleteSession deletes a chat session and all its messages.
func DeleteSession(projectPath, backend, sessionID string) error {
	// Delete all messages in this session first
	_, err := DB.Exec("DELETE FROM chat_history WHERE project_path = ? AND backend = ? AND session_id = ?", projectPath, backend, sessionID)
	if err != nil {
		return err
	}
	// Delete scheduled tasks that reference this session
	_, err = DB.Exec("DELETE FROM scheduled_tasks WHERE project_path = ? AND session_id = ?", projectPath, sessionID)
	if err != nil {
		slog.Warn("failed to delete tasks referencing deleted session",
			slog.String("session", sessionID),
			slog.String("err", err.Error()))
		// Continue to delete session anyway
	}
	// Delete the session record
	_, err = DB.Exec("DELETE FROM chat_sessions WHERE project_path = ? AND backend = ? AND id = ?", projectPath, backend, sessionID)
	return err
}

// GetSessionTitle returns the title of a session.
func GetSessionTitle(sessionID string) (string, error) {
	var title string
	err := DB.QueryRow("SELECT title FROM chat_sessions WHERE id = ?", sessionID).Scan(&title)
	if err != nil {
		return "", err
	}
	return title, nil
}

// GetSessionAgentID returns the agent_id of a session.
func GetSessionAgentID(sessionID string) string {
	var agentID string
	DB.QueryRow("SELECT agent_id FROM chat_sessions WHERE id = ?", sessionID).Scan(&agentID)
	return agentID
}

// Active session tracking - keyed by sessionID
var (
	activeSessions = make(map[string]bool)
	activeMu      sync.Mutex
)

// IsSessionRunning checks if a session is currently running.
func IsSessionRunning(sessionID string) bool {
	activeMu.Lock()
	defer activeMu.Unlock()
	return activeSessions[sessionID]
}

// SessionHasAssistant checks if a session already has finalized assistant replies (for Claude --resume).
func SessionHasAssistant(sessionID string) bool {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM chat_history WHERE session_id = ? AND role = 'assistant' AND streaming = 0", sessionID).Scan(&count)
	return count > 0
}

// SetSessionRunning sets the running state for a session.
func SetSessionRunning(sessionID string, running bool) {
	activeMu.Lock()
	defer activeMu.Unlock()
	if running {
		activeSessions[sessionID] = true
	} else {
		delete(activeSessions, sessionID)
	}
}

// TrySetSessionRunning atomically checks and sets running state.
// Returns true if session was successfully marked as running (was not running before).
// Returns false if session was already running.
func TrySetSessionRunning(sessionID string) bool {
	activeMu.Lock()
	defer activeMu.Unlock()

	if activeSessions[sessionID] {
		return false
	}
	activeSessions[sessionID] = true
	return true
}

// Session stream channel management for SSE streaming
var sessionStreams sync.Map // map[string]chan ai.StreamEvent

// Session cancel functions for aborting AI responses
var sessionCancels sync.Map // map[string]context.CancelFunc
var sessionCancelReasons sync.Map // map[string]string — "user", "disconnect"

// RegisterSessionCancel stores the cancel function for a session
func RegisterSessionCancel(sessionID string, cancel context.CancelFunc) {
	sessionCancels.Store(sessionID, cancel)
}

// GetAndClearCancelReason returns the reason for the most recent cancellation of a session.
// Returns "user" for user-initiated cancel, "disconnect" for SSE client disconnect.
// Returns "" if no reason was recorded (e.g. timeout or no cancel).
func GetAndClearCancelReason(sessionID string) string {
	val, ok := sessionCancelReasons.LoadAndDelete(sessionID)
	if !ok {
		return ""
	}
	return val.(string)
}

// UnregisterSessionCancel removes the cancel function for a session
func UnregisterSessionCancel(sessionID string) {
	sessionCancels.Delete(sessionID)
}

// CancelSession cancels an ongoing AI stream for a session.
// Returns true if session was found and cancelled, or if session is already not running (idempotent).
func CancelSession(sessionID string) bool {
	// Load and delete the cancel function
	val, ok := sessionCancels.LoadAndDelete(sessionID)
	if !ok {
		// If session is not in running state, consider it already cancelled (idempotent)
		if !IsSessionRunning(sessionID) {
			return true
		}
		return false
	}
	cancel, ok := val.(context.CancelFunc)
	if !ok {
		return false
	}

	// Cancel the context first (kills CLI subprocess), which causes the goroutine
	// to stop producing events and drain the channel, making room for the cancelled event.
	sessionCancelReasons.Store(sessionID, "user")
	cancel()

	// Send cancelled event to SSE stream after cancelling context (non-blocking)
	if streamVal, ok := sessionStreams.Load(sessionID); ok {
		if ch, ok := streamVal.(chan ai.StreamEvent); ok {
			select {
			case ch <- ai.StreamEvent{Type: "cancelled"}:
			default:
				// Channel full — SSE handler will detect session not running via checkSSE loop
			}
		}
	}

	// Mark session as not running
	SetSessionRunning(sessionID, false)

	return true
}

// ForceCancelSession cancels the AI context for a session without sending SSE events.
// Used when the SSE client has disconnected and we want to stop the AI goroutine
// to prevent zombie processes.
func ForceCancelSession(sessionID string) {
	val, ok := sessionCancels.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	sessionCancelReasons.Store(sessionID, "disconnect")
	if cancel, ok := val.(context.CancelFunc); ok {
		cancel()
	}
}

// RegisterSessionStream creates and registers a stream channel for a session
func RegisterSessionStream(sessionID string) chan ai.StreamEvent {
	ch := make(chan ai.StreamEvent, 64)
	sessionStreams.Store(sessionID, ch)
	return ch
}

// GetSessionStream returns the stream channel for a session
func GetSessionStream(sessionID string) (<-chan ai.StreamEvent, bool) {
	val, ok := sessionStreams.Load(sessionID)
	if !ok {
		return nil, false
	}
	ch, ok := val.(chan ai.StreamEvent)
	if !ok {
		return nil, false
	}
	return ch, true
}

// UnregisterSessionStream removes and closes the stream channel for a session
func UnregisterSessionStream(sessionID string) {
	if val, ok := sessionStreams.LoadAndDelete(sessionID); ok {
		if ch, ok := val.(chan ai.StreamEvent); ok {
			close(ch)
		}
	}
}

// SendSessionEvent sends an event to the session stream channel (non-blocking).
// Returns true if the event was sent successfully.
func SendSessionEvent(sessionID string, event ai.StreamEvent) bool {
	if streamVal, ok := sessionStreams.Load(sessionID); ok {
		if ch, ok := streamVal.(chan ai.StreamEvent); ok {
			select {
			case ch <- event:
				return true
			default:
			}
		}
	}
	return false
}

// UpdateStreamingMessage updates the content of the streaming assistant message for a session.
func UpdateStreamingMessage(projectPath, backend, sessionID, content string) error {
	_, err := DB.Exec(
		"UPDATE chat_history SET content = ? WHERE project_path = ? AND backend = ? AND session_id = ? AND role = 'assistant' AND streaming = 1",
		content, projectPath, backend, sessionID,
	)
	return err
}

// FinalizeStreamingMessage marks the streaming assistant message as complete and updates its content.
func FinalizeStreamingMessage(projectPath, backend, sessionID, content string) error {
	_, err := DB.Exec(
		"UPDATE chat_history SET content = ?, streaming = 0 WHERE project_path = ? AND backend = ? AND session_id = ? AND role = 'assistant' AND streaming = 1",
		content, projectPath, backend, sessionID,
	)
	return err
}

// UpdateMessageContent updates the content of a specific message by its ID.
func UpdateMessageContent(messageID int, content string) error {
	_, err := DB.Exec("UPDATE chat_history SET content = ? WHERE id = ?", content, messageID)
	return err
}

// UpdateExternalSessionID sets the external session ID for a ClawBench session.
// This is used by the OpenCode backend, which manages its own session IDs internally.
func UpdateExternalSessionID(sessionID, externalID string) error {
	_, err := DB.Exec("UPDATE chat_sessions SET external_session_id = ? WHERE id = ?", externalID, sessionID)
	return err
}

// GetExternalSessionID returns the external session ID for a ClawBench session.
// Returns empty string if not set or on error.
func GetExternalSessionID(sessionID string) string {
	var externalID string
	err := DB.QueryRow("SELECT external_session_id FROM chat_sessions WHERE id = ?", sessionID).Scan(&externalID)
	if err != nil {
		return ""
	}
	return externalID
}

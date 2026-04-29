package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"clawbench/internal/model"
)

// GetChatHistory retrieves all chat messages for a given project path, backend, and session.
func GetChatHistory(projectPath, backend, sessionID string) ([]model.ChatMessage, error) {
	return GetChatHistoryPaged(projectPath, backend, sessionID, 0, "")
}

// GetChatHistoryPaged retrieves chat messages with pagination.
// limit=0 means no limit (all messages).
// beforeTime: if non-empty, only return messages created before this timestamp (cursor-based for lazy load).
// When beforeTime is empty and limit > 0, returns the most recent (limit) messages.
// Returns messages in chronological (ASC) order.
func GetChatHistoryPaged(projectPath, backend, sessionID string, limit int, beforeTime string) ([]model.ChatMessage, error) {
	messages := []model.ChatMessage{}

	if limit > 0 && beforeTime != "" {
		// Cursor-based: load messages older than beforeTime
		query := `SELECT id, role, content, file_path, files, backend, streaming, created_at FROM (
			SELECT id, role, content, file_path, files, backend, streaming, created_at FROM chat_history
			WHERE project_path = ? AND session_id = ? AND created_at < ?
			ORDER BY created_at DESC LIMIT ?
		) sub ORDER BY created_at ASC`
		rows, err := DB.Query(query, projectPath, sessionID, beforeTime, limit)
		if err != nil {
			return messages, err
		}
		defer rows.Close()
		return scanMessages(rows, sessionID)
	}

	if limit > 0 {
		// Initial load: get the most recent (limit) messages
		query := `SELECT id, role, content, file_path, files, backend, streaming, created_at FROM (
			SELECT id, role, content, file_path, files, backend, streaming, created_at FROM chat_history
			WHERE project_path = ? AND session_id = ?
			ORDER BY created_at DESC LIMIT ?
		) sub ORDER BY created_at ASC`
		rows, err := DB.Query(query, projectPath, sessionID, limit)
		if err != nil {
			return messages, err
		}
		defer rows.Close()
		return scanMessages(rows, sessionID)
	}

	// No limit: return all messages in chronological order
	query := `SELECT id, role, content, file_path, files, backend, streaming, created_at FROM chat_history WHERE project_path = ? AND session_id = ? ORDER BY created_at ASC`
	rows, err := DB.Query(query, projectPath, sessionID)
	if err != nil {
		return messages, err
	}
	defer rows.Close()
	return scanMessages(rows, sessionID)
}

// scanMessages scans rows into ChatMessage slice.
func scanMessages(rows *sql.Rows, sessionID string) ([]model.ChatMessage, error) {
	messages := []model.ChatMessage{}
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

// RemoveRecentProject deletes a project path from the recent projects list.
func RemoveRecentProject(projectPath string) error {
	_, err := DB.Exec("DELETE FROM recent_projects WHERE project_path = ?", projectPath)
	return err
}

// generateSessionID generates a standard UUID v4 format session ID.
func generateSessionID() string {
	return generateUUID("", "chat_sessions", "id")
}

// GetSessions retrieves chat sessions for a given project path.
// If backend is non-empty, filters by backend; otherwise returns all backends.
func GetSessions(projectPath, backend string) ([]model.ChatSession, error) {
	sessions := []model.ChatSession{}
	query := `SELECT s.id, s.title, s.backend, s.agent_id, s.model, s.created_at, s.updated_at, s.last_read_at,
		(SELECT COUNT(*) FROM chat_history h WHERE h.session_id = s.id AND h.role = 'assistant' AND h.streaming = 0
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

// SessionHasAssistant checks if a session already has finalized assistant replies (for Claude --resume).
func SessionHasAssistant(sessionID string) bool {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM chat_history WHERE session_id = ? AND role = 'assistant' AND streaming = 0", sessionID).Scan(&count)
	return count > 0
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

// GetStreamingMessageID returns the ID of the finalized assistant message for a session.
// Returns 0 if not found.
func GetStreamingMessageID(sessionID string) int64 {
	var id int64
	err := DB.QueryRow(
		"SELECT id FROM chat_history WHERE session_id = ? AND role = 'assistant' AND streaming = 0 ORDER BY id DESC LIMIT 1",
		sessionID,
	).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}

// UpdateMessageContent updates the content of a specific message by its ID.
func UpdateMessageContent(messageID int, content string) error {
	_, err := DB.Exec("UPDATE chat_history SET content = ? WHERE id = ?", content, messageID)
	return err
}

// SaveRawResponse saves the raw AI backend output for debugging/analysis.
// Called only after the AI response is fully complete.
func SaveRawResponse(sessionID, backend string, messageID int64, rawOutput string) error {
	_, err := DB.Exec(
		"INSERT INTO ai_raw_responses (session_id, message_id, backend, raw_output) VALUES (?, ?, ?, ?)",
		sessionID, messageID, backend, rawOutput,
	)
	return err
}

// UpdateExternalSessionID sets the external session ID for a ClawBench session.
func UpdateExternalSessionID(sessionID, externalID string) error {
	_, err := DB.Exec("UPDATE chat_sessions SET external_session_id = ? WHERE id = ?", externalID, sessionID)
	return err
}

// GetExternalSessionID returns the external session ID for a ClawBench session.
func GetExternalSessionID(sessionID string) string {
	var externalID string
	err := DB.QueryRow("SELECT external_session_id FROM chat_sessions WHERE id = ?", sessionID).Scan(&externalID)
	if err != nil {
		return ""
	}
	return externalID
}

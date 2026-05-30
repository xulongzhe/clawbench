package service

import (
	"clawbench/internal/model"
	"database/sql"
	"fmt"
)

// CheckContinueSession checks whether a continued chat session already exists
// for the given task execution. Returns (exists, sessionID, error).
func CheckContinueSession(execID int64) (bool, string, error) {
	var sourceSessionID string
	err := DBRead.QueryRow("SELECT session_id FROM task_executions WHERE id = ?", execID).Scan(&sourceSessionID)
	if err == sql.ErrNoRows {
		return false, "", fmt.Errorf("execution %d not found", execID)
	}
	if err != nil {
		return false, "", err
	}

	var existingID string
	err = DBRead.QueryRow(
		"SELECT id FROM chat_sessions WHERE source_session_id = ? AND session_type = 'chat' AND deleted = 0",
		sourceSessionID,
	).Scan(&existingID)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, existingID, nil
}

// ContinueFromExecution creates a new chat session from a scheduled task execution,
// copying the original session's chat_history and summaries. If a continued session
// already exists (and is not deleted), it returns the existing session ID with
// alreadyExists=true.
//
// In production, DB has MaxOpenConns=1 so all writes are serialized through a single
// connection — this provides the same atomicity guarantee as BEGIN IMMEDIATE without
// the risk of connection-pool deadlocks in test environments.
func ContinueFromExecution(execID int64, projectPath string) (sessionID string, alreadyExists bool, err error) {
	// 1. Get execution info
	var sourceSessionID string
	var taskID int64
	var execStatus string
	err = DB.QueryRow(
		"SELECT session_id, task_id, status FROM task_executions WHERE id = ?",
		execID,
	).Scan(&sourceSessionID, &taskID, &execStatus)
	if err == sql.ErrNoRows {
		return "", false, fmt.Errorf("execution %d not found", execID)
	}
	if err != nil {
		return "", false, err
	}

	// 2. Check execution status
	if execStatus == "running" {
		return "", false, fmt.Errorf("execution %d is still running", execID)
	}

	// 3. Get task name and validate project ownership
	var taskName string
	var taskProjectPath string
	err = DB.QueryRow(
		"SELECT name, project_path FROM scheduled_tasks WHERE id = ?",
		taskID,
	).Scan(&taskName, &taskProjectPath)
	if err == sql.ErrNoRows {
		return "", false, fmt.Errorf("task %d not found", taskID)
	}
	if err != nil {
		return "", false, err
	}

	// 4. Validate project ownership
	if taskProjectPath != projectPath {
		return "", false, fmt.Errorf("execution %d does not belong to project %q", execID, projectPath)
	}

	// 5. Get source session metadata (without deleted=0 — soft-deleted sessions still have valid metadata)
	var backend, agentID, agentSource, modelName, thinkingEffort, sessProjectPath string
	err = DB.QueryRow(
		"SELECT backend, agent_id, agent_source, model, thinking_effort, project_path FROM chat_sessions WHERE id = ?",
		sourceSessionID,
	).Scan(&backend, &agentID, &agentSource, &modelName, &thinkingEffort, &sessProjectPath)
	if err == sql.ErrNoRows {
		return "", false, fmt.Errorf("source session %s not found", sourceSessionID)
	}
	if err != nil {
		return "", false, err
	}

	// 6. Dedup check — if a continued session already exists, return it
	var existingID string
	err = DB.QueryRow(
		"SELECT id FROM chat_sessions WHERE source_session_id = ? AND session_type = 'chat' AND deleted = 0",
		sourceSessionID,
	).Scan(&existingID)
	if err == nil {
		return existingID, true, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}

	// 7. Max session count check
	if model.SessionMaxCount > 0 {
		var count int
		err = DB.QueryRow(
			"SELECT COUNT(*) FROM chat_sessions WHERE project_path = ? AND deleted = 0 AND session_type = 'chat'",
			sessProjectPath,
		).Scan(&count)
		if err != nil {
			return "", false, err
		}
		if count >= model.SessionMaxCount {
			return "", false, fmt.Errorf("session limit reached (%d/%d)", count, model.SessionMaxCount)
		}
	}

	// 8. Create new chat session
	newSessionID := generateSessionID()
	_, err = DB.Exec(
		"INSERT INTO chat_sessions (id, project_path, backend, title, agent_id, agent_source, model, session_type, source_session_id, thinking_effort) VALUES (?, ?, ?, ?, ?, ?, ?, 'chat', ?, ?)",
		newSessionID, sessProjectPath, backend, taskName, agentID, agentSource, modelName, sourceSessionID, thinkingEffort,
	)
	if err != nil {
		return "", false, fmt.Errorf("failed to create continued session: %w", err)
	}

	// 9. Copy chat_history (only deleted=0 AND streaming=0)
	rows, err := DB.Query(
		"SELECT id, project_path, role, content, files, backend, created_at FROM chat_history WHERE session_id = ? AND deleted = 0 AND streaming = 0 ORDER BY id",
		sourceSessionID,
	)
	if err != nil {
		return "", false, fmt.Errorf("failed to query source messages: %w", err)
	}

	type sourceMsg struct {
		id          int64
		projectPath string
		role        string
		content     string
		files       sql.NullString
		backend     string
		createdAt   sql.NullString
	}
	var messages []sourceMsg
	for rows.Next() {
		var m sourceMsg
		if err := rows.Scan(&m.id, &m.projectPath, &m.role, &m.content, &m.files, &m.backend, &m.createdAt); err != nil {
			rows.Close()
			return "", false, fmt.Errorf("failed to scan source message: %w", err)
		}
		messages = append(messages, m)
	}
	rows.Close()

	// Insert messages and build old ID -> new ID mapping for summaries
	idMap := make(map[int64]int64)
	for _, m := range messages {
		var createdAt interface{}
		if m.createdAt.Valid {
			createdAt = m.createdAt.String
		} else {
			createdAt = nil
		}
		result, err := DB.Exec(
			"INSERT INTO chat_history (project_path, role, content, files, session_id, backend, streaming, deleted, created_at) VALUES (?, ?, ?, ?, ?, ?, 0, 0, ?)",
			m.projectPath, m.role, m.content, m.files, newSessionID, m.backend, createdAt,
		)
		if err != nil {
			return "", false, fmt.Errorf("failed to copy message %d: %w", m.id, err)
		}
		newID, _ := result.LastInsertId()
		idMap[m.id] = newID
	}

	// 10. Copy summaries (only chat_message type)
	for oldID, newID := range idMap {
		var summary string
		err := DB.QueryRow(
			"SELECT summary FROM summaries WHERE target_type = 'chat_message' AND target_id = ?",
			oldID,
		).Scan(&summary)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return "", false, fmt.Errorf("failed to query summary for message %d: %w", oldID, err)
		}
		_, err = DB.Exec(
			"INSERT OR REPLACE INTO summaries (target_type, target_id, summary, created_at) VALUES ('chat_message', ?, ?, CURRENT_TIMESTAMP)",
			newID, summary,
		)
		if err != nil {
			return "", false, fmt.Errorf("failed to copy summary for message %d: %w", oldID, err)
		}
	}

	return newSessionID, false, nil
}

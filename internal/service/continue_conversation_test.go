package service_test

import (
	"testing"

	"clawbench/internal/model"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
)

// ---------- ContinueFromExecution: dedup check ----------

func TestContinueFromExecution_CheckNotContinued(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Daily Review", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Daily Review")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	exists, sessionID, err := service.CheckContinueSession(execID)
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Empty(t, sessionID)
}

func TestContinueFromExecution_CheckAlreadyContinued(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Daily Review", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Daily Review")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	// Continue the execution
	newSessID, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)
	assert.NotEmpty(t, newSessID)

	// Check should now find it
	exists, foundSessID, err := service.CheckContinueSession(execID)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, newSessID, foundSessID)
}

// ---------- ContinueFromExecution: normal flow ----------

func TestContinueFromExecution_NormalFlow(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Daily Code Review", "claude")
	sessID := helperCreateScheduledSessionWithDetails(t, "/project", "claude", "Daily Code Review", "claude-agent", "claude-sonnet-4-6", "high")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	// Add messages to the scheduled session
	_, err := service.AddChatMessage("/project", "claude", sessID, "user", "Review the code", nil, false, "")
	assert.NoError(t, err)
	_, err = service.AddChatMessage("/project", "claude", sessID, "assistant", "Code looks good", nil, false, "")
	assert.NoError(t, err)

	newSessID, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)
	assert.NotEmpty(t, newSessID)

	// New session should be a chat session
	var sessionType string
	err = service.DB.QueryRow("SELECT session_type FROM chat_sessions WHERE id = ?", newSessID).Scan(&sessionType)
	assert.NoError(t, err)
	assert.Equal(t, "chat", sessionType)

	// New session should have the task name as title
	title, err := service.GetSessionTitle(newSessID)
	assert.NoError(t, err)
	assert.Equal(t, "Daily Code Review", title)

	// New session should inherit agent/model/thinking
	info, err := service.GetSessionInfo(newSessID)
	assert.NoError(t, err)
	assert.Equal(t, "claude", info.Backend)
	assert.Equal(t, "claude-agent", info.AgentID)
	assert.Equal(t, "claude-sonnet-4-6", info.Model)
	assert.Equal(t, "high", info.ThinkingEffort)

	// New session should have source_session_id
	var sourceSessID *string
	err = service.DB.QueryRow("SELECT source_session_id FROM chat_sessions WHERE id = ?", newSessID).Scan(&sourceSessID)
	assert.NoError(t, err)
	assert.NotNil(t, sourceSessID)
	assert.Equal(t, sessID, *sourceSessID)

	// External session ID should be empty
	assert.Equal(t, "", service.GetExternalSessionID(newSessID))

	// Messages should be copied
	msgs, err := service.GetChatHistory("/project", "claude", newSessID)
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "Review the code", msgs[0].Content)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "Code looks good", msgs[1].Content)
}

// ---------- ContinueFromExecution: dedup (already continued) ----------

func TestContinueFromExecution_AlreadyContinued(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	newSessID1, alreadyExists1, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)
	assert.False(t, alreadyExists1)

	// Second call should return the same session with alreadyExists=true
	newSessID2, alreadyExists2, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)
	assert.Equal(t, newSessID1, newSessID2)
	assert.True(t, alreadyExists2)
}

// ---------- ContinueFromExecution: delete then re-continue ----------

func TestContinueFromExecution_DeletedThenRecontinue(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	newSessID1, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)

	// Delete the continued session
	err = service.DeleteSession("/project", "claude", newSessID1)
	assert.NoError(t, err)

	// Should be able to continue again
	newSessID2, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)
	assert.NotEqual(t, newSessID1, newSessID2) // Different session ID
}

// ---------- ContinueFromExecution: session count limit ----------

func TestContinueFromExecution_SessionCountLimit(t *testing.T) {
	setupDB(t)

	// Set max session count
	origMax := model.SessionMaxCount
	model.SessionMaxCount = 1
	t.Cleanup(func() { model.SessionMaxCount = origMax })

	// Create a chat session to fill the limit
	helperCreateSession(t, "/project", "claude", "Existing")

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	_, _, err := service.ContinueFromExecution(execID, "/project")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session limit")
}

// ---------- ContinueFromExecution: running status rejection ----------

func TestContinueFromExecution_RunningExecution(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "running")

	_, _, err := service.ContinueFromExecution(execID, "/project")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "running")
}

// ---------- ContinueFromExecution: copy scope ----------

func TestContinueFromExecution_SkipsStreamingMessages(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	// Add finalized + streaming messages
	_, err := service.AddChatMessage("/project", "claude", sessID, "user", "prompt", nil, false, "")
	assert.NoError(t, err)
	_, err = service.AddChatMessage("/project", "claude", sessID, "assistant", "final", nil, false, "")
	assert.NoError(t, err)
	_, err = service.AddChatMessage("/project", "claude", sessID, "assistant", "streaming...", nil, true, "")
	assert.NoError(t, err)

	newSessID, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)

	msgs, err := service.GetChatHistory("/project", "claude", newSessID)
	assert.NoError(t, err)
	assert.Len(t, msgs, 2) // user + finalized assistant only
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "final", msgs[1].Content)
}

func TestContinueFromExecution_SkipsDeletedMessages(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	// Add a message then soft-delete it
	msgID, err := service.AddChatMessage("/project", "claude", sessID, "user", "deleted msg", nil, false, "")
	assert.NoError(t, err)
	_, err = service.DB.Exec("UPDATE chat_history SET deleted = 1 WHERE id = ?", msgID)
	assert.NoError(t, err)

	// Add an active message
	_, err = service.AddChatMessage("/project", "claude", sessID, "user", "active msg", nil, false, "")
	assert.NoError(t, err)

	newSessID, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)

	msgs, err := service.GetChatHistory("/project", "claude", newSessID)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "active msg", msgs[0].Content)
}

// ---------- ContinueFromExecution: summaries copy ----------

func TestContinueFromExecution_CopiesChatMessageSummaries(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	// Add messages
	_, err := service.AddChatMessage("/project", "claude", sessID, "user", "prompt", nil, false, "")
	assert.NoError(t, err)
	asstID, err := service.AddChatMessage("/project", "claude", sessID, "assistant", "reply", nil, false, "")
	assert.NoError(t, err)

	// Add a chat_message summary
	err = service.SaveSummary("chat_message", asstID, "AI replied with details")
	assert.NoError(t, err)

	// Add a task_execution summary (should NOT be copied)
	err = service.SaveSummary("task_execution", execID, "Task completed successfully")
	assert.NoError(t, err)

	newSessID, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)

	// The new assistant message should have a summary
	newMsgs, err := service.GetChatHistory("/project", "claude", newSessID)
	assert.NoError(t, err)
	assert.Len(t, newMsgs, 2)

	// Look up summary for the new assistant message
	newAsstID := newMsgs[1].ID
	summary, found := service.GetSummary("chat_message", newAsstID)
	assert.True(t, found)
	assert.Equal(t, "AI replied with details", summary)
}

// ---------- ContinueFromExecution: soft-deleted source session ----------

func TestContinueFromExecution_SoftDeletedSource(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSessionWithDetails(t, "/project", "claude", "Task", "agent-1", "model-1", "low")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	// Add messages before deleting
	_, err := service.AddChatMessage("/project", "claude", sessID, "user", "prompt", nil, false, "")
	assert.NoError(t, err)

	// Soft-delete the source session
	err = service.DeleteSession("/project", "claude", sessID)
	assert.NoError(t, err)

	// Should still be able to continue (source metadata still readable)
	newSessID, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)
	assert.NotEmpty(t, newSessID)

	// Inherited fields should still be correct
	info, err := service.GetSessionInfo(newSessID)
	assert.NoError(t, err)
	assert.Equal(t, "claude", info.Backend)
	assert.Equal(t, "agent-1", info.AgentID)
	assert.Equal(t, "model-1", info.Model)
	assert.Equal(t, "low", info.ThinkingEffort)
}

// ---------- ContinueFromExecution: execution not found ----------

func TestContinueFromExecution_ExecutionNotFound(t *testing.T) {
	setupDB(t)

	_, _, err := service.ContinueFromExecution(99999, "/project")
	assert.Error(t, err)
}

// ---------- ContinueFromExecution: project mismatch ----------

func TestContinueFromExecution_ProjectMismatch(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	// Wrong project path
	_, _, err := service.ContinueFromExecution(execID, "/other-project")
	assert.Error(t, err)
}

// ---------- ContinueFromExecution: field inheritance ----------

func TestContinueFromExecution_FieldInheritance(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "codebuddy")
	sessID := helperCreateScheduledSessionWithDetails(t, "/project", "codebuddy", "Task", "cb-agent", "gpt-4o", "medium")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	newSessID, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)

	info, err := service.GetSessionInfo(newSessID)
	assert.NoError(t, err)
	assert.Equal(t, "codebuddy", info.Backend)
	assert.Equal(t, "cb-agent", info.AgentID)
	assert.Equal(t, "gpt-4o", info.Model)
	assert.Equal(t, "medium", info.ThinkingEffort)

	// Project path should be inherited
	var projPath string
	err = service.DB.QueryRow("SELECT project_path FROM chat_sessions WHERE id = ?", newSessID).Scan(&projPath)
	assert.NoError(t, err)
	assert.Equal(t, "/project", projPath)
}

// ---------- ContinueFromExecution: original session unaffected ----------

func TestContinueFromExecution_OriginalSessionUnaffected(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	_, err := service.AddChatMessage("/project", "claude", sessID, "user", "prompt", nil, false, "")
	assert.NoError(t, err)

	newSessID, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)

	// Original session should still be scheduled type
	var origType string
	err = service.DB.QueryRow("SELECT session_type FROM chat_sessions WHERE id = ?", sessID).Scan(&origType)
	assert.NoError(t, err)
	assert.Equal(t, "scheduled", origType)

	// Original session's source_session_id should be NULL
	var origSource *string
	err = service.DB.QueryRow("SELECT source_session_id FROM chat_sessions WHERE id = ?", sessID).Scan(&origSource)
	assert.NoError(t, err)
	assert.Nil(t, origSource)

	// New session should NOT affect original messages
	origMsgs, err := service.GetChatHistory("/project", "claude", sessID)
	assert.NoError(t, err)
	assert.Len(t, origMsgs, 1)

	// New session should be completely separate
	newMsgs, err := service.GetChatHistory("/project", "claude", newSessID)
	assert.NoError(t, err)
	assert.Len(t, newMsgs, 1)
}

// ---------- ContinueFromExecution: no messages to copy ----------

func TestContinueFromExecution_NoMessages(t *testing.T) {
	setupDB(t)

	taskID := helperCreateScheduledTask(t, "/project", "Task", "claude")
	sessID := helperCreateScheduledSession(t, "/project", "claude", "Task")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	// No messages added — should still create the session
	newSessID, _, err := service.ContinueFromExecution(execID, "/project")
	assert.NoError(t, err)
	assert.NotEmpty(t, newSessID)

	msgs, err := service.GetChatHistory("/project", "claude", newSessID)
	assert.NoError(t, err)
	assert.Empty(t, msgs)
}

// ========== Test Helpers ==========

// helperCreateScheduledTask creates a scheduled task and returns its ID.
func helperCreateScheduledTask(t *testing.T, projectPath, name, agentID string) int64 {
	t.Helper()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status) VALUES (?, ?, '0 8 * * *', ?, 'Do task', 'active')",
		projectPath, name, agentID,
	)
	assert.NoError(t, err)
	id, err := result.LastInsertId()
	assert.NoError(t, err)
	return id
}

// helperCreateTaskExecution creates a task execution row and returns its ID.
func helperCreateTaskExecution(t *testing.T, taskID int64, sessionID, status string) int64 {
	t.Helper()
	result, err := service.DB.Exec(
		"INSERT INTO task_executions (task_id, session_id, status) VALUES (?, ?, ?)",
		taskID, sessionID, status,
	)
	assert.NoError(t, err)
	id, err := result.LastInsertId()
	assert.NoError(t, err)
	return id
}

// helperCreateScheduledSessionWithDetails creates a scheduled session with full metadata.
func helperCreateScheduledSessionWithDetails(t *testing.T, projectPath, backend, title, agentID, modelName, thinkingEffort string) string {
	t.Helper()
	id, err := service.CreateSession(projectPath, backend, title, agentID, modelName, "default", "scheduled")
	assert.NoError(t, err)
	assert.NotEmpty(t, id)
	if thinkingEffort != "" {
		err = service.UpdateSessionThinkingEffort(id, thinkingEffort)
		assert.NoError(t, err)
	}
	return id
}

func TestContinueFromExecution_CopiesTaskExecutionSummary(t *testing.T) {
	setupDB(t)
	projectPath := "/test/project"

	// Create task + session + assistant message + execution
	taskID := helperCreateScheduledTask(t, projectPath, "Summary Test", "agent1")
	sessionID := helperCreateScheduledSessionWithDetails(t, projectPath, "claude", "Summary Test", "agent1", "", "")
	_, err := service.DB.Exec("INSERT INTO chat_history (session_id, project_path, role, content, backend) VALUES (?, ?, 'user', 'hello', 'claude')", sessionID, projectPath)
	assert.NoError(t, err)
	_, err = service.DB.Exec("INSERT INTO chat_history (session_id, project_path, role, content, backend) VALUES (?, ?, 'assistant', '{\"blocks\":[{\"type\":\"text\",\"text\":\"response\"}]}', 'claude')", sessionID, projectPath)
	assert.NoError(t, err)
	execID := helperCreateTaskExecution(t, taskID, sessionID, "completed")

	// Add task_execution type summary (this is what scheduled sessions have)
	_, err = service.DB.Exec("INSERT INTO summaries (target_type, target_id, summary, created_at) VALUES ('task_execution', ?, 'This is the task execution summary', CURRENT_TIMESTAMP)", execID)
	assert.NoError(t, err)

	// Continue
	newSessionID, alreadyExists, err := service.ContinueFromExecution(execID, projectPath)
	assert.NoError(t, err)
	assert.False(t, alreadyExists)
	assert.NotEmpty(t, newSessionID)

	// Verify: task_execution summary is copied as chat_message type to the last assistant message
	var lastAssistantID int64
	err = service.DB.QueryRow("SELECT id FROM chat_history WHERE session_id = ? AND role = 'assistant' ORDER BY id DESC LIMIT 1", newSessionID).Scan(&lastAssistantID)
	assert.NoError(t, err)

	var copiedSummary string
	err = service.DB.QueryRow("SELECT summary FROM summaries WHERE target_type = 'chat_message' AND target_id = ?", lastAssistantID).Scan(&copiedSummary)
	assert.NoError(t, err)
	assert.Equal(t, "This is the task execution summary", copiedSummary)
}

// Regression test: copied messages must NOT have ISO 8601 UTC timestamps
// (e.g. "2026-05-29T01:59:53Z") because the Go SQLite driver converts DATETIME
// to that format when reading. When written back, the format breaks string-based
// time comparisons with CURRENT_TIMESTAMP format ("YYYY-MM-DD HH:MM:SS"),
// causing phantom unread badges. The fix: let the database assign CURRENT_TIMESTAMP
// as created_at instead of copying the ISO-format value.
func TestContinueFromExecution_CreatedAtFormatConsistent(t *testing.T) {
	setupDB(t)
	projectPath := "/project"

	taskID := helperCreateScheduledTask(t, projectPath, "Format Test", "claude")
	sessID := helperCreateScheduledSession(t, projectPath, "claude", "Format Test")
	execID := helperCreateTaskExecution(t, taskID, sessID, "completed")

	// Add messages (AddChatMessage uses DEFAULT CURRENT_TIMESTAMP → "YYYY-MM-DD HH:MM:SS")
	_, err := service.AddChatMessage(projectPath, "claude", sessID, "user", "prompt", nil, false, "")
	assert.NoError(t, err)
	_, err = service.AddChatMessage(projectPath, "claude", sessID, "assistant", "response", nil, false, "")
	assert.NoError(t, err)

	newSessID, _, err := service.ContinueFromExecution(execID, projectPath)
	assert.NoError(t, err)

	// Verify: copied messages' created_at should NOT contain 'T' or 'Z' (ISO format markers)
	var hasBadFormat int
	err = service.DB.QueryRow(
		"SELECT COUNT(*) FROM chat_history WHERE session_id = ? AND (created_at LIKE '%T%' OR created_at LIKE '%Z%')",
		newSessID,
	).Scan(&hasBadFormat)
	assert.NoError(t, err)
	assert.Equal(t, 0, hasBadFormat, "copied messages should use CURRENT_TIMESTAMP format, not ISO 8601")

	// Verify: unread count query should return 0 for the continued session
	// (last_read_at is set at creation time, and created_at uses the same format)
	var unreadCount int
	err = service.DB.QueryRow(`
		SELECT COALESCE(unread.cnt, 0) FROM chat_sessions s
		LEFT JOIN (
			SELECT h.session_id, COUNT(*) AS cnt
			FROM chat_history h
			JOIN chat_sessions s2 ON s2.id = h.session_id
			WHERE h.project_path = ?
			  AND h.role = 'assistant' AND h.streaming = 0 AND h.deleted = 0
			  AND (s2.last_read_at IS NULL OR h.created_at > s2.last_read_at)
			GROUP BY h.session_id
		) unread ON unread.session_id = s.id
		WHERE s.id = ?`,
		projectPath, newSessID,
	).Scan(&unreadCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, unreadCount, "continued session should have 0 unread messages after creation")
}

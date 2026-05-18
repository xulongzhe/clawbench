package cli

import (
	"testing"

	"clawbench/internal/model"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
)

// closeDBAfterTest closes the global DB on test cleanup to release
// the SQLite file lock on Windows (t.TempDir cleanup would fail otherwise).
func closeDBAfterTest(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if service.DB != nil {
			service.DB.Close()
			service.DB = nil
		}
	})
}

// setupOldSchemaDB creates a temp directory with an "old schema" database
// that has task_executions with a content column (pre-migration state).
func setupOldSchemaDB(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	model.BinDir = tmpDir
	// Keep ConfigInstance.WatchDir set so loadConfig() is a no-op inside RunMigrateCommand.
	// Otherwise loadConfig() would overwrite model.BinDir from os.Args[0] (test binary path).
	model.ConfigInstance = model.Config{WatchDir: tmpDir}

	// Initialize DB so all current-schema tables exist
	service.InitDB()
	closeDBAfterTest(t)

	db := service.DB

	// Replace scheduled_tasks with TEXT id for old-schema compatibility
	// (InitDB now creates INTEGER id, but old data uses TEXT IDs)
	_, err := db.Exec("DROP TABLE scheduled_tasks")
	assert.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE scheduled_tasks (
		id TEXT PRIMARY KEY,
		project_path TEXT NOT NULL,
		name TEXT NOT NULL,
		cron_expr TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		prompt TEXT NOT NULL,
		session_id TEXT,
		status TEXT NOT NULL DEFAULT 'active',
		repeat_mode TEXT NOT NULL DEFAULT 'unlimited',
		max_runs INTEGER DEFAULT 0,
		last_run_at DATETIME,
		next_run_at DATETIME,
		run_count INTEGER DEFAULT 0,
		last_read_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	assert.NoError(t, err)

	// Rename current table out of the way
	_, err = db.Exec("ALTER TABLE task_executions RENAME TO task_executions_new")
	assert.NoError(t, err)

	// Create old-style task_executions with content column
	_, err = db.Exec(`CREATE TABLE task_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		trigger_type TEXT NOT NULL DEFAULT 'auto',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	assert.NoError(t, err)

	// Drop the renamed table — we only need the old one
	_, err = db.Exec("DROP TABLE task_executions_new")
	assert.NoError(t, err)

	// Remove session_type from chat_sessions to simulate pre-migration state.
	// SQLite doesn't support DROP COLUMN before 3.35.0, so we recreate the table.
	_, err = db.Exec(`
		CREATE TABLE chat_sessions_old AS
		SELECT id, project_path, backend, title, agent_id, agent_source, model, external_session_id, deleted, last_read_at, created_at, updated_at
		FROM chat_sessions
	`)
	assert.NoError(t, err)
	_, err = db.Exec("DROP TABLE chat_sessions")
	assert.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE chat_sessions_old RENAME TO chat_sessions`)
	assert.NoError(t, err)

	// Insert a scheduled task
	_, err = db.Exec(`INSERT INTO scheduled_tasks (id, project_path, name, cron_expr, agent_id, prompt)
		VALUES ('task-1', ?, 'Test Task', '0 9 * * *', 'codebuddy', 'Do something')`, tmpDir)
	assert.NoError(t, err)

	// Insert an execution with content
	_, err = db.Exec(`INSERT INTO task_executions (task_id, content, trigger_type)
		VALUES ('task-1', 'Hello from the AI', 'auto')`)
	assert.NoError(t, err)

	// Set up an agent so model.Agents lookup works
	model.Agents = map[string]*model.Agent{
		"codebuddy": {ID: "codebuddy", Backend: "codebuddy"},
	}

	return tmpDir
}

func TestRunMigrateCommand_HelpFlag(t *testing.T) {
	exitCode := RunMigrateCommand([]string{"--help"})
	assert.Equal(t, 0, exitCode)
}

func TestRunMigrateCommand_ShortHelpFlag(t *testing.T) {
	exitCode := RunMigrateCommand([]string{"-h"})
	assert.Equal(t, 0, exitCode)
}

func TestRunMigrateCommand_NoMigrationNeeded(t *testing.T) {
	// Fresh DB with new schema — should report no migration needed
	tmpDir := t.TempDir()
	model.BinDir = tmpDir
	model.ConfigInstance = model.Config{WatchDir: tmpDir}
	service.InitDB()
	closeDBAfterTest(t)

	exitCode := RunMigrateCommand([]string{})
	assert.Equal(t, 0, exitCode)

	_ = tmpDir
}

func TestRunMigrateCommand_MigratesData(t *testing.T) {
	tmpDir := setupOldSchemaDB(t)

	// Verify old schema: task_executions has content column
	var hasContent int
	err := service.DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('task_executions') WHERE name='content'").Scan(&hasContent)
	assert.NoError(t, err)
	assert.Equal(t, 1, hasContent, "old schema should have content column")

	// Verify old schema: chat_sessions does NOT have session_type
	var hasSessionType int
	service.DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('chat_sessions') WHERE name='session_type'").Scan(&hasSessionType)
	assert.Equal(t, 0, hasSessionType, "old schema should not have session_type column")

	exitCode := RunMigrateCommand([]string{})
	assert.Equal(t, 0, exitCode)

	// Verify: chat_sessions now has session_type column
	err = service.DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('chat_sessions') WHERE name='session_type'").Scan(&hasSessionType)
	assert.NoError(t, err)
	assert.Equal(t, 1, hasSessionType, "chat_sessions should have session_type after migration")

	// Verify: task_executions now has session_id column, no content column
	var hasSessionID int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('task_executions') WHERE name='session_id'").Scan(&hasSessionID)
	assert.NoError(t, err)
	assert.Equal(t, 1, hasSessionID, "task_executions should have session_id after migration")

	var contentColCount int
	service.DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('task_executions') WHERE name='content'").Scan(&contentColCount)
	assert.Equal(t, 0, contentColCount, "task_executions should not have content column after migration")

	// Verify: execution data was migrated — check that a session was created
	var sessionCount int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE session_type = 'scheduled'").Scan(&sessionCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, sessionCount, "one scheduled session should be created")

	// Verify: the session_id in task_executions links to the chat session
	var execSessionID string
	err = service.DB.QueryRow("SELECT session_id FROM task_executions WHERE task_id = 'task-1'").Scan(&execSessionID)
	assert.NoError(t, err)
	assert.NotEmpty(t, execSessionID, "execution should have a session_id")

	// Verify: content was moved to chat_history
	var userMsgCount, assistantMsgCount int
	service.DB.QueryRow("SELECT COUNT(*) FROM chat_history WHERE session_id = ? AND role = 'user'", execSessionID).Scan(&userMsgCount)
	service.DB.QueryRow("SELECT COUNT(*) FROM chat_history WHERE session_id = ? AND role = 'assistant'", execSessionID).Scan(&assistantMsgCount)
	assert.Equal(t, 1, userMsgCount, "should have one user message in chat_history")
	assert.Equal(t, 1, assistantMsgCount, "should have one assistant message in chat_history")

	// Verify: assistant message content matches original execution content
	var assistantContent string
	err = service.DB.QueryRow("SELECT content FROM chat_history WHERE session_id = ? AND role = 'assistant'", execSessionID).Scan(&assistantContent)
	assert.NoError(t, err)
	assert.Equal(t, "Hello from the AI", assistantContent)

	// Verify: user message content matches task prompt
	var userContent string
	err = service.DB.QueryRow("SELECT content FROM chat_history WHERE session_id = ? AND role = 'user'", execSessionID).Scan(&userContent)
	assert.NoError(t, err)
	assert.Equal(t, "Do something", userContent)

	_ = tmpDir
}

// replaceWithTextIDScheduledTasks drops the current INTEGER-id scheduled_tasks
// and recreates it with TEXT id for old-schema compatibility in migration tests.
func replaceWithTextIDScheduledTasks(t *testing.T) {
	t.Helper()
	service.DB.Exec("DROP TABLE scheduled_tasks")
	_, err := service.DB.Exec(`CREATE TABLE scheduled_tasks (
		id TEXT PRIMARY KEY,
		project_path TEXT NOT NULL,
		name TEXT NOT NULL,
		cron_expr TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		prompt TEXT NOT NULL,
		session_id TEXT,
		status TEXT NOT NULL DEFAULT 'active',
		repeat_mode TEXT NOT NULL DEFAULT 'unlimited',
		max_runs INTEGER DEFAULT 0,
		last_run_at DATETIME,
		next_run_at DATETIME,
		run_count INTEGER DEFAULT 0,
		last_read_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	assert.NoError(t, err)
}

func TestRunMigrateCommand_SkipsExecutionWithMissingTask(t *testing.T) {
	tmpDir := t.TempDir()
	model.BinDir = tmpDir
	model.ConfigInstance = model.Config{WatchDir: tmpDir}
	service.InitDB()
	closeDBAfterTest(t)

	replaceWithTextIDScheduledTasks(t)

	// Create old-style task_executions
	service.DB.Exec("DROP TABLE task_executions")
	service.DB.Exec(`CREATE TABLE task_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		trigger_type TEXT NOT NULL DEFAULT 'auto',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	// Insert execution with non-existent task_id
	service.DB.Exec(`INSERT INTO task_executions (task_id, content, trigger_type)
		VALUES ('nonexistent-task', 'Some content', 'auto')`)

	model.Agents = map[string]*model.Agent{}

	exitCode := RunMigrateCommand([]string{})
	assert.Equal(t, 0, exitCode)

	// The migration should still succeed (skipping the orphaned execution)
	// and apply the new schema
	var hasSessionID int
	err := service.DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('task_executions') WHERE name='session_id'").Scan(&hasSessionID)
	assert.NoError(t, err)
	assert.Equal(t, 1, hasSessionID, "task_executions should have session_id after migration")

	_ = tmpDir
}

func TestRunMigrateCommand_Idempotent(t *testing.T) {
	// Running migrate twice should be safe — second run reports no migration needed
	_ = setupOldSchemaDB(t)

	exitCode := RunMigrateCommand([]string{})
	assert.Equal(t, 0, exitCode)

	// Second run — should detect no content column and exit cleanly
	exitCode = RunMigrateCommand([]string{})
	assert.Equal(t, 0, exitCode)
}

func TestRunMigrateCommand_EmptyContentSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	model.BinDir = tmpDir
	model.ConfigInstance = model.Config{WatchDir: tmpDir}
	service.InitDB()
	closeDBAfterTest(t)

	replaceWithTextIDScheduledTasks(t)

	// Create old-style task_executions
	service.DB.Exec("DROP TABLE task_executions")
	service.DB.Exec(`CREATE TABLE task_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		trigger_type TEXT NOT NULL DEFAULT 'auto',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	// Insert a scheduled task
	service.DB.Exec(`INSERT INTO scheduled_tasks (id, project_path, name, cron_expr, agent_id, prompt)
		VALUES ('task-2', ?, 'Empty Task', '0 9 * * *', 'codebuddy', 'Prompt only')`, tmpDir)

	// Insert execution with empty content
	service.DB.Exec(`INSERT INTO task_executions (task_id, content, trigger_type)
		VALUES ('task-2', '', 'auto')`)

	model.Agents = map[string]*model.Agent{
		"codebuddy": {ID: "codebuddy", Backend: "codebuddy"},
	}

	exitCode := RunMigrateCommand([]string{})
	assert.Equal(t, 0, exitCode)

	// Verify: session was created
	var sessionCount int
	service.DB.QueryRow("SELECT COUNT(*) FROM chat_sessions WHERE session_type = 'scheduled'").Scan(&sessionCount)
	assert.Equal(t, 1, sessionCount, "one scheduled session should be created even with empty content")

	// Verify: only user message exists (no assistant message for empty content)
	var execSessionID string
	service.DB.QueryRow("SELECT session_id FROM task_executions WHERE task_id = 'task-2'").Scan(&execSessionID)

	var assistantMsgCount int
	service.DB.QueryRow("SELECT COUNT(*) FROM chat_history WHERE session_id = ? AND role = 'assistant'", execSessionID).Scan(&assistantMsgCount)
	assert.Equal(t, 0, assistantMsgCount, "no assistant message should exist for empty content")

	_ = tmpDir
}

// Test that the migrate command handles the case where session_id already exists
// (partial migration was done before)
func TestRunMigrateCommand_PartialMigration(t *testing.T) {
	tmpDir := t.TempDir()
	model.BinDir = tmpDir
	model.ConfigInstance = model.Config{WatchDir: tmpDir}
	service.InitDB()
	closeDBAfterTest(t)

	// Simulate partial migration: task_executions has both content and session_id.
	// The current InitDB schema doesn't have content column, so add it first.
	service.DB.Exec("ALTER TABLE task_executions ADD COLUMN content TEXT NOT NULL DEFAULT ''")
	// Now session_id already exists (from InitDB schema), so the guard should trigger

	exitCode := RunMigrateCommand([]string{})
	assert.Equal(t, 0, exitCode)

	_ = tmpDir
}

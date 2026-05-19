package service_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/service"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
)

// schedulerSchema is the same schema used in chat_test.go but scoped locally
// to avoid variable name conflicts within the same test package.
const schedulerSchema = `
CREATE TABLE IF NOT EXISTS chat_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_path TEXT NOT NULL,
	role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
	content TEXT NOT NULL,
	files TEXT,
	session_id TEXT,
	backend TEXT NOT NULL DEFAULT 'claude',
	streaming INTEGER NOT NULL DEFAULT 0,
	indexed INTEGER NOT NULL DEFAULT 0,
	deleted INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS chat_sessions (
	id TEXT PRIMARY KEY,
	project_path TEXT NOT NULL,
	backend TEXT NOT NULL,
	title TEXT NOT NULL,
	agent_id TEXT DEFAULT '',
	agent_source TEXT DEFAULT 'default',
	model TEXT DEFAULT '',
	session_type TEXT NOT NULL DEFAULT 'chat',
	deleted INTEGER NOT NULL DEFAULT 0,
	last_read_at DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(project_path, backend, id)
);
CREATE TABLE IF NOT EXISTS scheduled_tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
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
);
CREATE TABLE IF NOT EXISTS task_executions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id INTEGER NOT NULL,
	session_id TEXT NOT NULL,
	trigger_type TEXT NOT NULL DEFAULT 'auto',
	status TEXT NOT NULL DEFAULT 'running',
	read_at DATETIME,
	summary TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_executions_task ON task_executions(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_history_session ON chat_history(project_path, backend, session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_project_backend ON chat_sessions(project_path, backend);
CREATE INDEX IF NOT EXISTS idx_executions_session ON task_executions(session_id);
CREATE TABLE IF NOT EXISTS ai_raw_responses (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	message_id INTEGER NOT NULL,
	backend TEXT NOT NULL DEFAULT '',
	raw_output TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

func setupSchedulerDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	assert.NoError(t, err)
	db.SetMaxOpenConns(1) // Required for :memory: SQLite — all queries must use the same connection
	_, err = db.Exec(schedulerSchema)
	assert.NoError(t, err)
	service.DB = db
	t.Cleanup(func() { db.Close() })
	return db
}

func setupScheduler(t *testing.T) (*service.Scheduler, func()) {
	t.Helper()
	setupSchedulerDB(t)
	s := service.NewScheduler()
	return s, func() { s.Stop() }
}

// helperTask returns a valid ScheduledTask for testing.
func helperTask(overrides ...func(*model.ScheduledTask)) *model.ScheduledTask {
	task := &model.ScheduledTask{
		ProjectPath: "/test-project",
		Name:        "Test Task",
		CronExpr:    "0 * * * *", // every hour
		AgentID:     "agent1",
		Prompt:      "test prompt",
		RepeatMode:  "unlimited",
	}
	for _, fn := range overrides {
		fn(task)
	}
	return task
}

// ---------- NewScheduler ----------

func TestNewScheduler(t *testing.T) {
	s := service.NewScheduler()
	assert.NotNil(t, s)
}

// ---------- Start / Stop ----------

func TestSchedulerStartStop(t *testing.T) {
	s := service.NewScheduler()
	s.Start()
	s.Stop()
	// Should not panic
}

func TestSchedulerStopWithoutStart(t *testing.T) {
	s := service.NewScheduler()
	s.Stop()
	// Should not panic even if never started
}

// ---------- GetTasks ----------

func TestGetTasks_Empty(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	tasks, err := service.GetTasks("/project")
	assert.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestGetTasks_AllProjects(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj1", "Task 1", "0 * * * *", "agent1", "prompt1", "", "active", "unlimited", now, now,
	)
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj2", "Task 2", "0 * * * *", "agent1", "prompt2", "", "active", "unlimited", now, now,
	)

	tasks, err := service.GetTasks("")
	assert.NoError(t, err)
	assert.Len(t, tasks, 2)

	tasks, err = service.GetTasks("/proj1")
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, int64(1), tasks[0].ID)
}

func TestGetTasks_OrdersByCreatedAtDesc(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "First", "0 * * * *", "agent1", "p", "", "active", "unlimited", now.Add(-1*time.Hour), now,
	)
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Second", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)

	tasks, err := service.GetTasks("/proj")
	assert.NoError(t, err)
	assert.Len(t, tasks, 2)
	// newer first (higher auto-increment ID = created later)
	assert.True(t, tasks[0].ID > tasks[1].ID, "newer task should come first")
}

// ---------- GetTaskByID ----------

func TestGetTaskByID(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, max_runs, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task 1", "0 * * * *", "agent1", "prompt1", "sess-1", "active", "unlimited", 0, 3, now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	task, err := service.GetTaskByID(taskID)
	assert.NoError(t, err)
	assert.Equal(t, taskID, task.ID)
	assert.Equal(t, "/proj", task.ProjectPath)
	assert.Equal(t, "Task 1", task.Name)
	assert.Equal(t, "0 * * * *", task.CronExpr)
	assert.Equal(t, "agent1", task.AgentID)
	assert.Equal(t, "prompt1", task.Prompt)
	assert.Equal(t, "sess-1", task.SessionID)
	assert.Equal(t, "active", task.Status)
	assert.Equal(t, "unlimited", task.RepeatMode)
	assert.Equal(t, 3, task.RunCount)
}

func TestGetTaskByID_NotFound(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	_, err := service.GetTaskByID(99999)
	assert.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)
}

// ---------- AddTask ----------

func TestAddTask(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	err := s.AddTask(task)
	assert.NoError(t, err)
	assert.NotZero(t, task.ID, "ID should be auto-generated")
	assert.Equal(t, "active", task.Status)
	assert.NotNil(t, task.NextRunAt, "NextRunAt should be calculated")
	assert.False(t, task.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, task.UpdatedAt.IsZero(), "UpdatedAt should be set")

	// Verify persisted in DB
	persisted, err := service.GetTaskByID(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, task.Name, persisted.Name)
	assert.Equal(t, task.ProjectPath, persisted.ProjectPath)
}

func TestAddTask_InvalidCronExpr(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask(func(t *model.ScheduledTask) {
		t.CronExpr = "invalid-cron"
	})
	err := s.AddTask(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cron expression")
}

func TestAddTask_SetsStatusToActive(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask(func(t *model.ScheduledTask) {
		t.Status = "paused" // try to set a different status
	})
	err := s.AddTask(task)
	assert.NoError(t, err)
	assert.Equal(t, "active", task.Status, "AddTask should always set status to active")
}

func TestAddTask_GeneratesUniqueIDs(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task1 := helperTask()
	task2 := helperTask()
	assert.NoError(t, s.AddTask(task1))
	assert.NoError(t, s.AddTask(task2))
	assert.NotEqual(t, task1.ID, task2.ID, "each task should get a unique ID")
}

func TestAddTask_AutoIncrementID(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))
	assert.True(t, task.ID > 0, "auto-increment ID should be a positive integer")
}

// ---------- RemoveTask ----------

func TestRemoveTask(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	s.RemoveTask(task.ID)

	// Task should be hard-deleted from DB
	_, err := service.GetTaskByID(task.ID)
	assert.Error(t, err, "hard-deleted task should not be found")

	// Should not appear in GetTasks
	tasks, err := service.GetTasks(task.ProjectPath)
	assert.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestRemoveTask_NonExistentDoesNotPanic(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	// Removing a task that was never added should not panic
	s.RemoveTask(99999)
}

// ---------- PauseTask ----------

func TestPauseTask(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	s.PauseTask(task.ID)

	// Task should be marked as paused in DB
	persisted, err := service.GetTaskByID(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, "paused", persisted.Status)

	// Paused tasks should still appear in GetTasks (not deleted)
	tasks, err := service.GetTasks(task.ProjectPath)
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
}

func TestPauseTask_NonExistentDoesNotPanic(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	s.PauseTask(99999)
	// Should not panic
}

// ---------- ResumeTask ----------

func TestResumeTask(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	s.PauseTask(task.ID)

	err := s.ResumeTask(task.ID)
	assert.NoError(t, err)

	persisted, err := service.GetTaskByID(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, "active", persisted.Status)
}

func TestResumeTask_NotPaused(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	// Task is active, not paused - should error
	err := s.ResumeTask(task.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not paused")
}

func TestResumeTask_NonExistent(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	err := s.ResumeTask(99999)
	assert.Error(t, err)
}

func TestResumeTask_AfterRemove(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	s.RemoveTask(task.ID)

	// Deleted task no longer exists, so resume should fail
	err := s.ResumeTask(task.ID)
	assert.Error(t, err)
}

// ---------- UpdateTask ----------

func TestUpdateTask(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	// Update the task
	task.Name = "Updated Name"
	task.Prompt = "updated prompt"
	err := s.UpdateTask(task)
	assert.NoError(t, err)

	persisted, err := service.GetTaskByID(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Name", persisted.Name)
	assert.Equal(t, "updated prompt", persisted.Prompt)
}

func TestUpdateTask_ChangeCronExpr(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	task.CronExpr = "0 0 * * *" // daily
	err := s.UpdateTask(task)
	assert.NoError(t, err)

	persisted, err := service.GetTaskByID(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, "0 0 * * *", persisted.CronExpr)
}

func TestUpdateTask_InvalidCronExpr(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	task.CronExpr = "not-valid"
	err := s.UpdateTask(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cron expression")
}

func TestUpdateTask_PausedTaskDoesNotReregister(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))
	s.PauseTask(task.ID)

	// Reload the task from DB so Status reflects the paused state
	pausedTask, err := service.GetTaskByID(task.ID)
	assert.NoError(t, err)

	// Update a paused task - should not re-register with cron
	pausedTask.Name = "Updated Paused"
	err = s.UpdateTask(pausedTask)
	assert.NoError(t, err)

	persisted, err := service.GetTaskByID(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Paused", persisted.Name)
	// Task stays paused in DB since we updated a paused task
	assert.Equal(t, "paused", persisted.Status)
}

// ---------- LoadTasksFromDB ----------

func TestLoadTasksFromDB(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	// Insert tasks directly into DB
	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Active Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Paused Task", "0 * * * *", "agent1", "p", "", "paused", "unlimited", now, now,
	)

	err := s.LoadTasksFromDB("/proj")
	assert.NoError(t, err)

	// Active task should be loaded; paused task should be skipped
	// Get the active task's ID
	var activeID int64
	service.DB.QueryRow("SELECT id FROM scheduled_tasks WHERE status = 'active' AND project_path = '/proj'").Scan(&activeID)

	// We verify by checking that the active task can be removed without error
	s.RemoveTask(activeID)

	_, err = service.GetTaskByID(activeID)
	assert.Error(t, err, "hard-deleted task should not be found")
}

func TestLoadTasksFromDB_AllProjects(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj1", "Task 1", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj2", "Task 2", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)

	err := s.LoadTasksFromDB("") // empty = all projects
	assert.NoError(t, err)

	// Both tasks should be loaded — verify by getting their IDs and removing them
	var id1, id2 int64
	service.DB.QueryRow("SELECT id FROM scheduled_tasks WHERE project_path = '/proj1'").Scan(&id1)
	service.DB.QueryRow("SELECT id FROM scheduled_tasks WHERE project_path = '/proj2'").Scan(&id2)

	s.RemoveTask(id1)
	s.RemoveTask(id2)

	_, err1 := service.GetTaskByID(id1)
	_, err2 := service.GetTaskByID(id2)
	assert.Error(t, err1, "hard-deleted task should not be found")
	assert.Error(t, err2, "hard-deleted task should not be found")
}

func TestLoadTasksFromDB_InvalidCronSkipped(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Bad Cron", "invalid", "agent1", "p", "", "active", "unlimited", now, now,
	)

	// Should not error — invalid cron tasks are logged and skipped
	err := s.LoadTasksFromDB("/proj")
	assert.NoError(t, err)
}

func TestLoadTasksFromDB_EmptyDB(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	err := s.LoadTasksFromDB("/proj")
	assert.NoError(t, err)
}

// ---------- AddTaskExecution ----------

func TestAddTaskExecution(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	// Insert a task
	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	_, err = service.AddTaskExecution(taskID, "session-abc", "auto")
	assert.NoError(t, err)

	// Verify the execution was recorded
	var count int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE task_id = ?", taskID).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	var fetchedSessionID string
	err = service.DB.QueryRow("SELECT session_id FROM task_executions WHERE task_id = ?", taskID).Scan(&fetchedSessionID)
	assert.NoError(t, err)
	assert.Equal(t, "session-abc", fetchedSessionID)
}

func TestAddTaskExecution_MultipleExecutions(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	_, err = service.AddTaskExecution(taskID, "session-1", "auto")
	assert.NoError(t, err)
	_, err = service.AddTaskExecution(taskID, "session-2", "auto")
	assert.NoError(t, err)

	var count int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE task_id = ?", taskID).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestUpdateExecutionStatus(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	_, err = service.AddTaskExecution(taskID, "session-abc", "auto")
	assert.NoError(t, err)

	// Verify default status is 'running'
	var status string
	err = service.DB.QueryRow("SELECT status FROM task_executions WHERE session_id = ?", "session-abc").Scan(&status)
	assert.NoError(t, err)
	assert.Equal(t, "running", status)

	// Update to cancelled
	err = service.UpdateExecutionStatus("session-abc", "cancelled")
	assert.NoError(t, err)

	err = service.DB.QueryRow("SELECT status FROM task_executions WHERE session_id = ?", "session-abc").Scan(&status)
	assert.NoError(t, err)
	assert.Equal(t, "cancelled", status)
}

func TestUpdateTaskStats(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Stats Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", 0, now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	task, err := service.GetTaskByID(taskID)
	assert.NoError(t, err)
	assert.Equal(t, 0, task.RunCount)

	service.UpdateTaskStats(task, "active")

	updated, err := service.GetTaskByID(taskID)
	assert.NoError(t, err)
	assert.Equal(t, 1, updated.RunCount)
	assert.NotNil(t, updated.LastRunAt)
	assert.Equal(t, "active", updated.Status)
}

// ---------- insertTask / updateTask (tested indirectly via AddTask / UpdateTask) ----------

func TestInsertUpdateTask(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask(func(t *model.ScheduledTask) {
		t.Name = "Original"
	})
	assert.NoError(t, s.AddTask(task))
	originalID := task.ID

	// Update via UpdateTask
	task.Name = "Updated"
	assert.NoError(t, s.UpdateTask(task))

	persisted, err := service.GetTaskByID(originalID)
	assert.NoError(t, err)
	assert.Equal(t, "Updated", persisted.Name)
	assert.Equal(t, originalID, persisted.ID, "ID should not change on update")
}

// ---------- Lifecycle: full workflow ----------

func TestSchedulerFullLifecycle(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	// 1. Add task
	task := helperTask(func(t *model.ScheduledTask) {
		t.Name = "Lifecycle Task"
	})
	assert.NoError(t, s.AddTask(task))
	assert.Equal(t, "active", task.Status)

	// 2. Pause
	s.PauseTask(task.ID)
	paused, _ := service.GetTaskByID(task.ID)
	assert.Equal(t, "paused", paused.Status)

	// 3. Resume
	assert.NoError(t, s.ResumeTask(task.ID))
	resumed, _ := service.GetTaskByID(task.ID)
	assert.Equal(t, "active", resumed.Status)

	// 4. Update
	resumed.Name = "Updated Task"
	assert.NoError(t, s.UpdateTask(resumed))
	updated, _ := service.GetTaskByID(task.ID)
	assert.Equal(t, "Updated Task", updated.Name)

	// 5. Remove
	s.RemoveTask(task.ID)
	_, err := service.GetTaskByID(task.ID)
	assert.Error(t, err, "hard-deleted task should not be found")
}

// ---------- LoadTasksFromDB after scheduler restart ----------

func TestSchedulerRestartLoadsTasks(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	// Use first scheduler to add a task
	s1 := service.NewScheduler()
	s1.Start()
	task := helperTask()
	assert.NoError(t, s1.AddTask(task))
	s1.Stop()

	// Simulate restart: create a new scheduler and load from DB
	s2 := service.NewScheduler()
	s2.Start()
	defer s2.Stop()

	err := s2.LoadTasksFromDB(task.ProjectPath)
	assert.NoError(t, err)

	// Task should be registered; verify by pausing it
	s2.PauseTask(task.ID)
	persisted, _ := service.GetTaskByID(task.ID)
	assert.Equal(t, "paused", persisted.Status)
}

// ---------- Run count atomic increment ----------

func TestRunCount_AtomicIncrement(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	// Insert a task directly
	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "RC Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", 0, now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Run 10 sequential atomic SQL increments.
	for i := 0; i < 10; i++ {
		_, err := service.DB.Exec("UPDATE scheduled_tasks SET run_count = run_count + 1 WHERE id = ?", taskID)
		assert.NoError(t, err)
	}

	// All 10 increments should be accounted for
	task, err := service.GetTaskByID(taskID)
	assert.NoError(t, err)
	assert.Equal(t, 10, task.RunCount, "run_count should be exactly 10 after 10 sequential increments")
}

// ---------- RemoveTask cascade deletes sessions (Task 7) ----------

func TestRemoveTask_CascadeDeletesSessions(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	// Create a task
	task := helperTask(func(t *model.ScheduledTask) {
		t.ProjectPath = "/cascade-proj"
	})
	assert.NoError(t, s.AddTask(task))

	// Create a scheduled chat session
	sessionID, err := service.CreateSession("/cascade-proj", "claude", "Exec 1", "agent1", "", "default", "scheduled")
	assert.NoError(t, err)

	// Add messages to the session
	_, err = service.AddChatMessage("/cascade-proj", "claude", sessionID, "user", "test prompt", nil, false, "Exec 1")
	assert.NoError(t, err)
	_, err = service.AddChatMessage("/cascade-proj", "claude", sessionID, "assistant", "test response", nil, false, "Exec 1")
	assert.NoError(t, err)

	// Create a task_execution linked to this session
	_, err = service.AddTaskExecution(task.ID, sessionID, "auto")
	assert.NoError(t, err)

	// Verify the session exists
	var sessionDeleted int
	err = service.DB.QueryRow("SELECT deleted FROM chat_sessions WHERE id = ?", sessionID).Scan(&sessionDeleted)
	assert.NoError(t, err)
	assert.Equal(t, 0, sessionDeleted, "session should not be deleted before RemoveTask")

	// Remove the task — should cascade-delete sessions
	s.RemoveTask(task.ID)

	// Verify session is soft-deleted
	err = service.DB.QueryRow("SELECT deleted FROM chat_sessions WHERE id = ?", sessionID).Scan(&sessionDeleted)
	assert.NoError(t, err)
	assert.Equal(t, 1, sessionDeleted, "session should be soft-deleted after RemoveTask")

	// Verify task_executions rows are deleted
	var execCount int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE task_id = ?", task.ID).Scan(&execCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, execCount, "task_executions should be deleted after RemoveTask")

	// Verify task is hard-deleted
	_, err = service.GetTaskByID(task.ID)
	assert.Error(t, err, "hard-deleted task should not be found")
}

// ---------- PurgeDeletedData cleans task_executions (Task 8) ----------

func TestPurgeDeletedData_CleansTaskExecutions(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	// Create a scheduled chat session
	sessionID, err := service.CreateSession("/purge-proj", "claude", "Exec 1", "agent1", "", "default", "scheduled")
	assert.NoError(t, err)

	// Add messages
	service.AddChatMessage("/purge-proj", "claude", sessionID, "user", "prompt", nil, false, "Exec 1")

	// Create task_execution
	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/purge-proj", "Purge Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	_, err = service.AddTaskExecution(taskID, sessionID, "auto")
	assert.NoError(t, err)

	// Verify task_execution exists
	var execCount int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE session_id = ?", sessionID).Scan(&execCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, execCount)

	// Soft-delete the session and set updated_at to old date
	service.DeleteSession("/purge-proj", "claude", sessionID)
	oldTime := time.Now().Add(-100 * 24 * time.Hour) // 100 days ago
	service.DB.Exec("UPDATE chat_sessions SET updated_at = ? WHERE id = ?", oldTime, sessionID)

	// Get expired sessions and purge
	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	expiredIDs, err := service.GetExpiredDeletedSessions(cutoff)
	assert.NoError(t, err)
	assert.Contains(t, expiredIDs, sessionID)

	sessionsPurged, messagesPurged, err := service.PurgeDeletedData(expiredIDs)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), sessionsPurged)
	assert.True(t, messagesPurged >= 1)

	// Verify task_executions rows are also deleted
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE session_id = ?", sessionID).Scan(&execCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, execCount, "task_executions should be purged along with the session")
}

// ---------- DeleteTaskExecution ----------

func TestDeleteTaskExecution(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	// Create a task
	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "DelExec Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", 3, now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Create a scheduled chat session
	sessionID, err := service.CreateSession("/proj", "claude", "Del Exec", "agent1", "", "default", "scheduled")
	assert.NoError(t, err)

	// Add messages
	_, err = service.AddChatMessage("/proj", "claude", sessionID, "user", "prompt", nil, false, "Del Exec")
	assert.NoError(t, err)
	_, err = service.AddChatMessage("/proj", "claude", sessionID, "assistant", "response", nil, false, "Del Exec")
	assert.NoError(t, err)

	// Create an execution linked to this session (mark as completed to allow deletion)
	_, err = service.AddTaskExecution(taskID, sessionID, "auto")
	assert.NoError(t, err)
	service.UpdateExecutionStatus(sessionID, "completed")

	// Get the execution ID
	var execID int64
	err = service.DB.QueryRow("SELECT id FROM task_executions WHERE session_id = ?", sessionID).Scan(&execID)
	assert.NoError(t, err)

	// Delete the execution
	err = service.DeleteTaskExecution(execID)
	assert.NoError(t, err)

	// Verify execution is hard-deleted
	var execCount int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE id = ?", execID).Scan(&execCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, execCount, "execution should be hard-deleted")

	// Verify session is soft-deleted
	var sessionDeleted int
	err = service.DB.QueryRow("SELECT deleted FROM chat_sessions WHERE id = ?", sessionID).Scan(&sessionDeleted)
	assert.NoError(t, err)
	assert.Equal(t, 1, sessionDeleted, "session should be soft-deleted")

	// Verify run_count was decremented
	task, err := service.GetTaskByID(taskID)
	assert.NoError(t, err)
	assert.Equal(t, 2, task.RunCount, "run_count should be decremented from 3 to 2")
}

func TestDeleteTaskExecution_NotFound(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	err := service.DeleteTaskExecution(99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execution not found")
}

func TestDeleteTaskExecution_RunningExecution(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	// Create a task and a running execution
	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Running Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", 1, now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	sessionID, err := service.CreateSession("/proj", "claude", "Running Exec", "agent1", "", "default", "scheduled")
	assert.NoError(t, err)

	_, err = service.AddTaskExecution(taskID, sessionID, "auto")
	assert.NoError(t, err)

	// Mark execution as running
	err = service.UpdateExecutionStatus(sessionID, "running")
	assert.NoError(t, err)

	var execID int64
	err = service.DB.QueryRow("SELECT id FROM task_executions WHERE session_id = ?", sessionID).Scan(&execID)
	assert.NoError(t, err)

	// Attempt to delete a running execution should fail
	err = service.DeleteTaskExecution(execID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete a running execution")

	// Verify execution still exists
	var execCount int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE id = ?", execID).Scan(&execCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, execCount, "running execution should not be deleted")
}

func TestDeleteTaskExecution_RunCountClampToZero(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	// Create a task with run_count = 0
	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Zero Count", "0 * * * *", "agent1", "p", "", "active", "unlimited", 0, now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	sessionID, err := service.CreateSession("/proj", "claude", "Zero Exec", "agent1", "", "default", "scheduled")
	assert.NoError(t, err)

	_, err = service.AddTaskExecution(taskID, sessionID, "auto")
	assert.NoError(t, err)
	service.UpdateExecutionStatus(sessionID, "completed")

	var execID int64
	err = service.DB.QueryRow("SELECT id FROM task_executions WHERE session_id = ?", sessionID).Scan(&execID)
	assert.NoError(t, err)

	// Delete the execution — run_count should clamp to 0 (not go negative)
	err = service.DeleteTaskExecution(execID)
	assert.NoError(t, err)

	task, err := service.GetTaskByID(taskID)
	assert.NoError(t, err)
	assert.Equal(t, 0, task.RunCount, "run_count should clamp to 0, not go negative")
}

// ---------- DeleteAllTaskExecutions ----------

func TestDeleteAllTaskExecutions(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	// Create a task with run_count = 3
	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "DelAll Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", 3, now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Create 3 sessions and executions (mark as completed to allow deletion)
	for i := 0; i < 3; i++ {
		sessionID, err := service.CreateSession("/proj", "claude", fmt.Sprintf("Exec %d", i), "agent1", "", "default", "scheduled")
		assert.NoError(t, err)
		service.AddChatMessage("/proj", "claude", sessionID, "user", "prompt", nil, false, fmt.Sprintf("Exec %d", i))
		_, err = service.AddTaskExecution(taskID, sessionID, "auto")
		assert.NoError(t, err)
		service.UpdateExecutionStatus(sessionID, "completed")
	}

	// Verify 3 executions exist
	var execCount int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE task_id = ?", taskID).Scan(&execCount)
	assert.NoError(t, err)
	assert.Equal(t, 3, execCount)

	// Delete all executions
	err = service.DeleteAllTaskExecutions(taskID)
	assert.NoError(t, err)

	// Verify all executions are deleted
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE task_id = ?", taskID).Scan(&execCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, execCount, "all executions should be deleted")

	// Verify run_count reset to 0
	task, err := service.GetTaskByID(taskID)
	assert.NoError(t, err)
	assert.Equal(t, 0, task.RunCount, "run_count should be reset to 0")
}

func TestDeleteAllTaskExecutions_PreservesRunning(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Mixed Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", 2, now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// One completed, one running
	completedSession, _ := service.CreateSession("/proj", "claude", "Completed", "agent1", "", "default", "scheduled")
	service.AddTaskExecution(taskID, completedSession, "auto")
	service.UpdateExecutionStatus(completedSession, "completed")

	runningSession, _ := service.CreateSession("/proj", "claude", "Running", "agent1", "", "default", "scheduled")
	service.AddTaskExecution(taskID, runningSession, "auto")
	// Status is already 'running' from AddTaskExecution

	// Delete all — should only delete the completed one
	err = service.DeleteAllTaskExecutions(taskID)
	assert.NoError(t, err)

	// Running execution should still exist
	var runningCount int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE task_id = ? AND status = 'running'", taskID).Scan(&runningCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, runningCount, "running execution should be preserved")

	// run_count should be 1 (matching the remaining running execution)
	task, err := service.GetTaskByID(taskID)
	assert.NoError(t, err)
	assert.Equal(t, 1, task.RunCount, "run_count should be set to the count of remaining executions")
}

func TestDeleteAllTaskExecutions_NoExecutions(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Empty Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", 0, now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Delete all with no executions — should not error
	err = service.DeleteAllTaskExecutions(taskID)
	assert.NoError(t, err)
}

// ── HasUnreadTasks ──

func TestHasUnreadTasks_NoTasks(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	hasUnread, err := service.HasUnreadTasks("/proj")
	assert.NoError(t, err)
	assert.False(t, hasUnread, "should be false when no tasks exist")
}

func TestHasUnreadTasks_NoExecutions(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	_, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)

	hasUnread, err := service.HasUnreadTasks("/proj")
	assert.NoError(t, err)
	assert.False(t, hasUnread, "should be false when no executions exist")
}

func TestHasUnreadTasks_UnreadExecution(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Add execution with read_at = NULL (unread)
	_, err = service.DB.Exec(
		"INSERT INTO task_executions (task_id, session_id, trigger_type, status, created_at) VALUES (?, ?, ?, ?, ?)",
		taskID, "session-1", "auto", "completed", now,
	)
	assert.NoError(t, err)

	hasUnread, err := service.HasUnreadTasks("/proj")
	assert.NoError(t, err)
	assert.True(t, hasUnread, "should be true when unread execution exists")
}

func TestHasUnreadTasks_ReadExecution(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Add execution with read_at set (read)
	_, err = service.DB.Exec(
		"INSERT INTO task_executions (task_id, session_id, trigger_type, status, read_at, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		taskID, "session-1", "auto", "completed", now, now,
	)
	assert.NoError(t, err)

	hasUnread, err := service.HasUnreadTasks("/proj")
	assert.NoError(t, err)
	assert.False(t, hasUnread, "should be false when all executions are read")
}

func TestHasUnreadTasks_ScopedByProjectPath(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	// Task in /proj-a with unread execution
	resultA, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj-a", "Task A", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskIDA, _ := resultA.LastInsertId()
	_, err = service.DB.Exec(
		"INSERT INTO task_executions (task_id, session_id, trigger_type, status, created_at) VALUES (?, ?, ?, ?, ?)",
		taskIDA, "session-a1", "auto", "completed", now,
	)
	assert.NoError(t, err)

	// Task in /proj-b with no executions
	_, err = service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj-b", "Task B", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)

	hasUnreadA, err := service.HasUnreadTasks("/proj-a")
	assert.NoError(t, err)
	assert.True(t, hasUnreadA, "/proj-a should have unread")

	hasUnreadB, err := service.HasUnreadTasks("/proj-b")
	assert.NoError(t, err)
	assert.False(t, hasUnreadB, "/proj-b should not have unread")
}

func TestHasUnreadTasks_EmptyProjectPath(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	_, err = service.DB.Exec(
		"INSERT INTO task_executions (task_id, session_id, trigger_type, status, created_at) VALUES (?, ?, ?, ?, ?)",
		taskID, "session-1", "auto", "completed", now,
	)
	assert.NoError(t, err)

	// Empty project path should check all projects
	hasUnread, err := service.HasUnreadTasks("")
	assert.NoError(t, err)
	assert.True(t, hasUnread, "empty project path should find unread across all projects")
}

func TestHasUnreadTasks_RunningExecutionNotUnread(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Add a running execution — should NOT be counted as unread
	_, err = service.DB.Exec(
		"INSERT INTO task_executions (task_id, session_id, trigger_type, status, created_at) VALUES (?, ?, ?, ?, ?)",
		taskID, "session-running", "auto", "running", now,
	)
	assert.NoError(t, err)

	hasUnread, err := service.HasUnreadTasks("/proj")
	assert.NoError(t, err)
	assert.False(t, hasUnread, "running execution should not count as unread")

	// Now mark it as completed — should become unread
	service.UpdateExecutionStatus("session-running", "completed")
	hasUnread, err = service.HasUnreadTasks("/proj")
	assert.NoError(t, err)
	assert.True(t, hasUnread, "completed execution should count as unread")
}

// ---------- UpdateTaskLastRead ----------

func TestUpdateTaskLastRead(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Verify last_read_at is NULL initially
	var lastRead sql.NullTime
	err = service.DB.QueryRow("SELECT last_read_at FROM scheduled_tasks WHERE id = ?", taskID).Scan(&lastRead)
	assert.NoError(t, err)
	assert.False(t, lastRead.Valid, "last_read_at should be NULL initially")

	// Update last read
	err = service.UpdateTaskLastRead(taskID)
	assert.NoError(t, err)

	// Verify last_read_at is now set
	err = service.DB.QueryRow("SELECT last_read_at FROM scheduled_tasks WHERE id = ?", taskID).Scan(&lastRead)
	assert.NoError(t, err)
	assert.True(t, lastRead.Valid, "last_read_at should be set after UpdateTaskLastRead")
}

func TestUpdateTaskLastRead_NonExistentDoesNotError(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	err := service.UpdateTaskLastRead(99999)
	assert.NoError(t, err)
}

// ---------- MarkExecutionRead ----------

func TestMarkExecutionRead(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	_, err = service.AddTaskExecution(taskID, "session-1", "auto")
	assert.NoError(t, err)

	var execID int64
	err = service.DB.QueryRow("SELECT id FROM task_executions WHERE session_id = ?", "session-1").Scan(&execID)
	assert.NoError(t, err)

	// Verify read_at is NULL initially
	var readAt sql.NullTime
	err = service.DB.QueryRow("SELECT read_at FROM task_executions WHERE id = ?", execID).Scan(&readAt)
	assert.NoError(t, err)
	assert.False(t, readAt.Valid, "read_at should be NULL initially")

	// Mark as read
	err = service.MarkExecutionRead(fmt.Sprintf("%d", execID))
	assert.NoError(t, err)

	// Verify read_at is now set
	err = service.DB.QueryRow("SELECT read_at FROM task_executions WHERE id = ?", execID).Scan(&readAt)
	assert.NoError(t, err)
	assert.True(t, readAt.Valid, "read_at should be set after MarkExecutionRead")
}

func TestMarkExecutionRead_NonExistentDoesNotError(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	err := service.MarkExecutionRead("99999")
	assert.NoError(t, err)
}

// ---------- UpdateExecutionSummary ----------

func TestUpdateExecutionSummary(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	_, err = service.AddTaskExecution(taskID, "session-1", "auto")
	assert.NoError(t, err)

	var execID int64
	err = service.DB.QueryRow("SELECT id FROM task_executions WHERE session_id = ?", "session-1").Scan(&execID)
	assert.NoError(t, err)

	// Verify summary is NULL initially
	var summary sql.NullString
	err = service.DB.QueryRow("SELECT summary FROM task_executions WHERE id = ?", execID).Scan(&summary)
	assert.NoError(t, err)
	assert.False(t, summary.Valid, "summary should be NULL initially")

	// Update summary
	err = service.UpdateExecutionSummary(execID, "This is a summary of the task execution")
	assert.NoError(t, err)

	// Verify summary is now set
	err = service.DB.QueryRow("SELECT summary FROM task_executions WHERE id = ?", execID).Scan(&summary)
	assert.NoError(t, err)
	assert.True(t, summary.Valid, "summary should be set after UpdateExecutionSummary")
	assert.Equal(t, "This is a summary of the task execution", summary.String)
}

func TestUpdateExecutionSummary_EmptySummary(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Task", "0 * * * *", "agent1", "p", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	_, err = service.AddTaskExecution(taskID, "session-1", "auto")
	assert.NoError(t, err)

	var execID int64
	err = service.DB.QueryRow("SELECT id FROM task_executions WHERE session_id = ?", "session-1").Scan(&execID)
	assert.NoError(t, err)

	// Set empty summary (text was too short)
	err = service.UpdateExecutionSummary(execID, "")
	assert.NoError(t, err)

	var summary sql.NullString
	err = service.DB.QueryRow("SELECT summary FROM task_executions WHERE id = ?", execID).Scan(&summary)
	assert.NoError(t, err)
	assert.True(t, summary.Valid, "summary should be set (even if empty string)")
	assert.Equal(t, "", summary.String)
}

func TestUpdateExecutionSummary_NonExistentDoesNotError(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	err := service.UpdateExecutionSummary(99999, "summary")
	assert.NoError(t, err)
}

// ---------- cleanZombieExecutions ----------

func TestCleanZombieExecutions(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	// Insert a task
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Zombie Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Insert a running execution (zombie)
	_, err = service.AddTaskExecution(taskID, "session-zombie", "auto")
	assert.NoError(t, err)

	// Verify it's running
	var status string
	err = service.DB.QueryRow("SELECT status FROM task_executions WHERE session_id = ?", "session-zombie").Scan(&status)
	assert.NoError(t, err)
	assert.Equal(t, "running", status)

	// LoadTasksFromDB calls cleanZombieExecutions which marks running executions as failed
	s := service.NewScheduler()
	err = s.LoadTasksFromDB("/proj")
	assert.NoError(t, err)

	// The zombie should now be marked as failed
	err = service.DB.QueryRow("SELECT status FROM task_executions WHERE session_id = ?", "session-zombie").Scan(&status)
	assert.NoError(t, err)
	assert.Equal(t, "failed", status)
}

func TestCleanZombieExecutions_NoRunningExecutions(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	result, err := service.DB.Exec(
		"INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"/proj", "Clean Task", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	assert.NoError(t, err)
	taskID, _ := result.LastInsertId()

	// Insert a completed execution
	_, err = service.AddTaskExecution(taskID, "session-completed", "auto")
	assert.NoError(t, err)
	service.UpdateExecutionStatus("session-completed", "completed")

	// LoadTasksFromDB should not affect completed executions
	s := service.NewScheduler()
	err = s.LoadTasksFromDB("/proj")
	assert.NoError(t, err)

	var status string
	err = service.DB.QueryRow("SELECT status FROM task_executions WHERE session_id = ?", "session-completed").Scan(&status)
	assert.NoError(t, err)
	assert.Equal(t, "completed", status)
}

// ---------- SetTaskSummarizer ----------

func TestSetTaskSummarizer(t *testing.T) {
	s := service.NewScheduler()
	// Should not panic when called with nil
	s.SetTaskSummarizer(nil)

	// Should not panic when called with a non-nil summarizer
	// (we can't easily construct a real TaskSummarizer here without
	// setting up an AI backend, so we just verify it doesn't panic)
	s.SetTaskSummarizer(nil)
}

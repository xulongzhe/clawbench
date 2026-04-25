package service_test

import (
	"database/sql"
	"strings"
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
	file_path TEXT,
	files TEXT,
	session_id TEXT,
	backend TEXT NOT NULL DEFAULT 'claude',
	streaming INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS chat_sessions (
	id TEXT PRIMARY KEY,
	project_path TEXT NOT NULL,
	backend TEXT NOT NULL,
	title TEXT NOT NULL,
	agent_id TEXT DEFAULT '',
	model TEXT DEFAULT '',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	last_read_at DATETIME,
	UNIQUE(project_path, backend, id)
);
CREATE TABLE IF NOT EXISTS scheduled_tasks (
	id TEXT PRIMARY KEY,
	project_path TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT,
	cron_expr TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	prompt TEXT NOT NULL,
	session_id TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	repeat_mode TEXT NOT NULL DEFAULT 'always',
	max_runs INTEGER DEFAULT 0,
	last_run_at DATETIME,
	next_run_at DATETIME,
	run_count INTEGER DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS task_executions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id TEXT NOT NULL,
	message_id INTEGER NOT NULL REFERENCES chat_history(id),
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_executions_task ON task_executions(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_history_session ON chat_history(project_path, backend, session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_project_backend ON chat_sessions(project_path, backend);
`

func setupSchedulerDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	assert.NoError(t, err)
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
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-1", "/proj1", "Task 1", "", "0 * * * *", "agent1", "prompt1", "", "active", "unlimited", now, now,
	)
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-2", "/proj2", "Task 2", "", "0 * * * *", "agent1", "prompt2", "", "active", "unlimited", now, now,
	)

	tasks, err := service.GetTasks("")
	assert.NoError(t, err)
	assert.Len(t, tasks, 2)

	tasks, err = service.GetTasks("/proj1")
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "task-1", tasks[0].ID)
}

func TestGetTasks_ExcludesDeleted(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-1", "/proj", "Active", "", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-2", "/proj", "Deleted", "", "0 * * * *", "agent1", "p", "", "deleted", "unlimited", now, now,
	)

	tasks, err := service.GetTasks("/proj")
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "task-1", tasks[0].ID)
}

func TestGetTasks_OrdersByCreatedAtDesc(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-1", "/proj", "First", "", "0 * * * *", "agent1", "p", "", "active", "unlimited", now.Add(-1*time.Hour), now,
	)
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-2", "/proj", "Second", "", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)

	tasks, err := service.GetTasks("/proj")
	assert.NoError(t, err)
	assert.Len(t, tasks, 2)
	assert.Equal(t, "task-2", tasks[0].ID) // newer first
	assert.Equal(t, "task-1", tasks[1].ID)
}

// ---------- GetTaskByID ----------

func TestGetTaskByID(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, max_runs, run_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-1", "/proj", "Task 1", "a description", "0 * * * *", "agent1", "prompt1", "sess-1", "active", "unlimited", 0, 3, now, now,
	)

	task, err := service.GetTaskByID("task-1")
	assert.NoError(t, err)
	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, "/proj", task.ProjectPath)
	assert.Equal(t, "Task 1", task.Name)
	assert.Equal(t, "a description", task.Description)
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

	_, err := service.GetTaskByID("non-existent")
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
	assert.NotEmpty(t, task.ID, "ID should be auto-generated")
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

func TestAddTask_WithExistingID(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask(func(t *model.ScheduledTask) {
		t.ID = "my-custom-id"
	})
	err := s.AddTask(task)
	assert.NoError(t, err)
	assert.Equal(t, "my-custom-id", task.ID, "ID should be preserved")

	persisted, err := service.GetTaskByID("my-custom-id")
	assert.NoError(t, err)
	assert.Equal(t, "my-custom-id", persisted.ID)
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

func TestAddTask_GeneratedIDFormat(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))
	assert.True(t, strings.HasPrefix(task.ID, "task-"), "generated ID should start with 'task-'")
}

// ---------- RemoveTask ----------

func TestRemoveTask(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	s.RemoveTask(task.ID)

	// Task should be marked as deleted in DB
	persisted, err := service.GetTaskByID(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, "deleted", persisted.Status)

	// Should not appear in GetTasks
	tasks, err := service.GetTasks(task.ProjectPath)
	assert.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestRemoveTask_NonExistentDoesNotPanic(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	// Removing a task that was never added should not panic
	s.RemoveTask("non-existent-id")
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

	s.PauseTask("non-existent-id")
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

	err := s.ResumeTask("non-existent-id")
	assert.Error(t, err)
}

func TestResumeTask_AfterRemove(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	s.RemoveTask(task.ID)

	// Deleted task is not paused, so resume should fail
	err := s.ResumeTask(task.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not paused")
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
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-1", "/proj", "Active Task", "", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-2", "/proj", "Paused Task", "", "0 * * * *", "agent1", "p", "", "paused", "unlimited", now, now,
	)

	err := s.LoadTasksFromDB("/proj")
	assert.NoError(t, err)

	// Active task should be loaded; paused task should be skipped
	// We verify by checking that the active task can be removed without error
	s.RemoveTask("task-1")

	persisted, _ := service.GetTaskByID("task-1")
	assert.Equal(t, "deleted", persisted.Status)
}

func TestLoadTasksFromDB_AllProjects(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-1", "/proj1", "Task 1", "", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-2", "/proj2", "Task 2", "", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)

	err := s.LoadTasksFromDB("") // empty = all projects
	assert.NoError(t, err)

	// Both tasks should be loaded
	s.RemoveTask("task-1")
	s.RemoveTask("task-2")

	p1, _ := service.GetTaskByID("task-1")
	p2, _ := service.GetTaskByID("task-2")
	assert.Equal(t, "deleted", p1.Status)
	assert.Equal(t, "deleted", p2.Status)
}

func TestLoadTasksFromDB_InvalidCronSkipped(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-bad", "/proj", "Bad Cron", "", "invalid", "agent1", "p", "", "active", "unlimited", now, now,
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

	// Insert a chat_history row to reference
	res, err := service.DB.Exec(
		"INSERT INTO chat_history (project_path, role, content, backend) VALUES (?, ?, ?, ?)",
		"/proj", "user", "test message", "claude",
	)
	assert.NoError(t, err)
	messageID, _ := res.LastInsertId()

	// Insert a task
	now := time.Now()
	service.DB.Exec(
		"INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"task-1", "/proj", "Task", "", "0 * * * *", "agent1", "p", "", "active", "unlimited", now, now,
	)

	err = service.AddTaskExecution("task-1", messageID)
	assert.NoError(t, err)

	// Verify the execution was recorded
	var count int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE task_id = ?", "task-1").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	var fetchedMsgID int64
	err = service.DB.QueryRow("SELECT message_id FROM task_executions WHERE task_id = ?", "task-1").Scan(&fetchedMsgID)
	assert.NoError(t, err)
	assert.Equal(t, messageID, fetchedMsgID)
}

func TestAddTaskExecution_MultipleExecutions(t *testing.T) {
	_, cleanup := setupScheduler(t)
	defer cleanup()

	res, _ := service.DB.Exec(
		"INSERT INTO chat_history (project_path, role, content, backend) VALUES (?, ?, ?, ?)",
		"/proj", "user", "msg1", "claude",
	)
	msgID1, _ := res.LastInsertId()

	res, _ = service.DB.Exec(
		"INSERT INTO chat_history (project_path, role, content, backend) VALUES (?, ?, ?, ?)",
		"/proj", "assistant", "msg2", "claude",
	)
	msgID2, _ := res.LastInsertId()

	err := service.AddTaskExecution("task-1", msgID1)
	assert.NoError(t, err)
	err = service.AddTaskExecution("task-1", msgID2)
	assert.NoError(t, err)

	var count int
	err = service.DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE task_id = ?", "task-1").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

// ---------- saveTask (tested indirectly via AddTask / UpdateTask) ----------

func TestSaveTask_Upsert(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask(func(t *model.ScheduledTask) {
		t.ID = "upsert-test"
		t.Name = "Original"
	})
	assert.NoError(t, s.AddTask(task))

	// Update via UpdateTask (which calls saveTask internally)
	task.Name = "Updated"
	assert.NoError(t, s.UpdateTask(task))

	persisted, err := service.GetTaskByID("upsert-test")
	assert.NoError(t, err)
	assert.Equal(t, "Updated", persisted.Name)
}

// ---------- generateTaskID (tested indirectly via AddTask) ----------

func TestGenerateTaskID_Format(t *testing.T) {
	s, cleanup := setupScheduler(t)
	defer cleanup()

	task := helperTask()
	assert.NoError(t, s.AddTask(task))

	// ID should start with "task-" and have UUID-like hex segments
	assert.True(t, strings.HasPrefix(task.ID, "task-"))
	parts := strings.TrimPrefix(task.ID, "task-")
	// Format: xxxx-xx-xx-xx-xxxxxxxxxxxx
	segments := strings.Split(parts, "-")
	assert.Len(t, segments, 5)
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
	removed, _ := service.GetTaskByID(task.ID)
	assert.Equal(t, "deleted", removed.Status)
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

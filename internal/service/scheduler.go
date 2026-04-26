package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"clawbench/internal/ai"
	"clawbench/internal/model"

	"github.com/robfig/cron/v3"
)

// GlobalScheduler is the singleton scheduler instance, set during startup.
var GlobalScheduler *Scheduler

// Scheduler manages cron-scheduled AI tasks.
type Scheduler struct {
	cron    *cron.Cron
	entries map[string]cron.EntryID // task ID -> cron entry ID
	mu      sync.Mutex
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler() *Scheduler {
	return &Scheduler{
		cron:    cron.New(cron.WithSeconds()), // support second-level precision
		entries: make(map[string]cron.EntryID),
	}
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	slog.Info("scheduler started")
}

// Stop halts the cron scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	slog.Info("scheduler stopped")
}

// LoadTasksFromDB loads active tasks from the database and registers them.
// If projectPath is empty, loads tasks from all projects.
func (s *Scheduler) LoadTasksFromDB(projectPath string) error {
	tasks, err := GetTasks(projectPath)
	if err != nil {
		return err
	}
	for i := range tasks {
		task := &tasks[i]
		if task.Status != "active" {
			continue
		}
		if err := s.registerTask(task); err != nil {
			slog.Warn("failed to register task on load",
				slog.String("task_id", task.ID),
				slog.String("err", err.Error()),
			)
		}
	}
	return nil
}

// AddTask creates a new scheduled task, persists it, and registers it with cron.
func (s *Scheduler) AddTask(task *model.ScheduledTask) error {
	if task.ID == "" {
		task.ID = generateTaskID()
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	task.Status = "active"

	// Calculate next run time
	schedule, err := cron.ParseStandard(task.CronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	nextRun := schedule.Next(now)
	task.NextRunAt = &nextRun

	// Persist to database
	if err := saveTask(task); err != nil {
		return err
	}

	// Register with cron
	return s.registerTask(task)
}

// RemoveTask removes a task from cron and marks it as deleted in the database.
func (s *Scheduler) RemoveTask(id string) {
	s.mu.Lock()
	if entryID, ok := s.entries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}
	s.mu.Unlock()

	DB.Exec("UPDATE scheduled_tasks SET status = 'deleted', updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
}

// PauseTask removes a task from cron but keeps it in the database as paused.
func (s *Scheduler) PauseTask(id string) {
	s.mu.Lock()
	if entryID, ok := s.entries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}
	s.mu.Unlock()

	DB.Exec("UPDATE scheduled_tasks SET status = 'paused', updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
}

// ResumeTask re-registers a paused task with cron.
func (s *Scheduler) ResumeTask(id string) error {
	task, err := GetTaskByID(id)
	if err != nil {
		return err
	}
	if task.Status != "paused" {
		return fmt.Errorf("task is not paused")
	}

	DB.Exec("UPDATE scheduled_tasks SET status = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	task.Status = "active"

	return s.registerTask(task)
}

// UpdateTask updates an existing task's configuration and re-registers if needed.
func (s *Scheduler) UpdateTask(task *model.ScheduledTask) error {
	// Update timestamp
	task.UpdatedAt = time.Now()

	// Recalculate next run time if cron expression changed
	schedule, err := cron.ParseStandard(task.CronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	nextRun := schedule.Next(time.Now())
	task.NextRunAt = &nextRun

	// If task is active, remove old cron entry and re-register atomically
	if task.Status == "active" {
		s.mu.Lock()
		if entryID, ok := s.entries[task.ID]; ok {
			s.cron.Remove(entryID)
			delete(s.entries, task.ID)
		}
		// Re-register while holding lock to ensure atomicity
		if err := s.registerTaskLocked(task); err != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to re-register task: %w", err)
		}
		s.mu.Unlock()
	}

	// Persist to database
	if err := saveTask(task); err != nil {
		return err
	}

	slog.Info("updated task",
		slog.String("task_id", task.ID),
		slog.String("name", task.Name),
		slog.String("status", task.Status),
	)
	return nil
}

// registerTask adds a task's cron job to the scheduler.
func (s *Scheduler) registerTask(task *model.ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.registerTaskLocked(task)
}

// registerTaskLocked adds a task's cron job to the scheduler.
// The caller must hold s.mu lock.
func (s *Scheduler) registerTaskLocked(task *model.ScheduledTask) error {
	schedule, err := cron.ParseStandard(task.CronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// Capture task by value for the closure
	taskID := task.ID
	projectPath := task.ProjectPath

	entryID := s.cron.Schedule(schedule, cron.FuncJob(func() {
		// Reload task from DB to get latest state
		current, err := GetTaskByID(taskID)
		if err != nil || current.Status != "active" {
			return
		}
		s.executeTask(current, projectPath)
	}))

	// Lock is already held by caller
	s.entries[taskID] = entryID

	slog.Info("registered cron task",
		slog.String("task_id", taskID),
		slog.String("cron", task.CronExpr),
	)
	return nil
}

// executeTask runs a scheduled task by invoking the AI backend and inserting
// the result as an assistant message in the original session.
func (s *Scheduler) executeTask(task *model.ScheduledTask, projectPath string) {
	slog.Info("executing scheduled task",
		slog.String("task_id", task.ID),
		slog.String("name", task.Name),
	)

	agent, ok := model.Agents[task.AgentID]
	if !ok {
		slog.Error("agent not found for task", slog.String("agent_id", task.AgentID))
		return
	}

	backendName := agent.Backend
	if backendName == "" {
		backendName = "codebuddy"
	}

	// Build chat request — no session resume, standalone execution
	systemPrompt := agent.SystemPrompt

	chatReq := ai.ChatRequest{
		Prompt:       task.Prompt,
		SessionID:    "", // no session — standalone execution
		WorkDir:      projectPath,
		SystemPrompt: systemPrompt,
		Model:        agent.Model,
		AgentID:      task.AgentID,
		Resume:       false,
	}

	// Execute AI backend (no timeout - let AI run indefinitely)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := ai.NewBackend(backendName)
	if err != nil {
		slog.Error("failed to create backend for task", slog.String("err", err.Error()))
		return
	}

	eventCh, err := backend.ExecuteStream(ctx, chatReq)
	if err != nil {
		slog.Error("failed to execute stream for task", slog.String("err", err.Error()))
		return
	}

	// Consume streaming events and build content blocks
	var blocks []model.ContentBlock
	var currentText strings.Builder
	var currentThinking strings.Builder
	var responseMetadata *ai.Metadata

	for event := range eventCh {
		switch event.Type {
		case "content":
			if currentThinking.Len() > 0 {
				blocks = append(blocks, model.ContentBlock{Type: "thinking", Text: currentThinking.String()})
				currentThinking.Reset()
			}
			currentText.WriteString(event.Content)
		case "thinking":
			if currentText.Len() > 0 {
				blocks = append(blocks, model.ContentBlock{Type: "text", Text: currentText.String()})
				currentText.Reset()
			}
			currentThinking.WriteString(event.Content)
		case "tool_use":
			if currentText.Len() > 0 {
				blocks = append(blocks, model.ContentBlock{Type: "text", Text: currentText.String()})
				currentText.Reset()
			}
			if currentThinking.Len() > 0 {
				blocks = append(blocks, model.ContentBlock{Type: "thinking", Text: currentThinking.String()})
				currentThinking.Reset()
			}
			if event.Tool != nil {
				inputMap := make(map[string]any)
				if event.Tool.Input != "" {
					json.Unmarshal([]byte(event.Tool.Input), &inputMap)
				}
				blocks = append(blocks, model.ContentBlock{
					Type:  "tool_use",
					Name:  event.Tool.Name,
					ID:    event.Tool.ID,
					Input: inputMap,
				})
			}
		case "metadata":
			if event.Meta != nil {
				responseMetadata = event.Meta
			}
		case "done", "error":
			// Terminal events
		}
	}

	// Flush remaining text/thinking
	if currentText.Len() > 0 {
		blocks = append(blocks, model.ContentBlock{Type: "text", Text: currentText.String()})
	}
	if currentThinking.Len() > 0 {
		blocks = append(blocks, model.ContentBlock{Type: "thinking", Text: currentThinking.String()})
	}

	// Build content JSON
	contentMap := map[string]any{"blocks": blocks}
	if responseMetadata != nil {
		contentMap["metadata"] = responseMetadata
	}
	// Attach scheduled task info so the frontend can render a distinctive style
	contentMap["scheduledTask"] = map[string]any{
		"taskId":   task.ID,
		"taskName": task.Name,
		"cronExpr": task.CronExpr,
		"agentId":  task.AgentID,
	}
	contentJSON, _ := json.Marshal(contentMap)

	// Save assistant message and record execution
	sessionID := task.SessionID
	if sessionID == "" {
		sessionID = "task-" + task.ID
	}
	messageID, err := AddChatMessage(projectPath, backendName, sessionID, "assistant", string(contentJSON), "", nil, false)
	if err != nil {
		slog.Error("failed to save assistant message for task", slog.String("err", err.Error()))
	} else {
		if err := AddTaskExecution(task.ID, messageID); err != nil {
			slog.Error("failed to record task execution", slog.String("err", err.Error()))
		}
	}

	// Update task execution stats
	now := time.Now()
	runCount := task.RunCount + 1
	newStatus := task.Status

	// Check repeat mode
	switch task.RepeatMode {
	case "once":
		newStatus = "completed"
	case "limited":
		if runCount >= task.MaxRuns {
			newStatus = "completed"
		}
	}

	schedule, _ := cron.ParseStandard(task.CronExpr)
	var nextRunAt *time.Time
	if newStatus == "active" {
		nr := schedule.Next(now)
		nextRunAt = &nr
	} else {
		// Task completed, remove from cron
		s.mu.Lock()
		if entryID, ok := s.entries[task.ID]; ok {
			s.cron.Remove(entryID)
			delete(s.entries, task.ID)
		}
		s.mu.Unlock()
	}

	if nextRunAt != nil {
		DB.Exec("UPDATE scheduled_tasks SET last_run_at = ?, next_run_at = ?, run_count = ?, status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			now, nextRunAt, runCount, newStatus, task.ID)
	} else {
		DB.Exec("UPDATE scheduled_tasks SET last_run_at = ?, next_run_at = NULL, run_count = ?, status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			now, runCount, newStatus, task.ID)
	}

	slog.Info("task execution completed",
		slog.String("task_id", task.ID),
		slog.Int("run_count", runCount),
		slog.String("status", newStatus),
	)
}

// GetTasks retrieves all tasks for a project path. If projectPath is empty, retrieves all tasks.
func GetTasks(projectPath string) ([]model.ScheduledTask, error) {
	var tasks []model.ScheduledTask
	var query string
	var args []interface{}

	if projectPath == "" {
		query = "SELECT id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, max_runs, last_run_at, next_run_at, run_count, created_at, updated_at FROM scheduled_tasks WHERE status != 'deleted' ORDER BY created_at DESC"
	} else {
		query = "SELECT id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, max_runs, last_run_at, next_run_at, run_count, created_at, updated_at FROM scheduled_tasks WHERE project_path = ? AND status != 'deleted' ORDER BY created_at DESC"
		args = []interface{}{projectPath}
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t model.ScheduledTask
		var lastRun, nextRun sql.NullTime
		if err := rows.Scan(&t.ID, &t.ProjectPath, &t.Name, &t.Description, &t.CronExpr, &t.AgentID, &t.Prompt, &t.SessionID, &t.Status, &t.RepeatMode, &t.MaxRuns, &lastRun, &nextRun, &t.RunCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if lastRun.Valid {
			t.LastRunAt = &lastRun.Time
		}
		if nextRun.Valid {
			t.NextRunAt = &nextRun.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetTaskByID retrieves a single task by its ID.
func GetTaskByID(id string) (*model.ScheduledTask, error) {
	var t model.ScheduledTask
	var lastRun, nextRun sql.NullTime
	err := DB.QueryRow(
		"SELECT id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, max_runs, last_run_at, next_run_at, run_count, created_at, updated_at FROM scheduled_tasks WHERE id = ?",
		id,
	).Scan(&t.ID, &t.ProjectPath, &t.Name, &t.Description, &t.CronExpr, &t.AgentID, &t.Prompt, &t.SessionID, &t.Status, &t.RepeatMode, &t.MaxRuns, &lastRun, &nextRun, &t.RunCount, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if lastRun.Valid {
		t.LastRunAt = &lastRun.Time
	}
	if nextRun.Valid {
		t.NextRunAt = &nextRun.Time
	}
	return &t, nil
}

// saveTask inserts or updates a task in the database.
func saveTask(task *model.ScheduledTask) error {
	_, err := DB.Exec(
		`INSERT INTO scheduled_tasks (id, project_path, name, description, cron_expr, agent_id, prompt, session_id, status, repeat_mode, max_runs, next_run_at, run_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=?, description=?, cron_expr=?, agent_id=?, prompt=?, session_id=?, status=?, repeat_mode=?, max_runs=?, next_run_at=?, run_count=?, updated_at=CURRENT_TIMESTAMP`,
		task.ID, task.ProjectPath, task.Name, task.Description, task.CronExpr, task.AgentID, task.Prompt, task.SessionID, task.Status, task.RepeatMode, task.MaxRuns, task.NextRunAt, task.RunCount, task.CreatedAt, task.UpdatedAt,
		task.Name, task.Description, task.CronExpr, task.AgentID, task.Prompt, task.SessionID, task.Status, task.RepeatMode, task.MaxRuns, task.NextRunAt, task.RunCount,
	)
	return err
}

// AddTaskExecution records that a chat_history message was produced by a scheduled task execution.
func AddTaskExecution(taskID string, messageID int64) error {
	_, err := DB.Exec(
		"INSERT INTO task_executions (task_id, message_id) VALUES (?, ?)",
		taskID, messageID,
	)
	return err
}

// generateTaskID creates a unique ID for a scheduled task.
func generateTaskID() string {
	return generateUUID("task-", "scheduled_tasks", "id")
}

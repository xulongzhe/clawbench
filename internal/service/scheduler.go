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
	"unicode/utf8"

	"clawbench/internal/ai"
	"clawbench/internal/model"
	"clawbench/internal/summarize"
	"clawbench/internal/ws"

	"github.com/robfig/cron/v3"
)

// GlobalScheduler is the singleton scheduler instance, set during startup.
var GlobalScheduler *Scheduler

// RunningExecution tracks a currently executing task instance.
type RunningExecution struct {
	ID          string
	TaskID      int64
	CancelFunc  context.CancelFunc
	StartedAt   time.Time
	TriggerType string // "auto" | "manual"
}

// Scheduler manages cron-scheduled AI tasks.
type Scheduler struct {
	cron              *cron.Cron
	entries           map[int64]cron.EntryID // task ID -> cron entry ID
	mu                sync.Mutex
	runningExecutions sync.Map // key: executionID, value: *RunningExecution
	taskSummarizer    *summarize.TaskSummarizer
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler() *Scheduler {
	return &Scheduler{
		cron:    cron.New(),
		entries: make(map[int64]cron.EntryID),
	}
}

// Start begins the cron scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	slog.Info("scheduler started")
}

// SetTaskSummarizer sets the task summarizer for generating execution summaries.
// Must be called before Start() to ensure all task executions use the summarizer.
func (s *Scheduler) SetTaskSummarizer(ts *summarize.TaskSummarizer) {
	s.taskSummarizer = ts
}

// Stop halts the cron scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	slog.Info("scheduler stopped")
}

// GetRunningExecutions returns the running execution views for a specific task.
func (s *Scheduler) GetRunningExecutions(taskID int64) []model.RunningExecutionView {
	var result []model.RunningExecutionView
	s.runningExecutions.Range(func(key, value any) bool {
		exec := value.(*RunningExecution)
		if exec.TaskID == taskID {
			result = append(result, model.RunningExecutionView{
				ID:          exec.ID,
				StartedAt:   exec.StartedAt,
				TriggerType: exec.TriggerType,
			})
		}
		return true
	})
	return result
}

// GetRunningCounts returns a map of taskID -> running execution count.
func (s *Scheduler) GetRunningCounts() map[int64]int {
	counts := make(map[int64]int)
	s.runningExecutions.Range(func(key, value any) bool {
		exec := value.(*RunningExecution)
		counts[exec.TaskID]++
		return true
	})
	return counts
}

// HasRunningExecutions checks if a task has any running executions.
func (s *Scheduler) HasRunningExecutions(taskID int64) bool {
	found := false
	s.runningExecutions.Range(func(key, value any) bool {
		if value.(*RunningExecution).TaskID == taskID {
			found = true
			return false
		}
		return true
	})
	return found
}

// CancelExecution cancels a specific running execution by its ID.
// Returns error if the execution is not found or already finished.
func (s *Scheduler) CancelExecution(executionID string) error {
	val, ok := s.runningExecutions.Load(executionID)
	if !ok {
		return fmt.Errorf("execution not found: %s", executionID)
	}
	exec := val.(*RunningExecution)
	exec.CancelFunc()
	slog.Info("cancelled running execution",
		slog.String("exec_id", executionID),
		slog.Int64("task_id", exec.TaskID),
	)
	return nil
}

// CancelAllExecutions cancels all running executions for a specific task.
func (s *Scheduler) CancelAllExecutions(taskID int64) {
	s.runningExecutions.Range(func(key, value any) bool {
		exec := value.(*RunningExecution)
		if exec.TaskID == taskID {
			exec.CancelFunc()
			slog.Info("cancelled running execution for task",
				slog.String("exec_id", exec.ID),
				slog.Int64("task_id", taskID),
			)
		}
		return true
	})
}

// LoadTasksFromDB loads active tasks from the database and registers them.
// If projectPath is empty, loads tasks from all projects.
func (s *Scheduler) LoadTasksFromDB(projectPath string) error {
	// Clean up zombie executions: on restart, any execution still marked as
	// "running" in the DB is a zombie (the in-memory runningExecutions map is
	// empty, so no CLI process is actually alive for it). Mark them as "failed".
	s.cleanZombieExecutions()

	tasks, err := GetTasks(projectPath)
	if err != nil {
		return err
	}
	for i := range tasks {
		task := &tasks[i]
		if task.Status != "active" {
			continue
		}
		// Validate agent_id against loaded agents
		if _, ok := model.Agents[task.AgentID]; !ok {
			// Skip registration but do NOT pause — the agent may not be loaded yet
			// (e.g., if LoadAgents hasn't run). The task stays active in DB and
			// will be registered on next restart when agents are available.
			// Runtime validation in executeTask() handles genuinely invalid agents.
			slog.Warn("skipping task with unavailable agent_id",
				slog.Int64("task_id", task.ID),
				slog.String("name", task.Name),
				slog.String("agent_id", task.AgentID),
			)
			continue
		}
		// Detect missed executions: if next_run_at is in the past, the server
		// was likely down when the cron should have fired.
		if task.NextRunAt != nil && task.NextRunAt.Before(time.Now()) {
			slog.Warn("detected missed scheduled execution",
				slog.Int64("task_id", task.ID),
				slog.String("name", task.Name),
				slog.Time("missed_run", *task.NextRunAt),
			)
		}
		if err := s.registerTask(task); err != nil {
			slog.Warn("failed to register task on load",
				slog.Int64("task_id", task.ID),
				slog.String("err", err.Error()),
			)
		}
	}
	return nil
}

// cleanZombieExecutions marks all "running" executions as "failed".
// Called on startup when no in-memory state exists — any DB row still
// marked "running" belongs to a CLI process that died with the previous
// server instance.
func (s *Scheduler) cleanZombieExecutions() {
	result, err := DB.Exec("UPDATE task_executions SET status = 'failed' WHERE status = 'running'")
	if err != nil {
		slog.Error("failed to clean zombie executions", slog.String("err", err.Error()))
		return
	}
	if n, _ := result.RowsAffected(); n > 0 {
		slog.Info("cleaned up zombie task executions",
			slog.Int64("count", n),
		)
	}
}

// AddTask creates a new scheduled task, persists it, and registers it with cron.
func (s *Scheduler) AddTask(task *model.ScheduledTask) error {
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

	// Persist to database (auto-increment ID)
	if err := insertTask(task); err != nil {
		return err
	}

	// Register with cron
	return s.registerTask(task)
}

// RemoveTask removes a task from cron and hard-deletes it from the database.
// Also soft-deletes associated chat sessions and removes task_executions rows.
func (s *Scheduler) RemoveTask(id int64) {
	s.mu.Lock()
	if entryID, ok := s.entries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}
	s.mu.Unlock()

	// Cascade: soft-delete associated chat sessions
	rows, err := DB.Query(`
		SELECT te.session_id, cs.project_path, cs.backend
		FROM task_executions te
		JOIN chat_sessions cs ON cs.id = te.session_id
		WHERE te.task_id = ?`, id)
	if err != nil {
		slog.Error("failed to query sessions for task removal",
			slog.Int64("task_id", id),
			slog.String("err", err.Error()),
		)
	} else {
		// Collect all sessions first before updating (avoids deadlock with SetMaxOpenConns(1))
		type sessionInfo struct {
			sessionID   string
			projectPath string
			backend     string
		}
		var sessions []sessionInfo
		for rows.Next() {
			var si sessionInfo
			if rows.Scan(&si.sessionID, &si.projectPath, &si.backend) == nil {
				sessions = append(sessions, si)
			}
		}
		rows.Close()

		// Now soft-delete each session
		for _, si := range sessions {
			if err := DeleteSession(si.projectPath, si.backend, si.sessionID); err != nil {
				slog.Error("failed to soft-delete session during task removal",
					slog.String("session_id", si.sessionID),
					slog.String("err", err.Error()),
				)
			}
		}
	}

	// Delete task_executions rows
	DB.Exec("DELETE FROM task_executions WHERE task_id = ?", id)

	// Hard-delete the task
	DB.Exec("DELETE FROM scheduled_tasks WHERE id = ?", id)
}

// PauseTask removes a task from cron but keeps it in the database as paused.
func (s *Scheduler) PauseTask(id int64) {
	s.mu.Lock()
	if entryID, ok := s.entries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, id)
	}
	s.mu.Unlock()

	DB.Exec("UPDATE scheduled_tasks SET status = 'paused', updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
}

// ResumeTask re-registers a paused task with cron.
func (s *Scheduler) ResumeTask(id int64) error {
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

// TriggerTask runs a task immediately in a background goroutine, regardless of its status.
func (s *Scheduler) TriggerTask(id int64) error {
	task, err := GetTaskByID(id)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	go s.executeTask(task, task.ProjectPath, "manual")
	return nil
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

	// If task is active (or reactivated from completed), remove old cron entry and re-register atomically
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
	} else if task.Status != "active" {
		// Remove from cron if task is not active (completed/paused)
		s.mu.Lock()
		if entryID, ok := s.entries[task.ID]; ok {
			s.cron.Remove(entryID)
			delete(s.entries, task.ID)
		}
		s.mu.Unlock()
	}

	// Persist to database
	if err := updateTask(task); err != nil {
		return err
	}

	slog.Info("updated task",
		slog.Int64("task_id", task.ID),
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
		s.executeTask(current, projectPath, "auto")
	}))

	// Lock is already held by caller
	s.entries[taskID] = entryID

	slog.Info("registered cron task",
		slog.Int64("task_id", taskID),
		slog.String("cron", task.CronExpr),
	)
	return nil
}

// UpdateTaskStats increments run_count and updates last_run_at for a task.
func UpdateTaskStats(task *model.ScheduledTask, newStatus string) {
	now := time.Now()
	DB.Exec("UPDATE scheduled_tasks SET last_run_at = ?, run_count = run_count + 1, status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		now, newStatus, task.ID)
}

// emitTaskEvent broadcasts a task_update event to connected clients.
func emitTaskEvent(taskID, status, executionID string) {
	mgr := ws.GetManager()
	if mgr == nil {
		return
	}
	mgr.BroadcastEvent(ws.ServerMessage{
		Type:  "event",
		ID:    ws.GenerateEventID(),
		Event: "task_update",
		Data: &ws.TaskUpdateData{
			TaskID:      taskID,
			Status:      status,
			ExecutionID: executionID,
		},
	})
}

// executeTask runs a scheduled task by invoking the AI backend and inserting
// the result as an assistant message in the original session.
func (s *Scheduler) executeTask(task *model.ScheduledTask, projectPath string, triggerType string) {
	agent, ok := model.Agents[task.AgentID]
	if !ok {
		slog.Error("agent not found for task, pausing",
			slog.String("agent_id", task.AgentID),
			slog.Int64("task_id", task.ID),
			slog.String("name", task.Name),
		)
		s.PauseTask(task.ID)
		return
	}

	backendName := agent.Backend
	if backendName == "" {
		backendName = "codebuddy"
	}

	// Create a chat session for this execution
	sessionID, err := CreateSession(projectPath, backendName, task.Name, task.AgentID, "", "default", "scheduled")
	if err != nil {
		slog.Error("failed to create session for task",
			slog.Int64("task_id", task.ID),
			slog.String("err", err.Error()),
		)
		return
	}

	slog.Info("executing scheduled task",
		slog.Int64("task_id", task.ID),
		slog.String("session_id", sessionID),
		slog.String("name", task.Name),
	)

	// Record execution linked to the session
	executionID, err := AddTaskExecution(task.ID, sessionID, triggerType)
	if err != nil {
		slog.Error("failed to record task execution", slog.String("err", err.Error()))
	}

	emitTaskEvent(fmt.Sprintf("%d", task.ID), "running", fmt.Sprintf("%d", executionID))

	// Write user message (the prompt)
	if _, err := AddChatMessage(projectPath, backendName, sessionID, "user", task.Prompt, nil, false, task.Name); err != nil {
		slog.Error("failed to write user message for task", slog.String("err", err.Error()))
	}

	// Build chat request — no session resume, standalone execution
	// ScheduledExecution flag prevents recursive task creation at the
	// handler level: even if the AI outputs a <schedule-proposal> tag,
	// the handler will not create a task from it.
	//
	// Rebuild system prompt without task-scheduler skill to prevent
	// the AI from discovering scheduled task capability (anti-recursion).
	systemPrompt := agent.SystemPrompt
	// Replace {{PROJECT_PATH}} per-request with the actual project path for this task
	if projectPath != "" {
		systemPrompt = strings.ReplaceAll(systemPrompt, "{{PROJECT_PATH}}", projectPath)
	}
	scheduledCommon := model.BuildCommonPrompt(true)
	normalCommon := model.BuildCommonPrompt(false)
	if normalCommon != "" && strings.HasPrefix(systemPrompt, normalCommon) {
		// Replace the common prompt prefix with the scheduled version
		remaining := systemPrompt[len(normalCommon):]
		if scheduledCommon != "" {
			systemPrompt = scheduledCommon + remaining
		} else {
			// No skills at all in scheduled mode — strip the common prefix
			systemPrompt = strings.TrimPrefix(remaining, "\n\n")
		}
	}

	chatReq := ai.ChatRequest{
		Prompt:             task.Prompt,
		SessionID:          sessionID,
		WorkDir:            projectPath,
		SystemPrompt:       systemPrompt,
		Model:              agent.DefaultModelID(),
		Command:            agent.Command,
		AgentID:            task.AgentID,
		ThinkingEffort:     agent.ThinkingEffort,
		Resume:             false,
		ScheduledExecution: true,
	}

	// Execute AI backend (no timeout - let AI run indefinitely)
	ctx, cancel := context.WithCancel(context.Background())

	// Register running execution
	running := &RunningExecution{
		ID:          sessionID,
		TaskID:      task.ID,
		CancelFunc:  cancel,
		StartedAt:   time.Now(),
		TriggerType: triggerType,
	}
	s.runningExecutions.Store(sessionID, running)

	// Notify system event bus: task execution started
	GlobalEventBus.Publish(SystemEvent{
		Type:    "task_exec_update",
		Payload: map[string]any{"taskId": task.ID, "execId": sessionID, "status": "running", "triggerType": triggerType},
	})

	defer func() {
		s.runningExecutions.Delete(sessionID)
		cancel()
	}()

	backend, err := ai.NewBackend(backendName)
	if err != nil {
		slog.Error("failed to create backend for task", slog.String("err", err.Error()))
		UpdateExecutionStatus(sessionID, "failed")
		GlobalEventBus.Publish(SystemEvent{
			Type:    "task_exec_update",
			Payload: map[string]any{"taskId": task.ID, "execId": sessionID, "status": "failed"},
		})
		emitTaskEvent(fmt.Sprintf("%d", task.ID), "failed", fmt.Sprintf("%d", executionID))
		return
	}

	eventCh, err := backend.ExecuteStream(ctx, chatReq)
	if err != nil {
		slog.Error("failed to execute stream for task", slog.String("err", err.Error()))
		UpdateExecutionStatus(sessionID, "failed")
		GlobalEventBus.Publish(SystemEvent{
			Type:    "task_exec_update",
			Payload: map[string]any{"taskId": task.ID, "execId": sessionID, "status": "failed"},
		})
		emitTaskEvent(fmt.Sprintf("%d", task.ID), "failed", fmt.Sprintf("%d", executionID))
		return
	}

	// Consume streaming events and build content blocks
	var blocks []model.ContentBlock
	var responseMetadata *ai.Metadata
	var receivedTerminal bool // tracks whether "done" or "error" was received
	wallStart := time.Now()

	for event := range eventCh {
		switch event.Type {
		case "metadata":
			if event.Meta != nil {
				responseMetadata = event.Meta
			}
		case "done", "error":
			receivedTerminal = true
		default:
			ai.AccumulateBlock(&blocks, event)
		}
	}

	// If context was cancelled, mark execution as cancelled and update stats
	if ctx.Err() == context.Canceled {
		slog.Info("task execution cancelled",
			slog.Int64("task_id", task.ID),
			slog.String("session_id", sessionID),
		)
		UpdateExecutionStatus(sessionID, "cancelled")
		GlobalEventBus.Publish(SystemEvent{
			Type:    "task_exec_update",
			Payload: map[string]any{"taskId": task.ID, "execId": sessionID, "status": "cancelled"},
		})
		emitTaskEvent(fmt.Sprintf("%d", task.ID), "cancelled", fmt.Sprintf("%d", executionID))
		newStatus := task.Status
		UpdateTaskStats(task, newStatus)
		return
	}

	// If the event channel closed without a terminal event (done/error),
	// the CLI process likely crashed or was killed (e.g. SIGKILL, OOM).
	// Mark as failed to prevent zombie "running" state in DB.
	if !receivedTerminal {
		slog.Warn("task execution ended without terminal event (CLI process crashed?)",
			slog.Int64("task_id", task.ID),
			slog.String("session_id", sessionID),
		)
		UpdateExecutionStatus(sessionID, "failed")
		GlobalEventBus.Publish(SystemEvent{
			Type:    "task_exec_update",
			Payload: map[string]any{"taskId": task.ID, "execId": sessionID, "status": "failed"},
		})
		emitTaskEvent(fmt.Sprintf("%d", task.ID), "failed", fmt.Sprintf("%d", executionID))
		newStatus := task.Status
		UpdateTaskStats(task, newStatus)
		return
	}

	// Compute wall-clock duration and inject into metadata
	wallMs := int(time.Since(wallStart).Milliseconds())
	if responseMetadata == nil {
		responseMetadata = &ai.Metadata{}
	}
	responseMetadata.WallMs = wallMs

	// Build content JSON for the assistant message
	contentMap := map[string]any{"blocks": blocks}
	if responseMetadata != nil {
		contentMap["metadata"] = responseMetadata
	}
	contentJSON, _ := json.Marshal(contentMap)

	// Write assistant message to chat_history
	if _, err := AddChatMessage(projectPath, backendName, sessionID, "assistant", string(contentJSON), nil, false, task.Name); err != nil {
		slog.Error("failed to write assistant message for task", slog.String("err", err.Error()))
	}

	// Mark execution as completed
	UpdateExecutionStatus(sessionID, "completed")
	emitTaskEvent(fmt.Sprintf("%d", task.ID), "completed", fmt.Sprintf("%d", executionID))

	// Notify system event bus: task execution completed
	GlobalEventBus.Publish(SystemEvent{
		Type:    "task_exec_update",
		Payload: map[string]any{"taskId": task.ID, "execId": sessionID, "status": "completed"},
	})

	// Update task execution stats
	newStatus := task.Status

	// Check repeat mode — for "limited", read current DB value to decide completion
	if task.RepeatMode == "limited" {
		var currentCount int
		if err := DB.QueryRow("SELECT run_count FROM scheduled_tasks WHERE id = ?", task.ID).Scan(&currentCount); err == nil {
			if currentCount+1 >= task.MaxRuns {
				newStatus = "completed"
			}
		}
	}
	if task.RepeatMode == "once" {
		newStatus = "completed"
	}

	schedule, _ := cron.ParseStandard(task.CronExpr)
	var nextRunAt *time.Time
	if newStatus == "active" {
		nr := schedule.Next(time.Now())
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
		DB.Exec("UPDATE scheduled_tasks SET last_run_at = ?, next_run_at = ?, run_count = run_count + 1, status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			time.Now(), nextRunAt, newStatus, task.ID)
	} else {
		DB.Exec("UPDATE scheduled_tasks SET last_run_at = ?, next_run_at = NULL, run_count = run_count + 1, status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			time.Now(), newStatus, task.ID)
	}

	slog.Info("task execution completed",
		slog.Int64("task_id", task.ID),
		slog.String("session_id", sessionID),
		slog.String("status", newStatus),
	)

	// Generate summary asynchronously if task summarizer is configured
	if s.taskSummarizer != nil {
		capturedExecID := executionID // capture for goroutine
		capturedBlocks := blocks      // capture for goroutine
		go func() {
			// Use independent context with timeout — do NOT inherit executeTask's
			// ctx which is cancelled when this function returns.
			sumCtx, sumCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer sumCancel()

			text := extractTextFromBlocks(capturedBlocks)
			if utf8.RuneCountInString(text) < summarize.ShortTextThreshold {
				// Text too short, mark as empty (frontend shows original)
				if err := UpdateExecutionSummary(capturedExecID, ""); err != nil {
					slog.Warn("failed to update execution summary (short text)",
						slog.Int64("exec_id", capturedExecID),
						slog.String("err", err.Error()),
					)
				}
				return
			}
			summary, err := s.taskSummarizer.Summarize(sumCtx, text, "")
			if err != nil {
				slog.Warn("task execution summary failed",
					slog.Int64("task_id", task.ID),
					slog.Int64("exec_id", capturedExecID),
					slog.String("err", err.Error()),
				)
				return // summary stays NULL, frontend shows original
			}
			if err := UpdateExecutionSummary(capturedExecID, summary); err != nil {
				slog.Warn("failed to update execution summary",
					slog.Int64("exec_id", capturedExecID),
					slog.String("err", err.Error()),
				)
			}
			slog.Info("task execution summary completed",
				slog.Int64("task_id", task.ID),
				slog.Int64("exec_id", capturedExecID),
				slog.Int("summary_len", utf8.RuneCountInString(summary)),
			)
		}()
	}
}

// GetTasks retrieves all tasks for a project path. If projectPath is empty, retrieves all tasks.
func GetTasks(projectPath string) ([]model.ScheduledTask, error) {
	var tasks []model.ScheduledTask
	var query string
	var args []interface{}

	if projectPath == "" {
		query = `SELECT s.id, s.project_path, s.name, s.cron_expr, s.agent_id, s.prompt, s.session_id,
			s.status, s.repeat_mode, s.max_runs, s.last_run_at, s.next_run_at, s.run_count,
			s.last_read_at, s.created_at, s.updated_at,
			(SELECT COUNT(*) FROM task_executions e
			 WHERE e.task_id = s.id AND e.read_at IS NULL AND e.status != 'running'
			 AND (s.last_read_at IS NULL OR e.created_at > s.last_read_at)) AS unread_count
			FROM scheduled_tasks s ORDER BY s.created_at DESC`
	} else {
		query = `SELECT s.id, s.project_path, s.name, s.cron_expr, s.agent_id, s.prompt, s.session_id,
			s.status, s.repeat_mode, s.max_runs, s.last_run_at, s.next_run_at, s.run_count,
			s.last_read_at, s.created_at, s.updated_at,
			(SELECT COUNT(*) FROM task_executions e
			 WHERE e.task_id = s.id AND e.read_at IS NULL AND e.status != 'running'
			 AND (s.last_read_at IS NULL OR e.created_at > s.last_read_at)) AS unread_count
			FROM scheduled_tasks s WHERE s.project_path = ? ORDER BY s.created_at DESC`
		args = []interface{}{projectPath}
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t model.ScheduledTask
		var lastRun, nextRun, lastRead sql.NullTime
		if err := rows.Scan(&t.ID, &t.ProjectPath, &t.Name, &t.CronExpr, &t.AgentID, &t.Prompt, &t.SessionID, &t.Status, &t.RepeatMode, &t.MaxRuns, &lastRun, &nextRun, &t.RunCount, &lastRead, &t.CreatedAt, &t.UpdatedAt, &t.UnreadCount); err != nil {
			return nil, err
		}
		if lastRun.Valid {
			t.LastRunAt = &lastRun.Time
		}
		if nextRun.Valid {
			t.NextRunAt = &nextRun.Time
		}
		if lastRead.Valid {
			t.LastReadAt = &lastRead.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetTaskByID retrieves a single task by its ID.
func GetTaskByID(id int64) (*model.ScheduledTask, error) {
	var t model.ScheduledTask
	var lastRun, nextRun, lastRead sql.NullTime
	err := DB.QueryRow(
		`SELECT s.id, s.project_path, s.name, s.cron_expr, s.agent_id, s.prompt, s.session_id,
		s.status, s.repeat_mode, s.max_runs, s.last_run_at, s.next_run_at, s.run_count,
		s.last_read_at, s.created_at, s.updated_at,
		(SELECT COUNT(*) FROM task_executions e
		 WHERE e.task_id = s.id AND e.read_at IS NULL AND e.status != 'running'
		 AND (s.last_read_at IS NULL OR e.created_at > s.last_read_at)) AS unread_count
		FROM scheduled_tasks s WHERE s.id = ?`,
		id,
	).Scan(&t.ID, &t.ProjectPath, &t.Name, &t.CronExpr, &t.AgentID, &t.Prompt, &t.SessionID, &t.Status, &t.RepeatMode, &t.MaxRuns, &lastRun, &nextRun, &t.RunCount, &lastRead, &t.CreatedAt, &t.UpdatedAt, &t.UnreadCount)
	if err != nil {
		return nil, err
	}
	if lastRun.Valid {
		t.LastRunAt = &lastRun.Time
	}
	if nextRun.Valid {
		t.NextRunAt = &nextRun.Time
	}
	if lastRead.Valid {
		t.LastReadAt = &lastRead.Time
	}
	return &t, nil
}

// insertTask inserts a new task into the database and sets the auto-generated ID.
func insertTask(task *model.ScheduledTask) error {
	result, err := DB.Exec(
		`INSERT INTO scheduled_tasks (project_path, name, cron_expr, agent_id, prompt, session_id, status, repeat_mode, max_runs, next_run_at, run_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ProjectPath, task.Name, task.CronExpr, task.AgentID, task.Prompt, task.SessionID, task.Status, task.RepeatMode, task.MaxRuns, task.NextRunAt, task.RunCount, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	task.ID = id
	return nil
}

// updateTask updates an existing task in the database.
func updateTask(task *model.ScheduledTask) error {
	_, err := DB.Exec(
		`UPDATE scheduled_tasks SET name=?, cron_expr=?, agent_id=?, prompt=?, session_id=?, status=?, repeat_mode=?, max_runs=?, next_run_at=?, run_count=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		task.Name, task.CronExpr, task.AgentID, task.Prompt, task.SessionID, task.Status, task.RepeatMode, task.MaxRuns, task.NextRunAt, task.RunCount, task.ID,
	)
	return err
}

// AddTaskExecution records a task execution linked to a chat session.
// Returns the auto-generated execution ID.
func AddTaskExecution(taskID int64, sessionID string, triggerType string) (int64, error) {
	result, err := DB.Exec(
		"INSERT INTO task_executions (task_id, session_id, trigger_type, status) VALUES (?, ?, ?, 'running')",
		taskID, sessionID, triggerType,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateExecutionStatus updates the status of a task execution by session_id.
func UpdateExecutionStatus(sessionID string, status string) error {
	_, err := DB.Exec(
		"UPDATE task_executions SET status = ? WHERE session_id = ?",
		status, sessionID,
	)
	return err
}

// UpdateTaskLastRead updates the last_read_at timestamp for a task, clearing unread status.
func UpdateTaskLastRead(taskID int64) error {
	_, err := DB.Exec(
		"UPDATE scheduled_tasks SET last_read_at = CURRENT_TIMESTAMP WHERE id = ?",
		taskID,
	)
	return err
}

// MarkExecutionRead marks a single execution as read by setting its read_at timestamp.
func MarkExecutionRead(executionID string) error {
	_, err := DB.Exec(
		"UPDATE task_executions SET read_at = CURRENT_TIMESTAMP WHERE id = ?",
		executionID,
	)
	return err
}

// UpdateExecutionSummary updates the summary column for a task execution.
// summary is NULL when not yet generated, "" when text was too short,
// and non-empty when summarization succeeded.
func UpdateExecutionSummary(executionID int64, summary string) error {
	_, err := DB.Exec(
		"UPDATE task_executions SET summary = ? WHERE id = ?",
		summary, executionID,
	)
	return err
}

// extractTextFromBlocks extracts plain text from ContentBlock array.
// Only text-type blocks are included; tool_use, thinking, etc. are skipped.
func extractTextFromBlocks(blocks []model.ContentBlock) string {
	var buf strings.Builder
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			if buf.Len() > 0 {
				buf.WriteString("\n\n")
			}
			buf.WriteString(b.Text)
		}
	}
	return buf.String()
}

// DeleteTaskExecution deletes a single task execution and soft-deletes the
// associated chat session. Running executions cannot be deleted.
func DeleteTaskExecution(executionID int64) error {
	// Fetch execution details
	var sessionID string
	var taskID int64
	var status string
	err := DB.QueryRow(
		"SELECT session_id, task_id, status FROM task_executions WHERE id = ?",
		executionID,
	).Scan(&sessionID, &taskID, &status)
	if err != nil {
		return fmt.Errorf("execution not found: %w", err)
	}

	if status == "running" {
		return fmt.Errorf("cannot delete a running execution")
	}

	// Soft-delete the associated chat session
	var projectPath, backend string
	err = DB.QueryRow(
		"SELECT project_path, backend FROM chat_sessions WHERE id = ?",
		sessionID,
	).Scan(&projectPath, &backend)
	if err == nil {
		if err := DeleteSession(projectPath, backend, sessionID); err != nil {
			slog.Error("failed to soft-delete session during execution deletion",
				slog.String("session_id", sessionID),
				slog.String("err", err.Error()),
			)
		}
	}

	// Hard-delete the execution row
	if _, err := DB.Exec("DELETE FROM task_executions WHERE id = ?", executionID); err != nil {
		return fmt.Errorf("failed to delete execution: %w", err)
	}

	// Decrement run_count on the parent task (clamp to 0)
	DB.Exec("UPDATE scheduled_tasks SET run_count = MAX(run_count - 1, 0), updated_at = CURRENT_TIMESTAMP WHERE id = ?", taskID)

	return nil
}

// DeleteAllTaskExecutions deletes all non-running executions for a task
// and soft-deletes the associated chat sessions.
func DeleteAllTaskExecutions(taskID int64) error {
	// Collect all non-running executions with their session info
	rows, err := DB.Query(`
		SELECT te.id, te.session_id, cs.project_path, cs.backend
		FROM task_executions te
		JOIN chat_sessions cs ON cs.id = te.session_id
		WHERE te.task_id = ? AND te.status != 'running'`, taskID)
	if err != nil {
		return fmt.Errorf("failed to query executions: %w", err)
	}

	type execInfo struct {
		id          int64
		sessionID   string
		projectPath string
		backend     string
	}
	var execs []execInfo
	for rows.Next() {
		var ei execInfo
		if rows.Scan(&ei.id, &ei.sessionID, &ei.projectPath, &ei.backend) == nil {
			execs = append(execs, ei)
		}
	}
	rows.Close()

	// Soft-delete chat sessions
	for _, ei := range execs {
		if err := DeleteSession(ei.projectPath, ei.backend, ei.sessionID); err != nil {
			slog.Error("failed to soft-delete session during bulk execution deletion",
				slog.String("session_id", ei.sessionID),
				slog.String("err", err.Error()),
			)
		}
	}

	// Hard-delete all non-running execution rows
	DB.Exec("DELETE FROM task_executions WHERE task_id = ? AND status != 'running'", taskID)

	// Reset run_count to match remaining (running) executions
	var runningCount int
	DB.QueryRow("SELECT COUNT(*) FROM task_executions WHERE task_id = ?", taskID).Scan(&runningCount)
	DB.Exec("UPDATE scheduled_tasks SET run_count = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", runningCount, taskID)

	return nil
}

// HasUnreadTasks checks if any task for the given project has unread executions.
func HasUnreadTasks(projectPath string) (bool, error) {
	var count int
	var err error
	if projectPath == "" {
		err = DB.QueryRow(
			`SELECT COUNT(*) FROM scheduled_tasks s
			 WHERE (SELECT COUNT(*) FROM task_executions e
			      WHERE e.task_id = s.id AND e.read_at IS NULL AND e.status != 'running'
			      AND (s.last_read_at IS NULL OR e.created_at > s.last_read_at)) > 0`,
		).Scan(&count)
	} else {
		err = DB.QueryRow(
			`SELECT COUNT(*) FROM scheduled_tasks s
			 WHERE s.project_path = ?
			 AND (SELECT COUNT(*) FROM task_executions e
			      WHERE e.task_id = s.id AND e.read_at IS NULL AND e.status != 'running'
			      AND (s.last_read_at IS NULL OR e.created_at > s.last_read_at)) > 0`,
			projectPath,
		).Scan(&count)
	}
	return count > 0, err
}

package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/service"
)

// ServeTasks handles GET (list) and POST (create) for scheduled tasks.
func ServeTasks(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		tasks, err := service.GetTasks(projectPath)
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to load tasks")))
			return
		}
		if tasks == nil {
			tasks = []model.ScheduledTask{}
		}
		// Enrich tasks with running counts from in-memory map
		runningCounts := service.GlobalScheduler.GetRunningCounts()
		for i := range tasks {
			tasks[i].RunningCount = runningCounts[tasks[i].ID]
		}
		// Check if any task has unread executions
		hasUnread, _ := service.HasUnreadTasks(projectPath)
		writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks, "hasUnread": hasUnread})

	case http.MethodPost:
	var req struct {
		Name       string `json:"name"`
		CronExpr   string `json:"cron_expr"`
		AgentID    string `json:"agent_id"`
		Prompt     string `json:"prompt"`
		RepeatMode string `json:"repeat_mode"`
		MaxRuns    int    `json:"max_runs"`
		SessionID  string `json:"session_id"`
	}
	if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" || req.CronExpr == "" || req.AgentID == "" || req.Prompt == "" {
			writeLocalizedErrorf(w, r, http.StatusBadRequest, "TaskFieldsRequired")
			return
		}
	if req.RepeatMode == "" {
			req.RepeatMode = "unlimited"
		}

		task := &model.ScheduledTask{
			ProjectPath: projectPath,
			Name:        req.Name,
			CronExpr:    req.CronExpr,
			AgentID:     req.AgentID,
			Prompt:      req.Prompt,
			RepeatMode:  req.RepeatMode,
			MaxRuns:     req.MaxRuns,
			SessionID:   req.SessionID,
		}

		if err := service.GlobalScheduler.AddTask(task); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to create task: %v", err)))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "task": task})

	default:
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

// ServeTaskByID handles operations on a single task by ID.
// GET /api/tasks/{id} - get task details
// PUT /api/tasks/{id} - update task (pause/resume)
// DELETE /api/tasks/{id} - delete task
// GET /api/tasks/{id}/executions - get execution history
func ServeTaskByID(w http.ResponseWriter, r *http.Request) {
	// Require project ownership for all task operations
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	// Extract task ID from path: /api/tasks/{id} or /api/tasks/{id}/executions
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.SplitN(path, "/", 2)
	taskIDStr := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	if taskIDStr == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "TaskIdRequired")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "TaskIdInvalid")
		return
	}

	// Handle sub-paths
	if subPath == "executions" && r.Method == http.MethodGet {
		serveTaskExecutions(w, r, taskID, projectPath)
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, err := service.GetTaskByID(taskID)
		if err != nil {
			writeLocalizedError(w, r, model.NotFound(nil, "TaskNotFound"))
			return
		}
		if task.ProjectPath != projectPath {
			writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
			return
		}
		// Enrich with running executions from in-memory map
		task.RunningExecutions = service.GlobalScheduler.GetRunningExecutions(taskID)
		task.RunningCount = len(task.RunningExecutions)
		writeJSON(w, http.StatusOK, task)

	case http.MethodPut:
		var req struct {
			Action      string `json:"action"`       // "pause", "resume", "read", "trigger", "cancel", or "update"
			ExecutionID string `json:"executionId"`  // required for "cancel"
			Name        string `json:"name"`
			CronExpr    string `json:"cron_expr"`
			AgentID     string `json:"agent_id"`
			Prompt      string `json:"prompt"`
			RepeatMode  string `json:"repeat_mode"`
			MaxRuns     int    `json:"max_runs"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}

		// For actions that need ownership verification, fetch task first
		// Actions that only need taskID (pause/resume/trigger/cancel/read) also need ownership check
		task, err := service.GetTaskByID(taskID)
		if err != nil {
			writeLocalizedError(w, r, model.NotFound(nil, "TaskNotFound"))
			return
		}
		if task.ProjectPath != projectPath {
			writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
			return
		}

		// Handle actions
		if req.Action == "pause" {
			service.GlobalScheduler.PauseTask(taskID)
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		if req.Action == "resume" {
			if err := service.GlobalScheduler.ResumeTask(taskID); err != nil {
				model.WriteError(w, model.Internal(err))
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		if req.Action == "read" {
			if req.ExecutionID != "" {
				// Per-execution read: only mark this single execution
				if err := service.MarkExecutionRead(req.ExecutionID); err != nil {
					model.WriteError(w, model.Internal(err))
					return
				}
			} else {
				// Task-level read: mark all executions as read
				if err := service.UpdateTaskLastRead(taskID); err != nil {
					model.WriteError(w, model.Internal(err))
					return
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		if req.Action == "trigger" {
			// Reject if task already has a running execution
			if service.GlobalScheduler.HasRunningExecutions(taskID) {
				writeLocalizedErrorf(w, r, http.StatusConflict, "TaskAlreadyRunning")
				return
			}
			if err := service.GlobalScheduler.TriggerTask(taskID); err != nil {
				writeLocalizedError(w, r, model.NotFound(err, "TaskNotFound"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		if req.Action == "cancel" {
			if req.ExecutionID == "" {
				writeLocalizedErrorf(w, r, http.StatusBadRequest, "TaskExecutionIdRequired")
				return
			}
			if err := service.GlobalScheduler.CancelExecution(req.ExecutionID); err != nil {
				writeLocalizedError(w, r, model.NotFound(err, "TaskExecutionNotFound"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}

		if req.Action == "deleteExecution" {
			if req.ExecutionID == "" {
				writeLocalizedErrorf(w, r, http.StatusBadRequest, "TaskExecutionIdRequired")
				return
			}
			executionID, err := strconv.ParseInt(req.ExecutionID, 10, 64)
			if err != nil {
				writeLocalizedErrorf(w, r, http.StatusBadRequest, "TaskExecutionIdInvalid")
				return
			}
			if err := service.DeleteTaskExecution(executionID); err != nil {
				if strings.Contains(err.Error(), "not found") {
					writeLocalizedError(w, r, model.NotFound(err, "TaskExecutionNotFound"))
				} else if strings.Contains(err.Error(), "cannot delete a running") {
					writeLocalizedErrorf(w, r, http.StatusConflict, "TaskExecutionRunning")
				} else {
					model.WriteError(w, model.Internal(err))
				}
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}

		if req.Action == "deleteAllExecutions" {
			if err := service.DeleteAllTaskExecutions(taskID); err != nil {
				model.WriteError(w, model.Internal(err))
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}

		// Full task update — task already fetched and verified above

		// Update fields if provided
		if req.Name != "" {
			task.Name = req.Name
		}
		if req.CronExpr != "" {
			task.CronExpr = req.CronExpr
		}
		if req.AgentID != "" {
			task.AgentID = req.AgentID
		}
		if req.Prompt != "" {
			task.Prompt = req.Prompt
		}
		if req.RepeatMode != "" {
			task.RepeatMode = req.RepeatMode
		}
		task.MaxRuns = req.MaxRuns

		// Editing a completed task implies reactivation — the user wants it to run again.
		// Reset status to active and clear runCount so it starts fresh.
		if task.Status == "completed" {
			task.Status = "active"
			task.RunCount = 0
		}

		// Update task
		if err := service.GlobalScheduler.UpdateTask(task); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to update task: %v", err)))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "task": task})

	case http.MethodDelete:
		// Verify ownership before deletion
		task, err := service.GetTaskByID(taskID)
		if err != nil {
			writeLocalizedError(w, r, model.NotFound(nil, "TaskNotFound"))
			return
		}
		if task.ProjectPath != projectPath {
			writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
			return
		}
		// Cancel any running executions before removing the task
		service.GlobalScheduler.CancelAllExecutions(taskID)
		service.GlobalScheduler.RemoveTask(taskID)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

// serveTaskExecutions returns the execution history for a task.
// It joins task_executions with chat_history to fetch the assistant content.
func serveTaskExecutions(w http.ResponseWriter, r *http.Request, taskID int64, projectPath string) {
	task, err := service.GetTaskByID(taskID)
	if err != nil {
		writeLocalizedError(w, r, model.NotFound(nil, "TaskNotFound"))
		return
	}
	if task.ProjectPath != projectPath {
		writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
		return
	}

	type Execution struct {
		ID          int64   `json:"id"`
		SessionID   string  `json:"sessionId"`
		TriggerType string  `json:"triggerType"`
		Status      string  `json:"status"`
		Content     *string `json:"content"`
		Summary     *string `json:"summary"`
		CreatedAt   string  `json:"createdAt"`
		IsUnread    bool    `json:"isUnread"`
	}

	rows, err := service.DB.Query(`
		SELECT te.id, te.session_id, te.trigger_type, te.status, te.created_at,
		       te.read_at, te.summary,
		       ch.content AS assistant_content
		FROM task_executions te
		LEFT JOIN chat_history ch ON ch.session_id = te.session_id
		    AND ch.role = 'assistant'
		    AND ch.deleted = 0
		    AND ch.streaming = 0
		WHERE te.task_id = ?
		ORDER BY te.created_at DESC
	`, taskID)
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to load execution history")))
		return
	}
	defer rows.Close()

	var executions []Execution
	for rows.Next() {
		var exec Execution
		var content sql.NullString
		var summary sql.NullString
		var readAt sql.NullTime
		if err := rows.Scan(&exec.ID, &exec.SessionID, &exec.TriggerType, &exec.Status, &exec.CreatedAt, &readAt, &summary, &content); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to scan execution record")))
			return
		}
		if content.Valid {
			exec.Content = &content.String
		}
		if summary.Valid {
			exec.Summary = &summary.String
		}
		// An execution is unread if it has no read_at AND is not running AND
		// (task has never been read OR execution is newer than last_read_at)
		if readAt.Valid || exec.Status == "running" {
			exec.IsUnread = false
		} else if task.LastReadAt == nil {
			exec.IsUnread = true
		} else {
			createdAt, parseErr := time.Parse(time.RFC3339, exec.CreatedAt)
			if parseErr == nil {
				exec.IsUnread = createdAt.After(*task.LastReadAt)
			}
		}
		executions = append(executions, exec)
	}

	if executions == nil {
		executions = []Execution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": executions})
}

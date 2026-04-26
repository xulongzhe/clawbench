package handler

import (
	"fmt"
	"net/http"
	"strings"

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
		writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})

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
			model.WriteErrorf(w, http.StatusBadRequest, "name, cronExpr, agentId, and prompt are required")
			return
		}
		if req.AgentID == "assistant" {
			model.WriteErrorf(w, http.StatusBadRequest, "assistant agent cannot execute scheduled tasks, please choose a specialized agent")
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
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ServeTaskByID handles operations on a single task by ID.
// GET /api/tasks/{id} - get task details
// PUT /api/tasks/{id} - update task (pause/resume)
// DELETE /api/tasks/{id} - delete task
// GET /api/tasks/{id}/executions - get execution history
func ServeTaskByID(w http.ResponseWriter, r *http.Request) {
	// Extract task ID from path: /api/tasks/{id} or /api/tasks/{id}/executions
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.SplitN(path, "/", 2)
	taskID := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	if taskID == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "Task ID required")
		return
	}

	// Handle sub-paths
	if subPath == "executions" && r.Method == http.MethodGet {
		serveTaskExecutions(w, r, taskID)
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, err := service.GetTaskByID(taskID)
		if err != nil {
			model.WriteError(w, model.NotFound(nil, "Task not found"))
			return
		}
		writeJSON(w, http.StatusOK, task)

	case http.MethodPut:
		var req struct {
			Action      string `json:"action"`       // "pause", "resume", or "update"
			Name        string `json:"name"`
			CronExpr    string `json:"cron_expr"`
			AgentID     string `json:"agent_id"`
			Prompt      string `json:"prompt"`
			Description string `json:"description"`
			RepeatMode  string `json:"repeat_mode"`
			MaxRuns     int    `json:"max_runs"`
		}
		if !decodeJSON(w, r, &req) {
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

		// Full task update
		task, err := service.GetTaskByID(taskID)
		if err != nil {
			model.WriteError(w, model.NotFound(nil, "Task not found"))
			return
		}

		// Update fields if provided
		if req.Name != "" {
			task.Name = req.Name
		}
		if req.CronExpr != "" {
			task.CronExpr = req.CronExpr
		}
		if req.AgentID != "" {
			if req.AgentID == "assistant" {
				model.WriteErrorf(w, http.StatusBadRequest, "assistant agent cannot execute scheduled tasks")
				return
			}
			task.AgentID = req.AgentID
		}
		if req.Prompt != "" {
			task.Prompt = req.Prompt
		}
		if req.Description != "" {
			task.Description = req.Description
		}
		if req.RepeatMode != "" {
			task.RepeatMode = req.RepeatMode
		}
		if req.MaxRuns > 0 {
			task.MaxRuns = req.MaxRuns
		}

		// Update task
		if err := service.GlobalScheduler.UpdateTask(task); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to update task: %v", err)))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "task": task})

	case http.MethodDelete:
		service.GlobalScheduler.RemoveTask(taskID)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// serveTaskExecutions returns the execution history for a task.
// It queries the task_executions association table to find only AI-triggered messages,
// excluding any user messages from the same session.
func serveTaskExecutions(w http.ResponseWriter, r *http.Request, taskID string) {
	_, err := service.GetTaskByID(taskID)
	if err != nil {
		model.WriteError(w, model.NotFound(nil, "Task not found"))
		return
	}

	// Query task_executions JOIN chat_history to get only scheduled-triggered messages
	type Execution struct {
		Content   string `json:"content"`
		CreatedAt string `json:"createdAt"`
	}

	rows, err := service.DB.Query(`
		SELECT ch.content, ch.created_at
		FROM task_executions te
		JOIN chat_history ch ON te.message_id = ch.id
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
		if err := rows.Scan(&exec.Content, &exec.CreatedAt); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to scan execution record")))
			return
		}
		executions = append(executions, exec)
	}

	if executions == nil {
		executions = []Execution{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": executions})
}

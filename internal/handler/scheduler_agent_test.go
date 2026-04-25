package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"clawbench/internal/model"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
)

// ---------- ServeAgents ----------

func TestServeAgents_Get(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Set up agent list
	model.AgentList = []*model.Agent{
		{ID: "agent-1", Name: "Agent 1", Backend: "claude"},
		{ID: "agent-2", Name: "Agent 2", Backend: "codebuddy"},
	}
	defer func() { model.AgentList = nil }()

	req := newRequest(t, http.MethodGet, "/api/agents", nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(ServeAgents, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	agents := result["agents"].([]interface{})
	assert.Len(t, agents, 2)
}

func TestServeAgents_PostNotAllowed(t *testing.T) {
	req := newRequest(t, http.MethodPost, "/api/agents", nil)
	w := callHandler(ServeAgents, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// ---------- ServeChatCount ----------

func TestServeChatCount(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sid := createTestSession(t, env.ProjectDir)

	// Add messages
	service.AddChatMessage(env.ProjectDir, "claude", sid, "user", "Hello", "", nil, false)
	service.AddChatMessage(env.ProjectDir, "claude", sid, "assistant", "Hi", "", nil, false)

	req := newRequest(t, http.MethodGet, "/api/ai/chat/count?session_id="+sid, nil)
	w := callHandler(ServeChatCount, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, float64(2), result["count"])
}

func TestServeChatCount_NoSessionID(t *testing.T) {
	req := newRequest(t, http.MethodGet, "/api/ai/chat/count", nil)
	w := callHandler(ServeChatCount, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatCount_PostNotAllowed(t *testing.T) {
	req := newRequest(t, http.MethodPost, "/api/ai/chat/count", nil)
	w := callHandler(ServeChatCount, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// ---------- ServeChatMessageUpdate ----------

func TestServeChatMessageUpdate(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sid := createTestSession(t, env.ProjectDir)
	msgID, err := service.AddChatMessage(env.ProjectDir, "claude", sid, "user", "original", "", nil, false)
	assert.NoError(t, err)

	req := newRequest(t, http.MethodPut, "/api/ai/chat/message", map[string]any{
		"messageId": msgID,
		"content":   "updated content",
	})
	w := callHandler(ServeChatMessageUpdate, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, true, result["ok"])
}

func TestServeChatMessageUpdate_NoMessageID(t *testing.T) {
	req := newRequest(t, http.MethodPut, "/api/ai/chat/message", map[string]any{
		"content": "no id",
	})
	w := callHandler(ServeChatMessageUpdate, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatMessageUpdate_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/ai/chat/message", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ServeChatMessageUpdate(w, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatMessageUpdate_GetNotAllowed(t *testing.T) {
	req := newRequest(t, http.MethodGet, "/api/ai/chat/message", nil)
	w := callHandler(ServeChatMessageUpdate, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// ---------- ServeTasks ----------

func TestServeTasks_GetEmpty(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	req := newRequest(t, http.MethodGet, "/api/tasks", nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(ServeTasks, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	tasks := result["tasks"].([]interface{})
	assert.Empty(t, tasks)
}

func TestServeTasks_Post(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Set up agents so the scheduler can resolve them
	model.Agents = map[string]*model.Agent{
		"coder": {ID: "coder", Name: "Coder", Backend: "claude"},
	}
	defer func() { model.Agents = nil }()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	req := newRequest(t, http.MethodPost, "/api/tasks", map[string]any{
		"name":      "Test Task",
		"cron_expr": "0 * * * *",
		"agent_id":  "coder",
		"prompt":    "Do something",
	})
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(ServeTasks, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, true, result["ok"])
}

func TestServeTasks_PostMissingFields(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	req := newRequest(t, http.MethodPost, "/api/tasks", map[string]any{
		"name": "Test Task",
		// Missing cron_expr, agent_id, prompt
	})
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(ServeTasks, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeTasks_PostAssistantAgent(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	req := newRequest(t, http.MethodPost, "/api/tasks", map[string]any{
		"name":      "Test Task",
		"cron_expr": "0 * * * *",
		"agent_id":  "assistant",
		"prompt":    "Do something",
	})
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(ServeTasks, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeTasks_NoProject(t *testing.T) {
	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	req := newRequest(t, http.MethodGet, "/api/tasks", nil)
	w := callHandler(ServeTasks, req)
	assertStatus(t, w, http.StatusForbidden)
}

func TestServeTasks_MethodNotAllowed(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	req := newRequest(t, http.MethodDelete, "/api/tasks", nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(ServeTasks, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// ---------- ServeTaskByID ----------

func TestServeTaskByID_Get(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	model.Agents = map[string]*model.Agent{
		"coder": {ID: "coder", Name: "Coder", Backend: "claude"},
	}
	defer func() { model.Agents = nil }()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	// Create a task first
	task := &model.ScheduledTask{
		ProjectPath: env.ProjectDir,
		Name:        "Test Task",
		CronExpr:    "0 * * * *",
		AgentID:     "coder",
		Prompt:      "Test prompt",
		RepeatMode:  "unlimited",
	}
	err := s.AddTask(task)
	assert.NoError(t, err)

	req := newRequest(t, http.MethodGet, "/api/tasks/"+task.ID, nil)
	w := callHandler(ServeTaskByID, req)

	assertOK(t, w)
}

func TestServeTaskByID_Delete(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	model.Agents = map[string]*model.Agent{
		"coder": {ID: "coder", Name: "Coder", Backend: "claude"},
	}
	defer func() { model.Agents = nil }()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	task := &model.ScheduledTask{
		ProjectPath: env.ProjectDir,
		Name:        "Delete Me",
		CronExpr:    "0 * * * *",
		AgentID:     "coder",
		Prompt:      "Test",
		RepeatMode:  "unlimited",
	}
	s.AddTask(task)

	req := newRequest(t, http.MethodDelete, "/api/tasks/"+task.ID, nil)
	w := callHandler(ServeTaskByID, req)
	assertOK(t, w)
}

func TestServeTaskByID_Pause(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	model.Agents = map[string]*model.Agent{
		"coder": {ID: "coder", Name: "Coder", Backend: "claude"},
	}
	defer func() { model.Agents = nil }()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	task := &model.ScheduledTask{
		ProjectPath: env.ProjectDir,
		Name:        "Pause Me",
		CronExpr:    "0 * * * *",
		AgentID:     "coder",
		Prompt:      "Test",
		RepeatMode:  "unlimited",
	}
	s.AddTask(task)

	req := newRequest(t, http.MethodPut, "/api/tasks/"+task.ID, map[string]any{
		"action": "pause",
	})
	w := callHandler(ServeTaskByID, req)
	assertOK(t, w)
}

func TestServeTaskByID_Resume(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	model.Agents = map[string]*model.Agent{
		"coder": {ID: "coder", Name: "Coder", Backend: "claude"},
	}
	defer func() { model.Agents = nil }()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	task := &model.ScheduledTask{
		ProjectPath: env.ProjectDir,
		Name:        "Resume Me",
		CronExpr:    "0 * * * *",
		AgentID:     "coder",
		Prompt:      "Test",
		RepeatMode:  "unlimited",
	}
	s.AddTask(task)
	s.PauseTask(task.ID)

	req := newRequest(t, http.MethodPut, "/api/tasks/"+task.ID, map[string]any{
		"action": "resume",
	})
	w := callHandler(ServeTaskByID, req)
	assertOK(t, w)
}

func TestServeTaskByID_NotFound(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	req := newRequest(t, http.MethodGet, "/api/tasks/non-existent", nil)
	w := callHandler(ServeTaskByID, req)
	assertStatus(t, w, http.StatusNotFound)
}

func TestServeTaskByID_NoTaskID(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/tasks/", nil)
	w := callHandler(ServeTaskByID, req)
	assertStatus(t, w, http.StatusBadRequest)
}

// ---------- ServeProjectDialog ----------

func TestServeProjectDialog_PostNotAllowed(t *testing.T) {
	req := newRequest(t, http.MethodPost, "/dialog/project", nil)
	w := callHandler(ServeProjectDialog, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// ---------- ServeIndex ----------

func TestServeIndex_NotFound(t *testing.T) {
	// In a test environment, public/ and web/ don't exist, so we get 404
	req := newRequest(t, http.MethodGet, "/nonexistent-path.js", nil)
	w := callHandler(ServeIndex, req)
	assertStatus(t, w, http.StatusNotFound)
}

// ---------- serveTaskExecutions ----------

func TestServeTaskByID_Executions(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	model.Agents = map[string]*model.Agent{
		"coder": {ID: "coder", Name: "Coder", Backend: "claude"},
	}
	defer func() { model.Agents = nil }()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	// Create a task
	task := &model.ScheduledTask{
		ProjectPath: env.ProjectDir,
		Name:        "Exec Task",
		CronExpr:    "0 * * * *",
		AgentID:     "coder",
		Prompt:      "Test",
		RepeatMode:  "unlimited",
	}
	s.AddTask(task)

	// Create a session and message for execution tracking
	sid := createTestSession(t, env.ProjectDir)
	msgID, _ := service.AddChatMessage(env.ProjectDir, "claude", sid, "assistant", "result", "", nil, false)
	service.AddTaskExecution(task.ID, msgID)

	// Get executions
	req := newRequest(t, http.MethodGet, "/api/tasks/"+task.ID+"/executions", nil)
	w := callHandler(ServeTaskByID, req)
	assertOK(t, w)

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	executions := result["executions"].([]interface{})
	assert.Len(t, executions, 1)
}

func TestServeTaskByID_ExecutionsTaskNotFound(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	req := newRequest(t, http.MethodGet, "/api/tasks/non-existent/executions", nil)
	w := callHandler(ServeTaskByID, req)
	assertStatus(t, w, http.StatusNotFound)
}

// ---------- ServeTaskByID Update ----------

func TestServeTaskByID_Update(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	model.Agents = map[string]*model.Agent{
		"coder": {ID: "coder", Name: "Coder", Backend: "claude"},
	}
	defer func() { model.Agents = nil }()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	task := &model.ScheduledTask{
		ProjectPath: env.ProjectDir,
		Name:        "Update Me",
		CronExpr:    "0 * * * *",
		AgentID:     "coder",
		Prompt:      "Test",
		RepeatMode:  "unlimited",
	}
	s.AddTask(task)

	req := newRequest(t, http.MethodPut, "/api/tasks/"+task.ID, map[string]any{
		"action":   "update",
		"name":     "Updated Name",
		"cron_expr": "0 */2 * * *",
		"prompt":   "Updated prompt",
	})
	w := callHandler(ServeTaskByID, req)
	assertOK(t, w)
}

func TestServeTaskByID_UpdateAssistantAgent(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	model.Agents = map[string]*model.Agent{
		"coder": {ID: "coder", Name: "Coder", Backend: "claude"},
	}
	defer func() { model.Agents = nil }()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	task := &model.ScheduledTask{
		ProjectPath: env.ProjectDir,
		Name:        "Bad Update",
		CronExpr:    "0 * * * *",
		AgentID:     "coder",
		Prompt:      "Test",
		RepeatMode:  "unlimited",
	}
	s.AddTask(task)

	req := newRequest(t, http.MethodPut, "/api/tasks/"+task.ID, map[string]any{
		"agent_id": "assistant",
	})
	w := callHandler(ServeTaskByID, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeTaskByID_UpdateTaskNotFound(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	s := service.NewScheduler()
	defer s.Stop()
	service.GlobalScheduler = s
	defer func() { service.GlobalScheduler = nil }()

	req := newRequest(t, http.MethodPut, "/api/tasks/non-existent", map[string]any{
		"action": "update",
		"name":   "Updated",
	})
	w := callHandler(ServeTaskByID, req)
	assertStatus(t, w, http.StatusNotFound)
}

// ---------- helper ----------

func createTestSession(t *testing.T, projectPath string) string {
	t.Helper()
	id, err := service.CreateSession(projectPath, "claude", "Test Session", "", "")
	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
	t.Cleanup(func() {
		service.SetSessionRunning(id, false)
	})
	return id
}

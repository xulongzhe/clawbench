package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"clawbench/internal/model"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
)

// --- POST /api/ai/session/resume tests ---

func TestServeSessionResume_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/ai/session/resume", nil)
	withProjectCookie(req, "/some/project")
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeSessionResume_MissingProject(t *testing.T) {
	body := `{"session_id": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/session/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestServeSessionResume_MissingSessionID(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/session/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeSessionResume_SessionNotFound(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	body := `{"session_id": "nonexistent-session"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/session/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestServeSessionResume_RestoresSoftDeletedSession(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "test-resume-session"
	_, err := service.DB.Exec(
		"INSERT INTO chat_sessions (id, project_path, backend, title, deleted) VALUES (?, ?, 'claude', 'Test Session', 1)",
		sessionID, env.ProjectDir,
	)
	assert.NoError(t, err)

	body := `{"session_id": "test-resume-session"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/session/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var deleted int
	err = service.DB.QueryRow("SELECT deleted FROM chat_sessions WHERE id = ?", sessionID).Scan(&deleted)
	assert.NoError(t, err)
	assert.Equal(t, 0, deleted, "session should be restored (deleted=0)")
}

func TestServeSessionResume_ActiveSessionPassthrough(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "test-active-session"
	_, err := service.DB.Exec(
		"INSERT INTO chat_sessions (id, project_path, backend, title, deleted) VALUES (?, ?, 'claude', 'Active Session', 0)",
		sessionID, env.ProjectDir,
	)
	assert.NoError(t, err)

	body := `{"session_id": "test-active-session"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/session/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServeSessionResume_InvalidJSON(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/ai/session/resume", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeSessionResume_SessionCountBelowLimit(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	origMax := model.SessionMaxCount
	model.SessionMaxCount = 10
	defer func() { model.SessionMaxCount = origMax }()

	// Create a soft-deleted session to resume
	sessionID := "test-resume-below-limit"
	_, err := service.DB.Exec(
		"INSERT INTO chat_sessions (id, project_path, backend, title, deleted) VALUES (?, ?, 'claude', 'Deleted Session', 1)",
		sessionID, env.ProjectDir,
	)
	assert.NoError(t, err)

	body := `{"session_id": "test-resume-below-limit"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/session/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var deleted int
	err = service.DB.QueryRow("SELECT deleted FROM chat_sessions WHERE id = ?", sessionID).Scan(&deleted)
	assert.NoError(t, err)
	assert.Equal(t, 0, deleted, "session should be restored (deleted=0)")
}

func TestServeSessionResume_CrossProjectDenied(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "test-other-project-session"
	_, err := service.DB.Exec(
		"INSERT INTO chat_sessions (id, project_path, backend, title, deleted) VALUES (?, '/other/project', 'claude', 'Other Session', 0)",
		sessionID,
	)
	assert.NoError(t, err)

	body := `{"session_id": "test-other-project-session"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/session/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestServeSessionResume_SessionCountLimit(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	origMax := model.SessionMaxCount
	model.SessionMaxCount = 1
	defer func() { model.SessionMaxCount = origMax }()

	// Create an active session (fills the 1-slot limit)
	_, err := service.DB.Exec(
		"INSERT INTO chat_sessions (id, project_path, backend, title, deleted) VALUES (?, ?, 'claude', 'Active', 0)",
		"existing-session", env.ProjectDir,
	)
	assert.NoError(t, err)

	// Create a soft-deleted session to resume
	_, err = service.DB.Exec(
		"INSERT INTO chat_sessions (id, project_path, backend, title, deleted) VALUES (?, ?, 'claude', 'Deleted', 1)",
		"deleted-session", env.ProjectDir,
	)
	assert.NoError(t, err)

	// Restoring the deleted session would make total active = 2, exceeding limit 1
	body := `{"session_id": "deleted-session"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/session/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)
	w := httptest.NewRecorder()
	ServeSessionResume(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

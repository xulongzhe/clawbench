package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
)

func TestServeProjectSet(t *testing.T) {
	t.Run("GET_WithProjectCookie_ReturnsPath", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		projectPath := filepath.Join(env.WatchDir, "myproject")
		os.MkdirAll(projectPath, 0755)

		req := newRequest(t, http.MethodGet, "/api/project", nil)
		withProjectCookie(req, projectPath)

		w := callHandler(ServeProjectSet, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assertJSONField(t, w, "path", projectPath)
	})

	t.Run("GET_WithoutCookie_FallsBackToWatchDir", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/project", nil)
		// No project cookie

		w := callHandler(ServeProjectSet, req)

		assert.Equal(t, http.StatusOK, w.Code)
		absWatchDir, _ := filepath.Abs(env.WatchDir)
		assertJSONField(t, w, "path", absWatchDir)
	})

	t.Run("GET_WithoutCookie_FallsBackToRecentProject", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		recentPath := filepath.Join(env.WatchDir, "recentproject")
		os.MkdirAll(recentPath, 0755)

		// Insert a recent project directly into the DB
		_, err := service.DB.Exec(
			"INSERT INTO recent_projects (project_path) VALUES (?)", recentPath,
		)
		assert.NoError(t, err)

		req := newRequest(t, http.MethodGet, "/api/project", nil)

		w := callHandler(ServeProjectSet, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assertJSONField(t, w, "path", recentPath)
	})

	t.Run("POST_ValidPath_SetsCookieAndClearsSession", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		projectPath := filepath.Join(env.WatchDir, "myproject")
		os.MkdirAll(projectPath, 0755)

		req := newRequest(t, http.MethodPost, "/api/project", map[string]string{
			"path": projectPath,
		})
		withSessionCookie(req, "old-session-id")

		w := callHandler(ServeProjectSet, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assertJSONField(t, w, "ok", "true")

		// Verify project cookie is set
		var projectCookieFound, sessionCleared bool
		for _, c := range w.Result().Cookies() {
			if c.Name == "clawbench_project" {
				projectCookieFound = true
				decoded, _ := url.QueryUnescape(c.Value)
				assert.Equal(t, projectPath, decoded)
			}
			if c.Name == "chat_session_id" {
				sessionCleared = true
				assert.Equal(t, -1, c.MaxAge, "session cookie should be cleared (MaxAge=-1)")
			}
		}
		assert.True(t, projectCookieFound, "expected project cookie to be set")
		assert.True(t, sessionCleared, "expected chat session cookie to be cleared")
	})

	t.Run("POST_PathOutsideWatchDir_Returns403", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		// Use a path with .. that resolves outside WatchDir
		// The handler resolves this to an absolute path and checks it's under WatchDir
		req := newRequest(t, http.MethodPost, "/api/project", map[string]string{
			"path": "../../../etc",
		})

		w := callHandler(ServeProjectSet, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("POST_NonExistentDirectory_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/project", map[string]string{
			"path": filepath.Join(env.WatchDir, "nonexistent"),
		})

		w := callHandler(ServeProjectSet, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST_InvalidBody_Returns400", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := httptest.NewRequest(http.MethodPost, "/api/project", nil)
		w := callHandler(ServeProjectSet, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("OtherMethod_Returns405", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodDelete, "/api/project", nil)
		w := callHandler(ServeProjectSet, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestServeRecentProjects(t *testing.T) {
	t.Run("GET_ReturnsEmptyList", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/recent-projects", nil)
		w := callHandler(ServeRecentProjects, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var result []string
		decodeJSON(t, w.Body, &result)
		assert.Empty(t, result)
	})

	t.Run("GET_WithExistingProjects_ReturnsList", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		projectPath := filepath.Join(env.WatchDir, "proj1")
		os.MkdirAll(projectPath, 0755)
		_, err := service.DB.Exec(
			"INSERT INTO recent_projects (project_path) VALUES (?)", projectPath,
		)
		assert.NoError(t, err)

		req := newRequest(t, http.MethodGet, "/api/recent-projects", nil)
		w := callHandler(ServeRecentProjects, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var result []string
		decodeJSON(t, w.Body, &result)
		assert.Contains(t, result, projectPath)
	})

	t.Run("POST_AddProject_ReturnsOK", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/recent-projects", map[string]string{
			"path": "/some/project",
		})
		w := callHandler(ServeRecentProjects, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assertJSONField(t, w, "ok", true)
	})

	t.Run("POST_InvalidBody_Returns400", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := httptest.NewRequest(http.MethodPost, "/api/recent-projects", nil)
		w := callHandler(ServeRecentProjects, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("OtherMethod_Returns405", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPut, "/api/recent-projects", nil)
		w := callHandler(ServeRecentProjects, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

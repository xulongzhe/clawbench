package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPathUnderAnyRoot(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	t.Run("PathUnderRoot_ReturnsTrue", func(t *testing.T) {
		assert.True(t, isPathUnderAnyRoot(env.ProjectDir))
		assert.True(t, isPathUnderAnyRoot(filepath.Join(env.WatchDir, "subdir")))
	})

	t.Run("PathOutsideRoot_ReturnsFalse", func(t *testing.T) {
		assert.False(t, isPathUnderAnyRoot("/etc/passwd"))
		assert.False(t, isPathUnderAnyRoot(os.TempDir()))
	})

	t.Run("ExactRootPath_ReturnsTrue", func(t *testing.T) {
		assert.True(t, isPathUnderAnyRoot(env.WatchDir))
	})
}

func TestResolveAbsPath(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	t.Run("AbsolutePathUnderRoot_ReturnsPath", func(t *testing.T) {
		createTestFile(t, env.WatchDir, "absfile.txt", "data")
		absPath := filepath.Join(env.WatchDir, "absfile.txt")

		req := newRequest(t, http.MethodPost, "/api/test", nil)
		w := httptest.NewRecorder()
		result, ok := resolveAbsPath(w, req, absPath)
		assert.True(t, ok)
		assert.Equal(t, absPath, result)
	})

	t.Run("AbsolutePathOutsideRoot_ReturnsFalse", func(t *testing.T) {
		req := newRequest(t, http.MethodPost, "/api/test", nil)
		w := httptest.NewRecorder()
		result, ok := resolveAbsPath(w, req, "/etc/passwd")
		assert.False(t, ok)
		assert.Empty(t, result)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("RelativePath_ResolvesAgainstProjectCookie", func(t *testing.T) {
		createTestFile(t, env.ProjectDir, "relfile.txt", "data")

		req := newRequest(t, http.MethodPost, "/api/test", nil)
		withProjectCookie(req, env.ProjectDir)
		w := httptest.NewRecorder()
		result, ok := resolveAbsPath(w, req, "relfile.txt")
		assert.True(t, ok)
		assert.Contains(t, result, "relfile.txt")
	})

	t.Run("RelativePathWithoutProject_Returns403", func(t *testing.T) {
		req := newRequest(t, http.MethodPost, "/api/test", nil)
		w := httptest.NewRecorder()
		result, ok := resolveAbsPath(w, req, "relfile.txt")
		assert.False(t, ok)
		assert.Empty(t, result)
	})
}

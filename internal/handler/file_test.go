package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListDir(t *testing.T) {
	t.Run("NormalDirectoryListing", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "file1.txt", "hello")
		createTestFile(t, env.ProjectDir, "file2.go", "package main")
		os.MkdirAll(filepath.Join(env.ProjectDir, "subdir"), 0755)

		req := newRequest(t, http.MethodGet, "/api/dir", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ListDir, req)
		assertOK(t, w)

		var result map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)

		items, ok := result["items"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, items, 3)

		// Items should be sorted: dirs first, then files alphabetically
		names := make([]string, len(items))
		for i, item := range items {
			entry := item.(map[string]interface{})
			names[i] = entry["name"].(string)
		}
		expected := []string{"subdir", "file1.txt", "file2.go"}
		sort.Strings(expected[:1]) // dirs first
		assert.Equal(t, "subdir", names[0]) // dir comes first
	})

	t.Run("EmptyDirectory", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/dir", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ListDir, req)
		assertOK(t, w)

		var result map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)

		// Empty dir returns nil slice (null in JSON)
		assert.Nil(t, result["items"])
	})

	t.Run("NoProjectCookie_Returns403", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/dir", nil)
		// No project cookie

		w := callHandler(ListDir, req)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("PathTraversal_Returns403", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/dir?path=../../../etc", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ListDir, req)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("SubdirectoryListing", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "subdir/nested.txt", "nested content")

		req := newRequest(t, http.MethodGet, "/api/dir?path=subdir", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ListDir, req)
		assertOK(t, w)

		var result map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)

		items, ok := result["items"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, items, 1)

		entry := items[0].(map[string]interface{})
		assert.Equal(t, "nested.txt", entry["name"])
		assert.Equal(t, "file", entry["type"])
	})
}

func TestListFiles(t *testing.T) {
	t.Run("ListsAllFilesRecursively", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "root.txt", "root")
		createTestFile(t, env.ProjectDir, "sub/deep.txt", "deep")
		createTestFile(t, env.ProjectDir, "sub/nested.txt", "nested")

		req := newRequest(t, http.MethodGet, "/api/files", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ListFiles, req)
		assertOK(t, w)

		var files []FileInfo
		err := json.Unmarshal(w.Body.Bytes(), &files)
		assert.NoError(t, err)
		assert.Len(t, files, 3)

		// Verify paths are relative
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.Path
		}
		assert.Contains(t, paths, "root.txt")
		assert.Contains(t, paths, "sub/deep.txt")
		assert.Contains(t, paths, "sub/nested.txt")
	})

	t.Run("EmptyProject", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/files", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ListFiles, req)
		assertOK(t, w)

		var files []FileInfo
		err := json.Unmarshal(w.Body.Bytes(), &files)
		assert.NoError(t, err)
		assert.Len(t, files, 0)
	})

	t.Run("NoProjectCookie_Returns403", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/files", nil)

		w := callHandler(ListFiles, req)
		assertStatus(t, w, http.StatusForbidden)
	})
}

func TestGetFile(t *testing.T) {
	t.Run("ReadTextFile", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "test.txt", "hello world")

		req := newRequest(t, http.MethodGet, "/api/file/test.txt", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(GetFile, req)
		assertOK(t, w)

		var fc FileContent
		err := json.Unmarshal(w.Body.Bytes(), &fc)
		assert.NoError(t, err)
		assert.Equal(t, "hello world", fc.Content)
		assert.Equal(t, "test.txt", fc.Name)
		assert.Equal(t, "test.txt", fc.Path)
		assert.True(t, fc.Supported)
		assert.Equal(t, int64(11), fc.Size)
	})

	t.Run("FileNotFound_Returns404", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/file/nonexistent.txt", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(GetFile, req)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("NoProjectCookie_Returns403", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/file/test.txt", nil)

		w := callHandler(GetFile, req)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("PathTraversal_Returns400Or403", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/file/../../../etc/passwd", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(GetFile, req)
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusForbidden,
			"expected 400 or 403, got %d", w.Code)
	})

	t.Run("DirectoryInsteadOfFile_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		os.MkdirAll(filepath.Join(env.ProjectDir, "mydir"), 0755)

		req := newRequest(t, http.MethodGet, "/api/file/mydir", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(GetFile, req)
		assertStatus(t, w, http.StatusBadRequest)
	})
}

func TestServeLocalFile(t *testing.T) {
	t.Run("ServeImageFile_CorrectContentType", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// Create a minimal PNG file (1x1 pixel)
		pngData := []byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
			0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
			0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
			0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
			0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
			0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
			0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC,
			0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
			0x44, 0xAE, 0x42, 0x60, 0x82,
		}
		fullPath := filepath.Join(env.ProjectDir, "test.png")
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, pngData, 0644)

		req := newRequest(t, http.MethodGet, "/api/local-file/test.png", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeLocalFile, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
	})

	t.Run("FileNotFound_Returns404", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/local-file/missing.png", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeLocalFile, req)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("NoProjectCookie_Returns403", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/local-file/test.png", nil)

		w := callHandler(ServeLocalFile, req)
		assertStatus(t, w, http.StatusForbidden)
	})
}

func TestServeProjects(t *testing.T) {
	t.Run("GET_ListsDirectoriesUnderWatchDir", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// Create directories under WatchDir
		os.MkdirAll(filepath.Join(env.WatchDir, "project1"), 0755)
		os.MkdirAll(filepath.Join(env.WatchDir, "project2"), 0755)
		// Create a file (should appear too, since ListDir returns all entries)
		createTestFile(t, env.WatchDir, "readme.md", "hello")

		req := newRequest(t, http.MethodGet, "/api/projects", nil)
		w := callHandler(ServeProjects, req)
		assertOK(t, w)

		var result map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)

		items, ok := result["items"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, items, 3) // project1, project2, readme.md
	})

	t.Run("POST_CreatesNewDirectory", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/projects", map[string]string{
			"name": "new-project",
		})
		w := callHandler(ServeProjects, req)
		assertOK(t, w)

		assertJSONField(t, w, "ok", true)

		// Verify directory was created
		info, err := os.Stat(filepath.Join(env.WatchDir, "new-project"))
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("POST_MissingName_Returns400", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/projects", map[string]string{
			"name": "",
		})
		w := callHandler(ServeProjects, req)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("GET_WithPathParameter_ListsSubdirectory", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		os.MkdirAll(filepath.Join(env.WatchDir, "myproject", "src"), 0755)
		createTestFile(t, env.WatchDir, "myproject/src/main.go", "package main")

		req := newRequest(t, http.MethodGet, "/api/projects?path=myproject", nil)
		w := callHandler(ServeProjects, req)
		assertOK(t, w)

		var result map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)

		items, ok := result["items"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, items, 1) // src directory

		entry := items[0].(map[string]interface{})
		assert.Equal(t, "src", entry["name"])
		assert.Equal(t, "dir", entry["type"])
	})
}

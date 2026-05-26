package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// createMultipartUploadRequest builds a multipart/form-data POST request with
// a file field and an optional "dir" field.
func createMultipartUploadRequest(t *testing.T, filename, content, dir string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write([]byte(content))

	if dir != "" {
		writer.WriteField("dir", dir)
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/file", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadFile_DefaultDir(t *testing.T) {
	t.Run("UploadToDefaultDir", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := createMultipartUploadRequest(t, "hello.txt", "hello world", "")
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertOK(t, w)

		var result map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, true, result["ok"])

		// Path should be under .clawbench/uploads/
		pathStr, ok := result["path"].(string)
		assert.True(t, ok)
		assert.Contains(t, pathStr, ".clawbench/uploads/hello.txt")

		// File should exist on disk
		fullPath := filepath.Join(env.ProjectDir, pathStr)
		data, err := os.ReadFile(fullPath)
		assert.NoError(t, err)
		assert.Equal(t, "hello world", string(data))
	})

	t.Run("NoFile_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// Empty multipart form without file field
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/upload/file", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("NoExtension_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := createMultipartUploadRequest(t, "noext", "content", "")
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("DangerousExtension_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := createMultipartUploadRequest(t, "evil.exe", "MZ", "")
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("DuplicateFilename_AppendsNumber", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// First upload
		req1 := createMultipartUploadRequest(t, "dup.txt", "first", "")
		withProjectCookie(req1, env.ProjectDir)
		w1 := callHandler(UploadFile, req1)
		assertOK(t, w1)

		// Second upload with same name
		req2 := createMultipartUploadRequest(t, "dup.txt", "second", "")
		withProjectCookie(req2, env.ProjectDir)
		w2 := callHandler(UploadFile, req2)
		assertOK(t, w2)

		var result map[string]interface{}
		json.Unmarshal(w2.Body.Bytes(), &result)
		pathStr := result["path"].(string)
		assert.Contains(t, pathStr, "dup_1.txt")
	})

	t.Run("SpacesInFilename_ReplacedWithUnderscore", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := createMultipartUploadRequest(t, "my file.txt", "content", "")
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertOK(t, w)

		var result map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &result)
		pathStr := result["path"].(string)
		assert.Contains(t, pathStr, "my_file.txt")
		assert.NotContains(t, pathStr, "my file.txt")
	})

	t.Run("WrongMethod_GET_Returns405", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/api/upload/file", nil)
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertStatus(t, w, http.StatusMethodNotAllowed)
	})
}

func TestUploadFile_CustomDir(t *testing.T) {
	t.Run("UploadToCustomDir", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// Create a subdirectory to upload into
		subDir := filepath.Join(env.ProjectDir, "subdir")
		os.MkdirAll(subDir, 0755)

		req := createMultipartUploadRequest(t, "test.txt", "custom dir content", "subdir")
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertOK(t, w)

		var result map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, true, result["ok"])

		pathStr, ok := result["path"].(string)
		assert.True(t, ok)
		assert.Contains(t, pathStr, "subdir/test.txt")

		// Verify file on disk
		fullPath := filepath.Join(env.ProjectDir, pathStr)
		data, err := os.ReadFile(fullPath)
		assert.NoError(t, err)
		assert.Equal(t, "custom dir content", string(data))
	})

	t.Run("DirNotFound_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := createMultipartUploadRequest(t, "test.txt", "content", "nonexistent_dir")
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("DirIsFile_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// Create a file where we expect a directory
		createTestFile(t, env.ProjectDir, "not_a_dir.txt", "I'm a file")

		req := createMultipartUploadRequest(t, "test.txt", "content", "not_a_dir.txt")
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("PathTraversalInDir_Returns403", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := createMultipartUploadRequest(t, "test.txt", "content", "../../../etc")
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("DuplicateFileInCustomDir_AppendsNumber", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		subDir := filepath.Join(env.ProjectDir, "mydir")
		os.MkdirAll(subDir, 0755)

		// First upload
		req1 := createMultipartUploadRequest(t, "dup.txt", "first", "mydir")
		withProjectCookie(req1, env.ProjectDir)
		w1 := callHandler(UploadFile, req1)
		assertOK(t, w1)

		// Second upload with same filename
		req2 := createMultipartUploadRequest(t, "dup.txt", "second", "mydir")
		withProjectCookie(req2, env.ProjectDir)
		w2 := callHandler(UploadFile, req2)
		assertOK(t, w2)

		var result map[string]interface{}
		json.Unmarshal(w2.Body.Bytes(), &result)
		pathStr := result["path"].(string)
		assert.Contains(t, pathStr, "dup_1.txt")
	})

	t.Run("NestedDirUpload", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// Create nested directory
		nestedDir := filepath.Join(env.ProjectDir, "a", "b", "c")
		os.MkdirAll(nestedDir, 0755)

		req := createMultipartUploadRequest(t, "deep.txt", "deep content", "a/b/c")
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(UploadFile, req)
		assertOK(t, w)

		var result map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &result)
		pathStr := result["path"].(string)
		assert.Contains(t, pathStr, "a/b/c/deep.txt")
	})
}

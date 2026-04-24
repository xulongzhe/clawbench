package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeFileRename(t *testing.T) {
	t.Run("RenameFile_Succeeds", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "old.txt", "hello")

		req := newRequest(t, http.MethodPost, "/api/file/rename", map[string]string{
			"path": "old.txt",
			"name": "new.txt",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileRename, req)
		assertOK(t, w)
		assertJSONField(t, w, "ok", true)

		_, err := os.Stat(filepath.Join(env.ProjectDir, "new.txt"))
		assert.NoError(t, err)
		_, err = os.Stat(filepath.Join(env.ProjectDir, "old.txt"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("MissingPathOrName_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// Missing name
		req := newRequest(t, http.MethodPost, "/api/file/rename", map[string]string{
			"path": "old.txt",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileRename, req)
		assertStatus(t, w, http.StatusBadRequest)

		// Missing path
		req2 := newRequest(t, http.MethodPost, "/api/file/rename", map[string]string{
			"name": "new.txt",
		})
		withProjectCookie(req2, env.ProjectDir)

		w2 := callHandler(ServeFileRename, req2)
		assertStatus(t, w2, http.StatusBadRequest)
	})

	t.Run("NoProjectCookieAndNoBasePath_Returns403", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/file/rename", map[string]string{
			"path": "old.txt",
			"name": "new.txt",
		})
		// No project cookie, no basePath

		w := callHandler(ServeFileRename, req)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("PathTraversalInPath_Returns403", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/file/rename", map[string]string{
			"path": "../../../etc/passwd",
			"name": "hacked",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileRename, req)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("WithBasePathOverride_UsesBasePath", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		altDir := t.TempDir()
		createTestFile(t, altDir, "file.txt", "data")

		req := newRequest(t, http.MethodPost, "/api/file/rename", map[string]string{
			"path":     "file.txt",
			"name":     "renamed.txt",
			"basePath": altDir,
		})
		// No project cookie needed when basePath is provided

		w := callHandler(ServeFileRename, req)
		assertOK(t, w)

		_, err := os.Stat(filepath.Join(altDir, "renamed.txt"))
		assert.NoError(t, err)
	})
}

func TestServeFileEditLine(t *testing.T) {
	t.Run("EditSpecificLine_ContentUpdated", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "edit.txt", "line1\nline2\nline3")

		req := newRequest(t, http.MethodPost, "/api/file/edit-line", map[string]interface{}{
			"path":    "edit.txt",
			"lineNum": 2,
			"content": "LINE2_EDITED",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileEditLine, req)
		assertOK(t, w)

		data, err := os.ReadFile(filepath.Join(env.ProjectDir, "edit.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "line1\nLINE2_EDITED\nline3", string(data))
	})

	t.Run("InsertLineAbove", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "insert.txt", "line1\nline2\nline3")

		req := newRequest(t, http.MethodPost, "/api/file/edit-line", map[string]interface{}{
			"path":        "insert.txt",
			"lineNum":     2,
			"insertAbove": true,
			"content":     "INSERTED",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileEditLine, req)
		assertOK(t, w)

		data, err := os.ReadFile(filepath.Join(env.ProjectDir, "insert.txt"))
		assert.NoError(t, err)
		lines := splitLines(string(data))
		assert.Equal(t, 4, len(lines))
		assert.Equal(t, "", lines[1]) // empty line inserted above line 2
	})

	t.Run("InsertLineBelow", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "insert.txt", "line1\nline2\nline3")

		req := newRequest(t, http.MethodPost, "/api/file/edit-line", map[string]interface{}{
			"path":        "insert.txt",
			"lineNum":     2,
			"insertBelow": true,
			"content":     "INSERTED",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileEditLine, req)
		assertOK(t, w)

		data, err := os.ReadFile(filepath.Join(env.ProjectDir, "insert.txt"))
		assert.NoError(t, err)
		lines := splitLines(string(data))
		assert.Equal(t, 4, len(lines))
		assert.Equal(t, "", lines[2]) // empty line inserted below line 2
	})

	t.Run("DeleteLine", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "delete.txt", "line1\nline2\nline3")

		req := newRequest(t, http.MethodPost, "/api/file/edit-line", map[string]interface{}{
			"path":    "delete.txt",
			"lineNum": 2,
			"delete":  true,
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileEditLine, req)
		assertOK(t, w)

		data, err := os.ReadFile(filepath.Join(env.ProjectDir, "delete.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "line1\nline3", string(data))
	})

	t.Run("LineNumberOutOfRange_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "short.txt", "only_one_line")

		req := newRequest(t, http.MethodPost, "/api/file/edit-line", map[string]interface{}{
			"path":    "short.txt",
			"lineNum": 99,
			"content": "nope",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileEditLine, req)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("MissingPathOrInvalidLineNum_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// Missing path
		req := newRequest(t, http.MethodPost, "/api/file/edit-line", map[string]interface{}{
			"lineNum": 1,
			"content": "x",
		})
		withProjectCookie(req, env.ProjectDir)
		w := callHandler(ServeFileEditLine, req)
		assertStatus(t, w, http.StatusBadRequest)

		// Invalid lineNum (0)
		req2 := newRequest(t, http.MethodPost, "/api/file/edit-line", map[string]interface{}{
			"path":    "file.txt",
			"lineNum": 0,
			"content": "x",
		})
		withProjectCookie(req2, env.ProjectDir)
		w2 := callHandler(ServeFileEditLine, req2)
		assertStatus(t, w2, http.StatusBadRequest)
	})

	t.Run("NoProjectCookie_Returns403", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/file/edit-line", map[string]interface{}{
			"path":    "file.txt",
			"lineNum": 1,
			"content": "x",
		})

		w := callHandler(ServeFileEditLine, req)
		assertStatus(t, w, http.StatusForbidden)
	})
}

func TestServeFileDelete(t *testing.T) {
	t.Run("DeleteFile_Succeeds", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "todelete.txt", "bye")

		req := newRequest(t, http.MethodPost, "/api/file/delete", map[string]string{
			"path": "todelete.txt",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileDelete, req)
		assertOK(t, w)
		assertJSONField(t, w, "ok", true)

		_, err := os.Stat(filepath.Join(env.ProjectDir, "todelete.txt"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("DeleteDirectoryRecursive_Succeeds", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "mydir/file1.txt", "a")
		createTestFile(t, env.ProjectDir, "mydir/file2.txt", "b")

		req := newRequest(t, http.MethodPost, "/api/file/delete", map[string]string{
			"path": "mydir",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileDelete, req)
		assertOK(t, w)

		_, err := os.Stat(filepath.Join(env.ProjectDir, "mydir"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("NonExistentFile_Returns404", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/file/delete", map[string]string{
			"path": "nonexistent.txt",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileDelete, req)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("WithBasePathOverride", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		altDir := t.TempDir()
		createTestFile(t, altDir, "del.txt", "gone")

		req := newRequest(t, http.MethodPost, "/api/file/delete", map[string]string{
			"path":     "del.txt",
			"basePath": altDir,
		})

		w := callHandler(ServeFileDelete, req)
		assertOK(t, w)

		_, err := os.Stat(filepath.Join(altDir, "del.txt"))
		assert.True(t, os.IsNotExist(err))
	})
}

func TestServeFileCreate(t *testing.T) {
	t.Run("CreateNewFile_Succeeds", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/file/create", map[string]string{
			"name": "newfile.txt",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileCreate, req)
		assertOK(t, w)
		assertJSONField(t, w, "ok", true)

		info, err := os.Stat(filepath.Join(env.ProjectDir, "newfile.txt"))
		assert.NoError(t, err)
		assert.Equal(t, int64(0), info.Size())
	})

	t.Run("FileAlreadyExists_Returns409", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "existing.txt", "already here")

		req := newRequest(t, http.MethodPost, "/api/file/create", map[string]string{
			"name": "existing.txt",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileCreate, req)
		assertStatus(t, w, http.StatusConflict)
	})

	t.Run("MissingName_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/file/create", map[string]string{
			"name": "",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileCreate, req)
		assertStatus(t, w, http.StatusBadRequest)
	})
}

func TestServeDirCreate(t *testing.T) {
	t.Run("CreateNewDirectory_Succeeds", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/dir/create", map[string]string{
			"name": "newdir",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeDirCreate, req)
		assertOK(t, w)
		assertJSONField(t, w, "ok", true)

		info, err := os.Stat(filepath.Join(env.ProjectDir, "newdir"))
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("MissingName_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/dir/create", map[string]string{
			"name": "",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeDirCreate, req)
		assertStatus(t, w, http.StatusBadRequest)
	})
}

func TestServeFileMove(t *testing.T) {
	t.Run("MoveFileToNewLocation_Succeeds", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "src.txt", "move me")
		os.MkdirAll(filepath.Join(env.ProjectDir, "dest"), 0755)

		req := newRequest(t, http.MethodPost, "/api/file/move", map[string]string{
			"path": "src.txt",
			"dest": "dest/src.txt",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileMove, req)
		assertOK(t, w)
		assertJSONField(t, w, "ok", true)

		_, err := os.Stat(filepath.Join(env.ProjectDir, "dest", "src.txt"))
		assert.NoError(t, err)
		_, err = os.Stat(filepath.Join(env.ProjectDir, "src.txt"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("MissingPathOrDest_Returns400", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// Missing dest
		req := newRequest(t, http.MethodPost, "/api/file/move", map[string]string{
			"path": "src.txt",
		})
		withProjectCookie(req, env.ProjectDir)
		w := callHandler(ServeFileMove, req)
		assertStatus(t, w, http.StatusBadRequest)

		// Missing path
		req2 := newRequest(t, http.MethodPost, "/api/file/move", map[string]string{
			"dest": "dest.txt",
		})
		withProjectCookie(req2, env.ProjectDir)
		w2 := callHandler(ServeFileMove, req2)
		assertStatus(t, w2, http.StatusBadRequest)
	})

	t.Run("NoProjectCookie_Returns403", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/file/move", map[string]string{
			"path": "src.txt",
			"dest": "dest.txt",
		})

		w := callHandler(ServeFileMove, req)
		assertStatus(t, w, http.StatusForbidden)
	})
}

func TestServeFileCopy(t *testing.T) {
	t.Run("CopyFile_Succeeds_ContentIdentical", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "original.txt", "copy this content")

		req := newRequest(t, http.MethodPost, "/api/file/copy", map[string]string{
			"path": "original.txt",
			"dest": "copy.txt",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileCopy, req)
		assertOK(t, w)
		assertJSONField(t, w, "ok", true)

		data, err := os.ReadFile(filepath.Join(env.ProjectDir, "copy.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "copy this content", string(data))

		// Original should still exist
		origData, err := os.ReadFile(filepath.Join(env.ProjectDir, "original.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "copy this content", string(origData))
	})

	t.Run("CopyDirectoryRecursive_Succeeds", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		createTestFile(t, env.ProjectDir, "srcdir/a.txt", "aaa")
		createTestFile(t, env.ProjectDir, "srcdir/sub/b.txt", "bbb")

		req := newRequest(t, http.MethodPost, "/api/file/copy", map[string]string{
			"path": "srcdir",
			"dest": "destdir",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileCopy, req)
		assertOK(t, w)

		data, err := os.ReadFile(filepath.Join(env.ProjectDir, "destdir", "a.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "aaa", string(data))

		data2, err := os.ReadFile(filepath.Join(env.ProjectDir, "destdir", "sub", "b.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "bbb", string(data2))
	})

	t.Run("SourceNotFound_Returns500", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPost, "/api/file/copy", map[string]string{
			"path": "nonexistent.txt",
			"dest": "copy.txt",
		})
		withProjectCookie(req, env.ProjectDir)

		w := callHandler(ServeFileCopy, req)
		assertStatus(t, w, http.StatusInternalServerError)
	})
}

// splitLines splits a string by newline, matching the handler's behavior.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

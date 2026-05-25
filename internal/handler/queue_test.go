package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
)

func TestQueueHandler_Enqueue_Success(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-enqueue-1"
	defer service.ClearQueue(sessionID)

	body := map[string]any{
		"message": "hello world",
	}
	req := newRequest(t, http.MethodPost, "/api/ai/queue?session_id="+sessionID, body)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, true, result["ok"])
	queue := result["queue"].([]any)
	assert.Len(t, queue, 1)
}

func TestQueueHandler_Enqueue_WithFilePaths(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-enqueue-paths"
	defer service.ClearQueue(sessionID)

	body := map[string]any{
		"message":   "check this file",
		"filePaths": []string{"/main.go", "/util.go"},
	}
	req := newRequest(t, http.MethodPost, "/api/ai/queue?session_id="+sessionID, body)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	queue := result["queue"].([]any)
	assert.Len(t, queue, 1)
	item := queue[0].(map[string]any)
	filePaths := item["filePaths"].([]any)
	assert.Len(t, filePaths, 2)
	assert.Equal(t, "/main.go", filePaths[0])
	assert.Equal(t, "/util.go", filePaths[1])
}

func TestQueueHandler_Enqueue_WithFiles(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-enqueue-files"
	defer service.ClearQueue(sessionID)

	body := map[string]any{
		"files": []string{"/upload/a.png", "/upload/b.jpg"},
	}
	req := newRequest(t, http.MethodPost, "/api/ai/queue?session_id="+sessionID, body)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	queue := result["queue"].([]any)
	assert.Len(t, queue, 1)
	item := queue[0].(map[string]any)
	files := item["files"].([]any)
	assert.Len(t, files, 2)
}

func TestQueueHandler_Enqueue_MissingSessionID(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]any{"message": "test"}
	req := newRequest(t, http.MethodPost, "/api/ai/queue", body)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestQueueHandler_Enqueue_InvalidJSON(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-enqueue-badjson"
	req := httptest.NewRequest(http.MethodPost, "/api/ai/queue?session_id="+sessionID, nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "clawbench_project", Value: env.ProjectDir})
	w := callHandler(QueueHandler, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestQueueHandler_Enqueue_EmptyMessage(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-enqueue-empty"
	defer service.ClearQueue(sessionID)

	body := map[string]any{
		"message": "",
	}
	req := newRequest(t, http.MethodPost, "/api/ai/queue?session_id="+sessionID, body)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestQueueHandler_Get_Success(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-get-1"
	defer service.ClearQueue(sessionID)

	service.EnqueueMessage(sessionID, model.QueuedMessage{
		Text:      "hello",
		CreatedAt: time.Now().Format(time.RFC3339),
	})

	req := newRequest(t, http.MethodGet, "/api/ai/queue?session_id="+sessionID, nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	queue := result["queue"].([]any)
	assert.Len(t, queue, 1)
}

func TestQueueHandler_Get_MissingSessionID(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/ai/queue", nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestQueueHandler_Get_EmptyQueue(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-get-empty"
	defer service.ClearQueue(sessionID)

	req := newRequest(t, http.MethodGet, "/api/ai/queue?session_id="+sessionID, nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	queue := result["queue"]
	assert.NotNil(t, queue)
}

func TestQueueHandler_Delete_ClearAll(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-del-clear"
	defer service.ClearQueue(sessionID)

	service.EnqueueMessage(sessionID, model.QueuedMessage{
		Text: "msg1", CreatedAt: time.Now().Format(time.RFC3339),
	})
	service.EnqueueMessage(sessionID, model.QueuedMessage{
		Text: "msg2", CreatedAt: time.Now().Format(time.RFC3339),
	})

	req := newRequest(t, http.MethodDelete, "/api/ai/queue?session_id="+sessionID, nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, true, result["ok"])
	assert.Nil(t, service.GetQueue(sessionID))
}

func TestQueueHandler_Delete_RemoveByIndex(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-del-index"
	defer service.ClearQueue(sessionID)

	service.EnqueueMessage(sessionID, model.QueuedMessage{Text: "msg1", CreatedAt: time.Now().Format(time.RFC3339)})
	service.EnqueueMessage(sessionID, model.QueuedMessage{Text: "msg2", CreatedAt: time.Now().Format(time.RFC3339)})
	service.EnqueueMessage(sessionID, model.QueuedMessage{Text: "msg3", CreatedAt: time.Now().Format(time.RFC3339)})

	req := newRequest(t, http.MethodDelete, "/api/ai/queue?session_id="+sessionID+"&index=1", nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, true, result["ok"])
	queue := result["queue"].([]any)
	assert.Len(t, queue, 2)
}

func TestQueueHandler_Delete_InvalidIndex(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-del-badindex"
	defer service.ClearQueue(sessionID)

	service.EnqueueMessage(sessionID, model.QueuedMessage{Text: "msg1", CreatedAt: time.Now().Format(time.RFC3339)})

	req := newRequest(t, http.MethodDelete, "/api/ai/queue?session_id="+sessionID+"&index=abc", nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestQueueHandler_MethodNotAllowed(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPut, "/api/ai/queue?session_id=x", nil)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// TestQueueHandler_Enqueue_FilesNoDuplicate verifies that when both filePaths
// and files are provided (with files already containing filePaths, as the
// frontend sends), the queued message stores files without duplication.
func TestQueueHandler_Enqueue_FilesNoDuplicate(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-enqueue-dedup"
	defer service.ClearQueue(sessionID)

	// Simulate frontend: files already includes filePaths
	body := map[string]any{
		"message":   "check this file",
		"filePaths": []string{"config.yaml"},
		"files":     []string{"config.yaml"},
	}
	req := newRequest(t, http.MethodPost, "/api/ai/queue?session_id="+sessionID, body)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	queue := result["queue"].([]any)
	assert.Len(t, queue, 1)
	item := queue[0].(map[string]any)

	// filePaths should be preserved as-is for prompt building
	filePaths := item["filePaths"].([]any)
	assert.Len(t, filePaths, 1)
	assert.Equal(t, "config.yaml", filePaths[0])

	// files should NOT contain duplicates
	files := item["files"].([]any)
	assert.Len(t, files, 1, "files should have exactly 1 entry (no duplicate), got %v", files)
	assert.Equal(t, "config.yaml", files[0])
}

// TestQueueHandler_Enqueue_FilesWithUploads verifies that when files includes
// both uploads and references, all are preserved correctly.
func TestQueueHandler_Enqueue_FilesWithUploads(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID := "q-enqueue-uploads"
	defer service.ClearQueue(sessionID)

	body := map[string]any{
		"message":   "check these",
		"filePaths": []string{"src/main.go"},
		"files":     []string{".clawbench/uploads/img.png", "src/main.go"},
	}
	req := newRequest(t, http.MethodPost, "/api/ai/queue?session_id="+sessionID, body)
	req = withProjectCookie(req, env.ProjectDir)
	w := callHandler(QueueHandler, req)

	assertOK(t, w)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	queue := result["queue"].([]any)
	assert.Len(t, queue, 1)
	item := queue[0].(map[string]any)

	// files should preserve all entries without duplication
	files := item["files"].([]any)
	assert.Len(t, files, 2, "files should have 2 entries (upload + ref), got %v", files)
	assert.Equal(t, ".clawbench/uploads/img.png", files[0])
	assert.Equal(t, "src/main.go", files[1])
}

// ---------- Session ownership validation (ISS-180) ----------

func TestQueueHandler_Enqueue_SessionBelongsToDifferentProject(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create a session that belongs to a different project
	otherProject := "/other-project"
	sessionID, err := service.CreateSession(otherProject, "claude", "Other Session", "claude", "", "default", "chat")
	assert.NoError(t, err)
	defer service.ClearQueue(sessionID)

	body := map[string]any{"message": "hello world"}
	req := newRequest(t, http.MethodPost, "/api/ai/queue?session_id="+sessionID, body)
	req = withProjectCookie(req, env.ProjectDir) // different project
	w := callHandler(QueueHandler, req)

	assertStatus(t, w, http.StatusForbidden)
}

func TestQueueHandler_Get_SessionBelongsToDifferentProject(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create a session that belongs to a different project
	otherProject := "/other-project"
	sessionID, err := service.CreateSession(otherProject, "claude", "Other Session", "claude", "", "default", "chat")
	assert.NoError(t, err)

	req := newRequest(t, http.MethodGet, "/api/ai/queue?session_id="+sessionID, nil)
	req = withProjectCookie(req, env.ProjectDir) // different project
	w := callHandler(QueueHandler, req)

	assertStatus(t, w, http.StatusForbidden)
}

func TestQueueHandler_Delete_SessionBelongsToDifferentProject(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create a session that belongs to a different project
	otherProject := "/other-project"
	sessionID, err := service.CreateSession(otherProject, "claude", "Other Session", "claude", "", "default", "chat")
	assert.NoError(t, err)

	req := newRequest(t, http.MethodDelete, "/api/ai/queue?session_id="+sessionID, nil)
	req = withProjectCookie(req, env.ProjectDir) // different project
	w := callHandler(QueueHandler, req)

	assertStatus(t, w, http.StatusForbidden)
}

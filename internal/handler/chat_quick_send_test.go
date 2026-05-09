package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"clawbench/internal/model"
	"clawbench/internal/service"
)

// ---------- GET /api/chat/quick-send ----------

func TestServeChatQuickSend_ListEmpty(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/chat/quick-send", nil)
	w := callHandler(ServeChatQuickSend, req)

	assertOK(t, w)

	var items []service.ChatQuickSendItem
	decodeRespJSON(t, w.Body, &items)
	if items == nil {
		t.Error("expected empty array, got nil")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestServeChatQuickSend_ListWithItems(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	service.AddChatQuickSend("继续", "继续", false)
	service.AddChatQuickSend("提交", "提交", true)

	req := newRequest(t, http.MethodGet, "/api/chat/quick-send", nil)
	w := callHandler(ServeChatQuickSend, req)

	assertOK(t, w)

	var items []service.ChatQuickSendItem
	decodeRespJSON(t, w.Body, &items)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Label != "继续" {
		t.Errorf("expected first item label '继续', got %q", items[0].Label)
	}
	if !items[1].Hidden {
		t.Error("expected second item to be hidden")
	}
}

// ---------- POST /api/chat/quick-send ----------

func TestServeChatQuickSend_Create(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]any{"label": "继续", "command": "继续", "hidden": false}
	req := newRequest(t, http.MethodPost, "/api/chat/quick-send", body)
	w := callHandler(ServeChatQuickSend, req)

	assertStatus(t, w, http.StatusCreated)

	var result map[string]any
	decodeRespJSON(t, w.Body, &result)
	if result["id"] == nil {
		t.Error("expected id in response")
	}
	if result["label"] != "继续" {
		t.Errorf("expected label '继续', got %v", result["label"])
	}
}

func TestServeChatQuickSend_CreateEmptyLabel(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]any{"label": "", "command": "继续", "hidden": false}
	req := newRequest(t, http.MethodPost, "/api/chat/quick-send", body)
	w := callHandler(ServeChatQuickSend, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatQuickSend_CreateEmptyCommand(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]any{"label": "继续", "command": "   ", "hidden": false}
	req := newRequest(t, http.MethodPost, "/api/chat/quick-send", body)
	w := callHandler(ServeChatQuickSend, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatQuickSend_CreateLabelTooLong(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	longLabel := ""
	for i := 0; i < 101; i++ {
		longLabel += "x"
	}
	body := map[string]any{"label": longLabel, "command": "test", "hidden": false}
	req := newRequest(t, http.MethodPost, "/api/chat/quick-send", body)
	w := callHandler(ServeChatQuickSend, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatQuickSend_CreateCommandTooLong(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	longCommand := ""
	for i := 0; i < 4097; i++ {
		longCommand += "x"
	}
	body := map[string]any{"label": "test", "command": longCommand, "hidden": false}
	req := newRequest(t, http.MethodPost, "/api/chat/quick-send", body)
	w := callHandler(ServeChatQuickSend, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatQuickSend_CreateWhitespaceTrimmed(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]any{"label": "  继续  ", "command": "  继续  ", "hidden": false}
	req := newRequest(t, http.MethodPost, "/api/chat/quick-send", body)
	w := callHandler(ServeChatQuickSend, req)

	assertStatus(t, w, http.StatusCreated)

	// Verify trimmed values
	var result map[string]any
	decodeRespJSON(t, w.Body, &result)
	if result["label"] != "继续" {
		t.Errorf("expected trimmed label '继续', got %v", result["label"])
	}
	if result["command"] != "继续" {
		t.Errorf("expected trimmed command '继续', got %v", result["command"])
	}
}

// ---------- PUT /api/chat/quick-send/reorder ----------

func TestServeChatQuickSend_Reorder(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	service.AddChatQuickSend("A", "a", false) // id=1, sort=0
	service.AddChatQuickSend("B", "b", false) // id=2, sort=1

	body := map[string]any{"ids": []int64{2, 1}}
	req := newRequest(t, http.MethodPut, "/api/chat/quick-send/reorder", body)
	w := callHandler(ServeChatQuickSend, req)

	assertOK(t, w)

	var result map[string]any
	decodeRespJSON(t, w.Body, &result)
	if result["success"] != true {
		t.Error("expected success:true")
	}

	// Verify order
	items, _ := service.GetChatQuickSend()
	if len(items) != 2 || items[0].Label != "B" {
		t.Errorf("expected B first after reorder, got %v", items[0].Label)
	}
}

func TestServeChatQuickSend_ReorderEmpty(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]any{"ids": []int64{}}
	req := newRequest(t, http.MethodPut, "/api/chat/quick-send/reorder", body)
	w := callHandler(ServeChatQuickSend, req)

	assertOK(t, w)
}

// ---------- PUT /api/chat/quick-send/{id} ----------

func TestServeChatQuickSendByID_Update(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	service.AddChatQuickSend("继续", "继续", false)

	body := map[string]any{"label": "▶️ 继续", "command": "请继续", "hidden": true}
	req := newRequest(t, http.MethodPut, "/api/chat/quick-send/1", body)
	w := callHandler(ServeChatQuickSendByID, req)

	assertOK(t, w)

	var result map[string]any
	decodeRespJSON(t, w.Body, &result)
	if result["success"] != true {
		t.Error("expected success:true")
	}

	items, _ := service.GetChatQuickSend()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Label != "▶️ 继续" {
		t.Errorf("expected updated label, got %q", items[0].Label)
	}
	if items[0].Command != "请继续" {
		t.Errorf("expected updated command, got %q", items[0].Command)
	}
	if !items[0].Hidden {
		t.Error("expected hidden=true after update")
	}
}

func TestServeChatQuickSendByID_UpdateEmptyLabel(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	service.AddChatQuickSend("继续", "继续", false)

	body := map[string]any{"label": "", "command": "test", "hidden": false}
	req := newRequest(t, http.MethodPut, "/api/chat/quick-send/1", body)
	w := callHandler(ServeChatQuickSendByID, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatQuickSendByID_UpdateInvalidID(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]any{"label": "test", "command": "test", "hidden": false}
	req := newRequest(t, http.MethodPut, "/api/chat/quick-send/abc", body)
	w := callHandler(ServeChatQuickSendByID, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ---------- DELETE /api/chat/quick-send/{id} ----------

func TestServeChatQuickSendByID_Delete(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	service.AddChatQuickSend("继续", "继续", false)
	service.AddChatQuickSend("提交", "提交", false)

	req := newRequest(t, http.MethodDelete, "/api/chat/quick-send/1", nil)
	w := callHandler(ServeChatQuickSendByID, req)

	assertOK(t, w)

	items, _ := service.GetChatQuickSend()
	if len(items) != 1 {
		t.Fatalf("expected 1 item after delete, got %d", len(items))
	}
	if items[0].Label != "提交" {
		t.Errorf("expected remaining item '提交', got %q", items[0].Label)
	}
}

func TestServeChatQuickSendByID_DeleteInvalidID(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodDelete, "/api/chat/quick-send/notanumber", nil)
	w := callHandler(ServeChatQuickSendByID, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ---------- Method not allowed ----------

func TestServeChatQuickSend_MethodNotAllowed(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPatch, "/api/chat/quick-send", nil)
	w := callHandler(ServeChatQuickSend, req)

	assertStatus(t, w, http.StatusMethodNotAllowed)
}

func TestServeChatQuickSendByID_MethodNotAllowed(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/chat/quick-send/1", nil)
	w := callHandler(ServeChatQuickSendByID, req)

	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// ---------- Auth required ----------

func TestChatQuickSendRouteRequiresAuth(t *testing.T) {
	origToken := model.SessionToken
	t.Cleanup(func() { model.SessionToken = origToken })

	model.SessionToken = "test-token"

	mux := http.NewServeMux()
	RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/quick-send", nil)
	req.RemoteAddr = "203.0.113.10:12345" // Non-localhost
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected chat quick-send to require auth, got status %d body %s", w.Code, w.Body.String())
	}
}

// ---------- Reorder sub-path routing ----------

func TestServeChatQuickSendByID_ReorderForwardedToServeChatQuickSend(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	service.AddChatQuickSend("A", "a", false)
	service.AddChatQuickSend("B", "b", false)

	// PUT /api/chat/quick-send/reorder should be handled by ServeChatQuickSendByID
	// which forwards to ServeChatQuickSend
	body := map[string]any{"ids": []int64{2, 1}}
	req := newRequest(t, http.MethodPut, "/api/chat/quick-send/reorder", body)
	w := callHandler(ServeChatQuickSendByID, req)

	assertOK(t, w)

	var result map[string]any
	decodeRespJSON(t, w.Body, &result)
	if result["success"] != true {
		t.Error("expected reorder to succeed via forwarded handler")
	}
}

// ---------- JSON null handling ----------

func TestServeChatQuickSend_ListNullToEmptyArray(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// No items — GetChatQuickSend returns nil
	req := newRequest(t, http.MethodGet, "/api/chat/quick-send", nil)
	w := callHandler(ServeChatQuickSend, req)

	assertOK(t, w)

	// Verify we get [] not null
	var result []any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON array, got parse error: %v; body: %s", err, w.Body.String())
	}
}

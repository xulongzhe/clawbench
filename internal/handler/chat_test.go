package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"clawbench/internal/ai"
	"clawbench/internal/model"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
)

// feedEvents processes a sequence of StreamEvents through accumulateBlock
// and returns the resulting blocks.
func feedEvents(events []ai.StreamEvent) []model.ContentBlock {
	var blocks []model.ContentBlock
	var currentText strings.Builder
	for _, event := range events {
		accumulateBlock(&blocks, &currentText, event)
	}
	// Flush remaining text (matches handler behavior)
	if currentText.Len() > 0 {
		blocks = append(blocks, model.ContentBlock{Type: "text", Text: currentText.String()})
	}
	return blocks
}

func TestAccumulateBlock_TextOnly(t *testing.T) {
	events := []ai.StreamEvent{
		{Type: "content", Content: "Hello "},
		{Type: "content", Content: "world"},
	}

	blocks := feedEvents(events)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" {
		t.Errorf("expected text block, got %q", blocks[0].Type)
	}
	if blocks[0].Text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", blocks[0].Text)
	}
}

func TestAccumulateBlock_ThinkingCoalescing(t *testing.T) {
	events := []ai.StreamEvent{
		{Type: "thinking", Content: "Let me think..."},
		{Type: "thinking", Content: " about this."},
	}

	blocks := feedEvents(events)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 thinking block (coalesced), got %d", len(blocks))
	}
	if blocks[0].Type != "thinking" {
		t.Errorf("expected thinking block, got %q", blocks[0].Type)
	}
	if blocks[0].Text != "Let me think... about this." {
		t.Errorf("expected coalesced thinking text, got %q", blocks[0].Text)
	}
}

func TestAccumulateBlock_ThinkingBreaksOnText(t *testing.T) {
	// Text between thinking deltas should create separate blocks
	events := []ai.StreamEvent{
		{Type: "thinking", Content: "First thought"},
		{Type: "content", Content: "Some text"},
		{Type: "thinking", Content: "Second thought"},
	}

	blocks := feedEvents(events)

	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks (thinking, text, thinking), got %d", len(blocks))
	}
	if blocks[0].Type != "thinking" || blocks[0].Text != "First thought" {
		t.Errorf("expected first thinking block, got %+v", blocks[0])
	}
	if blocks[1].Type != "text" || blocks[1].Text != "Some text" {
		t.Errorf("expected text block, got %+v", blocks[1])
	}
	if blocks[2].Type != "thinking" || blocks[2].Text != "Second thought" {
		t.Errorf("expected second thinking block, got %+v", blocks[2])
	}
}

func TestAccumulateBlock_ToolUseDedup(t *testing.T) {
	// Two tool_use events for the same tool ID should produce one block
	events := []ai.StreamEvent{
		{Type: "tool_use", Tool: &ai.ToolCall{Name: "Read", ID: "t1", Input: "", Done: false}},
		{Type: "tool_use", Tool: &ai.ToolCall{Name: "Read", ID: "t1", Input: `{"file_path":"/a.go"}`, Done: true}},
	}

	blocks := feedEvents(events)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 tool_use block (deduped), got %d", len(blocks))
	}
	if blocks[0].Type != "tool_use" {
		t.Errorf("expected tool_use block, got %q", blocks[0].Type)
	}
	if blocks[0].Name != "Read" {
		t.Errorf("expected tool name 'Read', got %q", blocks[0].Name)
	}
	if blocks[0].ID != "t1" {
		t.Errorf("expected tool ID 't1', got %q", blocks[0].ID)
	}
	// Input should be updated to the final value
	if fp, ok := blocks[0].Input["file_path"]; !ok || fp != "/a.go" {
		t.Errorf("expected input file_path '/a.go', got %v", blocks[0].Input)
	}
}

func TestAccumulateBlock_ToolUseDifferentIDs(t *testing.T) {
	events := []ai.StreamEvent{
		{Type: "tool_use", Tool: &ai.ToolCall{Name: "Read", ID: "t1", Input: `{"file_path":"/a.go"}`, Done: true}},
		{Type: "tool_use", Tool: &ai.ToolCall{Name: "Bash", ID: "t2", Input: `{"command":"ls"}`, Done: true}},
	}

	blocks := feedEvents(events)

	if len(blocks) != 2 {
		t.Fatalf("expected 2 tool_use blocks, got %d", len(blocks))
	}
	if blocks[0].Name != "Read" {
		t.Errorf("expected first tool 'Read', got %q", blocks[0].Name)
	}
	if blocks[1].Name != "Bash" {
		t.Errorf("expected second tool 'Bash', got %q", blocks[1].Name)
	}
}

func TestAccumulateBlock_ToolUseEmptyInput(t *testing.T) {
	// Tool use with empty input should have an empty map, not nil
	events := []ai.StreamEvent{
		{Type: "tool_use", Tool: &ai.ToolCall{Name: "Read", ID: "t1", Input: "", Done: false}},
	}

	blocks := feedEvents(events)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Input == nil {
		t.Error("expected Input to be empty map, not nil")
	}
	if len(blocks[0].Input) != 0 {
		t.Errorf("expected empty map, got %v", blocks[0].Input)
	}
}

func TestAccumulateBlock_MixedFlow(t *testing.T) {
	// Full flow: thinking → tool_use → text → tool_use → text
	events := []ai.StreamEvent{
		{Type: "thinking", Content: "I need to read the file"},
		{Type: "tool_use", Tool: &ai.ToolCall{Name: "Read", ID: "t1", Input: "", Done: false}},
		{Type: "tool_use", Tool: &ai.ToolCall{Name: "Read", ID: "t1", Input: `{"file_path":"/src/main.go"}`, Done: true}},
		{Type: "content", Content: "I can see "},
		{Type: "content", Content: "the code."},
		{Type: "tool_use", Tool: &ai.ToolCall{Name: "Edit", ID: "t2", Input: `{"file_path":"/src/main.go","old":"foo","new":"bar"}`, Done: true}},
		{Type: "content", Content: " Done!"},
	}

	blocks := feedEvents(events)

	if len(blocks) != 5 {
		t.Fatalf("expected 5 blocks (thinking, tool, text, tool, text), got %d: %+v", len(blocks), blocks)
	}

	// Block 0: thinking
	if blocks[0].Type != "thinking" || blocks[0].Text != "I need to read the file" {
		t.Errorf("block 0: expected thinking, got %+v", blocks[0])
	}

	// Block 1: tool_use (Read) — deduped
	if blocks[1].Type != "tool_use" || blocks[1].Name != "Read" {
		t.Errorf("block 1: expected Read tool_use, got %+v", blocks[1])
	}
	if fp, ok := blocks[1].Input["file_path"]; !ok || fp != "/src/main.go" {
		t.Errorf("block 1: expected file_path '/src/main.go', got %v", blocks[1].Input)
	}

	// Block 2: text
	if blocks[2].Type != "text" || blocks[2].Text != "I can see the code." {
		t.Errorf("block 2: expected text, got %+v", blocks[2])
	}

	// Block 3: tool_use (Edit)
	if blocks[3].Type != "tool_use" || blocks[3].Name != "Edit" {
		t.Errorf("block 3: expected Edit tool_use, got %+v", blocks[3])
	}

	// Block 4: text
	if blocks[4].Type != "text" || blocks[4].Text != " Done!" {
		t.Errorf("block 4: expected text, got %+v", blocks[4])
	}
}

func TestAccumulateBlock_EmptyEvents(t *testing.T) {
	// No events → no blocks
	blocks := feedEvents(nil)
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(blocks))
	}
}

func TestAccumulateBlock_MetadataIgnored(t *testing.T) {
	// Metadata events should not produce blocks
	events := []ai.StreamEvent{
		{Type: "content", Content: "Hello"},
		{Type: "metadata", Meta: &ai.Metadata{Model: "test"}},
	}

	blocks := feedEvents(events)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block (metadata ignored), got %d", len(blocks))
	}
	if blocks[0].Type != "text" {
		t.Errorf("expected text block, got %q", blocks[0].Type)
	}
}

func TestAccumulateBlock_TextFlushedBeforeThinking(t *testing.T) {
	// Text should be flushed to a block when thinking arrives
	events := []ai.StreamEvent{
		{Type: "content", Content: "Some text"},
		{Type: "thinking", Content: "A thought"},
	}

	blocks := feedEvents(events)

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text != "Some text" {
		t.Errorf("block 0: expected flushed text, got %+v", blocks[0])
	}
	if blocks[1].Type != "thinking" || blocks[1].Text != "A thought" {
		t.Errorf("block 1: expected thinking, got %+v", blocks[1])
	}
}

func TestAccumulateBlock_TextFlushedBeforeToolUse(t *testing.T) {
	// Text should be flushed to a block when tool_use arrives
	events := []ai.StreamEvent{
		{Type: "content", Content: "Checking..."},
		{Type: "tool_use", Tool: &ai.ToolCall{Name: "Read", ID: "t1", Input: `{"file_path":"/x"}`, Done: true}},
	}

	blocks := feedEvents(events)

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text != "Checking..." {
		t.Errorf("block 0: expected flushed text, got %+v", blocks[0])
	}
	if blocks[1].Type != "tool_use" {
		t.Errorf("block 1: expected tool_use, got %q", blocks[1].Type)
	}
}

func TestBlocksSerialization(t *testing.T) {
	// Verify that blocks can be serialized to JSON and deserialized correctly
	blocks := []model.ContentBlock{
		{Type: "thinking", Text: "Analyzing..."},
		{Type: "tool_use", Name: "Read", ID: "t1", Input: map[string]any{"file_path": "/src/main.go"}},
		{Type: "text", Text: "Here is the result."},
	}

	data, err := json.Marshal(map[string]any{"blocks": blocks})
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Deserialize and verify
	var result struct {
		Blocks []model.ContentBlock `json:"blocks"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(result.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(result.Blocks))
	}

	if result.Blocks[0].Type != "thinking" || result.Blocks[0].Text != "Analyzing..." {
		t.Errorf("block 0 mismatch: %+v", result.Blocks[0])
	}
	if result.Blocks[1].Type != "tool_use" || result.Blocks[1].Name != "Read" {
		t.Errorf("block 1 mismatch: %+v", result.Blocks[1])
	}
	if result.Blocks[1].Input["file_path"] != "/src/main.go" {
		t.Errorf("block 1 input mismatch: %v", result.Blocks[1].Input)
	}
	if result.Blocks[2].Type != "text" || result.Blocks[2].Text != "Here is the result." {
		t.Errorf("block 2 mismatch: %+v", result.Blocks[2])
	}
}

func TestBlocksSerialization_RoundTrip(t *testing.T) {
	// Verify blocks survive a full serialize → DB store → deserialize cycle
	original := []model.ContentBlock{
		{Type: "thinking", Text: "Deep thought"},
		{Type: "tool_use", Name: "Bash", ID: "toolu_1", Input: map[string]any{"command": "ls -la"}},
		{Type: "text", Text: "Result here."},
	}

	// Serialize (as handler does for DB storage)
	data, _ := json.Marshal(map[string]any{"blocks": original})
	content := string(data)

	// Deserialize (as frontend does when loading from DB)
	var parsed struct {
		Blocks []model.ContentBlock `json:"blocks"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}

	if len(parsed.Blocks) != 3 {
		t.Fatalf("expected 3 blocks after round-trip, got %d", len(parsed.Blocks))
	}

	// Verify thinking
	if parsed.Blocks[0].Type != "thinking" || parsed.Blocks[0].Text != "Deep thought" {
		t.Errorf("thinking block lost in round-trip: %+v", parsed.Blocks[0])
	}

	// Verify tool_use
	if parsed.Blocks[1].Type != "tool_use" || parsed.Blocks[1].Name != "Bash" {
		t.Errorf("tool_use block lost in round-trip: %+v", parsed.Blocks[1])
	}
	if parsed.Blocks[1].Input["command"] != "ls -la" {
		t.Errorf("tool input lost in round-trip: %v", parsed.Blocks[1].Input)
	}

	// Verify text
	if parsed.Blocks[2].Type != "text" || parsed.Blocks[2].Text != "Result here." {
		t.Errorf("text block lost in round-trip: %+v", parsed.Blocks[2])
	}
}

// ============================================================================
// HTTP-level handler tests
// ============================================================================

// --- ServeChatHistory ---

func TestServeChatHistory_Get_NoSessions(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/ai/history", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeChatHistory, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.NotNil(t, result["sessionId"])
	assert.NotNil(t, result["messages"])
}

func TestServeChatHistory_Get_WithExistingSession(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create a session first
	sessionID, err := service.CreateSession(env.ProjectDir, "codebuddy", "test session", "", "")
	assert.NoError(t, err)

	// Add a message to that session
	_, err = service.AddChatMessage(env.ProjectDir, "codebuddy", sessionID, "user", "hello", "", nil, false)
	assert.NoError(t, err)

	req := newRequest(t, http.MethodGet, "/api/ai/history", nil)
	withProjectCookie(req, env.ProjectDir)
	withSessionCookie(req, sessionID)

	w := callHandler(ServeChatHistory, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, sessionID, result["sessionId"])

	messages, ok := result["messages"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, messages, 1)
}

func TestServeChatHistory_Post_AddMessage(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create a session first
	sessionID, err := service.CreateSession(env.ProjectDir, "codebuddy", "test session", "", "")
	assert.NoError(t, err)

	body := map[string]string{
		"role":       "user",
		"content":    "Hello AI",
		"session_id": sessionID,
	}
	req := newRequest(t, http.MethodPost, "/api/ai/history", body)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeChatHistory, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["ok"])
	assert.NotNil(t, result["savedAt"])
}

func TestServeChatHistory_Post_InvalidRole(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]string{
		"role":    "admin",
		"content": "Hello",
	}
	req := newRequest(t, http.MethodPost, "/api/ai/history", body)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeChatHistory, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatHistory_Post_InvalidBody(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Send invalid JSON by using raw bytes
	req := httptest.NewRequest(http.MethodPost, "/api/ai/history", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeChatHistory, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeChatHistory_NoProjectCookie(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/ai/history", nil)
	// No project cookie set

	w := callHandler(ServeChatHistory, req)
	assertStatus(t, w, http.StatusForbidden)
}

// --- ServeSessions ---

func TestServeSessions_Get(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/ai/sessions", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeSessions, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.NotNil(t, result["sessions"])
}

func TestServeSessions_Get_WithExistingSessions(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create some sessions
	_, err := service.CreateSession(env.ProjectDir, "codebuddy", "session 1", "", "")
	assert.NoError(t, err)
	_, err = service.CreateSession(env.ProjectDir, "codebuddy", "session 2", "", "")
	assert.NoError(t, err)

	req := newRequest(t, http.MethodGet, "/api/ai/sessions", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeSessions, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	sessions, ok := result["sessions"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, sessions, 2)
}

func TestServeSessions_Post_CreateSession(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]string{}
	req := newRequest(t, http.MethodPost, "/api/ai/sessions", body)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeSessions, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["ok"])
	assert.NotNil(t, result["sessionId"])
	assert.Equal(t, "codebuddy", result["backend"])
}

func TestServeSessions_Post_CustomTitleAndBackend(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	body := map[string]string{
		"title":   "My Custom Session",
		"backend": "claude",
	}
	req := newRequest(t, http.MethodPost, "/api/ai/sessions", body)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeSessions, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["ok"])
	assert.NotNil(t, result["sessionId"])
	assert.Equal(t, "claude", result["backend"])

	// Verify session title in DB
	sessionID := result["sessionId"].(string)
	title, err := service.GetSessionTitle(sessionID)
	assert.NoError(t, err)
	assert.Equal(t, "My Custom Session", title)
}

func TestServeSessions_Post_InvalidBody(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/ai/sessions", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeSessions, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeSessions_NoProjectCookie(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/ai/sessions", nil)

	w := callHandler(ServeSessions, req)
	assertStatus(t, w, http.StatusForbidden)
}

// --- DeleteSession ---

func TestDeleteSession_ExistingSession(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID, err := service.CreateSession(env.ProjectDir, "codebuddy", "to delete", "", "")
	assert.NoError(t, err)

	req := newRequest(t, http.MethodDelete, "/api/ai/session/delete?session_id="+sessionID, nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(DeleteSession, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["ok"])
}

func TestDeleteSession_MissingSessionID(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodDelete, "/api/ai/session/delete", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(DeleteSession, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestDeleteSession_NoProjectCookie(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodDelete, "/api/ai/session/delete?session_id=abc", nil)

	w := callHandler(DeleteSession, req)
	assertStatus(t, w, http.StatusForbidden)
}

func TestDeleteSession_WrongMethod(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/ai/session/delete?session_id=abc", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(DeleteSession, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// --- CancelChat ---

func TestCancelChat_NoRunningSession(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/ai/chat/cancel?session_id=nonexistent", nil)

	w := callHandler(CancelChat, req)
	// Idempotent: cancelling a non-running session succeeds
	assertStatus(t, w, http.StatusOK)
}

func TestCancelChat_MissingSessionID(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/ai/chat/cancel", nil)
	// No session_id in query and no cookie

	w := callHandler(CancelChat, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCancelChat_WrongMethod(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/ai/chat/cancel?session_id=abc", nil)

	w := callHandler(CancelChat, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// --- ServeAISession ---

func TestServeAISession_DeleteNonExistentDir(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodDelete, "/api/ai/session", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeAISession, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["ok"])
	assert.Equal(t, float64(0), result["deleted"])
}

func TestServeAISession_WrongMethod(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/ai/session", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeAISession, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

func TestServeAISession_NoProjectCookie(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodDelete, "/api/ai/session", nil)

	w := callHandler(ServeAISession, req)
	assertStatus(t, w, http.StatusForbidden)
}

// --- ServeWatchDir ---

func TestServeWatchDir(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/watch-dir", nil)

	w := callHandler(ServeWatchDir, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result, "watchDir")
	watchDir, ok := result["watchDir"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, watchDir)
}

// --- UploadFile ---

func TestUploadFile_ValidFile(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.txt")
	assert.NoError(t, err)
	_, err = part.Write([]byte("hello world"))
	assert.NoError(t, err)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/file", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(UploadFile, req)
	assertOK(t, w)

	var result map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["ok"])

	path, ok := result["path"].(string)
	assert.True(t, ok)
	assert.Contains(t, path, ".clawbench"+string([]byte{filepath.Separator})+"uploads")
}

func TestUploadFile_NoProjectCookie(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("hello"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/file", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := callHandler(UploadFile, req)
	assertStatus(t, w, http.StatusForbidden)
}

func TestUploadFile_WrongMethod(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/upload/file", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(UploadFile, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

func TestUploadFile_DangerousExtension(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "malware.exe")
	assert.NoError(t, err)
	part.Write([]byte("evil content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/file", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(UploadFile, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestUploadFile_DangerousBatExtension(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "script.bat")
	part.Write([]byte("@echo off"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/file", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(UploadFile, req)
	assertStatus(t, w, http.StatusBadRequest)
}

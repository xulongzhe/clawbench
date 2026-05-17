package handler

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/service"
)

func setupTestEventBus(maxClients int) (cleanup func()) {
	model.WatchDir = "/tmp"
	origBus := service.GlobalEventBus
	service.GlobalEventBus = service.NewEventBus(maxClients)
	return func() { service.GlobalEventBus = origBus }
}

func newEventsRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.AddCookie(&http.Cookie{Name: "clawbench_project", Value: "/tmp"})
	return req
}

func TestSystemEventsSSE_ConnectAndReceive(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	req := newEventsRequest()
	w := httptest.NewRecorder()

	go SystemEventsSSE(w, req)
	time.Sleep(100 * time.Millisecond)

	service.GlobalEventBus.Publish(service.SystemEvent{
		Type:    "session_start",
		Payload: map[string]any{"sessionId": "s1"},
	})
	time.Sleep(100 * time.Millisecond)

	body := w.Body.String()

	if !strings.Contains(body, "event: connected") {
		t.Errorf("expected connected event, got:\n%s", body)
	}
	if !strings.Contains(body, "event: session_start") {
		t.Errorf("expected session_start event, got:\n%s", body)
	}
	if !strings.Contains(body, `"sessionId":"s1"`) {
		t.Errorf("expected sessionId=s1 in payload, got:\n%s", body)
	}
}

func TestSystemEventsSSE_MethodNotAllowed(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/events", nil)
	w := httptest.NewRecorder()

	SystemEventsSSE(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSystemEventsSSE_MaxClients(t *testing.T) {
	cleanup := setupTestEventBus(1)
	defer cleanup()

	service.GlobalEventBus.Subscribe("blocker")
	defer service.GlobalEventBus.Unsubscribe("blocker")

	req := newEventsRequest()
	w := httptest.NewRecorder()

	SystemEventsSSE(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 when max clients reached, got %d", w.Code)
	}
}

func TestSystemEventsSSE_ReferrerPolicyHeader(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	req := newEventsRequest()
	w := httptest.NewRecorder()

	go SystemEventsSSE(w, req)
	time.Sleep(100 * time.Millisecond)

	if v := w.Header().Get("Referrer-Policy"); v != "no-referrer" {
		t.Errorf("expected Referrer-Policy: no-referrer, got %q", v)
	}
}

func TestSystemEventsSSE_SSEHeaders(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	req := newEventsRequest()
	w := httptest.NewRecorder()

	go SystemEventsSSE(w, req)
	time.Sleep(100 * time.Millisecond)

	if v := w.Header().Get("Content-Type"); v != "text/event-stream" {
		t.Errorf("expected Content-Type: text/event-stream, got %q", v)
	}
	if v := w.Header().Get("Cache-Control"); v != "no-cache" {
		t.Errorf("expected Cache-Control: no-cache, got %q", v)
	}
	if v := w.Header().Get("Connection"); v != "keep-alive" {
		t.Errorf("expected Connection: keep-alive, got %q", v)
	}
}

func TestSystemEventsSSE_EventFormat(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	req := newEventsRequest()
	w := httptest.NewRecorder()

	go SystemEventsSSE(w, req)
	time.Sleep(50 * time.Millisecond)

	service.GlobalEventBus.Publish(service.SystemEvent{
		Type: "task_exec_update",
		Payload: map[string]any{
			"taskId": int64(42),
			"status": "completed",
		},
	})
	time.Sleep(50 * time.Millisecond)

	body := w.Body.String()

	if !strings.Contains(body, "event: task_exec_update\n") {
		t.Errorf("expected SSE event line, got:\n%s", body)
	}

	scanner := bufio.NewScanner(strings.NewReader(body))
	foundTaskEvent := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "event: task_exec_update" {
			foundTaskEvent = true
		}
		if foundTaskEvent && strings.HasPrefix(line, "data: ") {
			jsonStr := strings.TrimPrefix(line, "data: ")
			if !strings.Contains(jsonStr, `"type"`) || !strings.Contains(jsonStr, `"task_exec_update"`) {
				t.Errorf("task_exec_update data should contain type field, got: %s", jsonStr)
			}
			if !strings.Contains(jsonStr, `"status"`) {
				t.Errorf("task_exec_update data should contain status field, got: %s", jsonStr)
			}
			foundTaskEvent = false // only check the first data line after the event line
		}
	}
}

func TestSystemEventsSSE_ClientCleanupOnDisconnect(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	before := service.GlobalEventBus.ClientCount()

	req := newEventsRequest()
	w := httptest.NewRecorder()

	go SystemEventsSSE(w, req)
	time.Sleep(100 * time.Millisecond)

	afterConnect := service.GlobalEventBus.ClientCount()
	if afterConnect != before+1 {
		t.Errorf("expected %d clients after connect, got %d", before+1, afterConnect)
	}

	// Note: httptest.NewRecorder doesn't support context cancellation,
	// so we can't fully test disconnect cleanup here.
	// The defer Unsubscribe in the handler ensures cleanup on any exit path.
}

func TestSystemEventsSSE_MissingProjectCookie(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	// Request without project cookie
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()

	SystemEventsSSE(w, req)

	// requireProject should return 403 Forbidden
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for missing project cookie, got %d", w.Code)
	}
}

func TestSystemEventsSSE_DisconnectCleanupWithRealServer(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	// Use a real HTTP server to test context cancellation cleanup.
	// Register the raw handler (without Auth middleware) since we're testing
	// SSE lifecycle, not authentication.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/events", SystemEventsSSE)
	server := httptest.NewServer(mux)
	defer server.Close()

	before := service.GlobalEventBus.ClientCount()

	// Connect as a real SSE client with project cookie
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/events", nil)
	req.AddCookie(&http.Cookie{Name: "clawbench_project", Value: "/tmp"})

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to SSE: %v", err)
	}

	// Wait for connection to establish
	time.Sleep(200 * time.Millisecond)

	afterConnect := service.GlobalEventBus.ClientCount()
	if afterConnect != before+1 {
		t.Errorf("expected %d clients after connect, got %d", before+1, afterConnect)
	}

	// Close the connection — this should cancel the request context
	resp.Body.Close()

	// Wait for server to detect disconnect
	time.Sleep(300 * time.Millisecond)

	afterDisconnect := service.GlobalEventBus.ClientCount()
	if afterDisconnect != before {
		t.Errorf("expected %d clients after disconnect, got %d", before, afterDisconnect)
	}
}

func TestSystemEventsSSE_ChannelClosedOnUnsubscribe(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	// Subscribe a client, then unsubscribe to close the channel
	// The SSE handler should exit cleanly when the channel is closed
	ch, ok := service.GlobalEventBus.Subscribe("test-close-ch")
	if !ok {
		t.Fatal("subscribe should succeed")
	}

	// Unsubscribe closes the channel
	service.GlobalEventBus.Unsubscribe("test-close-ch")

	// Reading from a closed channel should return zero value with ok=false
	_, ok2 := <-ch
	if ok2 {
		t.Error("channel should be closed after Unsubscribe")
	}
}

func TestSystemEventsSSE_ConnectedEventContainsClientId(t *testing.T) {
	cleanup := setupTestEventBus(20)
	defer cleanup()

	req := newEventsRequest()
	w := httptest.NewRecorder()

	go SystemEventsSSE(w, req)
	time.Sleep(100 * time.Millisecond)

	body := w.Body.String()

	// The connected event should contain a clientId in the data
	if !strings.Contains(body, `"clientId"`) {
		t.Errorf("expected connected event to contain clientId, got:\n%s", body)
	}

	// The clientId should be in UUID-like format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, "clientId") {
			// Should contain a dash-formatted hex ID
			if !strings.Contains(line, "-") {
				t.Errorf("clientId should be in UUID-like format, got: %s", line)
			}
			break
		}
	}
}

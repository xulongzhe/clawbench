package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"clawbench/internal/middleware"
	"clawbench/internal/model"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
)

// setupAuthTestEnv creates an isolated EventBus + saves/restores SessionToken.
// Returns a cleanup function.
func setupAuthTestEnv(maxClients int) func() {
	origBus := service.GlobalEventBus
	origToken := model.SessionToken
	origWatch := model.WatchDir
	model.WatchDir = "/tmp"
	service.GlobalEventBus = service.NewEventBus(maxClients)
	return func() {
		service.GlobalEventBus = origBus
		model.SessionToken = origToken
		model.WatchDir = origWatch
	}
}

// TestSystemEventsSSEAuth_NoPassword_RemoteAccess verifies that when no password
// is configured, remote requests can connect to the SSE endpoint without auth.
func TestSystemEventsSSEAuth_NoPassword_RemoteAccess(t *testing.T) {
	cleanup := setupAuthTestEnv(20)
	defer cleanup()

	model.SessionToken = ""

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	withProjectCookie(req, "/tmp")

	w := httptest.NewRecorder()
	go middleware.Auth(SystemEventsSSE)(w, req)
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
}

// TestSystemEventsSSEAuth_ValidTokenQueryParam verifies that a remote request
// with a valid ?token= query parameter can connect to the SSE endpoint.
func TestSystemEventsSSEAuth_ValidTokenQueryParam(t *testing.T) {
	cleanup := setupAuthTestEnv(20)
	defer cleanup()

	model.SessionToken = "test-secret-token"

	req := httptest.NewRequest(http.MethodGet, "/api/events?token=test-secret-token", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	withProjectCookie(req, "/tmp")

	w := httptest.NewRecorder()
	go middleware.Auth(SystemEventsSSE)(w, req)
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
}

// TestSystemEventsSSEAuth_InvalidTokenQueryParam_401 verifies that a remote request
// with an invalid ?token= query parameter gets rejected with 401.
func TestSystemEventsSSEAuth_InvalidTokenQueryParam_401(t *testing.T) {
	cleanup := setupAuthTestEnv(20)
	defer cleanup()

	model.SessionToken = "test-secret-token"

	req := httptest.NewRequest(http.MethodGet, "/api/events?token=wrong-token", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	withProjectCookie(req, "/tmp")

	w := httptest.NewRecorder()
	middleware.Auth(SystemEventsSSE)(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestSystemEventsSSEAuth_ValidCookie verifies that a remote request with a valid
// session cookie can connect to the SSE endpoint.
func TestSystemEventsSSEAuth_ValidCookie(t *testing.T) {
	cleanup := setupAuthTestEnv(20)
	defer cleanup()

	model.SessionToken = "test-secret-token"

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	withProjectCookie(req, "/tmp")
	withAuthCookie(req, "test-secret-token")

	w := httptest.NewRecorder()
	go middleware.Auth(SystemEventsSSE)(w, req)
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
}

// TestSystemEventsSSEAuth_NoAuth_Remote401 verifies that a remote request without
// any authentication gets rejected with 401 when a password is configured.
func TestSystemEventsSSEAuth_NoAuth_Remote401(t *testing.T) {
	cleanup := setupAuthTestEnv(20)
	defer cleanup()

	model.SessionToken = "test-secret-token"

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	withProjectCookie(req, "/tmp")

	w := httptest.NewRecorder()
	middleware.Auth(SystemEventsSSE)(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestSystemEventsSSEAuth_LocalhostBypass verifies that localhost requests
// bypass auth even when a password is configured (CLI subcommands).
func TestSystemEventsSSEAuth_LocalhostBypass(t *testing.T) {
	cleanup := setupAuthTestEnv(20)
	defer cleanup()

	model.SessionToken = "test-secret-token"

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	withProjectCookie(req, "/tmp")

	w := httptest.NewRecorder()
	go middleware.Auth(SystemEventsSSE)(w, req)
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
}

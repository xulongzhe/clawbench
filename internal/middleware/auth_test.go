package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"clawbench/internal/middleware"
	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

// okHandler is an always-200 handler used as the "next" in middleware chains.
func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// withSavedToken saves model.SessionToken, runs f, then restores it.
func withSavedToken(f func()) {
	orig := model.SessionToken
	defer func() { model.SessionToken = orig }()
	f()
}

// --- Auth: no password configured ---

func TestAuth_NoPassword_PassThrough(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = ""

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

// --- Auth: localhost bypass ---

func TestAuth_Localhost_IPv4_BypassesAuth(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestAuth_Localhost_IPv6_BypassesAuth(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "[::1]:12345"

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

// --- Auth: remote with valid cookie ---

func TestAuth_ValidCookie_PassThrough(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		req.AddCookie(&http.Cookie{
			Name:  model.SessionCookie,
			Value: "valid-token",
		})

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

// --- Auth: remote with invalid/missing cookie ---

func TestAuth_InvalidCookieValue_Returns401(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		req.AddCookie(&http.Cookie{
			Name:  model.SessionCookie,
			Value: "wrong-token",
		})

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAuth_MissingCookie_Returns401(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.100:12345"

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// --- Auth: localhost + bad cookie still passes (localhost wins) ---

func TestAuth_LocalhostWithBadCookie_StillPasses(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.AddCookie(&http.Cookie{
			Name:  model.SessionCookie,
			Value: "wrong-token",
		})

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

// --- GetProjectFromCookie ---

func TestGetProjectFromCookie_NormalExtraction(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "clawbench_project",
		Value: "/home/user/myproject",
	})

	result := middleware.GetProjectFromCookie(req)
	assert.Equal(t, "/home/user/myproject", result)
}

func TestGetProjectFromCookie_URLEncodedValueDecoded(t *testing.T) {
	encoded := url.QueryEscape("/home/user/my project")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "clawbench_project",
		Value: encoded,
	})

	result := middleware.GetProjectFromCookie(req)
	assert.Equal(t, "/home/user/my project", result)
}

func TestGetProjectFromCookie_NoCookie_ReturnsEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	result := middleware.GetProjectFromCookie(req)
	assert.Equal(t, "", result)
}

func TestGetProjectFromCookie_EmptyValue_ReturnsEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "clawbench_project",
		Value: "",
	})

	result := middleware.GetProjectFromCookie(req)
	assert.Equal(t, "", result)
}

// --- Auth: ?token= query parameter ---

func TestAuth_ValidTokenQueryParam_PassThrough(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/events?token=valid-token", nil)
		req.RemoteAddr = "192.168.1.100:12345"

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestAuth_InvalidTokenQueryParam_Returns401(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/events?token=wrong-token", nil)
		req.RemoteAddr = "192.168.1.100:12345"

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAuth_EmptyTokenQueryParam_Returns401(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/events?token=", nil)
		req.RemoteAddr = "192.168.1.100:12345"

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAuth_CookieAndTokenBothValid_PassThrough(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/events?token=valid-token", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		req.AddCookie(&http.Cookie{
			Name:  model.SessionCookie,
			Value: "valid-token",
		})

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestAuth_CookieInvalidButTokenValid_PassThrough(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/events?token=valid-token", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		req.AddCookie(&http.Cookie{
			Name:  model.SessionCookie,
			Value: "wrong-cookie-token",
		})

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestAuth_CookieValidButTokenInvalid_PassThrough(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/events?token=wrong-token", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		req.AddCookie(&http.Cookie{
			Name:  model.SessionCookie,
			Value: "valid-token",
		})

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestAuth_NoTokenQueryParam_NoCookie_Returns401(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
		req.RemoteAddr = "192.168.1.100:12345"

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAuth_LocalhostWithTokenQueryParam_PassThrough(t *testing.T) {
	withSavedToken(func() {
		model.SessionToken = "valid-token"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/events?token=valid-token", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		middleware.Auth(okHandler).ServeHTTP(rec, req)

		// Localhost always passes regardless of token
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

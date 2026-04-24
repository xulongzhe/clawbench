package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"clawbench/internal/middleware"

	"github.com/stretchr/testify/assert"
)

func TestRecoverPanic_NoPanic_PassThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	middleware.RecoverPanic(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRecoverPanic_PanicWithString_Returns500(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	panicHandler := func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}

	middleware.RecoverPanic(panicHandler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := strings.TrimSpace(rec.Body.String())
	assert.Contains(t, body, `"error"`)
	assert.Contains(t, body, `"internal server error"`)
}

func TestRecoverPanic_PanicWithError_Returns500(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	panicHandler := func(w http.ResponseWriter, r *http.Request) {
		panic(errors.New("explicit error object"))
	}

	middleware.RecoverPanic(panicHandler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := strings.TrimSpace(rec.Body.String())
	assert.Contains(t, body, `{"error":"internal server error"}`)
}

func TestRecoverPanic_ContentTypeIsJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	panicHandler := func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}

	middleware.RecoverPanic(panicHandler).ServeHTTP(rec, req)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestRecoverPanic_ResponseBodyContainsError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	panicHandler := func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected failure")
	}

	middleware.RecoverPanic(panicHandler).ServeHTTP(rec, req)

	body := strings.TrimSpace(rec.Body.String())
	assert.Contains(t, body, `{"error":"internal server error"}`)
}

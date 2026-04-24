package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"clawbench/internal/middleware"

	"github.com/stretchr/testify/assert"
)

func TestWithRequestID_HeaderIsSet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	middleware.WithRequestID(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).ServeHTTP(rec, req)

	requestID := rec.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID, "X-Request-ID header should be set")
}

func TestWithRequestID_GetRequestID_ExtractsFromContext(t *testing.T) {
	var extractedID string

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	middleware.WithRequestID(func(w http.ResponseWriter, r *http.Request) {
		extractedID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}).ServeHTTP(rec, req)

	headerID := rec.Header().Get("X-Request-ID")
	assert.Equal(t, headerID, extractedID, "GetRequestID should return the same ID as the header")
}

func TestGetRequestID_NilContext_ReturnsEmpty(t *testing.T) {
	result := middleware.GetRequestID(context.Background())
	assert.Equal(t, "", result, "GetRequestID with empty context should return empty string")
}

func TestWithRequestID_UniqueIDs(t *testing.T) {
	ids := make(map[string]bool)

	for i := 0; i < 100; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		middleware.WithRequestID(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}).ServeHTTP(rec, req)

		id := rec.Header().Get("X-Request-ID")
		assert.NotEmpty(t, id)
		ids[id] = true
	}

	// All 100 IDs should be unique
	assert.Equal(t, 100, len(ids), "each request should get a unique ID")
}

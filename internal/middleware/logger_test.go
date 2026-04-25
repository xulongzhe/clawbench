package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestLogger(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})

	wrapped := RequestLogger(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	wrapped(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())
}

func TestRequestLogger_CapturesStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	wrapped := RequestLogger(handler)

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	wrapped(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRequestLogger_CapturesBytesWritten(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("some response data"))
	})

	wrapped := RequestLogger(handler)

	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	rec := httptest.NewRecorder()

	wrapped(rec, req)

	assert.Equal(t, "some response data", rec.Body.String())
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &ResponseWriter{ResponseWriter: rec, status: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, rw.status)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestResponseWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &ResponseWriter{ResponseWriter: rec, status: http.StatusOK}

	n, err := rw.Write([]byte("test data"))
	assert.NoError(t, err)
	assert.Equal(t, 9, n)
	assert.Equal(t, 9, rw.bytes)
	assert.Equal(t, "test data", rec.Body.String())
}

func TestResponseWriter_WriteAccumulates(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &ResponseWriter{ResponseWriter: rec, status: http.StatusOK}

	rw.Write([]byte("hello "))
	rw.Write([]byte("world"))
	assert.Equal(t, 11, rw.bytes) // "hello " (6) + "world" (5)
}

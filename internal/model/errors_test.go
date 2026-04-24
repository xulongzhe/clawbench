package model_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestSentinelErrors(t *testing.T) {
	sentinelTests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"ErrUnauthorized", model.ErrUnauthorized, "unauthorized"},
		{"ErrForbidden", model.ErrForbidden, "access denied"},
		{"ErrNotFound", model.ErrNotFound, "not found"},
		{"ErrBadRequest", model.ErrBadRequest, "bad request"},
		{"ErrInternal", model.ErrInternal, "internal server error"},
		{"ErrProjectNotSet", model.ErrProjectNotSet, "no project selected"},
		{"ErrInvalidPath", model.ErrInvalidPath, "invalid path"},
		{"ErrPathTraversal", model.ErrPathTraversal, "path traversal detected"},
	}

	for _, tt := range sentinelTests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantMsg, tt.err.Error())
		})
	}
}

func TestAppError_Error_WithoutInner(t *testing.T) {
	err := &model.AppError{Code: 400, Message: "bad input"}
	assert.Equal(t, "bad input", err.Error())
}

func TestAppError_Error_WithInner(t *testing.T) {
	inner := errors.New("connection refused")
	err := &model.AppError{Code: 500, Message: "internal server error", Err: inner}
	assert.Equal(t, "internal server error: connection refused", err.Error())
}

func TestAppError_Unwrap(t *testing.T) {
	t.Run("with inner error", func(t *testing.T) {
		inner := errors.New("root cause")
		err := &model.AppError{Code: 500, Message: "fail", Err: inner}
		assert.Equal(t, inner, err.Unwrap())
	})

	t.Run("without inner error", func(t *testing.T) {
		err := &model.AppError{Code: 400, Message: "bad"}
		assert.Nil(t, err.Unwrap())
	})
}

func TestAppError_Unwrap_WithErrorsIs(t *testing.T) {
	inner := model.ErrNotFound
	err := &model.AppError{Code: 404, Message: "resource missing", Err: inner}
	assert.True(t, errors.Is(err, model.ErrNotFound), "errors.Is should find the inner sentinel")
}

func TestNewAppError(t *testing.T) {
	inner := errors.New("db down")
	err := model.NewAppError(http.StatusServiceUnavailable, "service unavailable", inner)
	assert.Equal(t, http.StatusServiceUnavailable, err.Code)
	assert.Equal(t, "service unavailable", err.Message)
	assert.Equal(t, inner, err.Err)
}

func TestNewAppErrorf(t *testing.T) {
	t.Run("formatted message", func(t *testing.T) {
		inner := errors.New("timeout")
		err := model.NewAppErrorf(http.StatusBadRequest, "field %q is required", inner, "email")
		assert.Equal(t, http.StatusBadRequest, err.Code)
		assert.Equal(t, `field "email" is required`, err.Message)
		assert.Equal(t, inner, err.Err)
	})

	t.Run("no format args", func(t *testing.T) {
		err := model.NewAppErrorf(http.StatusBadRequest, "simple message", nil)
		assert.Equal(t, "simple message", err.Message)
		assert.Nil(t, err.Err)
	})

	t.Run("multiple format args", func(t *testing.T) {
		err := model.NewAppErrorf(http.StatusBadRequest, "got %d errors in %s", nil, 3, "module")
		assert.Equal(t, "got 3 errors in module", err.Message)
	})
}

func TestBadRequest(t *testing.T) {
	inner := errors.New("invalid json")
	err := model.BadRequest(inner, "invalid request body")
	assert.Equal(t, http.StatusBadRequest, err.Code)
	assert.Equal(t, "invalid request body", err.Message)
	assert.Equal(t, inner, err.Err)
}

func TestForbidden(t *testing.T) {
	inner := errors.New("insufficient permissions")
	err := model.Forbidden(inner, "access denied to resource")
	assert.Equal(t, http.StatusForbidden, err.Code)
	assert.Equal(t, "access denied to resource", err.Message)
	assert.Equal(t, inner, err.Err)
}

func TestNotFound(t *testing.T) {
	inner := errors.New("file missing")
	err := model.NotFound(inner, "file not found")
	assert.Equal(t, http.StatusNotFound, err.Code)
	assert.Equal(t, "file not found", err.Message)
	assert.Equal(t, inner, err.Err)
}

func TestInternal(t *testing.T) {
	inner := errors.New("database connection failed")
	err := model.Internal(inner)
	assert.Equal(t, http.StatusInternalServerError, err.Code)
	assert.Equal(t, "internal server error", err.Message)
	assert.Equal(t, inner, err.Err)
}

func TestInternal_NilErr(t *testing.T) {
	err := model.Internal(nil)
	assert.Equal(t, http.StatusInternalServerError, err.Code)
	assert.Equal(t, "internal server error", err.Message)
	assert.Nil(t, err.Err)
}

func TestUnauthorized(t *testing.T) {
	inner := errors.New("expired token")
	err := model.Unauthorized(inner)
	assert.Equal(t, http.StatusUnauthorized, err.Code)
	assert.Equal(t, "unauthorized", err.Message)
	assert.Equal(t, inner, err.Err)
}

func TestUnauthorized_NilErr(t *testing.T) {
	err := model.Unauthorized(nil)
	assert.Equal(t, http.StatusUnauthorized, err.Code)
	assert.Equal(t, "unauthorized", err.Message)
	assert.Nil(t, err.Err)
}

func TestWriteError_WithAppError(t *testing.T) {
	appErr := model.BadRequest(errors.New("missing field"), "validation failed")
	rec := httptest.NewRecorder()
	model.WriteError(rec, appErr)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp model.ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Equal(t, "validation failed", resp.Error)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestWriteError_WithAppError_VariousStatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		appErr   *model.AppError
		wantCode int
		wantMsg  string
	}{
		{"404", model.NotFound(nil, "not here"), http.StatusNotFound, "not here"},
		{"403", model.Forbidden(nil, "no access"), http.StatusForbidden, "no access"},
		{"401", model.Unauthorized(nil), http.StatusUnauthorized, "unauthorized"},
		{"500", model.Internal(nil), http.StatusInternalServerError, "internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			model.WriteError(rec, tt.appErr)

			assert.Equal(t, tt.wantCode, rec.Code)

			var resp model.ErrorResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantMsg, resp.Error)
			assert.Equal(t, tt.wantCode, resp.Code)
		})
	}
}

func TestWriteError_WithNonAppError(t *testing.T) {
	plainErr := errors.New("something broke")
	rec := httptest.NewRecorder()
	model.WriteError(rec, plainErr)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp model.ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Equal(t, "Internal server error", resp.Error)
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestWriteError_WithSentinelError(t *testing.T) {
	rec := httptest.NewRecorder()
	model.WriteError(rec, model.ErrNotFound)

	// Sentinel errors are not AppError, so should fall back to 500
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp model.ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Equal(t, "Internal server error", resp.Error)
}

func TestWriteErrorf(t *testing.T) {
	rec := httptest.NewRecorder()
	model.WriteErrorf(rec, http.StatusBadGateway, "upstream unavailable")

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp model.ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Equal(t, "upstream unavailable", resp.Error)
	assert.Equal(t, http.StatusBadGateway, resp.Code)
}

func TestWriteErrorf_CustomStatusAndMessage(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		msg      string
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests, "rate limit exceeded"},
		{"503 Service Unavailable", http.StatusServiceUnavailable, "try again later"},
		{"422 Unprocessable", http.StatusUnprocessableEntity, "validation failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			model.WriteErrorf(rec, tt.status, tt.msg)

			assert.Equal(t, tt.status, rec.Code)

			var resp model.ErrorResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			assert.NoError(t, err)
			assert.Equal(t, tt.msg, resp.Error)
			assert.Equal(t, tt.status, resp.Code)
		})
	}
}

func TestErrorResponse_JSONMarshaling(t *testing.T) {
	t.Run("with code", func(t *testing.T) {
		resp := model.ErrorResponse{Error: "not found", Code: 404}
		data, err := json.Marshal(resp)
		assert.NoError(t, err)
		assert.Contains(t, string(data), `"error":"not found"`)
		assert.Contains(t, string(data), `"code":404`)
	})

	t.Run("without code (omitempty)", func(t *testing.T) {
		resp := model.ErrorResponse{Error: "something went wrong"}
		data, err := json.Marshal(resp)
		assert.NoError(t, err)
		assert.Contains(t, string(data), `"error":"something went wrong"`)
		assert.NotContains(t, string(data), `"code"`)
	})
}

func TestAppError_ImplementsError(t *testing.T) {
	// Verify AppError satisfies the error interface
	var err error = &model.AppError{Code: 400, Message: "bad request"}
	assert.NotNil(t, err)
	assert.Equal(t, "bad request", err.Error())
}

func TestAppError_WrappedWithFmtErrorf(t *testing.T) {
	inner := errors.New("root")
	wrapped := fmt.Errorf("wrap: %w", inner)
	appErr := model.NewAppError(http.StatusInternalServerError, "fail", wrapped)
	assert.Equal(t, "fail: wrap: root", appErr.Error())
	assert.True(t, errors.Is(appErr, inner))
}

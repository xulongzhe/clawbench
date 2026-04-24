package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors
var (
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("access denied")
	ErrNotFound      = errors.New("not found")
	ErrBadRequest    = errors.New("bad request")
	ErrInternal      = errors.New("internal server error")
	ErrProjectNotSet = errors.New("no project selected")
	ErrInvalidPath   = errors.New("invalid path")
	ErrPathTraversal = errors.New("path traversal detected")
)

// AppError is the application-level error type with an HTTP status code.
type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

// Constructor helpers

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func NewAppErrorf(code int, msg string, err error, args ...any) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(msg, args...), Err: err}
}

func BadRequest(err error, msg string) *AppError {
	return NewAppError(http.StatusBadRequest, msg, err)
}

func Forbidden(err error, msg string) *AppError {
	return NewAppError(http.StatusForbidden, msg, err)
}

func NotFound(err error, msg string) *AppError {
	return NewAppError(http.StatusNotFound, msg, err)
}

func Internal(err error) *AppError {
	return NewAppError(http.StatusInternalServerError, "internal server error", err)
}

func Unauthorized(err error) *AppError {
	return NewAppError(http.StatusUnauthorized, "unauthorized", err)
}

// JSON error response helpers

type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code,omitempty"`
}

func WriteError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.Code)
		json.NewEncoder(w).Encode(ErrorResponse{Error: appErr.Message, Code: appErr.Code})
		return
	}
	WriteErrorf(w, http.StatusInternalServerError, "Internal server error")
}

func WriteErrorf(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg, Code: status})
}

package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawbench/internal/model"
)

func TestServeAndroidLogUpload(t *testing.T) {
	// Setup: create temp log dir
	tmpDir := t.TempDir()
	origLogDir := model.ConfigInstance.LogDir
	model.ConfigInstance.LogDir = filepath.Join(tmpDir, ".clawbench", "logs")
	defer func() { model.ConfigInstance.LogDir = origLogDir }()

	t.Run("successful upload", func(t *testing.T) {
		// Build multipart form
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "android-log.txt")
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		content := "=== ClawBench Android Log Dump ===\nTime: 2026-05-20 14:00:00\nDevice: Pixel 7\n\n--- AppLog Buffer ---\n(empty)\n\n--- logcat ---\nhello world\n"
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatalf("Write: %v", err)
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/android-log/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		ServeAndroidLogUpload(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
		}

		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("parse JSON: %v", err)
		}
		if result["ok"] != true {
			t.Errorf("expected ok=true, got %v", result["ok"])
		}
		pathStr, ok := result["path"].(string)
		if !ok || !strings.Contains(pathStr, "android-") {
			t.Errorf("expected path with 'android-', got %v", result["path"])
		}
		if size, ok := result["size"].(float64); !ok || size < 1 {
			t.Errorf("expected size > 0, got %v", result["size"])
		}

		// Verify file was actually written
		files, err := os.ReadDir(model.ConfigInstance.LogDir)
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		data, err := os.ReadFile(filepath.Join(model.ConfigInstance.LogDir, files[0].Name()))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if !strings.Contains(string(data), "ClawBench Android Log Dump") {
			t.Errorf("file content doesn't contain expected header")
		}
	})

	t.Run("rejects GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/android-log/upload", nil)
		w := httptest.NewRecorder()
		ServeAndroidLogUpload(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("rejects missing file", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.Close() // no file part

		req := httptest.NewRequest(http.MethodPost, "/api/android-log/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		ServeAndroidLogUpload(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d; body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejects invalid multipart", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/android-log/upload", strings.NewReader("not multipart"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
		w := httptest.NewRecorder()
		ServeAndroidLogUpload(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d; body: %s", w.Code, w.Body.String())
		}
	})
}

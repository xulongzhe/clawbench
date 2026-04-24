package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"clawbench/internal/model"
)

// maxUploadSize returns the maximum allowed upload size in bytes from config.
func maxUploadSize() int64 {
	return int64(model.UploadMaxSizeMB) * 1024 * 1024
}

// UploadFile handles POST /api/upload/file
func UploadFile(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize())

	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadSize()); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "File too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	// Validate file extension — reject only hidden files and dangerous extensions
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "File must have an extension")
		return
	}
	dangerousExts := map[string]bool{
		".exe": true, ".bat": true, ".cmd": true, ".com": true,
		".msi": true, ".scr": true, ".vbs": true, ".js": true,
		".wsf": true, ".ps1": true,
	}
	if dangerousExts[ext] {
		model.WriteErrorf(w, http.StatusBadRequest, fmt.Sprintf("File type not allowed: %s", ext))
		return
	}

	// Create uploads directory
	uploadsDir := filepath.Join(projectPath, ".clawbench", "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to create uploads directory")))
		return
	}

	// Generate unique filename
	filename := fmt.Sprintf("%d-%d%s", time.Now().UnixMilli(), time.Now().Nanosecond()%10000, ext)
	dstPath := filepath.Join(uploadsDir, filename)

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to create file")))
		return
	}
	defer dst.Close()

	// Copy file content
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(dstPath)
		model.WriteError(w, model.Internal(fmt.Errorf("failed to save file")))
		return
	}

	// Return relative path
	relativePath := filepath.Join(".clawbench", "uploads", filename)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"path": relativePath,
	})
}

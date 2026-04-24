package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"clawbench/internal/middleware"
	"clawbench/internal/model"
)

// ServeFileRename handles file and directory rename operations.
func ServeFileRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Path     string `json:"path"`
		Name     string `json:"name"`
		BasePath string `json:"basePath,omitempty"` // Optional: overrides project cookie path
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Path == "" || req.Name == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "Missing path or name")
		return
	}

	// Use provided basePath or fall back to project cookie
	basePath := req.BasePath
	if basePath == "" {
		basePath = middleware.GetProjectFromCookie(r)
		if basePath == "" {
			model.WriteError(w, model.Forbidden(model.ErrProjectNotSet, "no project selected"))
			return
		}
	}

	baseAbs, err := filepath.Abs(basePath)
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to resolve base path: %w", err)))
		return
	}

	absOld, ok := model.ValidatePath(baseAbs, req.Path)
	if !ok {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return
	}
	newPath := filepath.Join(filepath.Dir(absOld), req.Name)
	absNew, err := filepath.Abs(newPath)
	if err != nil || !strings.HasPrefix(absNew, baseAbs+string(filepath.Separator)) {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return
	}

	slog.Info("rename attempt", slog.String("base", baseAbs), slog.String("old", absOld), slog.String("new", absNew))
	if err := os.Rename(absOld, absNew); err != nil {
		slog.Error("rename failed", slog.String("old", absOld), slog.String("new", absNew), slog.String("err", err.Error()))
		model.WriteError(w, model.Internal(fmt.Errorf("rename failed: %w", err)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// ServeFileEditLine handles single-line editing operations.
func ServeFileEditLine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Path        string `json:"path"`
		LineNum     int    `json:"lineNum"`
		Content     string `json:"content,omitempty"`
		Delete      bool   `json:"delete,omitempty"`
		InsertAbove bool   `json:"insertAbove,omitempty"`
		InsertBelow bool   `json:"insertBelow,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" || req.LineNum < 1 {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request")
		return
	}
	basePath, _ := filepath.Abs(projectPath)
	absPath, ok := model.ValidatePath(basePath, req.Path)
	if !ok {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("cannot read file")))
		return
	}
	lines := strings.Split(string(data), "\n")
	if req.LineNum > len(lines) {
		model.WriteErrorf(w, http.StatusBadRequest, "Line number out of range")
		return
	}
	if req.Delete {
		lines = append(lines[:req.LineNum-1], lines[req.LineNum:]...)
	} else if req.InsertAbove {
		lines = append(lines[:req.LineNum-1], append([]string{""}, lines[req.LineNum-1:]...)...)
	} else if req.InsertBelow {
		lines = append(lines[:req.LineNum], append([]string{""}, lines[req.LineNum:]...)...)
	} else {
		lines[req.LineNum-1] = req.Content
	}
	if err := os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("cannot write file")))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// ServeFileDelete handles file and directory deletion.
func ServeFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Path     string `json:"path"`
		BasePath string `json:"basePath,omitempty"` // Optional: overrides project cookie path
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Path == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "Missing path")
		return
	}

	// Use provided basePath or fall back to project cookie
	basePath := req.BasePath
	if basePath == "" {
		basePath = middleware.GetProjectFromCookie(r)
		if basePath == "" {
			model.WriteError(w, model.Forbidden(model.ErrProjectNotSet, "no project selected"))
			return
		}
	}

	baseAbs, err := filepath.Abs(basePath)
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to resolve base path: %w", err)))
		return
	}

	absPath, ok := model.ValidatePath(baseAbs, req.Path)
	if !ok {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		model.WriteError(w, model.NotFound(nil, "Not found"))
		return
	}

	if info.IsDir() {
		os.RemoveAll(absPath)
	} else {
		os.Remove(absPath)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// validateCreatePath validates the path for file/directory creation operations.
// Returns the absolute path of the item to create, or empty string on error (response already written).
func validateCreatePath(w http.ResponseWriter, projectPath, reqPath, reqName string) string {
	basePath, _ := filepath.Abs(projectPath)
	absDir, ok := model.ValidatePath(basePath, reqPath)
	if !ok && reqPath != "" {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return ""
	}
	if reqPath == "" {
		absDir = basePath
	}

	fullPath := filepath.Join(absDir, reqName)
	absPath, err := filepath.Abs(fullPath)
	if err != nil || !strings.HasPrefix(absPath, basePath+string(filepath.Separator)) {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return ""
	}
	return absPath
}

// ServeFileCreate handles file creation.
func ServeFileCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "Missing name")
		return
	}

	absPath := validateCreatePath(w, projectPath, req.Path, req.Name)
	if absPath == "" {
		return
	}

	if _, err := os.Stat(absPath); err == nil {
		model.WriteErrorf(w, http.StatusConflict, "File already exists")
		return
	}

	if err := os.WriteFile(absPath, []byte{}, 0644); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("create file failed: %w", err)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// ServeDirCreate handles directory creation.
func ServeDirCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "Missing name")
		return
	}

	absPath := validateCreatePath(w, projectPath, req.Path, req.Name)
	if absPath == "" {
		return
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("create directory failed: %w", err)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// validateSrcDestPath validates source and destination paths for move/copy operations.
// Returns (srcAbsPath, destAbsPath) or empty strings on error (response already written).
func validateSrcDestPath(w http.ResponseWriter, projectPath, srcRel, destRel string) (string, string) {
	basePath, _ := filepath.Abs(projectPath)
	srcAbsPath, ok := model.ValidatePath(basePath, srcRel)
	if !ok {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return "", ""
	}
	destAbsPath, ok := model.ValidatePath(basePath, destRel)
	if !ok {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return "", ""
	}
	return srcAbsPath, destAbsPath
}

// ServeFileMove handles file and directory move operations.
func ServeFileMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	var req struct {
		Path string `json:"path"`
		Dest string `json:"dest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Path == "" || req.Dest == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "Missing path or dest")
		return
	}

	srcAbsPath, destAbsPath := validateSrcDestPath(w, projectPath, req.Path, req.Dest)
	if srcAbsPath == "" {
		return
	}

	if err := os.Rename(srcAbsPath, destAbsPath); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("move failed: %w", err)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// ServeFileCopy handles file and directory copy operations.
func ServeFileCopy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	var req struct {
		Path string `json:"path"`
		Dest string `json:"dest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Path == "" || req.Dest == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "Missing path or dest")
		return
	}

	srcAbsPath, destAbsPath := validateSrcDestPath(w, projectPath, req.Path, req.Dest)
	if srcAbsPath == "" {
		return
	}

	// Copy file or directory
	srcInfo, err := os.Stat(srcAbsPath)
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("source not found: %w", err)))
		return
	}

	if srcInfo.IsDir() {
		// Copy directory recursively
		err = copyDir(srcAbsPath, destAbsPath)
	} else {
		// Copy single file
		err = copyFile(srcAbsPath, destAbsPath)
	}

	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("copy failed: %w", err)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

// copyDir copies a directory recursively from src to dst.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// Note: ServeProjects POST (create directory) is handled here to keep all file operations together.
// This is called from RegisterRoutes in handler.go.
func serveProjectsCreate(w http.ResponseWriter, r *http.Request) {
	basePath, err := filepath.Abs(model.WatchDir)
	if err != nil {
		slog.Error("failed to resolve base path", slog.String("path", model.WatchDir), slog.String("err", err.Error()))
		model.WriteError(w, model.Internal(err))
		return
	}

	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if req.Name == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "Directory name required")
		return
	}
	var absPath string
	if req.Path == "" || req.Path == "/" {
		absPath = basePath
	} else if filepath.IsAbs(req.Path) {
		absPath = req.Path
	} else {
		rel := strings.TrimPrefix(req.Path, "/")
		absPath, err = filepath.Abs(filepath.Join(basePath, rel))
		if err != nil {
			slog.Warn("failed to resolve path", slog.String("path", req.Path), slog.String("err", err.Error()))
		}
	}
	if !strings.HasPrefix(absPath, basePath+string(filepath.Separator)) && absPath != basePath {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return
	}
	newDir := filepath.Join(absPath, req.Name)
	if err := os.Mkdir(newDir, 0755); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("create directory failed: %w", err)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "path": newDir})
}

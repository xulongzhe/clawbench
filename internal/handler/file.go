package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"clawbench/internal/model"
)

// mimeTypes maps file extensions to MIME types for ServeLocalFile.
var mimeTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".bmp":  "image/bmp",
	".pdf":  "application/pdf",
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".flac": "audio/flac",
	".wma":  "audio/x-ms-wma",
	".opus": "audio/opus",
	".mp4":  "video/mp4",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".flv":  "video/x-flv",
	".wmv":  "video/x-ms-wmv",
	".m4v":  "video/mp4",
	".3gp":  "video/3gpp",
	".m3u8": "application/vnd.apple.mpegurl",
}

// ListDir returns the contents of a directory within the current project.
func ListDir(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	relPath := strings.TrimPrefix(r.URL.Query().Get("path"), "/")
	basePath, err := filepath.Abs(projectPath)
	if err != nil {
		slog.Error("failed to resolve project path", slog.String("path", projectPath), slog.String("err", err.Error()))
		model.WriteError(w, model.Internal(err))
		return
	}

	absPath, ok := validateAndResolvePath(w, basePath, relPath)
	if !ok {
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("cannot read directory")))
		return
	}

	items := buildDirEntries(entries)

	relFromBase, _ := filepath.Rel(basePath, absPath)
	var parent *string
	if relFromBase != "." {
		parentDir := filepath.Dir(relFromBase)
		if parentDir != "." {
			parent = &parentDir
		} else {
			empty := ""
			parent = &empty
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":   relFromBase,
		"parent": parent,
		"items":  items,
	})
}

// ListFiles returns all files in the project directory recursively.
func ListFiles(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	var files []FileInfo
	err := filepath.Walk(projectPath, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(projectPath, fullPath)
		if err != nil {
			return nil
		}
		entryType := "file"
		if model.IsImageFile(info.Name()) {
			entryType = "image"
		}
		files = append(files, FileInfo{
			Name:      info.Name(),
			Path:      filepath.ToSlash(relPath),
			Modified:  info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
			Size:      info.Size(),
			Type:      entryType,
		Supported: model.IsSupportedFile(info.Name()),
	})
	return nil
})

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Cannot access directory"})
		return
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	writeJSON(w, http.StatusOK, files)
}

// GetFile returns the content of a single file.
func GetFile(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	filepathStr := r.URL.Path
	if !strings.HasPrefix(filepathStr, "/api/file/") {
		http.NotFound(w, r)
		return
	}
	filepathStr = filepathStr[len("/api/file/"):]
	filepathStr = path.Clean(filepathStr)

	if filepathStr == ".." || path.IsAbs(filepathStr) {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid file path")
		return
	}

	basePath, _ := filepath.Abs(projectPath)
	absPath, ok := validateAndResolvePath(w, basePath, filepathStr)
	if !ok {
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			model.WriteError(w, model.NotFound(nil, "File not found"))
		} else {
			model.WriteError(w, model.Internal(fmt.Errorf("cannot access file")))
		}
		return
	}
	if info.IsDir() {
		model.WriteErrorf(w, http.StatusBadRequest, "Not a file")
		return
	}

	if info.Size() > 10*1024*1024 {
		model.WriteErrorf(w, http.StatusBadRequest, "文件过大")
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("cannot read file")))
		return
	}

	relPath, _ := filepath.Rel(projectPath, absPath)
	writeJSON(w, http.StatusOK, FileContent{
		Content:   string(content),
		Name:      info.Name(),
		Path:      filepath.ToSlash(relPath),
		Supported: model.IsSupportedFile(info.Name()),
		Size:      info.Size(),
	})
}

// ServeLocalFile serves a file directly (for images, PDFs, etc.).
func ServeLocalFile(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	filepathStr := r.URL.Path
	if !strings.HasPrefix(filepathStr, "/api/local-file/") {
		http.NotFound(w, r)
		return
	}
	filepathStr = filepathStr[len("/api/local-file/"):]
	filepathStr = path.Clean(filepathStr)

	if filepathStr == ".." || path.IsAbs(filepathStr) {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid path")
		return
	}

	basePath, _ := filepath.Abs(projectPath)
	absPath, ok := validateAndResolvePath(w, basePath, filepathStr)
	if !ok {
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		model.WriteError(w, model.NotFound(nil, "File not found"))
		return
	}
	if info.IsDir() {
		model.WriteErrorf(w, http.StatusBadRequest, "Not a directory")
		return
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	mime := mimeTypes[ext]
	if mime == "" {
		mime = "application/octet-stream"
	}

	w.Header().Set("Content-Type", mime)
	http.ServeFile(w, r, absPath)
}

// ServeProjects handles GET (list directory) and POST (create directory) for projects.
func ServeProjects(w http.ResponseWriter, r *http.Request) {
	basePath, err := filepath.Abs(model.WatchDir)
	if err != nil {
		slog.Error("failed to resolve base path", slog.String("path", model.WatchDir), slog.String("err", err.Error()))
		model.WriteError(w, model.Internal(err))
		return
	}

	switch r.Method {
	case http.MethodPost:
		serveProjectsCreate(w, r)
		return
	case http.MethodGet:
		// continue below
	default:
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	rawPath := r.URL.Query().Get("path")

	var absPath string
	if rawPath == "" || rawPath == "/" {
		absPath = basePath
	} else if filepath.IsAbs(rawPath) {
		absPath = rawPath
		if !strings.HasPrefix(absPath, basePath+string(filepath.Separator)) && absPath != basePath {
			// Not under watchDir — treat leading "/" as part of a relative path
			relPath := strings.TrimPrefix(rawPath, "/")
			var absErr error
			absPath, absErr = filepath.Abs(filepath.Join(basePath, relPath))
			if absErr != nil {
				slog.Warn("failed to resolve path", slog.String("path", rawPath), slog.String("err", absErr.Error()))
			}
		}
	} else {
		relPath := strings.TrimPrefix(rawPath, "/")
		if relPath == "" {
			absPath = basePath
		} else {
			var absErr error
			absPath, absErr = filepath.Abs(filepath.Join(basePath, relPath))
			if absErr != nil {
				slog.Warn("failed to resolve path", slog.String("path", rawPath), slog.String("err", absErr.Error()))
			}
		}
	}

	if !strings.HasPrefix(absPath, basePath+string(filepath.Separator)) && absPath != basePath {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("cannot read directory")))
		return
	}

	items := buildDirEntries(entries)

	var parent *string
	if absPath != basePath {
		parent = new(string)
		*parent = filepath.Dir(absPath)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":   absPath,
		"parent": parent,
		"items":  items,
	})
}

// ── File-related DTOs ──────────────────────────────────────────────────────────

// DirEntry represents a directory entry in API responses
type DirEntry struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Modified  string `json:"modified,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Supported bool   `json:"supported"`
}

// FileInfo represents file information in API responses
type FileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Modified  string `json:"modified"`
	Size      int64  `json:"size"`
	Type      string `json:"type"`
	Supported bool   `json:"supported"`
}

// FileContent represents file content in API responses
type FileContent struct {
	Content   string `json:"content"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Supported bool   `json:"supported"`
	Size      int64  `json:"size"`
}

// buildDirEntries builds a sorted list of directory entries
func buildDirEntries(entries []os.DirEntry) []DirEntry {
	var items []DirEntry
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil {
			slog.Warn("failed to get file info", slog.String("name", entry.Name()), slog.String("err", infoErr.Error()))
			continue
		}
		if entry.IsDir() {
			modified := info.ModTime().Format(time.RFC3339)
			items = append(items, DirEntry{Name: entry.Name(), Type: "dir", Modified: modified})
		} else {
			name := entry.Name()
			entryType := "file"
			if model.IsImageFile(name) {
				entryType = "image"
			}
			items = append(items, DirEntry{
				Name:      name,
				Type:      entryType,
				Modified:  info.ModTime().Format(time.RFC3339),
				Size:      info.Size(),
				Supported: model.IsSupportedFile(name),
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type == "dir"
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// serveProjectsCreate handles POST /api/projects (create directory under watchDir).
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
	if !decodeJSON(w, r, &req) {
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
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "path": newDir})
}

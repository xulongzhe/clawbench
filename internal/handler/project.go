package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"clawbench/internal/model"
	"clawbench/internal/service"
)

// ServeRecentProjects handles GET (list) and POST (add) for recent projects.
func ServeRecentProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		paths, err := service.GetRecentProjects()
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to load recent projects")))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(paths)

	case http.MethodPost:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if err := service.AddRecentProject(req.Path); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to save recent project")))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})

	case http.MethodDelete:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if err := service.RemoveRecentProject(req.Path); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to remove recent project")))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})

	default:
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ServeProjectSet handles GET (current project) and POST (set project).
func ServeProjectSet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cookie, err := r.Cookie("clawbench_project")
		projectPath := ""
		if err == nil && cookie.Value != "" {
			decoded, decErr := url.QueryUnescape(cookie.Value)
			if decErr == nil {
				projectPath = decoded
			} else {
				projectPath = cookie.Value
			}
		} else {
			recents, _ := service.GetRecentProjects()
			if len(recents) > 0 {
				projectPath = recents[0]
			} else {
				projectPath, _ = filepath.Abs(model.WatchDir)
			}
		http.SetCookie(w, &http.Cookie{
				Name:     "clawbench_project",
				Value:    url.QueryEscape(projectPath),
				Path:     "/",
				MaxAge:   7 * 24 * 3600,
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": projectPath})

	case http.MethodPost:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Resolve path relative to watchDir (same logic as serveProjects)
		basePath, _ := filepath.Abs(model.WatchDir)
		rawPath := req.Path
		var absPath string
		if rawPath == "" || rawPath == "/" {
			absPath = basePath
		} else if filepath.IsAbs(rawPath) {
			// Looks absolute but might be under watchDir — check bounds first
			absPath = rawPath
			if !strings.HasPrefix(absPath, basePath+string(filepath.Separator)) && absPath != basePath {
				// Not under watchDir — treat leading "/" as part of a relative path
				relPath := strings.TrimPrefix(rawPath, "/")
				absPath, _ = filepath.Abs(filepath.Join(basePath, relPath))
			}
		} else {
			// Relative path — resolve from watchDir
			relPath := strings.TrimPrefix(rawPath, "/")
			absPath, _ = filepath.Abs(filepath.Join(basePath, relPath))
		}
		if !strings.HasPrefix(absPath, basePath+string(filepath.Separator)) && absPath != basePath {
			model.WriteError(w, model.Forbidden(nil, "Access denied"))
			return
		}

		info, err := os.Stat(absPath)
		if err != nil || !info.IsDir() {
			model.WriteErrorf(w, http.StatusBadRequest, "Not a directory")
			return
		}

		// Clear chat session cookie when switching project
		http.SetCookie(w, &http.Cookie{
			Name:     "chat_session_id",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "clawbench_project",
			Value:    url.QueryEscape(absPath),
			Path:     "/",
			MaxAge:   7 * 24 * 3600,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true", "path": absPath})

	default:
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ServeWatchDir returns the configured watchDir and upload limits as JSON.
func ServeWatchDir(w http.ResponseWriter, r *http.Request) {
	absWatchDir, err := filepath.Abs(model.WatchDir)
	if err != nil {
		slog.Warn("failed to resolve watch dir", slog.String("path", model.WatchDir), slog.String("err", err.Error()))
		absWatchDir = model.WatchDir
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"watchDir":              absWatchDir,
		"uploadMaxSizeMB":       model.UploadMaxSizeMB,
		"uploadMaxFiles":        model.UploadMaxFiles,
		"chatInitialMessages":   model.ChatInitialMessages,
		"chatPageSize":          model.ChatPageSize,
		"chatCollapsedHeight":   model.ChatCollapsedHeight,
	})
}

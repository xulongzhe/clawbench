package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"clawbench/internal/middleware"
	"clawbench/internal/model"
)

// requireProject extracts the project path from cookie and writes error if not set.
// Returns the project path and true on success, or empty string and false on failure.
func requireProject(w http.ResponseWriter, r *http.Request) (string, bool) {
	projectPath := middleware.GetProjectFromCookie(r)
	if projectPath == "" {
		model.WriteError(w, model.Forbidden(model.ErrProjectNotSet, "no project selected"))
		return "", false
	}
	return projectPath, true
}

// requireMethod checks that the request method is one of the allowed methods.
// Writes 405 on mismatch. Returns true if allowed.
func requireMethod(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, m := range methods {
		if r.Method == m {
			return true
		}
	}
	model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
	return false
}

// writeJSON sets Content-Type and encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// decodeJSON decodes the request body into v. Writes 400 on failure.
// Returns true on success.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
}

// validateAndResolvePath validates a relative path and returns the absolute path.
// Writes 403 on failure. Returns (absPath, true) on success.
func validateAndResolvePath(w http.ResponseWriter, basePath, relPath string) (string, bool) {
	absPath, ok := model.ValidatePath(basePath, relPath)
	if !ok {
		model.WriteError(w, model.Forbidden(nil, "Access denied"))
		return "", false
	}
	return absPath, true
}

// resolveAgentConfig resolves agent configuration from model.Agents.
// Returns (backend, agentModel, systemPrompt, command, ok).
func resolveAgentConfig(agentID string) (string, string, string, string, bool) {
	if agentID == "" {
		agentID = model.GetDefaultAgentID()
	}
	if agentID == "" {
		return "", "", "", "", false
	}
	agent, found := model.Agents[agentID]
	if !found {
		return "", "", "", "", false
	}
	return agent.Backend, agent.Model, agent.SystemPrompt, agent.Command, true
}

// requireSessionID extracts session ID from query param or cookie.
// Writes 400 if not found. Returns (sessionID, true) on success.
func requireSessionID(w http.ResponseWriter, r *http.Request) (string, bool) {
	sessionID := getSessionID(r)
	if sessionID == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "session_id required")
		return "", false
	}
	return sessionID, true
}

// requireGitRepo checks that a .git directory exists in projectPath.
// Writes 404 if not found. Returns true if repo exists.
func requireGitRepo(w http.ResponseWriter, projectPath string) bool {
	gitDir := filepath.Join(projectPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		model.WriteError(w, model.NotFound(nil, "Not a git repository"))
		return false
	}
	return true
}

// RegisterRoutes registers all HTTP routes with the given mux
func RegisterRoutes(mux *http.ServeMux) {
	register := func(pattern string, handler http.HandlerFunc) {
		wrapped := middleware.RecoverPanic(middleware.WithRequestID(middleware.RequestLogger(handler)))
		mux.HandleFunc(pattern, wrapped)
	}

	register("/", ServeIndex)
	register("/login", ServeLogin)
	register("/dialog/project", middleware.Auth(ServeProjectDialog))
	register("/api/me", ServeAuthCheck)
	register("/api/watch-dir", ServeWatchDir)
	register("/api/projects", middleware.Auth(ServeProjects))
	register("/api/project", ServeProjectSet)
	register("/api/ai/chat", middleware.Auth(AIChat))
	register("/api/ai/chat/stream", middleware.Auth(AIChatStream))
	register("/api/ai/chat/cancel", middleware.Auth(CancelChat))
	register("/api/ai/history", middleware.Auth(ServeChatHistory))
	register("/api/ai/session", middleware.Auth(ServeAISession))
	register("/api/ai/sessions", middleware.Auth(ServeSessions))
	register("/api/ai/session/delete", middleware.Auth(DeleteSession))
	register("/api/ai/chat/count", middleware.Auth(ServeChatCount))
	register("/api/ai/chat/message", middleware.Auth(ServeChatMessageUpdate))
	register("/api/upload/file", middleware.Auth(UploadFile))
	register("/api/dir", middleware.Auth(ListDir))
	register("/api/files", middleware.Auth(ListFiles))
	register("/api/file/", middleware.Auth(GetFile))
	register("/api/git/project-history", middleware.Auth(ServeGitProjectHistory))
	register("/api/git/init", middleware.Auth(ServeGitInit))
	register("/api/git/file-diff", middleware.Auth(ServeGitFileDiff))
	register("/api/git/commit-files", middleware.Auth(ServeGitCommitFiles))
	register("/api/git/history", middleware.Auth(ServeGitHistory))
	register("/api/git/diff", middleware.Auth(ServeGitDiff))
	register("/api/git/status", middleware.Auth(ServeGitStatus))
	register("/api/git/working-tree", middleware.Auth(ServeGitWorkingTreeFiles))
	register("/api/file/rename", middleware.Auth(ServeFileRename))
	register("/api/file/edit-line", middleware.Auth(ServeFileEditLine))
	register("/api/file/delete", middleware.Auth(ServeFileDelete))
	register("/api/file/create", middleware.Auth(ServeFileCreate))
	register("/api/file/copy", middleware.Auth(ServeFileCopy))
	register("/api/dir/create", middleware.Auth(ServeDirCreate))
	register("/api/file/move", middleware.Auth(ServeFileMove))
	register("/api/recent-projects", middleware.Auth(ServeRecentProjects))
	register("/api/local-file/", middleware.Auth(ServeLocalFile))
	register("/api/agents", middleware.Auth(ServeAgents))
	register("/api/tasks", middleware.Auth(ServeTasks))
	register("/api/tasks/", middleware.Auth(ServeTaskByID))
	register("/api/tts/generate", middleware.Auth(TTSGenerate))

	if _, err := os.Stat("public"); err == nil {
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("public"))))
	} else {
		mux.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir(filepath.Join("web", "css")))))
		mux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(filepath.Join("web", "js")))))
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	}
}

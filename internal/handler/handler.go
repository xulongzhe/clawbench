package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	i18npkg "clawbench/internal/i18n"
	"clawbench/internal/middleware"
	"clawbench/internal/model"
)

// loc returns the Localizer for the current request.
func loc(r *http.Request) *i18n.Localizer {
	return middleware.GetLocalizer(r)
}

// T is a shorthand for translating a message key in the handler layer.
func T(r *http.Request, msgKey string, templateData ...map[string]interface{}) string {
	return i18npkg.T(loc(r), msgKey, templateData...)
}

// writeLocalizedErrorf writes a localized error response with i18n message key.
func writeLocalizedErrorf(w http.ResponseWriter, r *http.Request, status int, msgKey string, templateData ...map[string]interface{}) {
	localizedMsg := T(r, msgKey, templateData...)
	var detail map[string]any
	if len(templateData) > 0 {
		detail = templateData[0]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.ErrorResponse{Error: localizedMsg, Code: status, MsgKey: msgKey, Detail: detail})
}

// writeLocalizedError writes a localized AppError response.
func writeLocalizedError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *model.AppError
	if err == nil {
		writeLocalizedErrorf(w, r, http.StatusInternalServerError, "InternalError")
		return
	}
	if ok := errors.As(err, &appErr); ok {
		localizedMsg := T(r, appErr.Message)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.Code)
		json.NewEncoder(w).Encode(model.ErrorResponse{Error: localizedMsg, Code: appErr.Code, MsgKey: appErr.Message})
		return
	}
	writeLocalizedErrorf(w, r, http.StatusInternalServerError, "InternalError")
}

// requireProject extracts the project path from cookie and writes error if not set.
// Returns the project path and true on success, or empty string and false on failure.
func requireProject(w http.ResponseWriter, r *http.Request) (string, bool) {
	projectPath := middleware.GetProjectFromCookie(r)
	if projectPath == "" {
		writeLocalizedError(w, r, model.Forbidden(model.ErrProjectNotSet, "NoProjectSelected"))
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
	writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
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
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRequestBody")
		return false
	}
	return true
}

// validateAndResolvePath validates a relative path and returns the absolute path.
// Writes 403 on failure. Returns (absPath, true) on success.
func validateAndResolvePath(w http.ResponseWriter, r *http.Request, basePath, relPath string) (string, bool) {
	absPath, ok := model.ValidatePath(basePath, relPath)
	if !ok {
		writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
		return "", false
	}
	return absPath, true
}

// resolveAgentConfig resolves agent configuration from model.Agents.
// Returns (backend, defaultModelID, systemPrompt, command, ok).
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
	return agent.Backend, agent.DefaultModelID(), agent.SystemPrompt, agent.Command, true
}

// requireSessionID extracts session ID from query param or cookie.
// Writes 400 if not found. Returns (sessionID, true) on success.
func requireSessionID(w http.ResponseWriter, r *http.Request) (string, bool) {
	sessionID := getSessionID(r)
	if sessionID == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "SessionIdRequired")
		return "", false
	}
	return sessionID, true
}

// requireGitRepo checks that a .git directory exists in projectPath.
// Writes 404 if not found. Returns true if repo exists.
func requireGitRepo(w http.ResponseWriter, r *http.Request, projectPath string) bool {
	gitDir := filepath.Join(projectPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		writeLocalizedError(w, r, model.NotFound(nil, "NotAGitRepo"))
		return false
	}
	return true
}

// RegisterRoutes registers all HTTP routes with the given mux
func RegisterRoutes(mux *http.ServeMux) {
	register := func(pattern string, handler http.HandlerFunc) {
		wrapped := middleware.RecoverPanic(
			middleware.WithRequestID(
				middleware.RequestLogger(
					middleware.WithLocalizer(handler))))
		mux.HandleFunc(pattern, wrapped)
	}

	register("/", ServeIndex)
	register("/login", ServeLogin)
	register("/dialog/project", middleware.Auth(ServeProjectDialog))
	register("/api/me", ServeAuthCheck)
	register("/api/watch-dir", middleware.Auth(ServeWatchDir))
	register("/api/projects", middleware.Auth(ServeProjects))
	register("/api/project", middleware.Auth(ServeProjectSet))
	register("/api/ai/chat", middleware.Auth(AIChat))
	register("/api/ai/chat/stream", middleware.Auth(AIChatStream))
	register("/api/ai/chat/cancel", middleware.Auth(CancelChat))
	register("/api/ai/queue", middleware.Auth(QueueHandler))
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
	register("/api/tts/generate", middleware.Auth(TTSGenerate))
	register("/api/tts/stream/", middleware.Auth(TTSStream))
	register("/api/tasks", middleware.Auth(ServeTasks))
	register("/api/tasks/", middleware.Auth(ServeTaskByID))
	register("/api/rag/search", middleware.Auth(ServeRAGSearch))
	register("/api/rag/message", middleware.Auth(ServeRAGMessage))
	register("/api/rag/session", middleware.Auth(ServeRAGSession))

	// File watch SSE (auto-refresh on file changes)
	register("/api/file/watch", middleware.Auth(FileWatchSSE))
	register("/api/file/watch/update", middleware.Auth(FileWatchUpdate))

	// Port forwarding (registration & detection only; actual forwarding uses SSH tunnels)
	register("/api/proxy/ports", middleware.Auth(ServeProxyPortAction))
	register("/api/proxy/detect", middleware.Auth(ServeProxyDetect))

	// SSH tunnel info (no auth required — port number and fingerprint are not sensitive)
	register("/api/ssh/info", ServeSSHInfo)

	// Terminal (interactive web terminal with PTY + WebSocket + xterm.js)
	register("/api/terminal/ws", middleware.Auth(TerminalWebSocket))
	register("/api/terminal/status", middleware.Auth(TerminalStatus))
	register("/api/terminal/close", middleware.Auth(TerminalClose))
	register("/api/terminal/config", middleware.Auth(TerminalConfigHandler))
	register("/api/terminal/quick-commands", middleware.Auth(ServeQuickCommands))
	register("/api/terminal/quick-commands/", middleware.Auth(ServeQuickCommandByID))

	// Chat quick-send (CRUD for quick-send presets stored in database)
	register("/api/chat/quick-send", middleware.Auth(ServeChatQuickSend))
	register("/api/chat/quick-send/", middleware.Auth(ServeChatQuickSendByID))

	if _, err := os.Stat("public"); err == nil {
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("public"))))
	} else {
		mux.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir(filepath.Join("web", "css")))))
		mux.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(filepath.Join("web", "js")))))
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	}
}

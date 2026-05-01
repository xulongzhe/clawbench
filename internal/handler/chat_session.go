package handler

import (
	"fmt"
	"net/http"

	"clawbench/internal/model"
	"clawbench/internal/service"
)

// ServeSessions handles GET (list) and POST (create) for chat sessions.
func ServeSessions(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		sessions, err := service.GetSessions(projectPath, "")
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to load sessions")))
			return
		}
		for i := range sessions {
			sessions[i].Running = service.IsSessionRunning(sessions[i].ID)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"sessions": sessions})

	case http.MethodPost:
		// Check session count limit before creating (0 = unlimited)
		if model.SessionMaxCount > 0 {
			if count, cerr := service.GetSessionCount(projectPath); cerr == nil && count >= model.SessionMaxCount {
				model.WriteErrorf(w, http.StatusConflict, fmt.Sprintf("已达会话数量上限（%d），请先删除旧会话", model.SessionMaxCount))
				return
			}
		}

		var req struct {
			Title   string `json:"title"`
			Backend string `json:"backend"`
			AgentID string `json:"agentId"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxChatBodySize)
		if !decodeJSON(w, r, &req) {
			return
		}
		backend := req.Backend
		agentModel := ""
		agentID := req.AgentID
		resolvedAgentID := agentID
		backend2, model2, _, _, ok := resolveAgentConfig(agentID)
		if !ok {
			model.WriteErrorf(w, http.StatusServiceUnavailable, "no agents available")
			return
		}
		if backend2 != "" {
			backend = backend2
		}
		agentModel = model2
		if resolvedAgentID == "" {
			resolvedAgentID = model.GetDefaultAgentID()
		}
		if backend == "" {
			backend = "codebuddy"
		}
		title := req.Title
		if title == "" {
			existingSessions, err := service.GetSessions(projectPath, backend)
			if err == nil {
				title = fmt.Sprintf("新会话 %d", len(existingSessions)+1)
			} else {
				title = "新会话"
			}
		}
		sessionID, err := service.CreateSession(projectPath, backend, title, resolvedAgentID, agentModel)
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to create session")))
			return
		}
		setSessionID(w, sessionID)
		// Return session count for UI indicator
		sessionCount, _ := service.GetSessionCount(projectPath)
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "sessionId": sessionID, "backend": backend, "agentId": resolvedAgentID, "sessionCount": sessionCount, "title": title})

	default:
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// DeleteSession handles DELETE for a single session.
func DeleteSession(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	if !requireMethod(w, r, http.MethodDelete) {
		return
	}

	sessionID, ok := requireSessionID(w, r)
	if !ok {
		return
	}

	backend := r.URL.Query().Get("backend")
	if backend == "" {
		backend = "codebuddy"
	}

	if err := service.DeleteSession(projectPath, backend, sessionID); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to delete session")))
		return
	}

	sessionCount, _ := service.GetSessionCount(projectPath)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "sessionCount": sessionCount})
}

// getSessionID retrieves session ID from query param or cookie.
func getSessionID(r *http.Request) string {
	if sessionID := r.URL.Query().Get("session_id"); sessionID != "" {
		return sessionID
	}
	cookie, err := r.Cookie("chat_session_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// setSessionID sets session ID in cookie.
func setSessionID(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "chat_session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

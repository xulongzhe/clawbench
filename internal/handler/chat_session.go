package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
		// Parse optional pagination parameters
		limit := 0
		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 {
				limit = v
			}
		}
		cursor := r.URL.Query().Get("cursor")
		cursorID := r.URL.Query().Get("cursor_id")
		// Normalize cursor timestamp: frontend sends ISO 8601 (2026-05-16T15:25:50Z)
		// but SQLite stores as "2026-05-16 15:25:50". Convert T→space and strip Z/+00:00.
		if cursor != "" {
			cursor = strings.ReplaceAll(cursor, "T", " ")
			cursor = strings.TrimSuffix(cursor, "Z")
			cursor = strings.TrimSuffix(cursor, "+00:00")
		}

		var sessions []model.ChatSession
		var hasMore bool
		var err error

		if limit > 0 {
			sessions, hasMore, err = service.GetSessionsPaged(projectPath, "", limit, cursor, cursorID)
		} else {
			sessions, err = service.GetSessions(projectPath, "")
			hasMore = false
		}
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to load sessions")))
			return
		}
		for i := range sessions {
			sessions[i].Running = service.IsSessionRunning(sessions[i].ID)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"sessions": sessions,
			"hasMore":  hasMore,
		})

	case http.MethodPost:
		// Check session count limit before creating (0 = unlimited)
		if model.SessionMaxCount > 0 {
			if count, cerr := service.GetSessionCount(projectPath); cerr == nil && count >= model.SessionMaxCount {
				writeLocalizedErrorf(w, r, http.StatusConflict, "SessionLimitReached", map[string]any{"MaxCount": model.SessionMaxCount})
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
		agentID := req.AgentID
		resolvedAgentID := agentID
		agentSource := "default"
		backend2, _, _, _, ok := resolveAgentConfig(agentID)
		if !ok {
		writeLocalizedErrorf(w, r, http.StatusServiceUnavailable, "NoAgentsAvailable")
			return
		}
		if backend2 != "" {
			backend = backend2
		}
		// Don't pre-fill agent default model into session — leave model empty so
		// the frontend falls back to the global localStorage preference, making the
		// user's model choice persist across projects. The model will be persisted
		// to the session only when the user explicitly sends a message with a modelId.
		agentModel := ""
		if resolvedAgentID == "" {
			resolvedAgentID = model.GetDefaultAgentID()
		}
		// If user explicitly specified an agent, mark source as "user"
		if agentID != "" {
			agentSource = "user"
		}
		if backend == "" {
			backend = "codebuddy"
		}
		title := req.Title
		if title == "" {
			existingSessions, err := service.GetSessions(projectPath, backend)
			if err == nil {
				title = T(r, "NewSessionN", map[string]any{"N": len(existingSessions) + 1})
			} else {
				title = T(r, "NewSession")
			}
		}
		sessionID, err := service.CreateSession(projectPath, backend, title, resolvedAgentID, agentModel, agentSource, "chat")
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to create session")))
			return
		}
		setSessionID(w, sessionID)
		// Return session count for UI indicator
		sessionCount, _ := service.GetSessionCount(projectPath)
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "sessionId": sessionID, "backend": backend, "agentId": resolvedAgentID, "sessionCount": sessionCount, "title": title})

	default:
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
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

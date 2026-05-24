package handler

import (
	"fmt"
	"net/http"

	"clawbench/internal/model"
	"clawbench/internal/service"
)

// ServeChatHistory handles GET (list), POST (add), DELETE (clear) for chat history.
func ServeChatHistory(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			sessionID = getSessionID(r)
			if sessionID == "" {
				sessions, err := service.GetSessions(projectPath, "")
				if err != nil {
					model.WriteError(w, model.Internal(fmt.Errorf("failed to load sessions")))
					return
				}
				if len(sessions) == 0 {
					agentID := model.GetDefaultAgentID()
					backend, _, _, _, ok := resolveAgentConfig(agentID)
					if !ok {
				writeLocalizedErrorf(w, r, http.StatusServiceUnavailable, "NoAgentsAvailable")
						return
					}
					// Don't pre-fill agent default model — leave empty so frontend
					// falls back to global localStorage preference (cross-project).
					sessionID, err = service.CreateSession(projectPath, backend, T(r, "NewSession"), agentID, "", "default", "chat")
					if err != nil {
						model.WriteError(w, model.Internal(fmt.Errorf("failed to create session")))
						return
					}
				} else {
					sessionID = sessions[0].ID
				}
				setSessionID(w, sessionID)
			}
		}
	// ISS-077: Verify the session belongs to the requesting project
	sessionProject := service.GetSessionProjectPath(sessionID)
	if sessionProject != projectPath {
		writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
		return
	}
	backend := service.GetSessionBackend(sessionID)
		if backend == "" {
		writeLocalizedErrorf(w, r, http.StatusNotFound, "SessionNotFound")
			return
		}
		messages, err := service.GetChatHistory(projectPath, backend, sessionID)
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to load history")))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"messages": messages, "sessionId": sessionID})

	case http.MethodPost:
		var req struct {
			Role      string   `json:"role"`
			Content   string   `json:"content"`
			Files     []string `json:"files"`
			SessionID string   `json:"session_id"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxChatBodySize)
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Role != "user" && req.Role != "assistant" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRole")
			return
		}
		sessionID := req.SessionID
		if sessionID == "" {
			sessionID = getSessionID(r)
		}
		// ISS-077: Verify the session belongs to the requesting project
		if sp := service.GetSessionProjectPath(sessionID); sp != projectPath {
			writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
			return
		}
		backend := service.GetSessionBackend(sessionID)
		if backend == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "SessionNotFound")
			return
		}
		if _, err := service.AddChatMessage(projectPath, backend, sessionID, req.Role, req.Content, req.Files, false, T(r, "NewSession")); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to save message")))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "savedAt": "now"})

	default:
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

// ServeChatCount returns the message count for a session (lightweight polling endpoint).
func ServeChatCount(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}
	sessionID, ok := requireSessionID(w, r)
	if !ok {
		return
	}
	// Verify the session belongs to the requesting project
	if sessionProject := service.GetSessionProjectPath(sessionID); sessionProject != projectPath {
		writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
		return
	}
	count := service.GetChatMessageCount(sessionID)
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

// ServeChatMessageUpdate handles PUT to update a specific message's content.
func ServeChatMessageUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPut) {
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}
	var req struct {
		MessageID int64  `json:"messageId"`
		Content   string `json:"content"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.MessageID == 0 {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "MessageIdRequired")
		return
	}
	// Verify the message belongs to the requesting project
	msg, err := service.GetMessageByID(req.MessageID)
	if err != nil {
		writeLocalizedError(w, r, model.NotFound(nil, "MessageNotFound"))
		return
	}
	// Check session ownership
	if sessionProject := service.GetSessionProjectPath(msg.SessionID); sessionProject != projectPath {
		writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
		return
	}
	if err := service.UpdateMessageContent(int(req.MessageID), req.Content); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to update message")))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

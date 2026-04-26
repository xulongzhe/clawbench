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
					backend, defaultModel, _, _, ok := resolveAgentConfig(agentID)
					if !ok {
						model.WriteErrorf(w, http.StatusServiceUnavailable, "no agents available")
						return
					}
					sessionID, err = service.CreateSession(projectPath, backend, "新会话", agentID, defaultModel)
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
		backend := service.GetSessionBackend(sessionID)
		if backend == "" {
			model.WriteErrorf(w, http.StatusNotFound, "session not found")
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
			FilePath  string   `json:"file_path"`
			Files     []string `json:"files"`
			SessionID string   `json:"session_id"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxChatBodySize)
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Role != "user" && req.Role != "assistant" {
			model.WriteErrorf(w, http.StatusBadRequest, "Invalid role")
			return
		}
		sessionID := req.SessionID
		if sessionID == "" {
			sessionID = getSessionID(r)
		}
		backend := service.GetSessionBackend(sessionID)
		if backend == "" {
			model.WriteErrorf(w, http.StatusBadRequest, "session not found")
			return
		}
		if _, err := service.AddChatMessage(projectPath, backend, sessionID, req.Role, req.Content, req.FilePath, req.Files, false); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to save message")))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "savedAt": "now"})

	default:
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ServeChatCount returns the message count for a session (lightweight polling endpoint).
func ServeChatCount(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	sessionID, ok := requireSessionID(w, r)
	if !ok {
		return
	}
	_ = sessionID
	count := service.GetChatMessageCount(sessionID)
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

// ServeChatMessageUpdate handles PUT to update a specific message's content.
func ServeChatMessageUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPut) {
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
		model.WriteErrorf(w, http.StatusBadRequest, "messageId required")
		return
	}
	if err := service.UpdateMessageContent(int(req.MessageID), req.Content); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to update message")))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

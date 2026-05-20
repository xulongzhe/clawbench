package handler

import (
	"net/http"
	"strconv"

	"clawbench/internal/model"
	"clawbench/internal/rag"
	"clawbench/internal/service"
)

// ServeRAGSearch handles POST /api/rag/search — hybrid/FTS/vector search.
// Auth: localhost bypasses auth (CLI); remote requires cookie.
// Project isolation: uses cookie-derived project path, ignoring client-supplied project field.
func ServeRAGSearch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	var req struct {
		Query            string `json:"q"`
		Limit            int    `json:"limit"`
		ProjectPath      string `json:"project"`
		Backend          string `json:"backend"`
		Role             string `json:"role"`
		SessionID        string `json:"session_id"`
		ExcludeSessionID string `json:"exclude_session_id"`
		FromTime         string `json:"from"`
		ToTime           string `json:"to"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Query == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "SearchQueryRequired")
		return
	}

	defaultLimit := 5
	if req.Limit > 0 {
		defaultLimit = req.Limit
	}

	// Enforce project isolation: always use cookie-derived project path,
	// ignoring any client-supplied project field to prevent cross-project data access.
	params := rag.SearchParams{
		Query:            req.Query,
		Limit:            req.Limit,
		ProjectPath:      projectPath,
		Backend:          req.Backend,
		Role:             req.Role,
		SessionID:        req.SessionID,
		ExcludeSessionID: req.ExcludeSessionID,
		FromTime:         req.FromTime,
		ToTime:           req.ToTime,
	}

	searchPoolSize := model.ConfigInstance.RAG.SearchPoolSize
	result, err := rag.RAGSearch(r.Context(), rag.GlobalStore, rag.GlobalEmbedder, params, defaultLimit, searchPoolSize)
	if err != nil {
		writeLocalizedErrorf(w, r, http.StatusServiceUnavailable, "RAGSearchFailed")
		return
	}

	if result.Results == nil {
		result.Results = []rag.SearchHit{}
	}
	writeJSON(w, http.StatusOK, result)
}

// ServeRAGMessage handles GET /api/rag/message?id=<id> — get full message by ID.
// Project isolation: verifies the message's session belongs to the authenticated project.
func ServeRAGMessage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "MessageIdRequired")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidMessageId")
		return
	}

	msg, err := service.GetMessageByID(id)
	if err != nil {
		writeLocalizedErrorf(w, r, http.StatusNotFound, "MessageNotFound")
		return
	}

	// Verify the message belongs to the authenticated project
	if msg.ProjectPath != projectPath {
		writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
		return
	}

	writeJSON(w, http.StatusOK, msg)
}

// ServeRAGSession handles GET /api/rag/session?id=<id> — get all messages in a session.
// Project isolation: verifies the session belongs to the authenticated project.
func ServeRAGSession(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "SessionIdRequired")
		return
	}

	// Verify the session belongs to the authenticated project
	if sessionProject := service.GetSessionProjectPath(sessionID); sessionProject != projectPath {
		writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
		return
	}

	messages, err := service.GetMessagesBySessionID(sessionID)
	if err != nil {
		writeLocalizedErrorf(w, r, http.StatusNotFound, "SessionNotFound")
		return
	}

	if messages == nil {
		messages = []model.ChatMessage{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"messages":   messages,
		"total":      len(messages),
	})
}

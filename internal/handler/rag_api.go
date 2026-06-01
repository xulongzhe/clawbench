//nolint:goconst // JSON response field names are domain strings, not config constants
package handler

import (
	"net/http"
	"strconv"

	"clawbench/internal/middleware"
	"clawbench/internal/model"
	"clawbench/internal/rag"
	"clawbench/internal/service"
)

// ServeRAGSearch handles POST /api/rag/search — hybrid/FTS/vector search.
// Auth: localhost bypasses auth (CLI); remote requires cookie.
// Project isolation: remote requests require project cookie; localhost (CLI) may omit it for global search.
func ServeRAGSearch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Remote requests require project cookie; localhost (CLI) may omit it for global search.
	projectPath := middleware.GetProjectFromCookie(r)
	if projectPath == "" && !middleware.IsLocalhost(r) {
		writeLocalizedError(w, r, model.Forbidden(model.ErrProjectNotSet, "NoProjectSelected"))
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

	// Project isolation: use cookie-derived project path when set.
	// Empty projectPath (CLI global search) searches across all projects.
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
// Project isolation: remote requires project cookie; localhost may omit it for cross-project access.
func ServeRAGMessage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Remote requests require project cookie; localhost (CLI) may omit it.
	projectPath := middleware.GetProjectFromCookie(r)
	if projectPath == "" && !middleware.IsLocalhost(r) {
		writeLocalizedError(w, r, model.Forbidden(model.ErrProjectNotSet, "NoProjectSelected"))
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

	// Verify the message belongs to the authenticated project (skip for localhost global access)
	if projectPath != "" && msg.ProjectPath != projectPath {
		writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
		return
	}

	writeJSON(w, http.StatusOK, msg)
}

// ServeRAGSession handles GET /api/rag/session?id=<id> — get all messages in a session.
// Project isolation: remote requires project cookie; localhost may omit it for cross-project access.
func ServeRAGSession(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Remote requests require project cookie; localhost (CLI) may omit it.
	projectPath := middleware.GetProjectFromCookie(r)
	if projectPath == "" && !middleware.IsLocalhost(r) {
		writeLocalizedError(w, r, model.Forbidden(model.ErrProjectNotSet, "NoProjectSelected"))
		return
	}

	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "SessionIdRequired")
		return
	}

	// Verify the session belongs to the authenticated project (skip for localhost global access)
	if projectPath != "" {
		if sessionProject := service.GetSessionProjectPath(sessionID); sessionProject != projectPath {
			writeLocalizedError(w, r, model.Forbidden(nil, "AccessDenied"))
			return
		}
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

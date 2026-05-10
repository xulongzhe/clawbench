package handler

import (
	"net/http"
	"strconv"

	"clawbench/internal/model"
	"clawbench/internal/rag"
	"clawbench/internal/service"
)

// ServeRAGSearch handles POST /api/rag/search — vector similarity search.
// Auth: localhost bypasses auth (CLI); remote requires cookie.
func ServeRAGSearch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
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

	params := rag.SearchParams{
		Query:            req.Query,
		Limit:            req.Limit,
		ProjectPath:      req.ProjectPath,
		Backend:          req.Backend,
		Role:             req.Role,
		SessionID:        req.SessionID,
		ExcludeSessionID: req.ExcludeSessionID,
		FromTime:         req.FromTime,
		ToTime:           req.ToTime,
	}

	result, err := rag.RAGSearch(r.Context(), rag.GlobalStore, rag.GlobalEmbedder, params, defaultLimit)
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
func ServeRAGMessage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
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

	writeJSON(w, http.StatusOK, msg)
}

// ServeRAGSession handles GET /api/rag/session?id=<id> — get all messages in a session.
func ServeRAGSession(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "SessionIdRequired")
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

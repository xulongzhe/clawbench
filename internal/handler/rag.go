package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"clawbench/internal/rag"
)

// RAGSearch handles GET /api/rag/search
// No auth required — only accessible from localhost.
func RAGSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	params := rag.SearchParams{
		Query:            r.URL.Query().Get("q"),
		ProjectPath:      r.URL.Query().Get("project"),
		Backend:          r.URL.Query().Get("backend"),
		SessionID:        r.URL.Query().Get("session_id"),
		ExcludeSessionID: r.URL.Query().Get("exclude_session_id"),
		FromTime:         r.URL.Query().Get("from"),
		ToTime:           r.URL.Query().Get("to"),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			params.Limit = limit
		}
	}

	if params.Query == "" {
		writeJSON(w, http.StatusOK, rag.SearchResult{Results: []rag.SearchHit{}, Total: 0})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := rag.RAGSearch(ctx, ragGlobalStore, ragGlobalEmbedder, params, ragDefaultLimit)
	if err != nil {
		http.Error(w, `{"error":"search failed"}`, http.StatusInternalServerError)
		return
	}

	if result.Results == nil {
		result.Results = []rag.SearchHit{}
	}

	writeJSON(w, http.StatusOK, result)
}

// ragGlobalStore and ragGlobalEmbedder are set by SetRAGService during startup.
var (
	ragGlobalStore    *rag.Store
	ragGlobalEmbedder *rag.EmbeddingClient
	ragDefaultLimit   int
)

// SetRAGService configures the RAG handler with store and embedder instances.
func SetRAGService(store *rag.Store, embedder *rag.EmbeddingClient, searchLimit int) {
	ragGlobalStore = store
	ragGlobalEmbedder = embedder
	ragDefaultLimit = searchLimit
}

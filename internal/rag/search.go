package rag

import (
	"context"
	"fmt"
	"log/slog"

	"clawbench/internal/service"
)

// SearchParams holds the parameters for a RAG search request.
type SearchParams struct {
	Query             string `json:"q"`
	Limit             int    `json:"limit"`
	ProjectPath       string `json:"project"`
	Backend           string `json:"backend"`
	Role              string `json:"role"`                // Filter by role: "user" or "assistant"
	SessionID         string `json:"session_id"`          // Limit search to this session
	ExcludeSessionID  string `json:"exclude_session_id"`  // Exclude this session from results (e.g., current session)
	FromTime          string `json:"from"`
	ToTime            string `json:"to"`
}

// SearchResult represents the response from a RAG search.
type SearchResult struct {
	Results []SearchHit `json:"results"`
	Total   int         `json:"total"`
}

// RAGSearch performs a vector similarity search using the given parameters.
func RAGSearch(ctx context.Context, store *Store, embedder *EmbeddingClient, params SearchParams, defaultLimit int) (*SearchResult, error) {
	if params.Query == "" {
		return &SearchResult{}, nil
	}

	if store == nil || embedder == nil {
		return nil, fmt.Errorf("RAG not initialized: store and embedder must not be nil")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	// Generate embedding for the query
	queryEmbedding, err := embedder.Embed(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Perform vector search
	hits, err := store.SearchSimple(queryEmbedding, limit, params.ProjectPath, params.Backend, params.Role, params.SessionID, params.ExcludeSessionID, params.FromTime, params.ToTime)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// Enrich hits with session titles from SQLite
	sessionIDs := make(map[string]bool)
	for _, h := range hits {
		sessionIDs[h.SessionID] = true
	}

	// Fetch session titles in batch
	titles := getSessionTitles(sessionIDs)
	for i := range hits {
		if title, ok := titles[hits[i].SessionID]; ok {
			hits[i].SessionTitle = title
		}
	}

	slog.Info("rag search completed",
		slog.String("query", params.Query),
		slog.Int("results", len(hits)),
		slog.Int("limit", limit),
	)

	return &SearchResult{
		Results: hits,
		Total:   len(hits),
	}, nil
}

// getSessionTitles fetches session titles for a set of session IDs from SQLite.
func getSessionTitles(sessionIDs map[string]bool) map[string]string {
	titles := make(map[string]string, len(sessionIDs))
	for id := range sessionIDs {
		title, err := service.GetSessionTitle(id)
		if err == nil && title != "" {
			titles[id] = title
		}
	}
	return titles
}

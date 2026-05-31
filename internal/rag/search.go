package rag

import (
	"context"
	"fmt"
	"log/slog"

	"clawbench/internal/service"
)

// SearchMode indicates which search strategy was used.
type SearchMode string

const (
	SearchModeHybrid SearchMode = "hybrid" // Vector + FTS with RRF fusion
	SearchModeVector SearchMode = "vector" // Vector similarity only
	SearchModeFTS    SearchMode = "fts"    // Full-text search only (BM25)
)

// SearchParams holds the parameters for a RAG search request.
type SearchParams struct {
	Query            string `json:"q"`
	Limit            int    `json:"limit"`
	ProjectPath      string `json:"project"`
	Backend          string `json:"backend"`
	Role             string `json:"role"`               // Filter by role: "user" or "assistant"
	SessionID        string `json:"session_id"`         // Limit search to this session
	ExcludeSessionID string `json:"exclude_session_id"` // Exclude this session from results (e.g., current session)
	FromTime         string `json:"from"`
	ToTime           string `json:"to"`
}

// SearchResult represents the response from a RAG search.
type SearchResult struct {
	Results []SearchHit `json:"results"`
	Total   int         `json:"total"`
	Mode    SearchMode  `json:"mode"` // Which search strategy was used
}

// RAGSearch performs a search using the best available strategy:
//   - Hybrid (vector + FTS with RRF) when both embedding API and FTS are available
//   - Vector-only when embedding API is available but FTS is not
//   - FTS-only when embedding API is unavailable
func RAGSearch(ctx context.Context, store *Store, embedder *EmbeddingClient, params SearchParams, defaultLimit int, searchPoolSize int) (*SearchResult, error) { //nolint:gocyclo // multi-mode search with fallback
	if params.Query == "" {
		return &SearchResult{Mode: SearchModeFTS}, nil
	}

	if store == nil {
		return nil, fmt.Errorf("RAG not initialized: store is nil")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	poolSize := searchPoolSize
	if poolSize <= 0 {
		poolSize = 20
	}

	// Determine search strategy using cached embedder health state
	// (avoids per-request HTTP probe — indexer refreshes on every polling cycle)
	embedderHealthy := EmbedderHealthy()
	// If no cached state and embedder is available, do a fresh probe
	if !embedderHealthy && embedder != nil {
		reachable, modelAvailable, _ := embedder.IsHealthy(ctx)
		embedderHealthy = reachable && modelAvailable
	}

	ftsAvailable := store.ftsAvailable

	var hits []SearchHit
	var mode SearchMode
	var err error

	switch {
	case embedderHealthy && ftsAvailable:
		// Hybrid: vector + FTS with RRF fusion
		mode = SearchModeHybrid
		var queryEmbedding []float64
		queryEmbedding, err = embedder.Embed(ctx, params.Query)
		if err != nil {
			// Embedding failed — fall back to FTS-only
			slog.Warn("rag: query embedding failed, falling back to FTS", slog.String("err", err.Error()))
			hits, err = store.SearchFTS(params.Query, limit, params.ProjectPath, params.Backend, params.Role, params.SessionID, params.ExcludeSessionID, params.FromTime, params.ToTime)
			mode = SearchModeFTS
		} else {
			hits, err = store.SearchHybrid(queryEmbedding, params.Query, poolSize, limit, params.ProjectPath, params.Backend, params.Role, params.SessionID, params.ExcludeSessionID, params.FromTime, params.ToTime)
		}

	case embedderHealthy && !ftsAvailable:
		// Vector-only
		mode = SearchModeVector
		var queryEmbedding []float64
		queryEmbedding, err = embedder.Embed(ctx, params.Query)
		if err != nil {
			return nil, fmt.Errorf("embed query: %w", err)
		}
		hits, err = store.SearchSimple(queryEmbedding, limit, params.ProjectPath, params.Backend, params.Role, params.SessionID, params.ExcludeSessionID, params.FromTime, params.ToTime)

	default:
		// FTS-only (embedding API unavailable or no embedding)
		mode = SearchModeFTS
		if !ftsAvailable {
			return nil, fmt.Errorf("no search available: embedding API not reachable and FTS not loaded")
		}
		hits, err = store.SearchFTS(params.Query, limit, params.ProjectPath, params.Backend, params.Role, params.SessionID, params.ExcludeSessionID, params.FromTime, params.ToTime)
	}

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

	slog.Info(
		"rag search completed",
		slog.String("query", params.Query),
		slog.String("mode", string(mode)),
		slog.Int("results", len(hits)),
		slog.Int("limit", limit),
	)

	return &SearchResult{
		Results: hits,
		Total:   len(hits),
		Mode:    mode,
	}, nil
}

// getSessionTitles fetches session titles for a set of session IDs from SQLite.
// Uses a single batched query with IN clause instead of N individual queries.
func getSessionTitles(sessionIDs map[string]bool) map[string]string {
	if len(sessionIDs) == 0 {
		return map[string]string{}
	}
	ids := make([]string, 0, len(sessionIDs))
	for id := range sessionIDs {
		ids = append(ids, id)
	}
	titles, err := service.GetSessionTitlesBatch(ids)
	if err != nil {
		// Fallback to individual queries if batch fails
		titles = make(map[string]string, len(sessionIDs))
		for id := range sessionIDs {
			title, err := service.GetSessionTitle(id)
			if err == nil && title != "" {
				titles[id] = title
			}
		}
	}
	return titles
}

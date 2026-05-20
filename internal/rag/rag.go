package rag

import (
	"fmt"
	"log/slog"
	"sync/atomic"

	"clawbench/internal/model"
)

var (
	// GlobalStore is the singleton DuckDB store instance.
	GlobalStore *Store
	// GlobalIndexer is the singleton indexer instance.
	GlobalIndexer *Indexer
	// GlobalEmbedder is the singleton embedding client instance (may be nil if Ollama not configured).
	GlobalEmbedder *EmbeddingClient
	// GlobalCleanupWorker is the singleton cleanup worker instance.
	GlobalCleanupWorker *CleanupWorker
	// ollamaHealthyFlag is the cached Ollama health state, updated by the indexer.
	// RAGSearch reads this to avoid per-request health probes.
	ollamaHealthyFlag atomic.Bool
)

// Init initializes the RAG system: segmenter, DuckDB store, embedding client, and dimension check.
// RAG is always enabled — if Ollama is unavailable, FTS-only search is used.
func Init(cfg model.RAGConfig) error {
	// Initialize Chinese segmenter (non-critical — FTS falls back to raw text)
	if err := InitSegmenter(); err != nil {
		slog.Warn("rag: gse segmenter not available, Chinese FTS may be limited", slog.String("err", err.Error()))
	}

	// Initialize DuckDB store
	store, err := InitStore()
	if err != nil {
		return fmt.Errorf("init rag store: %w", err)
	}

	// Check embedding dimension compatibility
	const bgeM3Dim = 1024
	existingDim, mismatch, err := store.CheckDimensionMismatch(bgeM3Dim)
	if err != nil {
		slog.Warn("rag: failed to check dimension, continuing", slog.String("err", err.Error()))
	} else if mismatch {
		slog.Warn("rag: embedding dimension mismatch, resetting table",
			slog.Int("existing_dim", existingDim),
			slog.Int("expected_dim", bgeM3Dim),
		)
		if err := store.ResetTable(); err != nil {
			store.Close()
			return fmt.Errorf("reset rag table: %w", err)
		}
	}

	// Initialize embedding client (may be nil if Ollama not configured)
	embedder := NewEmbeddingClient(cfg.OllamaBaseURL, cfg.OllamaModel)

	GlobalStore = store
	GlobalEmbedder = embedder

	slog.Info("rag initialized",
		slog.String("ollama_url", cfg.OllamaBaseURL),
		slog.String("model", cfg.OllamaModel),
		slog.Int("chunk_size", cfg.ChunkSize),
		slog.Bool("fts_available", store.ftsAvailable),
	)

	return nil
}

// StartIndexer creates and starts the RAG indexer.
// Starts even without embedder (FTS-only mode) — indexer will detect Ollama health.
func StartIndexer(cfg model.RAGConfig) {
	if GlobalStore == nil {
		slog.Warn("rag: cannot start indexer, store not initialized")
		return
	}
	GlobalIndexer = NewIndexer(GlobalStore, GlobalEmbedder, cfg)
	GlobalIndexer.Start()
}

// StartCleanupWorker creates and starts the cleanup worker.
// Starts regardless of whether RAG is enabled — soft-deleted SQLite data
// accumulates even without RAG. When RAG is disabled, store is nil and
// only SQLite cleanup runs.
func StartCleanupWorker(cfg model.RAGConfig) {
	if cfg.RetentionDays <= 0 {
		return
	}
	GlobalCleanupWorker = NewCleanupWorker(GlobalStore, cfg)
	GlobalCleanupWorker.Start()
}

// Shutdown gracefully stops the RAG system.
func Shutdown() {
	if GlobalCleanupWorker != nil {
		GlobalCleanupWorker.Stop()
		GlobalCleanupWorker = nil
	}
	if GlobalIndexer != nil {
		GlobalIndexer.Stop()
		GlobalIndexer = nil
	}
	if GlobalStore != nil {
		GlobalStore.Close()
		GlobalStore = nil
	}
	GlobalEmbedder = nil
	slog.Info("rag shutdown complete")
}

// OllamaHealthy returns the cached Ollama health state from the indexer.
// This avoids per-search HTTP health probes — the indexer refreshes the state
// on every polling cycle.
func OllamaHealthy() bool {
	return ollamaHealthyFlag.Load()
}

// SetOllamaHealthy updates the cached Ollama health state (called by the indexer).
func SetOllamaHealthy(healthy bool) {
	ollamaHealthyFlag.Store(healthy)
}

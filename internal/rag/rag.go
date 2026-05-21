package rag

import (
	"fmt"
	"log/slog"
	"sync/atomic"

	"clawbench/internal/model"
)

var (
	GlobalStore           *Store
	GlobalIndexer         *Indexer
	GlobalEmbedder        *EmbeddingClient
	GlobalCleanupWorker   *CleanupWorker
	embedderHealthyFlag   atomic.Bool
)

func Init(cfg model.RAGConfig) error {
	if err := InitSegmenter(); err != nil {
		slog.Warn("rag: gse segmenter not available, Chinese FTS may be limited", slog.String("err", err.Error()))
	}

	store, err := InitStore()
	if err != nil {
		return fmt.Errorf("init rag store: %w", err)
	}

	existingDim, mismatch, err := store.CheckDimensionMismatch()
	if err != nil {
		slog.Warn("rag: failed to check dimension, continuing", slog.String("err", err.Error()))
	} else if mismatch {
		slog.Warn("rag: embedding dimension mismatch, resetting table",
			slog.Int("existing_dim", existingDim),
			slog.Int("expected_dim", store.embeddingDim),
		)
		if err := store.ResetTable(); err != nil {
			store.Close()
			return fmt.Errorf("reset rag table: %w", err)
		}
	}

	embedder := NewEmbeddingClient(cfg.BaseURL, cfg.Model, cfg.APIKey)

	GlobalStore = store
	GlobalEmbedder = embedder

	slog.Info("rag initialized",
		slog.String("base_url", cfg.BaseURL),
		slog.String("model", cfg.Model),
		slog.Int("chunk_size", cfg.ChunkSize),
		slog.Bool("fts_available", store.ftsAvailable),
		slog.Int("embedding_dim", store.embeddingDim),
	)

	return nil
}

func StartIndexer(cfg model.RAGConfig) {
	if GlobalStore == nil {
		slog.Warn("rag: cannot start indexer, store not initialized")
		return
	}
	GlobalIndexer = NewIndexer(GlobalStore, GlobalEmbedder, cfg)
	GlobalIndexer.Start()
}

func StartCleanupWorker(cfg model.RAGConfig) {
	if cfg.RetentionDays <= 0 {
		return
	}
	GlobalCleanupWorker = NewCleanupWorker(GlobalStore, cfg)
	GlobalCleanupWorker.Start()
}

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

func EmbedderHealthy() bool {
	return embedderHealthyFlag.Load()
}

func SetEmbedderHealthy(healthy bool) {
	embedderHealthyFlag.Store(healthy)
}

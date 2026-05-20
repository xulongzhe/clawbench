package rag

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/service"
)

// Indexer polls for unindexed chat messages and generates embeddings.
// When the embedding API is unavailable, it indexes text-only (for FTS search).
// When the embedding API becomes available, it backfills embeddings for pending chunks.
type Indexer struct {
	store           *Store
	embedder        *EmbeddingClient
	cfg             model.RAGConfig
	stopCh          chan struct{}
	doneCh          chan struct{}
	mu              sync.Mutex
	running         bool
	modelWarn       bool  // Whether we've warned about missing model
	embedderHealthy bool  // Current embedding API health state
	dimensionSynced bool  // Whether dimension has been synced from embedder to store
	batchCancel     context.CancelFunc // cancels in-flight indexBatch on Stop
}

// NewIndexer creates a new RAG indexer.
func NewIndexer(store *Store, embedder *EmbeddingClient, cfg model.RAGConfig) *Indexer {
	return &Indexer{
		store:    store,
		embedder: embedder,
		cfg:      cfg,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the indexer loop in a goroutine.
func (idx *Indexer) Start() {
	idx.mu.Lock()
	if idx.running {
		idx.mu.Unlock()
		return
	}
	idx.running = true
	idx.mu.Unlock()

	go idx.run()
	slog.Info("rag indexer started",
		slog.String("poll_interval", idx.cfg.PollInterval),
		slog.Int("batch_size", idx.cfg.BatchSize),
		slog.Int("chunk_size", idx.cfg.ChunkSize),
	)
}

// Stop signals the indexer to stop and waits for it to finish.
// Cancels any in-flight batch and waits up to 5s for the goroutine to exit.
func (idx *Indexer) Stop() {
	idx.mu.Lock()
	if !idx.running {
		idx.mu.Unlock()
		return
	}
	idx.mu.Unlock()

	// Cancel any in-flight batch so indexBatch returns quickly
	if idx.batchCancel != nil {
		idx.batchCancel()
	}

	close(idx.stopCh)

	// Wait with timeout to avoid blocking forever on a stuck goroutine
	select {
	case <-idx.doneCh:
	case <-time.After(5 * time.Second):
		slog.Warn("rag: indexer did not stop within timeout, continuing shutdown")
	}

	idx.mu.Lock()
	idx.running = false
	idx.mu.Unlock()

	slog.Info("rag indexer stopped")
}

// run is the main indexer loop.
func (idx *Indexer) run() {
	defer close(idx.doneCh)

	pollInterval, err := time.ParseDuration(idx.cfg.PollInterval)
	if err != nil {
		slog.Error("invalid rag poll_interval, using 10s", slog.String("value", idx.cfg.PollInterval), slog.String("err", err.Error()))
		pollInterval = 10 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Run first indexing immediately
	idx.indexBatch()

	for {
		select {
		case <-idx.stopCh:
			return
		case <-ticker.C:
			// Check stop again before starting a new batch
			select {
			case <-idx.stopCh:
				return
			default:
			}
			idx.indexBatch()
		}
	}
}

// indexBatch processes one batch of unindexed messages and backfills embeddings.
// Uses a stop-aware context so Stop() can cancel in-flight work.
func (idx *Indexer) indexBatch() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Store cancel so Stop() can abort this batch
	idx.mu.Lock()
	idx.batchCancel = cancel
	idx.mu.Unlock()

	// Check embedding API health (startup probe + periodic retry)
	idx.checkEmbedderHealth(ctx)

	// Phase 1: Index new messages from SQLite
	idx.indexNewMessages(ctx)

	// Exit early if stopped
	if ctx.Err() != nil {
		return
	}

	// Phase 2: Backfill embeddings for chunks that were indexed without them
	if idx.embedderHealthy {
		idx.backfillEmbeddings(ctx)
	}

	// Phase 3: Rebuild FTS index if dirty
	if err := idx.store.RebuildFTSIfDirty(); err != nil {
		slog.Debug("rag: FTS rebuild deferred", slog.String("err", err.Error()))
	}
}

// checkEmbedderHealth checks embedding API availability and updates the healthy flag.
// Also updates the global cached health state for search requests.
// After health check passes, syncs embedding dimension from embedder to store.
func (idx *Indexer) checkEmbedderHealth(ctx context.Context) {
	reachable, modelAvailable, err := idx.embedder.IsHealthy(ctx)
	if err != nil {
		slog.Debug("rag: embedding API health check error", slog.String("err", err.Error()))
		idx.embedderHealthy = false
		SetEmbedderHealthy(false)
		return
	}
	if !reachable {
		if idx.embedderHealthy {
			slog.Info("rag: embedding API became unreachable")
		}
		idx.embedderHealthy = false
		SetEmbedderHealthy(false)
		return
	}
	if !modelAvailable {
		if !idx.modelWarn {
			slog.Warn("rag: embedding API reachable but model not available",
				slog.String("model", idx.cfg.Model),
			)
			idx.modelWarn = true
		}
		idx.embedderHealthy = false
		SetEmbedderHealthy(false)
		return
	}

	// Embedding API is healthy
	if !idx.embedderHealthy {
		slog.Info("rag: embedding API became healthy, will backfill embeddings")
	}

	// Sync dimension from embedder to store (one-time)
	if !idx.dimensionSynced {
		if dim := idx.embedder.Dim(); dim > 0 {
			if idx.store.SetEmbeddingDim(dim) {
				slog.Info("rag: synced embedding dimension from embedder", slog.Int("dim", dim))
			}
			idx.dimensionSynced = true
		}
	}

	idx.embedderHealthy = true
	idx.modelWarn = false
	SetEmbedderHealthy(true)
}

// indexNewMessages indexes new (unindexed) messages from SQLite.
// When the embedding API is healthy, generates embeddings; otherwise stores text-only (for FTS).
func (idx *Indexer) indexNewMessages(ctx context.Context) {
	// Fetch unindexed messages from SQLite (newest first)
	messages, err := service.GetUnindexedMessages(idx.cfg.BatchSize)
	if err != nil {
		slog.Error("rag: failed to fetch unindexed messages", slog.String("err", err.Error()))
		return
	}
	if len(messages) == 0 {
		return
	}

	// Get total unindexed count for progress tracking
	totalRemaining, _ := service.UnindexedCount()

	slog.Info("rag: indexing batch",
		slog.Int("batch_size", len(messages)),
		slog.Int("remaining", totalRemaining),
		slog.Bool("embedder_healthy", idx.embedderHealthy),
	)

	batchStart := time.Now()
	indexed := 0
	skipped := 0

	for _, msg := range messages {
		msgStart := time.Now()
		if err := idx.indexMessage(ctx, msg); err != nil {
			slog.Error("rag: failed to index message",
				slog.Int64("message_id", msg.ID),
				slog.String("session_id", msg.SessionID),
				slog.String("err", err.Error()),
			)
			// Continue with next message — don't let one failure stop the batch
			continue
		}

		// Mark message as indexed in SQLite
		if err := service.MarkMessageIndexed(msg.ID); err != nil {
			slog.Error("rag: failed to mark message indexed",
				slog.Int64("message_id", msg.ID),
				slog.String("err", err.Error()),
			)
		}

		text := ExtractTextFromContent(msg.Content, msg.Role)
		if text == "" {
			skipped++
		} else {
			indexed++
			slog.Debug("rag: indexed message",
				slog.Int64("message_id", msg.ID),
				slog.String("session_id", msg.SessionID),
				slog.String("role", msg.Role),
				slog.Duration("elapsed", time.Since(msgStart)),
			)
		}
	}

	slog.Info("rag: batch complete",
		slog.Int("indexed", indexed),
		slog.Int("skipped", skipped),
		slog.Duration("elapsed", time.Since(batchStart)),
		slog.Int("remaining", func() int {
			remaining, _ := service.UnindexedCount()
			return remaining
		}()),
	)
}

// indexMessage processes a single message: extract text, chunk, (optionally) embed, store.
func (idx *Indexer) indexMessage(ctx context.Context, msg service.UnindexedMessage) error {
	// Extract text content
	text := ExtractTextFromContent(msg.Content, msg.Role)
	if text == "" {
		// No text content (e.g., only tool_use blocks) — mark as indexed to skip
		slog.Debug("rag: skipping message with no text content",
			slog.Int64("message_id", msg.ID),
			slog.String("role", msg.Role),
		)
		return nil
	}

	// Chunk the text
	textChunks := ChunkText(text, idx.cfg.ChunkSize, idx.cfg.ChunkOverlap)
	if len(textChunks) == 0 {
		return nil
	}

	// Limit chunks per message to prevent runaway
	maxChunks := 50
	if len(textChunks) > maxChunks {
		slog.Warn("rag: message produced too many chunks, truncating",
			slog.Int64("message_id", msg.ID),
			slog.Int("original", len(textChunks)),
			slog.Int("truncated", maxChunks),
		)
		textChunks = textChunks[:maxChunks]
	}

	// Build Chunk objects for storage
	chunks := make([]Chunk, len(textChunks))
	for i, tc := range textChunks {
		chunks[i] = Chunk{
			SessionID:          msg.SessionID,
			MessageID:          msg.ID,
			ChunkText:          tc.Text,
			ChunkTextSegmented: SegmentText(tc.Text),
			ChunkIndex:         tc.Index,
			TokenCount:         tc.TokenCount,
			ProjectPath:        msg.ProjectPath,
			Backend:            msg.Backend,
			Role:               msg.Role,
			CreatedAt:          msg.CreatedAt,
		}
	}

	// Generate embeddings if the embedding API is healthy
	if idx.embedderHealthy {
		slog.Debug("rag: embedding message",
			slog.Int64("message_id", msg.ID),
			slog.Int("chunks", len(textChunks)),
			slog.String("session_id", msg.SessionID),
		)

		texts := make([]string, len(textChunks))
		for i, tc := range textChunks {
			texts[i] = tc.Text
		}

		embeddings, err := idx.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			// Embedding failed — store without embedding (FTS still works)
			slog.Warn("rag: embedding failed, storing text-only",
				slog.Int64("message_id", msg.ID),
				slog.String("err", err.Error()),
			)
			for i := range chunks {
				chunks[i].Embedding = nil
				chunks[i].HasEmbedding = false
			}
		} else {
			for i := range chunks {
				chunks[i].Embedding = embeddings[i]
				chunks[i].HasEmbedding = true
			}
		}
	}
	// When the embedding API is NOT healthy, chunks are stored without embedding
	// (Embedding: nil, HasEmbedding: false) — FTS search still works.

	// Store in DuckDB
	return idx.store.InsertChunks(chunks)
}

// backfillEmbeddings generates embeddings for chunks that were stored without them.
func (idx *Indexer) backfillEmbeddings(ctx context.Context) {
	pending, err := idx.store.PendingEmbeddingCount()
	if err != nil {
		slog.Debug("rag: failed to check pending embeddings", slog.String("err", err.Error()))
		return
	}
	if pending == 0 {
		return
	}

	slog.Info("rag: backfilling embeddings", slog.Int("pending", pending))

	batchSize := idx.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	// Limit backfill per cycle to avoid overloading
	maxBackfill := batchSize
	if maxBackfill > 50 {
		maxBackfill = 50
	}

	pendingChunks, err := idx.store.GetPendingEmbeddings(maxBackfill)
	if err != nil {
		slog.Error("rag: failed to fetch pending embeddings", slog.String("err", err.Error()))
		return
	}
	if len(pendingChunks) == 0 {
		return
	}

	// Batch embed all pending chunk texts
	texts := make([]string, len(pendingChunks))
	for i, p := range pendingChunks {
		texts[i] = p.ChunkText
	}

	embeddings, err := idx.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		slog.Warn("rag: backfill embedding failed", slog.String("err", err.Error()))
		return
	}

	// Update each chunk with its embedding
	backfilled := 0
	for i, p := range pendingChunks {
		if embeddings[i] == nil {
			continue
		}
		if err := idx.store.UpdateEmbedding(p.ID, embeddings[i]); err != nil {
			slog.Error("rag: failed to backfill embedding",
				slog.Int64("chunk_id", p.ID),
				slog.String("err", err.Error()),
			)
			continue
		}
		backfilled++
	}

	slog.Info("rag: backfill complete",
		slog.Int("backfilled", backfilled),
		slog.Int("total_pending", pending),
	)
}

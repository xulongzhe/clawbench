package rag

import (
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"clawbench/internal/model"

	_ "github.com/marcboeker/go-duckdb"
)

// Store manages the DuckDB connection and vector storage operations.
type Store struct {
	db             *sql.DB
	dbPath         string
	duckdbOpts     map[string]string // Connection-time DuckDB options (for recovery re-open)
	embeddingDim   int               // adaptive embedding dimension
	ftsAvailable   bool              // Whether FTS extension loaded successfully
	ftsDirty       bool              // Whether FTS index needs rebuild after data changes
	ftsLastRebuild time.Time         // Last time FTS index was rebuilt (for debounce)
}

// Chunk represents a text chunk with its embedding and metadata.
type Chunk struct {
	ID                  int64     `json:"id"`
	SessionID           string    `json:"session_id"`
	MessageID           int64     `json:"message_id"`
	ChunkText           string    `json:"chunk_text"`
	ChunkTextSegmented  string    `json:"chunk_text_segmented"` // gse-segmented text for FTS
	ChunkIndex          int       `json:"chunk_index"`
	TokenCount          int       `json:"token_count"`
	Embedding           []float64 `json:"embedding"`
	HasEmbedding        bool      `json:"has_embedding"`
	ProjectPath         string    `json:"project_path"`
	Backend             string    `json:"backend"`
	Role                string    `json:"role"`
	CreatedAt           time.Time `json:"created_at"`
}

// SearchHit represents a search result with similarity score.
type SearchHit struct {
	ChunkID      int64     `json:"chunk_id"`
	ChunkText    string    `json:"chunk_text"`
	Score        float64   `json:"score"`
	SessionID    string    `json:"session_id"`
	SessionTitle string    `json:"session_title"`
	MessageID    int64     `json:"message_id"`
	Role         string    `json:"role"`
	ProjectPath  string    `json:"project_path"`
	Backend      string    `json:"backend"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewStore creates a new DuckDB store at the given path.
// DuckDB connection settings (threads, memory_limit) are applied before any
// query runs to prevent SIGFPE crashes on low-memory systems where DuckDB's
// internal thread-count computation can divide by zero. (ISS-155)
func NewStore(dbPath string, duckdbOpts map[string]string) (*Store, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create rag db directory: %w", err)
	}

	// Build DSN with connection-time configuration to prevent SIGFPE on
	// low-memory systems. DuckDB computes thread count from available memory;
	// on systems with <4 GB RAM this can yield 0 threads, causing a divide-
	// by-zero (SIGFPE, sigcode=FPE_INTDIV) inside duckdb_execute_pending.
	// Setting threads and memory_limit at connection time ensures the values
	// are applied before any query, including INSTALL/LOAD extensions.
	dsn := dbPath
	if len(duckdbOpts) > 0 {
		params := url.Values{}
		for k, v := range duckdbOpts {
			params.Set(k, v)
		}
		dsn = dbPath + "?" + params.Encode()
	}

	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	s := &Store{db: db, dbPath: dbPath, duckdbOpts: duckdbOpts}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init duckdb schema: %w", err)
	}

	// Load persisted embedding dimension from metadata
	s.loadEmbeddingDim()

	// Initialize FTS extension
	if err := s.initFTS(); err != nil {
		slog.Warn("rag: FTS extension not available, full-text search disabled", slog.String("err", err.Error()))
		// Continue without FTS — vector search still works
	} else {
		s.ftsAvailable = true
		// Build FTS index if there are existing chunks (for migrated databases)
		if count, _ := s.ChunkCount(); count > 0 {
			slog.Info("rag: building initial FTS index", slog.Int("chunks", count))
			if err := s.CreateFTSIndex(); err != nil {
				slog.Warn("rag: failed to create initial FTS index", slog.String("err", err.Error()))
			}
		}
	}

	return s, nil
}

// InitStore creates the RAG store using the standard .clawbench location.
// Applies conservative DuckDB resource limits (threads=1, memory_limit=512MB)
// by default to prevent SIGFPE crashes on low-memory systems. These can be
// overridden via the RAGConfig duckdb_threads and duckdb_memory_limit fields.
func InitStore(cfg model.RAGConfig) (*Store, error) {
	dbPath := filepath.Join(model.BinDir, ".clawbench", "rag.duckdb")

	// Default DuckDB connection settings for low-memory safety.
	opts := map[string]string{
		"threads":      "1",
		"memory_limit": "512MB",
	}

	// Allow user overrides from config
	if cfg.DuckDBThreads > 0 {
		opts["threads"] = strconv.Itoa(cfg.DuckDBThreads)
	}
	if cfg.DuckDBMemoryLimit != "" {
		opts["memory_limit"] = cfg.DuckDBMemoryLimit
	}

	slog.Info("rag: opening DuckDB store",
		slog.String("path", dbPath),
		slog.String("threads", opts["threads"]),
		slog.String("memory_limit", opts["memory_limit"]),
	)

	return NewStore(dbPath, opts)
}

func (s *Store) initSchema() error {
	// Create metadata table first
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS rag_metadata (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("create rag_metadata table: %w", err)
	}

	// Read persisted embedding dimension (default 1024 for backward compat)
	dim := s.readMetadataInt("embedding_dim", 1024)
	s.embeddingDim = dim

	dimSQL := fmt.Sprintf("FLOAT[%d]", dim)

	_, err = s.db.Exec(fmt.Sprintf(`
		CREATE SEQUENCE IF NOT EXISTS chat_chunks_id_seq;
		CREATE TABLE IF NOT EXISTS chat_chunks (
			id INTEGER PRIMARY KEY,
			session_id TEXT NOT NULL,
			message_id INTEGER NOT NULL,
			chunk_text TEXT NOT NULL,
			chunk_text_segmented TEXT,
			chunk_index INTEGER NOT NULL DEFAULT 0,
			token_count INTEGER NOT NULL,
			embedding %s,
			has_embedding BOOLEAN NOT NULL DEFAULT false,
			project_path TEXT NOT NULL,
			backend TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_chunks_session ON chat_chunks(session_id);
		CREATE INDEX IF NOT EXISTS idx_chunks_project ON chat_chunks(project_path);
		CREATE INDEX IF NOT EXISTS idx_chunks_created ON chat_chunks(created_at);
		CREATE INDEX IF NOT EXISTS idx_chunks_message ON chat_chunks(message_id);
	`, dimSQL))
	if err != nil {
		return err
	}

	// Migrate existing databases: add new columns if they don't exist
	s.db.Exec("ALTER TABLE chat_chunks ADD COLUMN chunk_text_segmented TEXT")
	// DuckDB does not support ADD COLUMN with NOT NULL DEFAULT constraints,
	// so add as nullable, then set default and backfill.
	if _, err := s.db.Exec("ALTER TABLE chat_chunks ADD COLUMN has_embedding BOOLEAN"); err == nil {
		// Column was just added (didn't exist before), set default value
		s.db.Exec("UPDATE chat_chunks SET has_embedding = false WHERE has_embedding IS NULL")
	}
	// Mark existing rows with embeddings as having embedding
	s.db.Exec("UPDATE chat_chunks SET has_embedding = true WHERE embedding IS NOT NULL AND (has_embedding IS NULL OR has_embedding = false)")

	// Sync sequence to max(id) so next insert doesn't violate primary key.
	// This can happen when the table was created with auto-increment IDs
	// but the sequence was never advanced past the existing rows.
	// DuckDB doesn't support ALTER SEQUENCE RESTART or subqueries in CREATE SEQUENCE,
	// so we query the max id and drop+recreate the sequence with the correct start value.
	var maxID int
	if err := s.db.QueryRow("SELECT COALESCE(MAX(id), 0) FROM chat_chunks").Scan(&maxID); err == nil && maxID > 0 {
		s.db.Exec("DROP SEQUENCE IF EXISTS chat_chunks_id_seq")
		s.db.Exec(fmt.Sprintf("CREATE SEQUENCE chat_chunks_id_seq START WITH %d", maxID+1))
	}

	return nil
}

// initFTS loads the DuckDB FTS extension.
func (s *Store) initFTS() error {
	_, err := s.db.Exec("INSTALL fts; LOAD fts;")
	if err != nil {
		return fmt.Errorf("load fts extension: %w", err)
	}
	return nil
}

// CreateFTSIndex creates or recreates the FTS index on chunk_text_segmented.
// Must be called after inserting chunks — FTS index does not auto-update.
func (s *Store) CreateFTSIndex() error {
	if !s.ftsAvailable {
		return fmt.Errorf("FTS not available")
	}
	// Drop existing index first (FTS doesn't auto-update)
	s.db.Exec("PRAGMA drop_fts_index('chat_chunks')")
	_, err := s.db.Exec(`
		PRAGMA create_fts_index(
			'chat_chunks', 'id', 'chunk_text_segmented',
			stemmer = 'none',
			stopwords = 'none',
			ignore = '(\.|[，。！？、；：“”‘’（）【】《》…—\-\d])+',
			lower = 1,
			strip_accents = 1
		)
	`)
	return err
}

// RebuildFTSIfDirty rebuilds the FTS index if data has changed since last rebuild.
// Debounced: skips rebuild if less than 30 seconds since last rebuild to avoid I/O spikes.
func (s *Store) RebuildFTSIfDirty() error {
	if !s.ftsAvailable || !s.ftsDirty {
		return nil
	}
	// Debounce: don't rebuild more often than every 30 seconds
	if time.Since(s.ftsLastRebuild) < 30*time.Second {
		return nil
	}
	if err := s.CreateFTSIndex(); err != nil {
		slog.Warn("rag: failed to rebuild FTS index", slog.String("err", err.Error()))
		return err
	}
	s.ftsDirty = false
	s.ftsLastRebuild = time.Now()
	return nil
}

// InsertChunks inserts multiple chunks into DuckDB.
// Uses sql.Exec with interpolated embedding literals for maximum compatibility
// (supports both nil and non-nil embeddings, avoids Appender column-order sensitivity).
// ID is auto-generated via the chat_chunks_id_seq sequence.
func (s *Store) InsertChunks(chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	for _, c := range chunks {
		var embeddingSQL string
		if c.Embedding != nil {
			esql, err := embeddingToSQLArray(c.Embedding, s.embeddingDim)
			if err != nil {
				return fmt.Errorf("embedding validation for chunk %d: %w", c.ID, err)
			}
			embeddingSQL = esql
		} else {
			embeddingSQL = "NULL"
		}

		sqlStr := fmt.Sprintf(`
			INSERT INTO chat_chunks (id, session_id, message_id, chunk_text, chunk_text_segmented,
				chunk_index, token_count, embedding, has_embedding, project_path, backend, role, created_at)
			VALUES (nextval('chat_chunks_id_seq'), ?, ?, ?, ?, ?, ?, %s, ?, ?, ?, ?, ?)`,
			embeddingSQL)
		_, err := s.db.Exec(sqlStr,
			c.SessionID, c.MessageID, c.ChunkText, c.ChunkTextSegmented,
			c.ChunkIndex, c.TokenCount,
			c.HasEmbedding,
			c.ProjectPath, c.Backend, c.Role, c.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert chunk (message_id=%d, chunk_index=%d): %w", c.MessageID, c.ChunkIndex, err)
		}
	}

	// Mark FTS as needing rebuild
	s.ftsDirty = true

	return nil
}

// SearchSimple performs vector similarity search without JOIN to SQLite
// (DuckDB cannot access SQLite directly). Session titles are fetched separately.
func (s *Store) SearchSimple(queryEmbedding []float64, limit int, projectPath, backend, role, sessionID, excludeSessionID, fromTime, toTime string) ([]SearchHit, error) {
	// Build embedding as SQL array literal since go-duckdb cannot bind []float64
	// as a parameter for FLOAT[dim] columns.
	embeddingLiteral, err := embeddingToSQLArray(queryEmbedding, s.embeddingDim)
	if err != nil {
		return nil, fmt.Errorf("query embedding validation: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id,
		       chunk_text,
		       array_cosine_similarity(embedding, %s) AS score,
		       session_id,
		       message_id,
		       role,
		       project_path,
		       backend,
		       created_at
		FROM chat_chunks
		WHERE has_embedding = true
	`, embeddingLiteral)
	args := []any{}

	if projectPath != "" {
		query += " AND project_path = ?"
		args = append(args, projectPath)
	}
	if backend != "" {
		query += " AND backend = ?"
		args = append(args, backend)
	}
	if role != "" {
		query += " AND role = ?"
		args = append(args, role)
	}
	if sessionID != "" {
		query += " AND session_id = ?"
		args = append(args, sessionID)
	}
	if excludeSessionID != "" {
		query += " AND session_id != ?"
		args = append(args, excludeSessionID)
	}
	if fromTime != "" {
		query += " AND created_at >= ?::TIMESTAMP"
		args = append(args, fromTime)
	}
	if toTime != "" {
		query += " AND created_at <= ?::TIMESTAMP"
		args = append(args, toTime)
	}

	query += " ORDER BY score DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.ChunkID, &h.ChunkText, &h.Score, &h.SessionID, &h.MessageID, &h.Role, &h.ProjectPath, &h.Backend, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan hit: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return hits, nil
}

// SearchFTS performs BM25 full-text search using DuckDB FTS.
func (s *Store) SearchFTS(queryText string, limit int, projectPath, backend, role, sessionID, excludeSessionID, fromTime, toTime string) ([]SearchHit, error) {
	if !s.ftsAvailable {
		return nil, fmt.Errorf("FTS not available")
	}

	// Segment the query for Chinese support
	segmentedQuery := SegmentText(queryText)

	query := `
		SELECT id,
		       chunk_text,
		       fts_main_chat_chunks.match_bm25(id, ?) AS score,
		       session_id,
		       message_id,
		       role,
		       project_path,
		       backend,
		       created_at
		FROM chat_chunks
		WHERE fts_main_chat_chunks.match_bm25(id, ?) IS NOT NULL
	`
	args := []any{segmentedQuery, segmentedQuery}

	if projectPath != "" {
		query += " AND project_path = ?"
		args = append(args, projectPath)
	}
	if backend != "" {
		query += " AND backend = ?"
		args = append(args, backend)
	}
	if role != "" {
		query += " AND role = ?"
		args = append(args, role)
	}
	if sessionID != "" {
		query += " AND session_id = ?"
		args = append(args, sessionID)
	}
	if excludeSessionID != "" {
		query += " AND session_id != ?"
		args = append(args, excludeSessionID)
	}
	if fromTime != "" {
		query += " AND created_at >= ?::TIMESTAMP"
		args = append(args, fromTime)
	}
	if toTime != "" {
		query += " AND created_at <= ?::TIMESTAMP"
		args = append(args, toTime)
	}

	query += " ORDER BY score DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fts search query: %w", err)
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.ChunkID, &h.ChunkText, &h.Score, &h.SessionID, &h.MessageID, &h.Role, &h.ProjectPath, &h.Backend, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan fts hit: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return hits, nil
}

// SearchHybrid performs hybrid vector + FTS search using Reciprocal Rank Fusion (RRF).
// poolSize is how many candidates each source returns before fusion.
func (s *Store) SearchHybrid(queryEmbedding []float64, queryText string, poolSize, limit int, projectPath, backend, role, sessionID, excludeSessionID, fromTime, toTime string) ([]SearchHit, error) {
	// Run both searches
	vecHits, vecErr := s.SearchSimple(queryEmbedding, poolSize, projectPath, backend, role, sessionID, excludeSessionID, fromTime, toTime)
	ftsHits, ftsErr := s.SearchFTS(queryText, poolSize, projectPath, backend, role, sessionID, excludeSessionID, fromTime, toTime)

	// If one source fails completely, fall back to the other
	if vecErr != nil && ftsErr != nil {
		return nil, fmt.Errorf("both search sources failed: vector=%v, fts=%v", vecErr, ftsErr)
	}
	if vecErr != nil {
		return ftsHits, nil
	}
	if ftsErr != nil {
		return vecHits, nil
	}

	// RRF fusion: score = sum(1 / (k + rank_i)) for each source
	const k = 60 // standard RRF constant

	type rrfEntry struct {
		hit      SearchHit
		rrfScore float64
	}
	scores := make(map[int64]*rrfEntry) // keyed by chunk ID

	// Process vector results
	for rank, h := range vecHits {
		if _, ok := scores[h.ChunkID]; !ok {
			scores[h.ChunkID] = &rrfEntry{hit: h}
		}
		scores[h.ChunkID].rrfScore += 1.0 / float64(k+rank+1)
	}

	// Process FTS results
	for rank, h := range ftsHits {
		if _, ok := scores[h.ChunkID]; !ok {
			scores[h.ChunkID] = &rrfEntry{hit: h}
		}
		scores[h.ChunkID].rrfScore += 1.0 / float64(k+rank+1)
	}

	// Sort by RRF score descending
	entries := make([]*rrfEntry, 0, len(scores))
	for _, e := range scores {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].rrfScore > entries[j].rrfScore
	})

	// Limit results
	if limit > len(entries) {
		limit = len(entries)
	}
	results := make([]SearchHit, limit)
	for i, e := range entries[:limit] {
		e.hit.Score = e.rrfScore // Replace original score with RRF score
		results[i] = e.hit
	}
	return results, nil
}

// PendingEmbeddingCount returns the number of chunks that need embedding backfill.
func (s *Store) PendingEmbeddingCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chat_chunks WHERE COALESCE(has_embedding, false) = false").Scan(&count)
	return count, err
}

// GetPendingEmbeddings returns chunks that need embedding backfill.
type PendingChunk struct {
	ID        int64
	ChunkText string
}

// GetPendingEmbeddings returns chunk IDs and texts that need embedding backfill.
func (s *Store) GetPendingEmbeddings(limit int) ([]PendingChunk, error) {
	rows, err := s.db.Query("SELECT id, chunk_text FROM chat_chunks WHERE COALESCE(has_embedding, false) = false ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pending []PendingChunk
	for rows.Next() {
		var p PendingChunk
		if err := rows.Scan(&p.ID, &p.ChunkText); err != nil {
			return nil, err
		}
		pending = append(pending, p)
	}
	return pending, rows.Err()
}

// UpdateEmbedding updates the embedding for a specific chunk (for backfill).
// Uses DELETE+INSERT instead of UPDATE to work around a DuckDB v1.1.3 bug where
// UPDATE on FLOAT[dim] columns spuriously triggers "Duplicate key" constraint errors.
// Preserves the original row ID.
// Note: DELETE+INSERT must happen on the same connection outside a transaction,
// because DuckDB v1.1.3 also triggers the same spurious "Duplicate key" error
// when INSERT happens inside a transaction after DELETE.
func (s *Store) UpdateEmbedding(chunkID int64, embedding []float64) error {
	// Read the existing row
	var c Chunk
	err := s.db.QueryRow(`
		SELECT id, session_id, message_id, chunk_text, chunk_text_segmented,
			chunk_index, token_count, has_embedding,
			project_path, backend, role, created_at
		FROM chat_chunks WHERE id = ?`, chunkID).Scan(
		&c.ID, &c.SessionID, &c.MessageID, &c.ChunkText, &c.ChunkTextSegmented,
		&c.ChunkIndex, &c.TokenCount, &c.HasEmbedding,
		&c.ProjectPath, &c.Backend, &c.Role, &c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("read chunk for backfill: %w", err)
	}

	// Delete the old row
	_, err = s.db.Exec("DELETE FROM chat_chunks WHERE id = ?", chunkID)
	if err != nil {
		return fmt.Errorf("delete chunk for backfill: %w", err)
	}

	// Re-insert with the SAME id and embedding
	embeddingLiteral, err := embeddingToSQLArray(embedding, s.embeddingDim)
	if err != nil {
		return fmt.Errorf("embedding validation for update: %w", err)
	}
	_, err = s.db.Exec(fmt.Sprintf(`
		INSERT INTO chat_chunks (id, session_id, message_id, chunk_text, chunk_text_segmented,
			chunk_index, token_count, embedding, has_embedding, project_path, backend, role, created_at)
		VALUES (%d, ?, ?, ?, ?, ?, ?, %s, ?, ?, ?, ?, ?)`,
		chunkID, embeddingLiteral),
		c.SessionID, c.MessageID, c.ChunkText, c.ChunkTextSegmented,
		c.ChunkIndex, c.TokenCount,
		true, // has_embedding = true now
		c.ProjectPath, c.Backend, c.Role, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("re-insert chunk for backfill: %w", err)
	}

	s.ftsDirty = true
	return nil
}

// CheckDimensionMismatch checks if existing embeddings have a different dimension
// than the store's configured dimension. Returns the existing dimension (0 if no data)
// and whether there is a mismatch.
func (s *Store) CheckDimensionMismatch() (int, bool, error) {
	var dim int
	err := s.db.QueryRow(`
		SELECT CASE
			WHEN COUNT(*) = 0 THEN 0
			ELSE ANY_VALUE(array_length(embedding))
		END
		FROM chat_chunks
		WHERE has_embedding = true
	`).Scan(&dim)
	if err != nil {
		return 0, false, fmt.Errorf("check dimension: %w", err)
	}
	if dim == 0 {
		return 0, false, nil
	}
	return dim, dim != s.embeddingDim, nil
}

// SetEmbeddingDim persists a new embedding dimension to metadata.
// Returns true if the dimension changed (i.e. there was a mismatch).
func (s *Store) SetEmbeddingDim(dim int) bool {
	if dim == s.embeddingDim {
		return false
	}
	s.embeddingDim = dim
	s.writeMetadata("embedding_dim", strconv.Itoa(dim))
	return true
}

// loadEmbeddingDim reads the persisted embedding dimension from metadata.
func (s *Store) loadEmbeddingDim() {
	dim := s.readMetadataInt("embedding_dim", 0)
	if dim > 0 {
		s.embeddingDim = dim
		slog.Info("rag: loaded embedding dimension from metadata", slog.Int("dim", dim))
	}
}

// readMetadata reads a string value from rag_metadata.
func (s *Store) readMetadata(key string) string {
	var value string
	err := s.db.QueryRow("SELECT value FROM rag_metadata WHERE key = ?", key).Scan(&value)
	if err != nil {
		return ""
	}
	return value
}

// readMetadataInt reads an integer value from rag_metadata, returning fallback if missing.
func (s *Store) readMetadataInt(key string, fallback int) int {
	val := s.readMetadata(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

// writeMetadata persists a key-value pair to rag_metadata.
func (s *Store) writeMetadata(key, value string) {
	_, err := s.db.Exec("INSERT INTO rag_metadata (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = ?", key, value, value)
	if err != nil {
		slog.Warn("rag: failed to write metadata", slog.String("key", key), slog.String("err", err.Error()))
	}
}

// ResetTable drops and recreates the chat_chunks table.
// Used when embedding dimension changes after a model switch.
func (s *Store) ResetTable() error {
	_, err := s.db.Exec("DROP TABLE IF EXISTS chat_chunks")
	if err != nil {
		return err
	}
	s.ftsDirty = true
	return s.initSchema()
}

// ChunkCount returns the total number of chunks in the store.
func (s *Store) ChunkCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chat_chunks").Scan(&count)
	return count, err
}

// DeleteChunksBySessionIDs deletes all chunks belonging to the given session IDs.
// Returns the total number of deleted chunks.
func (s *Store) DeleteChunksBySessionIDs(sessionIDs []string) (int64, error) {
	if len(sessionIDs) == 0 {
		return 0, nil
	}

	// Build placeholders for IN clause
	placeholders := ""
	args := make([]any, len(sessionIDs))
	for i, id := range sessionIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = id
	}

	result, err := s.db.Exec("DELETE FROM chat_chunks WHERE session_id IN ("+placeholders+")", args...)
	if err != nil {
		return 0, fmt.Errorf("delete chunks by session ids: %w", err)
	}
	affected, _ := result.RowsAffected()

	// Mark FTS as needing rebuild
	s.ftsDirty = true

	return affected, nil
}

// Close closes the DuckDB connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// RecoverFromCorruption attempts to recover from a corrupted DuckDB file
// by deleting it and recreating from scratch.
func (s *Store) RecoverFromCorruption() error {
	s.db.Close()
	slog.Warn("deleting corrupted rag.duckdb for recovery", slog.String("path", s.dbPath))
	if err := os.Remove(s.dbPath); err != nil {
		return fmt.Errorf("remove corrupted db: %w", err)
	}
	// Re-open with the same DuckDB connection options (threads, memory_limit)
	// to prevent SIGFPE on low-memory systems.
	dsn := s.dbPath
	if len(s.duckdbOpts) > 0 {
		params := url.Values{}
		for k, v := range s.duckdbOpts {
			params.Set(k, v)
		}
		dsn = s.dbPath + "?" + params.Encode()
	}
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return fmt.Errorf("reopen duckdb: %w", err)
	}
	s.db = db
	return s.initSchema()
}

// embeddingToSQLArray converts a float64 slice to a DuckDB array literal string.
// e.g., [0.1, 0.2, 0.3] -> "array[0.1, 0.2, 0.3]::FLOAT[dim]"
// Returns error if any value is non-finite (NaN/Inf) to prevent SQL injection. (ISS-130)
func embeddingToSQLArray(vec []float64, dim int) (string, error) {
	var buf strings.Builder
	buf.WriteString("array[")
	for i, v := range vec {
		if i > 0 {
			buf.WriteString(", ")
		}
		// Guard against NaN/Inf which produce invalid SQL float literals (ISS-130)
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return "", fmt.Errorf("embedding contains non-finite value at index %d: %v", i, v)
		}
		buf.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	}
	buf.WriteString("]::FLOAT[")
	buf.WriteString(strconv.Itoa(dim))
	buf.WriteString("]")
	return buf.String(), nil
}

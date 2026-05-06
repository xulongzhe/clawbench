package rag

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"clawbench/internal/model"

	_ "github.com/marcboeker/go-duckdb"
)

// Store manages the DuckDB connection and vector storage operations.
type Store struct {
	db     *sql.DB
	dbPath string
}

// Chunk represents a text chunk with its embedding and metadata.
type Chunk struct {
	ID          int64     `json:"id"`
	SessionID   string    `json:"session_id"`
	MessageID   int64     `json:"message_id"`
	ChunkText   string    `json:"chunk_text"`
	ChunkIndex  int       `json:"chunk_index"`
	TokenCount  int       `json:"token_count"`
	Embedding   []float64 `json:"embedding"`
	ProjectPath string    `json:"project_path"`
	Backend     string    `json:"backend"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// SearchHit represents a search result with similarity score.
type SearchHit struct {
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
func NewStore(dbPath string) (*Store, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create rag db directory: %w", err)
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	s := &Store{db: db, dbPath: dbPath}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init duckdb schema: %w", err)
	}

	return s, nil
}

// InitStore creates the RAG store using the standard .clawbench location.
func InitStore() (*Store, error) {
	dbPath := filepath.Join(model.BinDir, ".clawbench", "rag.duckdb")
	return NewStore(dbPath)
}

func (s *Store) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_chunks (
			id INTEGER PRIMARY KEY,
			session_id TEXT NOT NULL,
			message_id INTEGER NOT NULL,
			chunk_text TEXT NOT NULL,
			chunk_index INTEGER NOT NULL DEFAULT 0,
			token_count INTEGER NOT NULL,
			embedding FLOAT[1024],
			project_path TEXT NOT NULL,
			backend TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_chunks_session ON chat_chunks(session_id);
		CREATE INDEX IF NOT EXISTS idx_chunks_project ON chat_chunks(project_path);
		CREATE INDEX IF NOT EXISTS idx_chunks_created ON chat_chunks(created_at);
		CREATE INDEX IF NOT EXISTS idx_chunks_message ON chat_chunks(message_id);
	`)
	return err
}

// InsertChunks inserts multiple chunks into DuckDB within a transaction.
func (s *Store) InsertChunks(chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO chat_chunks (session_id, message_id, chunk_text, chunk_index, token_count, embedding, project_path, backend, role, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, c := range chunks {
		_, err := stmt.Exec(c.SessionID, c.MessageID, c.ChunkText, c.ChunkIndex, c.TokenCount, c.Embedding, c.ProjectPath, c.Backend, c.Role, c.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert chunk (message_id=%d, chunk_index=%d): %w", c.MessageID, c.ChunkIndex, err)
		}
	}

	return tx.Commit()
}

// SearchSimple performs vector similarity search without JOIN to SQLite
// (DuckDB cannot access SQLite directly). Session titles are fetched separately.
// sessionID limits results to that session; excludeSessionID excludes that session.
func (s *Store) SearchSimple(queryEmbedding []float64, limit int, projectPath, backend, sessionID, excludeSessionID, fromTime, toTime string) ([]SearchHit, error) {
	query := `
		SELECT chunk_text,
		       array_cosine_similarity(embedding, ?::FLOAT[1024]) AS score,
		       session_id,
		       message_id,
		       role,
		       project_path,
		       backend,
		       created_at
		FROM chat_chunks
		WHERE embedding IS NOT NULL
	`
	args := []any{queryEmbedding}

	if projectPath != "" {
		query += " AND project_path = ?"
		args = append(args, projectPath)
	}
	if backend != "" {
		query += " AND backend = ?"
		args = append(args, backend)
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
		query += " AND created_at >= ?"
		args = append(args, fromTime)
	}
	if toTime != "" {
		query += " AND created_at <= ?"
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
		if err := rows.Scan(&h.ChunkText, &h.Score, &h.SessionID, &h.MessageID, &h.Role, &h.ProjectPath, &h.Backend, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan hit: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return hits, nil
}

// CheckDimensionMismatch checks if existing embeddings have a different dimension
// than the expected dimension. Returns the existing dimension (0 if no data) and
// whether there is a mismatch.
func (s *Store) CheckDimensionMismatch(expectedDim int) (int, bool, error) {
	var dim int
	err := s.db.QueryRow(`
		SELECT CASE
			WHEN COUNT(*) = 0 THEN 0
			ELSE array_length(embedding)
		END
		FROM chat_chunks
		LIMIT 1
	`).Scan(&dim)
	if err != nil {
		return 0, false, fmt.Errorf("check dimension: %w", err)
	}
	if dim == 0 {
		return 0, false, nil
	}
	return dim, dim != expectedDim, nil
}

// ResetTable drops and recreates the chat_chunks table.
// Used when embedding dimension changes after a model switch.
func (s *Store) ResetTable() error {
	_, err := s.db.Exec("DROP TABLE IF EXISTS chat_chunks")
	if err != nil {
		return err
	}
	return s.initSchema()
}

// ChunkCount returns the total number of chunks in the store.
func (s *Store) ChunkCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chat_chunks").Scan(&count)
	return count, err
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
	db, err := sql.Open("duckdb", s.dbPath)
	if err != nil {
		return fmt.Errorf("reopen duckdb: %w", err)
	}
	s.db = db
	return s.initSchema()
}

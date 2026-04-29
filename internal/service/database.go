package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"clawbench/internal/model"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// InitDB initializes the SQLite database with latest schema.
func InitDB() error {
	dbDir := filepath.Join(model.BinDir, ".clawbench")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	// Dev mode uses a separate database to avoid data conflicts
	dbName := "ClawBench.db"
	if model.DevMode {
		dbName = "ClawBench-dev.db"
	}
	dbPath := filepath.Join(dbDir, dbName)
	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite concurrency: single connection + WAL mode + busy timeout
	DB.SetMaxOpenConns(1)

	// Enable WAL mode for concurrent reads during writes
	if _, err := DB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to set WAL mode: %w", err)
	}
	// Wait up to 5 seconds when database is locked instead of failing immediately
	if _, err := DB.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	// Create tables with latest schema
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS chat_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_path TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
			content TEXT NOT NULL,
			file_path TEXT,
			files TEXT,
			session_id TEXT,
			backend TEXT NOT NULL DEFAULT 'claude',
			streaming INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS chat_sessions (
			id TEXT PRIMARY KEY,
			project_path TEXT NOT NULL,
			backend TEXT NOT NULL,
			title TEXT NOT NULL,
			agent_id TEXT DEFAULT '',
			model TEXT DEFAULT '',
			last_read_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(project_path, backend, id)
		);
		CREATE TABLE IF NOT EXISTS recent_projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_path TEXT UNIQUE NOT NULL,
			accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id TEXT PRIMARY KEY,
			project_path TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			cron_expr TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			session_id TEXT DEFAULT '',
			status TEXT DEFAULT 'active',
			repeat_mode TEXT DEFAULT 'unlimited',
			max_runs INTEGER DEFAULT 0,
			last_run_at DATETIME,
			next_run_at DATETIME,
			run_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS task_executions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL,
			message_id INTEGER NOT NULL REFERENCES chat_history(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS ai_raw_responses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			message_id INTEGER NOT NULL REFERENCES chat_history(id),
			backend TEXT NOT NULL DEFAULT '',
			raw_output TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- Create indexes for efficient queries
		CREATE INDEX IF NOT EXISTS idx_executions_task ON task_executions(task_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_history_session ON chat_history(project_path, backend, session_id, created_at);
		CREATE INDEX IF NOT EXISTS idx_sessions_project_backend ON chat_sessions(project_path, backend);
		CREATE INDEX IF NOT EXISTS idx_raw_responses_session ON ai_raw_responses(session_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_raw_responses_message ON ai_raw_responses(message_id);
	`)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Add external_session_id column if it doesn't exist (for OpenCode backend session mapping)
	var hasExternalSessionID int
	row := DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('chat_sessions') WHERE name = 'external_session_id'")
	if err := row.Scan(&hasExternalSessionID); err != nil {
		return fmt.Errorf("failed to check external_session_id column: %w", err)
	}
	if hasExternalSessionID == 0 {
		if _, err := DB.Exec("ALTER TABLE chat_sessions ADD COLUMN external_session_id TEXT DEFAULT ''"); err != nil {
			return fmt.Errorf("failed to add external_session_id column: %w", err)
		}
	}

	// Clean up orphaned streaming messages from previous crashes/restarts.
	// Any message with streaming=1 at startup can never be finalized since
	// its stream no longer exists. Mark them as cancelled so the UI shows
	// an interrupted state instead of silently completing.
	rows, err := DB.Query("SELECT id, content FROM chat_history WHERE streaming = 1")
	if err != nil {
		return fmt.Errorf("failed to query orphaned streaming messages: %w", err)
	}
	type orphanMsg struct {
		id      int64
		content string
	}
	var orphans []orphanMsg
	for rows.Next() {
		var m orphanMsg
		if err := rows.Scan(&m.id, &m.content); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan orphaned streaming message: %w", err)
		}
		orphans = append(orphans, m)
	}
	rows.Close()

	for _, m := range orphans {
		var contentMap map[string]any
		if err := json.Unmarshal([]byte(m.content), &contentMap); err != nil {
			// Non-JSON content — wrap it
			contentMap = map[string]any{
				"blocks":    []any{map[string]any{"type": "text", "text": m.content}},
				"cancelled": true,
			}
		} else {
			contentMap["cancelled"] = true
			// Append warning block
			blocks, _ := contentMap["blocks"].([]any)
			blocks = append(blocks, map[string]any{
				"type": "warning",
				"text": "服务重启，AI 响应中断",
			})
			contentMap["blocks"] = blocks
		}
		updatedContent, _ := json.Marshal(contentMap)
		if _, err := DB.Exec("UPDATE chat_history SET content = ?, streaming = 0 WHERE id = ?", string(updatedContent), m.id); err != nil {
			slog.Error("failed to finalize orphaned streaming message", slog.Int64("id", m.id), slog.String("err", err.Error()))
		}
	}
	if len(orphans) > 0 {
		slog.Info("cleaned up orphaned streaming messages", slog.Int("count", len(orphans)))
	}

	return nil
}

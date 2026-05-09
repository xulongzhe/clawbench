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
// When runFromServer is true (server startup), orphaned streaming messages
// from previous crashes are cleaned up. When false (CLI subcommand), cleanup
// is skipped because the server process may still be actively streaming.
func InitDB(runFromServer ...bool) error {
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
			files TEXT,
			session_id TEXT,
			backend TEXT NOT NULL DEFAULT 'claude',
			streaming INTEGER NOT NULL DEFAULT 0,
			indexed INTEGER NOT NULL DEFAULT 0,
			deleted INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS chat_sessions (
			id TEXT PRIMARY KEY,
			project_path TEXT NOT NULL,
			backend TEXT NOT NULL,
			title TEXT NOT NULL,
			agent_id TEXT DEFAULT '',
			agent_source TEXT DEFAULT 'default',
			model TEXT DEFAULT '',
			external_session_id TEXT DEFAULT '',
			deleted INTEGER NOT NULL DEFAULT 0,
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
			last_read_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS task_executions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			trigger_type TEXT NOT NULL DEFAULT 'auto',
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

		CREATE TABLE IF NOT EXISTS tts_summaries (
			cache_key TEXT PRIMARY KEY,
			summary TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS forwarded_ports (
			port INTEGER PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			protocol TEXT NOT NULL DEFAULT 'http',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS terminal_quick_commands (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			command TEXT NOT NULL,
			hidden INTEGER NOT NULL DEFAULT 0,
			auto_execute INTEGER NOT NULL DEFAULT 0,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE UNIQUE INDEX IF NOT EXISTS idx_quick_commands_auto_execute
			ON terminal_quick_commands(auto_execute) WHERE auto_execute = 1;

		CREATE TABLE IF NOT EXISTS chat_quick_send (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			command TEXT NOT NULL,
			hidden INTEGER NOT NULL DEFAULT 0,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Clean up orphaned streaming messages from previous crashes/restarts.
	// Any message with streaming=1 at startup can never be finalized since
	// its stream no longer exists. Mark them as cancelled so the UI shows
	// an interrupted state instead of silently completing.
	// SKIP when called from CLI subcommands (task/rag) — the server process
	// may still be actively streaming, and these are NOT orphaned messages.
	isServerStartup := len(runFromServer) > 0 && runFromServer[0]
	if isServerStartup {
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
					"type":   "warning",
					"text":   "Server restarted, AI response interrupted",
					"reason": "restart",
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
	}

	return nil
}

// GetTTSSummary looks up a cached TTS summary by cache key.
// Returns (summary, found).
func GetTTSSummary(cacheKey string) (string, bool) {
	var summary string
	err := DB.QueryRow(
		"SELECT summary FROM tts_summaries WHERE cache_key = ?",
		cacheKey,
	).Scan(&summary)
	if err != nil {
		return "", false
	}
	return summary, true
}

// SaveTTSSummary persists a TTS summary to the database.
func SaveTTSSummary(cacheKey, summary string) error {
	_, err := DB.Exec(
		"INSERT OR REPLACE INTO tts_summaries (cache_key, summary, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)",
		cacheKey, summary,
	)
	return err
}

// QuickCommand represents a terminal quick command stored in the database.
type QuickCommand struct {
	ID          int64  `json:"id"`
	Label       string `json:"label"`
	Command     string `json:"command"`
	Hidden      bool   `json:"hidden"`
	AutoExecute bool   `json:"auto_execute"`
	SortOrder   int    `json:"sort_order"`
}

// GetQuickCommands returns all quick commands ordered by sort_order.
func GetQuickCommands() ([]QuickCommand, error) {
	rows, err := DB.Query("SELECT id, label, command, hidden, auto_execute, sort_order FROM terminal_quick_commands ORDER BY sort_order")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cmds []QuickCommand
	for rows.Next() {
		var cmd QuickCommand
		var hidden, autoExec int
		if err := rows.Scan(&cmd.ID, &cmd.Label, &cmd.Command, &hidden, &autoExec, &cmd.SortOrder); err != nil {
			return nil, err
		}
		cmd.Hidden = hidden == 1
		cmd.AutoExecute = autoExec == 1
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

// AddQuickCommand inserts a new quick command and returns its ID.
// If autoExecute is true, other commands' auto_execute flag is cleared first.
func AddQuickCommand(label, command string, hidden, autoExecute bool) (int64, error) {
	if autoExecute {
		if _, err := DB.Exec("UPDATE terminal_quick_commands SET auto_execute = 0 WHERE auto_execute = 1"); err != nil {
			return 0, err
		}
	}
	var maxOrder sql.NullInt64
	_ = DB.QueryRow("SELECT MAX(sort_order) FROM terminal_quick_commands").Scan(&maxOrder)
	sortOrder := 0
	if maxOrder.Valid {
		sortOrder = int(maxOrder.Int64) + 1
	}
	hiddenInt := 0
	if hidden {
		hiddenInt = 1
	}
	autoExecInt := 0
	if autoExecute {
		autoExecInt = 1
	}
	result, err := DB.Exec(
		"INSERT INTO terminal_quick_commands (label, command, hidden, auto_execute, sort_order) VALUES (?, ?, ?, ?, ?)",
		label, command, hiddenInt, autoExecInt, sortOrder,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateQuickCommand updates an existing quick command.
// If autoExecute is true, other commands' auto_execute flag is cleared first.
func UpdateQuickCommand(id int64, label, command string, hidden, autoExecute bool) error {
	if autoExecute {
		if _, err := DB.Exec("UPDATE terminal_quick_commands SET auto_execute = 0 WHERE auto_execute = 1 AND id != ?", id); err != nil {
			return err
		}
	}
	hiddenInt := 0
	if hidden {
		hiddenInt = 1
	}
	autoExecInt := 0
	if autoExecute {
		autoExecInt = 1
	}
	_, err := DB.Exec(
		"UPDATE terminal_quick_commands SET label = ?, command = ?, hidden = ?, auto_execute = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		label, command, hiddenInt, autoExecInt, id,
	)
	return err
}

// DeleteQuickCommand deletes a quick command by ID.
func DeleteQuickCommand(id int64) error {
	_, err := DB.Exec("DELETE FROM terminal_quick_commands WHERE id = ?", id)
	return err
}

// ReorderQuickCommands updates sort_order for all commands based on the given ID order.
func ReorderQuickCommands(ids []int64) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := tx.Exec("UPDATE terminal_quick_commands SET sort_order = ? WHERE id = ?", i, id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ChatQuickSendItem represents a chat quick-send item stored in the database.
type ChatQuickSendItem struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	Command   string `json:"command"`
	Hidden    bool   `json:"hidden"`
	SortOrder int    `json:"sort_order"`
}

// GetChatQuickSend returns all quick-send items ordered by sort_order.
func GetChatQuickSend() ([]ChatQuickSendItem, error) {
	rows, err := DB.Query("SELECT id, label, command, hidden, sort_order FROM chat_quick_send ORDER BY sort_order")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ChatQuickSendItem
	for rows.Next() {
		var item ChatQuickSendItem
		var hidden int
		if err := rows.Scan(&item.ID, &item.Label, &item.Command, &hidden, &item.SortOrder); err != nil {
			return nil, err
		}
		item.Hidden = hidden == 1
		items = append(items, item)
	}
	return items, nil
}

// AddChatQuickSend inserts a new quick-send item and returns its ID.
func AddChatQuickSend(label, command string, hidden bool) (int64, error) {
	var maxOrder sql.NullInt64
	_ = DB.QueryRow("SELECT MAX(sort_order) FROM chat_quick_send").Scan(&maxOrder)
	sortOrder := 0
	if maxOrder.Valid {
		sortOrder = int(maxOrder.Int64) + 1
	}
	hiddenInt := 0
	if hidden {
		hiddenInt = 1
	}
	result, err := DB.Exec(
		"INSERT INTO chat_quick_send (label, command, hidden, sort_order) VALUES (?, ?, ?, ?)",
		label, command, hiddenInt, sortOrder,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateChatQuickSend updates an existing quick-send item.
func UpdateChatQuickSend(id int64, label, command string, hidden bool) error {
	hiddenInt := 0
	if hidden {
		hiddenInt = 1
	}
	_, err := DB.Exec(
		"UPDATE chat_quick_send SET label = ?, command = ?, hidden = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		label, command, hiddenInt, id,
	)
	return err
}

// DeleteChatQuickSend deletes a quick-send item by ID.
func DeleteChatQuickSend(id int64) error {
	_, err := DB.Exec("DELETE FROM chat_quick_send WHERE id = ?", id)
	return err
}

// ReorderChatQuickSend updates sort_order for all items based on the given ID order.
func ReorderChatQuickSend(ids []int64) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := tx.Exec("UPDATE chat_quick_send SET sort_order = ? WHERE id = ?", i, id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}


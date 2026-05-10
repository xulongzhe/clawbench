package terminal

import (
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

// ---------- NewManager ----------

func TestNewManager_ConfigMapping(t *testing.T) {
	cfg := model.TerminalConfig{
		Enabled:      true,
		IdleTimeout:  "5m",
		BufferLines:  500,
		MaxLineBytes: 32768,
		MaxBufferMB:  8,
		MaxSessions:  5,
	}
	m := NewManager(cfg, 20000)

	assert.True(t, m.IsEnabled())
	assert.Equal(t, 5, m.maxSessions)
	assert.Equal(t, TerminalConfig{
		Enabled:      true,
		IdleTimeout:  "5m",
		BufferLines:  500,
		MaxLineBytes: 32768,
		MaxBufferMB:  8,
		MaxSessions:  5,
	}, m.Config())
}

// ---------- IsEnabled ----------

func TestManager_IsEnabled_True(t *testing.T) {
	m := NewManager(model.TerminalConfig{Enabled: true}, 20000)
	assert.True(t, m.IsEnabled())
}

func TestManager_IsEnabled_False(t *testing.T) {
	m := NewManager(model.TerminalConfig{Enabled: false}, 20000)
	assert.False(t, m.IsEnabled())
}

// ---------- Config ----------

func TestManager_Config(t *testing.T) {
	cfg := model.TerminalConfig{
		Enabled:     true,
		IdleTimeout: "10m",
		BufferLines: 2000,
		MaxSessions: 10,
	}
	m := NewManager(cfg, 30000)
	result := m.Config()
	assert.Equal(t, true, result.Enabled)
	assert.Equal(t, "10m", result.IdleTimeout)
	assert.Equal(t, 2000, result.BufferLines)
	assert.Equal(t, 10, result.MaxSessions)
}

// ---------- SessionStatus ----------

func TestManager_SessionStatus_NotFound(t *testing.T) {
	m := NewManager(model.TerminalConfig{Enabled: true}, 20000)
	found, cwd, running := m.SessionStatus("nonexistent")
	assert.False(t, found)
	assert.Empty(t, cwd)
	assert.False(t, running)
}

// ---------- AllSessionStatus ----------

func TestManager_AllSessionStatus_Empty(t *testing.T) {
	m := NewManager(model.TerminalConfig{Enabled: true}, 20000)
	statuses := m.AllSessionStatus()
	assert.Nil(t, statuses, "no sessions should return nil")
}

// ---------- Close ----------

func TestManager_Close(t *testing.T) {
	m := NewManager(model.TerminalConfig{Enabled: true}, 20000)
	m.Close() // should not panic with no sessions
}

// ---------- CloseSessionByID ----------

func TestManager_CloseSessionByID_NotFound(t *testing.T) {
	m := NewManager(model.TerminalConfig{Enabled: true}, 20000)
	// Closing a non-existent session should not panic
	m.CloseSessionByID("nonexistent")
}

// ---------- CloseAllSessions ----------

func TestManager_CloseAllSessions_Empty(t *testing.T) {
	m := NewManager(model.TerminalConfig{Enabled: true}, 20000)
	m.CloseAllSessions() // should not panic with no sessions
}

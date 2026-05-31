package terminal

import (
	"testing"
	"time"

	"clawbench/internal/model"
)

func TestResolveShell(t *testing.T) {
	shell := resolveShell()
	if shell == "" {
		t.Error("resolveShell() returned empty string")
	}
	t.Logf("resolved shell: %s", shell)
}

func TestNewSessionAndClose(t *testing.T) {
	// PTY fork may be restricted in sandboxed environments
	cfg := TerminalConfig{
		IdleTimeout:  "5s",
		BufferLines:  100,
		MaxLineBytes: 65536,
		MaxBufferMB:  4,
	}

	session, err := NewSession("/tmp", "/tmp", cfg)
	if err != nil {
		t.Skipf("PTY not available in this environment: %v", err)
	}
	defer func() { session.Close() }()

	if session.ProjectPath() != "/tmp" {
		t.Errorf("expected projectPath /tmp, got %s", session.ProjectPath())
	}
	if session.Cwd() != "/tmp" {
		t.Errorf("expected cwd /tmp, got %s", session.Cwd())
	}
	if session.ID() == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestSessionIdleTimeout(t *testing.T) {
	cfg := TerminalConfig{
		IdleTimeout:  "1s", // Very short timeout for testing
		BufferLines:  100,
		MaxLineBytes: 65536,
		MaxBufferMB:  4,
	}

	session, err := NewSession("/tmp", "/tmp", cfg)
	if err != nil {
		t.Skipf("PTY not available in this environment: %v", err)
	}
	// Don't defer Close — the idle timer will close it

	// Wait for idle timeout to fire
	time.Sleep(2 * time.Second)

	// Session should be closed now
	session.mu.Lock()
	closed := session.closed
	session.mu.Unlock()

	if !closed {
		t.Error("expected session to be closed after idle timeout")
	}
}

func TestManagerCloseAllSessions(t *testing.T) {
	cfg := model.TerminalConfig{
		Enabled:      true,
		IdleTimeout:  "10m",
		BufferLines:  100,
		MaxLineBytes: 65536,
		MaxBufferMB:  4,
	}

	mgr := NewManager(cfg, 20000)
	defer func() { mgr.Close() }()

	// Close with no active sessions should not panic
	mgr.CloseAllSessions()

	// AllSessionStatus should return empty
	sessions := mgr.AllSessionStatus()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestManagerClearsSessionAfterShellExit(t *testing.T) {
	cwd := t.TempDir()
	cfg := model.TerminalConfig{
		Enabled:      true,
		IdleTimeout:  "1m",
		BufferLines:  100,
		MaxLineBytes: 65536,
		MaxBufferMB:  4,
	}

	mgr := NewManager(cfg, 20000)
	defer func() { mgr.Close() }()

	session, err := NewSession(cwd, cwd, mgr.Config())
	if err != nil {
		t.Skipf("PTY not available in this environment: %v", err)
	}

	// Set up onClose callback like HandleWebSocket does
	sid := session.ID()
	session.onClose = func() {
		mgr.mu.Lock()
		delete(mgr.sessions, sid)
		mgr.mu.Unlock()
	}

	mgr.mu.Lock()
	mgr.sessions[sid] = session
	mgr.mu.Unlock()

	if err := session.HandleInput("exit\r"); err != nil {
		t.Fatalf("failed to send exit: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sessions := mgr.AllSessionStatus()
		if len(sessions) == 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	sessions := mgr.AllSessionStatus()
	t.Fatalf("expected manager to clear exited shell session, got %d sessions", len(sessions))
}

func TestManagerIsEnabled(t *testing.T) {
	cfg := model.TerminalConfig{
		Enabled:      true,
		IdleTimeout:  "10m",
		BufferLines:  100,
		MaxLineBytes: 65536,
		MaxBufferMB:  4,
	}

	mgr := NewManager(cfg, 20000)
	defer func() { mgr.Close() }()

	if !mgr.IsEnabled() {
		t.Error("expected terminal to be enabled")
	}

	disabledCfg := model.TerminalConfig{
		Enabled:      false,
		IdleTimeout:  "10m",
		BufferLines:  100,
		MaxLineBytes: 65536,
		MaxBufferMB:  4,
	}

	disabledMgr := NewManager(disabledCfg, 20000)
	defer func() { disabledMgr.Close() }()

	if disabledMgr.IsEnabled() {
		t.Error("expected terminal to be disabled")
	}
}

func TestManagerConfig(t *testing.T) {
	cfg := model.TerminalConfig{
		Enabled:      true,
		IdleTimeout:  "10m",
		BufferLines:  2000,
		MaxLineBytes: 65536,
		MaxBufferMB:  4,
	}

	mgr := NewManager(cfg, 20000)
	defer func() { mgr.Close() }()

	tc := mgr.Config()
	if !tc.Enabled {
		t.Error("expected enabled")
	}
	if tc.BufferLines != 2000 {
		t.Errorf("expected 2000 buffer lines, got %d", tc.BufferLines)
	}
}

func TestManagerMultipleSessions(t *testing.T) {
	cfg := model.TerminalConfig{
		Enabled:      true,
		IdleTimeout:  "10m",
		BufferLines:  100,
		MaxLineBytes: 65536,
		MaxBufferMB:  4,
		MaxSessions:  5,
	}

	mgr := NewManager(cfg, 20000)
	defer func() { mgr.Close() }()

	tc := mgr.Config()

	// Create multiple sessions
	ids := make(map[string]bool)
	for range 3 {
		session, err := NewSession("/tmp", "/tmp", tc)
		if err != nil {
			t.Skipf("PTY not available in this environment: %v", err)
		}
		sid := session.ID()
		if sid == "" {
			t.Error("expected non-empty session ID")
		}
		if ids[sid] {
			t.Errorf("duplicate session ID: %s", sid)
		}
		ids[sid] = true

		mgr.mu.Lock()
		mgr.sessions[sid] = session
		mgr.mu.Unlock()
	}

	sessions := mgr.AllSessionStatus()
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}

	// Close one by ID
	for sid := range ids {
		mgr.CloseSessionByID(sid)
		break // only close one
	}

	sessions = mgr.AllSessionStatus()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions after closing one, got %d", len(sessions))
	}
}

package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePresenceMap(t *testing.T) {
	raw := map[string]any{
		"port": 20000,
		"proxy": map[string]any{
			"enabled":       true,
			"allowed_ports": "1024-65535",
		},
		"ssh": map[string]any{
			"enabled": false,
		},
		"tts": map[string]any{
			"engine": "edge",
		},
	}

	presence := ParsePresenceMap(raw)

	expectedKeys := []string{
		"port",
		"proxy",
		"proxy.enabled",
		"proxy.allowed_ports",
		"ssh",
		"ssh.enabled",
		"tts",
		"tts.engine",
	}
	for _, key := range expectedKeys {
		if !presence[key] {
			t.Errorf("expected key %q to be present", key)
		}
	}

	// Keys that should NOT be present
	unexpectedKeys := []string{"password", "proxy.port", "ssh.port"}
	for _, key := range unexpectedKeys {
		if presence[key] {
			t.Errorf("expected key %q to NOT be present", key)
		}
	}
}

func TestParsePresenceMapEmpty(t *testing.T) {
	presence := ParsePresenceMap(nil)
	if len(presence) != 0 {
		t.Errorf("expected empty presence map, got %d keys", len(presence))
	}

	presence = ParsePresenceMap(map[string]any{})
	if len(presence) != 0 {
		t.Errorf("expected empty presence map, got %d keys", len(presence))
	}
}

func TestParsePresenceMapDeeplyNested(t *testing.T) {
	raw := map[string]any{
		"tts": map[string]any{
			"api": map[string]any{
				"base_url": "https://api.openai.com/v1/chat/completions",
			},
		},
	}
	presence := ParsePresenceMap(raw)

	expectedKeys := []string{"tts", "tts.api", "tts.api.base_url"}
	for _, key := range expectedKeys {
		if !presence[key] {
			t.Errorf("expected key %q to be present", key)
		}
	}
}

// setupTestBinDir creates a temp dir, sets BinDir, and returns a cleanup function.
func setupTestBinDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	origBinDir := BinDir
	BinDir = tmpDir
	t.Cleanup(func() { BinDir = origBinDir })
	return tmpDir
}

func TestApplyDefaultsEmptyConfig(t *testing.T) {
	tmpDir := setupTestBinDir(t)

	cfg := Config{}
	autoPassword := ApplyDefaults(&cfg, nil)

	// Should have auto-generated a password
	if autoPassword == "" {
		t.Error("expected auto-generated password for empty config")
	}
	if cfg.Password != autoPassword {
		t.Errorf("cfg.Password = %q, want %q", cfg.Password, autoPassword)
	}

	// Password should be persisted to file
	pwFile := filepath.Join(tmpDir, ".clawbench", "auto-password")
	data, err := os.ReadFile(pwFile)
	if err != nil {
		t.Errorf("auto-password file should exist: %v", err)
	} else if string(data) != autoPassword {
		t.Errorf("auto-password file = %q, want %q", string(data), autoPassword)
	}

	// Check all defaults
	if cfg.Port != 20000 {
		t.Errorf("Port = %d, want 20000", cfg.Port)
	}
	if cfg.WatchDir == "" {
		t.Error("WatchDir should not be empty")
	}
	if cfg.LogDir == "" {
		t.Error("LogDir should not be empty")
	}
	if cfg.LogMaxDays != 7 {
		t.Errorf("LogMaxDays = %d, want 7", cfg.LogMaxDays)
	}
	if cfg.Upload.MaxSizeMB != 100 {
		t.Errorf("Upload.MaxSizeMB = %d, want 100", cfg.Upload.MaxSizeMB)
	}
	if cfg.Upload.MaxFiles != 20 {
		t.Errorf("Upload.MaxFiles = %d, want 20", cfg.Upload.MaxFiles)
	}
	if cfg.Chat.InitialMessages != 20 {
		t.Errorf("Chat.InitialMessages = %d, want 20", cfg.Chat.InitialMessages)
	}
	if cfg.Chat.PageSize != 20 {
		t.Errorf("Chat.PageSize = %d, want 20", cfg.Chat.PageSize)
	}
	if cfg.Chat.CollapsedHeight != 150 {
		t.Errorf("Chat.CollapsedHeight = %d, want 150", cfg.Chat.CollapsedHeight)
	}
	if cfg.Session.MaxCount != 10 {
		t.Errorf("Session.MaxCount = %d, want 10", cfg.Session.MaxCount)
	}
	if !cfg.PortForward.Enabled {
		t.Error("PortForward.Enabled should default to true when not in config")
	}
	if cfg.PortForward.AllowedPorts != "1024-65535" {
		t.Errorf("PortForward.AllowedPorts = %q, want %q", cfg.PortForward.AllowedPorts, "1024-65535")
	}
	if cfg.TTS.Engine != "edge" {
		t.Errorf("TTS.Engine = %q, want %q", cfg.TTS.Engine, "edge")
	}
	if cfg.TTS.SummarizeBackend != "simple" {
		t.Errorf("TTS.SummarizeBackend = %q, want %q", cfg.TTS.SummarizeBackend, "simple")
	}
	if cfg.TTS.Speed != 1.0 {
		t.Errorf("TTS.Speed = %v, want 1.0", cfg.TTS.Speed)
	}
	if cfg.RAG.SearchPoolSize != 20 {
		t.Errorf("RAG.SearchPoolSize = %d, want 20", cfg.RAG.SearchPoolSize)
	}
}

func TestApplyDefaultsPartialConfig(t *testing.T) {
	setupTestBinDir(t)

	cfg := Config{
		Port:     3000,
		Password: "my-secret",
		Upload: struct {
			MaxSizeMB int `yaml:"max_size_mb"`
			MaxFiles  int `yaml:"max_files"`
		}{
			MaxSizeMB: 50,
		},
	}

	autoPassword := ApplyDefaults(&cfg, map[string]bool{"port": true, "password": true, "upload": true, "upload.max_size_mb": true})

	// Explicitly set values should be preserved
	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want 3000 (explicitly set)", cfg.Port)
	}
	if cfg.Password != "my-secret" {
		t.Errorf("Password = %q, want %q (explicitly set)", cfg.Password, "my-secret")
	}
	if autoPassword != "" {
		t.Errorf("autoPassword = %q, want empty (password was set)", autoPassword)
	}
	if cfg.Upload.MaxSizeMB != 50 {
		t.Errorf("Upload.MaxSizeMB = %d, want 50 (explicitly set)", cfg.Upload.MaxSizeMB)
	}

	// Unset values should get defaults
	if cfg.Upload.MaxFiles != 20 {
		t.Errorf("Upload.MaxFiles = %d, want 20 (default)", cfg.Upload.MaxFiles)
	}
}

func TestApplyDefaults_ProxyAllowedPortsMigratedToPortForward(t *testing.T) {
	cfg := Config{}
	presence := map[string]bool{
		"proxy":               true,
		"proxy.allowed_ports": true,
	}
	cfg.Proxy.AllowedPorts = "3000-4000"

	ApplyDefaults(&cfg, presence)

	if cfg.PortForward.AllowedPorts != "3000-4000" {
		t.Errorf("PortForward.AllowedPorts = %q, want %q (migrated from proxy)", cfg.PortForward.AllowedPorts, "3000-4000")
	}
}

func TestApplyDefaults_PortForwardAllowedPortsNotOverwritten(t *testing.T) {
	cfg := Config{}
	presence := map[string]bool{
		"port_forward":                true,
		"port_forward.allowed_ports":  true,
	}
	cfg.PortForward.AllowedPorts = "5000-6000"
	cfg.Proxy.AllowedPorts = "3000-4000"

	ApplyDefaults(&cfg, presence)

	if cfg.PortForward.AllowedPorts != "5000-6000" {
		t.Errorf("PortForward.AllowedPorts = %q, want %q (should not be overwritten)", cfg.PortForward.AllowedPorts, "5000-6000")
	}
}

func TestApplyDefaults_PortForwardAllowedPortsDefault(t *testing.T) {
	cfg := Config{}
	ApplyDefaults(&cfg, nil)

	if cfg.PortForward.AllowedPorts != "1024-65535" {
		t.Errorf("PortForward.AllowedPorts = %q, want %q (default)", cfg.PortForward.AllowedPorts, "1024-65535")
	}
}

func TestApplyDefaultsBoolPresencePortForwardEnabledFalse(t *testing.T) {
	cfg := Config{}
	presence := map[string]bool{
		"port_forward":         true,
		"port_forward.enabled": true,
	}
	cfg.PortForward.Enabled = false

	ApplyDefaults(&cfg, presence)

	if cfg.PortForward.Enabled {
		t.Error("PortForward.Enabled should stay false when explicitly set to false")
	}
}

func TestApplyDefaultsBoolPresencePortForwardNoSection(t *testing.T) {
	cfg := Config{}
	// No port_forward section at all in config
	ApplyDefaults(&cfg, nil)

	if !cfg.PortForward.Enabled {
		t.Error("PortForward.Enabled should default to true when port_forward section is absent from config")
	}
}

func TestApplyDefaultsPasswordAutoGenerated(t *testing.T) {
	tmpDir := setupTestBinDir(t)

	cfg1 := Config{}
	pwd1 := ApplyDefaults(&cfg1, nil)

	cfg2 := Config{}
	pwd2 := ApplyDefaults(&cfg2, nil)

	if pwd1 == "" || pwd2 == "" {
		t.Error("expected non-empty auto-generated passwords")
	}
	// Second call should reuse the saved password from file
	if pwd1 != pwd2 {
		t.Errorf("second call should reuse saved password: pwd1=%q, pwd2=%q", pwd1, pwd2)
	}

	// UUID format: 8-4-4-4-12 hex chars
	if len(pwd1) != 36 {
		t.Errorf("auto-generated password length = %d, want 36 (UUID format)", len(pwd1))
	}

	// Verify the auto-password file exists
	pwFile := filepath.Join(tmpDir, ".clawbench", "auto-password")
	if _, err := os.Stat(pwFile); err != nil {
		t.Errorf("auto-password file should exist: %v", err)
	}
}

func TestApplyDefaultsPasswordReuse(t *testing.T) {
	setupTestBinDir(t)

	// First call: generates and saves password
	cfg1 := Config{}
	pwd1 := ApplyDefaults(&cfg1, nil)

	// Second call with empty config should reuse saved password
	cfg2 := Config{}
	pwd2 := ApplyDefaults(&cfg2, nil)

	if pwd1 != pwd2 {
		t.Errorf("password should be reused across calls: pwd1=%q, pwd2=%q", pwd1, pwd2)
	}
	if cfg1.Password != cfg2.Password {
		t.Errorf("cfg.Password should match: cfg1=%q, cfg2=%q", cfg1.Password, cfg2.Password)
	}
}

func TestApplyDefaultsPasswordExplicitRemovesFile(t *testing.T) {
	tmpDir := setupTestBinDir(t)

	// First: auto-generate password (creates file)
	cfg1 := Config{}
	ApplyDefaults(&cfg1, nil)

	pwFile := filepath.Join(tmpDir, ".clawbench", "auto-password")
	if _, err := os.Stat(pwFile); err != nil {
		t.Fatalf("auto-password file should exist after first call: %v", err)
	}

	// Second: user explicitly sets password (should remove file)
	cfg2 := Config{Password: "my-explicit-password"}
	autoPwd := ApplyDefaults(&cfg2, map[string]bool{"password": true})

	if autoPwd != "" {
		t.Errorf("autoPassword should be empty when user sets password explicitly, got %q", autoPwd)
	}
	if _, err := os.Stat(pwFile); !os.IsNotExist(err) {
		t.Error("auto-password file should be removed when user sets password explicitly")
	}
}

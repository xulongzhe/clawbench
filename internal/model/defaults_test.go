package model

import (
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
			"ollama": map[string]any{
				"base_url": "http://localhost:11434",
			},
		},
	}
	presence := ParsePresenceMap(raw)

	expectedKeys := []string{"tts", "tts.ollama", "tts.ollama.base_url"}
	for _, key := range expectedKeys {
		if !presence[key] {
			t.Errorf("expected key %q to be present", key)
		}
	}
}

func TestApplyDefaultsEmptyConfig(t *testing.T) {
	// Save and restore BinDir
	origBinDir := BinDir
	BinDir = "/tmp/clawbench-test"
	defer func() { BinDir = origBinDir }()

	cfg := Config{}
	autoPassword := ApplyDefaults(&cfg, nil)

	// Should have auto-generated a password
	if autoPassword == "" {
		t.Error("expected auto-generated password for empty config")
	}
	if cfg.Password != autoPassword {
		t.Errorf("cfg.Password = %q, want %q", cfg.Password, autoPassword)
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
	if !cfg.Proxy.Enabled {
		t.Error("Proxy.Enabled should default to true when not in config")
	}
	if cfg.Proxy.AllowedPorts != "1024-65535" {
		t.Errorf("Proxy.AllowedPorts = %q, want %q", cfg.Proxy.AllowedPorts, "1024-65535")
	}
	if !cfg.SSH.Enabled {
		t.Error("SSH.Enabled should default to true when not in config")
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
}

func TestApplyDefaultsPartialConfig(t *testing.T) {
	origBinDir := BinDir
	BinDir = "/tmp/clawbench-test"
	defer func() { BinDir = origBinDir }()

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

func TestApplyDefaultsBoolPresenceProxyEnabledTrue(t *testing.T) {
	cfg := Config{}
	presence := map[string]bool{
		"proxy":          true,
		"proxy.enabled":  true,
		"proxy.allowed_ports": true,
	}
	// Set Enabled to true explicitly in config
	cfg.Proxy.Enabled = true

	ApplyDefaults(&cfg, presence)

	if !cfg.Proxy.Enabled {
		t.Error("Proxy.Enabled should stay true when explicitly set to true")
	}
}

func TestApplyDefaultsBoolPresenceProxyEnabledFalse(t *testing.T) {
	cfg := Config{}
	presence := map[string]bool{
		"proxy":          true,
		"proxy.enabled":  true,
	}
	// User explicitly wrote "enabled: false"
	cfg.Proxy.Enabled = false

	ApplyDefaults(&cfg, presence)

	if cfg.Proxy.Enabled {
		t.Error("Proxy.Enabled should stay false when explicitly set to false")
	}
}

func TestApplyDefaultsBoolPresenceProxySectionNoEnabled(t *testing.T) {
	cfg := Config{}
	// User wrote a proxy section but didn't include "enabled" key
	presence := map[string]bool{
		"proxy":               true,
		"proxy.allowed_ports": true,
	}

	ApplyDefaults(&cfg, presence)

	// When proxy section exists but enabled key is absent, should still default to true
	if !cfg.Proxy.Enabled {
		t.Error("Proxy.Enabled should default to true when proxy section exists but enabled key is absent")
	}
}

func TestApplyDefaultsBoolPresenceSSHEnabledFalse(t *testing.T) {
	cfg := Config{}
	presence := map[string]bool{
		"ssh":          true,
		"ssh.enabled":  true,
	}
	cfg.SSH.Enabled = false

	ApplyDefaults(&cfg, presence)

	if cfg.SSH.Enabled {
		t.Error("SSH.Enabled should stay false when explicitly set to false")
	}
}

func TestApplyDefaultsBoolPresenceSSHNoSection(t *testing.T) {
	cfg := Config{}
	// No ssh section at all in config
	ApplyDefaults(&cfg, nil)

	if !cfg.SSH.Enabled {
		t.Error("SSH.Enabled should default to true when ssh section is absent from config")
	}
}

func TestApplyDefaultsPasswordAutoGenerated(t *testing.T) {
	origBinDir := BinDir
	BinDir = "/tmp/clawbench-test"
	defer func() { BinDir = origBinDir }()

	cfg1 := Config{}
	pwd1 := ApplyDefaults(&cfg1, nil)

	cfg2 := Config{}
	pwd2 := ApplyDefaults(&cfg2, nil)

	if pwd1 == "" || pwd2 == "" {
		t.Error("expected non-empty auto-generated passwords")
	}
	if pwd1 == pwd2 {
		t.Error("two auto-generated passwords should be different (random)")
	}

	// UUID format: 8-4-4-4-12 hex chars
	if len(pwd1) != 36 {
		t.Errorf("auto-generated password length = %d, want 36 (UUID format)", len(pwd1))
	}
}

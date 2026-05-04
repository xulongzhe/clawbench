package model

import (
	"crypto/rand"
	"fmt"
	"path/filepath"

	"clawbench/internal/platform"
)

// ParsePresenceMap walks a raw YAML map and returns a flat set of dot-separated
// keys that were explicitly present. For example, given:
//
//	proxy:
//	  enabled: true
//	  allowed_ports: "1024-65535"
//
// It returns: {"proxy": true, "proxy.enabled": true, "proxy.allowed_ports": true}
func ParsePresenceMap(raw map[string]any) map[string]bool {
	presence := make(map[string]bool)
	walkPresenceMap(raw, "", presence)
	return presence
}

func walkPresenceMap(m map[string]any, prefix string, presence map[string]bool) {
	for key, val := range m {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		presence[fullKey] = true
		if nested, ok := val.(map[string]any); ok {
			walkPresenceMap(nested, fullKey, presence)
		}
	}
}

// ApplyDefaults fills zero-value fields in cfg with sensible defaults.
// presence indicates which keys were explicitly set in the config file,
// used to distinguish "user wrote enabled: false" from "user omitted the section".
// Returns the auto-generated password if one was created, empty string otherwise.
func ApplyDefaults(cfg *Config, presence map[string]bool) string {
	var autoPassword string

	// --- Server ---
	if cfg.Port <= 0 {
		cfg.Port = 20000
	}

	// --- WatchDir ---
	if cfg.WatchDir == "" {
		cfg.WatchDir = platform.UserHomeDir()
	}
	cfg.WatchDir = platform.ExpandTilde(cfg.WatchDir)

	// --- Password ---
	if cfg.Password == "" {
		b := make([]byte, 16)
		rand.Read(b)
		cfg.Password = fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
		autoPassword = cfg.Password
	}

	// --- LogDir ---
	if cfg.LogDir == "" {
		cfg.LogDir = filepath.Join(BinDir, ".clawbench", "logs")
	}
	cfg.LogDir = platform.ExpandTilde(cfg.LogDir)

	if cfg.LogMaxDays <= 0 {
		cfg.LogMaxDays = 7
	}

	// --- Upload ---
	if cfg.Upload.MaxSizeMB <= 0 {
		cfg.Upload.MaxSizeMB = 100
	}
	if cfg.Upload.MaxFiles <= 0 {
		cfg.Upload.MaxFiles = 20
	}

	// --- Chat ---
	if cfg.Chat.InitialMessages <= 0 {
		cfg.Chat.InitialMessages = 20
	}
	if cfg.Chat.PageSize <= 0 {
		cfg.Chat.PageSize = 20
	}
	if cfg.Chat.CollapsedHeight <= 0 {
		cfg.Chat.CollapsedHeight = 150
	}

	// --- Session ---
	if cfg.Session.MaxCount <= 0 {
		cfg.Session.MaxCount = 10
	}

	// --- Proxy ---
	// Bool zero-value trap: Go defaults bool to false, but we want true.
	// Only keep false if user explicitly wrote "enabled: false".
	// If proxy section is absent OR proxy.enabled key is absent, default to true.
	if !presence["proxy.enabled"] {
		cfg.Proxy.Enabled = true
	}
	if cfg.Proxy.AllowedPorts == "" {
		cfg.Proxy.AllowedPorts = "1024-65535"
	}

	// --- SSH ---
	// Same bool zero-value trap as Proxy.
	if !presence["ssh.enabled"] {
		cfg.SSH.Enabled = true
	}

	// --- TTS ---
	if cfg.TTS.Engine == "" {
		cfg.TTS.Engine = "edge"
	}
	if cfg.TTS.SummarizeBackend == "" {
		cfg.TTS.SummarizeBackend = "simple"
	}
	if cfg.TTS.Speed <= 0 {
		cfg.TTS.Speed = 1.0
	}

	return autoPassword
}

package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"clawbench/internal/model"
	"net/http"

	"gopkg.in/yaml.v3"
)

// configMutex protects ConfigInstance from concurrent PATCH writes.
var configMutex sync.Mutex

// configResponse is the sanitized config returned to clients via GET /api/config.
// It only contains fields safe for frontend display — no passwords, keys, or
// internal paths.
type configResponse struct {
	Chat     configChat     `json:"chat"`
	Session  configSession  `json:"session"`
	Upload   configUpload   `json:"upload"`
	Terminal configTerminal `json:"terminal"`
	TTS      configTTS      `json:"tts"`
	RAG      configRAG      `json:"rag"`
	Proxy    configProxy    `json:"proxy"`
	SSH      configSSH      `json:"ssh"`
	Push     configPush     `json:"push"`
}

type configChat struct {
	InitialMessages      int `json:"initial_messages"`
	PageSize             int `json:"page_size"`
	CollapsedHeight      int `json:"collapsed_height"`
	SystemPromptInterval int `json:"system_prompt_interval"`
}

type configSession struct {
	MaxCount int `json:"max_count"`
}

type configUpload struct {
	MaxSizeMB int `json:"max_size_mb"`
	MaxFiles  int `json:"max_files"`
}

type configTerminal struct {
	Enabled     bool   `json:"enabled"`
	IdleTimeout string `json:"idle_timeout"`
	MaxSessions int    `json:"max_sessions"`
	BufferLines int    `json:"buffer_lines"`
}

type configTTS struct {
	Engine           string  `json:"engine"`
	TTSModel         string  `json:"tts_model"`
	Format           string  `json:"format"`
	SummarizeBackend string  `json:"summarize_backend"`
	SummarizeModel   string  `json:"summarize_model"`
	Speed            float64 `json:"speed"`
	Voice            string  `json:"voice"`
	MaxCacheFiles    int     `json:"max_cache_files"`
}

type configRAG struct {
	Enabled       bool   `json:"enabled"`
	OllamaBaseURL string `json:"ollama_base_url"`
	OllamaModel   string `json:"ollama_model"`
	ChunkSize     int    `json:"chunk_size"`
	SearchLimit   int    `json:"search_limit"`
	RetentionDays int    `json:"retention_days"`
}

type configProxy struct {
	Enabled      bool   `json:"enabled"`
	AllowedPorts string `json:"allowed_ports"`
}

type configSSH struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

type configPush struct {
	JPush configJPush `json:"jpush"`
}

type configJPush struct {
	Enabled bool   `json:"enabled"`
	AppKey  string `json:"app_key"`
}

// PatchableConfigPaths defines the whitelist of config paths that PATCH /api/config accepts.
// Any path not in this list will be rejected with 400 Bad Request.
var PatchableConfigPaths = map[string]bool{
	"chat.initial_messages":       true,
	"chat.page_size":              true,
	"chat.collapsed_height":       true,
	"chat.system_prompt_interval": true,
	"session.max_count":           true,
	"upload.max_size_mb":          true,
	"upload.max_files":            true,
	"terminal.enabled":            true,
	"terminal.idle_timeout":       true,
	"terminal.max_sessions":       true,
	"terminal.buffer_lines":       true,
	"tts.engine":                  true,
	"tts.tts_model":               true,
	"tts.format":                  true,
	"tts.summarize_backend":       true,
	"tts.summarize_model":        true,
	"tts.speed":                   true,
	"tts.voice":                   true,
	"tts.max_cache_files":         true,
	"rag.enabled":                 true,
	"rag.ollama_base_url":         true,
	"rag.ollama_model":            true,
	"rag.chunk_size":              true,
	"rag.search_limit":            true,
	"rag.retention_days":          true,
	"proxy.enabled":               true,
	"proxy.allowed_ports":         true,
	"ssh.enabled":                 true,
	"ssh.port":                    true,
	"push.jpush.enabled":          true,
	"push.jpush.app_key":          true,
}

// validTTSEngines is the set of valid TTS engine values.
var validTTSEngines = map[string]bool{
	"edge": true, "minimax": true, "piper": true, "kokoro": true, "moss-nano": true,
}

// validSummarizeBackends is the set of valid TTS summarization backend values.
var validSummarizeBackends = map[string]bool{
	"simple": true, "api": true,
	"claude": true, "codebuddy": true, "gemini": true,
	"opencode": true, "codex": true, "qoder": true,
	"vecli": true, "deepseek": true, "pi": true,
	"mmx-cli": true,
}

// validTTSFormats is the set of valid TTS output format values.
var validTTSFormats = map[string]bool{
	"": true, "mp3": true, "wav": true, "pcm": true,
}

// ServeConfig handles GET and PATCH /api/config.
func ServeConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		serveConfigGet(w, r)
	case http.MethodPatch:
		serveConfigPatch(w, r)
	default:
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

func serveConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg := model.ConfigInstance
	resp := configResponse{
		Chat: configChat{
			InitialMessages:      cfg.Chat.InitialMessages,
			PageSize:             cfg.Chat.PageSize,
			CollapsedHeight:      cfg.Chat.CollapsedHeight,
			SystemPromptInterval: cfg.Chat.SystemPromptInterval,
		},
		Session: configSession{
			MaxCount: cfg.Session.MaxCount,
		},
		Upload: configUpload{
			MaxSizeMB: cfg.Upload.MaxSizeMB,
			MaxFiles:  cfg.Upload.MaxFiles,
		},
		Terminal: configTerminal{
			Enabled:     cfg.Terminal.Enabled,
			IdleTimeout: cfg.Terminal.IdleTimeout,
			MaxSessions: cfg.Terminal.MaxSessions,
			BufferLines: cfg.Terminal.BufferLines,
		},
		TTS: configTTS{
			Engine:           cfg.TTS.Engine,
			TTSModel:         cfg.TTS.TTSModel,
			Format:           cfg.TTS.Format,
			SummarizeBackend: cfg.TTS.SummarizeBackend,
			SummarizeModel:   cfg.TTS.SummarizeModel,
			Speed:            cfg.TTS.Speed,
			Voice:            cfg.TTS.Voice,
			MaxCacheFiles:    cfg.TTS.MaxCacheFiles,
		},
		RAG: configRAG{
			Enabled:       cfg.RAG.Enabled,
			OllamaBaseURL: cfg.RAG.OllamaBaseURL,
			OllamaModel:   cfg.RAG.OllamaModel,
			ChunkSize:     cfg.RAG.ChunkSize,
			SearchLimit:   cfg.RAG.SearchLimit,
			RetentionDays: cfg.RAG.RetentionDays,
		},
		Proxy: configProxy{
			Enabled:      cfg.Proxy.Enabled,
			AllowedPorts: cfg.Proxy.AllowedPorts,
		},
		SSH: configSSH{
			Enabled: cfg.SSH.Enabled,
			Port:    cfg.SSH.Port,
		},
		Push: configPush{
			JPush: configJPush{
				Enabled: cfg.Push.JPush.Enabled,
				AppKey:  cfg.Push.JPush.AppKey,
			},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

func serveConfigPatch(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRequest")
		return
	}

	// Parse as generic map to validate fields against whitelist
	var patch map[string]any
	if err := json.Unmarshal(body, &patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_json",
			"message": "failed to parse request body as JSON",
		})
		return
	}

	// Validate all fields against whitelist
	changedFields, err := validatePatchFields(patch, "")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "forbidden_field",
			"message": err.Error(),
		})
		return
	}

	// Validate field values
	if err := validatePatchValues(patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_value",
			"message": err.Error(),
		})
		return
	}

	// Acquire lock for config mutation
	configMutex.Lock()
	defer configMutex.Unlock()

	// Apply patch to ConfigInstance
	if err := applyConfigPatch(patch); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "apply_failed",
			"message": err.Error(),
		})
		return
	}

	// Write config.yaml atomically
	if err := writeConfigYAML(); err != nil {
		slog.Error("failed to write config.yaml after patch", "err", err)
		// Config is already applied in memory, yaml write failure is non-fatal
		// but we should inform the client
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "write_failed",
			"message": fmt.Sprintf("failed to write config.yaml: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"needs_restart":        true,
		"changed_cold_fields":  changedFields,
	})
}

// validatePatchFields recursively validates that all paths in the patch are in the whitelist.
// Returns the list of dot-separated paths that were found.
func validatePatchFields(patch map[string]any, prefix string) ([]string, error) {
	var fields []string
	for key, value := range patch {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		// Check if this is a nested object
		if nested, ok := value.(map[string]any); ok {
			// Recurse into nested objects
			nestedFields, err := validatePatchFields(nested, path)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nestedFields...)
		} else {
			// Leaf value — check whitelist
			if !PatchableConfigPaths[path] {
				return nil, fmt.Errorf("field '%s' is not allowed", path)
			}
			fields = append(fields, path)
		}
	}
	return fields, nil
}

// validatePatchValues validates field values that have enum constraints.
func validatePatchValues(patch map[string]any) error {
	// Extract TTS section
	tts, ok := patch["tts"].(map[string]any)
	if ok {
		if engine, ok := tts["engine"].(string); ok {
			if !validTTSEngines[engine] {
				return fmt.Errorf("tts.engine must be one of: edge,minimax,piper,kokoro,moss-nano")
			}
		}
		if backend, ok := tts["summarize_backend"].(string); ok {
			if !validSummarizeBackends[backend] {
				return fmt.Errorf("tts.summarize_backend must be one of: simple,api,claude,codebuddy,gemini,opencode,codex,qoder,vecli,deepseek,pi,mmx-cli")
			}
		}
		if format, ok := tts["format"].(string); ok {
			if !validTTSFormats[format] {
				return fmt.Errorf("tts.format must be one of: mp3,wav,pcm")
			}
		}
		if speed, ok := tts["speed"].(float64); ok {
			if speed < 0.5 || speed > 3.0 {
				return fmt.Errorf("tts.speed must be between 0.5 and 3.0")
			}
		}
	}

	// Validate non-negative integers
	chat, ok := patch["chat"].(map[string]any)
	if ok {
		for _, key := range []string{"collapsed_height", "initial_messages", "page_size", "system_prompt_interval"} {
			if v, ok := chat[key].(float64); ok && v < 0 {
				return fmt.Errorf("chat.%s must be non-negative", key)
			}
		}
	}
	session, ok := patch["session"].(map[string]any)
	if ok {
		if v, ok := session["max_count"].(float64); ok && v < 0 {
			return fmt.Errorf("session.max_count must be non-negative")
		}
	}
	upload, ok := patch["upload"].(map[string]any)
	if ok {
		if v, ok := upload["max_size_mb"].(float64); ok && v < 0 {
			return fmt.Errorf("upload.max_size_mb must be non-negative")
		}
		if v, ok := upload["max_files"].(float64); ok && v < 0 {
			return fmt.Errorf("upload.max_files must be non-negative")
		}
	}

	return nil
}

// applyConfigPatch applies the patch to ConfigInstance in memory.
func applyConfigPatch(patch map[string]any) error {
	cfg := &model.ConfigInstance

	if chat, ok := patch["chat"].(map[string]any); ok {
		if v, ok := chat["collapsed_height"].(float64); ok {
			cfg.Chat.CollapsedHeight = int(v)
		}
		if v, ok := chat["initial_messages"].(float64); ok {
			cfg.Chat.InitialMessages = int(v)
		}
		if v, ok := chat["page_size"].(float64); ok {
			cfg.Chat.PageSize = int(v)
		}
		if v, ok := chat["system_prompt_interval"].(float64); ok {
			cfg.Chat.SystemPromptInterval = int(v)
		}
	}

	if session, ok := patch["session"].(map[string]any); ok {
		if v, ok := session["max_count"].(float64); ok {
			cfg.Session.MaxCount = int(v)
		}
	}

	if upload, ok := patch["upload"].(map[string]any); ok {
		if v, ok := upload["max_size_mb"].(float64); ok {
			cfg.Upload.MaxSizeMB = int(v)
		}
		if v, ok := upload["max_files"].(float64); ok {
			cfg.Upload.MaxFiles = int(v)
		}
	}

	if terminal, ok := patch["terminal"].(map[string]any); ok {
		if v, ok := terminal["enabled"].(bool); ok {
			cfg.Terminal.Enabled = v
		}
		if v, ok := terminal["idle_timeout"].(string); ok {
			cfg.Terminal.IdleTimeout = v
		}
		if v, ok := terminal["max_sessions"].(float64); ok {
			cfg.Terminal.MaxSessions = int(v)
		}
		if v, ok := terminal["buffer_lines"].(float64); ok {
			cfg.Terminal.BufferLines = int(v)
		}
	}

	if tts, ok := patch["tts"].(map[string]any); ok {
		if v, ok := tts["engine"].(string); ok {
			cfg.TTS.Engine = v
		}
		if v, ok := tts["tts_model"].(string); ok {
			cfg.TTS.TTSModel = v
		}
		if v, ok := tts["format"].(string); ok {
			cfg.TTS.Format = v
		}
		if v, ok := tts["summarize_backend"].(string); ok {
			cfg.TTS.SummarizeBackend = v
		}
		if v, ok := tts["summarize_model"].(string); ok {
			cfg.TTS.SummarizeModel = v
		}
		if v, ok := tts["speed"].(float64); ok {
			cfg.TTS.Speed = v
		}
		if v, ok := tts["voice"].(string); ok {
			cfg.TTS.Voice = v
		}
		if v, ok := tts["max_cache_files"].(float64); ok {
			cfg.TTS.MaxCacheFiles = int(v)
		}
	}

	if rag, ok := patch["rag"].(map[string]any); ok {
		if v, ok := rag["enabled"].(bool); ok {
			cfg.RAG.Enabled = v
		}
		if v, ok := rag["ollama_base_url"].(string); ok {
			cfg.RAG.OllamaBaseURL = v
		}
		if v, ok := rag["ollama_model"].(string); ok {
			cfg.RAG.OllamaModel = v
		}
		if v, ok := rag["chunk_size"].(float64); ok {
			cfg.RAG.ChunkSize = int(v)
		}
		if v, ok := rag["search_limit"].(float64); ok {
			cfg.RAG.SearchLimit = int(v)
		}
		if v, ok := rag["retention_days"].(float64); ok {
			cfg.RAG.RetentionDays = int(v)
		}
	}

	if proxy, ok := patch["proxy"].(map[string]any); ok {
		if v, ok := proxy["enabled"].(bool); ok {
			cfg.Proxy.Enabled = v
		}
		if v, ok := proxy["allowed_ports"].(string); ok {
			cfg.Proxy.AllowedPorts = v
		}
	}

	if ssh, ok := patch["ssh"].(map[string]any); ok {
		if v, ok := ssh["enabled"].(bool); ok {
			cfg.SSH.Enabled = v
		}
		if v, ok := ssh["port"].(float64); ok {
			cfg.SSH.Port = int(v)
		}
	}

	if push, ok := patch["push"].(map[string]any); ok {
		if jpush, ok := push["jpush"].(map[string]any); ok {
			if v, ok := jpush["enabled"].(bool); ok {
				cfg.Push.JPush.Enabled = v
			}
			if v, ok := jpush["app_key"].(string); ok {
				cfg.Push.JPush.AppKey = v
			}
		}
	}

	// Also update global variables for hot-reloadable fields
	model.ChatCollapsedHeight = cfg.Chat.CollapsedHeight
	model.ChatInitialMessages = cfg.Chat.InitialMessages
	model.ChatPageSize = cfg.Chat.PageSize
	model.ChatSystemPromptInterval = cfg.Chat.SystemPromptInterval
	model.SessionMaxCount = cfg.Session.MaxCount
	model.UploadMaxSizeMB = cfg.Upload.MaxSizeMB
	model.UploadMaxFiles = cfg.Upload.MaxFiles
	model.TTSMaxCacheFiles = cfg.TTS.MaxCacheFiles

	return nil
}

// writeConfigYAML writes the current ConfigInstance to config/config.yaml atomically.
func writeConfigYAML() error {
	configDir := filepath.Join(model.BinDir, "config")
	configPath := filepath.Join(configDir, "config.yaml")
	tmpPath := configPath + ".tmp"
	bakPath := configPath + ".bak"

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Backup existing config.yaml if it exists
	if _, err := os.Stat(configPath); err == nil {
		if err := copyFile(configPath, bakPath); err != nil {
			slog.Warn("failed to backup config.yaml", "err", err)
			// Non-fatal — continue without backup
		}
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(&model.ConfigInstance)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to temp file
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath) // cleanup temp file
		return fmt.Errorf("failed to rename config file: %w", err)
	}

	return nil
}

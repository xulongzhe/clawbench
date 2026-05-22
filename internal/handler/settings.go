package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/version"
	"net/http"

	"gopkg.in/yaml.v3"
)

// configMutex protects ConfigInstance from concurrent access.
// PATCH acquires a full lock; GET acquires a read lock to allow concurrent reads.
var configMutex sync.RWMutex

// hotReloadFields is the set of config dot-paths that take effect immediately
// via applyHotReloadGlobals() and do NOT require a server restart.
var hotReloadFields = map[string]bool{
	"chat.collapsed_height":      true,
	"chat.initial_messages":      true,
	"chat.page_size":             true,
	"chat.system_prompt_interval": true,
	"session.max_count":          true,
	"upload.max_size_mb":        true,
	"upload.max_files":          true,
	"tts.max_cache_files":       true,
}

// restartGracePeriod is the delay before shutting down the server after a restart
// request, giving the HTTP response time to reach the client.
const restartGracePeriod = 200 * time.Millisecond

// restartFunc is the function called to trigger a server restart.
// Set by main.go via SetRestartFunc(). Defaults to a no-op for tests.
var restartFunc func()

// SetRestartFunc sets the function called to trigger a server restart.
// main.go calls this to wire up the actual shutdown+sentinel logic.
func SetRestartFunc(f func()) {
	restartFunc = f
}

// configResponse is the sanitized config returned to clients via GET /api/config.
// It only contains fields safe for frontend display — no passwords, keys, or
// internal paths.
type configResponse struct {
	Version       string         `json:"version"`
	DefaultAgent  string         `json:"default_agent"`
	Chat          configChat     `json:"chat"`
	Session       configSession  `json:"session"`
	Upload        configUpload   `json:"upload"`
	Terminal      configTerminal `json:"terminal"`
	TTS           configTTS      `json:"tts"`
	RAG           configRAG      `json:"rag"`
	PortForward    configPortForward `json:"port_forward"`
	Push          configPush     `json:"push"`
	Tasks         configTasks    `json:"tasks"`
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
	Engine           string          `json:"engine"`
	TTSModel         string          `json:"tts_model"`
	Format           string          `json:"format"`
	SummarizeBackend string          `json:"summarize_backend"`
	SummarizeModel   string          `json:"summarize_model"`
	Speed            float64         `json:"speed"`
	Voice            string          `json:"voice"`
	MaxCacheFiles    int             `json:"max_cache_files"`
	Piper            *configPiper    `json:"piper,omitempty"`
	Kokoro           *configKokoro   `json:"kokoro,omitempty"`
	MossNano         *configMossNano `json:"moss_nano,omitempty"`
	API              *configAPI     `json:"api,omitempty"`
}

type configPiper struct {
	ModelPath       string  `json:"model_path"`
	NoiseScale      float64 `json:"noise_scale"`
	LengthScale     float64 `json:"length_scale"`
	SentenceSilence float64 `json:"sentence_silence"`
}

type configKokoro struct {
	ModelPath  string `json:"model_path"`
	VoicesPath string `json:"voices_path"`
	Lang       string `json:"lang"`
}

type configMossNano struct {
	ModelDir     string `json:"model_dir"`
	PromptSpeech string `json:"prompt_speech"`
	Voice        string `json:"voice"`
	Backend      string `json:"backend"`
}

type configAPI struct {
	BaseURL string `json:"base_url"`
	Key     string `json:"key"`
	Format  string `json:"format"`
}

type configRAG struct {
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	APIKey         string `json:"api_key"`
	ChunkSize      int    `json:"chunk_size"`
	SearchLimit    int    `json:"search_limit"`
	SearchPoolSize int    `json:"search_pool_size"`
	RetentionDays  int    `json:"retention_days"`
}

type configPortForward struct {
	Enabled      bool   `json:"enabled"`
	Port         int    `json:"port"`
	AllowedPorts string `json:"allowed_ports"`
}

type configPush struct {
	JPush configJPush `json:"jpush"`
}

type configJPush struct {
	Enabled      bool   `json:"enabled"`
	AppKey       string `json:"app_key"`
	MasterSecret string `json:"master_secret"`
}

type configTasks struct {
	SummarizeBackend string `json:"summarize_backend"`
	SummarizeModel   string `json:"summarize_model"`
}

// PatchableConfigPaths defines the whitelist of config paths that PATCH /api/config accepts.
// Any path not in this list will be rejected with 400 Bad Request.
var PatchableConfigPaths = map[string]bool{
	"default_agent":                true,
	"chat.initial_messages":        true,
	"chat.page_size":               true,
	"chat.collapsed_height":        true,
	"chat.system_prompt_interval":  true,
	"session.max_count":            true,
	"upload.max_size_mb":           true,
	"upload.max_files":             true,
	"terminal.enabled":             true,
	"terminal.idle_timeout":        true,
	"terminal.max_sessions":        true,
	"terminal.buffer_lines":        true,
	"tts.engine":                   true,
	"tts.tts_model":                true,
	"tts.format":                   true,
	"tts.summarize_backend":        true,
	"tts.summarize_model":          true,
	"tts.speed":                    true,
	"tts.voice":                    true,
	"tts.max_cache_files":          true,
	"tts.piper.model_path":         true,
	"tts.piper.noise_scale":        true,
	"tts.piper.length_scale":       true,
	"tts.piper.sentence_silence":  true,
	"tts.kokoro.model_path":       true,
	"tts.kokoro.voices_path":      true,
	"tts.kokoro.lang":             true,
	"tts.moss_nano.model_dir":     true,
	"tts.moss_nano.prompt_speech": true,
	"tts.moss_nano.voice":         true,
	"tts.moss_nano.backend":       true,
	"tts.api.base_url":            true,
	"tts.api.key":                  true,
	"tts.api.format":              true,
	"tts.api.model":               true,
	"rag.base_url":              true,
	"rag.model":                 true,
	"rag.api_key":               true,
	"rag.chunk_size":            true,
	"rag.search_limit":          true,
	"rag.search_pool_size":      true,
	"rag.retention_days":        true,
	"port_forward.enabled":                 true,
	"port_forward.port":                    true,
	"port_forward.allowed_ports":           true,
	"push.jpush.enabled":          true,
	"push.jpush.app_key":          true,
	"push.jpush.master_secret":    true,
	"tasks.summarize_backend":     true,
	"tasks.summarize_model":       true,
}

// validTTSEngines is the set of valid TTS engine values.
var validTTSEngines = map[string]bool{
	"edge": true, "piper": true, "kokoro": true, "moss-nano": true,
}

// validSummarizeBackends is the set of valid TTS summarization backend values.
var validSummarizeBackends = map[string]bool{
	"simple": true, "api": true,
	"claude": true, "codebuddy": true, "gemini": true,
	"opencode": true, "codex": true, "qoder": true,
	"vecli": true, "deepseek": true, "pi": true,
}

// validTTSFormats is the set of valid TTS output format values.
// Empty string is accepted separately (means "use default"), not as a format value.
var validTTSFormats = map[string]bool{
	"mp3": true, "wav": true, "pcm": true,
}

// validAPIFormats is the set of valid API format values.
var validAPIFormats = map[string]bool{
	"openai": true, "anthropic": true,
}

// validMossNanoBackends is the set of valid MOSS-Nano inference backend values.
var validMossNanoBackends = map[string]bool{
	"onnx": true, "pytorch": true,
}

// getBuildVersion returns a human-readable version string from build info.
// When version.Version is set via -ldflags (from git describe), it combines
// the semantic version with the VCS short SHA, e.g. "v1.0.0 (abc1234)".
// getBuildVersion returns the version string injected at build time via -ldflags.
// For release builds (HEAD on a tag), the format is: "v1.0.0"
// For dev builds, the format is: "v0.30.0-33-ga636beb (2026-05-21 10:30:00)"
// When not set (bare "go build"), falls back to VCS info or "dev".
func getBuildVersion() string {
	if version.Version != "" {
		return version.Version
	}
	// Fallback: no ldflags injected (bare "go build") — use VCS info
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var vcsRev string
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			vcsRev = s.Value[:7]
		}
	}
	if vcsRev != "" {
		return vcsRev
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// maskAPIKey masks an API key for safe display: first 4 + *** + last 3 chars.
// Returns "****" if the key is too short (< 8 chars).
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) < 8 {
		return "****"
	}
	return key[:4] + "***" + key[len(key)-3:]
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
	configMutex.RLock()
	cfg := model.ConfigInstance
	configMutex.RUnlock()

	resp := configResponse{
		Version:      getBuildVersion(),
		DefaultAgent: cfg.DefaultAgent,
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
			BaseURL:        cfg.RAG.BaseURL,
			Model:          cfg.RAG.Model,
			APIKey:         maskAPIKey(cfg.RAG.APIKey),
			ChunkSize:      cfg.RAG.ChunkSize,
			SearchLimit:    cfg.RAG.SearchLimit,
			SearchPoolSize: cfg.RAG.SearchPoolSize,
			RetentionDays:  cfg.RAG.RetentionDays,
		},
		PortForward: configPortForward{
			Enabled:      cfg.PortForward.Enabled,
			Port:         cfg.PortForward.Port,
			AllowedPorts: cfg.PortForward.AllowedPorts,
		},
		Push: configPush{
			JPush: configJPush{
				Enabled:      cfg.Push.JPush.Enabled,
				AppKey:       cfg.Push.JPush.AppKey, // AppKey is not a secret, no need to mask
				MasterSecret: maskAPIKey(cfg.Push.JPush.MasterSecret),
			},
		},
		Tasks: configTasks{
			SummarizeBackend: cfg.Tasks.SummarizeBackend,
			SummarizeModel:   cfg.Tasks.SummarizeModel,
		},
	}

	// Conditionally populate engine-specific sub-configs
	switch cfg.TTS.Engine {
	case "piper":
		resp.TTS.Piper = &configPiper{
			ModelPath:       cfg.TTS.Piper.ModelPath,
			NoiseScale:      cfg.TTS.Piper.NoiseScale,
			LengthScale:     cfg.TTS.Piper.LengthScale,
			SentenceSilence: cfg.TTS.Piper.SentenceSilence,
		}
	case "kokoro":
		resp.TTS.Kokoro = &configKokoro{
			ModelPath:  cfg.TTS.Kokoro.ModelPath,
			VoicesPath: cfg.TTS.Kokoro.VoicesPath,
			Lang:       cfg.TTS.Kokoro.Lang,
		}
	case "moss-nano":
		resp.TTS.MossNano = &configMossNano{
			ModelDir:     cfg.TTS.MossNano.ModelDir,
			PromptSpeech: cfg.TTS.MossNano.PromptSpeech,
			Voice:        cfg.TTS.MossNano.Voice,
			Backend:      cfg.TTS.MossNano.Backend,
		}
	}

	// Conditionally populate API sub-config when summarize_backend is "api"
	if cfg.TTS.SummarizeBackend == "api" || cfg.Tasks.SummarizeBackend == "api" {
		resp.TTS.API = &configAPI{
			BaseURL: cfg.TTS.API.BaseURL,
			Key:     maskAPIKey(cfg.TTS.API.Key),
			Format:  cfg.TTS.API.Format,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func serveConfigPatch(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB max
	if err != nil {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRequest")
		return
	}

	var patch map[string]any
	if err := json.Unmarshal(body, &patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_json",
			"message": "failed to parse request body as JSON",
		})
		return
	}

	changedFields, err := validatePatchFields(patch, "")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "forbidden_field",
			"message": err.Error(),
		})
		return
	}

	if err := validatePatchValues(patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_value",
			"message": err.Error(),
		})
		return
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	// Snapshot current config for rollback on write failure
	snapshot := model.ConfigInstance

	if err := applyConfigPatch(patch); err != nil {
		model.ConfigInstance = snapshot
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "apply_failed",
			"message": err.Error(),
		})
		return
	}

	if err := writeConfigYAML(patch); err != nil {
		// Rollback memory state — disk write failed
		model.ConfigInstance = snapshot
		// Re-apply hot-reload globals from snapshot
		applyHotReloadGlobals()
		slog.Error("failed to write config.yaml after patch", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "write_failed",
			"message": fmt.Sprintf("failed to write config.yaml: %v", err),
		})
		return
	}

	// Sort changed fields for deterministic response
	sort.Strings(changedFields)

	// Classify fields into hot (no restart) and cold (needs restart)
	coldFields := make([]string, 0) // always non-nil so JSON serializes as [] not null
	for _, f := range changedFields {
		if !hotReloadFields[f] {
			coldFields = append(coldFields, f)
		}
	}
	sort.Strings(coldFields)

	writeJSON(w, http.StatusOK, map[string]any{
		"needs_restart":       len(coldFields) > 0,
		"changed_cold_fields": coldFields,
	})
}

func validatePatchFields(patch map[string]any, prefix string) ([]string, error) {
	var fields []string
	for key, value := range patch {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if nested, ok := value.(map[string]any); ok {
			nestedFields, err := validatePatchFields(nested, path)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nestedFields...)
		} else {
			if !PatchableConfigPaths[path] {
				return nil, fmt.Errorf("field '%s' is not allowed", path)
			}
			fields = append(fields, path)
		}
	}
	return fields, nil
}

func validatePatchValues(patch map[string]any) error {
	tts, ok := patch["tts"].(map[string]any)
	if ok {
		if engine, ok := tts["engine"].(string); ok {
			if !validTTSEngines[engine] {
				return fmt.Errorf("tts.engine must be one of: edge,piper,kokoro,moss-nano")
			}
		}
		if backend, ok := tts["summarize_backend"].(string); ok {
			if !validSummarizeBackends[backend] {
				return fmt.Errorf("tts.summarize_backend must be one of: simple,api,claude,codebuddy,gemini,opencode,codex,qoder,vecli,deepseek,pi")
			}
		}
		if format, ok := tts["format"].(string); ok {
			if format != "" && !validTTSFormats[format] {
				return fmt.Errorf("tts.format must be one of: mp3,wav,pcm")
			}
		}
		if speed, ok := tts["speed"].(float64); ok {
			if speed < 0.5 || speed > 3.0 {
				return fmt.Errorf("tts.speed must be between 0.5 and 3.0")
			}
		}
		// Validate engine-specific sub-configs
		if piper, ok := tts["piper"].(map[string]any); ok {
			if v, ok := piper["noise_scale"].(float64); ok && (v < 0 || v > 1) {
				return fmt.Errorf("tts.piper.noise_scale must be between 0 and 1")
			}
			if v, ok := piper["length_scale"].(float64); ok && v <= 0 {
				return fmt.Errorf("tts.piper.length_scale must be positive")
			}
			if v, ok := piper["sentence_silence"].(float64); ok && v < 0 {
				return fmt.Errorf("tts.piper.sentence_silence must be non-negative")
			}
		}
		if kokoro, ok := tts["kokoro"].(map[string]any); ok {
			if v, ok := kokoro["lang"].(string); ok && v == "" {
				return fmt.Errorf("tts.kokoro.lang must not be empty")
			}
		}
		if mossNano, ok := tts["moss_nano"].(map[string]any); ok {
			if v, ok := mossNano["backend"].(string); ok {
				if !validMossNanoBackends[v] {
					return fmt.Errorf("tts.moss_nano.backend must be one of: onnx,pytorch")
				}
			}
		}
		// Validate API sub-config
		if api, ok := tts["api"].(map[string]any); ok {
			if v, ok := api["format"].(string); ok {
				if !validAPIFormats[v] {
					return fmt.Errorf("tts.api.format must be one of: openai,anthropic")
				}
			}
			if v, ok := api["key"].(string); ok && strings.Contains(v, "***") {
				return fmt.Errorf("tts.api.key must not contain '***' — please provide the full key value")
			}
		}
	}

	// ── Cross-field consistency checks ──────────────────────────
	cfg := model.ConfigInstance

	// 1. When tts.summarize_backend or tasks.summarize_backend is "api",
	//    tts.api.base_url must not be empty.
	effectiveSummarizeBackend := cfg.TTS.SummarizeBackend
	if tts, ok := patch["tts"].(map[string]any); ok {
		if v, ok := tts["summarize_backend"].(string); ok {
			effectiveSummarizeBackend = v
		}
	}
	effectiveTasksBackend := cfg.Tasks.SummarizeBackend
	if tasks, ok := patch["tasks"].(map[string]any); ok {
		if v, ok := tasks["summarize_backend"].(string); ok {
			effectiveTasksBackend = v
		}
	}
	if effectiveSummarizeBackend == "api" || effectiveTasksBackend == "api" {
		effectiveBaseURL := cfg.TTS.API.BaseURL
		if tts, ok := patch["tts"].(map[string]any); ok {
			if api, ok := tts["api"].(map[string]any); ok {
				if v, ok := api["base_url"].(string); ok {
					effectiveBaseURL = v
				}
			}
		}
		if effectiveBaseURL == "" {
			return fmt.Errorf("tts.api.base_url is required when summarize_backend is \"api\"")
		}
	}

	// 2. Engine-specific model path requirements.
	// When the patch switches tts.engine to a new value, skip required-field
	// validation for the *target* engine — the user hasn't had a chance to
	// fill in the sub-config yet (frontend auto-saves one field at a time).
	// Only enforce when the engine is already set to that value (i.e. the
	// user is saving sub-config fields for the current engine).
	effectiveEngine := cfg.TTS.Engine
	engineSwitched := false
	if tts, ok := patch["tts"].(map[string]any); ok {
		if v, ok := tts["engine"].(string); ok && v != cfg.TTS.Engine {
			effectiveEngine = v
			engineSwitched = true
		}
	}
	switch effectiveEngine {
	case "piper":
		if engineSwitched {
			break // will be validated after user fills in sub-config
		}
		effectiveModelPath := cfg.TTS.Piper.ModelPath
		if tts, ok := patch["tts"].(map[string]any); ok {
			if piper, ok := tts["piper"].(map[string]any); ok {
				if v, ok := piper["model_path"].(string); ok {
					effectiveModelPath = v
				}
			}
		}
		if effectiveModelPath == "" {
			return fmt.Errorf("tts.piper.model_path is required when tts.engine is \"piper\"")
		}
	case "kokoro":
		if engineSwitched {
			break // will be validated after user fills in sub-config
		}
		effectiveKokoroModel := cfg.TTS.Kokoro.ModelPath
		effectiveVoicesPath := cfg.TTS.Kokoro.VoicesPath
		if tts, ok := patch["tts"].(map[string]any); ok {
			if kokoro, ok := tts["kokoro"].(map[string]any); ok {
				if v, ok := kokoro["model_path"].(string); ok {
					effectiveKokoroModel = v
				}
				if v, ok := kokoro["voices_path"].(string); ok {
					effectiveVoicesPath = v
				}
			}
		}
		if effectiveKokoroModel == "" {
			return fmt.Errorf("tts.kokoro.model_path is required when tts.engine is \"kokoro\"")
		}
		if effectiveVoicesPath == "" {
			return fmt.Errorf("tts.kokoro.voices_path is required when tts.engine is \"kokoro\"")
		}
	case "moss-nano":
		if engineSwitched {
			break // will be validated after user fills in sub-config
		}
		effectiveModelDir := cfg.TTS.MossNano.ModelDir
		if tts, ok := patch["tts"].(map[string]any); ok {
			if mossNano, ok := tts["moss_nano"].(map[string]any); ok {
				if v, ok := mossNano["model_dir"].(string); ok {
					effectiveModelDir = v
				}
			}
		}
		if effectiveModelDir == "" {
			return fmt.Errorf("tts.moss_nano.model_dir is required when tts.engine is \"moss-nano\"")
		}
	}

	// 3. default_agent must be an existing agent ID.
	if v, ok := patch["default_agent"].(string); ok && v != "" {
		if model.Agents != nil {
			if _, exists := model.Agents[v]; !exists {
				available := make([]string, 0, len(model.AgentList))
				for _, a := range model.AgentList {
					available = append(available, a.ID)
				}
				return fmt.Errorf("default_agent \"%s\" not found (available: %s)", v, strings.Join(available, ", "))
			}
		}
	}

	// Validate tasks section
	if tasks, ok := patch["tasks"].(map[string]any); ok {
		if v, ok := tasks["summarize_backend"].(string); ok && v != "" {
			if !validSummarizeBackends[v] {
				return fmt.Errorf("tasks.summarize_backend must be one of: simple,api,claude,codebuddy,gemini,opencode,codex,qoder,vecli,deepseek,pi")
			}
		}
	}

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

func applyConfigPatch(patch map[string]any) error {
	cfg := &model.ConfigInstance

	// Top-level fields
	if v, ok := patch["default_agent"].(string); ok {
		cfg.DefaultAgent = v
		model.DefaultAgentID = v
	}

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
		// Piper sub-config
		if piper, ok := tts["piper"].(map[string]any); ok {
			if v, ok := piper["model_path"].(string); ok {
				cfg.TTS.Piper.ModelPath = v
			}
			if v, ok := piper["noise_scale"].(float64); ok {
				cfg.TTS.Piper.NoiseScale = v
			}
			if v, ok := piper["length_scale"].(float64); ok {
				cfg.TTS.Piper.LengthScale = v
			}
			if v, ok := piper["sentence_silence"].(float64); ok {
				cfg.TTS.Piper.SentenceSilence = v
			}
		}
		// Kokoro sub-config
		if kokoro, ok := tts["kokoro"].(map[string]any); ok {
			if v, ok := kokoro["model_path"].(string); ok {
				cfg.TTS.Kokoro.ModelPath = v
			}
			if v, ok := kokoro["voices_path"].(string); ok {
				cfg.TTS.Kokoro.VoicesPath = v
			}
			if v, ok := kokoro["lang"].(string); ok {
				cfg.TTS.Kokoro.Lang = v
			}
		}
		// MossNano sub-config
		if mossNano, ok := tts["moss_nano"].(map[string]any); ok {
			if v, ok := mossNano["model_dir"].(string); ok {
				cfg.TTS.MossNano.ModelDir = v
			}
			if v, ok := mossNano["prompt_speech"].(string); ok {
				cfg.TTS.MossNano.PromptSpeech = v
			}
			if v, ok := mossNano["voice"].(string); ok {
				cfg.TTS.MossNano.Voice = v
			}
			if v, ok := mossNano["backend"].(string); ok {
				cfg.TTS.MossNano.Backend = v
			}
		}
		// API sub-config
		if api, ok := tts["api"].(map[string]any); ok {
			if v, ok := api["base_url"].(string); ok {
				cfg.TTS.API.BaseURL = v
			}
			if v, ok := api["key"].(string); ok {
				cfg.TTS.API.Key = v
			}
			if v, ok := api["format"].(string); ok {
				cfg.TTS.API.Format = v
			}
		}
	}

	if rag, ok := patch["rag"].(map[string]any); ok {
		if v, ok := rag["base_url"].(string); ok {
			cfg.RAG.BaseURL = v
		}
		if v, ok := rag["model"].(string); ok {
			cfg.RAG.Model = v
		}
		if v, ok := rag["api_key"].(string); ok {
			if strings.Contains(v, "***") {
				return fmt.Errorf("rag.api_key must not contain '***' — please provide the full key value")
			}
			cfg.RAG.APIKey = v
		}
		if v, ok := rag["chunk_size"].(float64); ok {
			cfg.RAG.ChunkSize = int(v)
		}
		if v, ok := rag["search_limit"].(float64); ok {
			cfg.RAG.SearchLimit = int(v)
		}
		if v, ok := rag["search_pool_size"].(float64); ok {
			cfg.RAG.SearchPoolSize = int(v)
		}
		if v, ok := rag["retention_days"].(float64); ok {
			cfg.RAG.RetentionDays = int(v)
		}
	}

	if pf, ok := patch["port_forward"].(map[string]any); ok {
		if v, ok := pf["enabled"].(bool); ok {
			cfg.PortForward.Enabled = v
		}
		if v, ok := pf["port"].(float64); ok {
			cfg.PortForward.Port = int(v)
		}
		if v, ok := pf["allowed_ports"].(string); ok {
			cfg.PortForward.AllowedPorts = v
			cfg.Proxy.AllowedPorts = v // keep in sync for backward compat
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
			if v, ok := jpush["master_secret"].(string); ok {
				cfg.Push.JPush.MasterSecret = v
			}
		}
	}

	if tasks, ok := patch["tasks"].(map[string]any); ok {
		if v, ok := tasks["summarize_backend"].(string); ok {
			cfg.Tasks.SummarizeBackend = v
		}
		if v, ok := tasks["summarize_model"].(string); ok {
			cfg.Tasks.SummarizeModel = v
		}
	}

	// Also update global variables for hot-reloadable fields
	applyHotReloadGlobals()

	return nil
}

// applyHotReloadGlobals syncs the package-level "hot-reload" variables from
// ConfigInstance. Called after a successful patch (and on rollback).
func applyHotReloadGlobals() {
	cfg := model.ConfigInstance
	model.ChatCollapsedHeight = cfg.Chat.CollapsedHeight
	model.ChatInitialMessages = cfg.Chat.InitialMessages
	model.ChatPageSize = cfg.Chat.PageSize
	model.ChatSystemPromptInterval = cfg.Chat.SystemPromptInterval
	model.SessionMaxCount = cfg.Session.MaxCount
	model.UploadMaxSizeMB = cfg.Upload.MaxSizeMB
	model.UploadMaxFiles = cfg.Upload.MaxFiles
	model.TTSMaxCacheFiles = cfg.TTS.MaxCacheFiles
}

// writeConfigYAML writes the patched fields back to config/config.yaml atomically.
// Instead of rewriting the entire ConfigInstance (which would expand zero values
// and break ApplyDefaults presence semantics), it reads the existing YAML into a
// generic map, patches only the changed fields, and writes back. This preserves
// user comments, field ordering, and absent-as-default semantics for untouched fields.
func writeConfigYAML(patch map[string]any) error {
	configDir := filepath.Join(model.BinDir, "config")
	configPath := filepath.Join(configDir, "config.yaml")
	tmpPath := configPath + ".tmp"
	bakPath := configPath + ".bak"

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing YAML content into a generic map
	var raw map[string]any
	if data, err := os.ReadFile(configPath); err == nil && len(data) > 0 {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			slog.Warn("failed to parse existing config.yaml, starting fresh", "err", err)
			raw = make(map[string]any)
		}
	} else {
		// No existing file — marshal the full ConfigInstance as base
		data, err := yaml.Marshal(&model.ConfigInstance)
		if err != nil {
			return fmt.Errorf("failed to marshal initial config: %w", err)
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("failed to unmarshal initial config: %w", err)
		}
	}

	// Backup existing file before overwriting
	if _, err := os.Stat(configPath); err == nil {
		if err := copyFile(configPath, bakPath); err != nil {
			slog.Warn("failed to backup config.yaml", "err", err)
		}
	}

	// Merge patch into raw map (only touched fields are modified)
	mergePatchIntoRaw(raw, patch)

	data, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename config file: %w", err)
	}

	return nil
}

// mergePatchIntoRaw merges the PATCH payload into the raw YAML map.
// Only the leaf values present in the patch are updated; all other fields
// in the raw map are preserved untouched.
func mergePatchIntoRaw(raw map[string]any, patch map[string]any) {
	for key, value := range patch {
		if nested, ok := value.(map[string]any); ok {
			// Recursively merge nested maps
			existing, ok := raw[key].(map[string]any)
			if !ok {
				existing = make(map[string]any)
			}
			mergePatchIntoRaw(existing, nested)
			raw[key] = existing
		} else {
			raw[key] = value
		}
	}
}

// ServeConfigRestart handles POST /api/config/restart — triggers server restart.
func ServeConfigRestart(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	go func() {
		time.Sleep(restartGracePeriod)

		if restartFunc != nil {
			restartFunc()
		} else {
			slog.Warn("restart function not set, cannot restart server")
		}
	}()

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "restarting",
	})
}

// LaunchSentinelProcess starts a sentinel process that waits for the current
// process to exit, then starts a new one. Returns the sentinel cmd on success.
func LaunchSentinelProcess() (*exec.Cmd, error) {
	return launchSentinel()
}

// IsRunningUnderSupervisor detects if the process is managed by systemd, Docker, etc.
func IsRunningUnderSupervisor() bool {
	if os.Getenv("CLAWBENCH_NO_SUPERVISOR") != "" {
		return false
	}
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}
	if os.Getenv("container") != "" {
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if os.Getppid() == 1 {
		return true
	}
	return false
}

// joinArgs joins command-line args into a space-separated string with proper
// shell quoting. Single quotes within args are escaped using the '\'' idiom.
func joinArgs(args []string) string {
	var buf strings.Builder
	for i, arg := range args {
		if i > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(shellQuote(arg))
	}
	return buf.String()
}

// shellQuote wraps a string in single quotes, escaping any embedded single
// quotes using the '\'' idiom safe for POSIX shells.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

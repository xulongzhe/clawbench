package handler

import (
	"clawbench/internal/model"
	"net/http"
)

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

// ServeConfig handles GET /api/config — returns sanitized config.
func ServeConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

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

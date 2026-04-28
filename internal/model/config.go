package model

// Config holds the application configuration.
type Config struct {
	Port         int    `yaml:"port"`
	WatchDir     string `yaml:"watch_dir"`
	Password     string `yaml:"password"`
	DefaultAgent string `yaml:"default_agent"`
	LogDir       string `yaml:"log_dir"`
	LogMaxDays   int    `yaml:"log_max_days"`
	TLS        struct {
		Enabled  bool   `yaml:"enabled"`
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
	} `yaml:"tls"`
  Dev struct {
		Port     int    `yaml:"port"`
		Frontend int    `yaml:"frontend_port"`
		Host     string `yaml:"host"` // Bind address (empty = 0.0.0.0, "localhost" = 127.0.0.1 only)
	} `yaml:"dev"`
	Upload struct {
		MaxSizeMB int `yaml:"max_size_mb"` // Maximum file upload size in MB (default: 100)
		MaxFiles  int `yaml:"max_files"`  // Maximum number of files per upload (default: 20)
	} `yaml:"upload"`
	Chat struct {
		InitialMessages int `yaml:"initial_messages"` // Number of messages to load initially (default: 20)
		PageSize        int `yaml:"page_size"`        // Number of messages per lazy-load batch (default: 20)
		CollapsedHeight int `yaml:"collapsed_height"` // Collapsed message height in pixels (default: 150)
	} `yaml:"chat"`
	TTS struct {
		SummarizeModel string  `yaml:"summarize_model"` // LLM model for summarization (default: "MiniMax-Text-02-HS")
		TTSModel       string  `yaml:"tts_model"`       // TTS model for speech synthesis (default: "Speech-2.8-Turbo")
		Voice          string  `yaml:"voice"`           // Voice ID for TTS (default: "female-chengshu")
		Language       string  `yaml:"language"`        // Language boost code (default: "zh")
		Speed          float64 `yaml:"speed"`           // Speech speed multiplier (default: 1.0)
		Format         string  `yaml:"format"`          // Audio output format (default: "mp3")
	} `yaml:"tts"`
}

// Global application state
var (
	BinDir         string // Directory of the running binary
	WatchDir       string
	SessionToken   string
	SessionCookie  = "clawbench_session"
	DevMode        bool   // True when running in debug/development mode
	DefaultAgentID string // Default agent for new sessions, set from config or first agent

	// Upload limits (set from config, with defaults)
	UploadMaxSizeMB int // Default: 100
	UploadMaxFiles  int // Default: 20

	// Chat UI config (set from config, with defaults)
	ChatInitialMessages int // Default: 20
	ChatPageSize        int // Default: 20
	ChatCollapsedHeight int // Default: 150
)

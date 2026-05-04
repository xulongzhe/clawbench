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
		InitialMessages int      `yaml:"initial_messages"` // Number of messages to load initially (default: 20)
		PageSize        int      `yaml:"page_size"`        // Number of messages per lazy-load batch (default: 20)
		CollapsedHeight int      `yaml:"collapsed_height"` // Collapsed message height in pixels (default: 150)
		QuickSend       map[string]string `yaml:"quick_send"` // Quick-send presets: key=display label (with emoji), value=actual message text
	} `yaml:"chat"`
	Session struct {
		MaxCount int `yaml:"max_count"` // Maximum number of chat sessions per project (default: 10)
	} `yaml:"session"`
	TTS struct {
		Engine            string         `yaml:"engine"`             // TTS engine: "edge" (default), "minimax", "piper", "kokoro", "moss-nano"
		SummarizeBackend  string         `yaml:"summarize_backend"`  // Summarization backend: "mmx-cli" (default), "claude", "codebuddy", "gemini", "opencode", "codex", "ollama", "simple"
		SummarizeModel    string         `yaml:"summarize_model"`    // Model for summarization (default: "MiniMax-M2.7" for mmx-cli, "gemma3:270m" for ollama; empty = backend default for others)
		TTSModel          string         `yaml:"tts_model"`          // TTS model for speech synthesis (default: "Speech-2.8-Turbo")
		Voice             string         `yaml:"voice"`              // Voice ID for TTS (default: "female-chengshu")
		Speed             float64        `yaml:"speed"`              // Speech speed multiplier (default: 1.0)
		Format            string         `yaml:"format"`             // Audio output format (default: "mp3")
		InlineCodeMaxLen  int            `yaml:"inline_code_max_len"` // Max inline code content length (runes) to preserve for TTS; longer code is removed (default: 100)
		MaxSummarizeRunes int            `yaml:"max_summarize_runes"` // Max runes for summarization input; longer text is truncated (default: 10000, simple mode: 1000)
		Piper             PiperConfig    `yaml:"piper"`              // Piper-specific configuration (only used when engine: "piper")
		Kokoro            KokoroConfig   `yaml:"kokoro"`             // Kokoro-specific configuration (only used when engine: "kokoro")
		MossNano          MossNanoConfig `yaml:"moss_nano"`          // MOSS-TTS-Nano-specific configuration (only used when engine: "moss-nano")
		Ollama            OllamaConfig   `yaml:"ollama"`             // Ollama-specific configuration (only used when summarize_backend: "ollama")
	} `yaml:"tts"`
	Proxy ProxyConfig `yaml:"proxy"` // Port forwarding configuration
	SSH   SSHConfig   `yaml:"ssh"`   // SSH tunnel server configuration
}

// PiperConfig holds configuration for the Piper TTS engine.
type PiperConfig struct {
	ModelPath       string  `yaml:"model_path"`        // Path to .onnx model file (empty = .clawbench/piper-models/<voice>.onnx)
	NoiseScale      float64 `yaml:"noise_scale"`       // Noise scale for sampling (default: 0.667)
	LengthScale     float64 `yaml:"length_scale"`      // Length scale for speech rate (default: 1.0)
	SentenceSilence float64 `yaml:"sentence_silence"`  // Silence between sentences in seconds (default: 0.2)
}

// KokoroConfig holds configuration for the Kokoro TTS engine.
type KokoroConfig struct {
	ModelPath  string  `yaml:"model_path"`   // Path to kokoro .onnx model file (empty = .clawbench/kokoro-models/kokoro-v1.0.onnx)
	VoicesPath string  `yaml:"voices_path"`  // Path to voices .bin file (empty = .clawbench/kokoro-models/voices-v1.0.bin)
	Lang       string  `yaml:"lang"`         // espeak language code for phonemization (default: "cmn" for Mandarin Chinese)
}

// MossNanoConfig holds configuration for the MOSS-TTS-Nano TTS engine.
type MossNanoConfig struct {
	ModelDir     string `yaml:"model_dir"`      // Directory for ONNX model files (empty = .clawbench/moss-nano-models; CLI auto-downloads if missing)
	PromptSpeech string `yaml:"prompt_speech"`  // Path to reference audio for voice cloning (empty = use built-in voice preset)
	Voice        string `yaml:"voice"`           // Built-in voice preset for ONNX backend when no prompt-speech (default: "Junhao")
	Backend      string `yaml:"backend"`         // Inference backend: "onnx" (default, CPU) or "pytorch" (requires GPU)
}

// OllamaConfig holds configuration for the Ollama summarization backend.
type OllamaConfig struct {
	BaseURL string `yaml:"base_url"` // Ollama API base URL (default: "http://localhost:11434")
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
	ChatQuickSend       map[string]string // Quick-send presets: key=display label, value=actual message

	// Session limits (set from config, with defaults)
	SessionMaxCount int // Default: 10
)

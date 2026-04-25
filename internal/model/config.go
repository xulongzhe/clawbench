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
		Port     int `yaml:"port"`
		Frontend int `yaml:"frontend_port"`
	} `yaml:"dev"`
	Upload struct {
		MaxSizeMB int `yaml:"max_size_mb"` // Maximum file upload size in MB (default: 10)
		MaxFiles  int `yaml:"max_files"`  // Maximum number of files per upload (default: 20)
	} `yaml:"upload"`
}

// Global application state
var (
	WatchDir       string
	SessionToken   string
	SessionCookie  = "clawbench_session"
	DevMode        bool   // True when running in debug/development mode
	DefaultAgentID string // Default agent for new sessions, set from config or first agent

	// Upload limits (set from config, with defaults)
	UploadMaxSizeMB int // Default: 10
	UploadMaxFiles  int // Default: 20
)

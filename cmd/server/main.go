package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"

	"clawbench/internal/handler"
	"clawbench/internal/model"
	"clawbench/internal/platform"
	"clawbench/internal/service"
)

// multiHandler sends log records to multiple handlers
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var lastError error
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, r); err != nil {
			lastError = err
		}
	}
	return lastError
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

func main() {
	// Parse CLI flags
	devMode := false
	cliPort := 0
	for i, arg := range os.Args[1:] {
		if arg == "--dev" {
			devMode = true
		} else if arg == "--port" && i+1 < len(os.Args[1:]) {
			if p, err := strconv.Atoi(os.Args[i+2]); err == nil && p > 0 && p <= 65535 {
				cliPort = p
			}
		}
	}
	if devMode {
		model.DevMode = true
	}

	// Determine binary directory for data storage (green portable layout)
	absBinPath, _ := filepath.Abs(os.Args[0])
	model.BinDir = filepath.Dir(absBinPath)

	// Load configuration
	configPath := filepath.Join(model.BinDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config.yaml: %v\n", err)
		os.Exit(1)
	}

	var cfg model.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config.yaml: %v\n", err)
		os.Exit(1)
	}

	if cfg.WatchDir == "" {
		cfg.WatchDir = filepath.Join(platform.TempDir(), "clawbench-default")
	}
	cfg.WatchDir = platform.ExpandTilde(cfg.WatchDir)
	model.WatchDir = cfg.WatchDir

	// Set upload limits with defaults
	if cfg.Upload.MaxSizeMB <= 0 {
		cfg.Upload.MaxSizeMB = 10
	}
	if cfg.Upload.MaxFiles <= 0 {
		cfg.Upload.MaxFiles = 20
	}
	model.UploadMaxSizeMB = cfg.Upload.MaxSizeMB
	model.UploadMaxFiles = cfg.Upload.MaxFiles

	if cfg.LogMaxDays <= 0 {
		cfg.LogMaxDays = 7
	}
	if cfg.LogDir == "" {
		cfg.LogDir = filepath.Join(model.BinDir, ".clawbench", "logs")
	}
	cfg.LogDir = platform.ExpandTilde(cfg.LogDir)
	// In dev mode, use a separate log directory
	if devMode && cfg.LogDir != "" {
		cfg.LogDir = cfg.LogDir + "-dev"
	}
	fileHandler, err := service.NewFileHandler(cfg.LogDir, "clawbench", cfg.LogMaxDays)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize file logger: %v\n", err)
		os.Exit(1)
	}
	defer fileHandler.Close()

	// Dev mode uses DEBUG log level, release uses INFO
	logLevel := slog.LevelInfo
	if devMode {
		logLevel = slog.LevelDebug
	}

	// Create a multi-writer for both stderr and file
	textHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	multiHandler := &multiHandler{
		handlers: []slog.Handler{textHandler, fileHandler},
	}
	slog.SetDefault(slog.New(multiHandler))
	slog.Info("server starting",
		slog.Bool("dev_mode", devMode),
	)

	// Load agent configurations
	agentsDir := filepath.Join(model.BinDir, "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		agentsDir = "agents"
	}
	if err := model.LoadAgents(agentsDir); err != nil {
		slog.Warn("failed to load agents", slog.String("err", err.Error()))
	}
	slog.Info("agents loaded", slog.Int("count", len(model.AgentList)))

	// Set default agent ID from config, or fall back to first agent
	if cfg.DefaultAgent != "" {
		if _, ok := model.Agents[cfg.DefaultAgent]; ok {
			model.DefaultAgentID = cfg.DefaultAgent
		} else {
			slog.Warn("configured default_agent not found, using first agent",
				slog.String("configured", cfg.DefaultAgent))
		}
	}
	if model.DefaultAgentID == "" && len(model.AgentList) > 0 {
		model.DefaultAgentID = model.AgentList[0].ID
	}
	if model.DefaultAgentID != "" {
		slog.Info("default agent", slog.String("id", model.DefaultAgentID))
	} else {
		slog.Warn("no agents available, session creation will fail")
	}

	// Hash the password for session comparison
	if cfg.Password != "" {
		hash := sha256.Sum256([]byte(cfg.Password + "clawbench-salt"))
		model.SessionToken = hex.EncodeToString(hash[:])
	}

	// Ensure the watch directory exists
	if err := os.MkdirAll(model.WatchDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create watch directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize SQLite database
	if err := service.InitDB(); err != nil {
		slog.Error("failed to initialize database", slog.String("err", err.Error()))
		os.Exit(1)
	}

	// Initialize and start scheduler
	scheduler := service.NewScheduler()
	// Load all tasks from all projects
	if err := scheduler.LoadTasksFromDB(""); err != nil {
		slog.Warn("failed to load scheduled tasks", slog.String("err", err.Error()))
	}
	scheduler.Start()
	defer scheduler.Stop()
	service.GlobalScheduler = scheduler

	port := cfg.Port
	// Dev mode: use dev-specific port from config
	if devMode && cfg.Dev.Port > 0 {
		port = cfg.Dev.Port
	}
	// Allow PORT environment variable to override config
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p <= 65535 {
			port = p
		}
	}
	// CLI --port flag takes highest priority
	if cliPort > 0 {
		port = cliPort
	}
	if port <= 0 || port > 65535 {
		if devMode {
			port = 20002
		} else {
			port = 20000
		}
	}
	addr := fmt.Sprintf(":%d", port)
	slog.Info("server ready",
		slog.String("addr", addr),
		slog.String("watch_dir", model.WatchDir),
		slog.Bool("auth_enabled", model.SessionToken != ""),
	)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	if devMode || !cfg.TLS.Enabled {
		// Dev mode or TLS disabled: plain HTTP
		if !cfg.TLS.Enabled && !devMode {
			slog.Info("TLS disabled, starting with HTTP")
		} else {
			slog.Info("starting in dev mode (HTTP)")
		}
		if err := http.ListenAndServe(addr, mux); err != nil {
			slog.Error("server failed", slog.String("err", err.Error()))
			os.Exit(1)
		}
	} else {
		// Release mode: HTTPS with TLS
		certFile := cfg.TLS.CertFile
		keyFile := cfg.TLS.KeyFile
		if certFile == "" {
			certFile = os.Getenv("CERT_FILE")
		}
		if keyFile == "" {
			keyFile = os.Getenv("KEY_FILE")
		}
		if certFile == "" || keyFile == "" {
			slog.Error("TLS enabled but cert_file and key_file are not configured")
			os.Exit(1)
		}
		slog.Info("starting with TLS", slog.String("cert", certFile))
		if err := http.ListenAndServeTLS(addr, certFile, keyFile, mux); err != nil {
			slog.Error("server failed", slog.String("err", err.Error()))
			os.Exit(1)
		}
	}
}

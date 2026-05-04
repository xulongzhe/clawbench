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
	"clawbench/internal/service"
	"clawbench/internal/ssh"
	"clawbench/internal/speech"
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

	// Load configuration — config.yaml is optional
	var cfg model.Config
	var presence map[string]bool

	configPath := filepath.Join(model.BinDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err == nil {
		// Parse into raw map first for presence detection (bool defaults)
		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse config.yaml: %v\n", err)
			os.Exit(1)
		}
		presence = model.ParsePresenceMap(raw)

		// Parse into typed config struct
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse config.yaml: %v\n", err)
			os.Exit(1)
		}
	} else if !os.IsNotExist(err) {
		// File exists but can't be read (permissions, etc.)
		fmt.Fprintf(os.Stderr, "Failed to read config.yaml: %v\n", err)
		os.Exit(1)
	}
	// If file doesn't exist: cfg stays zero-value, presence is nil → all defaults apply

	// Apply all defaults (returns auto-generated password if created)
	autoPassword := model.ApplyDefaults(&cfg, presence)

	// Set global variables from config
	model.WatchDir = cfg.WatchDir
	model.UploadMaxSizeMB = cfg.Upload.MaxSizeMB
	model.UploadMaxFiles = cfg.Upload.MaxFiles
	model.ChatInitialMessages = cfg.Chat.InitialMessages
	model.ChatPageSize = cfg.Chat.PageSize
	model.ChatCollapsedHeight = cfg.Chat.CollapsedHeight
	model.ChatQuickSend = cfg.Chat.QuickSend
	model.SessionMaxCount = cfg.Session.MaxCount

	// Apply TTS text processing config (defaults applied in ApplyDefaults)
	speech.InlineCodeMaxLen = cfg.TTS.InlineCodeMaxLen
	speech.MaxSummarizeRunes = cfg.TTS.MaxSummarizeRunes

	// Initialize TTS summarizer from config
	// Language is now per-request (sent from frontend), not configured at startup.
	summarizeBackend := cfg.TTS.SummarizeBackend

	var ttsSummarizer speech.Summarizer
	if summarizeBackend == "simple" {
		ttsSummarizer = speech.NewSimpleSummarizer()
		slog.Info("tts summarizer configured",
			slog.String("backend", "simple"),
		)
	} else if summarizeBackend == "mmx-cli" {
		s := speech.NewMMXSummarizer()
		if cfg.TTS.SummarizeModel != "" {
			s.Model = cfg.TTS.SummarizeModel
		}
		ttsSummarizer = s
		slog.Info("tts summarizer configured",
			slog.String("backend", "mmx-cli"),
			slog.String("model", s.Model),
		)
	} else if summarizeBackend == "ollama" {
		s := speech.NewOllamaSummarizer(cfg.TTS.Ollama.BaseURL, cfg.TTS.SummarizeModel)
		ttsSummarizer = s
		slog.Info("tts summarizer configured",
			slog.String("backend", "ollama"),
			slog.String("model", s.Model),
			slog.String("base_url", s.BaseURL),
		)
	} else {
		s, err := speech.NewAIBackendSummarizer(summarizeBackend)
		if err != nil {
			slog.Error("failed to create AI backend summarizer, falling back to mmx-cli",
				slog.String("backend", summarizeBackend),
				slog.String("error", err.Error()),
			)
			fallback := speech.NewMMXSummarizer()
			if cfg.TTS.SummarizeModel != "" {
				fallback.Model = cfg.TTS.SummarizeModel
			}
			ttsSummarizer = fallback
		} else {
			s.Model = cfg.TTS.SummarizeModel // empty = use backend default
			ttsSummarizer = s
			slog.Info("tts summarizer configured",
				slog.String("backend", summarizeBackend),
				slog.String("model", s.Model),
			)
		}
	}
	handler.SetSummarizer(ttsSummarizer)

	// Initialize TTS synthesis provider from config
	var ttsProvider speech.SpeechProvider
	engine := cfg.TTS.Engine

	switch engine {
	case "edge":
		p := speech.NewEdgeTTSProvider()
		if cfg.TTS.Voice != "" {
			p.Voice = cfg.TTS.Voice
		}
		if cfg.TTS.Speed > 0 {
			// Convert speed multiplier (e.g. 1.5) to edge-tts rate percentage (e.g. "+50%")
			ratePercent := int((cfg.TTS.Speed - 1.0) * 100)
			if ratePercent > 0 {
				p.Rate = fmt.Sprintf("+%d%%", ratePercent)
			} else if ratePercent < 0 {
				p.Rate = fmt.Sprintf("%d%%", ratePercent)
			}
		}
		ttsProvider = p
		slog.Info("tts provider configured",
			slog.String("engine", "edge"),
			slog.String("voice", p.Voice),
			slog.String("rate", p.Rate),
		)
	case "piper":
		p := speech.NewPiperProvider()
		// Resolve model path: explicit config > voice-based path
		p.ModelPath = speech.ResolveModelPath(cfg.TTS.Voice, cfg.TTS.Piper.ModelPath)
		if cfg.TTS.Piper.NoiseScale > 0 {
			p.NoiseScale = cfg.TTS.Piper.NoiseScale
		}
		// LengthScale: explicit piper.length_scale takes priority;
		// otherwise convert speed multiplier (length_scale = 1/speed)
		if cfg.TTS.Piper.LengthScale > 0 {
			p.LengthScale = cfg.TTS.Piper.LengthScale
		} else if cfg.TTS.Speed > 0 {
			p.LengthScale = 1.0 / cfg.TTS.Speed
		}
		if cfg.TTS.Piper.SentenceSilence > 0 {
			p.SentenceSilence = cfg.TTS.Piper.SentenceSilence
		}
		ttsProvider = p
		slog.Info("tts provider configured",
			slog.String("engine", "piper"),
			slog.String("model_path", p.ModelPath),
			slog.Float64("noise_scale", p.NoiseScale),
			slog.Float64("length_scale", p.LengthScale),
			slog.Float64("sentence_silence", p.SentenceSilence),
		)
	case "kokoro":
		k := speech.NewKokoroProvider()
		if cfg.TTS.Voice != "" {
			k.Voice = cfg.TTS.Voice
		}
		if cfg.TTS.Speed > 0 {
			k.Speed = cfg.TTS.Speed
		}
		if cfg.TTS.Kokoro.Lang != "" {
			k.Lang = cfg.TTS.Kokoro.Lang
		}
		k.ModelPath, k.VoicesPath = speech.ResolveKokoroPaths(cfg.TTS.Kokoro.ModelPath, cfg.TTS.Kokoro.VoicesPath)
		ttsProvider = k
		slog.Info("tts provider configured",
			slog.String("engine", "kokoro"),
			slog.String("model_path", k.ModelPath),
			slog.String("voices_path", k.VoicesPath),
			slog.String("voice", k.Voice),
			slog.String("lang", k.Lang),
			slog.Float64("speed", k.Speed),
		)
	case "moss-nano":
		m := speech.NewMossNanoProvider()
		if cfg.TTS.MossNano.Backend != "" {
			m.Backend = cfg.TTS.MossNano.Backend
		}
		m.ModelDir = speech.ResolveMossNanoModelDir(cfg.TTS.MossNano.ModelDir)
		if cfg.TTS.MossNano.PromptSpeech != "" {
			m.PromptSpeech = cfg.TTS.MossNano.PromptSpeech
		}
		if cfg.TTS.MossNano.Voice != "" {
			m.Voice = cfg.TTS.MossNano.Voice
		}
		ttsProvider = m
		slog.Info("tts provider configured",
			slog.String("engine", "moss-nano"),
			slog.String("backend", m.Backend),
			slog.String("model_dir", m.ModelDir),
			slog.String("prompt_speech", m.PromptSpeech),
			slog.String("voice", m.Voice),
		)
	default:
		p := speech.NewMiniMaxProvider()
		if cfg.TTS.TTSModel != "" {
			p.TTSModel = cfg.TTS.TTSModel
		}
		if cfg.TTS.Voice != "" {
			p.TTSVoice = cfg.TTS.Voice
		}
		if cfg.TTS.Speed > 0 {
			p.TTSSpeed = cfg.TTS.Speed
		}
		if cfg.TTS.Format != "" {
			p.TTSFormat = cfg.TTS.Format
		}
		ttsProvider = p
		slog.Info("tts provider configured",
			slog.String("engine", "minimax"),
			slog.String("tts_model", p.TTSModel),
			slog.String("voice", p.TTSVoice),
			slog.Float64("speed", p.TTSSpeed),
		)
	}
	handler.SetSpeechProvider(ttsProvider)

	// In dev mode, use a separate log directory
	if devMode {
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
	agentsDir := filepath.Join(model.BinDir, "config", "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		agentsDir = filepath.Join("config", "agents")
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
			// List available agent IDs to help the user fix the config
			availableIDs := make([]string, 0, len(model.AgentList))
			for _, a := range model.AgentList {
				availableIDs = append(availableIDs, a.ID)
			}
			slog.Warn("configured default_agent not found, using first agent",
				slog.String("configured", cfg.DefaultAgent),
				slog.Any("available", availableIDs))
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

	// Print auto-generated password info
	if autoPassword != "" {
		slog.Info("auto-generated password (no password configured)",
			slog.String("password", autoPassword),
			slog.String("file", filepath.Join(model.BinDir, ".clawbench", "auto-password")),
		)
		// Also print to stdout for foreground mode and shell scripts to capture
		fmt.Printf("Auto-generated password: %s\n", autoPassword)
	}

	// Hash the password for session comparison
	hash := sha256.Sum256([]byte(cfg.Password + "clawbench-salt"))
	model.SessionToken = hex.EncodeToString(hash[:])

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
	host := ""
	if devMode && cfg.Dev.Host != "" {
		host = cfg.Dev.Host
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	slog.Info("server ready",
		slog.String("addr", addr),
		slog.String("watch_dir", model.WatchDir),
		slog.Bool("auth_enabled", model.SessionToken != ""),
	)

	// Initialize proxy service (port forwarding) — needs the final port number
	proxyService := service.NewProxyRegistry(cfg.Proxy, port)
	service.ProxyService = proxyService
	defer proxyService.Stop()

	// Initialize SSH tunnel server
	if cfg.SSH.Enabled {
		sshServer := ssh.NewServer(cfg.SSH, port, cfg.Password, proxyService)
		handler.SetSSHServer(sshServer)
		go func() {
			if err := sshServer.ListenAndServe(); err != nil {
				slog.Error("SSH server failed", slog.String("err", err.Error()))
			}
		}()
		defer sshServer.Close()
	}

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

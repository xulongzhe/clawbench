[中文](DEVELOPMENT.md) | [English](DEVELOPMENT.en.md)

# Build & Development Guide

## Quick Start

### Option 1: Use Release Package (Recommended)

Download the latest ZIP package from [GitHub Releases](https://github.com/xulongzhe/clawbench/releases), extract and deploy. All configuration items have default values, no config file needed to start.

```bash
# 1. Download and extract
wget https://github.com/xulongzhe/clawbench/releases/latest/download/clawbench-linux-amd64.zip
unzip clawbench-linux-amd64.zip

# 2. Start the server (no config file needed)
cd clawbench
./server.sh
```

> On first startup, a random password is auto-generated and saved to `.clawbench/auto-password`; the startup script will display it automatically. To customize configuration, copy `config/config.example.yaml` to `config/config.yaml` and modify as needed.

Release package contents (Linux):

| File | Description |
|------|-------------|
| `clawbench-linux-amd64` | Backend binary |
| `public/` | Frontend static assets (pre-built) |
| `config/config.example.yaml` | Config template (optional) |
| `config/agents/` | Agent configurations |
| `dev-server.sh` | Dev debug startup script |
| `server.sh` | Production startup script |

Release package contents (Windows):

| File | Description |
|------|-------------|
| `clawbench-windows-amd64.exe` | Backend binary |
| `public/` | Frontend static assets (pre-built) |
| `config/config.example.yaml` | Config template (optional) |
| `config/agents/` | Agent configurations |
| `server.ps1` | Start/stop script |

### Option 2: Build from Source

**Linux/macOS:**

```bash
# 1. Clone the project
git clone https://github.com/xulongzhe/clawbench.git
cd clawbench

# 2. One-click build and start (no config file needed, all items have defaults)
./build.sh && ./server.sh
```

**Dev debug mode:**

```bash
# Start in background (Go dev backend + Vite HMR)
./dev-server.sh

# Start in foreground, view live logs
./dev-server.sh --fg

# Stop background processes
./dev-server.sh --stop

# Restart
./dev-server.sh --restart
```

**Windows (PowerShell):**

```powershell
# 1. Clone the project
git clone https://github.com/xulongzhe/clawbench.git
cd clawbench

# 2. One-click build and start (no config file needed)
.\build.ps1; .\server.ps1
```

**Cross-compilation** (build Windows binary on Linux):

```bash
./build.sh --windows
```

### System Requirements

| Platform | Supported |
|----------|-----------|
| Linux x86_64 / ARM64 | ✅ |
| Windows x86_64 | ✅ |

| Dependency | Description |
|------------|-------------|
| **CodeBuddy CLI** or **Claude Code CLI** | AI backend (install and authenticate in advance; OpenCode / Gemini / Codex / Qoder / VeCLI optional) |

### Configuration File

`config/config.yaml` is entirely optional — all configuration items have default values. To customize, copy `config/config.example.yaml` to `config/config.yaml` and modify.

**Defaults:**

| Config Item | Default | Description |
|-------------|---------|-------------|
| `port` | 20000 | Server port |
| `watch_dir` | User home directory | Linux: `/home/username`, Windows: `C:\Users\username` |
| `password` | Auto-generated UUID | Generated on first run, saved to `.clawbench/auto-password`, reused on restart |
| `log_dir` | `<BinDir>/.clawbench/logs` | Under the binary's directory |
| `log_max_days` | 7 | Log retention days |
| `upload.max_size_mb` | 100 | Upload size limit in MB |
| `upload.max_files` | 20 | Max files per upload |
| `chat.initial_messages` | 20 | Initial message count to load |
| `chat.page_size` | 20 | Messages per page for lazy loading |
| `chat.collapsed_height` | 150 | Collapsed height for history messages in px |
| `session.max_count` | 10 | Max sessions per project |
| `recent_projects.max_count` | 10 | Max recent projects shown in header dropdown |
| `terminal.enabled` | true | Web terminal enabled by default |
| `terminal.idle_timeout` | 10m | Terminal idle timeout |
| `terminal.max_sessions` | 10 | Max terminal sessions per project |
| `port_forward.enabled` | true | Port forwarding enabled by default |
| `port_forward.port` | 0 (auto) | SSH port (0 = main port + 1) |
| `port_forward.host_key` | (auto) | Host key file path |
| `port_forward.allowed_ports` | (all) | Allowed port range for forwarding |
| `tts.engine` | edge | Edge TTS, free and unlimited |
| `tts.speed` | 1.0 | Normal speech rate |
| `tts.max_cache_files` | 100 | Max cached TTS audio files; oldest auto-deleted when exceeded (-1=unlimited) |
| `tts.inline_code_max_len` | 100 | Max characters (runes) to keep for inline code; exceeded content is removed |
| `tts.max_summarize_runes` | 10000 | Max input characters for summarization; tail is truncated if exceeded |
| `summarize.backend` | simple | Unified summarization backend (shared by TTS + scheduled tasks), zero latency |
| `summarize.model` | (empty) | Model for summarization, empty = backend default |
| `summarize.api` | (empty) | API sub-config (used when backend is "api"), includes base_url/key/format |
| `summarize.chat_summary` | true | Auto-generate summary of last assistant message on session complete (`*bool`, nil=true) |

**Auto-password mechanism**: When `password` is not configured, the system auto-generates a random UUID as password, saved to `.clawbench/auto-password` (permissions 0600). On restart, the saved password is reused and not regenerated. Once `password` is configured, the file is auto-deleted. The startup script reads and displays the password from the file.

**Example configuration:**

```yaml
# All values below are defaults; only configure when you need to override
# port: 20000
# watch_dir: "/home/user"       # Linux/macOS defaults to user home directory
# watch_dir: "C:\\Users\\user"  # Windows defaults to user home directory
# password: "your_password"     # Auto-generated if not configured
# default_agent: "assistant"   # Default agent; uses first agent if empty
```

### Startup Commands

#### Production

| Command | Description |
|---------|-------------|
| `./clawbench-linux-amd64` | Run directly (foreground) |
| `./server.sh` | Start in background (port 20000) |
| `./server.sh --fg` | Start in foreground (view live logs) |
| `./server.sh --stop` | Stop server |
| `./server.sh --restart` | Restart server |
| `./server.sh --port 8080` | Specify port |

#### Dev Debug Mode

| Command | Description |
|---------|-------------|
| `./dev-server.sh` | Start in background (dev backend + Vite, ports 20002/20001) |
| `./dev-server.sh --fg` | Start in foreground |
| `./dev-server.sh --stop` | Stop processes |
| `./dev-server.sh --restart` | Restart |

> **Note**: Dev debug mode uses separate ports and database from production, so both can run simultaneously without interference.

**Windows:**

| Command | Description |
|---------|-------------|
| `.\clawbench-windows-amd64.exe` | Run directly (foreground) |
| `.\server.ps1` | Start in background |
| `.\server.ps1 -Foreground` | Start in foreground |
| `.\server.ps1 -Stop` | Stop server |
| `.\server.ps1 -Restart` | Restart server |
| `.\server.ps1 -Port 8080` | Specify port |

---

## Advanced Configuration

For full configuration reference, see `config/config.example.yaml`. All items are optional; below are examples that override defaults:

```yaml
# port: 20000                        # Production server port (default 20000)
# watch_dir: "/home/user"            # Project watch directory (default: user home directory)
# password: "your_password"          # Access password (auto-generates UUID and saves if not configured)

# Default agent (optional)
default_agent: "assistant"      # Default agent ID; uses first agent if empty
                                 # Available agents: assistant (all-round assistant), coder (coding expert),
                                 # gemini (Gemini CLI), handyman (handyman), codebuddy2 (Gemini), gpt54 (GPT)

# Upload limits (default max_size_mb: 100, max_files: 20)
upload:
  max_size_mb: 10
  max_files: 20

# Log configuration (default .clawbench/logs, 7-day retention)
log_dir: ".clawbench/logs"
log_max_days: 7

# TLS (HTTPS) configuration (optional)
tls:
  enabled: false                # Enable HTTPS
  cert_file: "/path/to/fullchain.pem"   # Certificate file
  key_file: "/path/to/privkey.pem"      # Private key file

# Port forwarding + SSH tunnel configuration (enabled by default, merged into port_forward)
port_forward:
  enabled: true                    # Enable SSH tunnel server (default: true)
  port: 0                          # SSH port (0 = auto = main port + 1)
  host_key: ""                     # Host key file path (empty = auto-generate)
  allowed_ports: ""                # Allowed port range for forwarding (empty = allow all)

# Chat UI configuration (default initial_messages: 20, page_size: 20, collapsed_height: 150)
chat:
  initial_messages: 20
  page_size: 20
  collapsed_height: 150
```

### AI Backend Configuration

ClawBench interacts with AI programming tools by calling local CLIs, no extra API key configuration needed.

**CodeBuddy backend**: Install CodeBuddy CLI and complete login authentication, ensure the `codebuddy` command is available in PATH. Model auto-discovery is implemented by parsing `product.cloudhosted.json` in the installation directory, supporting 21+ models (GLM, DeepSeek, Kimi, MiniMax, Hunyuan, etc.).

**Claude Code backend**: Install Claude Code CLI and complete authentication, ensure the `claude` command is available in PATH.

**OpenCode backend**: Install OpenCode CLI and complete authentication, ensure the `opencode` command is available in PATH.

**Gemini CLI backend**: Install Gemini CLI and complete authentication, ensure the `gemini` command is available in PATH. Supports API-based auto-discovery of available models.

**Codex backend**: Install OpenAI Codex CLI and complete authentication, ensure the `codex` command is available in PATH. Model auto-discovery is implemented via binary strings/state DB scanning.

**Qoder backend**: Install Qoder CLI (Alibaba coding agent) and complete authentication, ensure the `qoder` command is available in PATH. Qoder supports automatic model routing without specifying a specific model. Model auto-discovery is implemented by parsing `dynamic-texts.json`.

**VeCLI backend**: Install VeCLI (Volcengine Doubao) and complete authentication, ensure the `vecli` command is available in PATH. VeCLI outputs plain text (not JSON Lines), does not support session resume, and metadata is extracted from a `--session-summary` file after the process exits. Model auto-discovery is implemented by parsing `MODEL_REGISTRY`.

**DeepSeek TUI backend**: Install DeepSeek TUI (requires v0.8.33+) and complete authentication, ensure the `deepseek` command is available in PATH. Uses `deepseek exec --auto --output-format stream-json` mode with native `--system-prompt`, `--model`, and `--resume` flags.

**Pi backend**: Install Pi CLI and complete authentication, ensure the `pi` command is available in PATH. Pi is a minimalist coding agent that outputs NDJSON event stream via `--mode json`, supports session resume (`--session`/`--continue`) and model selection (`--model`).

All nine backends can be switched in real time on the ClawBench Web UI, with isolated session data.

### TTS Speech Synthesis Configuration

ClawBench supports TTS speech synthesis, automatically summarizing and reading aloud AI replies. Supports 5 TTS engines and 12 summarization backends.

| TTS Engine | Description | Network Requirement |
|------------|-------------|---------------------|
| `edge` | Microsoft Edge TTS, free and unlimited (default) | Requires network |
| `minimax` | Cloud synthesis, best audio quality | Requires mmx CLI + API quota |
| `piper` | Local offline, extremely fast | No network needed |
| `kokoro` | Local offline, high-quality Chinese | No network needed |
| `moss-nano` | Local offline, multilingual, 48kHz voice cloning | Model download on first use |

For detailed instructions on installation, deployment, configuration examples, and available voices for each engine, please refer to **[TTS Speech Synthesis Deployment Guide](TTS.md)**.

---

## Deployment

### HTTPS Configuration (Public Deployment)

Enabling HTTPS is recommended for production environments:

1. **Obtain certificate**: Use Let's Encrypt or another CA to issue a certificate
2. **Configure TLS**: Enable in `config/config.yaml`
   ```yaml
   tls:
     enabled: true
     cert_file: "/etc/letsencrypt/live/your-domain.com/fullchain.pem"
     key_file: "/etc/letsencrypt/live/your-domain.com/privkey.pem"
   ```
3. **Restart server**: `./server.sh --restart`

### Data Storage

| Data | Path | Description |
|------|------|-------------|
| Database | `Binary directory/.clawbench/ClawBench.db` | SQLite, sessions/history/projects/scheduled tasks |
| Logs | `Binary directory/.clawbench/logs/` | Daily rotation, auto-cleanup |
| Auto-password | `Binary directory/.clawbench/auto-password` | Auto-generated when password is not configured, reused on restart |
| Uploaded files | `Project directory/.clawbench/uploads/` | User-uploaded files, belonging to specific projects |

> All runtime data is stored under `.clawbench/` next to the binary, enabling green portable deployment — delete the program directory to completely uninstall. When the project directory is the same as the binary directory, uploaded files are also in the same `.clawbench/` directory.

### Dev Debug Mode

Use `./dev-server.sh` to start an independent development environment:

- Backend: `http://localhost:20002`
- Frontend (Vite HMR): `http://localhost:20001`
- Database: Uses `ClawBench-dev.db`, isolated from production data
- RAG vector store: Uses `rag-dev.duckdb`, isolated from production vector data

```bash
./dev-server.sh              # Start in background
./dev-server.sh --fg         # Start in foreground
./dev-server.sh --stop       # Stop
./dev-server.sh --restart    # Restart
```

---

## Architecture Design

### Agent Architecture

ClawBench is more than just a "chat shell" — it is a complete agent runtime platform:

```
config/agents/
├── assistant.yaml     # All-round assistant — general Q&A, code, docs, ops
├── codebuddy2.yaml    # Gemini (via CodeBuddy)
├── coder.yaml         # Coding expert — complex coding, architecture design, code refactoring
├── codex.yaml         # Codex — OpenAI Codex CLI coding assistant
├── gemini.yaml        # Gemini CLI — Google Gemini-powered general assistant
├── gpt54.yaml         # GPT — via CodeBuddy calling GPT models
├── qoder.yaml         # Qoder — Alibaba coding agent, auto model routing
├── vecli.yaml         # VeCLI — Volcengine Doubao-powered coding assistant
└── handyman.yaml      # Handyman — scheduled tasks, simple coding, daily operations
```

- **Configurable Agents**: Each agent is defined via YAML with dedicated system prompt, model, backend, and thinking effort levels — no code changes needed
- **Auto-Discovery**: On first startup, if `config/agents/` is empty, the system auto-scans for installed AI CLIs (claude, codebuddy, opencode, gemini, codex, qodercli, vecli, deepseek, pi) and generates minimal YAML configs for each detected backend. One-time only; existing files are never overwritten
- **Shared Rules**: `config/rules.md` defines common behaviors and mandatory rules for all agents (scheduled task CLI, RAG search, media handling), avoiding duplicate configuration
- **Template Placeholder**: `{{AVAILABLE_AGENTS}}` is auto-replaced with the available agent list, facilitating inter-agent dispatching
- **Multi-Agent Dispatching**: Different tasks match different agents; the all-round assistant handles conversations while specialized agents execute scheduled tasks
- **Transparent Tool Calls**: AI tool calls (file read/write, Bash commands, code editing) are visualized in real time
- **Cron Scheduled Execution**: AI creates scheduled tasks via `clawbench task` CLI subcommands; after confirmation, Cron scheduler executes them automatically. Task cards are embedded in chat messages. `list` and `get` subcommands allow inspecting existing tasks; `--prompt` supports `@path` syntax to read prompt text from a file
- **Cron Governance**: During scheduled execution, the Scheduled Tasks section in rules.md is automatically stripped (`<!-- SCHEDULED_BEGIN/END -->` markers), preventing AI from recursively creating tasks; CLI layer provides dual-layer protection via `CLAWBENCH_SCHEDULED=1` env var
- **Multi-Backend Switching**: The same platform simultaneously supports CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, VeCLI, DeepSeek TUI, and Pi backends with isolated session data

### Project Structure

```
clawbench/
├── cmd/server/main.go           # Application entry point
├── internal/
│   ├── handler/                 # HTTP handlers
│   │   ├── handler.go           # Route registration
│   │   ├── auth.go              # Authentication
│   │   ├── chat.go              # AI chat (SSE streaming)
│   │   ├── chat_quick_send.go   # Quick send CRUD
│   │   ├── agent.go             # Agent management
│   │   ├── scheduler.go         # Scheduled tasks
│   │   ├── rag_api.go           # RAG search API
│   │   ├── file.go              # File reading
│   │   ├── file_ops.go          # File operations
│   │   ├── file_thumb.go        # Image thumbnail generation (square canvas + dominant-color padding)
│   │   ├── file_archive.go      # File archive download (zip, symlink traversal protection)
│   │   ├── file_watch.go        # File change SSE notifications
│   │   ├── settings.go          # Settings (password change + SHA-256 verification)
│   │   ├── upload.go            # File upload
│   │   ├── git.go               # Git operations
│   │   ├── project.go           # Project management
│   │   ├── ssh_info.go          # SSH tunnel info API
│   │   ├── terminal.go          # Terminal + quick commands CRUD + multi-session
│   │   └── static.go            # Static files
│   ├── middleware/              # Middleware (auth/log/recovery/request ID)
│   ├── platform/                # Platform adaptation (Windows paths, etc.)
│   ├── service/                 # Business logic
│   │   ├── database.go          # SQLite initialization
│   │   ├── chat.go              # Chat history management
│   │   ├── summary.go           # Chat auto-summary (AsyncSummarize + summaries table)
│   │   ├── scheduler.go         # Scheduled task scheduling
│   │   ├── uuid.go              # UUID utility
│   │   └── logger.go            # File logger (daily rotation)
│   ├── model/                   # Data models
│   │   ├── config.go / defaults.go / chat.go / file.go / agent.go / scheduler.go / path.go / ssh.go / discovery.go
│   │   └── errors.go
│   ├── ssh/                     # SSH tunnel server
│   │   ├── server.go            # SSH server (direct-tcpip port forwarding)
│   │   └── server_test.go       # Tests
│   ├── proxy/                   # HTTP reverse proxy + port forwarding logic
│   │   ├── reverse_proxy.go     # HTTP reverse proxy (solves SSH tunnel Host header mismatch)
│   │   └── reverse_proxy_test.go # Tests
│   ├── cli/                     # CLI subcommands (AI agent self-service)
│   │   ├── task.go              # Scheduled task subcommands (create/update/delete/pause/resume/trigger/list/get/list-agents; --prompt supports @path syntax)
│   │   ├── migrate.go           # One-time DB migration (task_executions content → chat_history)
│   │   ├── rag.go               # RAG search subcommands (search/message/session)
│   │   ├── help.go              # --help self-documentation infrastructure
│   │   └── helpers.go           # Shared code (loadConfig/apiURL/httpDo/TLS/cookie)
│   ├── ai/                      # AI backend abstraction
│       ├── interface.go         # AIBackend interface
│       ├── factory.go           # Backend factory
│       ├── cli_backend.go       # Shared CLI backend abstraction
│       ├── common_stream.go     # Shared stream args/tool normalization/system prompt
│       ├── stream_parser.go     # Shared stream parsing utilities
│       ├── claude.go / claude_stream.go
│       ├── codebuddy.go / codebuddy_stream.go
│       ├── opencode.go / opencode_stream.go
│       ├── gemini.go / gemini_stream.go
│       ├── codex.go / codex_stream.go
│       ├── qoder.go / qoder_stream.go
│       ├── vecli.go / vecli_stream.go
│       ├── deepseek.go / deepseek_stream.go
│       └── pi.go / pi_stream.go
│   └── speech/                  # TTS speech synthesis
│       ├── common_tts.go        # CLISpeechProvider shared base
│       ├── minimax.go / edge.go / piper.go / kokoro.go / moss_tts_nano.go  # TTS engine implementations
│   └── summarize/               # Text summarization (TTS + task execution summaries)
│       ├── summarizer.go        # Summarizer interface + factory
│       ├── simple.go            # Plain text cleanup
│       ├── ai_backend_summarizer.go # AIBackendSummarizer (CLI backend summarization)
│       ├── mmx.go               # MMXSummarizer (mmx-cli text chat)
│       ├── openai.go            # OpenAI Chat Completions API summarization
│       ├── anthropic.go         # Anthropic Messages API summarization
│       ├── strip_markdown.go    # Markdown stripping
│       ├── task.go              # Task execution summary generation
├── web/src/components/common/  # Common components
│   ├── SummaryToggle.vue        # Summary toggle (button/tab modes)
├── config/                      # Configuration files
│   ├── rules.md                 # Agent shared rules and CLI reference
│   ├── agents/                  # Agent configurations
│   │   ├── assistant.yaml       # All-round assistant
│   │   ├── codebuddy2.yaml      # Gemini (via CodeBuddy)
│   │   ├── coder.yaml           # Coding expert
│   │   ├── codex.yaml           # Codex CLI
│   │   ├── gemini.yaml          # Gemini CLI
│   │   ├── gpt54.yaml           # GPT (via CodeBuddy)
│   │   ├── qoder.yaml           # Qoder CLI
│   │   ├── vecli.yaml           # VeCLI
│   │   ├── deepseek.yaml        # DeepSeek TUI
│   │   ├── pi.yaml              # Pi
│   │   └── handyman.yaml        # Handyman
├── web/                         # Vue 3 frontend source
│   └── src/
│       ├── components/          # Vue components
│       ├── composables/         # Composable functions (useQuickSend, useQuickCommands, useChatStream, etc.)
│       ├── stores/              # State management
│       └── utils/               # Utility functions
├── config/config.example.yaml   # Config template
├── build.sh                     # Build script (Linux/macOS)
├── build.ps1                    # Build script (Windows)
├── dev-server.sh                # Dev debug startup script (Linux/macOS)
├── server.sh                    # Production startup script (Linux/macOS)
├── server.ps1                   # Production startup script (Windows)
└── vite.config.ts               # Vite configuration
```

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.25+ (net/http + SQLite) |
| Frontend | Vue 3 + Vite + TypeScript |
| Syntax Highlighting | highlight.js |
| Markdown | marked.js |
| Chart Rendering | Mermaid.js |
| Math Formulas | KaTeX |
| HTML Sanitization | DOMPurify |
| AI Backend | CodeBuddy CLI / Claude Code CLI / OpenCode CLI / Gemini CLI / Codex CLI / Qoder CLI / VeCLI (streaming output → SSE push) |
| TTS Summarization | OpenAI/Anthropic compatible API (local or cloud, e.g. Ollama with `format: "openai"`) |
| SSH Tunnel | golang.org/x/crypto/ssh (embedded SSH server, direct-tcpip port forwarding) |
| Scheduled Scheduling | robfig/cron |

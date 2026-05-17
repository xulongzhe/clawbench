# AGENTS.md

## Project Overview

ClawBench is a mobile-first AI workstation wrapping AI CLI tools (CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, VeCLI, DeepSeek TUI, Pi) into a web-accessible platform. Go backend shells out to CLI tools and streams JSON output via SSE; Vue 3 frontend renders the streamed events in real time. Supports SSH tunnel-based port forwarding for remote/mobile access and a scheduled task (cron) system for recurring AI execution.

## Build & Run Commands

```bash
./build.sh                # Full build (Go binary + Vue frontend)
./build.sh --windows      # Cross-compile: Windows amd64
./build.sh --linux        # Cross-compile: Linux amd64
./build.sh --darwin       # Cross-compile: macOS arm64

./dev-server.sh           # Dev mode (Vite HMR proxy to production backend's dev HTTP port)
./dev-server.sh --fg      #   foreground
./dev-server.sh --stop    #   stop
./dev-server.sh --restart #   restart

./server.sh               # Production (port 20000)
./server.sh --fg          #   foreground
./server.sh --stop        #   stop

go build -o clawbench ./cmd/server   # Go binary only
go test ./...                        # All Go tests
go test ./internal/ai/...            # Package-specific
npm test                             # Vitest (all frontend tests)

# Android APK (requires JDK 17)
cd android && JAVA_HOME=/usr/lib/jvm/jdk-17.0.12 ./gradlew assembleDebug    # Debug APK
cd android && JAVA_HOME=/usr/lib/jvm/jdk-17.0.12 ./gradlew assembleRelease  # Release APK
```

## Architecture

### Backend (Go)

**Entry point:** `cmd/server/main.go` — config → port → LoadAgents → SyncDiscoverAgents (every-boot CLI detection, generate minimal YAMLs) → SyncDiscoverModels (first-run synchronous model cache) → MergeDiscoveredData (fill models/levels from cache + registry, soft-remove missing) → AsyncRefreshModelCache (background refresh) → scheduler init.

**Layers:**
- `internal/handler/` — HTTP handlers, SSE streaming (`chat_stream.go`), CRUD endpoints. All `/api/` routes use `middleware.Auth` (localhost bypass for CLI). Key handlers: `file.go` (read), `file_ops.go` (CRUD), `file_thumb.go` (thumbnail generation), `file_archive.go` (zip download), `file_watch.go` (SSE change notifications), `events.go` (system event SSE stream).
- `internal/service/` — Business logic: chat persistence, scheduler (cron via `robfig/cron/v3`), SQLite, ProxyRegistry, session runtime, EventBus pub/sub (`eventbus.go`).
- `internal/ai/` — AI backend abstraction. `AIBackend` interface with `ExecuteStream()`. `CLIBackend` is the shared base; each backend provides CLI args and a `LineParser`. `AutoResumeBackend` wraps claude/codebuddy/qoder/deepseek/pi — detects ExitPlanMode and auto-resumes with "继续". `NewBackend()` factory in `factory.go`.
- `internal/model/` — Data models, config structs, structured errors (`NotFound`, `Forbidden`, `Internal`), auto-discovery of AI CLIs. `BackendRegistry` declares backend specs (CLI command, model discovery, thinking levels). Model cache layer (`.clawbench/model-cache/`) persists discovered models; `DiscoverModels` / `DiscoverClaudeModels` / `ParseCodebuddyModels` handle per-backend discovery; `MergeDiscoveredData` fills runtime agents; `ModelsAutoDetected` distinguishes auto-detected vs. user-defined model lists.
- `internal/cli/` — CLI subcommands for AI agent self-service: `task` (CRUD + trigger + `list-exec`), `rag` (search), `migrate`.
- `internal/middleware/` — Auth, request logging, panic recovery, request ID.
- `internal/speech/` — TTS providers: MiniMax (cloud), Edge TTS (cloud, free), Piper/Kokoro/MOSS-Nano (local).
- `internal/summarize/` — Text summarization for TTS/task summaries. Supports AI backend CLIs, OpenAI/Anthropic HTTP APIs, and simple text cleanup.
- `internal/ssh/` — SSH tunnel server with direct-tcpip channels, password auth, auto-persisted host key. Publishes `tunnel_status` events via EventBus on client connect/disconnect.
- `internal/rag/` — RAG history memory: DuckDB vector store, Ollama BGE-M3 embeddings, chunking, indexing, search, cleanup.
- `internal/terminal/` — Interactive web terminal: PTY sessions, ring buffer replay, concurrent session management.
- `internal/push/` — Push notification abstraction. `JPushClient` wraps JPush API (SDK 6.1.0). AppKey fetched at runtime from server config (not baked into APK). `AppKey()` getter exposed via `/api/push/config` endpoint.
- `internal/ws/` — WebSocket event channel (`/api/ai/events/ws`). Replaces all frontend polling for session/task/queue status. `Manager` handles subscriptions, event broadcast, JPush fallback when client is disconnected, and buffered event replay on reconnect. Push Registration ID is stored at login level via HTTP API (not per-WS-connection).

**Agent system:** `config/agents/*.yaml` defines agents (id, backend, system_prompt, optional model, thinking_effort). Shipped YAMLs are minimal — `models` and `thinking_effort_levels` are auto-discovered at runtime and injected via `MergeDiscoveredData`. `BackendRegistry` in `discovery.go` declares each backend's discovery strategy: `ListModelsCmd+ParseModels` (e.g. CodeBuddy `--help`, OpenCode `models`) or `DiscoverModelsFunc` (e.g. Claude binary `strings` scan). First run populates model cache synchronously (`SyncDiscoverModels`); subsequent boots merge from cache; background `AsyncRefreshModelCache` keeps it fresh. `ModelsAutoDetected` flag on `Agent` tracks whether models came from discovery (updatable) vs. user-defined YAML (preserved). Auto-discovery generates minimal YAMLs for newly detected CLIs (`SyncDiscoverAgents`). `config/rules.md` is injected into every agent's system prompt — placeholders `{{AVAILABLE_AGENTS}}`, `{{PORT}}`, `{{PROJECT_PATH}}` are replaced dynamically.

**Data flow (chat):** POST `/api/ai/chat` → resolve agent → `NewBackend()` → `ExecuteStream()` spawns CLI → `LineParser` → SSE events → SQLite persistence. EventBus publishes `session_start` / `session_complete` events for real-time state notification.

**EventBus & system events:** `GlobalEventBus` (in-process fan-out pub/sub) publishes lightweight state-change events. SSE endpoint `GET /api/events` streams events to authenticated clients (cookie or `?token=` query param for native clients). 6 event types: `session_start` (AI session begins), `session_complete` (AI session ends, with reason: done/user_cancel/disconnect/cancelled/error), `message_new` (non-streaming message persisted), `task_update` (scheduled task CRUD), `task_exec_update` (task execution lifecycle), `tunnel_status` (SSH tunnel client connect/disconnect). Events are intentionally lightweight — payloads contain IDs and status only; clients fetch full data via REST on reconnect (`fullStateSync`). Max 20 concurrent SSE subscribers; buffered channels (256 entries) silently drop on overflow. 15s heartbeat keeps connections alive.

**Data flow (real-time events):** Backend state changes (session completed, task finished, queue updated) → `ws.Manager.BroadcastEvent()` → if WebSocket connected: send via WS; if disconnected + JPush configured + Registration ID available: send push notification; if disconnected + no JPush: buffer for replay on reconnect. Client sends `ack` for each event. JPush Registration ID is reported via `POST /api/push/register` (login-level lifecycle, not per-WS-connection) and persisted in `Manager` so it survives WS disconnects.

**Push notification flow:** Android `MainActivity.fetchPushConfig()` → `GET /api/push/config` (unauthenticated) → if `jpush_enabled`: `JPushInterface.init(context, config)` with runtime AppKey. After login, frontend calls `POST /api/push/register` (authenticated) with JPush Registration ID — this is a login-level lifecycle event persisted server-side, not tied to any individual WebSocket connection. When app goes to background: if push available → disconnect WebSocket (push delivers events); if push NOT available → keep WebSocket alive in background for real-time events.

**Scheduled tasks:** POST `/api/tasks` → cron trigger → creates chat session → executes AI backend → writes assistant message. `CLAWBENCH_SCHEDULED=1` env var for anti-recursion. AI agents manage tasks via `clawbench task` CLI. Zombie executions auto-cleaned on startup. EventBus publishes `task_update` / `task_exec_update` events for real-time status.

**Soft-delete:** Chat sessions/messages use `deleted=1` (not `DELETE FROM`) so RAG can still search them. `CleanupWorker` purges soft-deleted data past retention. Scheduled tasks use hard delete.

### Frontend (Vue 3 + TypeScript)

**Source root:** `web/src/` — No Vue Router, drawer-based single-page layout.

**State management:** Single `reactive()` store in `stores/app.ts` — no Pinia/Vuex.

**Key composables (chat):** `useChatSession` (CRUD), `useChatStream` (SSE + reconnect + polling fallback), `useChatRender` (block parsing + coalescing), `useAutoSpeech` (TTS), `useQuickSend` (SQLite CRUD), `useReconnect` (generic exponential backoff), `useFileRefresh` (file change detection + flash highlight), `useSessionIdentity` (model/thinking effort persistence), `useLocalhostAnnotation` (detect localhost URLs in chat, append port-forward + WebView open buttons; App mode only).

**Key composables (system events):** `useSystemEvents` (module-level singleton) — connects to `GET /api/events` SSE stream, handles 6 event types (`session_start`, `session_complete`, `message_new`, `task_update`, `task_exec_update`, `tunnel_status`). Updates reactive store state (`chatRunning`, `chatUnread`, `taskRunning`, `tunnelConnected`). 5 reconnect attempts with linear backoff (2s×attempt); falls back to degraded HTTP polling on exhaustion. `fullStateSync()` on every (re)connect fetches current state from 3 REST endpoints. Disconnects SSE on visibility change (background) to save battery; reconnects on foreground. Network `online` event triggers immediate reconnect.

**Key composables (events):** `useGlobalEvents` (WebSocket event channel singleton — session/task/queue status, push availability check, background visibility strategy: disconnect WS when push available, keep alive when not).

**Key composables (terminal):** `useTerminalSession` (WebSocket lifecycle), `useTerminalKeys` (modifier state machine), `useTerminalGestures` (touch swipe/pinch), `useTerminalViewport` (xterm.js + soft keyboard avoidance).

**Key components:** `ChatPanel`, `FileManager`/`FileViewer`, `TaskTab` (4-level breadcrumb), `TerminalPanel` (xterm.js + virtual keys + gestures), `GitGraph`, `BottomSheet`, `Lightbox`, `PopupMenu` (auto-positioning with scroll/resize tracking).

**Vite config:** `hljsThemeWrapper` plugin for light/dark theme coexistence. Root `web/`, output `public/`. Path alias `@` → `web/src/`.

## Key Patterns

- **Module-level singletons:** `useAutoSpeech()`, `useToast()`, `useSystemEvents()` — instantiate once, share state via module-level refs.
- **SSE reconnection:** 3 attempts → fallback to HTTP polling (2s). 15s heartbeat, 30s timeout. `online` event triggers immediate reconnect. System events SSE: 5 reconnect attempts with linear backoff (2s, 4s, 6s, 8s, 10s); degrades to HTTP polling on exhaustion.
- **Block coalescing:** Text/thinking events merge into last block of same type; `tool_use` acts as boundary.
- **AutoResumeBackend:** ExitPlanMode → cancel → resume with "继续". Emits `resume_split` for DB finalization.
- **Thinking effort:** Per-agent configurable via `thinking_effort` in YAML. `thinking_effort_levels` auto-populated from `BackendRegistry` at runtime (not stored in YAML). Passed as CLI flags (`--effort`, `--thinking`, etc.). Frontend chip selector, persisted per session (DB) and per agent (localStorage). Priority: frontend selection > YAML default > auto.
- **Cancel reason tracking:** `"user"` (explicit) vs `"disconnect"` (SSE gone). `ForceCancelSession` kills zombie CLI processes.
- **Green portable deployment:** All runtime data under `.clawbench/` next to binary. Delete that dir = clean uninstall. Copy binary dir for multi-instance isolation.
- **Zero-config startup:** `config/config.yaml` optional. `model.ApplyDefaults()` fills sensible defaults. Auto-generated password persisted to `.clawbench/auto-password`.
- **Touch device CSS:** Use `@media (hover: hover)` to scope `:hover` styles.
- **Structured errors:** `model.NotFound()`, `model.Forbidden()`, `model.Internal()` constructors.
- **Model auto-discovery:** Every boot: `SyncDiscoverAgents` detects CLIs and generates minimal YAMLs for new ones. Model lists discovered per-backend: `ListModelsCmd+ParseModels` (CodeBuddy, OpenCode, DeepSeek, Pi) or `DiscoverModelsFunc` (Claude via `strings` binary scan). Gemini, Codex, VeCLI, Qoder do not support CLI model listing — models must be user-defined in YAML. First run: `SyncDiscoverModels` (synchronous). Background: `AsyncRefreshModelCache` (updates agents with `ModelsAutoDetected=true`). User-defined models in YAML are preserved — only auto-detected model lists are refreshed.
- **Android HTML login:** Static `login.html` in `android/app/src/main/assets/` replaces native `AlertDialog`. WebView hidden during connection attempts to prevent error page flash. `AndroidNative` JS bridge provides `connectToServer()`, `getSavedServerConfig()`, `getAppVersion()`. Auto-connects on returning visits; login page shown only on first launch or connection failure.
- **Android SSE notifications:** `TunnelEventService` (foreground service) runs a native SSE listener thread connecting directly to `/api/events?token=` via HTTPS (independent of SSH tunnel). When app is backgrounded (`onPause`), SSH tunnel is shut down to save battery (SSE keeps running on its own HTTPS connection), and `session_complete` and `task_exec_update` events trigger system notifications via `clawbench_events` channel. When foregrounded (`onResume`), SSH tunnel is restored if there are port forwards, and native notifications are suppressed (WebView handles UI). Session token passed from WebView JS via `AndroidNative.setSessionToken()`. Service persists for SSE even without SSH tunnels.
- **Push-aware background strategy:** `useGlobalEvents.pushAvailable` ref tracks JPush availability. On `visibilitychange` to hidden: if `pushAvailable=true` → disconnect WebSocket (JPush delivers events); if `pushAvailable=false` → keep WebSocket alive in background. Same logic in `useChatStream` for SSE connections.
- **Runtime JPush AppKey:** JPush AppKey is NOT baked into APK. Android fetches it from `GET /api/push/config` at startup and initializes JPush via `JPushConfig.setjAppKey()` + `JPushInterface.init(context, config)`. Unauthenticated endpoint — only exposes `jpush_enabled` and `jpush_app_key`, no secrets.
- **Login-level push registration:** JPush Registration ID is reported via `POST /api/push/register` (authenticated, cookie-based) — not via WS `register` message. The Registration ID is a login-level lifecycle event persisted in `ws.Manager`, surviving WS disconnects/reconnects. WS authentication uses only Cookie (HTTP Upgrade carries cookies), no `?token=` fallback needed.

## Configuration

`config/config.yaml` is entirely optional. See `config/config.example.yaml`.

| Section | Key options |
|---------|------------|
| Server | `port` (20000), `host`, `log_level` ("info"), `watch_dir`, `password` (auto-UUID), `default_agent`, `dev_port` (0=auto, Port+2 when TLS) |
| Upload | `upload.max_size_mb`, `upload.max_files` |
| Chat UI | `chat.initial_messages`, `chat.page_size`, `chat.collapsed_height`, `chat.system_prompt_interval` (10) |
| Session | `session.max_count` |
| TLS | `tls.enabled`, `tls.cert_file`, `tls.key_file` |
| TTS | `tts.engine`, `tts.summarize_backend`, `tts.summarize_model`, `tts.speed`, `tts.voice`, `tts.max_cache_files` (100, auto-eviction) |
| Proxy | `proxy.enabled`, `proxy.allowed_ports` |
| SSH | `ssh.enabled`, `ssh.port`, `ssh.host_key` |
| RAG | `rag.enabled`, `rag.ollama_base_url`, `rag.ollama_model` (bge-m3), `rag.chunk_size` (512), `rag.retention_days` (90) |
| Terminal | `terminal.enabled` (true), `terminal.idle_timeout` (10m), `terminal.max_sessions` (10) |
| Tasks | `tasks.summarize_backend`, `tasks.summarize_model` |
| Push | `push.jpush.enabled`, `push.jpush.app_key`, `push.jpush.master_secret` |

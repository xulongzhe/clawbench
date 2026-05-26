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

# Coverage gate (CI 合入门槛)
./scripts/check-go-coverage.sh              # Go: run tests + check per-package coverage
./scripts/check-go-coverage.sh --skip-test   # Go: reuse existing coverage.out
./scripts/check-go-coverage.sh --update      # Go: auto-update baseline after coverage improvement
./scripts/check-frontend-coverage.sh              # Frontend: run tests + check per-dir coverage
./scripts/check-frontend-coverage.sh --skip-test   # Frontend: reuse existing coverage data
./scripts/check-frontend-coverage.sh --update      # Frontend: auto-update baseline after improvement
./scripts/check-android-coverage.sh              # Android: run tests + check per-class coverage
./scripts/check-android-coverage.sh --skip-test   # Android: reuse existing JaCoCo report

# Android APK (requires JDK 17)
cd android && JAVA_HOME=/usr/lib/jvm/jdk-17.0.12 ./gradlew assembleDebug    # Debug APK
cd android && JAVA_HOME=/usr/lib/jvm/jdk-17.0.12 ./gradlew assembleRelease  # Release APK
```

## Architecture

### Backend (Go)

**Entry point:** `cmd/server/main.go` — config → port → LoadAgents → SyncDiscoverAgents → SyncDiscoverModels → MergeDiscoveredData → AsyncRefreshModelCache → scheduler init.

**Packages:**
- `internal/handler/` — HTTP/SSE endpoints. All `/api/` routes use `middleware.Auth` (localhost bypass for CLI). Key: `chat_stream.go`, `file.go`/`file_ops.go`/`file_thumb.go`/`file_archive.go`, `file_watch.go`, `events.go`, `git.go` (history/branch/worktree/tag CRUD + swipe-to-delete + parameter injection protection), `settings.go` (password change + SHA-256 verification), `agent.go` (agent info + model refresh via `POST /api/agents/{id}/refresh-models`).
- `internal/service/` — Business logic: chat persistence, chat auto-summary (`summary.go`, `AsyncSummarize`), scheduler (`robfig/cron/v3`), SQLite, ProxyRegistry, session runtime, EventBus (`eventbus.go`).
- `internal/ai/` — AI backend abstraction. `AIBackend` interface → `CLIBackend` base (each backend provides CLI args + `LineParser`) → `AutoResumeBackend` wraps claude/codebuddy/qoder/deepseek/pi (ExitPlanMode → cancel → resume "继续"). Factory: `factory.go`.
- `internal/model/` — Data models, config, structured errors (`NotFound`/`Forbidden`/`Internal`), `BackendRegistry` (backend specs + model discovery). Model cache (`.clawbench/model-cache/`); `ModelsAutoDetected` flag distinguishes auto-discovered vs. user-defined model lists.
- `internal/cli/` — AI agent self-service: `task` (CRUD + trigger + `list-exec`), `rag` (search), `migrate`.
- `internal/middleware/` — Auth, request logging, panic recovery, request ID.
- `internal/speech/` — TTS: MiniMax/Edge TTS (cloud), Piper/Kokoro/MOSS-Nano (local).
- `internal/summarize/` — Text summarization for chat auto-summary, TTS, and task summaries (AI backends, OpenAI/Anthropic HTTP, text cleanup). `extractTextFromBlocks` exported for shared use.
- `internal/ssh/` — SSH tunnel server (direct-tcpip, password auth, auto host key). Publishes `tunnel_status` via EventBus.
- `internal/proxy/` — HTTP reverse proxy (`reverse_proxy.go`) + port forwarding logic. Solves SSH tunnel's TCP-level Host header mismatch for virtual-host backends by rewriting Host to match the original target. Privileged ports (< 1024) auto-remapped to non-privileged range for Android/non-root compatibility.
- `internal/rag/` — RAG: DuckDB vector store, Ollama BGE-M3 embeddings, chunking, indexing, search.
- `internal/terminal/` — Web terminal: PTY sessions, ring buffer replay, concurrent session management.
- `internal/push/` — Push notifications via JPush. AppKey from server config at runtime (not baked into APK). Exposed via `/api/push/config`.
- `internal/ws/` — WebSocket event channel (`/api/ai/events/ws`). Subscriptions, broadcast, JPush fallback on disconnect, buffered replay on reconnect. Push Registration ID persisted at login level via HTTP API.

**Agent system:** `config/agents/*.yaml` defines agents (id, backend, system_prompt, optional model, thinking_effort). Models and thinking levels auto-discovered at runtime via `BackendRegistry` strategies (`ListModelsCmd+ParseModels` or `DiscoverModelsFunc`). CodeBuddy uses `product.cloudhosted.json` parsing; Gemini uses API-based discovery; Codex uses binary strings/state DB scanning; Qoder uses `dynamic-texts.json` parsing; VeCLI uses `MODEL_REGISTRY` parsing. First run: `SyncDiscoverModels` (sync); background: `AsyncRefreshModelCache`. User-defined models preserved; only auto-detected lists refreshed. `CanRefreshModels` flag indicates which agents support runtime model refresh (triggered via `POST /api/agents/{id}/refresh-models`). `config/rules.md` injected into system prompts with `{{AVAILABLE_AGENTS}}`, `{{PORT}}`, `{{PROJECT_PATH}}` placeholders.

**Core data flows:**
- **Chat:** POST `/api/ai/chat` → resolve agent → `ExecuteStream()` spawns CLI → `LineParser` → SSE → SQLite. EventBus: `session_start` / `session_complete`. Session complete triggers `AsyncSummarize` for last assistant message if `chat_summary` enabled.
- **Chat auto-summary:** `AsyncSummarize` generates summary on session complete → saves to `summaries` table → WS `summary_update` event → frontend `SummaryToggle` (button/tab modes) toggles summary display. `tts_summaries` table (keyed by `message_id`) replaces old `cache_key`-based table.
- **Real-time events:** State change → `ws.Manager.BroadcastEvent()` → WS if connected; JPush if disconnected + configured; buffer for replay if neither. Client sends `ack`.
- **System events SSE:** `GET /api/events` streams 6 types: `session_start`, `session_complete`, `message_new`, `task_update`, `task_exec_update`, `tunnel_status`. Lightweight payloads (IDs + status only); clients `fullStateSync()` via REST on reconnect.
- **Push flow:** Android fetches config from `/api/push/config` → init JPush → reports Registration ID via `POST /api/push/register` (login-level lifecycle). Background: push available → disconnect WS (JPush delivers); push unavailable → keep WS alive.
- **Scheduled tasks:** POST `/api/tasks` → cron → chat session → AI backend. `CLAWBENCH_SCHEDULED=1` for anti-recursion. Managed via `clawbench task` CLI.
- **Soft-delete:** Chat sessions/messages use `deleted=1` (RAG still searchable). `CleanupWorker` purges past retention. Tasks use hard delete.
- **Chat auto-summary:** On session complete, `AsyncSummarize` generates a summary of the last assistant message and stores it in the `summaries` table (unified for chat + task). Frontend `SummaryToggle` component provides button mode (in `ChatMessageItem` meta bar) and tab mode (in `TaskExecDetail`). WS `summary_update` event pushes new summaries in real time. `tts_summaries` table uses `message_id` (replaces old `cache_key`). Config: `summarize.chat_summary` (default: true, `*bool` nil=true).

### Frontend (Vue 3 + TypeScript)

**Source root:** `web/src/` — No Vue Router, drawer-based single-page layout. Single `reactive()` store in `stores/app.ts`.

**Composables by function:**
- **Chat:** `useChatSession` (CRUD), `useChatStream` (SSE + reconnect + polling), `useChatRender` (block parsing + coalescing), `useAutoSpeech` (TTS, `messageId`-based), `useQuickSend` (SQLite CRUD + placeholder hint), `useSessionIdentity` (model/thinking persistence), `useLocalhostAnnotation` (localhost URL detection + port-forward/WebView buttons; App mode only), `useWorktreeAnnotation` (worktree path detection + switch/browse buttons; list-based cache, coexists with file path annotation), `useFileUpload` (multi-file upload with validation + file manager integration). Chat summary state (`showingSummary`) managed per-message via `chatSessionUtils`.
- **Connectivity:** `useReconnect` (generic exponential backoff), `useFileRefresh` (file change + flash highlight), `useSystemEvents` (SSE singleton, 6 event types, 5 reconnect → HTTP polling fallback, visibility-aware), `useGlobalEvents` (WS singleton, push-aware background strategy, `summary_update` event handling), `usePortForward` (port forwarding CRUD, reconnect button, pre-open tunnel health check).
- **Navigation:** `useBackHandler` (global back navigation registry for drill-down pages), `useEdgeSwipeBack` (right-edge swipe gesture for back navigation on mobile).
- **Terminal:** `useTerminalSession` (WS lifecycle), `useTerminalKeys` (modifier state machine), `useTerminalGestures` (swipe/pinch), `useTerminalViewport` (xterm.js + keyboard avoidance).
- **Git:** `useSwipeDelete` (direction-locked swipe-to-delete with offset clamping), `useCommitNavigation` (commit/branch navigation state), `useSwipeSession` (chat session swipe switching, toggle-able via settings).

**Components:** `ChatPanel`, `FileManager`/`FileViewer`, `TaskTab` (4-level breadcrumb), `TerminalPanel` (xterm.js + virtual keys + gestures), `GitGraph`, `GitManageContent` (3-tab: Worktree/Branches/Tags), `SwipeToDeleteRow`, `BottomSheet`, `Lightbox`, `PopupMenu` (auto-positioning), `ModelModal` (dual-tab model/thinking selection + search + refresh + set-default), `PasswordChangeDialog` (SHA-256 password update in settings), `SummaryToggle` (button/tab mode toggle for chat/task summaries).

**Vite:** `hljsThemeWrapper` plugin for light/dark coexistence. Root `web/`, output `public/`. Alias `@` → `web/src/`.

## Key Patterns

- **Module-level singletons:** `useAutoSpeech()`, `useToast()`, `useSystemEvents()` — instantiate once, share via module-level refs.
- **SSE reconnection:** Chat: 3 attempts → HTTP polling (2s). System events: 5 attempts linear backoff (2s×n) → HTTP polling. 15s heartbeat, 30s timeout. `online` event = immediate reconnect.
- **Block coalescing:** Text/thinking merge into last same-type block; `tool_use` is boundary.
- **AutoResumeBackend:** ExitPlanMode → cancel → resume "继续". Emits `resume_split` for DB finalization.
- **Thinking effort:** Per-agent via YAML. Levels auto-populated from `BackendRegistry`. CLI flags: `--effort`, `--thinking`, etc. Priority: frontend selection > YAML default > auto.
- **Cancel reason:** `"user"` (explicit) vs `"disconnect"` (SSE gone). `ForceCancelSession` kills zombie CLI processes.
- **Green portable deployment:** All runtime data under `.clawbench/`. Delete = clean uninstall. Copy binary dir for multi-instance.
- **Zero-config startup:** `config/config.yaml` optional. `model.ApplyDefaults()` fills defaults. Auto-password persisted to `.clawbench/auto-password`.
- **Model auto-discovery:** `SyncDiscoverAgents` detects CLIs → generates minimal YAMLs. `SyncDiscoverModels` (sync first run) + `AsyncRefreshModelCache` (background). CodeBuddy: `product.cloudhosted.json` parsing (21+ models). Gemini: API-based discovery. Codex: binary strings/state DB scanning. Qoder: `dynamic-texts.json` parsing. VeCLI: `MODEL_REGISTRY` parsing. All backends now support `DiscoverModelsFunc`; `CanRefreshModels` flag controls which agents expose runtime model refresh. `CheckCLIExistsErr` classifies CLI-not-found vs discovery-not-supported errors for user-facing messages.
- **Android integration:** HTML login (static `login.html` + `AndroidNative` JS bridge). `BackgroundService` manages SSH port forwarding + native WebSocket event channel for background notifications. JPush AppKey from `/api/push/config` (runtime, not in APK). Push-aware: `pushAvailable=true` → disconnect WS on background (JPush delivers); `false` → keep WS alive. Registration ID via `POST /api/push/register` (login-level lifecycle, survives WS reconnects).
- **SPA hot project switch:** Switching projects uses in-place state reset + Vue `:key` rebuild (0.15s fade transition) instead of `window.location.reload()`. `hotSwitchProject()` resets store singletons (agents, identity, project state), rebuilds component tree, reloads data — no page flicker.
- **Worktree annotation:** `useWorktreeAnnotation` fetches worktree list from backend (`ServeGitWorktrees`), builds search entries, and annotates matching paths in chat messages with switch/browse buttons. Runs before file path annotation to prevent partial matches on worktree directory prefixes (e.g., `/.worktrees`). File path annotation skips already-annotated worktree elements. Non-secure context fallback: `crypto.randomUUID()` replaced with `crypto.getRandomValues()` UUID v4 for HTTP access.
- **Edge swipe back:** `useBackHandler` provides a global registry for back navigation on drill-down pages (file browser, git history, settings, tasks). `useEdgeSwipeBack` detects right-edge swipe gestures for back navigation on mobile. Android `onBackPressed` delegates to JS layer — handled by JS prevents exit, unhandled falls through to native `super.onBackPressed()`.
- **Swipe session toggle:** `useSwipeSession` controls whether left/right swipe in chat area switches sessions. Default off to prevent accidental switches when scrolling wide content (code blocks, tables). Togglable via Settings → Chat.
- **Push notification preview:** Task completion push notifications include response preview text (last text after tool_use block) and `Done:` prefix. Session title used as notification title on completed/cancelled tasks.
- **Port forward reconnect & health check:** `usePortForward` adds a reconnect button for disconnected tunnels. Before opening a localhost URL, the system auto-checks tunnel health; if unhealthy, it attempts reconnection before proceeding. Android `BackgroundService.forceReconnectTunnel()` provides native-level reconnect.
- **Coverage gate (CI 合入门槛):** Two-tier check. Tier 1 (Project): per-package/directory coverage `>= baseline% - 1.5%`, baseline from CI artifact. Exempt files excluded from both Tier 1 and Tier 2 calculations. Tier 2 (Diff): changed lines coverage `>= 80%`. CI enforces on every PR/push to main.
- **SHA-256 password storage:** Passwords stored as SHA-256 salted hashes (not plaintext). Auto-password and user-configured passwords both use the same hashing scheme. Password change via `POST /api/settings/password` with current password verification. `PasswordChangeDialog` component in settings panel.
- **Model selection modal:** `ModelModal` component provides unified model switching and thinking effort selection in a dual-tab interface. Replaces per-PopupMenu approach in chat input bar. Supports search filtering, runtime model refresh (`CanRefreshModels`), long-press to set default model, and auto-refresh on open. Removed agent preferences from Settings page (model/thinking now in modal only).
- **Bugfix workflow (GitHub Issues):** All AI agents should report newly discovered bugs as GitHub Issues (`gh issue create`). A scheduled task runs hourly to auto-fix open issues: fetches open issues without any `bugfix:*` label (sorted by creation time) → evaluates complexity → claims and fixes in bugfix worktree (`.worktrees/bugfix`) → writes test → builds → starts on port 20100 → auto-verifies → updates issue with result. Max 3 fixes per run. Merge to main requires manual trigger. Issue labels track state: none = pending, `bugfix:in-progress` = AI claimed, `bugfix:awaiting-review` = fixed awaiting human verification, `bugfix:needs-design` = too complex for auto-fix (skipped), `bugfix:failed` = auto-fix failed. Human closes issue after verification.

## Configuration

`config/config.yaml` is entirely optional. See `config/config.example.yaml`.

| Section | Key options |
|---------|------------|
| Server | `port` (20000), `host`, `log_level` ("info"), `watch_dir`, `password` (auto-UUID, SHA-256 salted hash storage), `default_agent`, `dev_port` (0=auto, Port+2 when TLS) |
| Upload | `upload.max_size_mb`, `upload.max_files` |
| Chat UI | `chat.initial_messages`, `chat.page_size`, `chat.collapsed_height`, `chat.system_prompt_interval` (10) |
| Session | `session.max_count` |
| Recent Projects | `recent_projects.max_count` (10, configurable limit for header dropdown) |
| TLS | `tls.enabled`, `tls.cert_file`, `tls.key_file` |
| TTS | `tts.engine`, `tts.speed`, `tts.voice`, `tts.max_cache_files` (100, auto-eviction) |
| Summarize | `summarize.backend` ("simple"), `summarize.model`, `summarize.api` (unified for TTS + task summaries), `summarize.chat_summary` (true, auto-summarize chat on session complete; `*bool` nil=true) |
| Port Forward | `port_forward.enabled` (true), `port_forward.port` (0=auto), `port_forward.host_key`, `port_forward.allowed_ports` (""=all) |
| RAG | `rag.enabled`, `rag.ollama_base_url`, `rag.ollama_model` (bge-m3), `rag.chunk_size` (512), `rag.retention_days` (90) |
| Terminal | `terminal.enabled` (true), `terminal.idle_timeout` (10m), `terminal.max_sessions` (10) |
| Push | `push.jpush.enabled`, `push.jpush.app_key`, `push.jpush.master_secret` |

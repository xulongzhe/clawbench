# CODEBUDDY.md

This file provides guidance to CodeBuddy Code when working with code in this repository.

## Project Overview

ClawBench is a mobile-first AI workstation that wraps AI CLI tools (CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, VeCLI) into a web-accessible platform. Go backend shells out to CLI tools and streams JSON output via SSE; Vue 3 frontend renders the streamed events in real time. Supports SSH tunnel-based port forwarding for remote/mobile access and a scheduled task (cron) system for recurring AI execution.

## Build & Run Commands

```bash
# Full build (Go binary + Vue frontend)
./build.sh

# Cross-compile
./build.sh --windows    # Windows amd64
./build.sh --linux      # Linux amd64
./build.sh --darwin     # macOS arm64

# Dev mode (Go dev backend on :20002 + Vite HMR on :20001)
./dev-server.sh
./dev-server.sh --fg       # foreground
./dev-server.sh --stop
./dev-server.sh --restart

# Production mode (port 20000)
./server.sh
./server.sh --fg
./server.sh --stop

# Frontend only
npm run dev          # Vite dev server (port 20001, proxies /api to :20000)
npm run build        # Production build -> public/
npm run preview      # Serve production build

# Go backend only
go build -o clawbench ./cmd/server
go test ./...                        # All Go tests
go test ./internal/ai/...            # Package-specific tests
go test -run TestStreamParser ./internal/ai/  # Single test

# Frontend tests
npm test             # Vitest (all tests)
npx vitest run web/src/components/__tests__/gitGraphUtils.test.ts  # Single test file
```

## Architecture

### Backend (Go)

**Entry point:** `cmd/server/main.go` — loads config, initializes SQLite, starts HTTP server, SSH tunnel server (if enabled), scheduler, and ProxyRegistry. Startup order: port → LoadAgents → scheduler init (LoadTasksFromDB runs after LoadAgents to ensure agent_id resolution succeeds).

**Layered structure:**
- `internal/handler/` — HTTP handlers (routes registered in `handler.go`). SSE streaming in `chat_stream.go`, scheduled task CRUD in `scheduler.go`, port forwarding API in `proxy_api.go`, SSH info in `ssh_info.go`, session CRUD in `chat_session.go`, RAG search API in `rag_api.go`, chat quick-send CRUD in `chat_quick_send.go`, terminal quick commands CRUD in `terminal.go`. All `/api/` routes use `middleware.Auth` (with `isLocalhost()` bypass for CLI access).
- `internal/service/` — Business logic: `chat.go` (history/persistence), `scheduler.go` (cron-based AI task execution via `robfig/cron/v3`), `database.go` (SQLite, including `chat_quick_send` and `terminal_quick_commands` tables), `proxy.go` (ProxyRegistry: port forwarding with health checks, auto-detection, TLS probing), `session_runtime.go` (active session tracking, stream channels, cancel functions with reason tracking).
- `internal/ai/` — AI backend abstraction. `AIBackend` interface (`interface.go`) with `ExecuteStream()`. `CLIBackend` (`cli_backend.go`) is the shared base that shells out to CLI tools; each backend (claude/codebuddy/opencode/gemini/codex/qoder/vecli) provides CLI args and a `LineParser` for its JSON output format. Stream parsers are in `*__stream.go` files. `AutoResumeBackend` (`auto_resume.go`) wraps claude, codebuddy, and qoder backends — detects ExitPlanMode tool_use and automatically resumes with "继续". `CodexBackend` (`codex.go`) provides full Codex CLI integration with resume support. `VeCLIBackend` (`vecli.go`) wraps CLIBackend to add post-stream session-summary parsing — VeCLI outputs plain text (not JSON Lines), so metadata (token counts, duration, model) is extracted from a `--session-summary` JSON file after process exit. `NewBackend()` factory in `factory.go`. Qoder backend (`qoder.go`) reuses the shared `StreamParser` since its `--output-format stream-json` produces the same NDJSON format as Claude/Codebuddy.
- `internal/model/` — Data models, config structs, path validation, structured error types (`errors.go`: `NotFound`, `Forbidden`, `Internal`, etc.), scheduled task model, proxy/SSH config models.
- `internal/cli/` — CLI subcommands for AI agent self-service. `task.go` (create/update/delete/pause/resume/trigger/list-agents), `rag.go` (search/message/session), `help.go` (HelpInfo/FlagHelp infrastructure for `--help` self-documentation), `helpers.go` (shared code: loadConfig/apiURL/httpDo, TLS self-signed cert support, cookie-based project path injection). AI agents call these via Bash instead of HTTP endpoints.
- `internal/middleware/` — Auth (with `isLocalhost()` bypass for CLI access), request logging, panic recovery, request ID.
- `internal/speech/` — TTS abstraction (`SpeechProvider` interface). Implementations: MiniMax (cloud), Edge TTS (cloud, free), Piper (local offline), Kokoro (local ONNX-based). `summarizer.go` provides TTS summarization via multiple AI backends (mmx-cli, claude, codebuddy, gemini, opencode, codex, qoder, vecli, ollama) for long-text compression before speech. `ollama_summarizer.go` calls Ollama HTTP API (`/api/chat`, stream:false) — the first direct HTTP client in the Go backend (all others shell out to CLI tools).
- `internal/ssh/` — SSH tunnel server (`server.go`). Supports direct-tcpip channels (-L port forwarding), password auth, ECDSA host key generation/persistence. Integrates with ProxyRegistry for port validation.
- `internal/rag/` — RAG history memory system. DuckDB vector store (`store.go`), text chunker (`chunker.go`), Ollama embedding client (`embedding.go`), indexer worker (`indexer.go`), search (`search.go`), cleanup worker (`cleanup.go`), entry point (`rag.go`). When `rag.enabled`, indexes chat messages after finalization and provides semantic search API. Cleanup worker runs regardless of RAG enablement to purge soft-deleted data past retention.
- `internal/terminal/` — Interactive web terminal. `manager.go` (concurrent sessions map keyed by session ID, session limit enforcement, auto-cleanup via onClose callback), `session.go` (PTY I/O pump, idle timeout, ring buffer replay, resize handling, auto-generated session ID), `buffer.go` (RingBuffer: configurable line count/size/memory cap, line-split replay), `shell.go` + `shell_posix.go` / `shell_windows.go` (shell detection: $SHELL→/bin/sh on POSIX, pwsh→powershell→cmd on Windows; process group kill via SIGTERM→3s→SIGKILL), `protocol.go` (WebSocket JSON message types, SessionID field, ErrCodeSessionLimit error code). Handler in `internal/handler/terminal.go` (WebSocket upgrade, per-session status/close, quick commands CRUD endpoints).
- `internal/platform/` — Platform-specific adaptations (Windows paths).

**Agent system:** YAML files in `config/agents/` define agents with id, backend, model, system_prompt, and optional `command` (custom CLI path). `config/rules.md` is always fully injected into every agent's system prompt at startup by `model.LoadAgents()` → `BuildCommonPrompt()`. It contains mandatory rules and CLI references (scheduled tasks, RAG search, etc.). Placeholders `{{AVAILABLE_AGENTS}}`, `{{PORT}}`, and `{{PROJECT_PATH}}` are replaced dynamically — `{{PROJECT_PATH}}` is replaced per-request using the project path from the cookie (not statically at startup). The `<!-- SCHEDULED_BEGIN/END -->` markers in rules.md wrap the scheduled tasks section, which is stripped by `BuildCommonPrompt(true)` during scheduled executions (anti-recursion). The skill system (`config/skills/`, `/api/skills` endpoints) has been removed — `rules.md` is now the single source of truth for all mandatory rules.

**Data flow for chat:**
1. Frontend sends POST to `/api/ai/chat`
2. Handler resolves agent config → creates `AIBackend` via `ai.NewBackend()`
3. `CLIBackend.ExecuteStream()` spawns CLI process, reads stdout line-by-line
4. Backend-specific `LineParser` converts JSON lines → `StreamEvent` channel
5. Handler relays events as SSE (`content`, `thinking`, `tool_use`, `tool_result`, `metadata`, `done`, `cancelled`, `warning`, `resume_split`, `raw_output`)
6. Messages are persisted to SQLite asynchronously

**Scheduled task system:**
1. Frontend sends POST to `/api/tasks` with cron expression, agent ID, prompt, repeat mode (once/limited/unlimited)
2. `service.Scheduler` registers cron job via `robfig/cron/v3`
3. On trigger, scheduler calls the agent's `AIBackend.ExecuteStream()` and persists results as chat messages with `scheduledTask` metadata. `CLAWBENCH_SCHEDULED=1` env var is injected for anti-recursion protection.
4. Frontend manages tasks via `/api/tasks` CRUD endpoints
5. AI agents can also manage tasks via `clawbench task` CLI subcommands (create/update/delete/pause/resume/trigger), which is the preferred method for AI-driven task creation. The old `<schedule-proposal>` passive tag detection system has been removed.

**SSH tunnel / port forwarding:**
1. SSH server listens on `port+1` (or configured port)
2. Android app connects via SSH and opens direct-tcpip channels
3. `ProxyRegistry` manages forwarded ports with health checks (5s interval), auto-detection, TLS probing

**RAG history memory system:**
1. When `rag.enabled: true`, chat messages are indexed into DuckDB vector store after finalization
2. `chat_history.indexed` column tracks indexing state; indexer polls every 10s for unindexed messages
3. Text blocks are extracted (excluding thinking/tool_use), chunked with 512-token sliding window, embedded via Ollama BGE-M3
4. AI agents search history via `clawbench rag` CLI subcommands (search/message/session), which call the backend RAG API endpoints (`/api/rag/search`, `/api/rag/message`, `/api/rag/session`). RAG search rules and CLI reference are in `config/rules.md`
5. Dev mode uses separate `rag-dev.duckdb` to avoid production DB conflict

**Interactive terminal:**
1. Frontend opens WebSocket to `GET /api/terminal/ws?cwd=<dir>`
2. Handler resolves cwd, creates PTY session via `TerminalManager`; each client gets an independent session (auto-generated 8-byte hex ID)
3. Session pump: PTY stdout → RingBuffer → WebSocket (`output` messages); WebSocket `input` → PTY stdin
4. On reconnect, client appends `&session=<id>` to WS URL to resume its specific session; RingBuffer replays buffered lines via `replay` message
5. `resize` messages sync terminal dimensions (cols/rows) to PTY
6. Multiple concurrent sessions per project (configurable `max_sessions`, default: 10); process exit auto-removes session via `onClose` callback
7. Close via `POST /api/terminal/close` or `close` WebSocket message → SIGTERM process group → SIGKILL
8. Quick commands: CRUD via `/api/terminal/quick-commands` endpoints; stored in SQLite `terminal_quick_commands` table; supports drag reorder, hidden flag, auto-execute (one command auto-runs on every connect/reconnect)

**Soft-delete & cleanup:**
1. Session deletion sets `deleted=1` on `chat_sessions` and `chat_history` instead of `DELETE FROM` — data stays in DB for RAG search
2. User-facing queries filter `AND deleted = 0`; RAG-specific queries (`GetMessageByID`, `GetMessagesBySessionID`, `GetUnindexedMessages`) intentionally skip the filter so deleted content remains searchable
3. `DeleteSession` also sets `updated_at = CURRENT_TIMESTAMP` to track deletion time
4. `AddChatMessage` rejects inserts into deleted sessions as a defensive guard
5. `CleanupWorker` (always runs, regardless of `rag.enabled`) periodically purges soft-deleted data older than `rag.retention_days` (default: 90). Cascade order: DuckDB `chat_chunks` → SQLite `ai_raw_responses` → `chat_history` → `chat_sessions`

**Session runtime management** (`session_runtime.go`):
- Mutex-protected active session tracking, stream channels via `sync.Map`
- Cancel functions with reason tracking: `"user"` (explicit cancel) vs `"disconnect"` (SSE client gone)
- `ForceCancelSession` kills zombie CLI processes on SSE disconnect

### Frontend (Vue 3 + TypeScript)

**Source root:** `web/src/`

**State management:** Single reactive store in `stores/app.ts` (project, files, navigation history, upload config, chat UI config, session limits, chat unread badge). No Pinia/Vuex — plain `reactive()` object.

**App structure** (`App.vue`): AppHeader (project root, theme toggle, project dialog) + bottom dock with navigation: (Chat, Files, History, Port Forward [app mode only], Refresh). File navigation (back/forward) moved to `FileHeader.vue` capsule button group. Drawers are mutually exclusive — opening one closes others. `ChatPanel` is a `BottomSheet` component. Authentication flow includes auto-login for Android app mode.

**Chat architecture** (the most complex UI feature):
- `ChatPanel.vue` — Orchestrator; composes composables and child components.
- `ChatMessageList.vue` — Virtual list of messages with lazy loading.
- `ChatMessageItem.vue` — Renders a single message with expandable tool calls, thinking blocks, inline actions, double-click copy, long-press context menu.
- `ChatInputBar.vue` — Input area with attach menu, auto-speech toggle, quick-send presets (managed via `useQuickSend` composable, stored in SQLite).
- `ChatMetadataModal.vue` — Token usage / model info modal.
- `QuickSendDialog.vue` — Quick-send CRUD dialog with drag reorder and inline edit.
- `useChatSession.ts` — Session CRUD, history loading, agent resolution, message count polling.
- `useChatStream.ts` — SSE connection, event parsing into message blocks, reconnection logic (3 attempts then fallback to polling), stream timeout handling.
- `useChatRender.ts` — Markdown rendering, block parsing (text/thinking/tool_use/tool_result/blockTasks), content coalescing. Tool result events update existing tool_use blocks with output and status. Scheduled task cards (`blockTasks`) replace the old `schedule-proposal` tag system.
- `useAutoSpeech.ts` — Auto-read toggle (module-level singleton ref), TTS playback via backend `/api/tts/generate`.
- `useMarkdownRenderer.ts` — Markdown rendering with highlight.js, KaTeX math, Mermaid diagrams.
- `useFileUpload.ts` — File upload handling with size/count limits from config.
- `useAgents.ts` — Agent listing, icon, name resolution.
- `useFilePathAnnotation.ts` — File path resolution and inline annotation.
- `useDoubleClickCopy.ts` — Double-click to copy code block text.
- `useLongPressLineMenu.ts` — Long-press context menu on code lines.
- `useSwipeNavigation.ts` — Swipe gestures for file navigation.
- `useSwipeSession.ts` — Swipe between chat sessions.
- `usePortForward.ts` — Port forwarding state and SSH info.
- `useAppMode.ts` — Android WebView detection, native bridge integration (addForwardedPort, openInBrowser, showServerDialog, setSSHPassword, getPassword, setVolumeKeyMode).
- `useNotificationSound.ts` — Notification sound + haptic feedback.
- `useNotification.ts` — Push notification support.
- `useToast.ts` — Toast notification system.
- `useQuickSend.ts` — Chat quick-send CRUD (module-level singleton), stored in SQLite, drag reorder with optimistic update and rollback.
- `useQuickCommands.ts` — Terminal quick commands CRUD (module-level singleton), stored in SQLite, drag reorder with optimistic update and rollback.

**Terminal architecture:**
- `terminal/TerminalPanel.vue` — BottomSheet container; composes all terminal UI: xterm.js viewport, virtual key toolbar (color-coded groups with visual dividers: modifiers, shortcuts, navigation, arrows, actions), toggleable symbol bar above main toolbar (19 symbols with exponential-decay smart sorting), gesture overlay, touch scroll in gesture-disabled mode, thin scrollbar indicator, quick commands popup, connection status dot.
- `terminal/terminalCwd.ts` — CWD resolution logic (current file dir > file manager dir > requested cwd; mismatch detection for reopen prompt).
- `terminal/QuickCommandDialog.vue` — Quick commands CRUD dialog with two-page drill-down (list with drag reorder, edit with label/command/hidden/auto_execute fields).
- `useTerminalSession.ts` — WebSocket lifecycle (connect/disconnect/reconnect with 3 attempts), message parsing (input/resize/close → server; output/replay/status/exit/error ← server), idle timeout handling.
- `useTerminalViewport.ts` — xterm.js FitAddon integration, ResizeObserver + visualViewport tracking (soft keyboard avoidance), debounced terminal resize.
- `useTerminalKeys.ts` — Modifier key state machine (inactive/once/locked for Ctrl/Alt/Shift), `processInput()` transforms (Ctrl+A→\x01, Alt+X→\x1bX, Shift+Tab→\x1b[Z), send functions for all special keys (arrows, Home/End/PgUp/PgDn/Enter/Backspace/Delete/Ctrl+C/Ctrl+Z/Escape/Tab).
- `useTerminalGestures.ts` — Termius-style touch gestures: swipe→arrow keys, hold-to-repeat, double-tap→Tab, pinch→zoom. Toggle on/off for xterm.js native touch selection compatibility. When gestures are disabled, separate touch listeners provide scroll-by-drag (`term.scrollLines()`) for mobile viewport scrolling.

**Other key components:**
- `file/FileManager.vue` + `FileViewer.vue` — Directory browser and file viewer (code/markdown/media). `FileDetailsDialog.vue` for file metadata. `FileHeader.vue` for viewer header with capsule navigation button group (back/forward).
- `git/` — Git history, diff view, branch graph (GitGraph, GitHistoryDrawer, GitDiffView, GitCommitList, GitCommitMeta, GitBreadcrumb).
- `session/` — SessionDrawer, SessionManager, SessionSelector for chat session management.
- `task/` — TaskDrawer, TaskFormDialog for scheduled task management. TaskFormDialog supports frequency presets (hourly/daily/weekly/monthly) with visual time selectors and custom cron input; pause/resume in edit mode. TaskDrawer shows status dots (active/paused/completed), running indicator, and unread badges.
- `proxy/` — ProxyPanel, PortForwardBrowser, ProxyPortItem for port forwarding UI (app mode only).
- `common/BottomSheet.vue` — Mobile-friendly bottom sheet drawer.
- `common/AppHeader.vue` — Top header bar with project name, theme toggle.
- `common/ModalDialog.vue` — Generic modal dialog.
- `common/SearchDrawer.vue` + `SearchInput.vue` — Search within files.
- `common/HeaderMarquee.vue` — Scrolling header text.
- `common/ToastNotification.vue` — Global toast notifications.
- `common/PWAInstallPrompt.vue` — PWA install prompt for browsers.
- `media/Lightbox.vue` — Image zoom/pan viewer (singleton, teleported to body).
- `media/AudioPreview.vue` + `VideoPreview.vue` — Inline media players.
- `LoginView.vue` — Authentication screen.
- `WelcomeView.vue` — Empty state landing page.
- `TocDrawer.vue` — Table of contents drawer for markdown files.
- `ProjectDialog.vue` — Project selection dialog.

**Utility modules** (`web/src/utils/`):
- `api.ts` — API helpers (apiGet, apiPost, apiPut, apiDelete) with AbortController + 10s timeout for resilience against unresponsive servers.
- `diff.ts` — Diff utilities for git views.
- `fileType.ts` — File type detection.
- `format.ts` — Formatting utilities.
- `gitGraph.ts` — Git graph rendering.
- `globals.ts` — Shared singletons (marked, hljs instances).
- `helpers.ts` — General helper functions.
- `html.ts` — HTML utilities.
- `mermaid.ts` — Mermaid diagram rendering.
- `path.ts` — Path utilities.
- `pwa-install.ts` — PWA install prompt logic.
- `renderToolDetail.ts` — Tool detail rendering for chat messages.
- `toc.ts` — Table of contents extraction.

**Vite config** (`vite.config.ts`): Custom plugin `hljsThemeWrapper` wraps highlight.js CSS with `[data-hljs-theme]` attribute selectors so light/dark themes coexist. Root is `web/`, build output goes to `public/`. Dev proxy forwards `/api` to Go backend. `allowedHosts` for remote access.

**Path alias:** `@` → `web/src/`

**No Vue Router** — navigation is entirely drawer-based within a single-page layout.

## Key Patterns

- **Module-level singletons:** `useAutoSpeech()` uses module-level refs so all consumers share the same state. Only instantiate once (in ChatPanel). Same pattern for `useToast()`.
- **SSE with reconnection:** `useChatStream` handles SSE disconnects with up to 3 reconnects, then falls back to HTTP polling every 2s. 60s timeout with no events triggers reconnect.
- **Block coalescing:** Streamed text/thinking events are merged into the last block of the same type (unless separated by a `tool_use` block which acts as a boundary).
- **Drawer mutual exclusion:** `App.vue` manages all drawer open states (chat, fileManager, projectHistory, fileHistory, toc, search, details, proxy); opening one instantly closes others.
- **AutoResumeBackend:** Wraps claude, codebuddy, and qoder backends. Detects ExitPlanMode tool_use → cancels CLI → resumes with "继续" in same session. Emits `resume_split` event for DB message finalization. Transparent to outer caller.
- **Cancel reason tracking:** Session cancels are tracked as `"user"` (explicit) or `"disconnect"` (SSE client gone). `ForceCancelSession` kills zombie CLI processes on disconnect.
- **ProxyRegistry health checks:** Forwarded ports are probed every 5s; auto-detection scans `/proc/net/tcp` (Linux), `lsof` (macOS), `netstat` (Windows); TLS probing for HTTPS ports.
- **Android native bridge:** `useAppMode()` detects Android WebView via JS bridge (`AndroidNative.*`). Supports auto-login, port forwarding registration, SSH password management, native dialogs, and volume key mode (setVolumeKeyMode: remap volume up/down to arrow keys when terminal is open).
- **Touch device CSS:** Use `@media (hover: hover)` to scope `:hover` styles — touch devices get sticky hover that masks `.active` class changes.
- **Green portable deployment:** All runtime data (SQLite DB, logs, uploads, SSH host keys, TTS models, auto-generated password) lives under `.clawbench/` next to the binary. Deleting that directory = clean uninstall.
- **Zero-config startup:** `config/config.yaml` is optional. `model.ApplyDefaults()` (in `defaults.go`) fills all zero-value fields with sensible defaults. When `password` is empty, a random UUID is generated and persisted to `.clawbench/auto-password` for reuse across restarts. `ParsePresenceMap()` handles the bool-defaults problem (Go zero value is `false`, but `proxy.enabled` and `ssh.enabled` should default to `true`).
- **Structured errors:** Backend uses `model.NotFound()`, `model.Forbidden()`, `model.Internal()` constructors for consistent HTTP error responses.
- **Terminal virtual key groups:** Toolbar keys are grouped by type (modifiers, shortcuts, navigation, arrows, actions) with color-coded visual dividers. Modifier keys use three-state toggle (inactive→once→locked); once auto-clears after next keypress, locked persists until tapped again. Arrow keys and Esc/Tab/PgUp/PgDn hide when gestures are enabled (handled by touch gestures instead). Gesture toggle and symbol toggle buttons sit outside the scroll area. Symbol bar is a separate row above the main toolbar with 19 terminal symbols, sorted by exponential-decay frequency (each click updates score with recency weighting; half-life ~4.6h; re-sorted on every bar open; persisted to localStorage).
- **Terminal drawer lifecycle:** Closing the terminal drawer (❌ button, swipe-down, parent hides) disconnects WebSocket + disposes xterm instance. Next open creates a fresh Terminal + new PTY session. `cleanupTerminal()` consolidates disposal logic. Multiple concurrent sessions are supported — each client gets its own PTY session with independent state.
- **Tool execution results:** `tool_result` SSE events carry output text and success/error status from AI backend tool calls. `useChatRender` accumulates these into existing `tool_use` blocks. `ContentBlocks.vue` renders a spinner while pending, green check on success, red X on error, with expandable output section. `StreamParser` suppresses `text_delta` events belonging to `tool_result` blocks to prevent tool output from leaking into content. Output is capped at 50KB via `truncateToolOutput()`.
- **CRUD migration from YAML to SQLite:** Chat quick-send and terminal quick commands were migrated from static YAML config to SQLite-backed CRUD APIs. Both use the same pattern: module-level singleton composable (`useQuickSend` / `useQuickCommands`), drag reorder with optimistic update and rollback, separate handler files for CRUD endpoints. The old `chat.quick_send` YAML section and `terminal.quick_commands` YAML section are no longer used.

## Configuration

`config/config.yaml` is entirely optional — all settings have sensible defaults. See `config/config.example.yaml` for available options.

| Section | Key options |
|---------|------------|
| Server | `port` (default: 20000), `watch_dir` (default: user home), `password` (default: auto-generated UUID saved to `.clawbench/auto-password`) |
| Upload | `upload.max_size_mb`, `upload.max_files` |
| Chat UI | `chat.initial_messages`, `chat.page_size`, `chat.collapsed_height` |
| Session | `session.max_count` |
| TLS | `tls.enabled`, `tls.cert_file`, `tls.key_file` |
| TTS | `tts.engine` (minimax/edge/piper/kokoro/moss-nano), `tts.summarize_backend` (mmx-cli/claude/codebuddy/gemini/opencode/codex/qoder/vecli/ollama), `tts.summarize_model`, `tts.speed`, `tts.voice`, engine-specific sub-configs, `tts.ollama.base_url` |
| Proxy | `proxy.enabled`, `proxy.allowed_ports` |
| SSH | `ssh.enabled`, `ssh.port`, `ssh.host_key` |
| RAG | `rag.enabled`, `rag.ollama_base_url`, `rag.ollama_model` (bge-m3), `rag.chunk_size` (512), `rag.chunk_overlap` (64), `rag.poll_interval` (10s), `rag.batch_size` (10), `rag.search_limit` (5), `rag.retention_days` (90) |
| Terminal | `terminal.enabled` (default: true), `terminal.idle_timeout` (default: 10m), `terminal.buffer_lines` (default: 2000), `terminal.max_line_bytes` (default: 65536), `terminal.max_buffer_mb` (default: 4), `terminal.max_sessions` (default: 10) |
| Dev | `dev.port`, `dev.frontend_port`, `dev.host` |
| Logging | `log_dir`, `log_max_days`, `default_agent` |

Dev mode uses separate port (20002), database (`ClawBench-dev.db`), and RAG database (`rag-dev.duckdb`).

## Testing

- Go tests use `testify/assert`. Test files colocated with source (`*_test.go`).
- Frontend tests use Vitest + `@vue/test-utils`. Located in `web/src/components/__tests__/`.
- Many handler tests need a running test server — see `testutil_test.go` in handler package.
- Key test packages: `ai/` (stream parsers, auto-resume, factory, tool_result accumulation), `handler/` (auth, chat, files, git, proxy, scheduler, SSH info, TTS, terminal handler auth + cwd validation, chat quick-send CRUD), `service/` (chat, proxy, scheduler, stream, uuid, soft-delete, cleanup, database), `speech/` (minimax, piper, kokoro, moss_tts_nano, ollama), `ssh/` (server), `rag/` (chunker, store, cleanup), `terminal/` (ring buffer, session/manager), `cli/` (task subcommands, rag subcommands, help infrastructure), `middleware/` (auth with localhost bypass).

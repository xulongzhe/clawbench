# CODEBUDDY.md

This file provides guidance to CodeBuddy Code when working with code in this repository.

## Project Overview

ClawBench is a mobile-first AI workstation that wraps AI CLI tools (CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex) into a web-accessible platform. Go backend shells out to CLI tools and streams JSON output via SSE; Vue 3 frontend renders the streamed events in real time. Supports SSH tunnel-based port forwarding for remote/mobile access and a scheduled task (cron) system for recurring AI execution.

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

**Entry point:** `cmd/server/main.go` — loads config, initializes SQLite, starts HTTP server, SSH tunnel server (if enabled), scheduler, and ProxyRegistry.

**Layered structure:**
- `internal/handler/` — HTTP handlers (routes registered in `handler.go`). SSE streaming in `chat_stream.go`, scheduled task CRUD in `scheduler.go`, port forwarding API in `proxy_api.go`, SSH info in `ssh_info.go`, session CRUD in `chat_session.go`.
- `internal/service/` — Business logic: `chat.go` (history/persistence), `scheduler.go` (cron-based AI task execution via `robfig/cron/v3`), `database.go` (SQLite), `proxy.go` (ProxyRegistry: port forwarding with health checks, auto-detection, TLS probing), `session_runtime.go` (active session tracking, stream channels, cancel functions with reason tracking).
- `internal/ai/` — AI backend abstraction. `AIBackend` interface (`interface.go`) with `ExecuteStream()`. `CLIBackend` (`cli_backend.go`) is the shared base that shells out to CLI tools; each backend (claude/codebuddy/opencode/gemini/codex) provides CLI args and a `LineParser` for its JSON output format. Stream parsers are in `*__stream.go` files. `AutoResumeBackend` (`auto_resume.go`) wraps claude and codebuddy backends — detects ExitPlanMode tool_use and automatically resumes with "继续". `CodexBackend` (`codex.go`) provides full Codex CLI integration with resume support. `NewBackend()` factory in `factory.go`.
- `internal/model/` — Data models, config structs, path validation, structured error types (`errors.go`: `NotFound`, `Forbidden`, `Internal`, etc.), scheduled task model, proxy/SSH config models.
- `internal/middleware/` — Auth, request logging, panic recovery, request ID.
- `internal/speech/` — TTS abstraction (`SpeechProvider` interface). Implementations: MiniMax (cloud), Edge TTS (cloud, free), Piper (local offline), Kokoro (local ONNX-based). `summarizer.go` provides TTS summarization via multiple AI backends (mmx, claude, codebuddy, gemini, opencode, codex, ollama) for long-text compression before speech. `ollama_summarizer.go` calls Ollama HTTP API (`/api/chat`, stream:false) — the first direct HTTP client in the Go backend (all others shell out to CLI tools).
- `internal/ssh/` — SSH tunnel server (`server.go`). Supports direct-tcpip channels (-L port forwarding), password auth, ECDSA host key generation/persistence. Integrates with ProxyRegistry for port validation.
- `internal/platform/` — Platform-specific adaptations (Windows paths).

**Agent system:** YAML files in `agents/` define agents with id, backend, model, system_prompt, and optional `command` (custom CLI path). `common_prompt.md` is prepended to all agents. `{{AVAILABLE_AGENTS}}` placeholder is replaced with the agent list. Loaded at startup by `model.LoadAgents()`. Agent prompts may include `<schedule-proposal>` tag format for the scheduled task system.

**Data flow for chat:**
1. Frontend sends POST to `/api/ai/chat`
2. Handler resolves agent config → creates `AIBackend` via `ai.NewBackend()`
3. `CLIBackend.ExecuteStream()` spawns CLI process, reads stdout line-by-line
4. Backend-specific `LineParser` converts JSON lines → `StreamEvent` channel
5. Handler relays events as SSE (`content`, `thinking`, `tool_use`, `metadata`, `done`, `cancelled`, `warning`, `resume_split`, `raw_output`)
6. Messages are persisted to SQLite asynchronously

**Scheduled task system:**
1. Frontend sends POST to `/api/tasks` with cron expression, agent ID, prompt, repeat mode (once/limited/unlimited)
2. `service.Scheduler` registers cron job via `robfig/cron/v3`
3. On trigger, scheduler calls the agent's `AIBackend.ExecuteStream()` and persists results as chat messages with `scheduledTask` metadata
4. Frontend manages tasks via `/api/tasks` CRUD endpoints

**SSH tunnel / port forwarding:**
1. SSH server listens on `port+1` (or configured port)
2. Android app connects via SSH and opens direct-tcpip channels
3. `ProxyRegistry` manages forwarded ports with health checks (5s interval), auto-detection, TLS probing
4. Frontend browses forwarded ports via `PortForwardBrowser` component

**Session runtime management** (`session_runtime.go`):
- Mutex-protected active session tracking, stream channels via `sync.Map`
- Cancel functions with reason tracking: `"user"` (explicit cancel) vs `"disconnect"` (SSE client gone)
- `ForceCancelSession` kills zombie CLI processes on SSE disconnect

### Frontend (Vue 3 + TypeScript)

**Source root:** `web/src/`

**State management:** Single reactive store in `stores/app.ts` (project, files, navigation history, upload config, chat UI config, session limits, chat unread badge). No Pinia/Vuex — plain `reactive()` object.

**App structure** (`App.vue`): AppHeader (project root, theme toggle, project dialog) + bottom dock with navigation: Back, (Chat, Files, History, Port Forward [app mode only], Refresh), Forward. Drawers are mutually exclusive — opening one closes others. `ChatPanel` is a `BottomSheet` component. Authentication flow includes auto-login for Android app mode.

**Chat architecture** (the most complex UI feature):
- `ChatPanel.vue` — Orchestrator; composes composables and child components.
- `ChatMessageList.vue` — Virtual list of messages with lazy loading.
- `ChatMessageItem.vue` — Renders a single message with expandable tool calls, thinking blocks, inline actions, double-click copy, long-press context menu.
- `ChatInputBar.vue` — Input area with attach menu, auto-speech toggle, quick-send presets.
- `ChatMetadataModal.vue` — Token usage / model info modal.
- `useChatSession.ts` — Session CRUD, history loading, agent resolution, message count polling.
- `useChatStream.ts` — SSE connection, event parsing into message blocks, reconnection logic (3 attempts then fallback to polling), stream timeout handling.
- `useChatRender.ts` — Markdown rendering, block parsing (text/thinking/tool_use/schedule-proposal), content coalescing.
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
- `useAppMode.ts` — Android WebView detection, native bridge integration (addForwardedPort, openInBrowser, showServerDialog, setSSHPassword, getPassword).
- `useNotificationSound.ts` — Notification sound + haptic feedback.
- `useNotification.ts` — Push notification support.
- `useToast.ts` — Toast notification system.

**Other key components:**
- `file/FileManager.vue` + `FileViewer.vue` — Directory browser and file viewer (code/markdown/media). `FileDetailsDialog.vue` for file metadata. `FileHeader.vue` for viewer header.
- `git/` — Git history, diff view, branch graph (GitGraph, GitHistoryDrawer, GitDiffView, GitCommitList, GitCommitMeta, GitBreadcrumb).
- `session/` — SessionDrawer, SessionManager, SessionSelector for chat session management.
- `task/` — TaskDrawer, TaskManager, TaskDetailDialog for scheduled task management.
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
- `api.ts` — API helpers (apiGet, apiPost, apiDelete).
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
- **AutoResumeBackend:** Wraps claude and codebuddy backends. Detects ExitPlanMode tool_use → cancels CLI → resumes with "继续" in same session. Emits `resume_split` event for DB message finalization. Transparent to outer caller.
- **Cancel reason tracking:** Session cancels are tracked as `"user"` (explicit) or `"disconnect"` (SSE client gone). `ForceCancelSession` kills zombie CLI processes on disconnect.
- **ProxyRegistry health checks:** Forwarded ports are probed every 5s; auto-detection scans `/proc/net/tcp` (Linux), `lsof` (macOS), `netstat` (Windows); TLS probing for HTTPS ports.
- **Android native bridge:** `useAppMode()` detects Android WebView via JS bridge (`AndroidNative.*`). Supports auto-login, port forwarding registration, SSH password management, and native dialogs.
- **Touch device CSS:** Use `@media (hover: hover)` to scope `:hover` styles — touch devices get sticky hover that masks `.active` class changes.
- **Green portable deployment:** All runtime data (SQLite DB, logs, uploads, SSH host keys, TTS models) lives under `.clawbench/` next to the binary. Deleting that directory = clean uninstall.
- **Structured errors:** Backend uses `model.NotFound()`, `model.Forbidden()`, `model.Internal()` constructors for consistent HTTP error responses.

## Configuration

`config.yaml` (not committed, see `config.example.yaml`):

| Section | Key options |
|---------|------------|
| Server | `port`, `watch_dir`, `password` |
| Upload | `upload.max_size_mb`, `upload.max_files` |
| Chat UI | `chat.initial_messages`, `chat.page_size`, `chat.collapsed_height`, `chat.quick_send` |
| Session | `session.max_count` |
| TLS | `tls.enabled`, `tls.cert_file`, `tls.key_file` |
| TTS | `tts.engine` (minimax/edge/piper/kokoro), `tts.summarize_backend` (mmx/claude/codebuddy/gemini/opencode/codex/ollama), `tts.summarize_model`, `tts.speed`, `tts.voice`, engine-specific sub-configs, `tts.ollama.base_url` |
| Proxy | `proxy.enabled`, `proxy.allowed_ports` |
| SSH | `ssh.enabled`, `ssh.port`, `ssh.host_key` |
| Dev | `dev.port`, `dev.frontend_port`, `dev.host` |
| Logging | `log_dir`, `log_max_days`, `default_agent` |

Dev mode uses separate port (20002) and database (`ClawBench-dev.db`).

## Testing

- Go tests use `testify/assert`. Test files colocated with source (`*_test.go`). 40 test files across 8 packages.
- Frontend tests use Vitest + `@vue/test-utils`. Located in `web/src/components/__tests__/`.
- Many handler tests need a running test server — see `testutil_test.go` in handler package.
- Key test packages: `ai/` (stream parsers, auto-resume, factory), `handler/` (auth, chat, files, git, proxy, scheduler, SSH info, TTS), `service/` (chat, proxy, scheduler, stream, uuid), `speech/` (minimax, piper, kokoro, ollama), `ssh/` (server).

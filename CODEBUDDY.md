# CODEBUDDY.md

This file provides guidance to CodeBuddy Code when working with code in this repository.

## Project Overview

ClawBench is a mobile-first AI workstation that wraps AI CLI tools (CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex) into a web-accessible platform. Go backend shells out to CLI tools and streams JSON output via SSE; Vue 3 frontend renders the streamed events in real time.

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

**Entry point:** `cmd/server/main.go` — loads config, initializes SQLite, starts HTTP server.

**Layered structure:**
- `internal/handler/` — HTTP handlers (routes registered in `handler.go`). SSE streaming in `chat_stream.go`.
- `internal/service/` — Business logic: `chat.go` (history/persistence), `scheduler.go` (cron tasks), `database.go` (SQLite).
- `internal/ai/` — AI backend abstraction. `AIBackend` interface (`interface.go`) with `ExecuteStream()`. `CLIBackend` (`cli_backend.go`) is the shared base that shells out to CLI tools; each backend (claude/codebuddy/opencode/gemini/codex) provides CLI args and a `LineParser` for its JSON output format. `AutoResumeBackend` wraps backends that support session resumption.
- `internal/model/` — Data models, config structs, path validation.
- `internal/middleware/` — Auth, request logging, panic recovery, request ID.
- `internal/speech/` — TTS abstraction (`SpeechProvider` interface). Implementations: MiniMax, Edge TTS.
- `internal/platform/` — Platform-specific adaptations (Windows paths).

**Agent system:** YAML files in `agents/` define agents with id, backend, model, system_prompt. `common_prompt.md` is prepended to all agents. `{{AVAILABLE_AGENTS}}` placeholder is replaced with the agent list. Loaded at startup by `model.LoadAgents()`.

**Data flow for chat:**
1. Frontend sends POST to `/api/ai/chat`
2. Handler resolves agent config → creates `AIBackend` via `ai.NewBackend()`
3. `CLIBackend.ExecuteStream()` spawns CLI process, reads stdout line-by-line
4. Backend-specific `LineParser` converts JSON lines → `StreamEvent` channel
5. Handler relays events as SSE (`content`, `thinking`, `tool_use`, `metadata`, `done`, `cancelled`)
6. Messages are persisted to SQLite asynchronously

### Frontend (Vue 3 + TypeScript)

**Source root:** `web/src/`

**State management:** Single reactive store in `stores/app.ts` (project, files, navigation history). No Pinia/Vuex — plain `reactive()` object.

**App structure** (`App.vue`): Bottom dock with 4 buttons (Chat, Files, History, Refresh). Drawers are mutually exclusive — opening one closes others. `ChatPanel` is a `BottomSheet` component.

**Chat architecture** (the most complex UI feature):
- `ChatPanel.vue` — Orchestrator; composes composables and child components.
- `useChatSession.ts` — Session CRUD, history loading, agent resolution, message count polling.
- `useChatStream.ts` — SSE connection, event parsing into message blocks, reconnection logic (3 attempts then fallback to polling), stream timeout handling.
- `useChatRender.ts` — Markdown rendering, block parsing (text/thinking/tool_use/schedule-proposal), content coalescing.
- `useAutoSpeech.ts` — Auto-read toggle (module-level singleton ref), TTS playback via backend `/api/tts/generate`.
- `ChatMessageItem.vue` — Renders a single message with expandable tool calls, thinking blocks, inline actions.
- `ChatInputBar.vue` — Input area with attach menu, auto-speech toggle.

**Other key components:**
- `file/FileManager.vue` + `FileViewer.vue` — Directory browser and file viewer (code/markdown/media).
- `git/` — Git history, diff view, branch graph.
- `common/BottomSheet.vue` — Mobile-friendly bottom sheet drawer.
- `media/Lightbox.vue` — Image zoom/pan viewer (singleton, teleported to body).

**Vite config** (`vite.config.ts`): Custom plugin `hljsThemeWrapper` wraps highlight.js CSS with `[data-hljs-theme]` attribute selectors so light/dark themes coexist. Root is `web/`, build output goes to `public/`. Dev proxy forwards `/api` to Go backend.

**Path alias:** `@` → `web/src/`

## Key Patterns

- **Module-level singletons:** `useAutoSpeech()` uses module-level refs so all consumers share the same state. Only instantiate once (in ChatPanel).
- **SSE with reconnection:** `useChatStream` handles SSE disconnects with up to 3 reconnects, then falls back to HTTP polling every 2s. 60s timeout with no events triggers reconnect.
- **Block coalescing:** Streamed text/thinking events are merged into the last block of the same type (unless separated by a `tool_use` block which acts as a boundary).
- **Drawer mutual exclusion:** `App.vue` manages all drawer open states; opening one instantly closes others.
- **Touch device CSS:** Use `@media (hover: hover)` to scope `:hover` styles — touch devices get sticky hover that masks `.active` class changes.
- **Green portable deployment:** All runtime data (SQLite DB, logs, uploads) lives under `.clawbench/` next to the binary. Deleting that directory = clean uninstall.

## Configuration

`config.yaml` (not committed, see `config.example.yaml`): port, watch_dir, password, default_agent, upload limits, TTS engine (minimax/edge), TLS, log retention. Dev mode uses separate port (20002) and database (`ClawBench-dev.db`).

## Testing

- Go tests use `testify/assert`. Test files colocated with source (`*_test.go`).
- Frontend tests use Vitest + `@vue/test-utils`. Located in `web/src/components/__tests__/`.
- Many handler tests need a running test server — see `testutil_test.go` in handler package.

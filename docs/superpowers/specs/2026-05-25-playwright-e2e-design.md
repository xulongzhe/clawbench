# Playwright E2E Testing + Coverage + CI Gate Design

## 1. Background & Goals

ClawBench has comprehensive unit tests (90 frontend Vitest, 92 Go, 15 Android) with a mature two-tier coverage gate (Tier 1: project baseline, Tier 2: diff 80%). However, there are **zero E2E tests** — no Playwright, Cypress, or any browser-level integration tests exist.

**Goals:**
1. Establish Playwright E2E test infrastructure for ClawBench
2. Cover 4 core user flows: Chat, File Manager, Navigation, Git
3. Collect V8 code coverage from E2E tests (Chromium only)
4. Phase 1: Report E2E coverage alongside unit coverage, no CI gate
5. Run on all 3 browser engines (Chromium + Firefox + WebKit)

## 2. Architecture Decisions

### 2.1 Real Backend + Mock AI CLI

**Decision:** Start the real Go backend, but inject a `MockBackend` that replaces AI CLI subprocess calls.

**Rationale:**
- SSE streaming, chat persistence, session management, and WebSocket events all go through the real backend — these cannot be meaningfully tested with mocked HTTP routes
- Real AI CLIs are non-deterministic, slow, and require API keys — unsuitable for CI
- The `AIBackend` interface is minimal (`Name()` + `ExecuteStream()`) and a `MockBackend` already exists in `auto_resume_test.go` as a proven pattern

**Implementation:**
- `MockAIBackend` added to `internal/ai/mock_backend.go`, registered in `factory.go` via `"mock"` case
- `NoCLI: true` flag on `BackendSpec` so the mock backend doesn't require a CLI binary
- Mock agent configured via `config/agents/mock.yaml` with `backend: mock`
- `MockAIBackend` streams "Hello! I am a mock assistant. How can I help you today?" word by word with 50ms delays, using `select` on `time.After` + `ctx.Done()` for instant cancellation
- Controlled via mock agent YAML + `default_agent: mock` in test config (no env var needed)

### 2.2 Auth Strategy

**Decision:** Rely on localhost auth bypass. Cannot test real login flow in E2E.

**Rationale:**
- Go server's auth middleware automatically bypasses authentication for requests from localhost
- E2E tests always run from localhost (both CI and local), so the login page is never shown
- Adding `auth.disable_localhost_bypass` would require backend changes outside E2E scope

**Implementation:**
- Auth tests verify that the app works correctly in the authenticated state (page loads, chat accessible, session persists across reload)
- Prominent comment in `auth.spec.ts` documents this limitation

### 2.3 Coverage Collection

**Decision:** Chromium-only V8 coverage via `page.coverage` API, converted to Istanbul format using `v8-to-istanbul`, merged with Vitest unit coverage via `nyc merge`.

**Rationale:**
- Playwright's `page.coverage` API only works on Chromium — this is a hard limitation
- Firefox and WebKit tests run without coverage collection (functionality tests only)
- Istanbul format is compatible with existing Vitest coverage toolchain
- Merged coverage provides a more accurate picture than unit tests alone

**Implementation:**
- Custom Playwright fixture `coverageFixture` (auto: true) in `e2e/fixtures/coverage.fixture.ts`
- Fixture checks `projectName === 'chromium-coverage'` (not `browserName`) to determine whether to collect coverage
- Per-test Istanbul JSON written to `.nyc_output/`, then `nyc report` generates combined report
- Coverage is NOT merged with Vitest output in Phase 1 (reported separately)

### 2.4 Phase Plan

| Phase | Scope | Gate |
|-------|-------|------|
| **Phase 1** | E2E coverage collected + reported; separate from unit gate | No gate — report only |
| **Phase 2** (future) | E2E + Unit coverage merged; unified Tier 1/2 gate | Merged into existing gate |

## 3. Directory Structure

```
e2e/
├── playwright.config.ts           # Playwright configuration
├── fixtures/                      # Custom Playwright fixtures
│   ├── index.ts                   # Merge & re-export all fixtures
│   ├── auth.fixture.ts            # Login + session cookie storageState
│   ├── mock-api.fixture.ts        # (unused in real-backend mode, kept for future)
│   └── coverage.fixture.ts        # V8 → Istanbul coverage collection (Chromium only)
├── pages/                         # Page Object Models
│   ├── chat.page.ts               # Chat panel POM
│   ├── file-manager.page.ts       # File manager POM
│   ├── git.page.ts                # Git history/manage POM
│   ├── task.page.ts               # Task tab POM
│   ├── terminal.page.ts           # Terminal panel POM
│   └── navigation.page.ts         # Tab/drawer navigation POM
├── helpers/                       # Test utilities
│   ├── server.ts                  # Start/stop Go backend; wait for ready
│   └── test-data.ts               # Shared test data constants
└── specs/                         # Test specifications
    ├── auth.spec.ts                # Authenticated state verification
    ├── chat.spec.ts                # Chat: send, SSE stream, model switch
    ├── file-manager.spec.ts        # File browsing, viewing, attachments
    ├── navigation.spec.ts          # Tab switching, drawer, project switch
    └── git.spec.ts                # History, branches, worktrees
```

## 4. Selector Strategy

The project currently has **no `data-testid` or `data-tab` attributes**. E2E tests must use existing CSS classes and ARIA roles.

**Key selector mappings (from source analysis):**

| Element | Selector | Source |
|---------|----------|--------|
| Chat textarea | `.chat-textarea` | ChatInputBar.vue |
| Send button | `.chat-send-btn` | ChatInputBar.vue |
| Stop button | `.chat-stop-btn` | ChatInputBar.vue |
| Model chip | `.model-chip` | ChatInputBar.vue |
| Login form | `.login-page` | login.html |
| File item | `.file-item` | FileManagerContent.vue |
| File viewer | `.file-viewer` | FileViewer.vue |

**Playwright locator priority (per best practices):**
1. `getByRole()` — e.g., `getByRole('button', { name: 'Send' })`
2. `getByText()` — e.g., `getByText('Quick send')`
3. CSS class — e.g., `.chat-send-btn` (last resort, but necessary for many ClawBench elements)

**Future improvement:** Add `data-testid` attributes to key interactive elements for more resilient E2E selectors. Priority targets: dock tab buttons (currently index-based), login form elements.

## 5. Test Data Construction

### 5.1 Data Source Map

Each frontend feature draws data from different sources:

| Feature | Data Source | API Endpoint | Test Setup |
|---------|-----------|-------------|-----------|
| **Chat sessions** | SQLite | `GET /api/chat/sessions` | Auto-created on first message |
| **Chat messages** | SQLite | `GET /api/chat/{id}/messages` | Auto-created by chat flow |
| **Quick-send items** | SQLite | `GET /api/chat/quick-send` | Seed via `POST /api/chat/quick-send` |
| **File listing** | Filesystem | `GET /api/dir` | Uses project directory (has files) |
| **File content** | Filesystem | `GET /api/file/{path}` | Uses project directory (has files) |
| **Git history** | `git` CLI | `GET /api/git/history` | Project is a git repo (has commits) |
| **Git branches** | `git` CLI | `GET /api/git/branches` | Project has branches |
| **Settings** | `config.yaml` | `GET /api/config` | Test `config.yaml` in temp dir |
| **Scheduled tasks** | SQLite | `GET /api/tasks` | Seed via `POST /api/tasks` |
| **Terminal** | PTY/WS | `WS /api/terminal/ws` | Terminal enabled in config |
| **Port forwarding** | SQLite + SSH | `GET /api/ssh/info` | Disabled in test config |

**Key insight:** Only `chat_quick_send` and `scheduled_tasks` need explicit seeding via API. Everything else comes from the filesystem, git CLI, or is auto-created.

### 5.2 Database Strategy

**Decision:** Fresh per-test-run database via temp directory isolation.

**Rationale:**
- Go server creates `ClawBench.db` at `<BinDir>/.clawench/ClawBench.db` on startup
- Setting `BinDir` to a temp directory gives each test run an empty database
- All tables are auto-created by `InitDB()` — no manual schema setup needed

### 5.3 Filesystem Test Data

**Decision:** Use the ClawBench source repo itself (Option A).

**Rationale:**
- The source tree always has directories, code files, and a `.git` directory
- `watch_dir` in config points to the project directory
- No extra test fixture maintenance needed
- For git tests needing specific history shape, create small fixture repo under `e2e/fixtures/`

## 6. Key Design: Server Lifecycle

The Go backend is managed externally (not via Playwright's `webServer` config):

1. `globalSetup` starts the server: spawns `./clawbench --port 20100` with temp config
2. Server creates fresh DB in temp dir, loads mock agent YAML
3. `waitForServer()` polls `GET /api/me` until 200 or 401
4. Tests run against `http://localhost:20100`
5. `globalTeardown` kills server process, cleans temp dir

Test config disables non-essential features (RAG, port forwarding) and sets `log_level: warn` for speed.

## 7. `data-testid` Strategy

**Adoption approach: Incremental, not big-bang.**

Phase 1 (initial E2E setup): Use existing CSS classes — they are already semantic (`.chat-send-btn`, `.chat-textarea`). Add `data-testid` only where CSS selectors are fragile (dock tab buttons by index).

Phase 2 (after initial tests stabilize): Add `data-testid` to elements that caused test fragility during Phase 1.

**Priority `data-testid` additions** (fragile selectors that need fixing first):

| Element | Current Selector | Problem | Proposed `data-testid` |
|---------|----------------|---------|----------------------|
| Dock: Chat tab | `.dock-btn:nth(0)` | Index-based, fragile | `tab-chat` |
| Dock: Files tab | `.dock-btn:nth(2)` | Index-based, fragile | `tab-browse` |
| Dock: Viewer tab | `.dock-btn:nth(1)` | Index-based, fragile | `tab-viewer` |
| Dock: Tasks tab | `.dock-btn:nth(3)` | Index-based, fragile | `tab-tasks` |

## 8. Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| V8 coverage path mismatch with source maps | Use `v8-to-istanbul` with source map support; validate in Phase 1 before merging |
| E2E tests flaky due to timing | Playwright auto-wait assertions; CI retries = 2; no `sleep` in tests |
| MockBackend diverges from real behavior | Keep mock responses simple (content → metadata → done); test real SSE format |
| Coverage collection slows Chromium tests | `reportAnonymousScripts: false` by default; only convert `/src/` files |
| Cross-browser inconsistencies | Chromium = reference; Firefox/WebKit failures are real bugs to fix |
| Go server startup time in CI | Build once, reuse across test shards; `waitForServer` poll pattern |
| localhost auth bypass limits auth testing | Document limitation; auth tests verify authenticated state only |

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
- Add `"mock"` case to `internal/ai/factory.go` returning a `MockAIBackend`
- `MockAIBackend` returns configurable canned SSE events (content → metadata → done)
- Controlled via env var `CLAWBENCH_MOCK_BACKEND=1` or a test-specific `config/agents/mock.yaml`
- The mock agent YAML:
  ```yaml
  id: mock
  name: Mock Agent
  icon: 🧪
  specialty: E2E Testing
  backend: mock
  system_prompt: "You are a mock assistant for E2E testing."
  ```

### 2.2 Auth Strategy

**Decision:** Use a known test password set via `config.yaml`.

**Rationale:**
- Localhost bypass exists but only works when Playwright browser and Go server are on the same machine (CI: always true; local: always true when running against `localhost`)
- However, for robustness and to test the actual login flow, we set a known password
- Login fixture performs `POST /login` and stores the session cookie

**Implementation:**
- Test `config.yaml` sets `password: "e2e-test-password"`
- Auth fixture reads this password and performs login
- Session cookie (`clawbench_session`) is stored in Playwright `storageState`

### 2.3 Coverage Collection

**Decision:** Chromium-only V8 coverage via `page.coverage` API, converted to Istanbul format using `v8-to-istanbul`, merged with Vitest unit coverage via `nyc merge`.

**Rationale:**
- Playwright's `page.coverage` API only works on Chromium — this is a hard limitation
- Firefox and WebKit tests run without coverage collection (functionality tests only)
- Istanbul format is compatible with existing Vitest coverage toolchain
- Merged coverage provides a more accurate picture than unit tests alone

**Implementation:**
- Custom Playwright fixture `coverageFixture` (auto: true) wraps each test:
  1. `page.coverage.startJSCoverage()` before navigation
  2. Test runs
  3. `page.coverage.stopJSCoverage()` after test
  4. Convert V8 → Istanbul via `v8-to-istanbul`
  5. Write per-test Istanbul JSON to `.nyc_output/` directory
- After all tests complete, `nyc report` generates the combined report
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
│   ├── login.page.ts              # Login page POM
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
    ├── auth.spec.ts                # Login/logout flow
    ├── chat.spec.ts                # Chat: send, SSE stream, quick send, model switch
    ├── file-manager.spec.ts        # File browsing, viewing, attachments
    ├── navigation.spec.ts          # Tab switching, drawer, project switch
    └── git.spec.ts                # History, branches, worktrees
```

## 4. Selector Strategy

The project currently has **no `data-testid` or `data-tab` attributes**. E2E tests must use existing CSS classes and ARIA roles.

**Key selector mappings (from source analysis):**

| Element | Selector | Source |
|---------|----------|--------|
| Tab: Chat | `.dock-btn.active` (first in dock) | App.vue dock buttons |
| Tab: Files | `.dock-btn` (3rd dock button, `FolderOpen` icon) | App.vue |
| Tab: Viewer | `.dock-btn` (2nd dock button, `FileText` icon) | App.vue |
| Tab: Tasks | `.dock-btn` (4th dock button) | App.vue |
| Tab: History | `.dock-overflow-item` (in overflow popup) | App.vue |
| Tab: Settings | `.dock-overflow-item` (settings in overflow) | App.vue |
| Chat textarea | `.chat-textarea` | ChatInputBar.vue:93 |
| Send button | `.chat-send-btn` | ChatInputBar.vue:103 |
| Stop button | `.chat-stop-btn` | ChatInputBar.vue:109 |
| Quick-send hint | `.quick-send-hint` | ChatInputBar.vue:84 |
| Quick-send menu | `.quick-send-title` | ChatInputBar.vue:149 |
| Model chip | `.model-chip` | ChatInputBar.vue:32 |
| Login form | `.login-page` | login.html |
| File item | `.file-item` | FileManagerContent.vue |
| File viewer | `.file-viewer` | FileViewer.vue |

**Playwright locator priority (per best practices):**
1. `getByRole()` — e.g., `getByRole('button', { name: 'Send' })`
2. `getByText()` — e.g., `getByText('Quick send')`
3. CSS class — e.g., `.chat-send-btn` (last resort, but necessary for many ClawBench elements)

**Future improvement:** Add `data-testid` attributes to key interactive elements for more resilient E2E selectors. This can be done incrementally as E2E tests are written.

## 5. MockAIBackend (Go)

Add to `internal/ai/mock_backend.go`:

```go
package ai

import (
    "context"
    "strings"
    "sync"
    "time"
)

// MockAIBackend implements AIBackend for E2E testing.
// Returns configurable canned stream events with a small delay to simulate streaming.
type MockAIBackend struct {
    mu        sync.Mutex
    callCount int
}

func (m *MockAIBackend) Name() string { return "mock" }

func (m *MockAIBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
    m.mu.Lock()
    m.callCount++
    m.mu.Unlock()

    ch := make(chan StreamEvent, 32)

    go func() {
        defer close(ch)

        // Simulate streaming: send content in chunks with delays
        response := "Hello! I am a mock assistant. How can I help you today?"
        words := strings.Fields(response)

        for i, word := range words {
            select {
            case <-ctx.Done():
                ch <- StreamEvent{Type: "cancelled", Reason: ReasonContextCancel}
                return
            default:
            }

            sep := " "
            if i == 0 {
                sep = ""
            }
            ch <- StreamEvent{Type: "content", Content: sep + word}
            time.Sleep(50 * time.Millisecond) // simulate streaming pace
        }

        // Send metadata
        ch <- StreamEvent{
            Type: "metadata",
            Meta: &Metadata{
                Model:        "mock-model",
                InputTokens:  10,
                OutputTokens: len(response) / 4,
                DurationMs:   500,
                StopReason:   "end_turn",
            },
        }

        ch <- StreamEvent{Type: "done"}
    }()

    return ch, nil
}
```

Register in `factory.go`:

```go
case "mock":
    return &MockAIBackend{}, nil
```

## 6. Playwright Configuration

```typescript
// e2e/playwright.config.ts
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './specs',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI
    ? [['html', { open: 'never' }], ['github']]
    : [['html', { open: 'on-failure' }], ['list']],
  timeout: 30000,
  expect: { timeout: 10000 },

  use: {
    baseURL: `http://localhost:${process.env.E2E_PORT || 20100}`,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },

  projects: [
    // Coverage project: Chromium only, with V8 coverage collection
    {
      name: 'chromium-coverage',
      use: {
        ...devices['Desktop Chrome'],
        // coverage.fixture.ts checks project name to enable collection
      },
    },
    // Cross-browser: no coverage, functionality only
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
  ],

  // Server is managed externally (Go backend started by helper)
  // No `webServer` config — we start it in globalSetup
})
```

## 7. Custom Fixtures

### 6.1 Auth Fixture

```typescript
// e2e/fixtures/auth.fixture.ts
import { test as base, expect } from '@playwright/test'

const E2E_PASSWORD = process.env.E2E_PASSWORD || 'e2e-test-password'

export const test = base.extend({
  // Auto-login: navigate to login page and authenticate
  page: async ({ page }, use) => {
    // If not already logged in (no session cookie), perform login
    const response = await page.goto('/')
    if (response?.status() === 401 || page.url().includes('/login')) {
      await page.fill('input[type="password"]', E2E_PASSWORD)
      await page.click('button[type="submit"]')
      await page.waitForURL('**/chat')  // or whatever the post-login URL is
    }
    await use(page)
  },
})
```

### 6.2 Coverage Fixture

```typescript
// e2e/fixtures/coverage.fixture.ts
import { test as base } from '@playwright/test'
import v8toIstanbul from 'v8-to-istanbul'
import { writeFileSync, mkdirSync } from 'fs'
import { join } from 'path'

const COVERAGE_DIR = '.nyc_output'

export const test = base.extend({
  coverageCollector: [async ({ page, browserName, project }, use, testInfo) => {
    // Only collect coverage on Chromium
    if (browserName !== 'chromium') {
      await use(null)
      return
    }

    // Start JS coverage before navigation
    const coverage = await page.coverage.startJSCoverage({
      reportAnonymousScripts: true,
    })

    await use(null)

    // Stop coverage and convert to Istanbul format
    const jsCoverage = await page.coverage.stopJSCoverage()

    for (const entry of jsCoverage) {
      // Only convert app code (skip node_modules, vendors)
      if (!entry.url.includes('/src/') && !entry.url.includes('/assets/')) continue

      const converter = v8toIstanbul('', 0, { source: entry.source })
      await converter.load()
      converter.applyCoverage(entry.functions)

      const istanbulData = converter.toIstanbul()
      const filename = `e2e-${testInfo.testId}-${entry.url.replace(/[^a-z0-9]/gi, '_')}.json`
      mkdirSync(COVERAGE_DIR, { recursive: true })
      writeFileSync(join(COVERAGE_DIR, filename), JSON.stringify(istanbulData))
    }
  }, { auto: true, scope: 'test' }],
})
```

### 6.3 Merged Fixtures

```typescript
// e2e/fixtures/index.ts
import { mergeTests } from '@playwright/test'
import { test as authTest } from './auth.fixture'
import { test as coverageTest } from './coverage.fixture'

export const test = mergeTests(authTest, coverageTest)
export { expect } from '@playwright/test'
```

## 8. Page Object Models

### 7.1 Chat Page

```typescript
// e2e/pages/chat.page.ts
import { type Locator, type Page, expect } from '@playwright/test'

export class ChatPage {
  readonly page: Page
  readonly textarea: Locator
  readonly sendButton: Locator
  readonly stopButton: Locator
  readonly quickSendHint: Locator
  readonly messageList: Locator
  readonly sessionTab: Locator

  constructor(page: Page) {
    this.page = page
    this.textarea = page.locator('.chat-textarea')
    this.sendButton = page.locator('.chat-send-btn')
    this.stopButton = page.locator('.chat-stop-btn')
    this.quickSendHint = page.locator('.quick-send-hint')
    this.messageList = page.locator('.chat-messages')
    this.sessionTab = page.locator('.chat-action-btn').first()
  }

  async sendMessage(text: string) {
    await this.textarea.fill(text)
    await this.sendButton.click()
  }

  async openQuickSendMenu() {
    // Click send with empty input to open quick-send popup
    await this.sendButton.click()
  }

  async waitForReply() {
    // Wait for AI response to appear (mock backend responds quickly)
    await expect(this.page.locator('.message-assistant')).toBeVisible({ timeout: 10000 })
  }

  async createSession() {
    await this.page.locator('.chat-action-btn', { hasText: /New|\+/ }).click()
  }
}
```

### 7.2 File Manager Page

```typescript
// e2e/pages/file-manager.page.ts
import { type Locator, type Page, expect } from '@playwright/test'

export class FileManagerPage {
  readonly page: Page
  readonly fileList: Locator
  readonly breadcrumb: Locator

  constructor(page: Page) {
    this.page = page
    this.fileList = page.locator('.file-list')
    this.breadcrumb = page.locator('.file-breadcrumb')
  }

  async navigateToTab() {
    await this.page.locator('[data-tab="files"]').click()
  }

  async openFile(name: string) {
    await this.page.locator('.file-item', { hasText: name }).dblclick()
  }

  async navigateToDirectory(name: string) {
    await this.page.locator('.file-item', { hasText: name }).click()
  }

  async expectFileVisible(name: string) {
    await expect(this.page.locator('.file-item', { hasText: name })).toBeVisible()
  }
}
```

### 7.3 Navigation Page

```typescript
// e2e/pages/navigation.page.ts
import { type Page, expect } from '@playwright/test'

// Dock tab indices: 0=chat, 1=viewer, 2=browse, 3=tasks
const TAB_CHAT = 0
const TAB_VIEWER = 1
const TAB_BROWSE = 2
const TAB_TASKS = 3

export class NavigationPage {
  readonly page: Page
  readonly dockButtons: Locator

  constructor(page: Page) {
    this.page = page
    this.dockButtons = page.locator('.dock-btn')
  }

  async switchToChat() {
    await this.dockButtons.nth(TAB_CHAT).click()
  }

  async switchToFileManager() {
    await this.dockButtons.nth(TAB_BROWSE).click()
  }

  async switchToTasks() {
    await this.dockButtons.nth(TAB_TASKS).click()
  }

  async switchToViewer() {
    await this.dockButtons.nth(TAB_VIEWER).click()
  }

  // History and other overflow tabs require opening the overflow menu first
  async openOverflowMenu() {
    await this.page.locator('.dock-overflow-btn').click()
  }

  async switchToHistory() {
    await this.openOverflowMenu()
    await this.page.locator('.dock-overflow-item', { hasText: /History|历史/ }).click()
  }

  async expectActiveTab(tabIndex: number) {
    await expect(this.dockButtons.nth(tabIndex)).toHaveClass(/active/)
  }
}
```

## 9. Test Specifications

### 8.1 Auth Spec

```typescript
// e2e/specs/auth.spec.ts
import { test, expect } from '../fixtures'

test.describe('Authentication', () => {
  test('should show login page for unauthenticated users', async ({ page }) => {
    // Clear cookies to force unauthenticated state
    await page.context().clearCookies()
    await page.goto('/')
    await expect(page.locator('.login-page')).toBeVisible()
  })

  test('should login with correct password', async ({ page }) => {
    await page.context().clearCookies()
    await page.goto('/')
    await page.fill('input[type="password"]', 'e2e-test-password')
    await page.click('button[type="submit"]')
    await expect(page.locator('.chat-panel')).toBeVisible()
  })

  test('should reject incorrect password', async ({ page }) => {
    await page.context().clearCookies()
    await page.goto('/')
    await page.fill('input[type="password"]', 'wrong-password')
    await page.click('button[type="submit"]')
    await expect(page.locator('.login-error')).toBeVisible()
  })

  test('should persist session across page reload', async ({ page }) => {
    await page.reload()
    await expect(page.locator('.chat-panel')).toBeVisible()
  })
})
```

### 8.2 Chat Spec

```typescript
// e2e/specs/chat.spec.ts
import { test, expect } from '../fixtures'
import { ChatPage } from '../pages/chat.page'

test.describe('Chat', () => {
  let chat: ChatPage

  test.beforeEach(async ({ page }) => {
    chat = new ChatPage(page)
  })

  test('should send a message and receive SSE stream reply', async ({ page }) => {
    await chat.sendMessage('Hello, mock assistant!')
    // Wait for the message to appear in the chat
    await expect(page.locator('.message-user')).toContainText('Hello, mock assistant!')
    // Wait for the mock AI response (SSE stream)
    await chat.waitForReply()
  })

  test('should show quick-send hint when input is empty', async ({ page }) => {
    // The hint should be visible when input is empty and quick-send items exist
    await expect(chat.quickSendHint).toBeVisible()
  })

  test('should hide quick-send hint when typing', async ({ page }) => {
    await expect(chat.quickSendHint).toBeVisible()
    await chat.textarea.fill('typing something')
    await expect(chat.quickSendHint).not.toBeVisible()
  })

  test('should open quick-send menu on empty send click', async ({ page }) => {
    await chat.openQuickSendMenu()
    await expect(page.locator('.quick-send-title')).toBeVisible()
  })

  test('should create a new session', async ({ page }) => {
    await chat.createSession()
    // Verify new session is created (empty chat area)
    await expect(chat.textarea).toBeVisible()
  })

  test('should switch model from model chip', async ({ page }) => {
    const modelChip = page.locator('.model-chip')
    if (await modelChip.isVisible()) {
      await modelChip.click()
      await expect(page.locator('.model-menu-item').first()).toBeVisible()
    }
  })
})
```

### 8.3 File Manager Spec

```typescript
// e2e/specs/file-manager.spec.ts
import { test, expect } from '../fixtures'
import { FileManagerPage } from '../pages/file-manager.page'

test.describe('File Manager', () => {
  let fm: FileManagerPage

  test.beforeEach(async ({ page }) => {
    fm = new FileManagerPage(page)
    await fm.navigateToTab()
  })

  test('should display files in the project directory', async ({ page }) => {
    // Project directory should contain at least some files
    await expect(page.locator('.file-item').first()).toBeVisible()
  })

  test('should navigate into a directory on click', async ({ page }) => {
    const dirItem = page.locator('.file-item[data-type="directory"]').first()
    if (await dirItem.isVisible()) {
      const dirName = await dirItem.textContent()
      await dirItem.click()
      await expect(fm.breadcrumb).toContainText(dirName!)
    }
  })

  test('should open file viewer on double-click', async ({ page }) => {
    const fileItem = page.locator('.file-item[data-type="file"]').first()
    if (await fileItem.isVisible()) {
      await fileItem.dblclick()
      await expect(page.locator('.file-viewer')).toBeVisible()
    }
  })
})
```

### 8.4 Navigation Spec

```typescript
// e2e/specs/navigation.spec.ts
import { test, expect } from '../fixtures'
import { NavigationPage } from '../pages/navigation.page'

test.describe('Navigation', () => {
  let nav: NavigationPage

  test.beforeEach(async ({ page }) => {
    nav = new NavigationPage(page)
  })

  test('should switch from Chat to Files tab', async ({ page }) => {
    await nav.switchToFileManager()
    await nav.expectActiveTab(2) // TAB_BROWSE
  })

  test('should switch from Files back to Chat', async ({ page }) => {
    await nav.switchToFileManager()
    await nav.switchToChat()
    await nav.expectActiveTab(0) // TAB_CHAT
  })

  test('should maintain state when switching tabs', async ({ page }) => {
    // Type something in chat
    const chatInput = page.locator('.chat-textarea')
    await chatInput.fill('test draft')

    // Switch to files and back
    await nav.switchToFileManager()
    await nav.switchToChat()

    // Draft should be preserved
    await expect(chatInput).toHaveValue('test draft')
  })

  test('should open History via overflow menu', async ({ page }) => {
    await nav.switchToHistory()
    // History tab content should be visible
    await expect(page.locator('.git-history-content, .commit-list')).toBeVisible()
  })
})
```

### 8.5 Git Spec

```typescript
// e2e/specs/git.spec.ts
import { test, expect } from '../fixtures'

test.describe('Git', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to History tab
    await page.locator('[data-tab="history"]').click()
  })

  test('should display git history', async ({ page }) => {
    // If it's a git repo, commit list should be visible
    const commitList = page.locator('.commit-list')
    // May or may not have commits depending on test project state
    await expect(commitList).toBeVisible()
  })

  test('should switch between history sub-tabs', async ({ page }) => {
    // File History / Project History / Manage
    const manageTab = page.locator('[data-subtab="manage"]')
    if (await manageTab.isVisible()) {
      await manageTab.click()
      await expect(page.locator('.git-manage')).toBeVisible()
    }
  })

  test('should show branch list in manage tab', async ({ page }) => {
    const manageTab = page.locator('[data-subtab="manage"]')
    if (await manageTab.isVisible()) {
      await manageTab.click()
      await page.locator('[data-manage-tab="branches"]').click()
      await expect(page.locator('.branch-list')).toBeVisible()
    }
  })
})
```

## 10. Server Management

```typescript
// e2e/helpers/server.ts
import { spawn, ChildProcess } from 'child_process'
import { createWriteStream, readFileSync } from 'fs'

const E2E_PORT = parseInt(process.env.E2E_PORT || '20100')
const E2E_PASSWORD = process.env.E2E_PASSWORD || 'e2e-test-password'

let serverProcess: ChildProcess | null = null

export async function startServer(): Promise<void> {
  const projectRoot = process.cwd()

  // Write test config with known password and mock agent
  const testConfig = `
server:
  port: ${E2E_PORT}
  password: "${E2E_PASSWORD}"
  log_level: warn
  watch_dir: ${projectRoot}
terminal:
  enabled: true
  idle_timeout: 1h
port_forward:
  enabled: false
rag:
  enabled: false
`

  // Build the Go binary first
  // ... (or use pre-built binary)

  serverProcess = spawn('./clawbench', [`--port`, String(E2E_PORT)], {
    env: {
      ...process.env,
      CLAWBENCH_MOCK_BACKEND: '1',
    },
    stdio: ['pipe', 'pipe', 'pipe'],
  })

  // Wait for server to be ready
  await waitForServer(E2E_PORT, 30000)
}

export async function stopServer(): Promise<void> {
  if (serverProcess) {
    serverProcess.kill('SIGTERM')
    await new Promise<void>(resolve => {
      serverProcess!.on('exit', () => resolve())
      setTimeout(() => {
        serverProcess!.kill('SIGKILL')
        resolve()
      }, 5000)
    })
    serverProcess = null
  }
}

async function waitForServer(port: number, timeoutMs: number): Promise<void> {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    try {
      const response = await fetch(`http://localhost:${port}/api/me`)
      if (response.ok || response.status === 401) return
    } catch {
      // Server not ready yet
    }
    await new Promise(r => setTimeout(r, 500))
  }
  throw new Error(`Server did not start within ${timeoutMs}ms`)
}
```

## 11. Coverage Reporting Script

```bash
# scripts/check-e2e-coverage.sh
#!/usr/bin/env bash
# Phase 1: Collect and report E2E coverage (no gate)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "==> Generating E2E coverage report..."

# NYC merges all .nyc_output/*.json files
npx nyc report --reporter=text --reporter=json-summary --report-dir=coverage/e2e

COVERAGE_JSON="$ROOT_DIR/coverage/e2e/coverage-summary.json"

if [ -f "$COVERAGE_JSON" ]; then
  echo ""
  echo "E2E Coverage Summary:"
  python3 - "$COVERAGE_JSON" << 'PYTHON'
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
total = data.get("total", {})
for metric in ["lines", "statements", "functions", "branches"]:
    m = total.get(metric, {})
    pct = (m.get("covered", 0) / m["total"] * 100) if m.get("total", 0) > 0 else 0
    print(f"  {metric:>12}: {m.get('covered', 0):>6}/{m.get('total', 0):<6} ({pct:.1f}%)")
PYTHON
else
  echo "WARNING: No E2E coverage data found"
fi
```

## 12. CI Integration (Phase 1)

Add to `.github/workflows/ci.yml`:

```yaml
  e2e:
    name: E2E Tests (Playwright)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - uses: actions/setup-node@v4
        with:
          node-version: '24'
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Install Playwright browsers
        run: npx playwright install --with-deps

      - name: Build Go binary
        run: go build -o clawbench ./cmd/server

      - name: Run E2E tests
        run: npx playwright test
        env:
          E2E_PORT: '20100'
          E2E_PASSWORD: 'e2e-test-password'
          CLAWBENCH_MOCK_BACKEND: '1'

      - name: Generate E2E coverage report
        if: always()
        run: ./scripts/check-e2e-coverage.sh

      - name: Upload E2E coverage
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: e2e-coverage
          path: coverage/e2e/
          retention-days: 90

      - name: Upload Playwright report
        if: ${{ !cancelled() }}
        uses: actions/upload-artifact@v4
        with:
          name: playwright-report
          path: playwright-report/
          retention-days: 30

      - name: Upload test results
        if: ${{ !cancelled() }}
        uses: actions/upload-artifact@v4
        with:
          name: playwright-results
          path: test-results/
          retention-days: 7

      - name: Upload server logs
        if: ${{ !cancelled() }}
        uses: actions/upload-artifact@v4
        with:
          name: e2e-server-logs
          path: /tmp/clawbench-e2e-*/.clawbench/logs/
          retention-days: 7
```

## 13. NPM Dependencies

Add to `package.json` `devDependencies`:

```json
{
  "@playwright/test": "^1.52.0",
  "v8-to-istanbul": "^9.2.0",
  "nyc": "^17.1.0"
}
```

Add to `package.json` `scripts`:

```json
{
  "test:e2e": "playwright test",
  "test:e2e:ui": "playwright test --ui",
  "test:e2e:debug": "playwright test --debug"
}
```

## 14. Future Phase 2 Plan

When Phase 1 is stable (E2E coverage collected for multiple CI runs):

1. **Merge E2E + Unit coverage**: Run `nyc merge` to combine Vitest and Playwright Istanbul outputs
2. **Unified coverage gate**: Modify `check-frontend-coverage.sh` to accept merged coverage
3. **Dynamic baseline**: E2E coverage will naturally increase the combined baseline
4. **E2E-specific Tier 2 exemption**: Some files may have low E2E coverage but high unit coverage — the merged view handles this naturally

## 15. Test Data Construction

### 15.1 Data Source Map

Each frontend feature draws data from different sources. Understanding this is critical for test data setup:

| Feature | Data Source | DB Table | API Endpoint | Test Setup |
|---------|-----------|----------|-------------|-----------|
| **Chat sessions** | SQLite | `chat_sessions` | `GET /api/chat/sessions` | Auto-created on first message |
| **Chat messages** | SQLite | `chat_history` | `GET /api/chat/{id}/messages` | Auto-created by chat flow |
| **Quick-send items** | SQLite | `chat_quick_send` | `GET /api/chat/quick-send` | Seed via `POST /api/chat/quick-send` |
| **File listing** | Filesystem | — | `GET /api/dir` | Uses project directory (has files) |
| **File content** | Filesystem | — | `GET /api/file/{path}` | Uses project directory (has files) |
| **Git history** | `git` CLI | — | `GET /api/git/history` | Project is a git repo (has commits) |
| **Git branches** | `git` CLI | — | `GET /api/git/branches` | Project has branches |
| **Settings** | `config.yaml` | — | `GET /api/config` | Test `config.yaml` in temp dir |
| **Scheduled tasks** | SQLite | `scheduled_tasks` | `GET /api/tasks` | Seed via `POST /api/tasks` |
| **Terminal** | PTY/WS | — | `WS /api/terminal/ws` | Terminal enabled in config |
| **Port forwarding** | SQLite + SSH | `forwarded_ports` | `GET /api/ssh/info` | Disabled in test config |

**Key insight:** There are only **3 SQLite tables** that need test data seeding: `chat_quick_send`, `chat_sessions`/`chat_history` (auto-seeded by chat flow), and `scheduled_tasks`. Everything else comes from the filesystem or `git` CLI.

### 15.2 Database Strategy

**Decision:** Use a **fresh per-test-run database** via temp directory isolation.

**Rationale:**
- The Go server creates `ClawBench.db` at `<BinDir>/.clawbench/ClawBench.db` on startup
- Setting `BinDir` to a temp directory gives each test run an empty database
- All tables are auto-created by `InitDB()` — no manual schema setup needed
- This mirrors the existing Go test pattern (`setupTestEnv()` uses `:memory:` for unit tests, but E2E needs a real file)

**Implementation:**

```typescript
// e2e/helpers/server.ts
import { mkdtempSync, writeFileSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

export async function startServer(): Promise<{ serverProcess: ChildProcess, tempDir: string, port: number }> {
  // 1. Create isolated temp directory for this test run
  const tempDir = mkdtempSync(join(tmpdir(), 'clawbench-e2e-'))
  const port = parseInt(process.env.E2E_PORT || '20100')
  const password = process.env.E2E_PASSWORD || 'e2e-test-password'

  // 2. Write minimal test config to temp dir
  //    The server looks for config/config.yaml relative to CWD
  const configDir = join(tempDir, 'config')
  mkdirSync(configDir, { recursive: true })
  writeFileSync(join(configDir, 'config.yaml'), `
server:
  port: ${port}
  password: "${password}"
  log_level: warn
  watch_dir: "${process.cwd()}"
chat:
  initial_messages: 20
  page_size: 20
terminal:
  enabled: true
  idle_timeout: 1h
port_forward:
  enabled: false
rag:
  enabled: false
`)

  // 3. Create .clawbench dir so DB is created in our temp dir
  mkdirSync(join(tempDir, '.clawbench'), { recursive: true })

  // 4. Write mock agent config
  const agentsDir = join(tempDir, 'config', 'agents')
  mkdirSync(agentsDir, { recursive: true })
  writeFileSync(join(agentsDir, 'mock.yaml'), `
id: mock
name: Mock Agent
icon: 🧪
specialty: E2E Testing
backend: mock
system_prompt: "You are a mock assistant for E2E testing."
`)

  // 5. Start server from temp dir so it picks up our config
  const serverProcess = spawn('./clawbench', [`--port`, String(port)], {
    cwd: tempDir,  // ← server finds config/config.yaml relative to CWD
    env: { ...process.env, CLAWBENCH_MOCK_BACKEND: '1' },
    stdio: ['pipe', 'pipe', 'pipe'],
  })

  await waitForServer(port, 30000)
  return { serverProcess, tempDir, port }
}
```

### 15.3 Test Data Seeding

Each test run starts with an **empty database**. Data is created via API calls in test fixtures or test bodies:

```typescript
// e2e/helpers/test-data.ts

/** Seed quick-send items via API */
export async function seedQuickSendItems(baseURL: string, items: { label: string; command: string }[]) {
  for (const item of items) {
    await fetch(`${baseURL}/api/chat/quick-send`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(item),
    })
  }
}

/** Default quick-send items for tests */
export const DEFAULT_QUICK_SEND_ITEMS = [
  { label: '继续', command: '继续' },
  { label: 'Review', command: '请 review 这个文件' },
  { label: 'Commit', command: '请提交当前的改动' },
]
```

**Data lifecycle per test:**

| Phase | Action |
|-------|--------|
| **Setup** | Start server with temp `BinDir` → empty DB → `InitDB()` creates tables |
| **Seed** | API calls create quick-send items, sessions, tasks as needed |
| **Test** | Test interacts with the UI, which reads/writes via the real API |
| **Teardown** | Kill server → delete temp directory → clean slate |

### 15.4 Why Not Mock the API?

The user chose **real backend**, which means:

- **No `page.route()` interception** — the browser talks to the real Go server
- **Database state is real** — created by actual handler/service code
- **SSE streaming is real** — actual `text/event-stream` from the Go handler
- **Only the AI CLI is mocked** — `MockAIBackend` replaces the non-deterministic AI subprocess

This gives **maximum confidence** that the frontend works correctly with the real backend, at the cost of slower tests and the need to manage server lifecycle.

### 15.5 Filesystem Test Data

Since the file manager and git features read directly from the filesystem, the test project directory must contain meaningful data. The options:

| Option | Pros | Cons |
|--------|------|------|
| **A: Use the ClawBench source repo itself** | Always has files, directories, git history | Sensitive to source changes; large directory |
| **B: Create a test fixture directory** | Controlled, stable, small | Needs maintenance; must create git history |
| **C: Generate test files on the fly** | Fully reproducible | Extra complexity in setup |

**Recommendation: Option A (use the source repo)** with Option B as fallback for git-specific tests that need a known commit history.

Rationale:
- The ClawBench source tree always has directories, code files, and a `.git` directory
- `watch_dir` in config points to `process.cwd()` (the source repo)
- No extra test fixture maintenance needed
- For git tests that need a specific history shape, create a small fixture repo under `e2e/fixtures/`

## 16. Test Pass/Fail Criteria

### 16.1 How Tests Determine Success

Playwright uses **web-first assertions** that auto-wait for conditions. A test passes when all assertions are satisfied within the timeout.

**Assertion hierarchy (from strongest to weakest):**

| Level | Pattern | Example | Confidence |
|-------|---------|---------|-----------|
| **1. DOM state** | `toBeVisible()`, `toHaveText()` | `await expect(page.locator('.chat-textarea')).toBeVisible()` | High — element exists and is visible |
| **2. Network response** | `waitForResponse()`, route intercept | `const res = await page.waitForResponse('**/api/ai/chat')` | High — API was called and returned |
| **3. SSE event** | EventSource listener | `await waitForSSEEvent('done')` | High — full stream completed |
| **4. UI behavior** | State change observation | `await expect(sendBtn).not.toBeVisible()` after sending | Medium — assumes specific UI flow |
| **5. Absence** | `not.toBeVisible()` | `await expect(hint).not.toBeVisible()` after typing | Medium — element gone but why? |

### 16.2 Per-Flow Pass/Fail Criteria

#### Auth Flow

| Test | Pass Condition | Fail Condition |
|------|---------------|---------------|
| Login with correct password | Chat panel is visible after login | Login form still showing; error message visible |
| Login with wrong password | Error message visible; still on login page | Redirected to chat (would be a security bug) |
| Session persistence | After reload, chat panel still visible | Redirected to login page |

#### Chat Flow

| Test | Pass Condition | Fail Condition |
|------|---------------|---------------|
| Send message | User message appears in chat; assistant response appears via SSE | Message not in DOM; no response after timeout |
| Quick-send hint visible | `.quick-send-hint` is visible when textarea is empty | Element not found or not visible |
| Quick-send hint hidden | `.quick-send-hint` is NOT visible after typing | Element still visible |
| Quick-send menu opens | `.quick-send-title` is visible after clicking send with empty input | Popup not visible |
| Quick-send item executes | Quick-send command text appears as user message | Message not sent or wrong text |
| Stop button appears | `.chat-stop-btn` is visible during AI response | Stop button not visible; AI completed too fast |

**Chat SSE stream assertion pattern:**

```typescript
// Wait for assistant response to appear (SSE stream)
test('should receive SSE stream reply', async ({ page }) => {
  await chat.sendMessage('Hello')

  // 1. User message appears immediately (synchronous POST)
  await expect(page.locator('.message-user').last()).toContainText('Hello')

  // 2. Assistant response appears (async SSE stream)
  //    MockAIBackend responds: "Hello! I am a mock assistant..."
  await expect(page.locator('.message-assistant').last()).toBeVisible({ timeout: 10000 })

  // 3. Response contains the mock text
  await expect(page.locator('.message-assistant').last())
    .toContainText('mock assistant')
})
```

#### File Manager Flow

| Test | Pass Condition | Fail Condition |
|------|---------------|---------------|
| Directory listing | File items are visible | No items; loading spinner stuck |
| Navigate into directory | Breadcrumb updates; new files shown | Same directory; breadcrumb unchanged |
| Open file | File viewer visible with content | Viewer not shown; empty content |
| Sort files | File order changes | Same order; no visual change |

#### Navigation Flow

| Test | Pass Condition | Fail Condition |
|------|---------------|---------------|
| Switch tab | Target tab's content is visible; dock button has `.active` class | Previous tab still showing |
| Preserve draft | After switching away and back, textarea still has value | Textarea is empty |
| Overflow menu | Overflow popup visible with History/Terminal items | Popup not visible |

#### Git Flow

| Test | Pass Condition | Fail Condition |
|------|---------------|---------------|
| History loads | Commit list is visible | Empty list; loading stuck |
| Branch list | Branch items visible in manage tab | No branches; error shown |
| Commit diff | Diff view shows changes | No diff content |

### 16.3 Common Failure Modes & Diagnosis

| Failure Mode | Symptom | Root Cause | Debug Method |
|-------------|---------|-----------|-------------|
| **Timeout** | `Timeout 10000ms exceeded` | Element not appearing; wrong selector | Screenshot + trace shows what was on screen |
| **Flaky** | Passes locally, fails in CI | Timing; network; font loading | `trace: 'on-first-retry'` captures DOM state |
| **Wrong assertion** | `Expected "X" but got "Y"` | UI changed; i18n mismatch | Screenshot shows actual text |
| **Server error** | 500 response; error toast | Backend crash; mock not registered | Server stderr log; `page.locator('.toast-error')` |
| **Auth failure** | Redirected to login | Cookie not set; wrong password | Check `storageState`; test `GET /api/me` |
| **SSE disconnect** | No assistant response | Server crash; connection dropped | Check server log; trace shows network |

### 16.4 Playwright Built-In Failure Handling

Playwright automatically captures diagnostics on failure:

```typescript
// playwright.config.ts — already configured
use: {
  trace: 'on-first-retry',          // DOM snapshot + network on first retry
  screenshot: 'only-on-failure',     // PNG of page state at failure
  video: 'retain-on-failure',       // Keep video only for failed tests
}
```

**What each artifact shows:**

| Artifact | What You See | When Generated |
|----------|-------------|---------------|
| **Screenshot** | The page as it looked at the moment of failure | Every failed assertion |
| **Trace** | Full DOM snapshot, network requests, console logs, actions timeline | First retry of any test |
| **Video** | Screen recording of the entire test execution | Failed tests only (`retain-on-failure`) |
| **HTML Report** | All of the above, organized per test | Always (after `npx playwright show-report`) |

## 17. `data-testid` Strategy & Artifact Retention

### 17.1 `data-testid` Adoption Plan

**What it is:** A custom HTML attribute (`data-testid="unique-name"`) dedicated to test element selection, decoupled from CSS class names and i18n text.

**Benefits for ClawBench:**

| Scenario | Without `data-testid` | With `data-testid` |
|----------|----------------------|-------------------|
| Rename CSS class | Test breaks silently | Test unaffected |
| i18n text changes | `getByText('Send')` breaks in zh | `getByTestId('chat-send')` works in all languages |
| Multiple same-class elements | `.dock-btn:nth(2)` — fragile index | `getByTestId('tab-browse')` — stable |
| Refactor component structure | DOM change breaks locator | Test targets semantic ID, survives restructure |

**Adoption approach: Incremental, not big-bang.**

Phase 1 (initial E2E setup): Use existing CSS classes — they are already semantic (`.chat-send-btn`, `.quick-send-hint`). Add `data-testid` only where CSS selectors are fragile (dock tab buttons by index).

Phase 2 (after initial tests stabilize): Add `data-testid` to elements that caused test fragility during Phase 1.

**Priority `data-testid` additions** (fragile selectors that need fixing first):

| Element | Current Selector | Problem | Proposed `data-testid` |
|---------|----------------|---------|----------------------|
| Dock: Chat tab | `.dock-btn:nth(0)` | Index-based, fragile | `tab-chat` |
| Dock: Files tab | `.dock-btn:nth(2)` | Index-based, fragile | `tab-browse` |
| Dock: Viewer tab | `.dock-btn:nth(1)` | Index-based, fragile | `tab-viewer` |
| Dock: Tasks tab | `.dock-btn:nth(3)` | Index-based, fragile | `tab-tasks` |
| Overflow button | `.dock-overflow-btn` | OK but could change | `dock-overflow` |
| Login form | Various | Multiple inputs | `login-password`, `login-submit` |

### 17.2 Artifact Retention & CI Storage

**Local development:**
```
test-results/          ← Screenshots, traces, videos (gitignored)
playwright-report/     ← HTML report (gitignored)
```

**CI (GitHub Actions):**

```yaml
# Upload HTML report (contains embedded screenshots and trace links)
- name: Upload Playwright report
  if: ${{ !cancelled() }}
  uses: actions/upload-artifact@v4
  with:
    name: playwright-report
    path: playwright-report/
    retention-days: 30

# Upload raw test results (screenshots, traces, videos for download)
- name: Upload test results
  if: ${{ !cancelled() }}
  uses: actions/upload-artifact@v4
  with:
    name: playwright-results
    path: test-results/
    retention-days: 7    # Short retention — large files

# Upload server logs (for backend error diagnosis)
- name: Upload server logs
  if: ${{ !cancelled() }}
  uses: actions/upload-artifact@v4
  with:
    name: e2e-server-logs
    path: /tmp/clawbench-e2e-*/.clawbench/logs/
    retention-days: 7
```

**Artifact size estimation:**

| Artifact | Per Test | Full Suite (~30 tests) | Notes |
|----------|----------|------------------------|-------|
| Screenshots | ~200 KB | ~6 MB | Only failures; PNG |
| Traces | ~2 MB | ~60 MB | Only first retry; ZIP |
| Videos | ~5 MB | ~150 MB | Only failures; WebM |
| HTML Report | — | ~10 MB | Embeds all artifacts |
| Server logs | — | ~1 MB | Text only |

**Total CI storage per run: ~30 MB** (most artifacts only generated on failure).

## 18. Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| V8 coverage path mismatch with source maps | Use `v8-to-istanbul` with source map support; validate in Phase 1 before merging |
| E2E tests flaky due to timing | Playwright auto-wait assertions; CI retries = 2; no `sleep` in tests |
| MockBackend diverges from real behavior | Keep mock responses simple (content → metadata → done); test real SSE format |
| Coverage collection slows Chromium tests | `reportAnonymousScripts: false` by default; only convert `/src/` files |
| Cross-browser inconsistencies | Chromium = reference; Firefox/WebKit failures are real bugs to fix |
| Go server startup time in CI | Build once, reuse across test shards; `waitForServer` poll pattern |

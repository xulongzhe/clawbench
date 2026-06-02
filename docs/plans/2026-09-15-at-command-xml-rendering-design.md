# @ Command & XML Rendering Design

Date: 2026-09-15

## Overview

Introduce an **@ command system** for on-demand capability injection, replacing the current approach of embedding RAG and scheduled task instructions in the global system prompt. When users type `@chatsearch` or `@task`, the backend injects the relevant command usage guide and output format instructions into the user message before forwarding to the AI. This keeps the system prompt lean and only provides capability context when actually needed.

Additionally, migrate the `<ask-question>` structured output format from JSON to pure XML (all data in child element text nodes, no attributes), and add a new `<rag-results>` XML format for rendering RAG search result cards.

## @ Command System

### Commands

| Command | Trigger | Injected Content | AI Behavior |
|---------|---------|-----------------|-------------|
| `@chatsearch <query>` | User types in chat input | RAG search CLI usage + `<rag-results>` XML output format spec | AI calls `clawbench rag search` via Bash → outputs structured `<rag-results>` XML |
| `@task <description>` | User types in chat input | Task management CLI usage + `<scheduled-task>` tag requirement | AI calls `clawbench task create` via Bash → outputs `<scheduled-task id="..." />` |

### Frontend Responsibilities

- **Autocomplete menu**: When user types `@` in the chat input bar, show a popup menu listing available commands (VS Code-style autocomplete). Selecting a command inserts it into the input.
- **Message rendering**: Display the `@` prefix as a styled badge/tag in the user message bubble (e.g., `@chatsearch` rendered as a highlighted chip, followed by the query text).
- **No command processing**: The frontend sends the message as-is (including the `@` prefix). All command detection and injection happens on the backend.
- **XML parsing**: Parse `<rag-results>` XML from AI responses and render as structured cards.

### Backend Responsibilities

1. **Command detection**: In the chat handler, check if `req.Message` (the raw user message, **before** any file path prefixes are added) starts with `@chatsearch ` or `@task `.
2. **Template injection**: Load the corresponding injection template from Go code constants, replace placeholders (`{{CLAWBENCH_BIN}}`, `{{PROJECT_PATH}}`, `{{SESSION_ID}}`).
3. **Message concatenation**: Prepend the injected template to the user's original message before sending to the AI backend.
4. **Database storage**: Store the **original user message** (including the `@` prefix) in `chat_history`. The injected template is ephemeral and never persisted.
5. **RAG availability check**: Before injecting `@chatsearch` template, verify RAG is enabled. If `rag.enabled = false`, return an error response instead of injecting.
6. **Empty query rejection**: If the user sends `@chatsearch ` with no query text (only whitespace after the command), return an error response similar to the existing `MessageOrFilesRequired` error.

### Injection Templates (Go Constants)

#### @chatsearch Template

```go
const chatSearchInjectTemplate = `[You have access to historical conversation search for this request. Use the Bash tool to execute commands.]

Search historical conversations: {{CLAWBENCH_BIN}} rag search -q "search terms" --project {{PROJECT_PATH}} --exclude-session-id {{SESSION_ID}}

Command flags:
- -q: Search query (required)
- --limit: Number of results (default 5)
- --project: Project path (required)
- --exclude-session-id: Exclude current session (required)
- --backend: Filter by backend
- --role: Filter by role (user/assistant)
- --from / --to: Time range

The search results include session_title for each match. Use it directly in your output.

After searching, you MUST output results using this XML format:

<rag-results>
  <rag-item>
    <session-id>session-id-here</session-id>
    <session-title>Session Title</session-title>
    <created-at>2026-01-01T12:00:00Z</created-at>
    <summary>Concise summary based on search results</summary>
  </rag-item>
</rag-results>

You may summarize or supplement the chunk content in <summary>.
If no results found, answer based on your own knowledge — do NOT mention the search process.
`
```

#### @task Template

```go
const taskInjectTemplate = `[You have access to scheduled task management for this request. Use the Bash tool to execute commands.]

Task management: {{CLAWBENCH_BIN}} task --project {{PROJECT_PATH}}

Available subcommands: create / list / get / list-exec / update / delete / pause / resume / trigger / list-agents

When creating a task, use the --agent-id flag. Run "{{CLAWBENCH_BIN}} task list-agents --project {{PROJECT_PATH}}" to discover available agent IDs. You may use the current session's agent if appropriate.

After creating a task, you MUST include in your response: <scheduled-task id="task-id" />

Rules:
- Always validate cron expression before creating a task
- Never create extremely high frequency tasks (e.g. * * * * *) without user confirmation
- Use the user's language for task names and prompts
`
```

### Backend Injection Logic

```go
// processAtCommand checks if the raw user message starts with an @ command
// and returns the prompt with the injected template prepended.
// rawMsg is the original req.Message (before file path prefixes).
// The returned string replaces the prompt passed to buildChatRequest().
func processAtCommand(rawMsg, projectPath, sessionID string) string {
    if strings.HasPrefix(rawMsg, "@chatsearch ") {
        query := strings.TrimPrefix(rawMsg, "@chatsearch ")
        if strings.TrimSpace(query) == "" {
            // Will be handled by caller as an error response
            return rawMsg
        }
        template := strings.ReplaceAll(chatSearchInjectTemplate, "{{CLAWBENCH_BIN}}", model.ClawbenchBin)
        template = strings.ReplaceAll(template, "{{PROJECT_PATH}}", projectPath)
        template = strings.ReplaceAll(template, "{{SESSION_ID}}", sessionID)
        return template + "\n\n" + rawMsg
    }
    if strings.HasPrefix(rawMsg, "@task ") {
        template := strings.ReplaceAll(taskInjectTemplate, "{{CLAWBENCH_BIN}}", model.ClawbenchBin)
        template = strings.ReplaceAll(template, "{{PROJECT_PATH}}", projectPath)
        return template + "\n\n" + rawMsg
    }
    return rawMsg
}
```

**Key implementation detail**: The `@` command detection must happen on `req.Message` (the raw user input), NOT on the constructed `prompt` variable. The `prompt` variable may already contain `[Current file: ...]` prefixes prepended by the chat handler (chat.go lines 296-304), which would cause `strings.HasPrefix(prompt, "@chatsearch ")` to fail. The detection runs on `req.Message`, and the resulting injected prompt replaces the `prompt` value before it reaches `buildChatRequest()`.

### Queue Message Handling

When a session is already running, user messages are enqueued and processed later via `buildChatRequestFromQueue()` (chat.go line 887). If a queued message starts with `@chatsearch ` or `@task `, the same injection must apply. The `buildChatRequestFromQueue()` function constructs the prompt from `qMsg.Text` — the injection must happen on `qMsg.Text` before building the chat request, using the same `processAtCommand()` function.

### Session ID for --exclude-session-id

The `@chatsearch` template includes `--exclude-session-id {{SESSION_ID}}`. The session ID used must be the **ClawBench UUID** (the `session_id` parameter in the chat handler), not the `external_session_id`. The RAG CLI routes through the server API which uses ClawBench IDs internally.

### @chatsearch During Scheduled Execution

If a scheduled task's prompt happens to start with `@chatsearch`, the backend would inject RAG search instructions. This is **intentional and desirable** — scheduled tasks may legitimately need to search history.

## rules.md Changes

### Remove

1. **`<!-- SCHEDULED_BEGIN -->...<!-- SCHEDULED_END -->`** — entire Scheduled Tasks section
2. **`## RAG History Search`** — entire section
3. **Ask-question JSON format spec** in `## User Interaction` — replace with XML format spec

### Replace

The `## User Interaction` section's ask-question format spec changes from JSON to XML:

```markdown
### How to ask questions

- **ALWAYS** output an `<ask-question>` XML tag with pure XML content (no JSON, no attributes).
- **NEVER** use the `AskUserQuestion` tool call — it will be rejected by the CLI and result in an error.

Format:
<ask-question>
  <item>
    <header>Section Header (max 12 chars)</header>
    <multi-select>false</multi-select>
    <question>Your question here?</question>
    <option>
      <label>Option A</label>
      <description>Description of option A</description>
    </option>
    <option>
      <label>Option B</label>
      <description>Description of option B</description>
    </option>
  </item>
</ask-question>

**Important:** Use pure XML child elements for all data — do NOT use tag attributes (attributes are invisible if rendering fails). Do NOT wrap in markdown code fences.
```

### BuildCommonPrompt Simplification

The `<!-- SCHEDULED_BEGIN/END -->` marker mechanism in `BuildCommonPrompt()` can be removed since the scheduled task section no longer exists in rules.md. The anti-recursion concern is now handled differently: since `@task` instructions are only injected when the user explicitly uses `@task`, they will never appear during scheduled task execution.

**Note**: The `CLAWBENCH_SCHEDULED=1` environment variable still exists for anti-recursion at the CLI level. The `BuildCommonPrompt(scheduled bool)` function should still accept the parameter for backward compatibility but no longer needs to strip any section.

## XML Formats

### Design Principle

All data is in **child element text nodes** — never in tag attributes. Rationale: if the frontend XML parser fails, attribute values are completely invisible to the user, but child element text content is still readable as plain text within the tags.

**Exception**: `<scheduled-task id="42" />` retains the attribute format because the task ID is meaningless to users even if visible, and changing it would require updating both the AI output format and the existing `extractScheduledTaskIds()` regex parser with no practical benefit.

### `<ask-question>` (Modified)

**Before** (XML wrapping JSON):
```xml
<ask-question>
{"questions":[{"header":"Approach","multiSelect":false,"options":[{"label":"Option A","description":"Fast"},{"label":"Option B","description":"Safe"}],"question":"Which approach?"}]}
</ask-question>
```

**After** (pure XML):
```xml
<ask-question>
  <item>
    <header>Approach</header>
    <multi-select>false</multi-select>
    <question>Which approach?</question>
    <option>
      <label>Option A</label>
      <description>Fast</description>
    </option>
    <option>
      <label>Option B</label>
      <description>Safe</description>
    </option>
  </item>
</ask-question>
```

Frontend interaction logic remains **unchanged** (click option → submit button). Only the parsing layer changes.

### Backward Compatibility for Historical Messages

**Dual parsing strategy**: The frontend and backend parsers must support **both** XML and JSON formats inside `<ask-question>` tags. This is necessary because:

1. Historical messages in the database contain JSON-format `<ask-question>` content
2. If a user views old chat history, JSON-format messages must still render as interactive cards
3. New AI output will use XML format (per the updated rules.md)

**Parsing order**: Try XML first → if XML parsing fails (no `<item>` child elements found), fall back to JSON parsing. The existing `extractJSONCandidate()` function in `chat.go` serves as the JSON fallback path.

**Backend `convertAskQuestionBlocks`**: The existing function in `chat.go` (line 1031) currently only parses JSON. It must be updated to try XML parsing first:

1. Extract content between `<ask-question>` and `</ask-question>` (existing regex logic)
2. Try parsing as XML: look for `<item>` child elements
3. If XML parsing succeeds, build the `tool_use` ContentBlock from XML child elements
4. If XML parsing fails, fall back to existing `extractJSONCandidate()` → JSON parsing

**Frontend `parseAskQuestionContent`**: Similarly updated in `chatRenderUtils.ts` — try XML parsing first, fall back to JSON.

### `<rag-results>` (New)

```xml
<rag-results>
  <rag-item>
    <session-id>abc-123</session-id>
    <session-title>Fix Login Bug</session-title>
    <created-at>2026-07-01T10:30:00Z</created-at>
    <summary>Authentication module refactor resolved the JWT expiry issue</summary>
  </rag-item>
  <rag-item>
    <session-id>def-456</session-id>
    <session-title>OAuth Integration</session-title>
    <created-at>2026-06-28T14:20:00Z</created-at>
    <summary>Added Google OAuth provider with PKCE flow</summary>
  </rag-item>
</rag-results>
```

### `<scheduled-task>` (Unchanged)

```xml
<scheduled-task id="42" />
```

No changes — the task ID in attributes is fine because it's meaningless to users even if visible.

### XML Parsing Strategy

**Detection**: Use simple string matching / regex for detection only:
- `<ask-question>`: existing `text.includes('<ask-question')` check in `streamPerf.ts`
- `<rag-results>`: new `text.includes('<rag-results')` check

**Parsing**: Use `DOMParser` for actual XML parsing (available in all browsers). This is more robust than regex for nested structures like `<rag-results>` and the new XML-format `<ask-question>`. Regex-based parsing is fragile for multi-level nested elements with text content.

```typescript
function parseRagResultsXML(xmlString: string): RagItem[] {
  const parser = new DOMParser()
  const doc = parser.parseFromString(xmlString, 'text/xml')
  const items = doc.querySelectorAll('rag-item')
  return Array.from(items).map(item => ({
    sessionId: item.querySelector('session-id')?.textContent || '',
    sessionTitle: item.querySelector('session-title')?.textContent || '',
    createdAt: item.querySelector('created-at')?.textContent || '',
    summary: item.querySelector('summary')?.textContent || '',
  }))
}
```

**Malformed XML fallback**: If `DOMParser` returns a parse error (`<parsererror>` element), fall back to displaying the raw text. Since all data is in child element text nodes (not attributes), even unrendered XML is human-readable.

## RAG Result Card Rendering

### Style

Similar to the existing Ask User Question card style:

```
┌─────────────────────────────────────────┐
│  📋 Fix Login Bug          2026-07-01   │
│                                         │
│  Authentication module refactor         │
│  resolved the JWT expiry issue          │
│                                         │
│              [ 恢复对话 ]                │
└─────────────────────────────────────────┘
```

Each card contains:
- **Header**: Session title (left) + relative time (right), styled with orange/accent color
- **Body**: Summary text from AI's `<summary>` element
- **Footer**: "Resume conversation" button

Multiple cards stack vertically.

### Streaming UX

During streaming, the raw `<rag-results>` XML will be visible as plain text in the chat. This is consistent with the existing behavior for `<ask-question>` and `<scheduled-task>` tags. The XML is replaced by structured cards only after streaming completes (in `renderTextBlock()` with `streaming=false`).

Since all data is in child element text nodes, the raw XML is reasonably readable during streaming — the user can see `<session-title>Fix Login Bug</session-title>` etc. This is an intentional trade-off: simpler implementation vs. hiding content during streaming.

### Resume Conversation Flow

1. User clicks "Resume conversation" button on a `<rag-item>` card
2. **Confirmation dialog** appears showing session title + creation time. Use a `BottomSheet` component (existing pattern) for the confirmation UI.
3. User confirms → **Session count validation** (check against `session.max_count`)
4. If session is soft-deleted → **Restore** (`UPDATE chat_sessions SET deleted = 0`)
5. **Navigate** to the session using `useSessionManager.switchSession(sessionId)`

### Backend API

New endpoint: `POST /api/ai/session/resume`

```json
Request:  { "session_id": "abc-123" }
Response: { "ok": true, "session_id": "abc-123" }
```

Logic:
1. **Project isolation**: Use `requireProject(w, r)` (reads from auth cookie) + verify `GetSessionProjectPath(sessionID) == projectPath`. This follows the existing project isolation pattern used in `ServeRAGSession` and other endpoints.
2. **Session count validation**: Check `session.max_count` limit before restoring
3. If soft-deleted, restore (`deleted = 0`)
4. Return session ID

### Frontend Data Flow

1. AI outputs `<rag-results>` XML in text block
2. `renderTextBlock()` in `useChatRender.ts` detects `<rag-results>` tag (post-streaming only)
3. Parse XML via `DOMParser` → extract `<rag-item>` elements
4. Store parsed results in `blockRagResults` reactive state (keyed by `${msgId}-${blockIdx}`)
5. Strip `<rag-results>` XML from rendered text
6. `ContentBlocks.vue` renders cards from `blockRagResults` state
7. "Resume conversation" button click → confirmation BottomSheet → API call → navigate

### Reactive State Management

Follow existing patterns:
- **`blockRagResults`**: Reactive object keyed by `${msgId}-${blockIdx}`, matching `blockAskQuestions` pattern
- **Session switch clearing**: Clear `blockRagResults` on session switch, like the existing `staticBlockCache.clear()` in `useChatRender.ts` (watch on `currentSessionId`)
- **NOT a module-level singleton**: `blockRagResults` is part of `useChatRender` composable state, not a separate module-level singleton. This matches the existing `blockTasks` and `blockAskQuestions` pattern.

### i18n

All user-facing strings in RAG cards must go through the i18n system:
- "Resume conversation" button → i18n key
- Confirmation dialog text → i18n key
- Session title and summary are AI-generated (not i18n'd)
- Time display uses existing relative time formatting

## @ Command Autocomplete

### Trigger

When user types `@` as the first character in the chat input bar, show a floating menu **above** the input (not below — on mobile, below would be off-screen or obscured by the keyboard).

### Menu Items

| Command | Description | Availability |
|---------|-------------|-------------|
| `@chatsearch` | Search historical conversations | Only shown when RAG is enabled (`rag.enabled = true`) |
| `@task` | Manage scheduled tasks | Always available |

### Availability Detection

The frontend needs to know if RAG is enabled to show/hide `@chatsearch` from the autocomplete menu. The existing settings config (`useSettingsConfig`) already fetches server config including `rag.enabled`. The autocomplete component should read this value.

### Behavior

- Typing after `@` filters the menu (e.g., `@ch` → shows `@chatsearch`)
- Arrow keys or tap to select
- Enter or tap to insert the command + space
- Clicking outside dismisses the menu
- Menu only appears at the start of input (not mid-sentence)

### Component Design

The autocomplete is a local component within `ChatInputBar.vue` (or a small composable `useAtCommandAutocomplete()`). It is **NOT** a module-level singleton — it's UI-only and session-specific. This follows the codebase convention: stateless UI → local composable; shared cross-component state → module-level singleton.

Use the existing `PopupMenu` component (`web/src/components/PopupMenu.vue`) for auto-positioning, or implement a similar lightweight popup that appears above the input.

## Message Display

### User Message Bubble

When the stored message starts with `@chatsearch ` or `@task `:
- Render the `@command` part as a styled badge/chip (e.g., rounded pill with accent background)
- Followed by the query/description text in normal style

Example: `@chatsearch 如何修复Bug` renders as:

```
[ @chatsearch ]  如何修复Bug
```

Where `[ @chatsearch ]` is a styled chip/tag.

### Historical Message Replay

When messages with `@` prefix are copied via `ContinueFromExecution` (which copies `chat_history` verbatim), the `@` prefix will be present in the continued session's history. This is harmless — the `@` command injection only happens on new messages being sent to the AI, not on historical messages being loaded. The CLI backend reads historical messages as context, not as new prompts.

## Implementation Plan

### Phase 1: Backend @ Command Injection

1. Add `chatSearchInjectTemplate` and `taskInjectTemplate` as Go constants
2. Add `processAtCommand()` function in chat handler — detect on `req.Message`, not constructed `prompt`
3. Integrate into `chat_stream.go` message processing pipeline (before `buildChatRequest()`)
4. Apply same injection in `buildChatRequestFromQueue()` for queued messages
5. Add RAG availability check — return error if `@chatsearch` used when RAG is disabled
6. Add empty query rejection for `@chatsearch`
7. Store original message (with `@` prefix) in `chat_history`
8. Add `POST /api/ai/session/resume` endpoint with project isolation
9. Update `BuildCommonPrompt` — remove SCHEDULED block stripping logic

### Phase 2: Frontend XML Parsing

1. Add XML parser for `<rag-results>` tag using `DOMParser` — detection in `streamPerf.ts`, parsing in `chatRenderUtils.ts`
2. Migrate `<ask-question>` parsing to support XML format (try XML first, fall back to JSON for backward compat)
3. Add `<rag-results>` detection in `renderTextBlock()` (post-streaming only)
4. Strip XML tags from rendered text
5. Store parsed results in `blockRagResults` reactive state
6. Clear `blockRagResults` on session switch
7. Update backend `convertAskQuestionBlocks()` to try XML parsing first, fall back to JSON

### Phase 3: Frontend Card Rendering

1. Add `<rag-results>` card template in `ContentBlocks.vue`
2. Style cards similar to Ask User Question
3. Add "Resume conversation" button with confirmation BottomSheet
4. Wire up resume flow: API call → session restore → navigate
5. Add i18n keys for card UI strings

### Phase 4: @ Command Autocomplete

1. Add autocomplete popup component in `ChatInputBar.vue` (positioned above input)
2. Detect `@` at start of input, show command menu
3. Filter `@chatsearch` based on RAG availability (from settings config)
4. Handle selection (insert command + space)
5. Filter as user types

### Phase 5: rules.md Cleanup

1. Remove `<!-- SCHEDULED_BEGIN/END -->` section
2. Remove `## RAG History Search` section
3. Replace ask-question JSON format spec with XML format spec
4. Verify AI behavior with new XML format

### Phase 6: User Message Display

1. Detect `@` prefix in user message rendering
2. Render command part as styled badge/chip
3. Display query/description text normally

## Testing Considerations

- **XML parser**: Unit tests for both `<ask-question>` (XML + JSON fallback) and `<rag-results>` XML parsing, including malformed XML fallback (should show raw text gracefully)
- **@ command detection**: Backend tests for `processAtCommand()` — verify injection only triggers on exact prefix match on `req.Message`, not on constructed `prompt` with file prefixes
- **Queue message injection**: Backend tests for `@` command injection in `buildChatRequestFromQueue()` path
- **Session resume**: Backend tests for count limit validation, soft-delete restore, project ownership check, RAG disabled error
- **Autocomplete**: Frontend tests for menu trigger, filtering, selection, RAG availability filtering
- **Historical JSON backward compat**: Frontend + backend tests for rendering old JSON-format `<ask-question>` content
- **Empty query rejection**: Backend test for `@chatsearch ` with no query text

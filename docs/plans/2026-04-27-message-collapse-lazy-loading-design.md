# Message Collapse & Lazy Loading Design

## Overview

Historical chat messages (except the latest round) are collapsed to a fixed height with click-to-expand. Only the most recent N messages are initially loaded, with scroll-up lazy loading for older messages.

## Design Decisions

| Item | Decision |
|---|---|
| Expanded messages | Last 1 round (last assistant + its preceding user message) |
| Collapsed height | 150px with gradient fade + "click to expand" button |
| Initial message count | 20 (most recent) |
| Lazy load batch size | 20 per load |
| Config location | Backend config.yaml, served via /api/config |

## Implementation Steps

### Step 1: Backend config & pagination API

**config.yaml** - new `chat` section:
```yaml
chat:
  initial_messages: 20
  page_size: 20
  collapsed_height: 150
```

**Go changes:**
- `internal/model/config.go`: Add `Chat` struct to Config
- `internal/handler/chat.go`: Add `limit` and `offset` query params to GET `/api/ai/chat`, add `total` to response
- Config loading code: parse new `chat` section
- `/api/config` endpoint: include chat config values

### Step 2: Frontend config integration

- Fetch chat config from `/api/config` on app init
- Store in app store (`stores/app.ts`): `chatInitialMessages`, `chatPageSize`, `chatCollapsedHeight`
- Fall back to defaults (20, 20, 150) if config unavailable

### Step 3: ChatMessageItem collapse UI

- Add `collapsed` prop to ChatMessageItem
- When collapsed: `max-height: 150px` + `overflow: hidden` + gradient overlay + expand button
- Streaming messages never collapse
- Click expand button emits `expand` event

### Step 4: ChatMessageList collapse logic

- Compute which messages should be collapsed (all except last round)
- Track expanded state in a Set (message indices that user manually expanded)
- Pass `collapsed` prop to each ChatMessageItem
- Handle expand event to update state

### Step 5: Lazy loading

- useChatSession: add `totalMessages` ref, `hasMore` computed, `loadMoreMessages()` method
- ChatMessageList: scroll listener, trigger loadMore when scrollTop < 50
- Loading indicator at top during fetch
- Preserve scroll position after prepend (scrollTop adjustment)
- Prevent duplicate loads with loading guard

# Quote Question (引用提问) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the long-press line editing menu with a "quote & ask" feature that lets users select text in code/markdown previews and send it as a formatted quote to a chat session.

**Architecture:** Remove `useLongPressLineMenu` composable and all related UI from `CodePreview.vue`. Add a new `useQuoteQuestion` composable that monitors text selection, and two new components: a floating `QuoteQuestionBar` (above dock) and a `QuoteQuestionSheet` (BottomSheet with session selector, quote preview, and input). Communication between ChatPanel and quote components uses Vue provide/inject.

**Tech Stack:** Vue 3 Composition API, TypeScript, existing BottomSheet component, `window.getSelection()` API.

**Worktree:** `.worktrees/quote-question` on branch `feature/quote-question`

---

### Task 1: Remove long-press line menu from CodePreview.vue

**Files:**
- Delete: `web/src/composables/useLongPressLineMenu.ts`
- Modify: `web/src/components/file/CodePreview.vue`
- Modify: `web/src/components/file/MarkdownPreview.vue` (remove `editable` and `@content-change` props)
- Modify: `web/src/components/file/FileViewer.vue` (remove `editable` and `@content-change` props from CodePreview usage)

**Step 1: Delete the composable file**

```bash
rm web/src/composables/useLongPressLineMenu.ts
```

**Step 2: Strip CodePreview.vue of all long-press menu code**

Remove from template:
- The entire `<Teleport>` block with context menu (lines 7-26)
- The `<BottomSheet>` edit dialog (lines 28-42)

Remove from script:
- Import `useLongPressLineMenu` (line 49)
- Import `BottomSheet` (line 50)
- `editable` prop definition (line 60)
- `emit('content-change')` definition (line 63)
- `internalContent`, `getContent()`, `setContent()` — no longer needed without edit
- All destructured values from `useLongPressLineMenu` (lines 79-92)
- `editDrawerTitle` computed (lines 95-99)
- `closeEditDrawer` function (lines 102-105)
- All three watchers: `editingLine/insertMode`, `copiedLine`, `highlightedLine` (lines 108-144)
- Remove `computed` from Vue imports (no longer used)

Remove from scoped CSS:
- `.line-context-backdrop` (lines 176-180)
- `.line-context-menu` (lines 182-191)
- `.line-context-item` (lines 193-210)
- `.line-context-item.danger` (lines 212-222)
- `.line-edit-textarea` (lines 224-239)
- `.line-edit-actions` and children (lines 241-273)
- `.code-line.line-editing` (lines 337-340)
- `.code-line.line-highlighted` (lines 342-344)
- `.code-line.line-copied` (lines 346-348)
- `.line-insert-marker` and variants (lines 350-360)
- `.line-num.copied` (lines 332-335)

Remove from non-scoped CSS:
- `@keyframes line-flash` and `.line-flash` (lines 365-375)

Keep in CSS:
- `@keyframes copy-flash` and `.copy-flash` — still used by `useDoubleClickCopy`
- `.line-num:hover` — still useful for line number interaction
- All `.raw-content-pre` styles
- `.code-line`, `.line-num`, `.code-text` styles

Also change:
- `pre` CSS: Remove `-webkit-touch-callout: none; -webkit-user-select: none; user-select: none;` — we WANT text selection now
- Add `user-select: text` to `pre` and `:deep(code)` so native text selection works

**Step 3: Update MarkdownPreview.vue**

Remove `editable` prop and `@content-change` from CodePreview usage:
```vue
<!-- Before -->
<CodePreview
  v-else
  :content="file.content"
  language="markdown"
  :file-path="file.path"
  :editable="true"
  @content-change="file.content = $event"
/>

<!-- After -->
<CodePreview
  v-else
  :content="file.content"
  language="markdown"
  :file-path="file.path"
/>
```

**Step 4: Update FileViewer.vue**

Remove `editable` and `@content-change` from CodePreview usage:
```vue
<!-- Before -->
<CodePreview
  :content="file.content"
  :language="rawFileLanguage"
  :file-path="file.path"
  :editable="true"
  @content-change="file.content = $event"
/>

<!-- After -->
<CodePreview
  :content="file.content"
  :language="rawFileLanguage"
  :file-path="file.path"
/>
```

**Step 5: Verify build**

```bash
cd /home/xulongzhe/projects/clawbench/.worktrees/quote-question
npm run build 2>&1 | tail -20
```

Expected: Build succeeds with no errors.

**Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove long-press line editing menu from CodePreview"
```

---

### Task 2: Create useQuoteQuestion composable

**Files:**
- Create: `web/src/composables/useQuoteQuestion.ts`

This composable handles:
1. Monitoring text selection via `document.selectionchange`
2. Detecting whether selection is inside a code/markdown preview area
3. Extracting selected text, file path, language, and line numbers
4. Formatting the message as a code block quote
5. Sending the message to a chat session via the injected `sendQuoteMessage` function

**Step 1: Create the composable**

```typescript
// web/src/composables/useQuoteQuestion.ts
import { ref, onMounted, onUnmounted, inject, type Ref } from 'vue'

export interface QuoteData {
  text: string           // selected text
  filePath: string       // file path
  language: string       // language identifier (empty for markdown preview)
  startLine: number      // start line number (1-based, 0 if unknown)
  endLine: number        // end line number (1-based, 0 if unknown)
}

export interface QuoteQuestionState {
  visible: Ref<boolean>
  quoteData: Ref<QuoteData | null>
  sheetOpen: Ref<boolean>
  openSheet: () => void
  closeSheet: () => void
  sendMessage: (userMessage: string) => Promise<void>
}

// Module-level singleton: selection state shared across all consumers
const selectionText = ref('')
const quoteData = ref<QuoteData | null>(null)
const barVisible = ref(false)
const sheetOpen = ref(false)

let debounceTimer: ReturnType<typeof setTimeout> | null = null

/**
 * Get line numbers from a selection range inside a code preview.
 * Walks up from anchor/focus nodes to find .code-line[data-line] elements.
 */
function getLineInfo(selection: Selection): { startLine: number; endLine: number } {
  const anchor = (selection.anchorNode as HTMLElement)?.closest?.('.code-line')
  const focus = (selection.focusNode as HTMLElement)?.closest?.('.code-line')
  if (!anchor || !focus) return { startLine: 0, endLine: 0 }

  const anchorLine = parseInt(anchor.getAttribute('data-line') || '0')
  const focusLine = parseInt(focus.getAttribute('data-line') || '0')
  return {
    startLine: Math.min(anchorLine, focusLine),
    endLine: Math.max(anchorLine, focusLine),
  }
}

/**
 * Get the file path and language from the container element.
 */
function getFileInfo(container: HTMLElement): { filePath: string; language: string } {
  // Walk up to find the preview container that has data attributes
  const codePreview = container.closest('.raw-content-pre')
  if (codePreview) {
    const filePath = codePreview.getAttribute('data-file-path') || ''
    const language = codePreview.getAttribute('data-language') || ''
    return { filePath, language }
  }
  const markdownBody = container.closest('.markdown-body')
  if (markdownBody) {
    const filePath = markdownBody.getAttribute('data-file-path') || ''
    return { filePath, language: '' }
  }
  return { filePath: '', language: '' }
}

function onSelectionChange() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    const sel = window.getSelection()
    if (!sel || sel.isCollapsed || !sel.toString().trim()) {
      barVisible.value = false
      selectionText.value = ''
      quoteData.value = null
      return
    }

    // Check if selection is within a code or markdown preview area
    const anchorNode = sel.anchorNode as HTMLElement
    const container = anchorNode?.closest?.('.raw-content-pre, .markdown-body')
    if (!container) {
      barVisible.value = false
      return
    }

    const text = sel.toString().trim()
    if (!text) {
      barVisible.value = false
      return
    }

    const { filePath, language } = getFileInfo(container)
    const { startLine, endLine } = getLineInfo(sel)

    selectionText.value = text.length > 80 ? text.slice(0, 80) + '...' : text
    quoteData.value = { text, filePath, language, startLine, endLine }
    barVisible.value = true
  }, 150)
}

// Global listener management
let listenerCount = 0

export function useQuoteQuestion(): QuoteQuestionState {
  const sendQuoteMessage = inject<(message: string, sessionId?: string) => Promise<void>>('sendQuoteMessage', null)
  const toast = inject<any>('toast', null)

  onMounted(() => {
    listenerCount++
    if (listenerCount === 1) {
      document.addEventListener('selectionchange', onSelectionChange)
    }
  })

  onUnmounted(() => {
    listenerCount--
    if (listenerCount === 0) {
      document.removeEventListener('selectionchange', onSelectionChange)
    }
  })

  function openSheet() {
    sheetOpen.value = true
  }

  function closeSheet() {
    sheetOpen.value = false
    // Clear selection when closing
    const sel = window.getSelection()
    if (sel) sel.removeAllRanges()
    barVisible.value = false
    quoteData.value = null
  }

  async function sendMessage(userMessage: string) {
    if (!quoteData.value || !userMessage.trim()) return

    const q = quoteData.value
    let langPrefix = q.language ? `${q.language}:` : ':'
    let lineSuffix = ''
    if (q.startLine && q.endLine && q.startLine !== q.endLine) {
      lineSuffix = `:${q.startLine}-${q.endLine}`
    } else if (q.startLine) {
      lineSuffix = `:${q.startLine}`
    }

    const message = `\`\`\`${langPrefix}${q.filePath}${lineSuffix}\n${q.text}\n\`\`\`\n\n${userMessage.trim()}`

    if (sendQuoteMessage) {
      await sendQuoteMessage(message)
    } else {
      // Fallback: direct API call
      try {
        const resp = await fetch('/api/ai/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ message }),
        })
        if (!resp.ok) throw new Error('发送失败')
        const data = await resp.json()
        if (toast) toast.show('已发送到会话', { icon: '✅', type: 'success', duration: 2000 })
      } catch (err) {
        if (toast) toast.show('发送失败', { icon: '⚠️', type: 'error' })
      }
    }

    closeSheet()
  }

  return {
    visible: barVisible,
    quoteData,
    sheetOpen,
    openSheet,
    closeSheet,
    sendMessage,
  }
}
```

**Step 2: Commit**

```bash
git add web/src/composables/useQuoteQuestion.ts
git commit -m "feat: add useQuoteQuestion composable for selection-based quoting"
```

---

### Task 3: Create QuoteQuestionBar component

**Files:**
- Create: `web/src/components/common/QuoteQuestionBar.vue`

This is the floating bar that appears above the dock when text is selected.

**Step 1: Create the component**

```vue
<!-- web/src/components/common/QuoteQuestionBar.vue -->
<template>
  <Transition name="quote-bar">
    <div v-if="visible && quoteData" class="quote-question-bar">
      <div class="quote-bar-preview">
        <span class="quote-bar-icon">💬</span>
        <span class="quote-bar-text">{{ quoteData.text.length > 60 ? quoteData.text.slice(0, 60) + '…' : quoteData.text }}</span>
      </div>
      <button class="quote-bar-btn" @click="$emit('open')">
        引用提问
      </button>
    </div>
  </Transition>
</template>

<script setup>
defineProps({
  visible: Boolean,
  quoteData: Object,
})
defineEmits(['open'])
</script>

<style scoped>
.quote-question-bar {
  position: fixed;
  bottom: calc(56px + env(safe-area-inset-bottom, 0px)); /* dock height + safe area */
  left: 8px;
  right: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  box-shadow: var(--shadow-md);
  z-index: 2400;
  max-width: 400px;
  margin: 0 auto;
}

.quote-bar-preview {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.quote-bar-icon {
  flex-shrink: 0;
  font-size: 14px;
}

.quote-bar-text {
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.quote-bar-btn {
  flex-shrink: 0;
  padding: 6px 14px;
  border: none;
  border-radius: 8px;
  background: var(--accent-color);
  color: #fff;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.15s;
}

.quote-bar-btn:active {
  opacity: 0.8;
}

.quote-bar-enter-active {
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.quote-bar-leave-active {
  transition: all 0.15s ease-in;
}

.quote-bar-enter-from,
.quote-bar-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/components/common/QuoteQuestionBar.vue
git commit -m "feat: add QuoteQuestionBar floating component"
```

---

### Task 4: Create QuoteQuestionSheet component

**Files:**
- Create: `web/src/components/common/QuoteQuestionSheet.vue`

This is the BottomSheet that shows: session selector, quote preview, and input.

**Step 1: Create the component**

```vue
<!-- web/src/components/common/QuoteQuestionSheet.vue -->
<template>
  <BottomSheet
    :open="open"
    title="引用提问"
    compact
    @close="$emit('close')"
  >
    <!-- Session selector -->
    <div class="qq-session" @click="showSessionPicker = true">
      <span class="qq-session-icon">{{ sessionIcon }}</span>
      <span class="qq-session-name">{{ sessionName }}</span>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
        <polyline points="6 9 12 15 18 9"/>
      </svg>
    </div>

    <!-- Quote preview -->
    <div v-if="quoteData" class="qq-quote-preview">
      <div class="qq-quote-label">引用内容</div>
      <pre class="qq-quote-code"><code>{{ formatQuote() }}</code></pre>
    </div>

    <!-- Input -->
    <div class="qq-input-area">
      <textarea
        ref="inputRef"
        v-model="inputText"
        class="qq-input"
        rows="3"
        placeholder="输入你的问题..."
        @keydown.enter.meta="handleSend"
        @keydown.enter.ctrl="handleSend"
      />
      <button class="qq-send-btn" :disabled="!canSend" @click="handleSend">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18">
          <line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>
        </svg>
      </button>
    </div>

    <template #footer>
      <div class="qq-footer-safe" />
    </template>
  </BottomSheet>

  <!-- Session picker overlay -->
  <Teleport to="body">
    <div v-if="showSessionPicker" class="qq-picker-overlay" @click="showSessionPicker = false">
      <div class="qq-picker" @click.stop>
        <div class="qq-picker-header">选择会话</div>
        <div class="qq-picker-list">
          <div v-if="loadingSessions" class="qq-picker-empty">加载中...</div>
          <div v-else-if="sessions.length === 0" class="qq-picker-empty">暂无会话</div>
          <div
            v-for="s in sessions"
            :key="s.id"
            class="qq-picker-item"
            :class="{ active: s.id === selectedSessionId }"
            @click="pickSession(s.id)"
          >
            <span class="qq-picker-item-title">{{ s.title || '新会话' }}</span>
            <span class="qq-picker-item-time">{{ formatTime(s.updatedAt) }}</span>
          </div>
        </div>
        <div class="qq-picker-footer">
          <button class="qq-picker-create" @click="createAndPick">+ 新会话</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import BottomSheet from '@/components/common/BottomSheet.vue'

const props = defineProps({
  open: Boolean,
  quoteData: Object,
  sessionIcon: { type: String, default: '🤖' },
  sessionName: { type: String, default: 'AI 对话' },
  currentSessionId: { type: String, default: '' },
})
const emit = defineEmits(['close', 'send'])

const inputText = ref('')
const inputRef = ref(null)
const showSessionPicker = ref(false)
const sessions = ref([])
const loadingSessions = ref(false)
const selectedSessionId = ref('')

const canSend = computed(() => inputText.value.trim().length > 0)

// Sync selected session with prop
watch(() => props.currentSessionId, (id) => {
  if (!selectedSessionId.value) selectedSessionId.value = id
}, { immediate: true })

// Load sessions when picker opens
watch(showSessionPicker, async (val) => {
  if (val) {
    loadingSessions.value = true
    try {
      const resp = await fetch('/api/ai/sessions')
      const data = await resp.json()
      sessions.value = data.sessions || []
    } catch (err) {
      sessions.value = []
    } finally {
      loadingSessions.value = false
    }
  }
})

// Focus input when sheet opens
watch(() => props.open, async (val) => {
  if (val) {
    selectedSessionId.value = props.currentSessionId
    await nextTick()
    inputRef.value?.focus()
  } else {
    inputText.value = ''
    showSessionPicker.value = false
  }
})

function formatQuote() {
  if (!props.quoteData) return ''
  const q = props.quoteData
  let langPrefix = q.language ? `${q.language}:` : ':'
  let lineSuffix = ''
  if (q.startLine && q.endLine && q.startLine !== q.endLine) {
    lineSuffix = `:${q.startLine}-${q.endLine}`
  } else if (q.startLine) {
    lineSuffix = `:${q.startLine}`
  }
  return `\`\`\`${langPrefix}${q.filePath}${lineSuffix}\n${q.text}\n\`\`\``
}

async function createAndPick() {
  try {
    const resp = await fetch('/api/ai/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    })
    const data = await resp.json()
    if (data.ok && data.sessionId) {
      selectedSessionId.value = data.sessionId
      showSessionPicker.value = false
      emit('send', inputText.value, data.sessionId)
    }
  } catch (err) {
    console.error('Failed to create session:', err)
  }
}

function pickSession(sessionId) {
  selectedSessionId.value = sessionId
  showSessionPicker.value = false
}

function handleSend() {
  if (!canSend.value) return
  emit('send', inputText.value, selectedSessionId.value || undefined)
}

function formatTime(date) {
  if (!date) return ''
  const d = new Date(date)
  const now = new Date()
  const diff = now - d
  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(diff / 86400000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  if (hours < 24) return `${hours}小时前`
  if (days < 7) return `${days}天前`
  return d.toLocaleDateString('zh-CN')
}
</script>

<style scoped>
.qq-session {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  margin: 4px 0;
  background: var(--bg-tertiary);
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.15s;
}

.qq-session:active {
  background: var(--bg-secondary);
}

.qq-session-icon {
  font-size: 14px;
}

.qq-session-name {
  flex: 1;
  font-size: 13px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.qq-quote-preview {
  margin: 8px 0;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid var(--border-color);
}

.qq-quote-label {
  font-size: 11px;
  color: var(--text-muted);
  padding: 4px 10px;
  background: var(--bg-tertiary);
}

.qq-quote-code {
  margin: 0;
  padding: 8px 10px;
  background: var(--code-bg);
  overflow-x: auto;
  font-size: 12px;
  line-height: 1.5;
  font-family: 'SF Mono', Monaco, Consolas, monospace;
}

.qq-quote-code code {
  white-space: pre-wrap;
  word-break: break-all;
}

.qq-input-area {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  padding: 8px 12px 0;
}

.qq-input {
  flex: 1;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
  resize: none;
  min-height: 72px;
  outline: none;
  font-family: inherit;
}

.qq-input:focus {
  border-color: var(--accent-color);
}

.qq-send-btn {
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  background: var(--accent-color);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  flex-shrink: 0;
  transition: opacity 0.15s;
}

.qq-send-btn:active {
  opacity: 0.8;
}

.qq-send-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

.qq-footer-safe {
  height: env(safe-area-inset-bottom, 0px);
}

/* Session picker */
.qq-picker-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 3000;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.qq-picker {
  width: 100%;
  max-width: 400px;
  max-height: 60vh;
  background: var(--bg-primary);
  border-radius: 16px 16px 0 0;
  display: flex;
  flex-direction: column;
  animation: qq-picker-up 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes qq-picker-up {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}

.qq-picker-header {
  padding: 14px 16px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-color);
}

.qq-picker-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.qq-picker-empty {
  padding: 24px;
  text-align: center;
  color: var(--text-muted);
  font-size: 14px;
}

.qq-picker-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.15s;
}

.qq-picker-item:active {
  background: var(--bg-tertiary);
}

.qq-picker-item.active {
  background: var(--accent-bg, rgba(0, 102, 204, 0.1));
}

.qq-picker-item-title {
  flex: 1;
  font-size: 14px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.qq-picker-item.active .qq-picker-item-title {
  color: var(--accent-color);
  font-weight: 500;
}

.qq-picker-item-time {
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
  margin-left: 8px;
}

.qq-picker-footer {
  padding: 10px 12px;
  border-top: 1px solid var(--border-color);
}

.qq-picker-create {
  width: 100%;
  padding: 10px;
  border: none;
  border-radius: 8px;
  background: var(--accent-color);
  color: #fff;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
}

.qq-picker-create:active {
  opacity: 0.85;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/components/common/QuoteQuestionSheet.vue
git commit -m "feat: add QuoteQuestionSheet component with session selector"
```

---

### Task 5: Wire up provide/inject for cross-component messaging

**Files:**
- Modify: `web/src/components/chat/ChatPanel.vue`
- Modify: `web/src/App.vue`

ChatPanel needs to `provide('sendQuoteMessage', ...)` so that components outside the chat panel (like QuoteQuestionBar/Sheet) can send messages to the current session.

**Step 1: Add provide in ChatPanel.vue**

Add after the existing `provide()` calls (around line 243):

```typescript
// Provide a function for external components (QuoteQuestion) to send messages
// to the current session without opening the chat panel
provide('sendQuoteMessage', async (message: string, sessionId?: string) => {
  const targetSessionId = sessionId || session.currentSessionId.value

  // If no session exists, create one first
  if (!targetSessionId) {
    await session.createSession()
  }

  const sid = sessionId || session.currentSessionId.value
  const effectiveAgentId = session.currentAgentId.value

  try {
    const url = sid
      ? `/api/ai/chat?session_id=${encodeURIComponent(sid)}`
      : '/api/ai/chat'
    const resp = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        message,
        filePaths: [],
        files: [],
        agentId: effectiveAgentId,
      }),
    })
    const data = await resp.json()
    if (!resp.ok) throw new Error(data.error || '发送失败')

    // If this was sent to the current session, the stream needs to be connected
    if (sid === session.currentSessionId.value) {
      stream.connectStream(sid)
    }

    toast.show('已发送到会话', { icon: '✅', type: 'success', duration: 2000 })
  } catch (err) {
    toast.show('发送失败: ' + err.message, { icon: '⚠️', type: 'error' })
  }
})

// Also provide session info for the QuoteQuestionSheet
provide('chatSessionInfo', {
  currentSessionId: session.currentSessionId,
  currentSessionTitle: session.currentSessionTitle,
  currentAgentId: session.currentAgentId,
  agentHeaderTitle: session.agentHeaderTitle,
  getAgentIcon: session.getAgentIcon,
  getAgentName: session.getAgentName,
})
```

**Step 2: Add QuoteQuestion components to App.vue**

Import and add the QuoteQuestionBar and QuoteQuestionSheet in App.vue, placed above the dock:

```vue
<!-- In template, before bottom-dock-wrapper -->
<QuoteQuestionBar
  :visible="quoteQuestion.visible.value"
  :quoteData="quoteQuestion.quoteData.value"
  @open="quoteQuestion.openSheet()"
/>
<QuoteQuestionSheet
  :open="quoteQuestion.sheetOpen.value"
  :quoteData="quoteQuestion.quoteData.value"
  :sessionIcon="chatSessionInfo?.getAgentIcon?.(chatSessionInfo?.currentAgentId?.value) || '🤖'"
  :sessionName="chatSessionInfo?.agentHeaderTitle?.value || 'AI 对话'"
  :currentSessionId="chatSessionInfo?.currentSessionId?.value || ''"
  @close="quoteQuestion.closeSheet()"
  @send="quoteQuestion.sendMessage"
/>
```

Add imports and composable usage:

```typescript
import QuoteQuestionBar from './components/common/QuoteQuestionBar.vue'
import QuoteQuestionSheet from './components/common/QuoteQuestionSheet.vue'
import { useQuoteQuestion } from './composables/useQuoteQuestion.ts'

// In script:
const quoteQuestion = useQuoteQuestion()
const chatSessionInfo = inject('chatSessionInfo', null)
```

**Step 3: Add data attributes to CodePreview and MarkdownPreview**

So the composable can detect which preview area the selection is in, add `data-file-path` and `data-language` attributes:

In CodePreview.vue template:
```vue
<pre class="raw-content-pre" ref="codeRef" :data-file-path="filePath" :data-language="language">
```

In MarkdownPreview.vue template:
```vue
<div v-if="viewMode === 'rendered'" class="markdown-body" ref="bodyRef" :data-file-path="file?.path || ''" v-html="renderedHtml" @click="handleClick" />
```

**Step 4: Verify build**

```bash
npm run build 2>&1 | tail -20
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: wire up QuoteQuestion with provide/inject and data attributes"
```

---

### Task 6: Integration test and polish

**Files:**
- Modify: various files for bug fixes and polish

**Step 1: Manual testing checklist**

Start dev server and test:

```bash
cd /home/xulongzhe/projects/clawbench/.worktrees/quote-question
./dev-server.sh
```

Test cases:
1. ✅ Code preview: long-press no longer shows context menu
2. ✅ Code preview: text is selectable (no -webkit-user-select: none)
3. ✅ Code preview: select text → QuoteQuestionBar appears above dock
4. ✅ Code preview: click "引用提问" → QuoteQuestionSheet opens with quote preview
5. ✅ QuoteQuestionSheet: shows current session info
6. ✅ QuoteQuestionSheet: click session → picker opens, can switch
7. ✅ QuoteQuestionSheet: type message and send → toast shows "已发送"
8. ✅ Message format in chat: code block with language:filePath:lines + user message
9. ✅ Markdown preview (rendered mode): select text → bar appears
10. ✅ Markdown preview (raw mode): select text → bar appears
11. ✅ Selection cleared after closing sheet
12. ✅ Bar hides when selection is collapsed/empty
13. ✅ Bar hides when clicking outside preview area

**Step 2: Fix any issues found during testing**

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat: quote question integration - test and polish"
```

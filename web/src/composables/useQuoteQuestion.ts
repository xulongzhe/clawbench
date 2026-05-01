import { ref, onMounted, onUnmounted, inject, type Ref } from 'vue'

export interface QuoteData {
  text: string           // selected text
  filePath: string       // file path
  language: string       // language identifier (empty for markdown preview)
  startLine: number      // start line number (1-based, 0 if unknown)
  endLine: number        // end line number (1-based, 0 if unknown)
}

// Module-level singleton: selection state shared across all consumers
const quoteData = ref<QuoteData | null>(null)
const barVisible = ref(false)
const sheetOpen = ref(false)

let debounceTimer: ReturnType<typeof setTimeout> | null = null

/**
 * Helper: get the closest Element matching a selector from a node.
 * The node may be a Text node, so we use parentElement first.
 */
function closestElement(node: Node | null, selector: string): HTMLElement | null {
  if (!node) return null
  const el = (node instanceof HTMLElement ? node : node.parentElement)
  return el?.closest?.(selector) ?? null
}

/**
 * Get line numbers from a selection range inside a code preview.
 * Walks up from anchor/focus nodes to find .code-line[data-line] elements.
 */
function getLineInfo(selection: Selection): { startLine: number; endLine: number } {
  const anchor = closestElement(selection.anchorNode, '.code-line')
  const focus = closestElement(selection.focusNode, '.code-line')
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
      quoteData.value = null
      return
    }

    // Check if selection is within a code or markdown preview area
    const container = closestElement(sel.anchorNode, '.raw-content-pre, .markdown-body')
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

    quoteData.value = { text, filePath, language, startLine, endLine }
    barVisible.value = true
  }, 150)
}

// Global listener management
let listenerCount = 0

export function useQuoteQuestion() {
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

  function closeSheet() {
    // Clear selection when closing
    const sel = window.getSelection()
    if (sel) sel.removeAllRanges()
    barVisible.value = false
    quoteData.value = null
  }

  async function sendMessage(userMessage: string, sessionId?: string) {
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

    // Direct API call — always works regardless of provide/inject hierarchy
    try {
      // If no session, create one first
      let sid = sessionId
      if (!sid) {
        const createResp = await fetch('/api/ai/sessions', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
        })
        const createData = await createResp.json()
        if (createData.ok && createData.sessionId) {
          sid = createData.sessionId
        }
      }

      const url = sid
        ? `/api/ai/chat?session_id=${encodeURIComponent(sid)}`
        : '/api/ai/chat'
      const resp = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message }),
      })
      if (!resp.ok) {
        const errData = await resp.json().catch(() => ({}))
        throw new Error(errData.error || '发送失败')
      }
      if (toast) toast.show('已发送到会话', { icon: '✅', type: 'success', duration: 2000 })
    } catch (err) {
      if (toast) toast.show('发送失败: ' + (err as Error).message, { icon: '⚠️', type: 'error' })
    }

    closeSheet()
  }

  return {
    visible: barVisible,
    quoteData,
    sheetOpen,
    openSheet: () => { sheetOpen.value = true },
    closeSheet,
    sendMessage,
  }
}

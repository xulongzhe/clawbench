import { ref, onMounted, onUnmounted } from 'vue'
import { useSessionIdentity } from '@/composables/useSessionIdentity.ts'
import { useToast } from '@/composables/useToast.ts'

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
const barPinned = ref(false)  // When pinned, selection loss won't auto-hide the bar
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
      // When bar is pinned (user clicked "引用提问"), don't auto-hide on selection loss
      if (!barPinned.value) {
        barVisible.value = false
        quoteData.value = null
      }
      return
    }

    // Check if selection is within a code or markdown preview area
    const container = closestElement(sel.anchorNode, '.raw-content-pre, .markdown-body')
    if (!container) {
      if (!barPinned.value) {
        barVisible.value = false
      }
      return
    }

    const text = sel.toString().trim()
    if (!text) {
      if (!barPinned.value) {
        barVisible.value = false
      }
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
  const toast = useToast()
  const sessionIdentity = useSessionIdentity()

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
    barPinned.value = false
    quoteData.value = null
  }

  function pinBar() {
    // Pin the bar so it survives selection loss (e.g. after clicking a button)
    barPinned.value = true
  }

  function unpinBar() {
    barPinned.value = false
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

    const message = `${userMessage.trim()}\n\n\`\`\`${langPrefix}${q.filePath}${lineSuffix}\n${q.text}\n\`\`\``

    // Pass the quoted file as a file attachment so the backend builds
    // the [当前文件: ...] prompt prefix and sets the CLI work_dir.
    const filePaths = q.filePath ? [q.filePath] : []

    // Delegate to session identity singleton — it routes to ChatPanel's
    // sendMessage if registered, otherwise falls back to a direct API call.
    try {
      await sessionIdentity.sendMessage(message, filePaths)
      toast.show('已发送到会话', { icon: '✅', type: 'success', duration: 2000 })
    } catch (err) {
      toast.show('发送失败: ' + (err as Error).message, { icon: '⚠️', type: 'error' })
    }

    // Don't close the bar — keep the preview visible for follow-up questions.
    // Just unpin so selection changes can auto-hide the bar again.
    unpinBar()
  }

  return {
    visible: barVisible,
    quoteData,
    sheetOpen,
    openSheet: () => { sheetOpen.value = true },
    closeSheet,
    pinBar,
    unpinBar,
    sendMessage,
  }
}

/**
 * useFileRefresh — shared logic for refreshing the currently viewed file
 * while preserving scroll position and flash-highlighting changed characters.
 *
 * Used by three independent refresh triggers:
 * 1. Manual refresh (refresh button in FileHeader / FileManager)
 * 2. fsnotify auto-refresh (useFileWatch SSE file_change event)
 * 3. Chat-driven refresh (ChatPanel onFileModified callback)
 *
 * Two-phase refresh when deletions are detected:
 *   Phase 1: Red-flash deleted characters in old content → wait ~1.2s
 *   Phase 2: Update store with new content → blue-flash added characters
 *
 * Two flash mechanisms:
 * - flashRanges (line+char offset): Used by CodePreview for code/raw files
 * - flashTextSnippets (text strings): Used by MarkdownPreview for rendered HTML,
 *   where line/offset mapping is lost after markdown→HTML transformation
 */
import { ref, watch } from 'vue'
import { store } from '@/stores/app.ts'
import { renderMarkdown } from '@/composables/useMarkdownRenderer.ts'
import { computeDiff, wholeLineRanges, charMapToRanges } from '@/utils/diffUtils.ts'
import type { LineDiff } from '@/utils/diffUtils.ts'

// ─── Flash state (consumed by CodePreview & MarkdownPreview) ───

export type FlashType = 'delete' | 'add'

/** A range of characters to highlight within a single line (0-based, inclusive start, exclusive end) */
export interface FlashRange {
    line: number   // 1-based line number
    start: number   // 0-based char offset within the line (string offset, not char index)
    end: number     // 0-based char offset (exclusive; use Infinity for "rest of line")
}

/**
 * Reactive flash ranges — CodePreview reads this to wrap characters
 * in <span class="char-flash-{type}"> during rendering.
 *
 * IMPORTANT: Must always be reassigned (not mutated in-place) for Vue
 * reactivity to trigger the watch in CodePreview.
 */
export const flashRanges = ref<FlashRange[]>([])

/**
 * Text snippets that changed — MarkdownPreview reads this to search
 * for matching text in the rendered DOM and wrap it in flash spans.
 * Used for rendered markdown where line/offset mapping is unavailable.
 */
export const flashTextSnippets = ref<string[]>([])
export const flashType = ref<FlashType>('add')
let flashTimer: ReturnType<typeof setTimeout> | null = null

// Generation counter to prevent race conditions with concurrent refreshCurrentFile calls
let refreshGeneration = 0

function clearFlash() {
    if (flashTimer) { clearTimeout(flashTimer); flashTimer = null }
    flashRanges.value = []
    flashTextSnippets.value = []
    flashType.value = 'add'
}

function scheduleClearFlash(ms: number) {
    if (flashTimer) clearTimeout(flashTimer)
    flashTimer = setTimeout(() => { flashRanges.value = []; flashTextSnippets.value = []; flashType.value = 'add'; flashTimer = null }, ms)
}

// ─── Extract text snippets from diff result (for Markdown rendered mode) ───

/** DOMParser singleton (reused across calls) */
const domParser = new DOMParser()

/**
 * Convert markdown source text to plain text that matches the rendered DOM.
 * Uses the same renderMarkdown pipeline as MarkdownPreview, then extracts
 * textContent — so headings, bold, code blocks, lists etc. are all handled
 * correctly without any fragile regex stripping.
 */
function markdownToPlainText(mdText) {
    if (!mdText) return ''
    const html = renderMarkdown(mdText, { sanitize: false })
    const doc = domParser.parseFromString(html, 'text/html')
    return doc.body.textContent || ''
}

/**
 * Extract changed text snippets from the diff result and source text.
 * Uses renderMarkdown + DOMParser to get rendered plain text, so snippets
 * match what appears in the rendered DOM (not raw markdown source).
 * Filters out very short snippets (1-2 chars) to reduce false positives.
 */
function extractSnippets(text, diff, mode) {
    // Convert the full source text to rendered plain text, line by line.
    // We render each line individually so we can map line numbers.
    const lines = text.split('\n')
    const renderedLines = lines.map(line => markdownToPlainText(line))

    const snippets = []
    const MIN_SNIPPET_LEN = 3

    const collect = (lineNum, charStart, charEnd) => {
        // Get the rendered plain text for this line
        const renderedLine = renderedLines[lineNum - 1]
        if (!renderedLine) return

        // For char-level ranges, extract the corresponding portion from rendered text.
        // The char offsets are in the raw markdown source, but after rendering
        // the offsets shift. For whole-line changes we use the full rendered line.
        // For char-level changes, we take the full rendered line (the char offsets
        // from the raw source don't map 1:1 to rendered text).
        const snippet = (charStart === 0 && charEnd === Infinity)
            ? renderedLine.trim()
            : renderedLine.trim()

        if (snippet.length >= MIN_SNIPPET_LEN) {
            snippets.push(snippet)
            // Also add individual words as fallback for cross-element matching
            const words = snippet.split(/[\s,;:]+/).filter(w => w.length >= MIN_SNIPPET_LEN)
            for (const w of words) {
                if (w !== snippet) snippets.push(w)
            }
        }
    }

    if (mode === 'delete') {
        for (const lineNum of diff.deletedInOld) {
            collect(lineNum, 0, Infinity)
        }
        for (const [lineNum, ranges] of diff.deletedChars) {
            for (const { start, end } of ranges) {
                collect(lineNum, start, end)
            }
        }
    } else {
        for (const lineNum of diff.addedInNew) {
            collect(lineNum, 0, Infinity)
        }
        for (const [lineNum, ranges] of diff.addedChars) {
            for (const { start, end } of ranges) {
                collect(lineNum, start, end)
            }
        }
    }

    // Deduplicate and limit count
    return [...new Set(snippets)].slice(0, 30)
}

// ─── Scroll helpers ───

function getScrollContainer(): HTMLElement | null {
  return (document.querySelector('.markdown-body') || document.querySelector('.raw-content-pre')) as HTMLElement | null
}

function getScrollRatio(el: HTMLElement | null): number {
  if (!el) return 0
  const maxScroll = el.scrollHeight - el.clientHeight
  if (maxScroll <= 0) return 0
  return el.scrollTop / maxScroll
}

function restoreScrollRatio(ratio: number): void {
  if (ratio <= 0) return
  const startTime = Date.now()
  const MAX_WAIT = 3000

  function tryRestore() {
    const el = getScrollContainer()
    if (!el) {
      if (Date.now() - startTime < MAX_WAIT) requestAnimationFrame(tryRestore)
      return
    }
    const maxScroll = el.scrollHeight - el.clientHeight
    if (maxScroll <= 0) {
      if (Date.now() - startTime < MAX_WAIT) requestAnimationFrame(tryRestore)
      return
    }
    el.scrollTop = ratio * maxScroll
  }
  requestAnimationFrame(() => requestAnimationFrame(tryRestore))
}

// ─── Pre-fetch helper (does NOT update store) ───

async function prefetchFileContent(path: string): Promise<string | null> {
    try {
        const resp = await fetch(`/api/file/${encodeURIComponent(path)}`)
        if (!resp.ok) return null
        const data = await resp.json()
        // Don't try to diff binary or too-large files
        if (data.isBinary || data.tooLarge || data.error) return null
        return data.content ?? null
    } catch {
        return null
    }
}

// ─── Clear flash on file navigation ───
// When the user navigates to a different file (not via refreshCurrentFile),
// stale flash ranges would show on the new file. Watch for path changes.

watch(() => store.state.currentFile?.path, (newPath, oldPath) => {
    if (newPath !== oldPath) clearFlash()
})

// ─── Main refresh function ───

const DELETE_FLASH_MS = 1200
const ADD_FLASH_CLEAR_MS = 2000

/**
 * Refresh the currently viewed file content while preserving scroll position.
 * When changes are detected, flash-highlights deleted chars (red) first,
 * then updates the content and flash-highlights added chars (blue).
 *
 * @param options.loadDir - Also refresh the directory listing (default: false)
 * @param options.clearOnError - If the file fails to load, clear currentFile (default: false)
 */
export async function refreshCurrentFile(options: {
  loadDir?: boolean
  clearOnError?: boolean
} = {}): Promise<void> {
  const { loadDir = false, clearOnError = false } = options
  const gen = ++refreshGeneration

  const currentFilePath = store.state.currentFile?.path
  const currentFile = store.state.currentFile

  // Save old content for change detection
  const oldContent = currentFile?.content ?? null
  const oldPath = currentFilePath

  // Save scroll position as ratio before refresh
  const scrollEl = getScrollContainer()
  const scrollRatio = getScrollRatio(scrollEl)

  // Refresh directory listing if requested
  if (loadDir && store.state.currentDir !== undefined) {
    store.loadFiles(store.state.currentDir)
  }

  if (!currentFilePath) return

  // ─── Phase 0: Pre-fetch new content for diff ───
  let newContent: string | null = null
  let hasDeletions = false
  let diffResult: LineDiff | null = null

  if (oldContent) {
      newContent = await prefetchFileContent(currentFilePath)
      // Abort if a newer refresh has started
      if (gen !== refreshGeneration) return
      if (newContent !== null && newContent !== oldContent) {
          diffResult = computeDiff(oldContent, newContent)
          hasDeletions = diffResult.deletedInOld.length > 0 || diffResult.deletedChars.size > 0
      }
  }

  // ─── Phase 1: Red-flash deletions (if any) ───
  if (hasDeletions && diffResult) {
      // For CodePreview (code files): line+offset ranges
      const delRanges: FlashRange[] = [
          ...wholeLineRanges(diffResult.deletedInOld),
          ...charMapToRanges(diffResult.deletedChars),
      ]
      // For MarkdownPreview (rendered mode): text snippets to search in DOM
      const delSnippets = extractSnippets(oldContent, diffResult, 'delete')

      flashRanges.value = delRanges
      flashTextSnippets.value = delSnippets
      flashType.value = 'delete'

      // Wait for user to see the red flash
      await new Promise<void>(resolve => setTimeout(resolve, DELETE_FLASH_MS))

      // Abort if a newer refresh started or user navigated away
      if (gen !== refreshGeneration || store.state.currentFile?.path !== oldPath) {
          clearFlash()
          return
      }
  }

  // ─── Phase 2: Update store with new content ───
  await store.selectFile(
    currentFilePath,
    currentFile?.isImage,
    currentFile?.isAudio,
    false, // addToHistory=false — this is a refresh, not navigation
  )

  // Abort if a newer refresh started
  if (gen !== refreshGeneration) return

  // Clear file on error if requested
  if (clearOnError && store.state.currentFile?.error) {
    store.state.currentFile = null
    clearFlash()
    return
  }

  // ─── Phase 3: Blue-flash additions ───
  if (diffResult) {
      const addRanges: FlashRange[] = [
          ...wholeLineRanges(diffResult.addedInNew),
          ...charMapToRanges(diffResult.addedChars),
      ]
      const addSnippets = newContent
          ? extractSnippets(newContent, diffResult, 'add')
          : []

      if (addRanges.length > 0 || addSnippets.length > 0) {
          flashRanges.value = addRanges
          flashTextSnippets.value = addSnippets
          flashType.value = 'add'
          scheduleClearFlash(ADD_FLASH_CLEAR_MS)
      } else {
          clearFlash()
      }
  } else {
      clearFlash()
  }

  // Restore scroll position
  restoreScrollRatio(scrollRatio)
}

export { getScrollContainer, getScrollRatio, restoreScrollRatio }

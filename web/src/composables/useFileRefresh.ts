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

// ─── LCS diff algorithms ───

interface LineDiff {
    /** Lines deleted in old file (1-based line number in old text) */
    deletedInOld: number[]
    /** Lines added in new file (1-based line number in new text) */
    addedInNew: number[]
    /** For modified lines: old line → char-level ranges deleted */
    deletedChars: Map<number, { start: number; end: number }[]>
    /** For modified lines: new line → char-level ranges added */
    addedChars: Map<number, { start: number; end: number }[]>
}

/**
 * Full diff: line-level LCS + char-level LCS for changed lines.
 */
function computeDiff(oldText: string, newText: string): LineDiff {
    const oldLines = oldText.split('\n')
    const newLines = newText.split('\n')

    const result: LineDiff = {
        deletedInOld: [],
        addedInNew: [],
        deletedChars: new Map(),
        addedChars: new Map(),
    }

    // For large files, skip char-level diff (performance)
    if (oldLines.length > 500 || newLines.length > 500) {
        simpleLineDiff(oldLines, newLines, result)
        return result
    }

    lcsLineDiff(oldLines, newLines, result)
    return result
}

function simpleLineDiff(oldLines: string[], newLines: string[], result: LineDiff) {
    const maxLen = Math.max(oldLines.length, newLines.length)
    for (let i = 0; i < maxLen; i++) {
        if (i >= oldLines.length) {
            // Pure addition
            result.addedInNew.push(i + 1)
        } else if (i >= newLines.length) {
            // Pure deletion
            result.deletedInOld.push(i + 1)
        } else if (oldLines[i] !== newLines[i]) {
            // Modified line — do char-level diff
            charDiff(oldLines[i], newLines[i], i + 1, i + 1, result)
        }
    }
}

function lcsLineDiff(oldLines: string[], newLines: string[], result: LineDiff) {
    const m = oldLines.length
    const n = newLines.length

    // Build LCS table
    const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0))
    for (let i = 1; i <= m; i++) {
        for (let j = 1; j <= n; j++) {
            if (oldLines[i - 1] === newLines[j - 1]) {
                dp[i][j] = dp[i - 1][j - 1] + 1
            } else {
                dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1])
            }
        }
    }

    // Backtrack to classify lines
    const oldMatched = new Set<number>() // old line indices (0-based) that are in LCS
    const newMatched = new Set<number>() // new line indices (0-based) that are in LCS
    let i = m, j = n
    while (i > 0 && j > 0) {
        if (oldLines[i - 1] === newLines[j - 1]) {
            oldMatched.add(i - 1)
            newMatched.add(j - 1)
            i--; j--
        } else if (dp[i - 1][j] >= dp[i][j - 1]) {
            i--
        } else {
            j--
        }
    }

    // Unmatched old lines = deleted
    for (let oi = 0; oi < m; oi++) {
        if (!oldMatched.has(oi)) {
            result.deletedInOld.push(oi + 1)
        }
    }

    // Unmatched new lines = added
    for (let nj = 0; nj < n; nj++) {
        if (!newMatched.has(nj)) {
            result.addedInNew.push(nj + 1)
        }
    }

    // For lines that are "replacements" (adjacent unmatched old→new pairs),
    // do char-level diff to show precise changes within the line.
    // Heuristic: pair by proximity (deleted line ↔ closest added line within 3 positions)
    const deletedSorted = result.deletedInOld.slice().sort((a, b) => a - b)
    const addedSorted = result.addedInNew.slice().sort((a, b) => a - b)
    const pairedOld = new Set<number>()
    const pairedNew = new Set<number>()

    for (const oi of deletedSorted) {
        // Find closest unpaired added line
        let bestJ = -1, bestDist = Infinity
        for (const nj of addedSorted) {
            if (pairedNew.has(nj)) continue
            const dist = Math.abs(oi - nj)
            if (dist < bestDist) { bestDist = dist; bestJ = nj }
        }
        if (bestJ >= 0 && bestDist <= 3) {
            pairedOld.add(oi)
            pairedNew.add(bestJ)
            charDiff(oldLines[oi - 1], newLines[bestJ - 1], oi, bestJ, result)
        }
    }

    // Remove paired entries from the pure delete/add lists
    result.deletedInOld = result.deletedInOld.filter(l => !pairedOld.has(l))
    result.addedInNew = result.addedInNew.filter(l => !pairedNew.has(l))
}

/**
 * Char-level LCS diff for a single modified line.
 * Populates result.deletedChars and result.addedChars.
 */
function charDiff(oldLine: string, newLine: string, oldLineNum: number, newLineNum: number, result: LineDiff) {
    const a = [...oldLine] // split into chars (handles unicode)
    const b = [...newLine]
    const m = a.length
    const n = b.length

    // Skip char-level diff for very long lines
    if (m > 200 || n > 200) {
        result.deletedChars.set(oldLineNum, [{ start: 0, end: oldLine.length }])
        result.addedChars.set(newLineNum, [{ start: 0, end: newLine.length }])
        return
    }

    const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0))
    for (let i = 1; i <= m; i++) {
        for (let j = 1; j <= n; j++) {
            if (a[i - 1] === b[j - 1]) {
                dp[i][j] = dp[i - 1][j - 1] + 1
            } else {
                dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1])
            }
        }
    }

    // Backtrack to find deleted and added char ranges
    const deletedChars = new Set<number>() // 0-based indices in oldLine
    const addedChars = new Set<number>()   // 0-based indices in newLine
    let i = m, j = n
    while (i > 0 && j > 0) {
        if (a[i - 1] === b[j - 1]) {
            i--; j--
        } else if (dp[i - 1][j] >= dp[i][j - 1]) {
            deletedChars.add(i - 1)
            i--
        } else {
            addedChars.add(j - 1)
            j--
        }
    }
    while (i > 0) { deletedChars.add(i - 1); i-- }
    while (j > 0) { addedChars.add(j - 1); j-- }

    // Convert sets of char indices into contiguous ranges
    // using string offsets (not char indices) for proper HTML slicing
    result.deletedChars.set(oldLineNum, charIndicesToRanges(deletedChars, oldLine))
    result.addedChars.set(newLineNum, charIndicesToRanges(addedChars, newLine))
}

/**
 * Convert a set of character indices (0-based) into contiguous {start, end} ranges
 * using string offsets (for proper substring slicing with unicode).
 */
function charIndicesToRanges(indices: Set<number>, text: string): { start: number; end: number }[] {
    if (indices.size === 0) return []
    const sorted = [...indices].sort((a, b) => a - b)
    const chars = [...text]
    // Build char-index → string-offset map
    const offsets: number[] = []
    let offset = 0
    for (const ch of chars) {
        offsets.push(offset)
        offset += ch.length // ch might be multi-byte in JS string
    }
    offsets.push(offset) // end offset for last char + 1

    const ranges: { start: number; end: number }[] = []
    let rangeStart = sorted[0]
    for (let k = 1; k <= sorted.length; k++) {
        const isEnd = k === sorted.length || sorted[k] !== sorted[k - 1] + 1
        if (isEnd) {
            const rangeEnd = sorted[k - 1]
            ranges.push({ start: offsets[rangeStart], end: offsets[rangeEnd + 1] })
            if (k < sorted.length) rangeStart = sorted[k]
        }
    }
    return ranges
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

// ─── Convert line-level deletions to FlashRange (whole line) ───
// end=Infinity is clamped to rawLine.length in CodePreview's applyFlashToLine

function wholeLineRanges(lineNums: number[]): FlashRange[] {
    return lineNums.map(line => ({ line, start: 0, end: Infinity }))
}

// ─── Convert char-level diff maps to FlashRange arrays ───

function charMapToRanges(map: Map<number, { start: number; end: number }[]>): FlashRange[] {
    const ranges: FlashRange[] = []
    for (const [line, chars] of map) {
        for (const { start, end } of chars) {
            ranges.push({ line, start, end })
        }
    }
    return ranges
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

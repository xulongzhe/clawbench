/**
 * LCS diff algorithms for file content comparison.
 * Extracted from useFileRefresh for independent testability.
 *
 * Two-level diff:
 *   1. Line-level LCS to identify added/deleted lines
 *   2. Char-level LCS for modified line pairs (proximity-paired)
 */

export interface LineDiff {
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
export function computeDiff(oldText: string, newText: string): LineDiff {
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

export function simpleLineDiff(oldLines: string[], newLines: string[], result: LineDiff) {
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

export function lcsLineDiff(oldLines: string[], newLines: string[], result: LineDiff) {
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
export function charDiff(oldLine: string, newLine: string, oldLineNum: number, newLineNum: number, result: LineDiff) {
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
export function charIndicesToRanges(indices: Set<number>, text: string): { start: number; end: number }[] {
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

/**
 * Convert line-level deletions to whole-line FlashRange.
 * end=Infinity is clamped to rawLine.length in CodePreview's applyFlashToLine.
 */
export function wholeLineRanges(lineNums: number[]): { line: number; start: number; end: number }[] {
    return lineNums.map(line => ({ line, start: 0, end: Infinity }))
}

/**
 * Convert char-level diff maps to FlashRange arrays.
 */
export function charMapToRanges(map: Map<number, { start: number; end: number }[]>): { line: number; start: number; end: number }[] {
    const ranges: { line: number; start: number; end: number }[] = []
    for (const [line, chars] of map) {
        for (const { start, end } of chars) {
            ranges.push({ line, start, end })
        }
    }
    return ranges
}

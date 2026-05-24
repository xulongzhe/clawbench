import { escapeHtml } from '@/utils/html.ts'
import { gt } from '@/composables/useLocale'
import { resolveFilePath } from '@/composables/useFilePathAnnotation.ts'

/**
 * SVG icon markup for the worktree button (GitBranch icon from lucide).
 */
export const WORKTREE_ICON_SVG = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12"><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg>'

/**
 * Generate HTML for a single worktree action button.
 * Clicking it shows a modal with "Switch to worktree" or "Open directory" options.
 */
export function worktreeButtonHtml(absPath: string, resolvedPath?: string): string {
    let attrs = `class="chat-worktree-btn" data-worktree-path="${escapeHtml(absPath)}"`
    if (resolvedPath) {
        attrs += ` data-file-path="${escapeHtml(resolvedPath)}"`
    }
    return `<button ${attrs} title="${escapeHtml(gt('chat.attach.openWorktree'))}">${WORKTREE_ICON_SVG}</button>`
}

// ── Worktree list cache (for async verification) ──

interface WorktreeInfo {
    path: string
    displayPath: string
    branch: string
    isCurrent: boolean
    dirty: boolean
    changeCount: number
    untrackedCount: number
    locked: boolean
    missing: boolean
}

/** Cache: projectRoot → worktree list (from GET /api/git/worktrees) */
const worktreeListCache = new Map<string, WorktreeInfo[]>()

/**
 * Fetch the worktree list for a project and cache it.
 * Returns the cached list if available, otherwise fetches and caches.
 */
async function fetchWorktreeList(projectRoot: string): Promise<WorktreeInfo[]> {
    const cached = worktreeListCache.get(projectRoot)
    if (cached) return cached

    try {
        const resp = await fetch('/api/git/worktrees')
        if (!resp.ok) return []
        const data = await resp.json()
        const worktrees: WorktreeInfo[] = data.worktrees || []
        worktreeListCache.set(projectRoot, worktrees)
        return worktrees
    } catch {
        return []
    }
}

/**
 * Regex that matches worktree paths in plain text.
 * Matches:
 *   1. Absolute paths containing /.worktrees/ (e.g. /home/user/project/.worktrees/feature-x)
 *   2. Relative paths starting with .worktrees/ (e.g. .worktrees/feature-x)
 *   3. Relative paths starting with ./.worktrees/ (e.g. ./.worktrees/feature-x)
 *
 * This regex runs on text node content only (never on HTML attributes or tags),
 * same design as FILE_PATH_RE. It enables synchronous annotation without
 * waiting for the async worktree list API.
 */
const WORKTREE_PATH_RE = /(?:\/[^\s<>"')\]]+\/\.worktrees\/[^\s<>"')\]]+|\.\/\.worktrees\/[^\s<>"')\]]+|\.worktrees\/[^\s<>"')\]]+)/g

/**
 * Check if a string looks like a worktree path that should be annotated.
 * Used for <code> tag content detection (simpler than full regex).
 */
function looksLikeWorktreePath(text: string): boolean {
    return text.includes('.worktrees/')
}

/**
 * Detect worktree paths in rendered HTML and insert action buttons after them.
 *
 * Design: regex-first, verify-later — aligned with localhost/commit/file-path annotations.
 *   1. Synchronously annotate all text matching .worktrees/ patterns (no API call needed)
 *   2. Asynchronously verify via GET /api/git/worktrees, removing false positives
 *
 * This ensures annotations appear immediately on first render AND survive app restart
 * (no async cache dependency in the annotation step itself).
 *
 * Processing order:
 *   1. <a href> tags matching a worktree path → append action button
 *   2. <code> and <span class="chat-file-path"> tags matching → add class + button
 *   3. Text nodes (outside pre/a/code) → regex match → insert span + button
 *
 * Returns the annotated HTML and a list of detected worktree path strings.
 */
export function annotateWorktreePaths(
    html: string,
    { projectRoot }: { projectRoot: string },
): { html: string; detectedWorktreePaths: string[] } {
    if (!html) return { html: '', detectedWorktreePaths: [] }

    const detectedWorktreePaths: string[] = []
    const doc = new DOMParser().parseFromString(html, 'text/html')

    // ── Step 1: <a href> tags matching a worktree path ──
    for (const a of doc.querySelectorAll('a[href]')) {
        const href = a.getAttribute('href') || ''
        if (!looksLikeWorktreePath(href)) continue
        const match = extractWorktreePath(href, projectRoot)
        if (!match) continue
        detectedWorktreePaths.push(match)
        a.insertAdjacentHTML('afterend', worktreeButtonHtml(match, resolveFilePath(match, projectRoot) || undefined))
    }

    // ── Step 2: <code> and <span class="chat-file-path"> matching a worktree ──
    for (const el of doc.querySelectorAll('code, span.chat-file-path')) {
        if (el.closest('pre')) continue
        if (el.classList.contains('chat-commit-hash')) continue
        if (el.classList.contains('chat-worktree-path')) continue
        const stripped = (el.textContent || '').trim()
        if (!looksLikeWorktreePath(stripped)) continue
        const match = extractWorktreePath(stripped, projectRoot)
        if (!match) continue
        detectedWorktreePaths.push(match)
        el.classList.add('chat-worktree-path')
        el.setAttribute('data-worktree-path', match)
        const resolved = resolveFilePath(match, projectRoot)
        if (resolved) {
            el.setAttribute('data-file-path', resolved)
        }
        el.insertAdjacentHTML('afterend', worktreeButtonHtml(match, resolved || undefined))
    }

    // ── Step 3: Text nodes (outside pre/a/code) → regex match → insert span + button ──
    const textNodes: Text[] = []
    const walker = doc.createTreeWalker(doc.body, NodeFilter.SHOW_TEXT, {
        acceptNode(node: Text) {
            const parent = node.parentElement
            if (!parent) return NodeFilter.FILTER_REJECT
            if (parent.closest('pre')) return NodeFilter.FILTER_REJECT
            if (parent.tagName === 'A' || parent.closest('a')) return NodeFilter.FILTER_REJECT
            if (parent.tagName === 'CODE' || parent.closest('code')) return NodeFilter.FILTER_REJECT
            if (parent.classList.contains('chat-file-path') || parent.classList.contains('chat-commit-hash') || parent.classList.contains('chat-worktree-path')) return NodeFilter.FILTER_REJECT
            return NodeFilter.FILTER_ACCEPT
        }
    })
    while (walker.nextNode()) textNodes.push(walker.currentNode as Text)

    // Process text nodes in reverse order so that DOM insertions
    // don't invalidate later node positions.
    for (let i = textNodes.length - 1; i >= 0; i--) {
        const textNode = textNodes[i]
        const text = textNode.textContent || ''

        WORKTREE_PATH_RE.lastIndex = 0
        if (!WORKTREE_PATH_RE.test(text)) continue

        // Re-run regex to collect matches
        WORKTREE_PATH_RE.lastIndex = 0
        const parts: Array<{ text: string; absPath: string | null }> = []
        let lastIndex = 0
        let match: RegExpExecArray | null
        while ((match = WORKTREE_PATH_RE.exec(text)) !== null) {
            const pathStr = match[0]
            const absPath = extractWorktreePath(pathStr, projectRoot)
            // Push text before this match
            if (match.index > lastIndex) {
                parts.push({ text: text.slice(lastIndex, match.index), absPath: null })
            }
            parts.push({ text: pathStr, absPath })
            lastIndex = match.index + pathStr.length
        }
        // Push remaining text after last match
        if (lastIndex < text.length) {
            parts.push({ text: text.slice(lastIndex), absPath: null })
        }

        // Build replacement nodes
        const parent = textNode.parentNode!
        const frag = doc.createDocumentFragment()
        let hasAnnotation = false
        for (const part of parts) {
            if (part.absPath) {
                hasAnnotation = true
                detectedWorktreePaths.push(part.absPath)
                const span = doc.createElement('span')
                span.className = 'chat-worktree-path'
                span.setAttribute('data-worktree-path', part.absPath)
                const resolved = resolveFilePath(part.absPath, projectRoot)
                if (resolved) {
                    span.setAttribute('data-file-path', resolved)
                }
                span.textContent = part.text
                frag.appendChild(span)
                // Single action button
                const btnContainer = doc.createElement('span')
                btnContainer.innerHTML = worktreeButtonHtml(part.absPath, resolved || undefined)
                while (btnContainer.firstChild) frag.appendChild(btnContainer.firstChild)
            } else {
                frag.appendChild(doc.createTextNode(part.text))
            }
        }

        if (hasAnnotation) {
            parent.replaceChild(frag, textNode)
        }
    }

    return { html: doc.body.innerHTML, detectedWorktreePaths }
}

/**
 * Extract an absolute worktree path from a matched string.
 * - If the match starts with `/`, it's already absolute — return as-is if under projectRoot.
 * - If the match starts with `.worktrees/` or `./.worktrees/`, prepend projectRoot.
 * Returns null if the path cannot be resolved.
 */
function extractWorktreePath(matched: string, projectRoot: string): string | null {
    if (matched.startsWith('/')) {
        // Absolute path — must be under projectRoot
        if (!projectRoot || !matched.startsWith(projectRoot + '/')) return null
        return matched
    }
    // Relative path like .worktrees/feature-x or ./.worktrees/feature-x
    if (!projectRoot) return null
    const clean = matched.replace(/^\.\//, '')
    return projectRoot + '/' + clean
}

/**
 * Verify which worktree paths actually exist on the server,
 * and remove annotations for paths that are not real worktrees.
 * Called asynchronously after the synchronous annotation pass.
 */
export async function verifyWorktreePaths(paths: string[], containerEl: HTMLElement, projectRoot: string): Promise<void> {
    if (paths.length === 0) return

    const worktrees = await fetchWorktreeList(projectRoot)
    if (worktrees.length === 0) return

    // Build set of valid absolute worktree paths (excluding current)
    const validPaths = new Set<string>()
    for (const wt of worktrees) {
        if (!wt.isCurrent && wt.path) {
            validPaths.add(wt.path)
        }
    }

    // Remove annotations for paths not in the valid set
    for (const path of paths) {
        if (!validPaths.has(path)) {
            containerEl.querySelectorAll(`.chat-worktree-btn[data-worktree-path="${CSS.escape(path)}"]`).forEach(btn => {
                btn.remove()
            })
            containerEl.querySelectorAll(`.chat-worktree-path[data-worktree-path="${CSS.escape(path)}"]`).forEach(span => {
                span.replaceWith(...span.childNodes)
            })
        }
    }
}

/**
 * Clear the worktree list cache (e.g. when switching projects).
 */
export function clearWorktreeCache(): void {
    worktreeListCache.clear()
}

/**
 * Composable for worktree path annotation in rendered HTML (v-html content).
 */
export function useWorktreeAnnotation() {
    return {
        annotateWorktreePaths,
        verifyWorktreePaths,
        clearWorktreeCache,
    }
}

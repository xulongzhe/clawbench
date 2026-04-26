import { nextTick } from 'vue'
import { escapeHtml } from '@/utils/html.ts'
import { splitPath } from '@/utils/path.ts'
import { store } from '@/stores/app.ts'

/**
 * Resolve a file path to a project-relative path usable by store.selectFile().
 * Returns null if the path is not within the current project.
 * When projectRoot is empty, relative paths are returned as-is (best-effort).
 */
export function resolveFilePath(path: string, projectRoot: string): string | null {
    if (path.startsWith('/')) {
        // Absolute path: must be under projectRoot
        if (!projectRoot) return null
        if (!path.startsWith(projectRoot)) return null
        const absolutePath = path
        if (absolutePath.startsWith(projectRoot + '/')) {
            return absolutePath.slice(projectRoot.length + 1)
        }
        if (absolutePath === projectRoot) return null
        return null
    }

    // Relative path
    if (!projectRoot) {
        // No projectRoot — strip leading ./ and return as-is (best-effort)
        let clean = path.replace(/^\.\//, '')
        // Reject paths that go above root (../) when we can't verify
        if (clean.startsWith('../')) return null
        return clean
    }

    // Resolve ./ and ../ against projectRoot
    const parts = projectRoot.split('/').filter(Boolean)
    const segments = path.split('/')
    for (const seg of segments) {
        if (seg === '..') {
            if (parts.length > 0) parts.pop()
            else return null // goes above project root
        } else if (seg !== '.' && seg !== '') {
            parts.push(seg)
        }
    }
    const absolutePath = '/' + parts.join('/')
    if (absolutePath.startsWith(projectRoot + '/')) {
        return absolutePath.slice(projectRoot.length + 1)
    }
    return null
}

/**
 * Generate HTML for the small open-file button.
 */
export function fileOpenButtonHtml(resolvedPath: string): string {
    return `<button class="chat-file-open-btn" data-file-path="${escapeHtml(resolvedPath)}" title="打开文件"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg></button>`
}

export interface AnnotateFilePathsOptions {
    projectRoot: string
    /** Base directory for resolving relative <a href="..."> links (e.g. the md file's dir) */
    baseDir?: string
}

/**
 * Detect file paths in rendered HTML and insert open-file buttons after them.
 * Supports:
 *   - Absolute paths: /home/user/project/src/main.go
 *   - Project-relative paths: src/main.go, ./lib/utils.ts
 * A path must contain at least one / and a file extension to qualify.
 * Only paths within the current project get a button.
 *
 * Returns the annotated HTML and a list of detected (resolved) paths
 * for the caller to verify asynchronously.
 */
export function annotateFilePaths(
    html: string,
    options: AnnotateFilePathsOptions
): { html: string; detectedPaths: string[] } {
    const { projectRoot, baseDir } = options

    // Protect <pre> blocks from annotation (multi-line code blocks should not get buttons)
    // But allow <code> (inline code) — AI often references file paths inside inline code
    const preBlocks: string[] = []
    html = html.replace(/<pre[^>]*>[\s\S]*?<\/pre>/gi, (match) => {
        preBlocks.push(match)
        return `<!--PREBLOCK${preBlocks.length - 1}-->`
    })

    // Protect <a> tags from the bare-path regexes (they will be handled separately)
    // This prevents the regexes from breaking <a> tags by replacing content inside them
    const aBlocks: string[] = []
    html = html.replace(/<a\s+[^>]*href="[^"]*"[^>]*>[\s\S]*?<\/a>/gi, (match) => {
        aBlocks.push(match)
        return `<!--ABLOCK${aBlocks.length - 1}-->`
    })

    // Protect <code> tags from the bare-path regexes (they will be handled separately)
    // The regexes can match paths after the '>' in <code>, breaking the tag structure
    const codeBlocks: string[] = []
    html = html.replace(/<code[^>]*>[\s\S]*?<\/code>/gi, (match) => {
        codeBlocks.push(match)
        return `<!--CODEBLOCK${codeBlocks.length - 1}-->`
    })

    const detectedPaths: string[] = []

    // Absolute paths: /.../.../file.ext  (only if projectRoot is available)
    if (projectRoot) {
        html = html.replace(/(^|[\s(>"'])(\/[^\s<>"')\]]+(?:\/[^\s<>"')\]]+)+\.[a-zA-Z][a-zA-Z0-9]*)/gm, (match, prefix, path) => {
            const resolved = resolveFilePath(path, projectRoot)
            if (!resolved) return match
            detectedPaths.push(resolved)
            return `${prefix}<span class="chat-file-path" data-file-path="${escapeHtml(resolved)}">${escapeHtml(path)}</span>${fileOpenButtonHtml(resolved)}`
        })
    }

    // Relative paths starting with ./ or ../
    html = html.replace(/(^|[\s(>"'])(\.\.?\/[^\s<>"')\]]+(?:\/[^\s<>"')\]]+)*\.[a-zA-Z][a-zA-Z0-9]*)/gm, (match, prefix, path) => {
        const resolved = resolveFilePath(path, projectRoot)
        if (!resolved) return match
        detectedPaths.push(resolved)
        return `${prefix}<span class="chat-file-path" data-file-path="${escapeHtml(resolved)}">${escapeHtml(path)}</span>${fileOpenButtonHtml(resolved)}`
    })

    // Bare relative paths: word/word/file.ext  (at least two path segments + extension)
    html = html.replace(/(^|[\s("'()])([a-zA-Z0-9_-]+(?:\/[a-zA-Z0-9_.-]+)+\.[a-zA-Z][a-zA-Z0-9]*)/gm, (match, prefix, path) => {
        if (prefix === '>') return match
        const resolved = resolveFilePath(path, projectRoot)
        if (!resolved) return match
        detectedPaths.push(resolved)
        return `${prefix}<span class="chat-file-path" data-file-path="${escapeHtml(resolved)}">${escapeHtml(path)}</span>${fileOpenButtonHtml(resolved)}`
    })

    // Restore <code> blocks and annotate file paths inside them
    html = html.replace(/<!--CODEBLOCK(\d+)-->/g, (_, idx) => {
        const match = codeBlocks[parseInt(idx)]
        // Extract content between <code>...</code>
        const codeMatch = match.match(/<code([^>]*)>([\s\S]*?)<\/code>/i)
        if (!codeMatch) return match
        const attrs = codeMatch[1]
        const codeContent = codeMatch[2]
        const stripped = codeContent.replace(/<[^>]+>/g, '').trim()
        const resolved = resolveFilePath(stripped, projectRoot)
        if (!resolved) return match
        detectedPaths.push(resolved)
        return `<code${attrs} class="chat-file-path" data-file-path="${escapeHtml(resolved)}">${codeContent}</code>${fileOpenButtonHtml(resolved)}`
    })

    // Restore <a> blocks (now they are safe from the bare-path regexes above)
    html = html.replace(/<!--ABLOCK(\d+)-->/g, (_, idx) => aBlocks[parseInt(idx)])

    // Annotate <a> links that point to local files (append open button after the link)
    // Only matches non-http, non-anchor, non-mailto links
    html = html.replace(/<a\s+([^>]*href="([^"]+)"[^>]*)>([\s\S]*?)<\/a>/gi, (match, attrs, href, linkContent) => {
        // Skip external links, anchors, mailto, tel
        if (/^(https?:|\/\/|mailto:|tel:|#)/i.test(href)) return match
        // Resolve the href against baseDir (for MarkdownPreview) or projectRoot (for ChatPanel)
        const resolved = baseDir
            ? resolveRelativePath(href, baseDir)
            : resolveFilePath(href, projectRoot)
        if (!resolved) return match
        detectedPaths.push(resolved)
        // Keep the <a> tag intact, just append the open button after it
        return `${match}${fileOpenButtonHtml(resolved)}`
    })

    // Restore pre blocks
    html = html.replace(/<!--PREBLOCK(\d+)-->/g, (_, idx) => preBlocks[parseInt(idx)])

    return { html, detectedPaths }
}

// Cache of verified paths: path -> true (exists) | false (not found)
const verifiedCache = new Map<string, boolean>()
// In-flight verification requests to avoid duplicates
const inFlight = new Map<string, Promise<boolean>>()

async function checkPathExists(path: string): Promise<boolean> {
    if (verifiedCache.has(path)) return verifiedCache.get(path)!
    if (inFlight.has(path)) return inFlight.get(path)!

    const promise = (async () => {
        try {
            const [fileResp, dirResp] = await Promise.all([
                fetch(`/api/file/${encodeURIComponent(path)}`, { method: 'HEAD' }),
                fetch(`/api/dir?path=${encodeURIComponent(path)}`, { method: 'HEAD' }),
            ])
            return fileResp.ok || dirResp.ok
        } catch {
            return true // Network error — assume exists (best effort)
        }
    })()

    inFlight.set(path, promise)
    const exists = await promise
    verifiedCache.set(path, exists)
    inFlight.delete(path)
    return exists
}

/**
 * Check which file paths actually exist on the server,
 * and hide buttons for files that don't exist.
 */
export async function verifyFilePaths(paths: string[], containerEl: HTMLElement): Promise<void> {
    // Batch verification with concurrency limit
    const limit = 6
    const unique = [...new Set(paths)]
    const results = new Map<string, boolean>()

    for (let i = 0; i < unique.length; i += limit) {
        const batch = unique.slice(i, i + limit)
        const batchResults = await Promise.all(batch.map(async (path) => {
            const exists = await checkPathExists(path)
            return { path, exists }
        }))
        for (const { path, exists } of batchResults) {
            results.set(path, exists)
        }
    }

    for (const [path, exists] of results) {
        if (!exists) {
            containerEl.querySelectorAll(`.chat-file-open-btn[data-file-path="${CSS.escape(path)}"]`).forEach(btn => {
                btn.remove()
            })
            containerEl.querySelectorAll(`.chat-file-path[data-file-path="${CSS.escape(path)}"]`).forEach(span => {
                span.replaceWith(...span.childNodes)
            })
        }
    }
}

/**
 * Clear the verification cache (e.g. when switching projects).
 */
export function clearVerifiedCache(): void {
    verifiedCache.clear()
    inFlight.clear()
}

/**
 * Composable for file path annotation in rendered HTML (v-html content).
 */
export function useFilePathAnnotation() {
    return {
        resolveFilePath,
        fileOpenButtonHtml,
        annotateFilePaths,
        verifyFilePaths,
        resolveRelativePath,
        openFilePath,
        clearVerifiedCache,
    }
}

/**
 * Resolve a relative href against a base directory.
 * Normalizes . and .. segments.
 * Returns the resolved project-relative path.
 */
export function resolveRelativePath(href: string, baseDir: string): string {
    if (!baseDir) return href
    const parts = splitPath(baseDir + '/' + href)
    const normalized: string[] = []
    for (const part of parts) {
        if (part === '.' || part === '') continue
        if (part === '..') { normalized.pop(); continue }
        normalized.push(part)
    }
    return normalized.join('/')
}

/**
 * Open a file or directory path.
 * If the path is a directory, navigates to it and opens the file manager.
 * If it's a file, selects it in the store.
 */
export async function openFilePath(resolvedPath: string): Promise<void> {
    // Check if path is a directory
    try {
        const resp = await fetch(`/api/dir?path=${encodeURIComponent(resolvedPath)}`)
        if (resp.ok) {
            await store.navigateToDir(resolvedPath)
            window.dispatchEvent(new CustomEvent('open-file-manager'))
            return
        }
    } catch {
        // Ignore, fall through to open as file
    }

    store.selectFile(resolvedPath)
}

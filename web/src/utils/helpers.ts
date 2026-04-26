// Utility helpers — re-export from dedicated modules for backward compatibility
// New code should import directly from the specific module (e.g., @/utils/path.ts)

// Shared copy utility
export function copyText(text: string, onSuccess?: () => void, onError?: () => void): void {
    const fallbackCopy = (text: string): boolean => {
        const ta = document.createElement('textarea')
        ta.value = text
        ta.style.cssText = 'position:fixed;opacity:0;top:0'
        document.body.appendChild(ta)
        ta.focus()
        ta.select()
        try { document.execCommand('copy'); return true } catch (_) { return false }
        finally { document.body.removeChild(ta) }
    }

    if (navigator.clipboard?.writeText) {
        navigator.clipboard.writeText(text).then(() => {
            onSuccess?.()
        }).catch(() => {
            if (fallbackCopy(text)) onSuccess?.()
            else onError?.()
        })
    } else {
        if (fallbackCopy(text)) onSuccess?.()
        else onError?.()
    }
}

// Re-exports from dedicated modules
export { splitPath, baseName, dirName } from './path.ts'
export { escapeHtml } from './html.ts'
export { getFileType, formatFileSize } from './fileType.ts'
export type { FileType } from './fileType.ts'
export { extractToc, slugify } from './toc.ts'
export type { TocItem } from './toc.ts'
export { formatRelativeTime, formatDateTime, humanizeCron, repeatLabel, statusLabel } from './format.ts'
export { initMermaid, reRenderMermaid } from './mermaid.ts'

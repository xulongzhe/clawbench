// Utility helpers (shared across components)
import { mermaid } from './globals.ts'

// =============================================
// Cross-platform path utilities
// =============================================

// Split a path into segments, handling both / and \ separators
export function splitPath(path: string): string[] {
    return path.split(/[/\\]/)
}

// Get the last segment of a path (filename or directory name)
export function baseName(path: string): string {
    return splitPath(path).pop() || path
}

// Get the parent directory of a path
export function dirName(path: string): string {
    const parts = splitPath(path)
    parts.pop()
    if (parts.length === 0) return ''
    // Rejoin with original separator style
    const useBackslash = path.includes('\\') && !path.includes('/')
    const result = useBackslash ? parts.join('\\') : parts.join('/')
    // On Windows, a lone "C:" should be "C:\" (drive root)
    if (/^[A-Za-z]:$/.test(result)) return result + '\\'
    return result
}

// Escape HTML
export function escapeHtml(text: string): string {
    const map: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' }
    return String(text).replace(/[&<>"']/g, m => map[m])
}

// File type detection
export interface FileType {
    exts: string[]
    lang: string
    label: string
    color: string
    isMarkdown: boolean
    isImage?: boolean
    isAudio?: boolean
    isVideo?: boolean
}

const FILE_TYPES: FileType[] = [
    { exts: ['.md', '.markdown'], lang: 'markdown', label: 'MD', color: '#4a90d9', isMarkdown: true },
    { exts: ['.json', '.jsonc', '.json5'], lang: 'json', label: 'JSON', color: '#e0a030', isMarkdown: false },
    { exts: ['.yaml', '.yml'], lang: 'yaml', label: 'YAML', color: '#cb6f1e', isMarkdown: false },
    { exts: ['.toml'], lang: 'toml', label: 'TOML', color: '#9c4122', isMarkdown: false },
    { exts: ['.xml', '.plist'], lang: 'xml', label: 'XML', color: '#e44d26', isMarkdown: false },
    { exts: ['.ini', '.properties', '.conf', '.cfg'], lang: 'ini', label: 'INI', color: '#8b8b8b', isMarkdown: false },
    { exts: ['.go', '.mod', '.sum'], lang: 'go', label: 'Go', color: '#00acd7', isMarkdown: false },
    { exts: ['.py', '.pyi'], lang: 'python', label: 'PY', color: '#3572a5', isMarkdown: false },
    { exts: ['.rs'], lang: 'rust', label: 'RS', color: '#ce412b', isMarkdown: false },
    { exts: ['.js', '.mjs', '.cjs'], lang: 'javascript', label: 'JS', color: '#f7df1e', isMarkdown: false },
    { exts: ['.ts', '.tsx', '.mts', '.cts'], lang: 'typescript', label: 'TS', color: '#3178c6', isMarkdown: false },
    { exts: ['.java'], lang: 'java', label: 'Java', color: '#b07219', isMarkdown: false },
    { exts: ['.cs'], lang: 'csharp', label: 'C#', color: '#68217a', isMarkdown: false },
    { exts: ['.rb'], lang: 'ruby', label: 'RB', color: '#cc342d', isMarkdown: false },
    { exts: ['.php'], lang: 'php', label: 'PHP', color: '#4f5d95', isMarkdown: false },
    { exts: ['.swift'], lang: 'swift', label: 'Swift', color: '#f05138', isMarkdown: false },
    { exts: ['.kt', '.kts'], lang: 'kotlin', label: 'Kotlin', color: '#7f52ff', isMarkdown: false },
    { exts: ['.scala'], lang: 'scala', label: 'Scala', color: '#dc322f', isMarkdown: false },
    { exts: ['.c', '.h'], lang: 'c', label: 'C', color: '#555555', isMarkdown: false },
    { exts: ['.cpp', '.hpp', '.cc', '.cxx'], lang: 'cpp', label: 'C++', color: '#f34b7d', isMarkdown: false },
    { exts: ['.lua'], lang: 'lua', label: 'Lua', color: '#000080', isMarkdown: false },
    { exts: ['.r', '.R'], lang: 'r', label: 'R', color: '#198ce7', isMarkdown: false },
    { exts: ['.pl', '.pm'], lang: 'perl', label: 'Perl', color: '#cc99cc', isMarkdown: false },
    { exts: ['.sh', '.bash', '.zsh', '.fish', '.ksh', '.ash'], lang: 'bash', label: 'SH', color: '#89e051', isMarkdown: false },
    { exts: ['.ps1', '.psm1'], lang: 'powershell', label: 'PS', color: '#012456', isMarkdown: false },
    { exts: ['.sql'], lang: 'sql', label: 'SQL', color: '#e38c00', isMarkdown: false },
    { exts: ['.graphql', '.gql'], lang: 'graphql', label: 'GraphQL', color: '#e10098', isMarkdown: false },
    { exts: ['.html', '.htm', '.xhtml'], lang: 'xml', label: 'HTML', color: '#e44d26', isMarkdown: false },
    { exts: ['.css', '.scss', '.sass', '.less', '.styl'], lang: 'css', label: 'CSS', color: '#563d7c', isMarkdown: false },
    { exts: ['.vue', '.svelte'], lang: 'xml', label: 'Vue', color: '#41b883', isMarkdown: false },
    { exts: ['.dockerfile', '.dockerignore'], lang: 'dockerfile', label: 'Docker', color: '#384d54', isMarkdown: false },
    { exts: ['.makefile', '.mak'], lang: 'makefile', label: 'Make', color: '#6d8086', isMarkdown: false },
    { exts: ['.nginx'], lang: 'nginx', label: 'Nginx', color: '#009639', isMarkdown: false },
    { exts: ['.gitignore', '.gitattributes', '.gitconfig', '.editorconfig', '.ignore'], lang: 'plaintext', label: 'Config', color: '#6d8086', isMarkdown: false },
    { exts: ['.env', '.env.example', '.env.local'], lang: 'bash', label: 'ENV', color: '#ecd53f', isMarkdown: false },
    { exts: ['.txt', '.text', '.log'], lang: 'plaintext', label: 'TXT', color: '#8b8b8b', isMarkdown: false },
    { exts: ['.diff', '.patch'], lang: 'diff', label: 'Diff', color: '#8b8b8b', isMarkdown: false },
    { exts: ['.csv', '.tsv'], lang: 'plaintext', label: 'CSV', color: '#237f4a', isMarkdown: false },
    { exts: ['.tex'], lang: 'latex', label: 'LaTeX', color: '#3d6118', isMarkdown: false },
    { exts: ['.pem', '.crt', '.key', '.pub'], lang: 'plaintext', label: 'Cert', color: '#8b8b8b', isMarkdown: false },
    { exts: ['.regex', '.regexp'], lang: 'regex', label: 'Regex', color: '#8b8b8b', isMarkdown: false },
    { exts: ['.png'], lang: 'image', label: 'PNG', color: '#a855f7', isMarkdown: false, isImage: true },
    { exts: ['.jpg', '.jpeg'], lang: 'image', label: 'JPG', color: '#a855f7', isMarkdown: false, isImage: true },
    { exts: ['.gif'], lang: 'image', label: 'GIF', color: '#a855f7', isMarkdown: false, isImage: true },
    { exts: ['.webp'], lang: 'image', label: 'WEBP', color: '#a855f7', isMarkdown: false, isImage: true },
    { exts: ['.svg'], lang: 'image', label: 'SVG', color: '#a855f7', isMarkdown: false, isImage: true },
    { exts: ['.bmp'], lang: 'image', label: 'BMP', color: '#a855f7', isMarkdown: false, isImage: true },
    { exts: ['.ico'], lang: 'image', label: 'ICO', color: '#a855f7', isMarkdown: false, isImage: true },
    { exts: ['.tiff', '.tif'], lang: 'image', label: 'TIFF', color: '#a855f7', isMarkdown: false, isImage: true },
    { exts: ['.avif'], lang: 'image', label: 'AVIF', color: '#a855f7', isMarkdown: false, isImage: true },
    { exts: ['.pdf'], lang: 'pdf', label: 'PDF', color: '#e53e3e', isMarkdown: false, isImage: true },
    { exts: ['.mp3'], lang: 'audio', label: 'MP3', color: '#22c55e', isMarkdown: false, isAudio: true },
    { exts: ['.wav'], lang: 'audio', label: 'WAV', color: '#22c55e', isMarkdown: false, isAudio: true },
    { exts: ['.ogg'], lang: 'audio', label: 'OGG', color: '#22c55e', isMarkdown: false, isAudio: true },
    { exts: ['.m4a'], lang: 'audio', label: 'M4A', color: '#22c55e', isMarkdown: false, isAudio: true },
    { exts: ['.aac'], lang: 'audio', label: 'AAC', color: '#22c55e', isMarkdown: false, isAudio: true },
    { exts: ['.flac'], lang: 'audio', label: 'FLAC', color: '#22c55e', isMarkdown: false, isAudio: true },
    { exts: ['.wma'], lang: 'audio', label: 'WMA', color: '#22c55e', isMarkdown: false, isAudio: true },
    { exts: ['.opus'], lang: 'audio', label: 'OPUS', color: '#22c55e', isMarkdown: false, isAudio: true },
    { exts: ['.mp4'], lang: 'video', label: 'MP4', color: '#ef4444', isMarkdown: false, isVideo: true },
    { exts: ['.mkv'], lang: 'video', label: 'MKV', color: '#ef4444', isMarkdown: false, isVideo: true },
    { exts: ['.avi'], lang: 'video', label: 'AVI', color: '#ef4444', isMarkdown: false, isVideo: true },
    { exts: ['.mov'], lang: 'video', label: 'MOV', color: '#ef4444', isMarkdown: false, isVideo: true },
    { exts: ['.webm'], lang: 'video', label: 'WEBM', color: '#ef4444', isMarkdown: false, isVideo: true },
    { exts: ['.flv'], lang: 'video', label: 'FLV', color: '#ef4444', isMarkdown: false, isVideo: true },
    { exts: ['.wmv'], lang: 'video', label: 'WMV', color: '#ef4444', isMarkdown: false, isVideo: true },
    { exts: ['.m4v'], lang: 'video', label: 'M4V', color: '#ef4444', isMarkdown: false, isVideo: true },
    { exts: ['.3gp'], lang: 'video', label: '3GP', color: '#ef4444', isMarkdown: false, isVideo: true },
    { exts: ['.m3u8'], lang: 'video', label: 'M3U8', color: '#ef4444', isMarkdown: false, isVideo: true },
]

export function getFileType(name: string): FileType {
    const lower = name.toLowerCase()
    for (const ft of FILE_TYPES) {
        for (const ext of ft.exts) {
            if (lower.endsWith(ext)) return ft
        }
    }
    return { lang: 'plaintext', label: 'TXT', color: '#8b8b8b', isMarkdown: false }
}

// Shared copy utility - used by multiple components
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

export function formatFileSize(bytes: number): string {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

// Generate a slug from text (compatible with markdown anchor links)
export function slugify(text: string): string {
    return text
        .toLowerCase()
        .replace(/[^\w\u4e00-\u9fa5]+/g, '-')  // Keep Chinese, letters, digits, replace others with -
        .replace(/^-+|-+$/g, '');  // Remove leading/trailing dashes
}

export interface TocItem {
    level: number
    text: string
    id: string
    line: number
}

export function extractToc(content: string, lang: string): TocItem[] {
    if (lang === 'markdown') return extractTocMarkdown(content)
    return extractTocForCode(content, lang)
}

function extractTocMarkdown(content: string): TocItem[] {
    const toc: TocItem[] = []
    const headerRegex = /^(#{1,6})\s+(.+)$/gm
    let match
    while ((match = headerRegex.exec(content)) !== null) {
        const level = match[1].length
        const text = match[2].trim()
        const id = slugify(text)
        const line = content.substring(0, match.index).split('\n').length
        toc.push({ level, text, id, line })
    }
    return toc
}

// Patterns for code languages: [regex, lineOffset(0-based), level]
type CodePattern = [RegExp, number, number]

const CODE_TOC_PATTERNS: Record<string, CodePattern[]> = {
    go: [
        [/^func\s+(?:\(\S+\)\s+)?(\S+)/gm, 1, 1],
        [/^type\s+(\S+)\s+struct\b/gm, 1, 1],
        [/^type\s+(\S+)\s+interface\b/gm, 1, 1],
        [/^type\s+(\S+)\s+(?:=\s+|$)/gm, 1, 1],
        [/^var\s+(\S+)/gm, 1, 1],
        [/^const\s+(\S+)/gm, 1, 1],
    ],
    python: [
        [/^(?:async\s+)?def\s+(\S+)\s*\(/gm, 1, 1],
        [/^class\s+(\S+)\s*[:\(]/gm, 1, 1],
    ],
    javascript: [
        [/^(?:export\s+)?(?:async\s+)?function\s+(\S+)\s*\(/gm, 1, 1],
        [/^(?:export\s+)?(?:default\s+)?const\s+(\S+)\s*=/gm, 1, 1],
        [/^(?:export\s+)?class\s+(\S+)/gm, 1, 1],
    ],
    typescript: [
        [/^(?:export\s+)?(?:async\s+)?function\s+(\S+)\s*\(/gm, 1, 1],
        [/^(?:export\s+)?(?:default\s+)?const\s+(\S+)\s*[:=]/gm, 1, 1],
        [/^(?:export\s+)?interface\s+(\S+)/gm, 1, 1],
        [/^(?:export\s+)?type\s+(\S+)\s*=/gm, 1, 1],
        [/^(?:export\s+)?class\s+(\S+)/gm, 1, 1],
        [/^(?:export\s+)?enum\s+(\S+)/gm, 1, 1],
    ],
    rust: [
        [/^(?:pub\s+)?(?:async\s+)?fn\s+(\S+)\s*\(/gm, 1, 1],
        [/^(?:pub\s+)?struct\s+(\S+)/gm, 1, 1],
        [/^(?:pub\s+)?enum\s+(\S+)/gm, 1, 1],
        [/^(?:pub\s+)?trait\s+(\S+)/gm, 1, 1],
        [/^(?:pub\s+)?mod\s+(\S+)/gm, 1, 1],
        [/^(?:pub\s+)?type\s+(\S+)/gm, 1, 1],
        [/^(?:pub\s+)?impl\s*(?:<[^>]*>)?\s+(\S+)/gm, 1, 1],
    ],
    java: [
        [/^(?:public|private|protected)?\s*(?:static\s+)?(?:synchronized\s+)?(?:class|interface|enum)\s+(\S+)/gm, 1, 1],
        [/^(?:public|private|protected)\s+.*?(?:static\s+)?(?:\S+\s+)?(\S+)\s*\(/gm, 1, 1],
    ],
    csharp: [
        [/^(?:public|private|protected|internal)?\s*(?:static|abstract|virtual|override|sealed|partial)*\s*(class|struct|interface|enum)\s+(\S+)/gm, 2, 2],
        [/^(?:public|private|protected|internal)?\s*(?:static|abstract|virtual|override|async)*\s+\S+\s+(\S+)\s*\(/gm, 1, 1],
    ],
    ruby: [
        [/^(?:private|protected|public)?\s*(?:class|module)\s+(\S+)/gm, 1, 1],
        [/^def\s+(?:self\.)?(\S+)/gm, 1, 1],
    ],
    php: [
        [/^(?:abstract\s+)?(?:class|interface|trait)\s+(\S+)/gm, 1, 1],
        [/^(?:public|private|protected)\s+static\s+function\s+(\S+)\s*\(/gm, 1, 1],
        [/^function\s+(\S+)\s*\(/gm, 1, 1],
    ],
    kotlin: [
        [/^(?:fun|val|var)\s+(\S+)/gm, 1, 1],
        [/^(?:class|object|interface|enum|data class)\s+(\S+)/gm, 1, 1],
    ],
    scala: [
        [/^(?:def|val|var|lazy val)\s+(\S+)/gm, 1, 1],
        [/^(?:class|object|trait|case class|enum)\s+(\S+)/gm, 1, 1],
    ],
    c: [
        [/^(?:static\s+)?(?:inline\s+)?(?:\S+\s+)+(\S+)\s*\(/gm, 1, 1],
        [/^(?:typedef\s+)?struct\s+(\S+)\s*\{/gm, 1, 1],
        [/^(?:typedef\s+)?enum\s+(\S+)\s*\{/gm, 1, 1],
    ],
    cpp: [
        [/^(?:static|inline|virtual|explicit)\s+(?:\S+\s+)+(\S+)\s*\(/gm, 1, 1],
        [/^(?:class|struct)\s+(\S+)/gm, 1, 1],
        [/^(?:enum|namespace)\s+(\S+)/gm, 1, 1],
        [/^template\s*<[^>]+>\s*(?:class|struct)\s+(\S+)/gm, 1, 1],
    ],
    lua: [
        [/^function\s+(?:[\w.]+[\.:])(\S+)\s*\(/gm, 1, 1],
        [/^local\s+function\s+(\S+)\s*\(/gm, 1, 1],
    ],
    bash: [
        [/^(?:function\s+)?(\S+)\s*\(\)/gm, 1, 1],
    ],
    sql: [
        [/^(?:CREATE|ALTER|DROP)\s+(?:OR\s+REPLACE\s+)?(?:TABLE|VIEW|INDEX|FUNCTION|PROCEDURE|TRIGGER)\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?[`"]?(\S+)[`"]?/gim, 1, 1],
    ],
    css: [
        [/^@(\S+)/gm, 1, 1],
    ],
    makefile: [
        [/^(\S+):\s*$/gm, 1, 1],
    ],
    nginx: [
        [/^\s*(?:server|location|upstream)\s+(\S+)/gm, 1, 1],
    ],
    ini: [
        [/^\[([^\]]+)\]/gm, 1, 1],
    ],
}

function extractLine(content: string, offset: number): number {
    return content.substring(0, offset).split('\n').length
}

function extractTocForCode(content: string, lang: string): TocItem[] {
    const patterns = CODE_TOC_PATTERNS[lang]
    if (!patterns) return extractTocGeneric(content)

    const seen = new Set<string>()
    const toc: TocItem[] = []
    for (const [regex] of patterns) {
        regex.lastIndex = 0
        let match
        while ((match = regex.exec(content)) !== null) {
            const text = match[1].replace(/[{].*$/, '').replace(/[<(].*$/, '').trim()
            if (!text || seen.has(text)) continue
            seen.add(text)
            const line = extractLine(content, match.index)
            toc.push({ level: 1, text, id: 'toc-l' + line, line })
        }
    }
    // Sort by line number
    toc.sort((a, b) => a.line - b.line)
    // Assign levels based on dedup/sort
    return toc
}

function extractTocGeneric(content: string): TocItem[] {
    // For JSON/YAML/TOML/XML: extract top-level keys/sections by indentation
    const lines = content.split('\n')
    const toc: TocItem[] = []

    for (let i = 0; i < lines.length; i++) {
        const line = lines[i]
        const trimmed = line.trim()
        if (!trimmed || trimmed.startsWith('//') || trimmed.startsWith('#') || trimmed.startsWith('<!--')) continue

        const indent = line.search(/\S/)
        // JSON: only top-level keys (indent 2 or 4)
        if (trimmed.endsWith(':') || trimmed.endsWith(',') || trimmed.endsWith('{') || trimmed.endsWith('[') || trimmed.endsWith('(')) {
            // Extract key name
            const keyMatch = trimmed.match(/^["']?([^"':{\[\s,]+)["']?\s*[:{[\(,]/)
            if (keyMatch) {
                const key = keyMatch[1].trim()
                if (key.length < 2 || key === '{' || key === '[') continue
                if (indent <= 2) {
                    toc.push({ level: Math.floor(indent / 2) + 1, text: key, id: 'toc-l' + (i + 1), line: i + 1 })
                }
            }
        }
    }
    return toc.slice(0, 100)
}

// Initialize Mermaid
export function initMermaid(): void {
    mermaid.initialize({
        startOnLoad: false,
        theme: document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'default',
        securityLevel: 'loose',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    })
}

// Re-render all rendered mermaid diagrams on the page (called after theme switch)
export function reRenderMermaid(): void {
    document.querySelectorAll<HTMLDivElement>('div.mermaid[data-mermaid]').forEach(container => {
        const source = container.dataset.mermaid
        if (!source) return
        const id = container.id || `mermaid-${Date.now()}`
        container.removeAttribute('id')
        mermaid.render(id, source).then(result => {
            container.innerHTML = result.svg
            container.id = id
        }).catch(() => {})
    })
}

// =============================================
// Time formatting utilities
// =============================================

/** Format a date as relative time (Chinese locale) */
export function formatRelativeTime(date: string | Date): string {
    if (!date) return ''
    const d = new Date(date)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    const minutes = Math.floor(diff / 60000)
    const hours = Math.floor(diff / 3600000)
    const days = Math.floor(diff / 86400000)

    if (minutes < 1) return '刚刚'
    if (minutes < 60) return `${minutes}分钟前`
    if (hours < 24) return `${hours}小时前`
    if (days < 7) return `${days}天前`
    return d.toLocaleDateString('zh-CN')
}

/** Format a date as a localized datetime string (zh-CN) */
export function formatDateTime(date: string | Date): string {
    if (!date) return ''
    const d = new Date(date)
    return d.toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit'
    })
}

// =============================================
// Task/scheduler utilities
// =============================================

/** Humanize a cron expression into Chinese description */
export function humanizeCron(expr: string): string {
    const parts = expr.split(' ')
    if (parts.length !== 5) return expr
    const [min, hour, day, month, weekday] = parts
    if (min.startsWith('*/') && hour === '*') return `每 ${min.slice(2)} 分钟`
    if (hour.startsWith('*/') && min === '0') return `每 ${hour.slice(2)} 小时`
    if (min === '0' && !hour.includes('/') && day === '*' && month === '*' && weekday === '*') return `每天 ${hour}:00`
    if (min === '0' && weekday === '1-5') return `工作日 ${hour}:00`
    return expr
}

/** Get a Chinese label for task repeat mode */
export function repeatLabel(mode: string, maxRuns: number): string {
    if (mode === 'once') return '单次'
    if (mode === 'limited') return `${maxRuns}次`
    return '不限'
}

/** Get a Chinese label for task status */
export function statusLabel(status: string): string {
    if (status === 'active') return '运行中'
    if (status === 'paused') return '已暂停'
    if (status === 'completed') return '已完成'
    return status
}

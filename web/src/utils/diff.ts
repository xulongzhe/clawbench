// Diff rendering utilities

import { hljs } from './globals.ts'
import { escapeHtml, getFileType } from './helpers.ts'

export function detectLang(filePath: string): string {
    if (!filePath) return 'plaintext'
    return getFileType(filePath).lang
}

export function highlightLine(line: string, lang: string): string {
    if (!line) return ''
    try {
        return hljs.highlight(line, { language: lang, ignoreIllegals: true }).value
    } catch {
        return escapeHtml(line)
    }
}

interface Hunk {
    loc: string
    adds: string[]
    dels: string[]
}

export function renderDiff(raw: string, filePath: string): string {
    const lang = detectLang(filePath)
    const lines = raw.split('\n')
    const hunks: Hunk[] = []
    let currentHunk: Hunk | null = null

    for (const line of lines) {
        if (line.startsWith('@@')) {
            if (currentHunk && (currentHunk.adds.length > 0 || currentHunk.dels.length > 0)) {
                hunks.push(currentHunk)
            }
            const content = line.replace(/^@@.*?@@\s*/, '').trim()
            currentHunk = { loc: content || '', adds: [], dels: [] }
        } else if (line.startsWith('+') && !line.startsWith('+++')) {
            if (currentHunk) currentHunk.adds.push(line.substring(1))
        } else if (line.startsWith('-') && !line.startsWith('---')) {
            if (currentHunk) currentHunk.dels.push(line.substring(1))
        } else if (/^(diff |index |---|\+\+\+)/.test(line)) {
            // skip meta
        }
    }
    if (currentHunk && (currentHunk.adds.length > 0 || currentHunk.dels.length > 0)) {
        hunks.push(currentHunk)
    }

    if (hunks.length === 0) {
        if (raw.trim().length === 0) return ''
        const clean = lines
            .filter(l => !/^(diff |index |---|\+\+\+)/.test(l))
            .map(l => l.replace(/^[+-]{2}/, ''))
            .join('\n')
        return `<div class="diff-view"><pre class="diff-raw">${escapeHtml(clean)}</pre></div>`
    }

    let html = `<div class="diff-view diff-card-view">`
    for (const hunk of hunks) {
        if (hunk.loc) {
            html += `<div class="diff-hunk-loc">📍 ${escapeHtml(hunk.loc)}</div>`
        }

        const hasBoth = hunk.adds.length > 0 && hunk.dels.length > 0

        if (hasBoth) {
            html += `<div class="diff-card-pair">`
            if (hunk.dels.length > 0) {
                html += `<div class="diff-card diff-card-del">`
                html += `<div class="diff-card-label">删除</div>`
                for (const line of hunk.dels) {
                    html += `<div class="diff-card-line">${highlightLine(line, lang)}</div>`
                }
                html += `</div>`
            }
            if (hunk.adds.length > 0) {
                html += `<div class="diff-card diff-card-add">`
                html += `<div class="diff-card-label">新增</div>`
                for (const line of hunk.adds) {
                    html += `<div class="diff-card-line">${highlightLine(line, lang)}</div>`
                }
                html += `</div>`
            }
            html += `</div>`
        } else if (hunk.adds.length > 0) {
            html += `<div class="diff-card diff-card-add">`
            html += `<div class="diff-card-label">新增</div>`
            for (const line of hunk.adds) {
                html += `<div class="diff-card-line">${highlightLine(line, lang)}</div>`
            }
            html += `</div>`
        } else if (hunk.dels.length > 0) {
            html += `<div class="diff-card diff-card-del">`
            html += `<div class="diff-card-label">删除</div>`
            for (const line of hunk.dels) {
                html += `<div class="diff-card-line">${highlightLine(line, lang)}</div>`
            }
            html += `</div>`
        }
    }
    return html + '</div>'
}

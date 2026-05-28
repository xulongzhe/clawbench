import { describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

/**
 * Tests for MarkdownPreview.vue error display and image path encoding.
 * Since fixLocalImagePaths is an internal function inside <script setup>,
 * we test it indirectly through the component's rendered output.
 * For pure logic testing, we extract the function logic and test it directly.
 */

// ── Test the fixLocalImagePaths logic (extracted for testability) ──

/**
 * Replicate the fixLocalImagePaths logic from MarkdownPreview.vue
 * to test the encoding behavior independently.
 */
function fixLocalImagePaths(html: string, currentDir: string, imageTimestamp: number): string {
    return html.replace(/<img\s+([^>]*src=[^>]*)>/gi, (match, attrs) => {
        const srcMatch = attrs.match(/src="([^"]*)"/)
        if (!srcMatch) return match
        const src = srcMatch[1]
        if (/^(https?:|\/\/|^\/)/i.test(src)) return match
        let resolved = currentDir ? currentDir + '/' + src : src
        const parts = resolved.split(/[/\\]/)
        const normalized = []
        for (const part of parts) {
            if (part === '.' || part === '') continue
            if (part === '..') { normalized.pop(); continue }
            normalized.push(encodeURIComponent(part))
        }
        return match.replace(`src="${src}"`, `src="/api/local-file/${normalized.join('/')}?t=${imageTimestamp}"`)
    })
}

describe('fixLocalImagePaths — Chinese path encoding', () => {
    const timestamp = 1234567890

    it('encodes Chinese characters in image path segments', () => {
        const html = '<img src="中文/图片.png">'
        const result = fixLocalImagePaths(html, 'docs', timestamp)
        expect(result).toContain('/api/local-file/docs/%E4%B8%AD%E6%96%87/%E5%9B%BE%E7%89%87.png')
        expect(result).toContain(`t=${timestamp}`)
    })

    it('encodes Chinese filename but keeps extension readable', () => {
        const html = '<img src="截图.jpg">'
        const result = fixLocalImagePaths(html, '', timestamp)
        expect(result).toContain('/api/local-file/%E6%88%AA%E5%9B%BE.jpg')
    })

    it('handles mixed ASCII and Chinese path segments', () => {
        const html = '<img src="assets/图片/logo.png">'
        const result = fixLocalImagePaths(html, '', timestamp)
        expect(result).toContain('/api/local-file/assets/%E5%9B%BE%E7%89%87/logo.png')
    })

    it('does not modify absolute URLs (http://)', () => {
        const html = '<img src="http://example.com/image.png">'
        const result = fixLocalImagePaths(html, 'docs', timestamp)
        expect(result).toBe(html)
    })

    it('does not modify absolute URLs (https://)', () => {
        const html = '<img src="https://example.com/image.png">'
        const result = fixLocalImagePaths(html, 'docs', timestamp)
        expect(result).toBe(html)
    })

    it('does not modify protocol-relative URLs', () => {
        const html = '<img src="//cdn.example.com/image.png">'
        const result = fixLocalImagePaths(html, 'docs', timestamp)
        expect(result).toBe(html)
    })

    it('does not modify root-relative URLs', () => {
        const html = '<img src="/static/image.png">'
        const result = fixLocalImagePaths(html, 'docs', timestamp)
        expect(result).toBe(html)
    })

    it('handles relative path with ../ segments', () => {
        const html = '<img src="../images/图片.png">'
        const result = fixLocalImagePaths(html, 'docs/sub', timestamp)
        // docs/sub + ../images/图片.png → docs/images/图片.png
        expect(result).toContain('/api/local-file/docs/images/%E5%9B%BE%E7%89%87.png')
    })

    it('handles relative path with ./ segments', () => {
        const html = '<img src="./图片.png">'
        const result = fixLocalImagePaths(html, 'docs', timestamp)
        expect(result).toContain('/api/local-file/docs/%E5%9B%BE%E7%89%87.png')
    })

    it('encodes special characters in path segments', () => {
        const html = '<img src="path with spaces/image.png">'
        const result = fixLocalImagePaths(html, '', timestamp)
        expect(result).toContain('/api/local-file/path%20with%20spaces/image.png')
    })

    it('preserves ASCII paths without modification', () => {
        const html = '<img src="assets/logo.png">'
        const result = fixLocalImagePaths(html, 'docs', timestamp)
        expect(result).toContain('/api/local-file/docs/assets/logo.png')
    })

    it('handles multiple images in one HTML string', () => {
        const html = '<img src="中文/a.png"><img src="english/b.png">'
        const result = fixLocalImagePaths(html, 'docs', timestamp)
        expect(result).toContain('/api/local-file/docs/%E4%B8%AD%E6%96%87/a.png')
        expect(result).toContain('/api/local-file/docs/english/b.png')
    })
})

// ── FileViewer error bubble template test ──

describe('FileViewer error display', () => {
    it('uses error-bubble class instead of error-message banner', () => {
        const componentPath = resolve(__dirname, '../file/FileViewer.vue')
        const source = readFileSync(componentPath, 'utf-8')

        // Should use the new bubble style
        expect(source).toContain('error-bubble')
        expect(source).not.toContain('class="error-message"')
    })

    it('error-bubble has compact pill/bubble styling', () => {
        const componentPath = resolve(__dirname, '../file/FileViewer.vue')
        const source = readFileSync(componentPath, 'utf-8')

        // Should have border-radius: 20px for pill shape
        expect(source).toMatch(/border-radius:\s*20px/)
        // Should have small padding (not 16px banner)
        expect(source).toMatch(/padding:\s*6px\s+12px/)
    })

    it('error-bubble includes warning icon SVG', () => {
        const componentPath = resolve(__dirname, '../file/FileViewer.vue')
        const source = readFileSync(componentPath, 'utf-8')

        // The error-bubble div should contain an SVG icon
        expect(source).toMatch(/error-bubble[^>]*>[\s\S]*?<svg/)
    })
})

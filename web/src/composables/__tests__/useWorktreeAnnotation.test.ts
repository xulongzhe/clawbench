import { describe, expect, it, vi, beforeEach } from 'vitest'
import {
    worktreeButtonHtml,
    annotateWorktreePaths,
    verifyWorktreePaths,
    clearWorktreeCache,
    useWorktreeAnnotation,
} from '@/composables/useWorktreeAnnotation'

// Mock escapeHtml to pass through (for asserting HTML structure)
vi.mock('@/utils/html', () => ({
    escapeHtml: (s: string) => s.replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c] || c)),
}))

vi.mock('@/composables/useLocale', () => ({
    gt: (key: string) => key,
}))

// Ensure CSS.escape is available in jsdom
if (typeof (globalThis as any).CSS === 'undefined') {
    ;(globalThis as any).CSS = {}
}
if (typeof (globalThis as any).CSS.escape === 'undefined') {
    ;(globalThis as any).CSS.escape = (s: string) => s.replace(/[!"#$%&'()*+,.\/:;<=>?@[\\\]^`{|}~]/g, '\\$&')
}

const PROJECT_ROOT = '/home/user/project'

// ── worktreeButtonHtml ──

describe('worktreeButtonHtml', () => {
    it('contains chat-worktree-btn class', () => {
        const html = worktreeButtonHtml('/home/user/project/.worktrees/feature-x')
        expect(html).toContain('chat-worktree-btn')
    })

    it('contains data-worktree-path with the absolute path', () => {
        const path = '/home/user/project/.worktrees/feature-x'
        const html = worktreeButtonHtml(path)
        expect(html).toContain(`data-worktree-path="${path}"`)
    })

    it('contains data-file-path when resolvedPath is provided', () => {
        const html = worktreeButtonHtml('/home/user/project/.worktrees/feature-x', '.worktrees/feature-x')
        expect(html).toContain('data-file-path=".worktrees/feature-x"')
    })

    it('does NOT contain data-file-path when resolvedPath is omitted', () => {
        const html = worktreeButtonHtml('/home/user/project/.worktrees/feature-x')
        expect(html).not.toContain('data-file-path')
    })

    it('contains the SVG icon', () => {
        const html = worktreeButtonHtml('/home/user/project/.worktrees/feature-x')
        expect(html).toContain('<svg')
    })

    it('contains title attribute', () => {
        const html = worktreeButtonHtml('/home/user/project/.worktrees/feature-x')
        expect(html).toContain('title=')
    })
})

// ── annotateWorktreePaths ──

describe('annotateWorktreePaths', () => {
    beforeEach(() => {
        clearWorktreeCache()
    })

    it('annotates .worktrees/ path in text node (regex-first, no cache needed)', () => {
        const result = annotateWorktreePaths('<p>.worktrees/feature-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-btn')
    })

    it('annotates ./.worktrees/ path in text node', () => {
        const result = annotateWorktreePaths('<p>./.worktrees/feature-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-path')
    })

    it('annotates absolute path in text node', () => {
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<p>${absPath}</p>`, { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain(absPath)
        expect(result.html).toContain('chat-worktree-path')
    })

    it('appends button after <a> tag with worktree href', () => {
        const result = annotateWorktreePaths('<a href=".worktrees/feature-x">link</a>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-btn')
    })

    it('adds class + button to <code> tag with worktree content', () => {
        const result = annotateWorktreePaths('<code>.worktrees/feature-x</code>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-btn')
    })

    it('does NOT annotate paths inside <pre> blocks', () => {
        const result = annotateWorktreePaths('<pre><code>.worktrees/feature-x</code></pre>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
        expect(result.html).not.toContain('chat-worktree-path')
    })

    it('does NOT double-annotate paths inside <a> tags in text node step', () => {
        const result = annotateWorktreePaths('<a href=".worktrees/feature-x">.worktrees/feature-x</a>', { projectRoot: PROJECT_ROOT })
        const path = '/home/user/project/.worktrees/feature-x'
        // Should have exactly one detected path (from the <a> tag step, text inside <a> skipped)
        const count = result.detectedWorktreePaths.filter(p => p === path).length
        expect(count).toBe(1)
    })

    it('does NOT annotate non-worktree paths', () => {
        const result = annotateWorktreePaths('<p>src/main.go</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
        expect(result.html).not.toContain('chat-worktree-path')
    })

    it('annotates multiple worktree paths in one text node', () => {
        const result = annotateWorktreePaths('<p>.worktrees/feature-x and .worktrees/bugfix-y</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/bugfix-y')
    })

    it('returns empty result for empty input', () => {
        const result = annotateWorktreePaths('', { projectRoot: PROJECT_ROOT })
        expect(result.html).toBe('')
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('sets data-worktree-path attribute with absolute path', () => {
        const result = annotateWorktreePaths('<code>.worktrees/feature-x</code>', { projectRoot: PROJECT_ROOT })
        expect(result.html).toContain('data-worktree-path="/home/user/project/.worktrees/feature-x"')
    })

    it('skips <a> tags with non-worktree href', () => {
        const result = annotateWorktreePaths('<a href="https://example.com">link</a>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
        expect(result.html).not.toContain('chat-worktree-btn')
    })

    it('skips <code> with chat-commit-hash class', () => {
        const result = annotateWorktreePaths('<code class="chat-commit-hash">.worktrees/feature-x</code>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('skips <code> with non-worktree text content', () => {
        const result = annotateWorktreePaths('<code>src/main.go</code>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('preserves text before and after worktree path in text node', () => {
        const result = annotateWorktreePaths('<p>Check .worktrees/feature-x for details</p>', { projectRoot: PROJECT_ROOT })
        expect(result.html).toContain('Check')
        expect(result.html).toContain('for details')
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
    })

    it('handles mixed absolute and relative paths in same text node', () => {
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<p>${absPath} and .worktrees/bugfix-y</p>`, { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain(absPath)
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/bugfix-y')
    })

    it('skips text nodes inside elements with chat-commit-hash class', () => {
        const result = annotateWorktreePaths('<span class="chat-commit-hash">.worktrees/feature-x</span>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('skips text nodes inside elements with chat-worktree-path class', () => {
        const result = annotateWorktreePaths('<span class="chat-worktree-path">.worktrees/feature-x</span>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('APPENDS worktree annotation to .chat-file-path elements (coexistence)', () => {
        const result = annotateWorktreePaths('<span class="chat-file-path" data-file-path=".worktrees/feature-x">.worktrees/feature-x</span>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-btn')
        expect(result.html).toContain('chat-file-path')
    })

    it('annotates without cache (regex-first design — works on first render)', () => {
        // No cache seeding needed — regex matches .worktrees/ pattern directly
        const result = annotateWorktreePaths('<p>.worktrees/feature-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths.length).toBeGreaterThan(0)
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-btn')
    })

    it('skips <code> that already has chat-worktree-path class (already annotated)', () => {
        const result = annotateWorktreePaths('<code class="chat-worktree-path" data-worktree-path="/home/user/project/.worktrees/feature-x">.worktrees/feature-x</code>', { projectRoot: PROJECT_ROOT })
        // Should not add another button (no double annotation)
        const btnCount = (result.html.match(/chat-worktree-btn/g) || []).length
        expect(btnCount).toBe(0)
    })

    it('skips <code> inside <pre> with chat-file-path (worktree coexistence in pre)', () => {
        const result = annotateWorktreePaths('<pre><code class="chat-file-path">.worktrees/feature-x</code></pre>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('annotates full absolute worktree path in text node', () => {
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<p>${absPath}</p>`, { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain(absPath)
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-btn')
        expect(result.html).toContain('data-worktree-path="/home/user/project/.worktrees/feature-x"')
    })

    it('annotates full absolute worktree path in <code> tag', () => {
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<code>${absPath}</code>`, { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain(absPath)
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-btn')
    })

    it('single button with data-worktree-path and data-file-path for path under projectRoot', () => {
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<p>${absPath}</p>`, { projectRoot: PROJECT_ROOT })
        expect(result.html).toContain('chat-worktree-btn')
        expect(result.html).toContain('data-worktree-path="/home/user/project/.worktrees/feature-x"')
        expect(result.html).toContain('data-file-path=".worktrees/feature-x"')
    })

    it('does NOT add data-file-path for path outside projectRoot', () => {
        // Absolute path that is not under projectRoot won't match regex
        const result = annotateWorktreePaths('<p>/other/location/worktree-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('does NOT annotate worktree-like path not under projectRoot', () => {
        // .worktrees/ path always resolves under projectRoot, so this always works
        // But /other/.worktrees/x won't match because it's not under projectRoot
        const result = annotateWorktreePaths('<p>/other/path/.worktrees/x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })
})

// ── verifyWorktreePaths ──

describe('verifyWorktreePaths', () => {
    beforeEach(() => {
        clearWorktreeCache()
    })

    it('removes annotations for paths not in worktree list', async () => {
        const mockFetch = vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ worktrees: [
                { path: '/home/user/project/.worktrees/real-wt', displayPath: './.worktrees/real-wt', branch: 'real-wt', isCurrent: false, dirty: false, changeCount: 0, untrackedCount: 0, locked: false, missing: false },
                { path: '/home/user/project', displayPath: '.', branch: 'main', isCurrent: true, dirty: false, changeCount: 0, untrackedCount: 0, locked: false, missing: false },
            ] }),
        })
        vi.stubGlobal('fetch', mockFetch)

        const container = document.createElement('div')
        container.innerHTML = '<span class="chat-worktree-path" data-worktree-path="/home/user/project/.worktrees/fake-wt">.worktrees/fake-wt</span><button class="chat-worktree-btn" data-worktree-path="/home/user/project/.worktrees/fake-wt"></button>'

        await verifyWorktreePaths(['/home/user/project/.worktrees/fake-wt'], container, PROJECT_ROOT)

        // The fake worktree's span and button should be removed
        expect(container.querySelector('.chat-worktree-btn')).toBeNull()
        expect(container.querySelector('.chat-worktree-path')).toBeNull()
        // The text content should remain (unwrapped from span)
        expect(container.textContent).toContain('.worktrees/fake-wt')

        vi.unstubAllGlobals()
    })

    it('keeps annotations for paths in worktree list', async () => {
        const mockFetch = vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ worktrees: [
                { path: '/home/user/project/.worktrees/real-wt', displayPath: './.worktrees/real-wt', branch: 'real-wt', isCurrent: false, dirty: false, changeCount: 0, untrackedCount: 0, locked: false, missing: false },
                { path: '/home/user/project', displayPath: '.', branch: 'main', isCurrent: true, dirty: false, changeCount: 0, untrackedCount: 0, locked: false, missing: false },
            ] }),
        })
        vi.stubGlobal('fetch', mockFetch)

        const container = document.createElement('div')
        container.innerHTML = '<span class="chat-worktree-path" data-worktree-path="/home/user/project/.worktrees/real-wt">.worktrees/real-wt</span><button class="chat-worktree-btn" data-worktree-path="/home/user/project/.worktrees/real-wt"></button>'

        await verifyWorktreePaths(['/home/user/project/.worktrees/real-wt'], container, PROJECT_ROOT)

        // The real worktree's span and button should remain
        expect(container.querySelector('.chat-worktree-btn')).not.toBeNull()
        expect(container.querySelector('.chat-worktree-path')).not.toBeNull()

        vi.unstubAllGlobals()
    })
})

// ── useWorktreeAnnotation composable ──

describe('useWorktreeAnnotation', () => {
    it('returns annotateWorktreePaths, verifyWorktreePaths and clearWorktreeCache', () => {
        const { annotateWorktreePaths: annotate, verifyWorktreePaths: verify, clearWorktreeCache: clear } = useWorktreeAnnotation()
        expect(typeof annotate).toBe('function')
        expect(typeof verify).toBe('function')
        expect(typeof clear).toBe('function')
    })
})

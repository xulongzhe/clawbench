import { describe, expect, it, vi, beforeEach } from 'vitest'
import {
    worktreeSwitchButtonHtml,
    buildSearchEntries,
    annotateWorktreePaths,
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

const MOCK_WORKTREES = [
    {
        path: '/home/user/project',
        displayPath: '.',
        branch: 'main',
        isCurrent: true,
        dirty: false,
        changeCount: 0,
        untrackedCount: 0,
        locked: false,
        missing: false,
    },
    {
        path: '/home/user/project/.worktrees/feature-x',
        displayPath: './.worktrees/feature-x',
        branch: 'feature-x',
        isCurrent: false,
        dirty: false,
        changeCount: 0,
        untrackedCount: 0,
        locked: false,
        missing: false,
    },
    {
        path: '/home/user/project/.worktrees/bugfix-y',
        displayPath: './.worktrees/bugfix-y',
        branch: 'bugfix-y',
        isCurrent: false,
        dirty: true,
        changeCount: 3,
        untrackedCount: 1,
        locked: false,
        missing: false,
    },
]

/**
 * Helper: seed the worktree list cache so annotateWorktreePaths can use it.
 * We mock fetch to populate the cache, then call annotateWorktreePaths
 * which will find the cached data.
 */
async function seedCache(projectRoot: string, worktrees: any[]) {
    const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ worktrees }),
    })
    vi.stubGlobal('fetch', mockFetch)
    // First call triggers fetch and caches; annotateWorktreePaths returns empty
    annotateWorktreePaths('<p>test</p>', { projectRoot })
    // Wait for async fetch to complete
    await vi.waitFor(() => expect(mockFetch).toHaveBeenCalled())
    vi.unstubAllGlobals()
}

// ── buildSearchEntries ──

describe('buildSearchEntries', () => {
    it('excludes isCurrent worktree', () => {
        const entries = buildSearchEntries(MOCK_WORKTREES)
        expect(entries.every(e => e.absPath !== '/home/user/project')).toBe(true)
    })

    it('generates absolute path entries', () => {
        const entries = buildSearchEntries(MOCK_WORKTREES)
        expect(entries.some(e => e.keyword === '/home/user/project/.worktrees/feature-x')).toBe(true)
    })

    it('generates displayPath entries', () => {
        const entries = buildSearchEntries(MOCK_WORKTREES)
        expect(entries.some(e => e.keyword === './.worktrees/feature-x')).toBe(true)
    })

    it('generates displayPath without ./ prefix entries', () => {
        const entries = buildSearchEntries(MOCK_WORKTREES)
        expect(entries.some(e => e.keyword === '.worktrees/feature-x')).toBe(true)
    })

    it('sorts entries by keyword length descending', () => {
        const entries = buildSearchEntries(MOCK_WORKTREES)
        for (let i = 1; i < entries.length; i++) {
            expect(entries[i - 1].keyword.length).toBeGreaterThanOrEqual(entries[i].keyword.length)
        }
    })

    it('skips displayPath that is just "."', () => {
        const entries = buildSearchEntries(MOCK_WORKTREES)
        expect(entries.some(e => e.keyword === '.')).toBe(false)
    })

    it('returns empty for empty worktree list', () => {
        const entries = buildSearchEntries([])
        expect(entries).toEqual([])
    })

    it('all entries have correct absPath', () => {
        const entries = buildSearchEntries(MOCK_WORKTREES)
        for (const entry of entries) {
            expect(entry.absPath).toMatch(/^\/home\/user\/project\/\.worktrees\//)
        }
    })
})

// ── worktreeSwitchButtonHtml ──

describe('worktreeSwitchButtonHtml', () => {
    it('contains chat-worktree-switch-btn class', () => {
        const html = worktreeSwitchButtonHtml('/home/user/project/.worktrees/feature-x')
        expect(html).toContain('chat-worktree-switch-btn')
    })

    it('contains data-worktree-path with the absolute path', () => {
        const path = '/home/user/project/.worktrees/feature-x'
        const html = worktreeSwitchButtonHtml(path)
        expect(html).toContain(`data-worktree-path="${path}"`)
    })

    it('contains the SVG icon', () => {
        const html = worktreeSwitchButtonHtml('/home/user/project/.worktrees/feature-x')
        expect(html).toContain('<svg')
    })

    it('contains title attribute', () => {
        const html = worktreeSwitchButtonHtml('/home/user/project/.worktrees/feature-x')
        expect(html).toContain('title=')
    })
})

// ── annotateWorktreePaths ──

describe('annotateWorktreePaths', () => {
    beforeEach(() => {
        clearWorktreeCache()
    })

    it('annotates .worktrees/ path in text node (via worktree list)', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<p>.worktrees/feature-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-switch-btn')
    })

    it('annotates ./.worktrees/ path in text node', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<p>./.worktrees/feature-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-path')
    })

    it('annotates absolute path in text node', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<p>${absPath}</p>`, { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain(absPath)
        expect(result.html).toContain('chat-worktree-path')
    })

    it('appends button after <a> tag with worktree href', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<a href=".worktrees/feature-x">link</a>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-switch-btn')
    })

    it('adds class + button to <code> tag with worktree content', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<code>.worktrees/feature-x</code>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-switch-btn')
    })

    it('does NOT annotate paths inside <pre> blocks', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<pre><code>.worktrees/feature-x</code></pre>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
        expect(result.html).not.toContain('chat-worktree-path')
    })

    it('does NOT double-annotate paths inside <a> tags in text node step', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<a href=".worktrees/feature-x">.worktrees/feature-x</a>', { projectRoot: PROJECT_ROOT })
        const path = '/home/user/project/.worktrees/feature-x'
        // Should have exactly one detected path (from the <a> tag step, text inside <a> skipped)
        const count = result.detectedWorktreePaths.filter(p => p === path).length
        expect(count).toBe(1)
    })

    it('does NOT annotate non-worktree paths', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<p>src/main.go</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
        expect(result.html).not.toContain('chat-worktree-path')
    })

    it('annotates multiple worktree paths in one text node', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<p>.worktrees/feature-x and .worktrees/bugfix-y</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/bugfix-y')
    })

    it('returns empty result for empty input', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('', { projectRoot: PROJECT_ROOT })
        expect(result.html).toBe('')
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('sets data-worktree-path attribute with absolute path', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<code>.worktrees/feature-x</code>', { projectRoot: PROJECT_ROOT })
        expect(result.html).toContain('data-worktree-path="/home/user/project/.worktrees/feature-x"')
    })

    it('skips <a> tags with non-worktree href', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<a href="https://example.com">link</a>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
        expect(result.html).not.toContain('chat-worktree-switch-btn')
    })

    it('skips <code> with chat-commit-hash class', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<code class="chat-commit-hash">.worktrees/feature-x</code>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('skips <code> with non-worktree text content', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<code>src/main.go</code>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('preserves text before and after worktree path in text node', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<p>Check .worktrees/feature-x for details</p>', { projectRoot: PROJECT_ROOT })
        expect(result.html).toContain('Check')
        expect(result.html).toContain('for details')
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
    })

    it('handles mixed absolute and relative paths in same text node', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<p>${absPath} and .worktrees/bugfix-y</p>`, { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain(absPath)
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/bugfix-y')
    })

    it('skips text nodes inside elements with chat-commit-hash class', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<span class="chat-commit-hash">.worktrees/feature-x</span>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('skips text nodes inside elements with chat-worktree-path class', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<span class="chat-worktree-path">.worktrees/feature-x</span>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('APPENDS worktree annotation to .chat-file-path elements (coexistence)', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        // Simulate what useFilePathAnnotation would produce: a span with chat-file-path
        const result = annotateWorktreePaths('<span class="chat-file-path" data-file-path=".worktrees/feature-x">.worktrees/feature-x</span>', { projectRoot: PROJECT_ROOT })
        // Should detect and annotate (coexistence)
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-switch-btn')
        // The original chat-file-path should still be there
        expect(result.html).toContain('chat-file-path')
    })

    it('does NOT annotate when cache is empty (triggers background fetch)', () => {
        // No seedCache call — cache is empty
        const result = annotateWorktreePaths('<p>.worktrees/feature-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
        // The HTML should be returned as-is (no annotation)
        expect(result.html).not.toContain('chat-worktree-path')
    })

    it('does NOT annotate when all worktrees are current (no search entries)', async () => {
        const onlyCurrent = [
            {
                path: '/home/user/project',
                displayPath: '.',
                branch: 'main',
                isCurrent: true,
                dirty: false,
                changeCount: 0,
                untrackedCount: 0,
                locked: false,
                missing: false,
            },
        ]
        await seedCache(PROJECT_ROOT, onlyCurrent)
        const result = annotateWorktreePaths('<p>.worktrees/feature-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('does not annotate paths that are not in the worktree list', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<p>.worktrees/nonexistent-wt</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
        expect(result.html).not.toContain('chat-worktree-path')
    })

    it('uses cached worktree list on subsequent calls (fetchWorktreeList cache hit)', async () => {
        // Seed cache using our helper (which waits for fetch to complete and cache to populate)
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        // Now make a second call — should use cache without fetching again
        const mockFetch = vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ worktrees: [] }),
        })
        vi.stubGlobal('fetch', mockFetch)
        const result = annotateWorktreePaths('<p>.worktrees/feature-x</p>', { projectRoot: PROJECT_ROOT })
        // fetch should NOT be called again (cache hit)
        expect(mockFetch).not.toHaveBeenCalled()
        expect(result.detectedWorktreePaths).toContain('/home/user/project/.worktrees/feature-x')
        vi.unstubAllGlobals()
    })

    it('handles fetch returning non-ok response', async () => {
        const mockFetch = vi.fn().mockResolvedValue({ ok: false, status: 500 })
        vi.stubGlobal('fetch', mockFetch)
        // First call triggers fetch
        annotateWorktreePaths('<p>test</p>', { projectRoot: PROJECT_ROOT })
        await vi.waitFor(() => expect(mockFetch).toHaveBeenCalled())
        // The cache should remain empty (fetch failed), so second call returns empty
        const result = annotateWorktreePaths('<p>.worktrees/feature-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
        vi.unstubAllGlobals()
    })

    it('skips <code> that already has chat-worktree-path class (already annotated)', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        // An element that already has the worktree-path class should not be double-annotated
        const result = annotateWorktreePaths('<code class="chat-worktree-path" data-worktree-path="/home/user/project/.worktrees/feature-x">.worktrees/feature-x</code>', { projectRoot: PROJECT_ROOT })
        // Should not add another button (no double annotation)
        const btnCount = (result.html.match(/chat-worktree-switch-btn/g) || []).length
        expect(btnCount).toBe(0) // no new button added since it's already annotated
    })

    it('skips <code> inside <pre> with chat-file-path (worktree coexistence in pre)', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<pre><code class="chat-file-path">.worktrees/feature-x</code></pre>', { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toEqual([])
    })

    it('annotates full absolute worktree path in text node', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<p>${absPath}</p>`, { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain(absPath)
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-switch-btn')
        // The full absolute path should be inside the span, not just a partial match
        expect(result.html).toContain('data-worktree-path="/home/user/project/.worktrees/feature-x"')
    })

    it('annotates full absolute worktree path in <code> tag', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<code>${absPath}</code>`, { projectRoot: PROJECT_ROOT })
        expect(result.detectedWorktreePaths).toContain(absPath)
        expect(result.html).toContain('chat-worktree-path')
        expect(result.html).toContain('chat-worktree-switch-btn')
    })

    it('adds file-open button alongside worktree switch button for absolute path', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<p>${absPath}</p>`, { projectRoot: PROJECT_ROOT })
        // Both buttons should be present
        expect(result.html).toContain('chat-worktree-switch-btn')
        expect(result.html).toContain('chat-file-open-btn')
        // The span should have both data attributes
        expect(result.html).toContain('data-worktree-path="/home/user/project/.worktrees/feature-x"')
        expect(result.html).toContain('data-file-path=".worktrees/feature-x"')
    })

    it('adds file-open button alongside worktree switch button for <code> element', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const absPath = '/home/user/project/.worktrees/feature-x'
        const result = annotateWorktreePaths(`<code>${absPath}</code>`, { projectRoot: PROJECT_ROOT })
        expect(result.html).toContain('chat-worktree-switch-btn')
        expect(result.html).toContain('chat-file-open-btn')
        expect(result.html).toContain('data-file-path=".worktrees/feature-x"')
    })

    it('adds file-open button alongside worktree switch button for <a> element', async () => {
        await seedCache(PROJECT_ROOT, MOCK_WORKTREES)
        const result = annotateWorktreePaths('<a href=".worktrees/feature-x">link</a>', { projectRoot: PROJECT_ROOT })
        expect(result.html).toContain('chat-worktree-switch-btn')
        expect(result.html).toContain('chat-file-open-btn')
    })

    it('does NOT add file-open button for worktree path outside projectRoot', async () => {
        // If a worktree path is not under projectRoot, only worktree switch button is added
        const externalWorktrees = [
            {
                path: '/home/user/project',
                displayPath: '.',
                branch: 'main',
                isCurrent: true,
                dirty: false,
                changeCount: 0,
                untrackedCount: 0,
                locked: false,
                missing: false,
            },
            {
                path: '/other/location/worktree-x',
                displayPath: './worktree-x',
                branch: 'worktree-x',
                isCurrent: false,
                dirty: false,
                changeCount: 0,
                untrackedCount: 0,
                locked: false,
                missing: false,
            },
        ]
        await seedCache(PROJECT_ROOT, externalWorktrees)
        const result = annotateWorktreePaths('<p>/other/location/worktree-x</p>', { projectRoot: PROJECT_ROOT })
        expect(result.html).toContain('chat-worktree-switch-btn')
        // No file-open button because /other/location/worktree-x is outside projectRoot
        expect(result.html).not.toContain('chat-file-open-btn')
    })
})

// ── useWorktreeAnnotation composable ──

describe('useWorktreeAnnotation', () => {
    it('returns annotateWorktreePaths and clearWorktreeCache', () => {
        const { annotateWorktreePaths: annotate, clearWorktreeCache: clear } = useWorktreeAnnotation()
        expect(typeof annotate).toBe('function')
        expect(typeof clear).toBe('function')
    })
})

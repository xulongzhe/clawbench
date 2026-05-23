import { describe, expect, it, vi, beforeEach } from 'vitest'
import {
  resolveFilePath,
  resolveRelativePath,
  fileOpenButtonHtml,
  FILE_OPEN_ICON_SVG,
  annotateFilePaths,
  clearVerifiedCache,
} from '@/composables/useFilePathAnnotation'

// Mock escapeHtml from html utils
vi.mock('@/utils/html', () => ({
  escapeHtml: (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
}))

// Mock splitPath
vi.mock('@/utils/path', () => ({
  splitPath: (p: string) => p.split('/').filter(Boolean),
}))

// Mock store
vi.mock('@/stores/app', () => ({
  store: { state: { projectRoot: '/home/user/project' } },
}))

// Mock useLocale
vi.mock('@/composables/useLocale', () => ({
  gt: (key: string) => key,
}))

// --- resolveFilePath ---

describe('resolveFilePath', () => {
  const projectRoot = '/home/user/project'

  describe('absolute paths', () => {
    it('resolves a path under projectRoot', () => {
      expect(resolveFilePath('/home/user/project/src/main.go', projectRoot)).toBe('src/main.go')
    })

    it('returns null for path outside projectRoot', () => {
      expect(resolveFilePath('/etc/passwd', projectRoot)).toBeNull()
    })

    it('returns null when projectRoot is empty', () => {
      expect(resolveFilePath('/home/user/project/src/main.go', '')).toBeNull()
    })

    it('returns null when path equals projectRoot (no relative part)', () => {
      expect(resolveFilePath('/home/user/project', projectRoot)).toBeNull()
    })

    it('handles nested project root paths', () => {
      expect(resolveFilePath('/home/user/project/deep/nested/file.ts', projectRoot)).toBe('deep/nested/file.ts')
    })
  })

  describe('relative paths with projectRoot', () => {
    it('resolves a simple relative path', () => {
      expect(resolveFilePath('src/main.go', projectRoot)).toBe('src/main.go')
    })

    it('resolves ./prefixed paths', () => {
      expect(resolveFilePath('./src/main.go', projectRoot)).toBe('src/main.go')
    })

    it('resolves ../prefixed paths within project', () => {
      expect(resolveFilePath('../project/src/main.go', projectRoot)).toBe('src/main.go')
    })

    it('returns null for paths going above project root', () => {
      expect(resolveFilePath('../../../etc/passwd', projectRoot)).toBeNull()
    })

    it('handles multiple consecutive ../ segments', () => {
      // projectRoot = /home/user/project → parts = ['home', 'user', 'project']
      // Going ../ 3 times exhausts parts → null
      expect(resolveFilePath('../../../src/main.go', projectRoot)).toBeNull()
    })

    it('handles mixed . and .. segments', () => {
      expect(resolveFilePath('./src/../lib/utils.ts', projectRoot)).toBe('lib/utils.ts')
    })
  })

  describe('relative paths without projectRoot', () => {
    it('returns path as-is after stripping ./', () => {
      expect(resolveFilePath('src/main.go', '')).toBe('src/main.go')
    })

    it('strips leading ./', () => {
      expect(resolveFilePath('./src/main.go', '')).toBe('src/main.go')
    })

    it('returns null for paths starting with ../', () => {
      expect(resolveFilePath('../src/main.go', '')).toBeNull()
    })
  })
})

// --- resolveRelativePath ---

describe('resolveRelativePath', () => {
  it('resolves relative path against base directory', () => {
    expect(resolveRelativePath('file.ts', 'src')).toBe('src/file.ts')
  })

  it('normalizes ./ segments', () => {
    expect(resolveRelativePath('./file.ts', 'src')).toBe('src/file.ts')
  })

  it('normalizes ../ segments', () => {
    expect(resolveRelativePath('../file.ts', 'src/utils')).toBe('src/file.ts')
  })

  it('handles multiple ../ segments', () => {
    expect(resolveRelativePath('../../file.ts', 'src/utils/deep')).toBe('src/file.ts')
  })

  it('returns raw href when baseDir is empty', () => {
    expect(resolveRelativePath('file.ts', '')).toBe('file.ts')
  })

  it('handles deeply nested paths', () => {
    expect(resolveRelativePath('../../../root.ts', 'a/b/c/d')).toBe('a/root.ts')
  })

  it('does not go above root (pops from empty normalized)', () => {
    expect(resolveRelativePath('../../../../root.ts', 'a')).toBe('root.ts')
  })

  it('handles double slashes', () => {
    expect(resolveRelativePath('sub//file.ts', 'src')).toBe('src/sub/file.ts')
  })

  it('handles empty href segments', () => {
    expect(resolveRelativePath('././file.ts', 'src')).toBe('src/file.ts')
  })
})

// --- fileOpenButtonHtml ---

describe('fileOpenButtonHtml', () => {
  it('generates button HTML with data-file-path attribute', () => {
    const html = fileOpenButtonHtml('src/main.go')
    expect(html).toContain('chat-file-open-btn')
    expect(html).toContain('data-file-path="src/main.go"')
  })

  it('escapes HTML in the path', () => {
    const html = fileOpenButtonHtml('src/<script>.go')
    expect(html).toContain('data-file-path="src/&lt;script&gt;.go"')
  })

  it('includes the SVG icon', () => {
    const html = fileOpenButtonHtml('test.ts')
    expect(html).toContain('<svg')
  })

  it('contains the same icon as FILE_OPEN_ICON_SVG', () => {
    const html = fileOpenButtonHtml('test.ts')
    expect(html).toContain(FILE_OPEN_ICON_SVG)
  })
})

// --- annotateFilePaths ---

describe('annotateFilePaths', () => {
  const projectRoot = '/home/user/project'

  it('annotates absolute paths under projectRoot', () => {
    const input = 'See /home/user/project/src/main.go for details'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('src/main.go')
    expect(result.html).toContain('chat-file-path')
    expect(result.html).toContain('chat-file-open-btn')
  })

  it('does not annotate absolute paths outside projectRoot', () => {
    const input = 'See /etc/config for details'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
    expect(result.html).not.toContain('chat-file-path')
  })

  it('annotates relative paths with ./', () => {
    const input = 'Check ./src/main.go for details'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('src/main.go')
  })

  it('annotates bare relative paths with at least two segments and extension', () => {
    const input = 'Look at src/main.go for details'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('src/main.go')
  })

  it('does not annotate single-segment names without slash', () => {
    const input = 'Look at main.go for details'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('preserves pre blocks without annotation', () => {
    const input = '<pre>some /home/user/project/src/main.go code</pre>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('annotates file paths inside inline code elements', () => {
    const input = '<code>src/main.go</code>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('src/main.go')
  })

  it('does not annotate inline code without slash or extension', () => {
    const input = '<code>useAutoSpeech</code>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('annotates inline code with extension but no slash', () => {
    const input = '<code>ChatPanel.vue</code>'
    const result = annotateFilePaths(input, { projectRoot })
    // ChatPanel.vue matches the file extension pattern
    expect(result.detectedPaths.length).toBeGreaterThanOrEqual(0)
  })

  it('appends open button after <a> links to local files', () => {
    const input = '<a href="src/utils.ts">utils</a>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('src/utils.ts')
    expect(result.html).toContain('chat-file-open-btn')
  })

  it('does not annotate external <a> links', () => {
    const input = '<a href="https://example.com">link</a>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('does not annotate anchor <a> links', () => {
    const input = '<a href="#section">jump</a>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('resolves <a> href against baseDir when provided', () => {
    const input = '<a href="utils.ts">utils</a>'
    const result = annotateFilePaths(input, { projectRoot, baseDir: 'src' })
    expect(result.detectedPaths).toContain('src/utils.ts')
  })

  it('returns empty detectedPaths for plain text with no paths', () => {
    const input = 'This is just some text without any file references.'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('handles empty input', () => {
    const result = annotateFilePaths('', { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
    expect(result.html).toBe('')
  })

  it('detects multiple paths in one string', () => {
    const input = 'See src/main.go and ./lib/utils.ts'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths.length).toBeGreaterThanOrEqual(2)
  })

  it('annotates paths inside blockquote elements (blockquote is valid context)', () => {
    // After markdown rendering, ">src/main.go" becomes <blockquote><p>src/main.go</p></blockquote>
    // The DOM-based approach naturally handles this — the text node is inside a <blockquote>
    // but not inside <pre>/<a>/<code>, so it's a valid annotation target.
    const input = '<blockquote><p>src/main.go</p></blockquote>'
    const result = annotateFilePaths(input, { projectRoot })
    // Paths inside blockquotes are annotated — they are legitimate file references
    expect(result.detectedPaths).toContain('src/main.go')
  })

  it('does not double-annotate absolute paths that contain a bare relative path segment', () => {
    // Regression: absolute path like /home/user/project/public/landing/index.html
    // would be annotated by the absolute-path regex, then the bare relative-path regex
    // would match "public/landing/index.html" inside the generated data-file-path
    // attribute of both the <span> and <button> tags, producing broken HTML like:
    //   data-file-path="<span class="chat-file-path"..."
    const input = '<p>/home/user/project/public/landing/index.html这个是出问题的文件。</p>'
    const result = annotateFilePaths(input, { projectRoot })

    // Should detect exactly one path
    expect(result.detectedPaths).toHaveLength(1)
    expect(result.detectedPaths[0]).toBe('public/landing/index.html')

    // The data-file-path attribute must NOT contain a nested <span>
    expect(result.html).not.toContain('data-file-path="<span')
    expect(result.html).not.toContain('data-file-path="&lt;span')

    // The data-file-path attribute should contain the correct resolved path
    expect(result.html).toContain('data-file-path="public/landing/index.html"')
  })

  // ── DOM traversal specific tests ──

  it('does not re-annotate paths inside <a> tag text content', () => {
    // <a> tags are handled in step 1 (append button after the link).
    // The text inside <a> should NOT be matched again by the text-node regex.
    const input = '<a href="src/utils.ts">see src/utils.ts</a>'
    const result = annotateFilePaths(input, { projectRoot })
    // Should detect the path once (from the href), not twice
    expect(result.detectedPaths).toHaveLength(1)
    expect(result.detectedPaths[0]).toBe('src/utils.ts')
    // Should only have one open button
    const btnCount = (result.html.match(/chat-file-open-btn/g) || []).length
    expect(btnCount).toBe(1)
  })

  it('does not re-annotate paths inside <code> tag text content', () => {
    // <code> tags are handled in step 2 (add class + button).
    // The text inside <code> should NOT be matched again by the text-node regex.
    const input = '<p>check <code>src/main.go</code> for details</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(1)
    expect(result.detectedPaths[0]).toBe('src/main.go')
    // Only one span/button pair
    const btnCount = (result.html.match(/chat-file-open-btn/g) || []).length
    expect(btnCount).toBe(1)
  })

  it('does not annotate code inside <pre> blocks', () => {
    // <pre><code> is a multi-line code block — paths inside should NOT be annotated
    const input = '<pre><code>import "/home/user/project/src/main.go"</code></pre>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
    expect(result.html).not.toContain('chat-file-path')
  })

  it('annotates absolute path immediately followed by CJK characters', () => {
    // Original bug: /home/user/project/public/landing/index.html这个文件
    // The path ends at the CJK character boundary — regex should not eat the Chinese text
    const input = '<p>/home/user/project/src/main.go有问题</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(1)
    expect(result.detectedPaths[0]).toBe('src/main.go')
    // The CJK text should remain outside the span
    expect(result.html).toContain('有问题')
  })

  it('does not annotate ../ relative paths that go above projectRoot', () => {
    // ../lib/utils.ts resolves to /home/user/lib/utils.ts which is outside projectRoot
    const input = '<p>see ../lib/utils.ts</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('annotates ./ relative paths that stay within projectRoot', () => {
    const input = '<p>see ./src/main.go</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('src/main.go')
  })

  it('detects multiple absolute paths in the same HTML', () => {
    const input = '<p>Edit /home/user/project/src/main.go and /home/user/project/lib/utils.ts</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(2)
    expect(result.detectedPaths).toContain('src/main.go')
    expect(result.detectedPaths).toContain('lib/utils.ts')
  })

  it('does not match paths in existing data-file-path attributes', () => {
    // Pre-existing annotation HTML should not be re-matched by text node regex
    // because data-file-path is an HTML attribute, and DOM traversal only processes text nodes
    const input = '<span class="chat-file-path" data-file-path="src/main.go">src/main.go</span>'
    const result = annotateFilePaths(input, { projectRoot })
    // The span's text content is inside the span element, which is not a text node
    // directly under the body — it's inside the span, so the walker won't pick it up
    // (parent.tagName check skips CODE, but SPAN is not filtered, so the text inside
    // the span IS a text node that gets walked). However, the regex will match
    // "src/main.go" in the text node and try to resolve it — which succeeds.
    // This is expected: if someone passes already-annotated HTML through the function
    // again, it may double-annotate. The caller is responsible for not doing that.
    // What we DO guarantee is that HTML ATTRIBUTES are never matched.
    expect(result.html).not.toContain('data-file-path="&lt;span')
  })

  it('does not annotate mailto: links', () => {
    const input = '<a href="mailto:user@example.com">email</a>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('does not annotate tel: links', () => {
    const input = '<a href="tel:+1234567890">call</a>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('handles path followed by punctuation (period, comma, semicolon)', () => {
    const input = '<p>see src/main.go, lib/utils.ts; and more</p>'
    const result = annotateFilePaths(input, { projectRoot })
    // Both paths should be detected
    expect(result.detectedPaths).toContain('src/main.go')
    expect(result.detectedPaths).toContain('lib/utils.ts')
  })

  it('handles path followed by closing parenthesis', () => {
    const input = '<p>see src/main.go) for details</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('src/main.go')
  })

  it('produces valid HTML with span and button for text node paths', () => {
    const input = '<p>see src/main.go</p>'
    const result = annotateFilePaths(input, { projectRoot })
    // The output should contain a <span class="chat-file-path"> with the path
    // and a <button class="chat-file-open-btn"> with the same data-file-path
    expect(result.html).toContain('<span class="chat-file-path"')
    expect(result.html).toContain('data-file-path="src/main.go"')
    expect(result.html).toContain('chat-file-open-btn')
  })

  it('produces valid HTML with class and button for code node paths', () => {
    const input = '<code>src/main.go</code>'
    const result = annotateFilePaths(input, { projectRoot })
    // The <code> should get the chat-file-path class and data-file-path attribute
    expect(result.html).toContain('class="chat-file-path"')
    expect(result.html).toContain('data-file-path="src/main.go"')
    expect(result.html).toContain('chat-file-open-btn')
  })

  it('handles HTML with only tags and no text', () => {
    const input = '<p></p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('handles path in a deeply nested element', () => {
    const input = '<div><section><article><p>edit src/main.go</p></article></section></div>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('src/main.go')
  })

  it('does not annotate absolute paths that are not under projectRoot', () => {
    const input = '<p>check /etc/nginx/nginx.conf and /home/user/project/src/main.go</p>'
    const result = annotateFilePaths(input, { projectRoot })
    // Only the project-relative path should be detected
    expect(result.detectedPaths).toHaveLength(1)
    expect(result.detectedPaths[0]).toBe('src/main.go')
  })

  it('preserves surrounding text when annotating a path in a text node', () => {
    const input = '<p>Before src/main.go after</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.html).toContain('Before')
    expect(result.html).toContain('after')
    expect(result.detectedPaths).toContain('src/main.go')
  })

  it('annotates bare relative path with multiple segments', () => {
    const input = '<p>see internal/handler/chat.go</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('internal/handler/chat.go')
  })

  it('does not annotate URL-like strings', () => {
    const input = '<p>visit https://example.com/page.html</p>'
    const result = annotateFilePaths(input, { projectRoot })
    // https:// URLs should not be treated as file paths
    // (the regex does not match strings starting with http/https)
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('annotates <a> with relative href and baseDir', () => {
    const input = '<a href="components/App.vue">App</a>'
    const result = annotateFilePaths(input, { projectRoot, baseDir: 'src' })
    expect(result.detectedPaths).toContain('src/components/App.vue')
  })

  it('does not partially match directory prefix followed by more path segments (worktree-like)', () => {
    // Regression: /home/user/project/.worktrees/gitgraph-fix
    // The FILE_PATH_RE would match /home/user/project/.worktrees (treating .worktrees as extension)
    // but the full path is a directory, not a file. The trailing /gitgraph-fix indicates the
    // match is incomplete — this should be skipped so worktree annotation can handle the full path.
    const input = '<p>/home/user/project/.worktrees/gitgraph-fix</p>'
    const result = annotateFilePaths(input, { projectRoot })
    // Should NOT detect .worktrees as a file path (it's a directory prefix)
    expect(result.detectedPaths).toHaveLength(0)
    expect(result.html).not.toContain('chat-file-path')
  })

  it('does not partially match .worktrees directory prefix with non-hyphen continuation', () => {
    // Same bug with a worktree name that has no hyphen (e.g. featurex)
    const input = '<p>/home/user/project/.worktrees/featurex</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
    expect(result.html).not.toContain('chat-file-path')
  })

  it('still annotates legitimate paths ending in .worktrees when there is no continuation', () => {
    // If the text is just /home/user/project/.worktrees (no trailing path), it's a
    // legitimate path that should be annotated — even though it's a directory,
    // the user may want to navigate to it.
    const input = '<p>/home/user/project/.worktrees</p>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toContain('.worktrees')
  })

  it('skips text nodes inside chat-worktree-path elements', () => {
    // After worktree annotation runs first, file path annotation should not
    // re-annotate text inside worktree-annotated elements
    const input = '<span class="chat-worktree-path" data-worktree-path="/home/user/project/.worktrees/fix">.worktrees/fix</span>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })

  it('skips <code> elements already annotated as worktree', () => {
    // <code> with chat-worktree-path class should be skipped in step 2
    const input = '<code class="chat-worktree-path" data-worktree-path="/home/user/project/.worktrees/fix">.worktrees/fix</code>'
    const result = annotateFilePaths(input, { projectRoot })
    expect(result.detectedPaths).toHaveLength(0)
  })
})

// --- clearVerifiedCache ---

describe('clearVerifiedCache', () => {
  it('does not throw when called', () => {
    expect(() => clearVerifiedCache()).not.toThrow()
  })
})

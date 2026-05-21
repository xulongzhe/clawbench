import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ref } from 'vue'

// Mock composables before importing the module
const mockIsAppMode = ref(false)
const mockSshInfo = ref<any>(null)

vi.mock('@/composables/useAppMode', () => ({
  useAppMode: () => ({ isAppMode: mockIsAppMode }),
}))

vi.mock('@/composables/usePortForward', () => ({
  usePortForward: () => ({ sshInfo: mockSshInfo }),
}))

import {
  isLocalhostUrl,
  parseLocalhostUrl,
  localhostOpenButtonHtml,
  annotateLocalhostUrls,
} from '@/composables/useLocalhostAnnotation'

describe('useLocalhostAnnotation', () => {
  // --- isLocalhostUrl ---

  describe('isLocalhostUrl', () => {
    it('returns true for http://localhost with port', () => {
      expect(isLocalhostUrl('http://localhost:3000')).toBe(true)
    })

    it('returns true for https://localhost with port', () => {
      expect(isLocalhostUrl('https://localhost:3000')).toBe(true)
    })

    it('returns true for http://127.0.0.1 with port', () => {
      expect(isLocalhostUrl('http://127.0.0.1:5173')).toBe(true)
    })

    it('returns false for non-localhost URLs', () => {
      expect(isLocalhostUrl('https://example.com')).toBe(false)
    })

    it('returns false for localhost without port', () => {
      expect(isLocalhostUrl('http://localhost')).toBe(false)
    })

    it('returns false for empty string', () => {
      expect(isLocalhostUrl('')).toBe(false)
    })
  })

  // --- parseLocalhostUrl ---

  describe('parseLocalhostUrl', () => {
    it('parses http://localhost:3000', () => {
      const result = parseLocalhostUrl('http://localhost:3000')
      expect(result).toEqual({
        port: 3000,
        protocol: 'http',
        fullUrl: 'http://localhost:3000',
      })
    })

    it('parses https://127.0.0.1:5173/path', () => {
      const result = parseLocalhostUrl('https://127.0.0.1:5173/path')
      expect(result).toEqual({
        port: 5173,
        protocol: 'https',
        fullUrl: 'https://127.0.0.1:5173',
      })
    })

    it('returns null for non-localhost URL', () => {
      expect(parseLocalhostUrl('https://example.com:3000')).toBeNull()
    })

    it('returns null for localhost without port', () => {
      expect(parseLocalhostUrl('http://localhost')).toBeNull()
    })
  })

  // --- localhostOpenButtonHtml ---

  describe('localhostOpenButtonHtml', () => {
    it('generates button HTML with data attributes', () => {
      const html = localhostOpenButtonHtml(3000, 'http', 'http://localhost:3000')
      expect(html).toContain('class="chat-url-open-btn"')
      expect(html).toContain('data-port="3000"')
      expect(html).toContain('data-protocol="http"')
      expect(html).toContain('data-url="http://localhost:3000"')
      expect(html).toContain('title="Open in WebView"')
    })

    it('escapes special characters in URL', () => {
      const html = localhostOpenButtonHtml(3000, 'http', 'http://localhost:3000/path?a=1&b=2')
      expect(html).toContain('data-url="http://localhost:3000/path?a=1&amp;b=2"')
    })
  })

  // --- annotateLocalhostUrls ---

  describe('annotateLocalhostUrls', () => {
    beforeEach(() => {
      mockIsAppMode.value = false
      mockSshInfo.value = null
    })

    it('returns empty string unchanged', () => {
      expect(annotateLocalhostUrls('')).toBe('')
    })

    it('returns HTML unchanged in web mode (isAppMode=false)', () => {
      mockIsAppMode.value = false
      const html = '<p>Visit http://localhost:3000</p>'
      expect(annotateLocalhostUrls(html)).toBe(html)
    })

    it('returns HTML unchanged when SSH is disabled', () => {
      mockIsAppMode.value = true
      mockSshInfo.value = { enabled: false }
      const html = '<p>Visit http://localhost:3000</p>'
      expect(annotateLocalhostUrls(html)).toBe(html)
    })

    it('annotates bare localhost URLs when SSH enabled and app mode', () => {
      mockIsAppMode.value = true
      mockSshInfo.value = { enabled: true }
      const html = '<p>Visit http://localhost:3000</p>'
      const result = annotateLocalhostUrls(html)
      expect(result).toContain('chat-url-open-btn')
      expect(result).toContain('data-port="3000"')
    })

    it('annotates localhost URLs inside <a> tags', () => {
      mockIsAppMode.value = true
      mockSshInfo.value = { enabled: true }
      const html = '<a href="http://localhost:5173">link</a>'
      const result = annotateLocalhostUrls(html)
      expect(result).toContain('chat-url-open-btn')
      expect(result).toContain('data-port="5173"')
    })

    it('annotates localhost URLs inside <code> tags', () => {
      mockIsAppMode.value = true
      mockSshInfo.value = { enabled: true }
      const html = '<code>http://localhost:8080</code>'
      const result = annotateLocalhostUrls(html)
      expect(result).toContain('chat-url-open-btn')
      expect(result).toContain('data-port="8080"')
    })

    it('does not annotate inside <pre> blocks', () => {
      mockIsAppMode.value = true
      mockSshInfo.value = { enabled: true }
      const html = '<pre>http://localhost:3000</pre>'
      const result = annotateLocalhostUrls(html)
      expect(result).not.toContain('chat-url-open-btn')
    })

    it('annotates 127.0.0.1 URLs', () => {
      mockIsAppMode.value = true
      mockSshInfo.value = { enabled: true }
      const html = '<p>http://127.0.0.1:9090</p>'
      const result = annotateLocalhostUrls(html)
      expect(result).toContain('chat-url-open-btn')
      expect(result).toContain('data-port="9090"')
    })

    it('handles null sshInfo (not yet loaded) as enabled', () => {
      mockIsAppMode.value = true
      mockSshInfo.value = null
      const html = '<p>http://localhost:3000</p>'
      const result = annotateLocalhostUrls(html)
      // null = still loading, should annotate (same as enabled)
      expect(result).toContain('chat-url-open-btn')
    })
  })
})

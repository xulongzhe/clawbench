import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useDoubleClickCopy } from '@/composables/useDoubleClickCopy'

// Mock clipboard
vi.mock('@/utils/clipboard', () => ({
  copyText: vi.fn(),
}))

// Mock useLocale
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

describe('useDoubleClickCopy', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  describe('handleAnchorClick — percent-encoded href decoding', () => {
    it('decodes percent-encoded Chinese href and calls onOpenFile with decoded path', () => {
      const { handleDblClick } = useDoubleClickCopy()
      const onOpenFile = vi.fn()

      // Create an anchor with percent-encoded Chinese href
      const anchor = document.createElement('a')
      anchor.setAttribute('href', '%E4%B8%AD%E6%96%87/%E6%96%87%E4%BB%B6.md')
      anchor.textContent = '链接'
      document.body.appendChild(anchor)

      const event = new MouseEvent('click', { bubbles: true, cancelable: true })
      anchor.dispatchEvent(event)
      // Re-create event to have the correct target
      Object.defineProperty(event, 'target', { value: anchor, writable: false })

      handleDblClick(event, onOpenFile)

      // onOpenFile should receive the decoded path, not the percent-encoded one
      expect(onOpenFile).toHaveBeenCalledWith('中文/文件.md')

      document.body.removeChild(anchor)
    })

    it('passes already-decoded Chinese href as-is to onOpenFile', () => {
      const { handleDblClick } = useDoubleClickCopy()
      const onOpenFile = vi.fn()

      const anchor = document.createElement('a')
      anchor.setAttribute('href', '中文/文件.md')
      anchor.textContent = '链接'
      document.body.appendChild(anchor)

      const event = new MouseEvent('click', { bubbles: true, cancelable: true })
      Object.defineProperty(event, 'target', { value: anchor, writable: false })

      handleDblClick(event, onOpenFile)

      expect(onOpenFile).toHaveBeenCalledWith('中文/文件.md')

      document.body.removeChild(anchor)
    })

    it('does not call onOpenFile for external https links with percent encoding', () => {
      const { handleDblClick } = useDoubleClickCopy()
      const onOpenFile = vi.fn()

      const anchor = document.createElement('a')
      anchor.setAttribute('href', 'https://example.com/%E4%B8%AD%E6%96%87')
      anchor.textContent = '外部链接'
      document.body.appendChild(anchor)

      const event = new MouseEvent('click', { bubbles: true, cancelable: true })
      Object.defineProperty(event, 'target', { value: anchor, writable: false })

      handleDblClick(event, onOpenFile)

      expect(onOpenFile).not.toHaveBeenCalled()

      document.body.removeChild(anchor)
    })

    it('decodes mixed ASCII and percent-encoded Chinese segments', () => {
      const { handleDblClick } = useDoubleClickCopy()
      const onOpenFile = vi.fn()

      const anchor = document.createElement('a')
      anchor.setAttribute('href', 'src/%E5%B7%A5%E5%85%B7/utils.ts')
      anchor.textContent = '工具'
      document.body.appendChild(anchor)

      const event = new MouseEvent('click', { bubbles: true, cancelable: true })
      Object.defineProperty(event, 'target', { value: anchor, writable: false })

      handleDblClick(event, onOpenFile)

      expect(onOpenFile).toHaveBeenCalledWith('src/工具/utils.ts')

      document.body.removeChild(anchor)
    })

    it('handles malformed percent encoding gracefully', () => {
      const { handleDblClick } = useDoubleClickCopy()
      const onOpenFile = vi.fn()

      const anchor = document.createElement('a')
      // %ZZ is not valid percent encoding
      anchor.setAttribute('href', 'path/%ZZfile.md')
      anchor.textContent = 'bad'
      document.body.appendChild(anchor)

      const event = new MouseEvent('click', { bubbles: true, cancelable: true })
      Object.defineProperty(event, 'target', { value: anchor, writable: false })

      handleDblClick(event, onOpenFile)

      // Should still call onOpenFile with the original (un-decoded) href
      expect(onOpenFile).toHaveBeenCalledWith('path/%ZZfile.md')

      document.body.removeChild(anchor)
    })

    it('does not call onOpenFile for anchor links (#section)', () => {
      const { handleDblClick } = useDoubleClickCopy()
      const onOpenFile = vi.fn()

      const anchor = document.createElement('a')
      anchor.setAttribute('href', '#section')
      anchor.textContent = 'jump'
      document.body.appendChild(anchor)

      const event = new MouseEvent('click', { bubbles: true, cancelable: true })
      Object.defineProperty(event, 'target', { value: anchor, writable: false })

      handleDblClick(event, onOpenFile)

      expect(onOpenFile).not.toHaveBeenCalled()

      document.body.removeChild(anchor)
    })

    it('does not call onOpenFile when no onOpenFile handler is provided', () => {
      const { handleDblClick } = useDoubleClickCopy()

      const anchor = document.createElement('a')
      anchor.setAttribute('href', '%E4%B8%AD%E6%96%87.md')
      anchor.textContent = '中文'
      document.body.appendChild(anchor)

      const event = new MouseEvent('click', { bubbles: true, cancelable: true })
      Object.defineProperty(event, 'target', { value: anchor, writable: false })

      // Should not throw
      expect(() => handleDblClick(event)).not.toThrow()

      document.body.removeChild(anchor)
    })
  })
})

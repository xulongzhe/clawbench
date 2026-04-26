import '../../../css/layout.css'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import FileViewer from '../file/FileViewer.vue'
import MarkdownPreview from '../file/MarkdownPreview.vue'
import WelcomeView from '../WelcomeView.vue'
import CodePreview from '../file/CodePreview.vue'

vi.mock('@/composables/useMarkdownRenderer.ts', () => ({
  useMarkdownRenderer: () => ({
    renderMarkdown: (content: string) => `<p>${content}</p>`,
    renderMermaidInElement: vi.fn().mockResolvedValue(undefined),
  }),
}))

vi.mock('@/composables/useDoubleClickCopy.ts', () => ({
  useDoubleClickCopy: () => ({
    handleDblClick: vi.fn(),
  }),
}))

vi.mock('@/composables/useFilePathAnnotation.ts', () => ({
  useFilePathAnnotation: () => ({
    annotateFilePaths: (html: string) => ({ html, detectedPaths: [] }),
    verifyFilePaths: vi.fn(),
    resolveRelativePath: (href: string) => href,
    openFilePath: vi.fn(),
  }),
}))

vi.mock('@/stores/app.ts', () => ({
  store: {
    state: {
      projectRoot: '/tmp/project',
    },
  },
}))

describe('preview layout contract', () => {
  it('stretches file viewer as a flex child', () => {
    const wrapper = mount(FileViewer, {
      props: {
        file: {
          name: 'README.md',
          path: '/tmp/README.md',
          content: '# Hello',
        },
      },
      global: {
        stubs: {
          FileHeader: { template: '<div class="file-header-stub" />' },
          ImagePreview: true,
          AudioPreview: true,
          VideoPreview: true,
          CodePreview: true,
          MarkdownPreview: { template: '<div class="markdown-preview-stub" />' },
        },
      },
    })

    const style = getComputedStyle(wrapper.get('.file-viewer').element)

    expect(style.flexGrow).toBe('1')
    expect(style.minHeight).toBe('0px')
  })

  it('stretches markdown preview for short rendered content', async () => {
    const wrapper = mount(MarkdownPreview, {
      props: {
        file: {
          path: '/tmp/README.md',
          content: '# Hello',
        },
        viewMode: 'rendered',
      },
    })

    await nextTick()
    await nextTick()

    const style = getComputedStyle(wrapper.get('.markdown-preview').element)

    expect(style.display).toBe('flex')
    expect(style.flexGrow).toBe('1')
    expect(style.minHeight).toBe('0px')
  })

  it('stretches welcome view as the empty state', () => {
    const wrapper = mount(WelcomeView)
    const style = getComputedStyle(wrapper.get('.welcome-view').element)

    expect(style.flexGrow).toBe('1')
    expect(style.minHeight).toBe('0px')
  })

  it('keeps the shell content area from becoming an extra scroll container', () => {
    const el = document.createElement('div')
    el.className = 'content-area'
    document.body.appendChild(el)

    const style = getComputedStyle(el)

    expect(style.overflowY).toBe('hidden')

    el.remove()
  })

  it('uses a flex column container for file viewer content', () => {
    const wrapper = mount(FileViewer, {
      props: {
        file: {
          name: 'main.ts',
          path: '/tmp/main.ts',
          content: 'const x = 1',
        },
      },
      global: {
        stubs: {
          FileHeader: { template: '<div class="file-header-stub" />' },
          ImagePreview: true,
          AudioPreview: true,
          VideoPreview: true,
        },
      },
    })

    const style = getComputedStyle(wrapper.get('.file-viewer-content').element)

    expect(style.display).toBe('flex')
    expect(style.flexDirection).toBe('column')
  })

  it('stretches raw code preview for short files', () => {
    const wrapper = mount(CodePreview, {
      props: {
        content: 'const x = 1',
        language: 'typescript',
        editable: false,
      },
      global: {
        stubs: {
          BottomSheet: true,
          teleport: true,
        },
      },
    })

    const style = getComputedStyle(wrapper.get('.raw-content-pre').element)

    expect(style.flexGrow).toBe('1')
    expect(style.minHeight).toBe('0px')
  })
})

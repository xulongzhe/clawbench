import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import HeaderMarquee from '@/components/common/HeaderMarquee.vue'

/**
 * Helper: flush Vue's reactive DOM update cycle.
 * After a watcher triggers checkOverflow() → isScrolling change,
 * one nextTick runs the watcher callback, another is needed for DOM patch.
 */
async function flushDom() {
  await nextTick()
  await nextTick()
}

describe('HeaderMarquee', () => {
  let observeSpy: vi.SpyInstance
  let disconnectSpy: vi.SpyInstance

  beforeEach(() => {
    observeSpy = vi.fn()
    disconnectSpy = vi.fn()
    vi.stubGlobal('ResizeObserver', class MockResizeObserver {
      observe = observeSpy
      unobserve = vi.fn()
      disconnect = disconnectSpy
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  /**
   * Mock offsetWidth on the actual wrapper and text DOM elements.
   * jsdom's prototype getter delegates to an internal impl object, so we must
   * define own properties on each element instance for the mock to take effect.
   * Returns a setter to update mock values for testing prop changes.
   */
  function mockElementOffsetWidths(
    wrapperEl: HTMLElement,
    textEl: HTMLElement,
    wrapperWidth: number,
    textWidth: number
  ) {
    const state = { wrapperWidth, textWidth }

    Object.defineProperty(wrapperEl, 'offsetWidth', {
      get: () => state.wrapperWidth,
      configurable: true,
    })
    Object.defineProperty(textEl, 'offsetWidth', {
      get: () => state.textWidth,
      configurable: true,
    })

    return {
      set(ww: number, tw: number) {
        state.wrapperWidth = ww
        state.textWidth = tw
      },
    }
  }

  function mountMarquee(props = {}, slots = {}) {
    return mount(HeaderMarquee, {
      props: { text: 'Hello', ...props },
      slots: { default: 'Hello World', ...slots },
    })
  }

  it('renders slot content inside the marquee wrapper', () => {
    const wrapper = mountMarquee()
    const textSpan = wrapper.find('.hm-text')
    expect(textSpan.exists()).toBe(true)
    expect(textSpan.text()).toBe('Hello World')
  })

  it('uses title prop when provided', () => {
    const wrapper = mountMarquee({ title: 'My Title' })
    expect(wrapper.find('.hm-wrapper').attributes('title')).toBe('My Title')
  })

  it('falls back to text prop for title when title is not provided', () => {
    const wrapper = mountMarquee()
    expect(wrapper.find('.hm-wrapper').attributes('title')).toBe('Hello')
  })

  it('prefers title over text when both are provided', () => {
    const wrapper = mountMarquee({ title: 'Title Text', text: 'Content Text' })
    expect(wrapper.find('.hm-wrapper').attributes('title')).toBe('Title Text')
  })

  it('does not scroll when text fits within wrapper', async () => {
    const wrapper = mountMarquee()
    mockElementOffsetWidths(
      wrapper.find('.hm-wrapper').element as HTMLElement,
      wrapper.find('.hm-text').element as HTMLElement,
      200, 100
    )

    wrapper.vm.checkOverflow?.()
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).not.toContain('hm-scrolling')
  })

  it('does not render duplicate text when not scrolling', async () => {
    const wrapper = mountMarquee()
    mockElementOffsetWidths(
      wrapper.find('.hm-wrapper').element as HTMLElement,
      wrapper.find('.hm-text').element as HTMLElement,
      200, 100
    )

    wrapper.vm.checkOverflow?.()
    await flushDom()

    expect(wrapper.find('.hm-text-copy').exists()).toBe(false)
  })

  it('enters scrolling state when text overflows wrapper', async () => {
    const wrapper = mountMarquee()
    mockElementOffsetWidths(
      wrapper.find('.hm-wrapper').element as HTMLElement,
      wrapper.find('.hm-text').element as HTMLElement,
      100, 200
    )

    wrapper.vm.checkOverflow?.()
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).toContain('hm-scrolling')
    expect(wrapper.findAll('.hm-text')).toHaveLength(2)
  })

  it('renders hm-text-copy span when scrolling', async () => {
    const wrapper = mountMarquee()
    mockElementOffsetWidths(
      wrapper.find('.hm-wrapper').element as HTMLElement,
      wrapper.find('.hm-text').element as HTMLElement,
      100, 200
    )

    wrapper.vm.checkOverflow?.()
    await flushDom()

    const copySpan = wrapper.find('.hm-text-copy')
    expect(copySpan.exists()).toBe(true)
    expect(copySpan.text()).toBe('Hello World')
  })

  it('re-checks overflow when text prop changes', async () => {
    const wrapper = mountMarquee()
    const mocks = mockElementOffsetWidths(
      wrapper.find('.hm-wrapper').element as HTMLElement,
      wrapper.find('.hm-text').element as HTMLElement,
      200, 100
    )

    wrapper.vm.checkOverflow?.()
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).not.toContain('hm-scrolling')

    // Update mock: text now overflows
    mocks.set(100, 200)

    await wrapper.setProps({ text: 'A Very Long Title That Overflows' })
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).toContain('hm-scrolling')
    expect(wrapper.findAll('.hm-text')).toHaveLength(2)
  })

  it('transitions from scrolling to not scrolling when text shrinks', async () => {
    const wrapper = mountMarquee()
    const mocks = mockElementOffsetWidths(
      wrapper.find('.hm-wrapper').element as HTMLElement,
      wrapper.find('.hm-text').element as HTMLElement,
      100, 200
    )

    wrapper.vm.checkOverflow?.()
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).toContain('hm-scrolling')

    mocks.set(200, 100)

    await wrapper.setProps({ text: 'Short' })
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).not.toContain('hm-scrolling')
    expect(wrapper.findAll('.hm-text')).toHaveLength(1)
  })

  it('observes wrapper and text elements with ResizeObserver on mount', () => {
    mountMarquee()

    expect(observeSpy).toHaveBeenCalledTimes(2)
  })

  it('disconnects ResizeObserver on unmount', () => {
    const wrapper = mountMarquee()

    expect(disconnectSpy).not.toHaveBeenCalled()

    wrapper.unmount()

    expect(disconnectSpy).toHaveBeenCalledTimes(1)
  })

  it('responds to ResizeObserver callback by re-checking overflow', async () => {
    let resizeCallback: () => void = () => {}

    vi.stubGlobal('ResizeObserver', class MockResizeObserver {
      observe = vi.fn()
      unobserve = vi.fn()
      disconnect = vi.fn()
      constructor(cb: () => void) {
        resizeCallback = cb
      }
    })

    const wrapper = mountMarquee()
    const mocks = mockElementOffsetWidths(
      wrapper.find('.hm-wrapper').element as HTMLElement,
      wrapper.find('.hm-text').element as HTMLElement,
      200, 50
    )

    wrapper.vm.checkOverflow?.()
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).not.toContain('hm-scrolling')

    mocks.set(100, 200)

    resizeCallback()
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).toContain('hm-scrolling')
  })

  it('handles null refs gracefully in checkOverflow', () => {
    const wrapper = mountMarquee()
    expect(wrapper.find('.hm-wrapper').exists()).toBe(true)
    expect(wrapper.find('.hm-text').exists()).toBe(true)
  })

  it('uses text prop value as default title when title is empty string', () => {
    const wrapper = mountMarquee({ title: '', text: 'Content' })
    expect(wrapper.find('.hm-wrapper').attributes('title')).toBe('Content')
  })

  it('does not scroll when text exactly equals wrapper width minus padding', async () => {
    const wrapper = mountMarquee()
    mockElementOffsetWidths(
      wrapper.find('.hm-wrapper').element as HTMLElement,
      wrapper.find('.hm-text').element as HTMLElement,
      108, 100
    )

    wrapper.vm.checkOverflow?.()
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).not.toContain('hm-scrolling')
  })

  it('scrolls when text exceeds wrapper width minus padding by 1px', async () => {
    const wrapper = mountMarquee()
    mockElementOffsetWidths(
      wrapper.find('.hm-wrapper').element as HTMLElement,
      wrapper.find('.hm-text').element as HTMLElement,
      108, 101
    )

    wrapper.vm.checkOverflow?.()
    await flushDom()

    expect(wrapper.find('.hm-wrapper').classes()).toContain('hm-scrolling')
  })
})

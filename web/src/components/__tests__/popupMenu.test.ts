import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import PopupMenu from '@/components/common/PopupMenu.vue'
import * as popupMenuPosition from '@/utils/popupMenuPosition'

describe('PopupMenu', () => {
  let targetElement: HTMLDivElement
  let computeMenuStyleSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    // Spy on the real function and mock its return value
    computeMenuStyleSpy = vi.spyOn(popupMenuPosition, 'computeMenuStyle').mockReturnValue(
      { position: 'fixed', top: '100px', left: '50px' } as any
    )
    targetElement = document.createElement('div')
    targetElement.classList.add('target')
    targetElement.getBoundingClientRect = vi.fn(() => ({
      top: 400, bottom: 440, left: 100, right: 200, width: 100, height: 40,
      x: 100, y: 400, toJSON: () => {},
    }) as DOMRect)
    targetElement.contains = vi.fn(() => false)
    document.body.appendChild(targetElement)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    document.body.removeChild(targetElement)
  })

  function mountMenu(props: Record<string, any> = {}, slots: Record<string, string> = {}) {
    return mount(PopupMenu, {
      props: { targetElement, ...props },
      slots: { default: '<div class="menu-item">Item 1</div>', ...slots },
      global: { stubs: { teleport: true } },
    })
  }

  /** Open the menu by toggling show prop from false to true */
  async function openMenu(props: Record<string, any> = {}, slots: Record<string, string> = {}) {
    const wrapper = mountMenu({ show: false, ...props }, slots)
    await wrapper.setProps({ show: true })
    await nextTick()
    return wrapper
  }

  it('renders menu when show is true', async () => {
    const wrapper = await openMenu()
    expect(wrapper.find('.popup-menu').exists()).toBe(true)
    expect(wrapper.find('.menu-item').text()).toBe('Item 1')
  })

  it('does not render menu when show is false', () => {
    const wrapper = mountMenu({ show: false })
    expect(wrapper.find('.popup-menu').exists()).toBe(false)
  })

  it('has role="menu"', async () => {
    const wrapper = await openMenu()
    expect(wrapper.find('.popup-menu').attributes('role')).toBe('menu')
  })

  it('calls computeMenuStyle on open', async () => {
    await openMenu()
    expect(computeMenuStyleSpy).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ maxWidth: 220, maxHeight: 320, edgeMargin: 6, menuItemsCount: 10 }),
    )
  })

  it('passes custom props to computeMenuStyle', async () => {
    await openMenu({ maxWidth: 300, maxHeight: 400, edgeMargin: 10, menuItemsCount: 5 })
    expect(computeMenuStyleSpy).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ maxWidth: 300, maxHeight: 400, edgeMargin: 10, menuItemsCount: 5 }),
    )
  })

  it('applies computed style to menu element', async () => {
    const wrapper = await openMenu()
    const style = wrapper.find('.popup-menu').attributes('style')
    expect(style).toContain('position: fixed')
  })

  it('emits update:show false when menu is clicked', async () => {
    const wrapper = await openMenu()
    await wrapper.find('.popup-menu').trigger('click')
    expect(wrapper.emitted('update:show')).toBeTruthy()
    expect(wrapper.emitted('update:show')![0]).toEqual([false])
  })

  it('emits update:show false on Escape key', async () => {
    const wrapper = await openMenu()
    await wrapper.find('.popup-menu').trigger('keydown.escape')
    expect(wrapper.emitted('update:show')).toBeTruthy()
    expect(wrapper.emitted('update:show')![0]).toEqual([false])
  })

  it('click outside target and menu closes the menu', async () => {
    vi.useFakeTimers()
    const wrapper = await openMenu()
    vi.advanceTimersByTime(1)
    await nextTick()

    // Simulate a document click outside both target and menu
    const outsideEl = document.createElement('div')
    const event = new MouseEvent('click', { bubbles: true })
    Object.defineProperty(event, 'target', { value: outsideEl, writable: false })
    document.dispatchEvent(event)
    await nextTick()

    expect(wrapper.emitted('update:show')).toBeTruthy()
    expect(wrapper.emitted('update:show')!.find(e => e[0] === false)).toBeTruthy()
  })

  it('does not close on click on target element', async () => {
    vi.useFakeTimers()
    const wrapper = await openMenu()
    vi.advanceTimersByTime(1)
    await nextTick()

    // Click on target element — targetElement.contains returns true
    ;(targetElement.contains as ReturnType<typeof vi.fn>).mockReturnValue(true)
    const event = new MouseEvent('click', { bubbles: true })
    Object.defineProperty(event, 'target', { value: targetElement, writable: false })
    document.dispatchEvent(event)
    await nextTick()

    expect(wrapper.emitted('update:show')).toBeFalsy()
  })

  it('does not close on click inside popup menu', async () => {
    vi.useFakeTimers()
    const wrapper = await openMenu()
    vi.advanceTimersByTime(1)
    await nextTick()

    // Create an element that has .popup-menu as ancestor via closest()
    const menuEl = document.createElement('div')
    menuEl.classList.add('popup-menu')
    const innerEl = document.createElement('span')
    menuEl.appendChild(innerEl)
    innerEl.closest = vi.fn((sel: string) => sel === '.popup-menu' ? menuEl : null)

    const event = new MouseEvent('click', { bubbles: true })
    Object.defineProperty(event, 'target', { value: innerEl, writable: false })
    document.dispatchEvent(event)
    await nextTick()

    expect(wrapper.emitted('update:show')).toBeFalsy()
  })

  it('adds scroll and resize listeners on open', async () => {
    const addSpy = vi.spyOn(window, 'addEventListener')
    await openMenu()
    expect(addSpy).toHaveBeenCalledWith('scroll', expect.any(Function), true)
    expect(addSpy).toHaveBeenCalledWith('resize', expect.any(Function))
  })

  it('removes all listeners when show becomes false', async () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener')
    const docRemoveSpy = vi.spyOn(document, 'removeEventListener')
    const wrapper = await openMenu()
    await wrapper.setProps({ show: false })
    await nextTick()
    expect(removeSpy).toHaveBeenCalledWith('scroll', expect.any(Function), true)
    expect(removeSpy).toHaveBeenCalledWith('resize', expect.any(Function))
    expect(docRemoveSpy).toHaveBeenCalledWith('click', expect.any(Function))
  })

  it('removes all listeners on unmount while open', async () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener')
    const docRemoveSpy = vi.spyOn(document, 'removeEventListener')
    const wrapper = await openMenu()
    wrapper.unmount()
    expect(removeSpy).toHaveBeenCalledWith('scroll', expect.any(Function), true)
    expect(removeSpy).toHaveBeenCalledWith('resize', expect.any(Function))
    expect(docRemoveSpy).toHaveBeenCalledWith('click', expect.any(Function))
  })

  it('click-outside listener is registered via setTimeout (not synchronously)', async () => {
    vi.useFakeTimers()
    const docAddSpy = vi.spyOn(document, 'addEventListener')
    const wrapper = mountMenu({ show: false })
    await wrapper.setProps({ show: true })
    await nextTick()

    // Before advancing timers, click listener should NOT be registered yet
    const clicksBeforeTimer = docAddSpy.mock.calls.filter(c => c[0] === 'click').length
    expect(clicksBeforeTimer).toBe(0)

    // After advancing past setTimeout(0), it should be registered
    vi.advanceTimersByTime(1)
    const clicksAfterTimer = docAddSpy.mock.calls.filter(c => c[0] === 'click').length
    expect(clicksAfterTimer).toBeGreaterThan(clicksBeforeTimer)
  })

  it('does not crash when targetElement is null on open', async () => {
    const wrapper = mount(PopupMenu, {
      props: { show: false, targetElement: null },
      slots: { default: '<div class="menu-item">Item</div>' },
      global: { stubs: { teleport: true } },
    })
    await wrapper.setProps({ show: true })
    await nextTick()
    expect(wrapper.find('.popup-menu').exists()).toBe(true)
    expect(wrapper.find('.popup-menu').attributes('style')).toBeUndefined()
  })

  it('does not close on outside click when targetElement is null', async () => {
    vi.useFakeTimers()
    const wrapper = mount(PopupMenu, {
      props: { show: false, targetElement: null },
      slots: { default: '<div class="menu-item">Item</div>' },
      global: { stubs: { teleport: true } },
    })
    await wrapper.setProps({ show: true })
    await nextTick()
    vi.advanceTimersByTime(1)
    await nextTick()

    const event = new MouseEvent('click', { bubbles: true })
    Object.defineProperty(event, 'target', { value: document.body, writable: false })
    document.dispatchEvent(event)
    await nextTick()

    expect(wrapper.emitted('update:show')).toBeFalsy()
  })

  it('recalculates position on scroll while open', async () => {
    await openMenu()
    const callCountBefore = computeMenuStyleSpy.mock.calls.length

    // Dispatch a scroll event (captured)
    window.dispatchEvent(new Event('scroll', { bubbles: true }))
    await nextTick()

    expect(computeMenuStyleSpy.mock.calls.length).toBeGreaterThan(callCountBefore)
  })

  it('recalculates position on resize while open', async () => {
    await openMenu()
    const callCountBefore = computeMenuStyleSpy.mock.calls.length

    window.dispatchEvent(new Event('resize'))
    await nextTick()

    expect(computeMenuStyleSpy.mock.calls.length).toBeGreaterThan(callCountBefore)
  })

  it('removes scroll/resize listeners when closed', async () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener')
    const wrapper = await openMenu()
    // Close — scroll/resize listeners should be removed
    await wrapper.setProps({ show: false })
    await nextTick()
    expect(removeSpy).toHaveBeenCalledWith('scroll', expect.any(Function), true)
    expect(removeSpy).toHaveBeenCalledWith('resize', expect.any(Function))
  })

  it('opens and closes reactively via show prop', async () => {
    const wrapper = mount(PopupMenu, {
      props: { show: false, targetElement },
      slots: { default: '<div class="menu-item">Item 1</div>' },
      global: { stubs: { teleport: true } },
    })
    expect(wrapper.find('.popup-menu').exists()).toBe(false)

    await wrapper.setProps({ show: true })
    await nextTick()
    expect(wrapper.find('.popup-menu').exists()).toBe(true)
    expect(wrapper.find('.menu-item').text()).toBe('Item 1')

    await wrapper.setProps({ show: false })
    await nextTick()
    expect(wrapper.find('.popup-menu').exists()).toBe(false)
  })

  it('renders slot content', async () => {
    const wrapper = await openMenu({}, { default: '<span class="custom-item">Custom</span>' })
    expect(wrapper.find('.custom-item').text()).toBe('Custom')
  })

  it('emits update:show false only once per interaction', async () => {
    const wrapper = await openMenu()
    await wrapper.find('.popup-menu').trigger('click')
    expect(wrapper.emitted('update:show')!.length).toBe(1)
  })
})

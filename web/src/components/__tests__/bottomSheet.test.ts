import { describe, expect, it, vi, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import BottomSheet from '@/components/common/BottomSheet.vue'

describe('BottomSheet', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  function mountSheet(props = {}, slots = {}) {
    return mount(BottomSheet, {
      props: { open: true, ...props },
      slots,
      global: {
        stubs: { teleport: true },
      },
    })
  }

  it('renders content when open', async () => {
    const wrapper = mountSheet({}, { default: '<div class="content">Hello</div>' })

    expect(wrapper.find('.bs-overlay').exists()).toBe(true)
    expect(wrapper.find('.content').text()).toBe('Hello')
  })

  it('shows header with title when noHeader is false (default)', async () => {
    const wrapper = mountSheet({ title: 'Test Sheet' })

    expect(wrapper.find('.bs-header').exists()).toBe(true)
    expect(wrapper.find('.bs-title').text()).toBe('Test Sheet')
  })

  it('hides header when noHeader is true', async () => {
    const wrapper = mountSheet({ noHeader: true })

    expect(wrapper.find('.bs-header').exists()).toBe(false)
  })

  it('shows handle-only header when handleOnly is true', async () => {
    const wrapper = mountSheet({ handleOnly: true, title: 'Ignored Title' })

    expect(wrapper.find('.bs-header').exists()).toBe(true)
    expect(wrapper.find('.bs-header').classes()).toContain('bs-header-handle-only')
    expect(wrapper.find('.bs-title').exists()).toBe(false)
    expect(wrapper.find('.bs-handle').exists()).toBe(true)
  })

  it('noHeader takes precedence over handleOnly', async () => {
    const wrapper = mountSheet({ noHeader: true, handleOnly: true })

    expect(wrapper.find('.bs-header').exists()).toBe(false)
  })

  it('uses header slot when provided', async () => {
    const wrapper = mountSheet({}, { header: '<span class="custom-hdr">Custom</span>' })

    expect(wrapper.find('.custom-hdr').text()).toBe('Custom')
  })

  it('emits close when overlay is clicked (after animation)', async () => {
    vi.useFakeTimers()
    const wrapper = mountSheet()

    await wrapper.find('.bs-overlay').trigger('click')
    // handleClose sets leaving=true and starts 250ms timer
    expect(wrapper.find('.bs-panel').classes()).toContain('bs-leaving')

    vi.advanceTimersByTime(250)
    await nextTick()

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('emits close immediately in instant mode', async () => {
    const wrapper = mountSheet({ instant: true })

    await wrapper.find('.bs-overlay').trigger('click')

    // With instant mode, close emitted immediately (no animation)
    expect(wrapper.emitted('close')).toBeTruthy()
    expect(wrapper.find('.bs-panel').classes()).toContain('bs-instant')
  })

  it('applies compact class when compact prop is true', async () => {
    const wrapper = mountSheet({ compact: true })

    expect(wrapper.find('.bs-panel').classes()).toContain('bs-compact')
  })

  it('applies auto class when auto prop is true', async () => {
    const wrapper = mountSheet({ auto: true })

    expect(wrapper.find('.bs-panel').classes()).toContain('bs-auto')
  })

  it('keeps content in DOM after closing (everOpened)', async () => {
    const wrapper = mountSheet({}, { default: '<div class="content">Hello</div>' })

    expect(wrapper.find('.bs-overlay').exists()).toBe(true)

    // Close the sheet via prop
    await wrapper.setProps({ open: false })
    await nextTick()

    // everOpened remains true, v-show hides it
    expect(wrapper.find('.bs-overlay').exists()).toBe(true)
    expect(wrapper.find('.bs-overlay').isVisible()).toBe(false)
  })

  it('renders footer slot when provided', async () => {
    const wrapper = mountSheet({}, { footer: '<button class="footer-btn">OK</button>' })

    expect(wrapper.find('.bs-footer').exists()).toBe(true)
    expect(wrapper.find('.footer-btn').text()).toBe('OK')
  })

  it('does not render footer when slot not provided', async () => {
    const wrapper = mountSheet()

    expect(wrapper.find('.bs-footer').exists()).toBe(false)
  })

  it('enters leaving state before closing (animation)', async () => {
    vi.useFakeTimers()
    const wrapper = mountSheet()

    await wrapper.find('.bs-overlay').trigger('click')
    await nextTick()

    // Should have leaving class
    expect(wrapper.find('.bs-panel').classes()).toContain('bs-leaving')

    // After 250ms, close event should fire
    vi.advanceTimersByTime(250)
    await nextTick()

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('does not emit close twice when already leaving', async () => {
    vi.useFakeTimers()
    const wrapper = mountSheet()

    await wrapper.find('.bs-overlay').trigger('click')
    await nextTick()

    // Click again while leaving — should be blocked
    await wrapper.find('.bs-overlay').trigger('click')
    await nextTick()

    // Advance past the timer — should only have emitted once
    vi.advanceTimersByTime(250)
    await nextTick()

    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('does not render when never opened', () => {
    const wrapper = mount(BottomSheet, {
      props: { open: false },
      global: { stubs: { teleport: true } },
    })

    expect(wrapper.find('.bs-overlay').exists()).toBe(false)
  })

  it('renders when opened after initial mount', async () => {
    const wrapper = mount(BottomSheet, {
      props: { open: false },
      slots: { default: '<div class="content">Hello</div>' },
      global: { stubs: { teleport: true } },
    })

    expect(wrapper.find('.bs-overlay').exists()).toBe(false)

    await wrapper.setProps({ open: true })
    await nextTick()

    expect(wrapper.find('.bs-overlay').exists()).toBe(true)
    expect(wrapper.find('.content').text()).toBe('Hello')
  })

  it('cancels leaving animation when re-opened', async () => {
    vi.useFakeTimers()
    const wrapper = mountSheet()

    // Start closing
    await wrapper.find('.bs-overlay').trigger('click')
    await nextTick()
    expect(wrapper.find('.bs-panel').classes()).toContain('bs-leaving')

    // Simulate parent responding to close event by setting open=false, then re-opening
    vi.advanceTimersByTime(250)
    await nextTick()
    expect(wrapper.emitted('close')).toBeTruthy()

    // Parent sets open=false
    await wrapper.setProps({ open: false })
    await nextTick()

    // Then parent re-opens
    await wrapper.setProps({ open: true })
    await nextTick()

    // leaving should be reset
    expect(wrapper.find('.bs-panel').classes()).not.toContain('bs-leaving')

    vi.useRealTimers()
  })
})

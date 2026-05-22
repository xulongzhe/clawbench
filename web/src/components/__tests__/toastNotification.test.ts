import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import ToastNotification from '@/components/common/ToastNotification.vue'

function createMockToast(overrides = {}) {
  return {
    visible: ref(overrides.visible ?? true),
    type: ref(overrides.type ?? 'info'),
    message: ref(overrides.message ?? 'Test message'),
    icon: ref(overrides.icon ?? ''),
    onClick: ref(overrides.onClick ?? null),
    dismiss: vi.fn(),
  }
}

function mountToast(toast) {
  return mount(ToastNotification, {
    props: { toast },
    global: {
      stubs: { teleport: true },
    },
  })
}

describe('ToastNotification', () => {
  it('renders when visible', () => {
    const toast = createMockToast({ visible: true })
    const wrapper = mountToast(toast)

    expect(wrapper.find('.toast').exists()).toBe(true)
  })

  it('hides when not visible', () => {
    const toast = createMockToast({ visible: false })
    const wrapper = mountToast(toast)

    expect(wrapper.find('.toast').exists()).toBe(false)
  })

  it('applies type-specific class for info', () => {
    const toast = createMockToast({ type: 'info' })
    const wrapper = mountToast(toast)

    expect(wrapper.find('.toast').classes()).toContain('toast-info')
  })

  it('applies type-specific class for success', () => {
    const toast = createMockToast({ type: 'success' })
    const wrapper = mountToast(toast)

    expect(wrapper.find('.toast').classes()).toContain('toast-success')
  })

  it('applies type-specific class for error', () => {
    const toast = createMockToast({ type: 'error' })
    const wrapper = mountToast(toast)

    expect(wrapper.find('.toast').classes()).toContain('toast-error')
  })

  it('displays the message text', () => {
    const toast = createMockToast({ message: 'Hello world' })
    const wrapper = mountToast(toast)

    expect(wrapper.find('.toast-text').text()).toBe('Hello world')
  })

  it('shows icon when icon value is truthy', () => {
    const toast = createMockToast({ icon: '✓' })
    const wrapper = mountToast(toast)

    expect(wrapper.find('.toast-icon').exists()).toBe(true)
    expect(wrapper.find('.toast-icon').text()).toBe('✓')
  })

  it('hides icon when icon value is falsy', () => {
    const toast = createMockToast({ icon: '' })
    const wrapper = mountToast(toast)

    expect(wrapper.find('.toast-icon').exists()).toBe(false)
  })

  it('calls onClick and then dismiss when clicked with onClick', async () => {
    const onClick = vi.fn()
    const toast = createMockToast({ onClick })
    const wrapper = mountToast(toast)

    await wrapper.find('.toast').trigger('click')

    expect(onClick).toHaveBeenCalledTimes(1)
    expect(toast.dismiss).toHaveBeenCalledTimes(1)
    // onClick should be called before dismiss
    const onClickCallOrder = onClick.mock.invocationCallOrder[0]
    const dismissCallOrder = toast.dismiss.mock.invocationCallOrder[0]
    expect(onClickCallOrder).toBeLessThan(dismissCallOrder)
  })

  it('calls only dismiss when clicked without onClick', async () => {
    const toast = createMockToast({ onClick: null })
    const wrapper = mountToast(toast)

    await wrapper.find('.toast').trigger('click')

    expect(toast.dismiss).toHaveBeenCalledTimes(1)
  })

  it('has base toast class alongside type class', () => {
    const toast = createMockToast({ type: 'error' })
    const wrapper = mountToast(toast)

    const classes = wrapper.find('.toast').classes()
    expect(classes).toContain('toast')
    expect(classes).toContain('toast-error')
  })
})

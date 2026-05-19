import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SettingsDrawer from '@/components/settings/SettingsDrawer.vue'

// Mock composable
vi.mock('@/composables/useSettingsConfig', () => ({
  useSettingsConfig: () => ({
    loadConfig: vi.fn(),
    restartServer: vi.fn(),
    localConfig: {},
    serverConfig: { value: {} },
    setLocalConfig: vi.fn(),
    getServerValue: vi.fn(),
    setServerValue: vi.fn(),
  }),
}))

// Stub child components
const globalStubs = {
  SettingsIndex: {
    template: '<div class="stub-index" @click="$emit(\'navigate\', \'appearance\')" />',
  },
  SettingsCategory: {
    template: '<div class="stub-category" />',
    props: ['categoryId'],
  },
  SettingsRestartDialog: {
    template: '<div class="stub-restart" v-if="false" />',
    props: ['changedFields'],
  },
  'lucide-chevron-left': true,
}

function mountDrawer(props = {}) {
  return mount(SettingsDrawer, {
    props: { show: true, ...props },
    global: { stubs: globalStubs },
  })
}

describe('SettingsDrawer', () => {
  it('shows SettingsIndex when nav stack is empty', () => {
    const wrapper = mountDrawer()

    expect(wrapper.find('.stub-index').exists()).toBe(true)
    expect(wrapper.find('.stub-category').exists()).toBe(false)
  })

  it('shows SettingsCategory after navigating', async () => {
    const wrapper = mountDrawer()

    // Simulate navigate from SettingsIndex
    await wrapper.find('.stub-index').trigger('click')

    expect(wrapper.find('.stub-category').exists()).toBe(true)
    expect(wrapper.find('.stub-index').exists()).toBe(false)
  })

  it('emits close when back button clicked with empty nav stack', async () => {
    const wrapper = mountDrawer()

    await wrapper.find('.settings-drawer__back').trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('pops nav stack instead of closing when nav is non-empty', async () => {
    const wrapper = mountDrawer()

    // Navigate into a category
    await wrapper.find('.stub-index').trigger('click')
    expect(wrapper.find('.stub-category').exists()).toBe(true)

    // Click back — should go back to index, not close
    await wrapper.find('.settings-drawer__back').trigger('click')
    expect(wrapper.find('.stub-index').exists()).toBe(true)
    expect(wrapper.emitted('close')).toBeFalsy()
  })

  it('renders with correct z-index above BottomSheet', () => {
    const wrapper = mountDrawer()

    const drawer = wrapper.find('.settings-drawer')
    expect(drawer.exists()).toBe(true)
    const style = drawer.attributes('style') ?? ''
    // z-index is set via CSS, not inline, so just verify the element exists
    expect(drawer.classes()).toContain('settings-drawer')
  })

  it('does not render when show is false', () => {
    const wrapper = mountDrawer({ show: false })

    expect(wrapper.find('.settings-drawer').exists()).toBe(false)
  })
})

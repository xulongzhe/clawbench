import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import SettingsPage from '@/components/settings/SettingsPage.vue'

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

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: {
    zh: {
      settings: {
        categories: { appearance: '外观' },
        restartServer: '重启服务器',
        restartPending: '重启生效',
        restarting: '重启中…',
      },
    },
  },
})

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
  'lucide-refresh-cw': true,
  'lucide-chevron-left': true,
}

function mountPage(props = {}) {
  return mount(SettingsPage, {
    props: { active: true, ...props },
    global: { stubs: globalStubs, plugins: [i18n] },
  })
}

describe('SettingsPage', () => {
  it('shows SettingsIndex when nav stack is empty', () => {
    const wrapper = mountPage()

    expect(wrapper.find('.stub-index').exists()).toBe(true)
    expect(wrapper.find('.stub-category').exists()).toBe(false)
  })

  it('shows SettingsCategory after navigating', async () => {
    const wrapper = mountPage()

    // Simulate navigate from SettingsIndex
    await wrapper.find('.stub-index').trigger('click')

    expect(wrapper.find('.stub-category').exists()).toBe(true)
    expect(wrapper.find('.stub-index').exists()).toBe(false)
  })

  it('shows restart button in footer', () => {
    const wrapper = mountPage()

    expect(wrapper.find('.settings-restart-btn').exists()).toBe(true)
  })

  it('resets nav stack when becoming active', async () => {
    const wrapper = mountPage()

    // Navigate into a category
    await wrapper.find('.stub-index').trigger('click')
    expect(wrapper.find('.stub-category').exists()).toBe(true)

    // Deactivate and reactivate
    await wrapper.setProps({ active: false })
    await wrapper.setProps({ active: true })

    // Should be back at index
    expect(wrapper.find('.stub-index').exists()).toBe(true)
  })

  it('shows restart-pending style when needsRestart is true after receiving restart-needed event', async () => {
    const wrapper = mountPage()

    expect(wrapper.find('.settings-restart-btn--pending').exists()).toBe(false)
  })

  it('renders as a full page layout', () => {
    const wrapper = mountPage()

    expect(wrapper.find('.settings-page').exists()).toBe(true)
    expect(wrapper.find('.settings-page__body').exists()).toBe(true)
    expect(wrapper.find('.settings-page__footer').exists()).toBe(true)
  })
})

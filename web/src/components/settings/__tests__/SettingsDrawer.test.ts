import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
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
    patchConfig: vi.fn(),
  }),
}))

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: {
    zh: {
      nav: { settings: '设置' },
      settings: {
        categories: { appearance: '外观', chat: '聊天', agents: 'Agent偏好', files: '文件', terminal: '终端', tts: 'TTS语音', rag: 'RAG记忆', network: '网络', android: 'Android', about: '关于' },
        restartServer: '重启服务器',
        restartPending: '重启生效',
        restarting: '重启中…',
        restartingPleaseWait: '正在重启，请稍候…',
      },
    },
  },
})

// Stub child components
const globalStubs = {
  BottomSheet: {
    template: '<div class="stub-bottomsheet" v-if="$slots.default"><slot name="header" /><slot /><slot name="footer" /></div>',
    props: ['open'],
  },
  SettingsIndex: {
    template: '<div class="stub-index" @click="$emit(\'navigate\', \'appearance\')" />',
  },
  SettingsCategory: {
    template: '<div class="stub-category" />',
    props: ['categoryId'],
    methods: {
      saveChanges: vi.fn().mockResolvedValue(undefined),
    },
  },
  SettingsRestartDialog: {
    template: '<div class="stub-restart" v-if="false" />',
    props: ['changedFields'],
  },
}

function mountDrawer(props = {}) {
  return mount(SettingsDrawer, {
    props: { open: true, ...props },
    global: { plugins: [i18n], stubs: globalStubs },
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

  it('emits close when close button clicked', async () => {
    const wrapper = mountDrawer()

    await wrapper.find('.settings-close-btn').trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('shows back button when in a category', async () => {
    const wrapper = mountDrawer()

    // Initially no back button
    expect(wrapper.find('.settings-back-btn').exists()).toBe(false)

    // Navigate into a category
    await wrapper.find('.stub-index').trigger('click')

    // Back button should now be visible
    expect(wrapper.find('.settings-back-btn').exists()).toBe(true)
  })

  it('pops nav stack when back button clicked in category', async () => {
    const wrapper = mountDrawer()

    // Navigate into a category
    await wrapper.find('.stub-index').trigger('click')
    expect(wrapper.find('.stub-category').exists()).toBe(true)

    // Click back — should go back to index, not close
    await wrapper.find('.settings-back-btn').trigger('click')
    expect(wrapper.find('.stub-index').exists()).toBe(true)
    expect(wrapper.emitted('close')).toBeFalsy()
  })

  it('resets nav stack when reopened', async () => {
    const wrapper = mountDrawer()

    // Navigate into a category
    await wrapper.find('.stub-index').trigger('click')
    expect(wrapper.find('.stub-category').exists()).toBe(true)

    // Close and reopen
    await wrapper.setProps({ open: false })
    await wrapper.setProps({ open: true })

    // Should be back at index
    expect(wrapper.find('.stub-index').exists()).toBe(true)
  })

  it('shows close button always', () => {
    const wrapper = mountDrawer()

    expect(wrapper.find('.settings-close-btn').exists()).toBe(true)
  })
})

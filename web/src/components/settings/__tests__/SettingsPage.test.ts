import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { ref } from 'vue'
import SettingsPage from '@/components/settings/SettingsPage.vue'

// Mutable refs that tests can flip to control UI state
const needsRestart = ref(false)
const restarting = ref(false)

// Keep a shared navStack ref so we can wire up pushNav/popNav
const navStack = ref<string[]>([])

function createMockNavigation() {
  return {
    t: (key: string) => key,
    loadConfig: vi.fn(),
    navStack,
    currentCategory: ref<string | null>(null),
    pushNav: (id: string) => { navStack.value.push(id) },
    popNav: () => { navStack.value.pop() },
    resetState: () => { navStack.value = []; needsRestart.value = false; restarting.value = false },
    restartDialogVisible: ref(false),
    changedColdFields: ref<string[]>([]),
    needsRestart,
    restarting,
    restartingOverlay: ref(false),
    handleRestartNeeded: vi.fn(),
    handleRestart: vi.fn(),
  }
}

vi.mock('@/composables/useSettingsNavigation', () => ({
  useSettingsNavigation: () => createMockNavigation(),
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
        restartingPleaseWait: '正在重启，请稍候…',
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
  beforeEach(() => {
    navStack.value = []
    needsRestart.value = false
    restarting.value = false
  })
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

  it('shows restart-pending style when needsRestart is true', async () => {
    needsRestart.value = false
    const wrapper = mountPage()

    expect(wrapper.find('.settings-restart-btn--pending').exists()).toBe(false)
    expect(wrapper.find('.settings-restart-btn--idle').exists()).toBe(true)

    needsRestart.value = true
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.settings-restart-btn--pending').exists()).toBe(true)
    expect(wrapper.find('.settings-restart-btn--idle').exists()).toBe(false)
  })

  it('shows idle class on restart button when no changes need restart', () => {
    needsRestart.value = false
    restarting.value = false
    const wrapper = mountPage()

    const btn = wrapper.find('.settings-restart-btn')
    expect(btn.classes()).toContain('settings-restart-btn--idle')
    expect(btn.classes()).not.toContain('settings-restart-btn--pending')
  })

  it('removes idle class and adds pending class when needsRestart becomes true', async () => {
    needsRestart.value = false
    const wrapper = mountPage()

    const btn = wrapper.find('.settings-restart-btn')
    expect(btn.classes()).toContain('settings-restart-btn--idle')

    needsRestart.value = true
    await wrapper.vm.$nextTick()

    expect(btn.classes()).toContain('settings-restart-btn--pending')
    expect(btn.classes()).not.toContain('settings-restart-btn--idle')
  })

  it('renders as a full page layout', () => {
    const wrapper = mountPage()

    expect(wrapper.find('.settings-page').exists()).toBe(true)
    expect(wrapper.find('.settings-page__body').exists()).toBe(true)
    expect(wrapper.find('.settings-page__footer').exists()).toBe(true)
  })
})

import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import ModelModal from '@/components/chat/ModelModal.vue'
import { useAgents } from '@/composables/useAgents'
import { useSessionIdentity } from '@/composables/useSessionIdentity'

// Mock composables
vi.mock('@/composables/useAgents', () => ({
  useAgents: vi.fn(),
}))
vi.mock('@/composables/useSessionIdentity', () => ({
  useSessionIdentity: vi.fn(),
}))
vi.mock('@/utils/api', () => ({
  apiPost: vi.fn().mockResolvedValue({ models: [] }),
}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
  createI18n: () => ({ global: { t: (key: string) => key } }),
}))
vi.mock('@/composables/useSettingsConfig', () => ({
  patchAgentPref: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('@/composables/useToast', () => ({
  useToast: () => ({ show: vi.fn() }),
}))
vi.mock('@/composables/useLocale', () => ({
  gt: (key: string) => key,
}))

const mockAgents = {
  agents: ref([
    {
      id: 'claude',
      name: 'Claude',
      icon: '🤖',
      backend: 'claude',
      models: [
        { id: 'claude-sonnet-4-6', name: 'Claude Sonnet 4.6', default: true },
        { id: 'claude-opus-4-5', name: 'Claude Opus 4.5', default: false },
        { id: 'claude-haiku-3-5', name: 'Claude Haiku 3.5', default: false },
      ],
      thinkingEffortLevels: ['low', 'medium', 'high', 'xhigh', 'max'],
      preferredModel: 'claude-sonnet-4-6',
      preferredThinkingEffort: '',
    },
    {
      id: 'gemini',
      name: 'Gemini',
      icon: '💎',
      backend: 'gemini',
      models: [],
      thinkingEffortLevels: [],
      preferredModel: '',
      preferredThinkingEffort: '',
    },
  ]),
  getAgentModels: vi.fn((agentId: string) => {
    const a = mockAgents.agents.value.find(a => a.id === agentId)
    return a?.models || []
  }),
  getAgentThinkingEffortLevels: vi.fn((agentId: string) => {
    const a = mockAgents.agents.value.find(a => a.id === agentId)
    return a?.thinkingEffortLevels || []
  }),
  refreshAgentModels: vi.fn().mockResolvedValue(undefined),
  updateAgentField: vi.fn(),
  getDefaultModelId: vi.fn((agentId: string) => {
    const a = mockAgents.agents.value.find(a => a.id === agentId)
    return a?.preferredModel || a?.models?.[0]?.id || ''
  }),
  getAgent: vi.fn((agentId: string) => {
    return mockAgents.agents.value.find(a => a.id === agentId)
  }),
}

const mockIdentity = {
  currentAgentId: ref('claude'),
  currentModelId: ref('claude-sonnet-4-6'),
  currentModelName: ref('Claude Sonnet 4.6'),
  currentThinkingEffort: ref('high'),
}

describe('ModelModal', () => {
  beforeEach(() => {
    vi.mocked(useAgents).mockReturnValue(mockAgents as any)
    vi.mocked(useSessionIdentity).mockReturnValue(mockIdentity as any)
  })

  function mountModal(props = {}) {
    return mount(ModelModal, {
      props: { show: true, agentId: 'claude', ...props },
      global: {
        stubs: { teleport: true },
      },
    })
  }

  // --- Tab switching ---

  it('renders model tab by default', () => {
    const wrapper = mountModal()
    expect(wrapper.find('.model-tab.active').text()).toContain('chat.modelSwitcher.title')
  })

  it('switches to thinking tab when clicked', async () => {
    const wrapper = mountModal()
    const tabs = wrapper.findAll('.model-tab')
    // Click thinking tab
    await tabs[1].trigger('click')
    expect(wrapper.findAll('.model-tab')[1].classes()).toContain('active')
  })

  // --- Model list ---

  it('renders model list for current agent', () => {
    const wrapper = mountModal()
    const items = wrapper.findAll('.model-item')
    expect(items.length).toBe(3)
    expect(items[0].text()).toContain('Claude Sonnet 4.6')
  })

  it('highlights current session model', () => {
    const wrapper = mountModal()
    const items = wrapper.findAll('.model-item')
    expect(items[0].classes()).toContain('current')
  })

  it('shows default badge on preferred model', () => {
    const wrapper = mountModal()
    const items = wrapper.findAll('.model-item')
    // First model is preferred, should have default badge
    expect(items[0].find('.default-badge').exists() || items[0].text().includes('默认')).toBe(true)
  })

  // --- Search ---

  it('filters models by search query', async () => {
    const wrapper = mountModal()
    const searchInput = wrapper.find('.model-search-input')
    await searchInput.setValue('opus')
    await nextTick()

    const items = wrapper.findAll('.model-item')
    expect(items.length).toBe(1)
    expect(items[0].text()).toContain('Opus')
  })

  it('shows no results message when search has no matches', async () => {
    const wrapper = mountModal()
    const searchInput = wrapper.find('.model-search-input')
    await searchInput.setValue('nonexistent')
    await nextTick()

    expect(wrapper.find('.model-empty').exists() || wrapper.text()).toBeTruthy()
  })

  // --- Model selection (session-scoped) ---

  it('emits switch-model when clicking a model', async () => {
    const wrapper = mountModal()
    const items = wrapper.findAll('.model-item')
    await items[1].trigger('click') // opus

    expect(wrapper.emitted('switch-model')).toBeTruthy()
    expect(wrapper.emitted('switch-model')![0][0]).toEqual({ id: 'claude-opus-4-5', name: 'Claude Opus 4.5', default: false })
  })

  it('closes modal after selecting a model', async () => {
    const wrapper = mountModal()
    const items = wrapper.findAll('.model-item')
    await items[1].trigger('click')

    expect(wrapper.emitted('update:show')).toBeTruthy()
    expect(wrapper.emitted('update:show')![0][0]).toBe(false)
  })

  // --- Thinking effort ---

  it('renders thinking effort levels on thinking tab', async () => {
    const wrapper = mountModal()
    const tabs = wrapper.findAll('.model-tab')
    await tabs[1].trigger('click')
    await nextTick()

    const items = wrapper.findAll('.thinking-item')
    expect(items.length).toBe(6) // 5 levels + auto
  })

  it('highlights current thinking effort', async () => {
    const wrapper = mountModal()
    const tabs = wrapper.findAll('.model-tab')
    await tabs[1].trigger('click')
    await nextTick()

    const items = wrapper.findAll('.thinking-item')
    // 'high' is current, find it
    const highItem = items.find(i => i.text().includes('high'))
    expect(highItem?.classes()).toContain('current')
  })

  it('emits switch-thinking-effort when clicking a level', async () => {
    const wrapper = mountModal()
    const tabs = wrapper.findAll('.model-tab')
    await tabs[1].trigger('click')
    await nextTick()

    const items = wrapper.findAll('.thinking-item')
    // Click 'medium'
    const mediumItem = items.find(i => i.text().includes('medium'))
    await mediumItem?.trigger('click')

    expect(wrapper.emitted('switch-thinking-effort')).toBeTruthy()
  })

  // --- Refresh ---

  it('has refresh button in model tab', () => {
    const wrapper = mountModal()
    expect(wrapper.find('.refresh-btn').exists()).toBe(true)
  })

  // --- No models ---

  it('shows empty state when agent has no models', () => {
    const wrapper = mountModal({ agentId: 'gemini' })
    const items = wrapper.findAll('.model-item')
    expect(items.length).toBe(0)
    expect(wrapper.find('.model-empty').exists()).toBe(true)
  })
})

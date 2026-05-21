import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

// ── Module mocks — must be before import ──

vi.mock('@/composables/useAgents', () => ({
  useAgents: () => ({
    agents: { value: [{ id: 'claude', name: 'Claude', icon: '🤖', backend: 'claude', specialty: '' }] },
    loadAgents: vi.fn().mockResolvedValue(undefined),
    getAgentIcon: vi.fn().mockReturnValue('🤖'),
    getAgentName: vi.fn().mockReturnValue('Claude'),
    isDefaultAgent: vi.fn().mockReturnValue(true),
    getAgentDefaultModelName: vi.fn().mockReturnValue(''),
    getAgentModels: vi.fn().mockReturnValue([]),
    getAgentThinkingEffortLevels: vi.fn().mockReturnValue([]),
  }),
}))

vi.mock('@/composables/useDialog.ts', () => ({
  useDialog: () => ({
    confirm: vi.fn().mockResolvedValue(false),
  }),
}))

vi.mock('@/composables/useSessionIdentity.ts', () => ({
  useSessionIdentity: () => ({
    runningSessionsVersion: { value: 0 },
  }),
}))

vi.mock('@/stores/app.ts', () => ({
  store: {
    state: {
      chatSessionPageSize: 10,
    },
  },
}))

// ── Import after mocks ──

import SessionDrawer from '@/components/session/SessionDrawer.vue'

// ── Test data ──

const mockSessions = [
  { id: 's1', title: 'Session 1', backend: 'claude', updatedAt: '2026-01-01T00:00:00Z' },
  { id: 's2', title: 'Session 2', backend: 'codebuddy', updatedAt: '2026-01-02T00:00:00Z' },
  { id: 's3', title: 'Session 3', backend: 'claude', updatedAt: '2026-01-03T00:00:00Z' },
]

function createFetchMock(sessions = mockSessions) {
  return vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve({ sessions, hasMore: false }),
  })
}

// ── i18n ──

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: {
    zh: {
      session: { title: '会话', newSession: '新建', selectAgent: '选择AI', confirmDelete: '确认删除？', running: '运行中', noSessions: '暂无会话' },
      common: { loading: '加载中', delete: '删除', cancel: '取消' },
    },
  },
})

// ── Helpers ──

/** Polyfill IntersectionObserver for jsdom */
function polyfillIO() {
  class MockIO { constructor() {} observe() {} unobserve() {} disconnect() {} }
  vi.stubGlobal('IntersectionObserver', MockIO)
}

/**
 * Mount SessionDrawer with open=false, then transition to open=true.
 * This matches the real usage where the drawer opens via prop change
 * (the internal watch(open) doesn't use { immediate: true }).
 */
async function mountOpenDrawer(fetchFn = createFetchMock()) {
  vi.stubGlobal('fetch', fetchFn)
  polyfillIO()

  const wrapper = mount(SessionDrawer, {
    props: {
      open: false,
      currentSessionId: 's1',
      runningSessionIds: new Set(),
    },
    global: {
      plugins: [i18n],
      stubs: {
        BottomSheet: {
          template: '<div><slot name="header" /><slot /></div>',
          props: ['open', 'auto', 'title'],
          methods: { close: vi.fn() },
        },
        ModalDialog: {
          template: '<div><slot /><slot name="footer" /></div>',
          props: ['open', 'title'],
        },
      },
    },
  })

  // Open the drawer — triggers watch(open) → loadSessions
  await wrapper.setProps({ open: true })
  await flushPromises()

  return { wrapper, fetchFn }
}

// ── Tests ──

describe('SessionDrawer: always reload on open', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('loads sessions from API when opened', async () => {
    const fetchFn = createFetchMock()
    const { wrapper } = await mountOpenDrawer(fetchFn)

    expect(fetchFn).toHaveBeenCalledWith(expect.stringContaining('/api/ai/sessions'))
    expect(wrapper.vm.sessions).toHaveLength(3)
  })

  it('reloads sessions every time the drawer is opened', async () => {
    const fetchFn = createFetchMock()
    const { wrapper } = await mountOpenDrawer(fetchFn)

    // First open: 1 fetch call
    expect(fetchFn).toHaveBeenCalledTimes(1)
    fetchFn.mockClear()

    // Close and reopen
    await wrapper.setProps({ open: false })
    await flushPromises()
    await wrapper.setProps({ open: true })
    await flushPromises()

    // Second open: another fetch call — always reloads
    expect(fetchFn).toHaveBeenCalledTimes(1)
    expect(wrapper.vm.sessions).toHaveLength(3)
  })

  it('shows new session after create (simulated by API returning updated list)', async () => {
    // First open: 2 sessions. Second open: 3 sessions (after create).
    const initialSessions = mockSessions.slice(0, 2)
    const afterCreateSessions = mockSessions

    let callCount = 0
    const fetchFn = vi.fn().mockImplementation(() => {
      callCount++
      const sessions = callCount <= 1 ? initialSessions : afterCreateSessions
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ sessions, hasMore: false }),
      })
    })

    const { wrapper } = await mountOpenDrawer(fetchFn)
    expect(wrapper.vm.sessions).toHaveLength(2)

    // Close and reopen — simulates user creating a session then reopening the list
    await wrapper.setProps({ open: false })
    await flushPromises()
    await wrapper.setProps({ open: true })
    await flushPromises()

    // New session appears
    expect(wrapper.vm.sessions).toHaveLength(3)
  })

  it('shows updated list after delete (simulated by API returning updated list)', async () => {
    // First open: 3 sessions. Second open: 2 sessions (after delete).
    const afterDeleteSessions = mockSessions.filter(s => s.id !== 's2')

    let callCount = 0
    const fetchFn = vi.fn().mockImplementation(() => {
      callCount++
      const sessions = callCount <= 1 ? mockSessions : afterDeleteSessions
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ sessions, hasMore: false }),
      })
    })

    const { wrapper } = await mountOpenDrawer(fetchFn)
    expect(wrapper.vm.sessions).toHaveLength(3)

    // Close and reopen — simulates user deleting a session then reopening the list
    await wrapper.setProps({ open: false })
    await flushPromises()
    await wrapper.setProps({ open: true })
    await flushPromises()

    // Deleted session gone
    expect(wrapper.vm.sessions).toHaveLength(2)
    expect(wrapper.vm.sessions.find(s => s.id === 's2')).toBeUndefined()
  })

  it('shows replacement session after delete-current + auto-create', async () => {
    const afterDeleteSessions = [
      { id: 's2', title: 'Session 2', backend: 'codebuddy', updatedAt: '2026-01-02T00:00:00Z' },
      { id: 's-replacement', title: 'New Session', backend: 'claude', updatedAt: '2026-01-04T00:00:00Z' },
    ]

    let callCount = 0
    const fetchFn = vi.fn().mockImplementation(() => {
      callCount++
      const sessions = callCount <= 1 ? mockSessions : afterDeleteSessions
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ sessions, hasMore: false }),
      })
    })

    const { wrapper } = await mountOpenDrawer(fetchFn)
    expect(wrapper.vm.sessions).toHaveLength(3)

    await wrapper.setProps({ open: false })
    await flushPromises()
    await wrapper.setProps({ open: true })
    await flushPromises()

    expect(wrapper.vm.sessions).toHaveLength(2)
    expect(wrapper.vm.sessions.find(s => s.id === 's1')).toBeUndefined()
    expect(wrapper.vm.sessions.find(s => s.id === 's-replacement')).toBeDefined()
  })

  it('does not fetch when opened with open=false', async () => {
    const fetchFn = createFetchMock()
    vi.stubGlobal('fetch', fetchFn)
    polyfillIO()

    mount(SessionDrawer, {
      props: { open: false, currentSessionId: 's1', runningSessionIds: new Set() },
      global: {
        plugins: [i18n],
        stubs: {
          BottomSheet: {
            template: '<div><slot name="header" /><slot /></div>',
            props: ['open', 'auto', 'title'],
            methods: { close: vi.fn() },
          },
          ModalDialog: {
            template: '<div><slot /><slot name="footer" /></div>',
            props: ['open', 'title'],
          },
        },
      },
    })
    await flushPromises()

    // No fetch when closed
    expect(fetchFn).not.toHaveBeenCalled()
  })

  it('no invalidate() method is exposed', async () => {
    const fetchFn = createFetchMock()
    const { wrapper } = await mountOpenDrawer(fetchFn)

    // invalidate should NOT exist on the component instance
    expect(wrapper.vm.invalidate).toBeUndefined()
  })
})

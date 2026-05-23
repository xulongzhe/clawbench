import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import AppHeader from '@/components/common/AppHeader.vue'

// ── Mock setup ──
// AppHeader's computed(() => store.state.gitBranch) creates a reactive
// dependency on the mock store. When multiple instances are mounted,
// each instance's computed subscribes to store.state, and changing
// mockState between tests triggers cascading "Maximum recursive updates
// exceeded" from accumulated reactive effects.
//
// Workaround: only assert initial-render state. No setProps, triggers,
// or async interactions. The status-dot (outside PopupMenu) can be
// tested directly; status-indicator/value (inside PopupMenu) cannot be
// tested without opening the menu (which causes re-renders that trigger
// recursive updates from the store mock's reactive tracking).

// Mock static assets that can't be resolved in test environment
vi.mock('/logo.png', () => ({ default: '/logo.png' }))

const {
  loadGitBranchFn,
  setPendingManageNavigationFn,
  mockState,
  wsConfig,
  pushConfig,
} = vi.hoisted(() => ({
  loadGitBranchFn: vi.fn(),
  setPendingManageNavigationFn: vi.fn(),
  mockState: { gitBranch: '' },
  wsConfig: { value: 'connected' as string },
  pushConfig: { value: false as boolean },
}))

vi.mock('@/stores/app.ts', () => ({
  store: { state: mockState, loadGitBranch: loadGitBranchFn },
}))
vi.mock('@/composables/useGlobalEvents', () => {
  const vue = require('vue')
  return {
    useGlobalEvents: () => ({
      wsStatus: vue.ref(wsConfig.value),
      pushRegistered: vue.ref(pushConfig.value),
    }),
  }
})
vi.mock('@/composables/useCommitNavigation.ts', () => ({
  setPendingManageNavigation: setPendingManageNavigationFn,
}))

const i18n = createI18n({
  legacy: false, locale: 'en',
  messages: { en: { common: { loading: 'Loading...' }, appHeader: {
    switchProject: 'Switch project', selectProject: 'Select project',
    noRecentProjects: 'No recent projects', browse: 'Browse...',
    connectionStatus: 'Connection Status', wsConnected: 'Connected',
    wsReconnecting: 'Reconnecting...', wsDisconnected: 'Disconnected',
    pushRegistered: 'Registered', pushNotEnabled: 'Not Enabled',
    websocket: 'WebSocket', jpush: 'Push Notifications',
    projectPathNotFound: 'Project path does not exist or has been deleted',
    switchProjectFailed: 'Switch project failed: {error}',
    switchProjectNetworkError: 'Switch project failed: network error',
  } } },
})

const TeleportStub = { template: '<div class="teleport-stub"><slot /></div>' }
const PopupMenuStub = { template: '<div class="popup-menu-stub" v-if="$props.show"><slot /></div>', props: ['show','targetElement','maxWidth','maxHeight','menuItemsCount'] }
const LucideStub = { template: '<span class="lucide-stub" />' }

function mountHeader(props: Record<string, unknown> = {}) {
  return mount(AppHeader, {
    props: { projectRoot: '/home/user/my-project', hidden: false, ...props },
    global: {
      plugins: [i18n],
      stubs: { Teleport: TeleportStub, PopupMenu: PopupMenuStub, 'lucide-vue-next': LucideStub },
      provide: { switchTab: vi.fn(), toast: { show: vi.fn() }, hotSwitchProject: vi.fn() },
    },
  })
}

describe('AppHeader', () => {
  beforeEach(() => {
    wsConfig.value = 'connected'
    pushConfig.value = false
    mockState.gitBranch = ''
    loadGitBranchFn.mockReset()
    setPendingManageNavigationFn.mockReset()
  })

  // ── projectName computed (5) ──

  it('shows "Select project" when projectRoot is undefined', () => {
    expect(mountHeader({ projectRoot: undefined }).find('.project-name').text()).toBe('Select project')
  })
  it('shows "Select project" when projectRoot is empty', () => {
    expect(mountHeader({ projectRoot: '' }).find('.project-name').text()).toBe('Select project')
  })
  it('shows base name of the path', () => {
    expect(mountHeader({ projectRoot: '/home/user/my-project' }).find('.project-name').text()).toBe('my-project')
  })
  it('handles trailing slash', () => {
    expect(mountHeader({ projectRoot: '/home/user/my-project/' }).find('.project-name').text()).toBe('my-project')
  })
  it('handles deep nested path', () => {
    expect(mountHeader({ projectRoot: '/a/b/c/deep-project' }).find('.project-name').text()).toBe('deep-project')
  })

  // ── Connection status dot (3) ──
  // NOTE: .status-dot is outside PopupMenu and always rendered.
  // .status-indicator and .status-value are inside PopupMenu and
  // only visible when menu is open — testing those requires triggers
  // which cause recursive updates from the store mock.

  it('status dot - connected', () => {
    wsConfig.value = 'connected'
    expect(mountHeader().find('.status-dot').classes()).toContain('status-dot-connected')
  })
  it('status dot - reconnecting', () => {
    wsConfig.value = 'reconnecting'
    expect(mountHeader().find('.status-dot').classes()).toContain('status-dot-reconnecting')
  })
  it('status dot - disconnected', () => {
    wsConfig.value = 'disconnected'
    expect(mountHeader().find('.status-dot').classes()).toContain('status-dot-disconnected')
  })

  // ── Visibility (2) ──

  it('visible by default', () => {
    expect(mountHeader({ hidden: false }).find('.header').isVisible()).toBe(true)
  })
  it('hidden when hidden=true', () => {
    expect(mountHeader({ hidden: true }).find('.header').isVisible()).toBe(false)
  })

  // ── Structure (4) ──

  it('has logo', () => {
    expect(mountHeader().find('.header-logo').exists()).toBe(true)
  })
  it('has status toggle button', () => {
    expect(mountHeader().find('.status-toggle').exists()).toBe(true)
  })
  it('has project switch button', () => {
    expect(mountHeader().find('.project-switch-btn').exists()).toBe(true)
  })
  it('displays project name', () => {
    expect(mountHeader().find('.project-name').text()).toBe('my-project')
  })

  // ── Git branch (3) ──

  it('no badge when gitBranch is empty', () => {
    expect(mountHeader().find('.branch-badge').exists()).toBe(false)
  })
  it('shows badge when gitBranch is set before mount', () => {
    mockState.gitBranch = 'main'
    const wrapper = mountHeader()
    expect(wrapper.find('.branch-badge').exists()).toBe(true)
    expect(wrapper.find('.branch-name').text()).toBe('main')
  })
  it('uses gitBranch as title attribute', () => {
    mockState.gitBranch = 'feature/login'
    const wrapper = mountHeader()
    expect(wrapper.find('.branch-badge').attributes('title')).toBe('feature/login')
  })

  // ── loadGitBranch watcher (2) ──

  it('calls loadGitBranch on mount when projectRoot is truthy', () => {
    mountHeader({ projectRoot: '/home/user/my-project' })
    expect(loadGitBranchFn).toHaveBeenCalled()
  })
  it('does not call loadGitBranch on mount when projectRoot is empty', () => {
    mountHeader({ projectRoot: '' })
    expect(loadGitBranchFn).not.toHaveBeenCalled()
  })
})

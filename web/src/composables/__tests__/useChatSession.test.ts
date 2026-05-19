import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { ref } from 'vue'

// ── Hoisted mock state (plain objects, no Vue imports needed) ──

const { mockState, resetMockState } = vi.hoisted(() => {
  const mockState = {
    runningSessions: new Set<string>(),
    runningSessionsVersion: 0,
    currentSessionId: '',
    chatRunning: false,
    chatUnread: false,
    chatInitialMessages: 20,
    chatPageSize: 20,
  }
  function resetMockState() {
    mockState.runningSessions.clear()
    mockState.runningSessionsVersion = 0
    mockState.currentSessionId = ''
    mockState.chatRunning = false
    mockState.chatUnread = false
    mockState.chatInitialMessages = 20
    mockState.chatPageSize = 20
  }
  return { mockState, resetMockState }
})

// ── Mocks ──

vi.mock('@/composables/useSessionIdentity', () => ({
  useSessionIdentity: () => ({
    currentSessionId: { value: mockState.currentSessionId, get value() { return mockState.currentSessionId } },
    currentSessionTitle: { value: '' },
    currentBackend: { value: '' },
    currentAgentId: { value: '' },
    currentModelId: { value: '' },
    currentModelName: { value: '' },
    currentThinkingEffort: { value: '' },
    runningSessions: {
      get value() { return mockState.runningSessions },
    },
    runningSessionsVersion: {
      get value() { return mockState.runningSessionsVersion },
      set value(v: number) { mockState.runningSessionsVersion = v },
    },
    agentHeaderTitle: { value: '' },
    switchSession: vi.fn(),
    createSession: vi.fn(),
    deleteSession: vi.fn(),
    sendMessage: vi.fn(),
    openChatPanel: vi.fn(),
    registerSessionActions: vi.fn(),
    initSessionFromAPI: vi.fn(),
    saveModelPref: vi.fn(),
    saveThinkingPref: vi.fn(),
    loadModelPref: vi.fn(),
    loadThinkingPref: vi.fn(),
  }),
}))

vi.mock('@/composables/useToast', () => ({
  useToast: () => ({ show: vi.fn() }),
}))
vi.mock('@/composables/useNotification', () => ({
  useNotification: () => ({ play: vi.fn() }),
}))
vi.mock('@/composables/useAgents', () => ({
  useAgents: () => ({
    agents: { value: [] },
    loadAgents: vi.fn().mockResolvedValue(undefined),
    getAgentIcon: vi.fn().mockReturnValue('🤖'),
    getAgentName: vi.fn().mockReturnValue('Test'),
    syncModelFromAgent: vi.fn().mockReturnValue({ modelId: '', modelName: '' }),
    getAgentModel: vi.fn().mockReturnValue(undefined),
    agentHeaderTitle: vi.fn().mockReturnValue('🤖 Test'),
  }),
}))

vi.mock('@/stores/app', () => ({
  store: {
    get state() {
      return mockState
    },
  },
}))

vi.mock('@/utils/chatSessionUtils', () => ({
  buildMessageSnapshot: vi.fn().mockReturnValue(''),
  parseMessages: vi.fn().mockReturnValue([]),
}))

// ── Import after mocks ──

import { useChatSession } from '@/composables/useChatSession'

// ── Helpers ──

function createSession() {
  const options = {
    currentSessionId: ref('current-s1'),
    messages: ref([]),
    loading: ref(false),
    inputDisabled: ref(false),
    blockTasks: {},
    blockAskQuestions: {},
    expandedTools: ref({}),
    onParseAssistantContent: vi.fn(),
    onExtractScheduledTasks: vi.fn(),
    onRenderUpdate: vi.fn(),
    onScrollBottom: vi.fn(),
    onConnectStream: vi.fn(),
    onStopPolling: vi.fn(),
    onDisconnectStream: vi.fn(),
    onOpen: vi.fn(),
  }
  return useChatSession(options)
}

// ── Tests ──

describe('onSessionEvent', () => {
  beforeEach(() => {
    resetMockState()
  })

  it('does nothing when data is null', () => {
    const session = createSession()
    const versionBefore = mockState.runningSessionsVersion
    session.onSessionEvent(null as any)
    expect(mockState.chatRunning).toBe(false)
    expect(mockState.runningSessions.size).toBe(0)
    expect(mockState.runningSessionsVersion).toBe(versionBefore)
  })

  it('does nothing when data is undefined', () => {
    const session = createSession()
    const versionBefore = mockState.runningSessionsVersion
    session.onSessionEvent(undefined)
    expect(mockState.chatRunning).toBe(false)
    expect(mockState.runningSessions.size).toBe(0)
    expect(mockState.runningSessionsVersion).toBe(versionBefore)
  })

  it('sets chatRunning=true and adds session to runningSessions on status=running', () => {
    const session = createSession()
    session.onSessionEvent({ session_id: 's1', status: 'running' })
    expect(mockState.chatRunning).toBe(true)
    expect(mockState.runningSessions.has('s1')).toBe(true)
    expect(mockState.runningSessionsVersion).toBe(1)
  })

  it('sets chatRunning=true but does not add to set when session_id is missing on running', () => {
    const session = createSession()
    session.onSessionEvent({ status: 'running' })
    expect(mockState.chatRunning).toBe(true)
    expect(mockState.runningSessions.size).toBe(0)
    expect(mockState.runningSessionsVersion).toBe(0)
  })

  it('removes session from runningSessions and derives chatRunning from set on status=completed', () => {
    const session = createSession()
    // Start two sessions
    session.onSessionEvent({ session_id: 's1', status: 'running' })
    session.onSessionEvent({ session_id: 's2', status: 'running' })
    expect(mockState.runningSessions.size).toBe(2)

    // Complete s1 — s2 still running
    session.onSessionEvent({ session_id: 's1', status: 'completed' })
    expect(mockState.runningSessions.has('s1')).toBe(false)
    expect(mockState.runningSessions.has('s2')).toBe(true)
    expect(mockState.chatRunning).toBe(true)
  })

  it('sets chatRunning=false when last running session completes', () => {
    const session = createSession()
    session.onSessionEvent({ session_id: 's1', status: 'running' })
    expect(mockState.chatRunning).toBe(true)

    session.onSessionEvent({ session_id: 's1', status: 'completed' })
    expect(mockState.runningSessions.size).toBe(0)
    expect(mockState.chatRunning).toBe(false)
  })

  it('marks chatUnread=true when a different session completes', () => {
    const session = createSession()
    mockState.currentSessionId = 'current-s1'

    session.onSessionEvent({ session_id: 's1', status: 'running' })
    // A different session completes
    session.onSessionEvent({ session_id: 's2', status: 'completed' })
    expect(mockState.chatUnread).toBe(true)
  })

  it('does not mark chatUnread when the current session completes', () => {
    const session = createSession()
    mockState.currentSessionId = 'current-s1'

    session.onSessionEvent({ session_id: 'current-s1', status: 'running' })
    session.onSessionEvent({ session_id: 'current-s1', status: 'completed' })
    expect(mockState.chatUnread).toBe(false)
  })

  it('handles status=cancelled by removing from runningSessions', () => {
    const session = createSession()
    session.onSessionEvent({ session_id: 's1', status: 'running' })
    expect(mockState.runningSessions.has('s1')).toBe(true)

    session.onSessionEvent({ session_id: 's1', status: 'cancelled' })
    expect(mockState.runningSessions.has('s1')).toBe(false)
    expect(mockState.chatRunning).toBe(false)
  })

  it('increments runningSessionsVersion on each add and delete', () => {
    const session = createSession()
    expect(mockState.runningSessionsVersion).toBe(0)

    session.onSessionEvent({ session_id: 's1', status: 'running' })
    expect(mockState.runningSessionsVersion).toBe(1)

    session.onSessionEvent({ session_id: 's2', status: 'running' })
    expect(mockState.runningSessionsVersion).toBe(2)

    session.onSessionEvent({ session_id: 's1', status: 'completed' })
    expect(mockState.runningSessionsVersion).toBe(3)
  })

  it('handles multiple concurrent sessions correctly', () => {
    const session = createSession()

    // Start 3 sessions
    session.onSessionEvent({ session_id: 's1', status: 'running' })
    session.onSessionEvent({ session_id: 's2', status: 'running' })
    session.onSessionEvent({ session_id: 's3', status: 'running' })

    expect(mockState.chatRunning).toBe(true)
    expect(mockState.runningSessions.size).toBe(3)

    // Complete s2 — s1 and s3 still running
    session.onSessionEvent({ session_id: 's2', status: 'completed' })
    expect(mockState.chatRunning).toBe(true)
    expect(mockState.runningSessions.has('s2')).toBe(false)
    expect(mockState.runningSessions.has('s1')).toBe(true)
    expect(mockState.runningSessions.has('s3')).toBe(true)

    // Complete s3
    session.onSessionEvent({ session_id: 's3', status: 'completed' })
    expect(mockState.chatRunning).toBe(true)

    // Complete s1
    session.onSessionEvent({ session_id: 's1', status: 'completed' })
    expect(mockState.chatRunning).toBe(false)
    expect(mockState.runningSessions.size).toBe(0)
  })

  it('does not increment version when completing a session without session_id', () => {
    const session = createSession()
    const versionBefore = mockState.runningSessionsVersion
    // completed without session_id — no sid to delete, no version bump
    session.onSessionEvent({ status: 'completed' })
    expect(mockState.runningSessionsVersion).toBe(versionBefore)
  })

  it('session running status is determined by both runningSessions Set and API running field', () => {
    // Simulates SessionDrawer's sessionsWithStatus logic:
    //   running: runningSessionIds.has(s.id) || !!s.running
    const session = createSession()

    // Scenario 1: WS event marks session as running (no API data yet)
    session.onSessionEvent({ session_id: 's1', status: 'running' })
    const runningSessionIds = mockState.runningSessions
    // s1 is in the set → should show as running
    expect(runningSessionIds.has('s1') || false).toBe(true)

    // Scenario 2: API returns running=true, but WS event hasn't arrived yet
    // (s2 is NOT in the set, but API would say running=true)
    const s2FromAPI = { id: 's2', running: true }
    expect(runningSessionIds.has('s2') || !!s2FromAPI.running).toBe(true)

    // Scenario 3: Session completed via WS but API still has stale data
    // (after onSessionEvent, s1 is removed from set)
    session.onSessionEvent({ session_id: 's1', status: 'completed' })
    expect(runningSessionIds.has('s1')).toBe(false)

    // The merged logic ensures a session shows as running if EITHER source says so
    // This covers the gap where TrySetSessionRunning's WS event arrives
    // before loadSessions is called
  })

  it('ignores data with empty/undefined status', () => {
    const session = createSession()
    const versionBefore = mockState.runningSessionsVersion

    // status is empty string → falls into else branch (treated as not-running)
    session.onSessionEvent({ session_id: 's1', status: '' })
    // No session_id in the Set (it was never added), but chatRunning derives from set size
    expect(mockState.chatRunning).toBe(false)
    // session_id is present → delete from empty set is a no-op, but version still increments
    expect(mockState.runningSessionsVersion).toBe(versionBefore + 1)
  })

  it('handles data with undefined status (missing key)', () => {
    const session = createSession()
    const versionBefore = mockState.runningSessionsVersion

    // status is undefined → else branch
    session.onSessionEvent({ session_id: 's1' })
    expect(mockState.chatRunning).toBe(false)
    expect(mockState.runningSessionsVersion).toBe(versionBefore + 1)
  })

  it('does not add duplicate entries to runningSessions for same session_id', () => {
    const session = createSession()

    session.onSessionEvent({ session_id: 's1', status: 'running' })
    expect(mockState.runningSessions.size).toBe(1)

    // Duplicate running event — Set.add is idempotent
    session.onSessionEvent({ session_id: 's1', status: 'running' })
    expect(mockState.runningSessions.size).toBe(1)
    // But version still increments
    expect(mockState.runningSessionsVersion).toBe(2)
  })

  it('completing a non-running session does not crash', () => {
    const session = createSession()

    // Complete a session that was never started
    expect(() => {
      session.onSessionEvent({ session_id: 'ghost', status: 'completed' })
    }).not.toThrow()
    expect(mockState.runningSessions.has('ghost')).toBe(false)
    expect(mockState.chatRunning).toBe(false)
  })

  it('preserves chatUnread=true when completing a non-current session even if already unread', () => {
    const session = createSession()
    mockState.currentSessionId = 'current-s1'
    mockState.chatUnread = true

    session.onSessionEvent({ session_id: 's2', status: 'completed' })
    // Should remain true (not reset to false)
    expect(mockState.chatUnread).toBe(true)
  })

  it('marks chatUnread on cancelled status for non-current session', () => {
    const session = createSession()
    mockState.currentSessionId = 'current-s1'

    session.onSessionEvent({ session_id: 's1', status: 'running' })
    // Cancel a different session
    session.onSessionEvent({ session_id: 's2', status: 'cancelled' })
    expect(mockState.chatUnread).toBe(true)
  })
})

// ── loadSessionsOnce tests ──

// Need to re-import loadSessionsOnce with a separate mock setup for fetch
describe('loadSessionsOnce', () => {
  let originalFetch: typeof globalThis.fetch

  beforeEach(() => {
    resetMockState()
    originalFetch = globalThis.fetch
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('populates runningSessions from API response with running sessions', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 's1', running: true },
          { id: 's2', running: false },
          { id: 's3', running: true },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.runningSessions.has('s1')).toBe(true)
    expect(mockState.runningSessions.has('s2')).toBe(false)
    expect(mockState.runningSessions.has('s3')).toBe(true)
    expect(mockState.runningSessions.size).toBe(2)
    expect(mockState.chatRunning).toBe(true)
    expect(mockState.runningSessionsVersion).toBeGreaterThan(0)
  })

  it('clears runningSessions and sets chatRunning=false when no sessions are running', async () => {
    // Pre-populate with a running session
    mockState.runningSessions.add('old-session')
    mockState.chatRunning = true

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 's1', running: false },
          { id: 's2', running: false },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.runningSessions.size).toBe(0)
    expect(mockState.chatRunning).toBe(false)
  })

  it('does not throw on fetch failure', async () => {
    globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error'))

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    // Should not throw
    await expect(loadSessionsOnce()).resolves.toBeUndefined()
  })

  it('does not throw on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await expect(loadSessionsOnce()).resolves.toBeUndefined()
  })

  it('increments runningSessionsVersion after populating', async () => {
    const versionBefore = mockState.runningSessionsVersion

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [{ id: 's1', running: true }],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.runningSessionsVersion).toBeGreaterThan(versionBefore)
  })

  // ── chatUnread recalculation tests ──

  it('sets chatUnread=true when another session has unreadCount > 0', async () => {
    mockState.currentSessionId = 'current-s1'
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 'current-s1', unreadCount: 0, running: false },
          { id: 's2', unreadCount: 3, running: false },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.chatUnread).toBe(true)
  })

  it('sets chatUnread=false when no other session has unreadCount > 0', async () => {
    mockState.currentSessionId = 'current-s1'
    mockState.chatUnread = true  // pre-set to true
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 'current-s1', unreadCount: 2, running: false },
          { id: 's2', unreadCount: 0, running: false },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    // current session's unreadCount is ignored; s2 has 0 → chatUnread = false
    expect(mockState.chatUnread).toBe(false)
  })

  it('sets chatUnread=false when all sessions are read', async () => {
    mockState.currentSessionId = 'current-s1'
    mockState.chatUnread = true
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 'current-s1', unreadCount: 0, running: false },
          { id: 's2', unreadCount: 0, running: false },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.chatUnread).toBe(false)
  })

  it('ignores current session unreadCount even if it is > 0', async () => {
    // Key behavior: only OTHER sessions' unread counts matter
    mockState.currentSessionId = 'current-s1'
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 'current-s1', unreadCount: 5, running: false },  // current — ignored
          { id: 's2', unreadCount: 0, running: false },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.chatUnread).toBe(false)
  })

  it('handles empty sessions array', async () => {
    mockState.chatUnread = true
    mockState.chatRunning = true
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ sessions: [] }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.chatUnread).toBe(false)
    expect(mockState.chatRunning).toBe(false)
  })

  it('does not change chatUnread/chatRunning when fetch is not ok', async () => {
    mockState.chatUnread = true
    mockState.chatRunning = true
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 500 })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.chatUnread).toBe(true)
    expect(mockState.chatRunning).toBe(true)
  })

  it('does not change chatUnread/chatRunning when fetch throws', async () => {
    mockState.chatUnread = true
    mockState.chatRunning = true
    globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error'))

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.chatUnread).toBe(true)
    expect(mockState.chatRunning).toBe(true)
  })

  it('clears stale runningSessions before repopulating', async () => {
    // Pre-populate with sessions that are no longer running
    mockState.runningSessions.add('old-1')
    mockState.runningSessions.add('old-2')

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 'new-1', running: true },
          { id: 'old-1', running: false },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    // old entries should be cleared, only new-1 should remain
    expect(mockState.runningSessions.has('old-1')).toBe(false)
    expect(mockState.runningSessions.has('old-2')).toBe(false)
    expect(mockState.runningSessions.has('new-1')).toBe(true)
    expect(mockState.runningSessions.size).toBe(1)
  })

  it('handles json() throwing an error', async () => {
    mockState.chatRunning = true
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.reject(new SyntaxError('Unexpected token')),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    // Should not throw
    await expect(loadSessionsOnce()).resolves.toBeUndefined()
    // State should not change (error was caught)
    expect(mockState.chatRunning).toBe(true)
  })

  it('handles missing sessions field in response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({}),  // no sessions field
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.chatRunning).toBe(false)
    expect(mockState.runningSessions.size).toBe(0)
  })

  it('handles sessions=null in response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ sessions: null }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.chatRunning).toBe(false)
    expect(mockState.runningSessions.size).toBe(0)
  })

  it('does not clear runningSessions when fetch is not ok', async () => {
    mockState.runningSessions.add('s1')
    mockState.chatRunning = true

    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 500 })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    // Pre-existing data should not be cleared on failed fetch
    expect(mockState.runningSessions.has('s1')).toBe(true)
    expect(mockState.chatRunning).toBe(true)
  })

  it('does not clear runningSessions when fetch throws', async () => {
    mockState.runningSessions.add('s1')
    mockState.chatRunning = true

    globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error'))

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    expect(mockState.runningSessions.has('s1')).toBe(true)
    expect(mockState.chatRunning).toBe(true)
  })
})

// ───────────────────────────────────────────────────────────
// switchSession — recalculate chatUnread after switching
// ───────────────────────────────────────────────────────────

describe('switchSession', () => {
  let originalFetch: typeof globalThis.fetch

  beforeEach(() => {
    resetMockState()
    mockState.currentSessionId = 'current-s1'
    originalFetch = globalThis.fetch
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('calls loadSessionsOnce after successful switch to recalculate chatUnread', async () => {
    // First call: GET /api/ai/chat?session_id=s2 (switchSession fetch)
    // Second call: GET /api/ai/sessions (loadSessionsOnce fetch)
    globalThis.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessionId: 's2',
          messages: [],
          total: 0,
          backend: 'claude',
          agentId: 'agent1',
          modelId: '',
          thinkingEffort: '',
          running: false,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessions: [
            { id: 's2', unreadCount: 0, running: false },
          ],
        }),
      })

    const session = createSession()
    await session.switchSession('s2')

    // After switching to s2 and recalculating, no unread sessions remain
    expect(mockState.chatUnread).toBe(false)
    // fetch called twice: once for chat history, once for sessions list
    expect(globalThis.fetch).toHaveBeenCalledTimes(2)
  })

  it('clears chatUnread after switching when all sessions are read', async () => {
    mockState.chatUnread = true  // was flashing before

    globalThis.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessionId: 's2',
          messages: [],
          total: 0,
          backend: 'claude',
          agentId: 'agent1',
          modelId: '',
          thinkingEffort: '',
          running: false,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessions: [
            { id: 's2', unreadCount: 0, running: false },
          ],
        }),
      })

    const session = createSession()
    await session.switchSession('s2')

    expect(mockState.chatUnread).toBe(false)
  })

  it('keeps chatUnread=true after switching when other sessions still have unread messages', async () => {
    mockState.chatUnread = true

    // Switch to s2, but s3 still has unread messages
    globalThis.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessionId: 's2',
          messages: [],
          total: 0,
          backend: 'claude',
          agentId: 'agent1',
          modelId: '',
          thinkingEffort: '',
          running: false,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessions: [
            { id: 's2', unreadCount: 0, running: false },
            { id: 's3', unreadCount: 2, running: false },
          ],
        }),
      })

    const session = createSession()
    await session.switchSession('s2')

    // s3 is still unread — flashing should continue
    expect(mockState.chatUnread).toBe(true)
  })

  it('sets inputDisabled=false even when switchSession fetch fails', async () => {
    globalThis.fetch = vi.fn().mockRejectedValueOnce(new Error('Network error'))

    const inputDisabled = ref(false)
    const options = {
      currentSessionId: ref('current-s1'),
      messages: ref([]),
      loading: ref(false),
      inputDisabled,
      blockTasks: {},
      blockAskQuestions: {},
      expandedTools: ref({}),
      onParseAssistantContent: vi.fn(),
      onExtractScheduledTasks: vi.fn(),
      onRenderUpdate: vi.fn(),
      onScrollBottom: vi.fn(),
      onConnectStream: vi.fn(),
      onStopPolling: vi.fn(),
      onDisconnectStream: vi.fn(),
      onOpen: vi.fn(),
    }
    const session = useChatSession(options)

    await session.switchSession('s2')

    // inputDisabled must be restored in finally block
    expect(inputDisabled.value).toBe(false)
  })

  it('does not call loadSessionsOnce when switchSession fetch is not ok', async () => {
    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      json: () => Promise.resolve({ error: 'not found' }),
    })

    const session = createSession()
    await session.switchSession('s2')

    // Only one fetch call (the failed chat request), no sessions fetch
    expect(globalThis.fetch).toHaveBeenCalledTimes(1)
  })
})

// ───────────────────────────────────────────────────────────
// Integration: onSessionEvent → switchSession clears chatUnread
// ───────────────────────────────────────────────────────────

describe('chatUnread integration', () => {
  let originalFetch: typeof globalThis.fetch

  beforeEach(() => {
    resetMockState()
    mockState.currentSessionId = 's1'
    originalFetch = globalThis.fetch
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('chatUnread set by onSessionEvent is cleared after switchSession recalculates', async () => {
    const session = createSession()
    mockState.currentSessionId = 's1'

    // Session s2 completes in the background → marks unread
    session.onSessionEvent({ session_id: 's2', status: 'completed' })
    expect(mockState.chatUnread).toBe(true)

    // User switches to s2 (backend UpdateLastRead marks it as read)
    globalThis.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessionId: 's2',
          messages: [],
          total: 0,
          backend: 'claude',
          agentId: 'agent1',
          modelId: '',
          thinkingEffort: '',
          running: false,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessions: [
            { id: 's2', unreadCount: 0, running: false },
          ],
        }),
      })

    await session.switchSession('s2')

    expect(mockState.chatUnread).toBe(false)
  })

  it('chatUnread stays true when switching to one unread session but another still has unread', async () => {
    const session = createSession()
    mockState.currentSessionId = 's1'

    // Both s2 and s3 complete in the background
    session.onSessionEvent({ session_id: 's2', status: 'completed' })
    session.onSessionEvent({ session_id: 's3', status: 'completed' })
    expect(mockState.chatUnread).toBe(true)

    // User switches to s2, but s3 is still unread
    globalThis.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessionId: 's2',
          messages: [],
          total: 0,
          backend: 'claude',
          agentId: 'agent1',
          modelId: '',
          thinkingEffort: '',
          running: false,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessions: [
            { id: 's2', unreadCount: 0, running: false },
            { id: 's3', unreadCount: 1, running: false },
          ],
        }),
      })

    await session.switchSession('s2')

    // s3 still has unread → chatUnread should stay true
    expect(mockState.chatUnread).toBe(true)
  })

  it('chatUnread is not set when current session completes', () => {
    const session = createSession()
    // createSession sets options.currentSessionId = ref('current-s1')
    // onSessionEvent compares against the options ref, not mockState
    mockState.currentSessionId = 'current-s1'

    session.onSessionEvent({ session_id: 'current-s1', status: 'completed' })
    expect(mockState.chatUnread).toBe(false)
  })

  it('simulates the bug scenario: user on chat tab, other session completes, switch clears unread', async () => {
    // Exact scenario from the bug report:
    // 1. User is on chat tab viewing s1
    // 2. Session s2 completes → chatUnread = true, Dock/session button flashes
    // 3. User switches to s2 → chatUnread should be recalculated to false
    const session = createSession()
    mockState.currentSessionId = 's1'

    // Step 2: s2 completes in the background
    session.onSessionEvent({ session_id: 's2', status: 'completed' })
    expect(mockState.chatUnread).toBe(true)

    // Step 3: User switches to s2 (e.g. from SessionDrawer)
    globalThis.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessionId: 's2',
          messages: [],
          total: 0,
          backend: 'claude',
          agentId: 'agent1',
          modelId: '',
          thinkingEffort: '',
          running: false,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          sessions: [
            { id: 's2', unreadCount: 0, running: false },
          ],
        }),
      })

    await session.switchSession('s2')

    // Bug is fixed: chatUnread is recalculated and cleared
    expect(mockState.chatUnread).toBe(false)
  })

  it('simulates the bug scenario: user switches to chat tab but does not open unread session', async () => {
    // Second bug scenario:
    // 1. User is on another tab
    // 2. Session s2 completes → chatUnread = true
    // 3. User clicks Dock chat button → switchTab('chat') calls loadSessionsOnce()
    // 4. loadSessionsOnce should recalculate: s2 still has unreadCount > 0 → chatUnread stays true
    mockState.currentSessionId = 's1'
    mockState.chatUnread = true  // was set by onSessionEvent

    // switchTab('chat') now calls loadSessionsOnce() instead of blindly clearing
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 's1', unreadCount: 0, running: false },
          { id: 's2', unreadCount: 2, running: false },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    // chatUnread should remain true — user hasn't opened s2 yet
    expect(mockState.chatUnread).toBe(true)
  })

  it('loadSessionsOnce after stream done clears chatUnread for current session', async () => {
    // Bug #10 scenario: user views session s1, AI finishes, chatUnread should be recalculated
    // After AI finishes, loadHistory calls UpdateLastRead, so the API returns unreadCount=0 for s1.
    // loadSessionsOnce should then set chatUnread=false.
    mockState.currentSessionId = 's1'
    mockState.chatUnread = true  // was set incorrectly during initial load

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 's1', unreadCount: 0, running: false },  // current session, now read
          { id: 's2', unreadCount: 0, running: false },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    // chatUnread should be cleared — no other sessions have unread messages
    expect(mockState.chatUnread).toBe(false)
  })

  it('chatUnread stays false after loadSessionsOnce when only current session has unread', async () => {
    // Edge case: current session has unreadCount > 0 but it's the current one
    // This can happen if UpdateLastRead hasn't been called yet
    mockState.currentSessionId = 's1'
    mockState.chatUnread = false

    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        sessions: [
          { id: 's1', unreadCount: 5, running: false },  // current session — ignored
          { id: 's2', unreadCount: 0, running: false },
        ],
      }),
    })

    const { loadSessionsOnce } = await import('@/composables/useChatSession')
    await loadSessionsOnce()

    // Current session's unreadCount is excluded, s2 has 0 → chatUnread = false
    expect(mockState.chatUnread).toBe(false)
  })
})

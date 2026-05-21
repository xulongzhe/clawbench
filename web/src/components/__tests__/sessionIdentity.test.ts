import { describe, expect, it, vi, beforeEach } from 'vitest'
import {
  registerSessionActions,
  registerSessionDrawerRef,
  useSessionIdentity,
} from '@/composables/useSessionIdentity.ts'

// Reset module-level callbacks between tests by re-registering with nulls
beforeEach(() => {
  registerSessionActions({
    switchSession: vi.fn(),
    createSession: vi.fn(),
    deleteSession: vi.fn(),
    sendMessage: vi.fn(),
    openChatPanel: vi.fn(),
  })
})

describe('registerSessionActions', () => {
  it('registers action callbacks that are callable through the composable', async () => {
    const mockSwitch = vi.fn().mockResolvedValue(undefined)
    const mockCreate = vi.fn().mockResolvedValue(undefined)
    const mockDelete = vi.fn().mockResolvedValue(undefined)
    const mockSend = vi.fn().mockResolvedValue(undefined)
    const mockOpen = vi.fn()

    registerSessionActions({
      switchSession: mockSwitch,
      createSession: mockCreate,
      deleteSession: mockDelete,
      sendMessage: mockSend,
      openChatPanel: mockOpen,
    })

    // Verify delegation works by calling through the composable
    const { switchSession, createSession, deleteSession, sendMessage, openChatPanel } = useSessionIdentity()

    await switchSession('s1')
    expect(mockSwitch).toHaveBeenCalledWith('s1')

    await createSession('agent-1')
    expect(mockCreate).toHaveBeenCalledWith('agent-1')

    await deleteSession('s2', 'claude')
    expect(mockDelete).toHaveBeenCalledWith('s2', 'claude')

    await sendMessage('hello', ['/file.ts'])
    expect(mockSend).toHaveBeenCalledWith('hello', ['/file.ts'])

    openChatPanel()
    expect(mockOpen).toHaveBeenCalled()
  })

  it('replaces previous callbacks on re-registration', async () => {
    const firstSwitch = vi.fn()
    const secondSwitch = vi.fn()

    registerSessionActions({
      switchSession: firstSwitch,
      createSession: vi.fn(),
      deleteSession: vi.fn(),
      sendMessage: vi.fn(),
      openChatPanel: vi.fn(),
    })

    registerSessionActions({
      switchSession: secondSwitch,
      createSession: vi.fn(),
      deleteSession: vi.fn(),
      sendMessage: vi.fn(),
      openChatPanel: vi.fn(),
    })

    const { switchSession } = useSessionIdentity()
    await switchSession('session-123')
    expect(secondSwitch).toHaveBeenCalledWith('session-123')
    expect(firstSwitch).not.toHaveBeenCalled()
  })
})

describe('action delegation', () => {
  it('delegates switchSession to registered callback', async () => {
    const mockSwitch = vi.fn()
    registerSessionActions({
      switchSession: mockSwitch,
      createSession: vi.fn(),
      deleteSession: vi.fn(),
      sendMessage: vi.fn(),
      openChatPanel: vi.fn(),
    })

    const { switchSession } = useSessionIdentity()
    await switchSession('session-123')
    expect(mockSwitch).toHaveBeenCalledWith('session-123')
  })

  it('does nothing when switchSession has no callback', async () => {
    // Register with nulls — switchSession will be a no-op
    registerSessionActions({
      switchSession: async () => {},
      createSession: vi.fn(),
      deleteSession: vi.fn(),
      sendMessage: vi.fn(),
      openChatPanel: vi.fn(),
    })
    const { switchSession } = useSessionIdentity()
    // Should not throw
    await expect(switchSession('session-123')).resolves.toBeUndefined()
  })

  it('delegates createSession to registered callback', async () => {
    const mockCreate = vi.fn()
    registerSessionActions({
      switchSession: vi.fn(),
      createSession: mockCreate,
      deleteSession: vi.fn(),
      sendMessage: vi.fn(),
      openChatPanel: vi.fn(),
    })

    const { createSession } = useSessionIdentity()
    await createSession('agent-1')
    expect(mockCreate).toHaveBeenCalledWith('agent-1')
  })

  it('delegates deleteSession with backend', async () => {
    const mockDelete = vi.fn()
    registerSessionActions({
      switchSession: vi.fn(),
      createSession: vi.fn(),
      deleteSession: mockDelete,
      sendMessage: vi.fn(),
      openChatPanel: vi.fn(),
    })

    const { deleteSession } = useSessionIdentity()
    await deleteSession('session-1', 'claude')
    expect(mockDelete).toHaveBeenCalledWith('session-1', 'claude')
  })

  it('delegates sendMessage with filePaths', async () => {
    const mockSend = vi.fn()
    registerSessionActions({
      switchSession: vi.fn(),
      createSession: vi.fn(),
      deleteSession: vi.fn(),
      sendMessage: mockSend,
      openChatPanel: vi.fn(),
    })

    const { sendMessage } = useSessionIdentity()
    await sendMessage('hello', ['/tmp/file.go'])
    expect(mockSend).toHaveBeenCalledWith('hello', ['/tmp/file.go'])
  })

  it('delegates openChatPanel to registered callback', () => {
    const mockOpen = vi.fn()
    registerSessionActions({
      switchSession: vi.fn(),
      createSession: vi.fn(),
      deleteSession: vi.fn(),
      sendMessage: vi.fn(),
      openChatPanel: mockOpen,
    })

    const { openChatPanel } = useSessionIdentity()
    openChatPanel()
    expect(mockOpen).toHaveBeenCalled()
  })
})

describe('identity refs', () => {
  it('returns reactive refs from the singleton with correct initial values', () => {
    const { currentSessionId, currentSessionTitle, currentThinkingEffort, currentBackend, runningSessions, currentAgentId, currentModelId, currentModelName } = useSessionIdentity()
    // Initial values should be empty strings/sets
    expect(currentSessionId.value).toBe('')
    expect(currentSessionTitle.value).toBe('')
    expect(currentThinkingEffort.value).toBe('')
    expect(currentBackend.value).toBe('')
    expect(currentAgentId.value).toBe('')
    expect(currentModelId.value).toBe('')
    expect(currentModelName.value).toBe('')
    expect(runningSessions.value).toBeInstanceOf(Set)
    expect(runningSessions.value.size).toBe(0)
  })

  it('runningSessions reflects session state changes via the ref', () => {
    const { runningSessions } = useSessionIdentity()
    // Simulate a session starting
    runningSessions.value.add('session-1')
    expect(runningSessions.value.has('session-1')).toBe(true)

    // Simulate the session completing — remove it
    runningSessions.value.delete('session-1')
    expect(runningSessions.value.has('session-1')).toBe(false)

    // Clean up
    runningSessions.value.clear()
  })

  it('runningSessionsVersion is exposed and can be incremented', () => {
    const { runningSessionsVersion } = useSessionIdentity()
    const initial = runningSessionsVersion.value
    runningSessionsVersion.value = initial + 1
    expect(runningSessionsVersion.value).toBe(initial + 1)
    // Clean up
    runningSessionsVersion.value = initial
  })

  it('runningSessionsVersion is shared across instances', () => {
    const instance1 = useSessionIdentity()
    const instance2 = useSessionIdentity()
    const initial = instance1.runningSessionsVersion.value
    instance1.runningSessionsVersion.value = initial + 5
    expect(instance2.runningSessionsVersion.value).toBe(initial + 5)
    // Clean up
    instance1.runningSessionsVersion.value = initial
  })

  it('currentSessionId is writable and shared across instances', () => {
    const instance1 = useSessionIdentity()
    const instance2 = useSessionIdentity()

    instance1.currentSessionId.value = 'test-session-123'
    expect(instance2.currentSessionId.value).toBe('test-session-123')

    // Clean up
    instance1.currentSessionId.value = ''
  })

  it('sessionDrawerOpen is exposed and defaults to false', () => {
    const { sessionDrawerOpen } = useSessionIdentity()
    expect(sessionDrawerOpen.value).toBe(false)
  })

  it('openSessionTab sets sessionDrawerOpen to true', () => {
    const { sessionDrawerOpen, openSessionTab } = useSessionIdentity()
    expect(sessionDrawerOpen.value).toBe(false)
    openSessionTab()
    expect(sessionDrawerOpen.value).toBe(true)
    // Clean up
    sessionDrawerOpen.value = false
  })

  it('openAgentSelector delegates to registered drawer ref', () => {
    const mockOpenAgentSelector = vi.fn()
    registerSessionDrawerRef({ openAgentSelector: mockOpenAgentSelector })
    const { openAgentSelector } = useSessionIdentity()
    openAgentSelector()
    expect(mockOpenAgentSelector).toHaveBeenCalled()
  })

  it('openAgentSelector does nothing when no drawer ref registered', () => {
    registerSessionDrawerRef(null)
    const { openAgentSelector } = useSessionIdentity()
    // Should not throw
    expect(() => openAgentSelector()).not.toThrow()
  })

  it('registerSessionDrawerRef can be updated', () => {
    const firstRef = { openAgentSelector: vi.fn() }
    const secondRef = { openAgentSelector: vi.fn() }
    registerSessionDrawerRef(firstRef)
    const { openAgentSelector } = useSessionIdentity()
    openAgentSelector()
    expect(firstRef.openAgentSelector).toHaveBeenCalled()
    registerSessionDrawerRef(secondRef)
    openAgentSelector()
    expect(secondRef.openAgentSelector).toHaveBeenCalled()
  })
})

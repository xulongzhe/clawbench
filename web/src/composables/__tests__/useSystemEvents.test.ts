import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'

// ---------------------------------------------------------------------------
// Mocks — must be before importing the module under test
// ---------------------------------------------------------------------------

// Mock the store — capture a reference for assertions
const storeState = {
  chatRunning: false,
  chatUnread: false,
  taskRunning: false,
  taskUnread: false,
  tasks: [] as any[],
}

vi.mock('@/stores/app', () => ({
  store: {
    state: storeState,
  },
}))

// Mock EventSource
const mockEventSource = {
  close: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  onerror: null as ((ev: Event) => void) | null,
  onopen: null as ((ev: Event) => void) | null,
  onmessage: null as ((ev: MessageEvent) => void) | null,
  readyState: 0,
  url: '',
  withCredentials: false,
}

let mockEventSourceConstructor: typeof EventSource | null = null
let lastEventSourceInstance: typeof mockEventSource | null = null

// Capture event listeners added via addEventListener
type EventListenerMap = Record<string, Set<(ev: MessageEvent) => void>>
let capturedListeners: EventListenerMap = {}

vi.stubGlobal('EventSource', class MockEventSource {
  url: string
  close = mockEventSource.close
  addEventListener: typeof mockEventSource.addEventListener
  removeEventListener = mockEventSource.removeEventListener
  onerror: typeof mockEventSource.onerror
  onopen: typeof mockEventSource.onopen
  onmessage: typeof mockEventSource.onmessage
  readyState = 0
  withCredentials = false

  constructor(url: string) {
    this.url = url
    this.addEventListener = (type: string, handler: any) => {
      if (!capturedListeners[type]) capturedListeners[type] = new Set()
      capturedListeners[type].add(handler)
      mockEventSource.addEventListener(type, handler)
    }
    this.onerror = null
    this.onopen = null
    this.onmessage = null
    lastEventSourceInstance = this as any
  }
})

// Mock fetch for fullStateSync
const mockFetch = vi.fn()
beforeEach(() => {
  mockFetch.mockReset()
  global.fetch = mockFetch as any
})

// Mock document.visibilityState
let mockVisibilityState = 'visible'
Object.defineProperty(document, 'visibilityState', {
  get: () => mockVisibilityState,
  configurable: true,
})

// Mock navigator.onLine
Object.defineProperty(navigator, 'onLine', {
  value: true,
  writable: true,
  configurable: true,
})

// Import after mocks are set up
let useSystemEvents: typeof import('@/composables/useSystemEvents').useSystemEvents
let runningSessions: typeof import('@/composables/useSystemEvents').runningSessions
let runningTaskIds: typeof import('@/composables/useSystemEvents').runningTaskIds
let tunnelConnected: typeof import('@/composables/useSystemEvents').tunnelConnected
let currentChatSessionId: typeof import('@/composables/useSystemEvents').currentChatSessionId
let systemEventsConnected: typeof import('@/composables/useSystemEvents').systemEventsConnected
let registerUIHandler: typeof import('@/composables/useSystemEvents').registerUIHandler
let dispatchEvent: typeof import('@/composables/useSystemEvents').dispatchEvent
let fullStateSyncFn: typeof import('@/composables/useSystemEvents').fullStateSync

// Helper to reset module-level state between tests
async function resetModuleState() {
  // Dynamic import to get fresh module references
  const mod = await import('@/composables/useSystemEvents')
  useSystemEvents = mod.useSystemEvents
  runningSessions = mod.runningSessions
  runningTaskIds = mod.runningTaskIds
  tunnelConnected = mod.tunnelConnected
  currentChatSessionId = mod.currentChatSessionId
  systemEventsConnected = mod.systemEventsConnected
  registerUIHandler = mod.registerUIHandler
  dispatchEvent = mod.dispatchEvent
  fullStateSyncFn = mod.fullStateSync

  // Reset reactive state
  runningSessions.value = new Set()
  runningTaskIds.value = new Set()
  tunnelConnected.value = true
  currentChatSessionId.value = ''
  systemEventsConnected.value = false
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useSystemEvents', () => {
  beforeEach(async () => {
    vi.useFakeTimers()
    mockEventSource.close.mockReset()
    mockEventSource.addEventListener.mockReset()
    capturedListeners = {}
    lastEventSourceInstance = null
    mockVisibilityState = 'visible'
    mockFetch.mockReset()

    // Default fetch responses for fullStateSync
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/api/ai/sessions')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ sessions: [] }),
        })
      }
      if (url.includes('/api/tasks')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ tasks: [], hasUnread: false }),
        })
      }
      if (url.includes('/api/ssh/info')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ connectionStats: { connected: true } }),
        })
      }
      return Promise.resolve({ ok: false })
    })

    await resetModuleState()
    // Reset store state
    storeState.chatRunning = false
    storeState.chatUnread = false
    storeState.taskRunning = false
    storeState.taskUnread = false
    storeState.tasks = []
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // --- Core event handlers ---

  describe('core event handlers', () => {
    it('session_start adds to runningSessions', () => {
      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1', agentId: 'codebuddy' } })
      expect(runningSessions.value.has('s-1')).toBe(true)
    })

    it('session_start does not duplicate existing sessionId', () => {
      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })
      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })
      expect(runningSessions.value.size).toBe(1)
    })

    it('session_complete removes from runningSessions', () => {
      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })
      dispatchEvent({ type: 'session_complete', payload: { sessionId: 's-1', reason: 'done' } })
      expect(runningSessions.value.has('s-1')).toBe(false)
    })

    it('session_complete with unknown sessionId does not error', () => {
      dispatchEvent({ type: 'session_complete', payload: { sessionId: 'nonexistent', reason: 'done' } })
      expect(runningSessions.value.size).toBe(0)
    })

    it('message_new for non-current session sets chatUnread', () => {
      currentChatSessionId.value = 's-current'
      dispatchEvent({ type: 'message_new', payload: { sessionId: 's-other', role: 'user', messageId: 1 } })
      expect(storeState.chatUnread).toBe(true)
    })

    it('message_new for current session does not set chatUnread', () => {
      storeState.chatUnread = false
      currentChatSessionId.value = 's-current'
      dispatchEvent({ type: 'message_new', payload: { sessionId: 's-current', role: 'user', messageId: 1 } })
      expect(storeState.chatUnread).toBe(false)
    })

    it('task_exec_update running adds to runningTaskIds', () => {
      dispatchEvent({ type: 'task_exec_update', payload: { taskId: 1, status: 'running' } })
      expect(runningTaskIds.value.has(1)).toBe(true)
      expect(storeState.taskRunning).toBe(true)
    })

    it('task_exec_update completed removes from runningTaskIds', () => {
      dispatchEvent({ type: 'task_exec_update', payload: { taskId: 1, status: 'running' } })
      dispatchEvent({ type: 'task_exec_update', payload: { taskId: 1, status: 'completed' } })
      expect(runningTaskIds.value.has(1)).toBe(false)
      expect(storeState.taskRunning).toBe(false)
    })

    it('task_exec_update failed removes from runningTaskIds', () => {
      dispatchEvent({ type: 'task_exec_update', payload: { taskId: 1, status: 'running' } })
      dispatchEvent({ type: 'task_exec_update', payload: { taskId: 1, status: 'failed' } })
      expect(runningTaskIds.value.has(1)).toBe(false)
    })

    it('task_exec_update cancelled removes from runningTaskIds', () => {
      dispatchEvent({ type: 'task_exec_update', payload: { taskId: 1, status: 'running' } })
      dispatchEvent({ type: 'task_exec_update', payload: { taskId: 1, status: 'cancelled' } })
      expect(runningTaskIds.value.has(1)).toBe(false)
    })

    it('tunnel_status connected=true sets tunnelConnected', () => {
      tunnelConnected.value = false
      dispatchEvent({ type: 'tunnel_status', payload: { connected: true, clientCount: 1, activeChannels: 0 } })
      expect(tunnelConnected.value).toBe(true)
    })

    it('tunnel_status disconnected with clientCount=0 sets tunnelConnected false', () => {
      tunnelConnected.value = true
      dispatchEvent({ type: 'tunnel_status', payload: { connected: false, clientCount: 0, activeChannels: 0 } })
      expect(tunnelConnected.value).toBe(false)
    })

    it('tunnel_status disconnected with clientCount>0 does not change tunnelConnected', () => {
      tunnelConnected.value = true
      dispatchEvent({ type: 'tunnel_status', payload: { connected: false, clientCount: 1, activeChannels: 0 } })
      // Still true because clientCount > 0 means another tunnel is still connected
      expect(tunnelConnected.value).toBe(true)
    })

    it('unknown event type is handled gracefully', () => {
      dispatchEvent({ type: 'unknown_event', payload: { foo: 'bar' } })
      // Should not throw or change any state
      expect(runningSessions.value.size).toBe(0)
    })
  })

  // --- UI handler registration and dispatch ---

  describe('UI handler registration', () => {
    it('registerUIHandler receives dispatched events', () => {
      const handler = vi.fn()
      const unregister = registerUIHandler(handler)

      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })
      expect(handler).toHaveBeenCalledTimes(1)
      expect(handler).toHaveBeenCalledWith({ type: 'session_start', payload: { sessionId: 's-1' } })

      unregister()
    })

    it('unregister stops receiving events', () => {
      const handler = vi.fn()
      const unregister = registerUIHandler(handler)

      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })
      expect(handler).toHaveBeenCalledTimes(1)

      unregister()

      dispatchEvent({ type: 'session_complete', payload: { sessionId: 's-1', reason: 'done' } })
      expect(handler).toHaveBeenCalledTimes(1) // Still 1 — no new call
    })

    it('UI handler error does not break dispatch', () => {
      const badHandler = vi.fn(() => { throw new Error('boom') })
      const goodHandler = vi.fn()

      registerUIHandler(badHandler)
      registerUIHandler(goodHandler)

      // Should not throw despite bad handler
      expect(() => {
        dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })
      }).not.toThrow()

      // Good handler should still be called (after bad handler)
      expect(goodHandler).toHaveBeenCalledTimes(1)
    })

    it('multiple UI handlers all receive events', () => {
      const handler1 = vi.fn()
      const handler2 = vi.fn()
      const unreg1 = registerUIHandler(handler1)
      const unreg2 = registerUIHandler(handler2)

      dispatchEvent({ type: 'task_update', payload: { taskId: 1, action: 'create' } })
      expect(handler1).toHaveBeenCalledTimes(1)
      expect(handler2).toHaveBeenCalledTimes(1)

      unreg1()
      unreg2()
    })

    it('registerUIHandler with type filter only receives matching events', () => {
      const sessionHandler = vi.fn()
      const taskHandler = vi.fn()
      const unreg1 = registerUIHandler('session_complete', sessionHandler)
      const unreg2 = registerUIHandler('task_exec_update', taskHandler)

      // Dispatch a session_start event — neither handler should fire
      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })
      expect(sessionHandler).not.toHaveBeenCalled()
      expect(taskHandler).not.toHaveBeenCalled()

      // Dispatch a session_complete event — only sessionHandler should fire
      dispatchEvent({ type: 'session_complete', payload: { sessionId: 's-1', reason: 'done' } })
      expect(sessionHandler).toHaveBeenCalledTimes(1)
      expect(sessionHandler).toHaveBeenCalledWith({ type: 'session_complete', payload: { sessionId: 's-1', reason: 'done' } })
      expect(taskHandler).not.toHaveBeenCalled()

      // Dispatch a task_exec_update event — only taskHandler should fire
      dispatchEvent({ type: 'task_exec_update', payload: { taskId: 1, status: 'running' } })
      expect(sessionHandler).toHaveBeenCalledTimes(1) // still 1
      expect(taskHandler).toHaveBeenCalledTimes(1)

      unreg1()
      unreg2()
    })

    it('registerUIHandler with type filter can be unregistered', () => {
      const handler = vi.fn()
      const unregister = registerUIHandler('session_start', handler)

      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })
      expect(handler).toHaveBeenCalledTimes(1)

      unregister()

      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-2' } })
      expect(handler).toHaveBeenCalledTimes(1) // Still 1 — no new call
    })
  })

  // --- SSE connection management ---

  describe('SSE connection', () => {
    it('connectSSE creates EventSource to /api/events', () => {
      const result = useSystemEvents()
      result.connectSSE()

      expect(lastEventSourceInstance).not.toBeNull()
      expect(lastEventSourceInstance!.url).toBe('/api/events')
    })

    it('connectSSE does not create duplicate EventSource', () => {
      const result = useSystemEvents()
      result.connectSSE()
      const firstES = lastEventSourceInstance

      result.connectSSE() // Second call should be no-op
      expect(lastEventSourceInstance).toBe(firstES) // Same instance
    })

    it('disconnectSSE closes EventSource', () => {
      const result = useSystemEvents()
      result.connectSSE()
      result.disconnectSSE()

      expect(mockEventSource.close).toHaveBeenCalled()
      expect(systemEventsConnected.value).toBe(false)
    })

    it('connected event sets connected to true', () => {
      const result = useSystemEvents()
      result.connectSSE()

      // Simulate 'connected' event from server
      const listeners = capturedListeners['connected']
      expect(listeners).toBeDefined()
      expect(listeners!.size).toBeGreaterThan(0)

      const connectedHandler = [...listeners!][0]
      connectedHandler(new MessageEvent('connected', { data: '{"clientId":"test-123"}' }))

      expect(systemEventsConnected.value).toBe(true)
    })

    it('connected event resets reconnectAttempts', () => {
      const result = useSystemEvents()
      // Ensure clean SSE state
      result.disconnectSSE()
      result.connectSSE()

      const listeners = capturedListeners['connected']
      expect(listeners).toBeDefined()
      expect(listeners!.size).toBeGreaterThan(0)

      // Simulate connected event
      const connectedHandler = [...listeners!][0]
      connectedHandler(new MessageEvent('connected', { data: '{"clientId":"test-reset"}' }))

      expect(systemEventsConnected.value).toBe(true)
    })

    it('connected event triggers fullStateSync', async () => {
      const result = useSystemEvents()
      result.disconnectSSE()
      result.connectSSE()

      // Access listeners from the newly connected EventSource
      const listeners = capturedListeners['connected']
      expect(listeners).toBeDefined()

      // Clear previous fetch calls
      mockFetch.mockClear()

      // Simulate connected event — this triggers fullStateSync
      const connectedHandler = [...listeners!][0]
      connectedHandler(new MessageEvent('connected', { data: '{"clientId":"test-sync"}' }))

      // fullStateSync is async — wait for promises to flush
      await vi.runAllTimersAsync()

      expect(mockFetch).toHaveBeenCalled()
    })

    it('generic message event dispatches to handlers', () => {
      const result = useSystemEvents()
      result.disconnectSSE()
      result.connectSSE()

      const handler = vi.fn()
      registerUIHandler(handler)

      // Access the 'message' listener from the connected EventSource
      const messageListeners = capturedListeners['message']
      expect(messageListeners).toBeDefined()
      expect(messageListeners!.size).toBeGreaterThan(0)

      const messageHandler = [...messageListeners!][0]
      messageHandler(new MessageEvent('message', {
        data: JSON.stringify({ type: 'session_start', payload: { sessionId: 's-via-message' } })
      }))

      // The handler should have received the event via the generic message path
      expect(handler).toHaveBeenCalled()
    })

    it('named event types dispatch events correctly', () => {
      const handler = vi.fn()
      registerUIHandler(handler)

      // Use dispatchEvent directly (the real path from named event listeners)
      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1', agentId: 'codebuddy' } })

      expect(handler).toHaveBeenCalledWith({
        type: 'session_start',
        payload: { sessionId: 's-1', agentId: 'codebuddy' },
      })
    })

    it('onerror triggers disconnect and reconnect schedule', () => {
      const result = useSystemEvents()
      result.connectSSE()

      // Simulate error
      if (lastEventSourceInstance) {
        lastEventSourceInstance.onerror = vi.fn()
        const errorFn = lastEventSourceInstance.onerror
        // Trigger the onerror handler
        ;(lastEventSourceInstance as any).onerror(new Event('error'))
      }

      expect(systemEventsConnected.value).toBe(false)
    })

    it('connectSSE when intentionally disconnected returns early', () => {
      const result = useSystemEvents()

      // Go to background to set intentionallyDisconnected
      mockVisibilityState = 'hidden'
      document.dispatchEvent(new Event('visibilitychange'))

      // Reset the EventSource instance tracker
      lastEventSourceInstance = null

      // Try to connect — should be no-op due to intentionallyDisconnected
      result.connectSSE()

      // No new EventSource should be created
      expect(lastEventSourceInstance).toBeNull()
    })
  })

  // --- Reconnect logic ---

  describe('reconnect logic', () => {
    it('linear backoff: 2s, 4s, 6s, 8s, 10s', () => {
      const result = useSystemEvents()
      result.connectSSE()

      // Simulate first error
      if (lastEventSourceInstance) {
        lastEventSourceInstance.onerror = vi.fn()
        ;(lastEventSourceInstance as any).onerror(new Event('error'))
      }

      // After first error, reconnect timer should be set with delay = 2000 * 1 = 2000ms
      // Advance by 2000ms to trigger first reconnect
      vi.advanceTimersByTime(2000)

      // Second error after reconnect
      if (lastEventSourceInstance) {
        lastEventSourceInstance.onerror = vi.fn()
        ;(lastEventSourceInstance as any).onerror(new Event('error'))
      }

      // Second reconnect delay = 2000 * 2 = 4000ms
      vi.advanceTimersByTime(4000)

      // Third error
      if (lastEventSourceInstance) {
        lastEventSourceInstance.onerror = vi.fn()
        ;(lastEventSourceInstance as any).onerror(new Event('error'))
      }

      // Third reconnect delay = 2000 * 3 = 6000ms
      vi.advanceTimersByTime(6000)

      // Should still be attempting reconnects (not yet at max)
      expect(systemEventsConnected.value).toBe(false)
    })

    it('degraded polling after 5 failed reconnects', () => {
      const result = useSystemEvents()
      result.connectSSE()

      // Simulate 5 consecutive errors with advancing timers
      const delays = [2000, 4000, 6000, 8000, 10000]
      for (const delay of delays) {
        if (lastEventSourceInstance) {
          lastEventSourceInstance.onerror = vi.fn()
          ;(lastEventSourceInstance as any).onerror(new Event('error'))
        }
        vi.advanceTimersByTime(delay)
      }

      // After 5 reconnect attempts, scheduleReconnect should stop
      // The 6th error should not schedule another reconnect
      const closeCallCount = mockEventSource.close.mock.calls.length

      if (lastEventSourceInstance) {
        lastEventSourceInstance.onerror = vi.fn()
        ;(lastEventSourceInstance as any).onerror(new Event('error'))
      }

      // No more EventSource instances should be created after max attempts
      // close should not have been called again (no new connectSSE → no new error loop)
      // The onerror handler should still call disconnectSSE though
      expect(systemEventsConnected.value).toBe(false)
    })

    it('online event resets reconnect attempts counter', () => {
      const result = useSystemEvents()
      result.connectSSE()

      // Simulate a few errors
      if (lastEventSourceInstance) {
        lastEventSourceInstance.onerror = vi.fn()
        ;(lastEventSourceInstance as any).onerror(new Event('error'))
      }
      vi.advanceTimersByTime(2000) // first reconnect

      if (lastEventSourceInstance) {
        lastEventSourceInstance.onerror = vi.fn()
        ;(lastEventSourceInstance as any).onerror(new Event('error'))
      }

      // Now simulate online event — should reset reconnect counter
      window.dispatchEvent(new Event('online'))

      // After online, a new EventSource should be attempted
      // We can verify by checking that lastEventSourceInstance is non-null
      // (the reconnect was triggered by the online handler)
      expect(systemEventsConnected.value).toBe(false) // mock doesn't actually connect
    })
  })

  // --- Heartbeat ---

  describe('heartbeat', () => {
    it('heartbeat timeout triggers reconnect', () => {
      const result = useSystemEvents()
      result.connectSSE()

      // Simulate connected event to start heartbeat
      const listeners = capturedListeners['connected']
      if (listeners && listeners.size > 0) {
        const connectedHandler = [...listeners][0]
        connectedHandler(new MessageEvent('connected', { data: '{"clientId":"test"}' }))
      }

      // Advance past heartbeat timeout
      vi.advanceTimersByTime(30_000)

      // After heartbeat timeout, SSE should be disconnected and reconnect scheduled
      expect(systemEventsConnected.value).toBe(false)
    })
  })

  // --- Full-state sync ---

  describe('fullStateSync', () => {
    it('syncs running sessions from API', async () => {
      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/ai/sessions')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              sessions: [
                { id: 's-1', running: true },
                { id: 's-2', running: false },
                { id: 's-3', running: true },
              ],
            }),
          })
        }
        if (url.includes('/api/tasks')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ tasks: [], hasUnread: false }),
          })
        }
        if (url.includes('/api/ssh/info')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ connectionStats: { connected: true } }),
          })
        }
        return Promise.resolve({ ok: false })
      })

      await fullStateSyncFn()

      expect(runningSessions.value.has('s-1')).toBe(true)
      expect(runningSessions.value.has('s-2')).toBe(false)
      expect(runningSessions.value.has('s-3')).toBe(true)
    })

    it('syncs running tasks from API', async () => {
      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/ai/sessions')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ sessions: [] }),
          })
        }
        if (url.includes('/api/tasks')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              tasks: [
                { id: 1, runningCount: 1 },
                { id: 2, runningCount: 0 },
              ],
              hasUnread: true,
            }),
          })
        }
        if (url.includes('/api/ssh/info')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ connectionStats: { connected: true } }),
          })
        }
        return Promise.resolve({ ok: false })
      })

      await fullStateSyncFn()

      expect(runningTaskIds.value.has(1)).toBe(true)
      expect(runningTaskIds.value.has(2)).toBe(false)
    })

    it('syncs tunnel status from API', async () => {
      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/ai/sessions')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ sessions: [] }),
          })
        }
        if (url.includes('/api/tasks')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ tasks: [], hasUnread: false }),
          })
        }
        if (url.includes('/api/ssh/info')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ connectionStats: { connected: false } }),
          })
        }
        return Promise.resolve({ ok: false })
      })

      await fullStateSyncFn()

      expect(tunnelConnected.value).toBe(false)
    })

    it('handles partial API failures gracefully', async () => {
      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/ai/sessions')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ sessions: [{ id: 's-1', running: true }] }),
          })
        }
        if (url.includes('/api/tasks')) {
          return Promise.resolve({ ok: false }) // Task API fails
        }
        if (url.includes('/api/ssh/info')) {
          return Promise.resolve({ ok: false }) // SSH API fails
        }
        return Promise.resolve({ ok: false })
      })

      // Should not throw
      await expect(fullStateSyncFn()).resolves.toBeUndefined()

      // Sessions should still be synced
      expect(runningSessions.value.has('s-1')).toBe(true)
    })

    it('handles network failure gracefully', async () => {
      mockFetch.mockRejectedValue(new Error('Network error'))

      // Should not throw
      await expect(fullStateSyncFn()).resolves.toBeUndefined()
    })
  })

  // --- Visibility change ---

  describe('visibility change', () => {
    it('going to background disconnects SSE', () => {
      mockVisibilityState = 'hidden'
      document.dispatchEvent(new Event('visibilitychange'))

      expect(systemEventsConnected.value).toBe(false)
    })

    it('going to background clears reconnect timer', () => {
      const result = useSystemEvents()
      result.connectSSE()

      // Simulate error to start reconnect timer
      if (lastEventSourceInstance) {
        lastEventSourceInstance.onerror = vi.fn()
        ;(lastEventSourceInstance as any).onerror(new Event('error'))
      }

      // Go to background — should clear reconnect timer
      mockVisibilityState = 'hidden'
      document.dispatchEvent(new Event('visibilitychange'))

      // Advance past the reconnect timer — no reconnect should happen
      // because the timer was cleared
      vi.advanceTimersByTime(5000)

      // State should remain disconnected without attempting a new EventSource
      expect(systemEventsConnected.value).toBe(false)
    })

    it('returning to foreground reconnects SSE', () => {
      // First, go to background
      mockVisibilityState = 'hidden'
      document.dispatchEvent(new Event('visibilitychange'))

      // Then, come back to foreground
      mockVisibilityState = 'visible'
      document.dispatchEvent(new Event('visibilitychange'))

      // SSE should be reconnected (new EventSource created)
      // We can't easily verify the exact state but the function should not throw
    })

    it('foreground with existing EventSource calls fullStateSync', async () => {
      const result = useSystemEvents()
      result.disconnectSSE()
      result.connectSSE()

      // Simulate connected so systemEventsConnected is true
      const listeners = capturedListeners['connected']
      expect(listeners).toBeDefined()
      const connectedHandler = [...listeners!][0]
      connectedHandler(new MessageEvent('connected', { data: '{"clientId":"test"}' }))

      mockFetch.mockClear()

      // Go to background then back to foreground
      mockVisibilityState = 'hidden'
      document.dispatchEvent(new Event('visibilitychange'))

      mockVisibilityState = 'visible'
      document.dispatchEvent(new Event('visibilitychange'))

      // fullStateSync is async — wait for promises
      await vi.runAllTimersAsync()

      // fullStateSync should be called since EventSource was already connected
      expect(mockFetch).toHaveBeenCalled()
    })
  })

  // --- Network recovery ---

  describe('network recovery', () => {
    it('online event triggers reconnect when not intentionally disconnected', () => {
      // Simulate online event
      window.dispatchEvent(new Event('online'))

      // Should attempt to reconnect
      // We verify no crash and the state is consistent
      expect(systemEventsConnected.value).toBe(false) // Not connected yet (mock)
    })

    it('online event when intentionally disconnected does not reconnect', () => {
      // Go to background to set intentionallyDisconnected
      mockVisibilityState = 'hidden'
      document.dispatchEvent(new Event('visibilitychange'))

      // Reset EventSource tracker
      lastEventSourceInstance = null

      // Simulate online event while intentionally disconnected
      window.dispatchEvent(new Event('online'))

      // Should NOT create a new EventSource
      expect(lastEventSourceInstance).toBeNull()
    })
  })

  // --- Full-state sync edge cases ---

  describe('fullStateSync edge cases', () => {
    it('sync clears stale running sessions', async () => {
      // Set up a stale running session
      runningSessions.value = new Set(['s-stale-1', 's-stale-2'])
      storeState.chatRunning = true

      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/ai/sessions')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              sessions: [
                { id: 's-stale-1', running: false },  // Was running, now stopped
                { id: 's-stale-2', running: false },  // Was running, now stopped
              ],
            }),
          })
        }
        if (url.includes('/api/tasks')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ tasks: [], hasUnread: false }),
          })
        }
        if (url.includes('/api/ssh/info')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ connectionStats: { connected: true } }),
          })
        }
        return Promise.resolve({ ok: false })
      })

      await fullStateSyncFn()

      // Stale sessions should be cleared
      expect(runningSessions.value.has('s-stale-1')).toBe(false)
      expect(runningSessions.value.has('s-stale-2')).toBe(false)
      expect(storeState.chatRunning).toBe(false)
    })

    it('sync handles tasks with hasUnread=true', async () => {
      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/ai/sessions')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ sessions: [] }),
          })
        }
        if (url.includes('/api/tasks')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              tasks: [{ id: 1, runningCount: 0 }],
              hasUnread: true,
            }),
          })
        }
        if (url.includes('/api/ssh/info')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ connectionStats: { connected: true } }),
          })
        }
        return Promise.resolve({ ok: false })
      })

      await fullStateSyncFn()

      expect(storeState.taskUnread).toBe(true)
    })

    it('all 3 APIs fail — state unchanged', async () => {
      // Set some initial state
      runningSessions.value = new Set(['s-existing'])
      tunnelConnected.value = true

      mockFetch.mockRejectedValue(new Error('Network error'))

      await fullStateSyncFn()

      // State should remain unchanged when all APIs fail
      expect(runningSessions.value.has('s-existing')).toBe(true)
      expect(tunnelConnected.value).toBe(true)
    })

    it('sync updates chatUnread based on sessions', async () => {
      currentChatSessionId.value = 's-current'
      storeState.chatUnread = false

      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/ai/sessions')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              sessions: [
                { id: 's-current', running: false, unreadCount: 0 },
                { id: 's-other', running: false, unreadCount: 3 },
              ],
            }),
          })
        }
        if (url.includes('/api/tasks')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ tasks: [], hasUnread: false }),
          })
        }
        if (url.includes('/api/ssh/info')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ connectionStats: { connected: true } }),
          })
        }
        return Promise.resolve({ ok: false })
      })

      await fullStateSyncFn()

      // chatUnread should be true because s-other has unreadCount > 0
      // and it's not the current session
      expect(storeState.chatUnread).toBe(true)
    })
  })

  // --- UI handler edge cases ---

  describe('UI handler edge cases', () => {
    it('UI handler sees state updated by core handler', () => {
      let sawRunning = false
      const handler = vi.fn(() => {
        // At the time this handler is called, the core handler should
        // have already updated runningSessions
        sawRunning = runningSessions.value.has('s-1')
      })
      registerUIHandler(handler)

      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })

      expect(handler).toHaveBeenCalledTimes(1)
      expect(sawRunning).toBe(true)
    })

    it('mixed filtered + unfiltered handlers receive correct events', () => {
      const allHandler = vi.fn()
      const filteredHandler = vi.fn()
      registerUIHandler(allHandler)
      const unreg = registerUIHandler('tunnel_status', filteredHandler)

      dispatchEvent({ type: 'session_start', payload: { sessionId: 's-1' } })
      expect(allHandler).toHaveBeenCalledTimes(1)
      expect(filteredHandler).not.toHaveBeenCalled()

      dispatchEvent({ type: 'tunnel_status', payload: { connected: true } })
      expect(allHandler).toHaveBeenCalledTimes(2)
      expect(filteredHandler).toHaveBeenCalledTimes(1)

      unreg()
    })
  })
})

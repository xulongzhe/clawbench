import { ref, onUnmounted } from 'vue'
import { store } from '@/stores/app'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface SystemEvent {
  type: string
  payload: Record<string, any>
}

type UIEventHandler = (event: SystemEvent) => void

// ---------------------------------------------------------------------------
// Module-level state (singleton — shared across all consumers)
// ---------------------------------------------------------------------------

/** Active EventSource connection (null when disconnected) */
let eventSource: EventSource | null = null

/** Subscriber counter — connect when first subscriber, disconnect when last leaves */
let subscriberCount = 0

/** Whether the SSE is intentionally disconnected (e.g. app backgrounded) */
let intentionallyDisconnected = false

/** Reconnect attempts counter */
let reconnectAttempts = 0
const MAX_RECONNECT_ATTEMPTS = 5
const BASE_RECONNECT_DELAY = 2000

/** Reconnect timer handle */
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

/** Heartbeat monitor — if no event received within 30s, reconnect */
let heartbeatTimer: ReturnType<typeof setTimeout> | null = null
const HEARTBEAT_TIMEOUT_MS = 30_000

// ---------------------------------------------------------------------------
// Module-level event handlers (registered once, not per-component)
// These handle core state updates. Component-level handlers are registered
// separately via onEvent() for UI-specific actions (notifications, toasts, etc.)
// ---------------------------------------------------------------------------

/** Core handlers: update store.state and runningSessions based on events */
const coreHandlers: Record<string, (payload: Record<string, any>) => void> = {
  session_start(payload) {
    // Add to running sessions set
    const sid = payload.sessionId as string
    if (sid && !runningSessions.value.has(sid)) {
      runningSessions.value = new Set([...runningSessions.value, sid])
    }
    store.state.chatRunning = runningSessions.value.size > 0
  },

  session_complete(payload) {
    const sid = payload.sessionId as string
    if (sid) {
      const next = new Set(runningSessions.value)
      next.delete(sid)
      runningSessions.value = next
    }
    store.state.chatRunning = runningSessions.value.size > 0
  },

  message_new(payload) {
    // Increment unread indicator for non-current sessions
    const sid = payload.sessionId as string
    if (sid && sid !== currentChatSessionId.value) {
      store.state.chatUnread = true
    }
  },

  task_update() {
    // Task list changed — mark for refresh on next poll/visit
    // (Full state sync will pick up the details)
  },

  task_exec_update(payload) {
    const status = payload.status as string
    const taskId = payload.taskId as number
    if (status === 'running') {
      // Track running task
      runningTaskIds.value = new Set([...runningTaskIds.value, taskId])
      store.state.taskRunning = true
    } else if (['completed', 'failed', 'cancelled'].includes(status)) {
      const next = new Set(runningTaskIds.value)
      next.delete(taskId)
      runningTaskIds.value = next
      store.state.taskRunning = runningTaskIds.value.size > 0
    }
  },

  tunnel_status(payload) {
    const connected = payload.connected as boolean
    const clientCount = payload.clientCount as number
    // Update tunnel status in store/portforward if needed
    // This is a lightweight notification — detailed health check
    // happens via usePortForward's existing tunnel polling
    if (!connected && clientCount === 0) {
      tunnelConnected.value = false
    } else if (connected) {
      tunnelConnected.value = true
    }
  },
}

// ---------------------------------------------------------------------------
// Module-level reactive state (shared across all consumers)
// ---------------------------------------------------------------------------

/** Set of currently running chat session IDs */
const runningSessions = ref<Set<string>>(new Set())

/** Set of currently running task IDs */
const runningTaskIds = ref<Set<number>>(new Set())

/** Whether SSH tunnel is connected (per last tunnel_status event) */
const tunnelConnected = ref(true)

/** Current chat session ID — for message_new unread detection */
const currentChatSessionId = ref<string>('')

/** Whether the system events SSE is currently connected */
const connected = ref(false)

// ---------------------------------------------------------------------------
// UI-level event handlers (component-specific — notifications, toasts)
// ---------------------------------------------------------------------------

interface UIHandlerRegistration {
  /** Optional event type filter — if set, handler only receives events of this type */
  type?: string
  handler: UIEventHandler
}

const uiHandlers: UIHandlerRegistration[] = []

/**
 * Register a UI-level event handler.
 * Supports two calling conventions:
 *   registerUIHandler(handler)           — receives ALL event types
 *   registerUIHandler('type', handler)   — receives only events of that type
 */
function registerUIHandler(typeOrHandler: string | UIEventHandler, handler?: UIEventHandler) {
  const reg: UIHandlerRegistration = typeof typeOrHandler === 'string'
    ? { type: typeOrHandler, handler: handler! }
    : { handler: typeOrHandler }
  uiHandlers.push(reg)
  return () => {
    const idx = uiHandlers.indexOf(reg)
    if (idx !== -1) uiHandlers.splice(idx, 1)
  }
}

// ---------------------------------------------------------------------------
// Dispatch — route event to core handlers + UI handlers
// ---------------------------------------------------------------------------

function dispatchEvent(event: SystemEvent) {
  // 1. Core state handler (always runs)
  const handler = coreHandlers[event.type]
  if (handler) {
    handler(event.payload ?? {})
  }

  // 2. UI handlers (component-level — notifications, toasts, etc.)
  for (const reg of uiHandlers) {
    // Skip if handler has a type filter that doesn't match
    if (reg.type && reg.type !== event.type) continue
    try {
      reg.handler(event)
    } catch (err) {
      console.error('[useSystemEvents] UI handler error:', err)
    }
  }
}

// ---------------------------------------------------------------------------
// SSE connection management
// ---------------------------------------------------------------------------

function resetHeartbeat() {
  if (heartbeatTimer) clearTimeout(heartbeatTimer)
  heartbeatTimer = setTimeout(() => {
    console.warn('[useSystemEvents] Heartbeat timeout — reconnecting')
    disconnectSSE()
    scheduleReconnect()
  }, HEARTBEAT_TIMEOUT_MS)
}

function connectSSE() {
  // Don't reconnect if intentionally disconnected or already connected
  if (intentionallyDisconnected) return
  if (eventSource) return

  try {
    eventSource = new EventSource('/api/events')
  } catch (err) {
    console.error('[useSystemEvents] Failed to create EventSource:', err)
    scheduleReconnect()
    return
  }

  eventSource.addEventListener('connected', (e: MessageEvent) => {
    reconnectAttempts = 0
    connected.value = true
    resetHeartbeat()

    // Parse connected event data for clientId
    try {
      const data = JSON.parse(e.data)
      // data may be: { clientId: "..." } or just a string clientId
      const clientId = typeof data === 'object' ? data.clientId : data
      console.log('[useSystemEvents] Connected, clientId:', clientId)
    } catch {
      console.log('[useSystemEvents] Connected')
    }

    // After (re)connecting, perform full-state sync
    fullStateSync()
  })

  eventSource.addEventListener('message', (e: MessageEvent) => {
    resetHeartbeat()
    try {
      const event: SystemEvent = JSON.parse(e.data)
      dispatchEvent(event)
    } catch (err) {
      console.error('[useSystemEvents] Failed to parse event:', err)
    }
  })

  // Named event types (server sends `event: xxx\ndata: {...}`)
  // We also handle generic messages above, but named events are
  // the primary path for system events.
  const eventTypes = [
    'session_start', 'session_complete', 'message_new',
    'task_update', 'task_exec_update', 'tunnel_status',
  ]
  for (const type of eventTypes) {
    eventSource.addEventListener(type, (e: MessageEvent) => {
      resetHeartbeat()
      try {
        const payload = JSON.parse(e.data)
        dispatchEvent({ type, payload })
      } catch (err) {
        console.error(`[useSystemEvents] Failed to parse ${type} event:`, err)
      }
    })
  }

  eventSource.onerror = () => {
    console.warn('[useSystemEvents] SSE error')
    disconnectSSE()

    if (!intentionallyDisconnected) {
      scheduleReconnect()
    }
  }
}

function disconnectSSE() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  connected.value = false
  if (heartbeatTimer) {
    clearTimeout(heartbeatTimer)
    heartbeatTimer = null
  }
}

function scheduleReconnect() {
  if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
    console.warn('[useSystemEvents] Max reconnect attempts reached — falling back to degraded polling')
    // Signal that SSE is unavailable — composables should keep their existing polling
    return
  }

  const delay = BASE_RECONNECT_DELAY * (reconnectAttempts + 1)
  reconnectAttempts++

  if (reconnectTimer) clearTimeout(reconnectTimer)
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    console.log(`[useSystemEvents] Reconnect attempt ${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS}`)
    connectSSE()
  }, delay)
}

// ---------------------------------------------------------------------------
// Full-state sync — called after (re)connect to catch up on missed events
// ---------------------------------------------------------------------------

async function fullStateSync() {
  try {
    // 1. Sync running sessions
    const sessionsRes = await fetch('/api/ai/sessions')
    if (sessionsRes.ok) {
      const data = await sessionsRes.json()
      const sessions = data.sessions ?? []
      const newRunning = new Set<string>()
      for (const s of sessions) {
        if (s.running) newRunning.add(s.id)
      }
      runningSessions.value = newRunning
      store.state.chatRunning = newRunning.size > 0

      // Check for completed sessions (were running before, now not)
      // This is critical for detecting completions while disconnected
      store.state.chatUnread = sessions.some(
        (s: any) => s.unreadCount > 0 && s.id !== currentChatSessionId.value
      )
    }

    // 2. Sync task state
    const tasksRes = await fetch('/api/tasks')
    if (tasksRes.ok) {
      const data = await tasksRes.json()
      const tasks = data.tasks ?? []
      const newRunningTaskIds = new Set<number>()
      for (const t of tasks) {
        if (t.runningCount > 0) {
          newRunningTaskIds.add(t.id)
        }
      }
      runningTaskIds.value = newRunningTaskIds
      store.state.taskRunning = newRunningTaskIds.size > 0
      store.state.taskUnread = !!data.hasUnread
      store.state.tasks = tasks
    }

    // 3. Sync tunnel status
    const sshRes = await fetch('/api/ssh/info')
    if (sshRes.ok) {
      const sshData = await sshRes.json()
      const stats = sshData.connectionStats
      if (stats) {
        tunnelConnected.value = stats.connected
      }
    }
  } catch (err) {
    console.error('[useSystemEvents] Full-state sync failed:', err)
  }
}

// ---------------------------------------------------------------------------
// Visibility change handling
// ---------------------------------------------------------------------------

let visibilityHandler: (() => void) | null = null

function setupVisibilityHandler() {
  if (visibilityHandler) return // already set up

  visibilityHandler = () => {
    if (document.visibilityState === 'visible') {
      // App came to foreground — reconnect SSE + full-state sync
      intentionallyDisconnected = false
      reconnectAttempts = 0
      if (!eventSource) {
        connectSSE()
      } else {
        fullStateSync()
      }
    } else {
      // App went to background — disconnect SSE to save battery
      // (Native TunnelEventService will receive notifications)
      intentionallyDisconnected = true
      disconnectSSE()
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
    }
  }

  document.addEventListener('visibilitychange', visibilityHandler)
}

function teardownVisibilityHandler() {
  if (visibilityHandler) {
    document.removeEventListener('visibilitychange', visibilityHandler)
    visibilityHandler = null
  }
}

// ---------------------------------------------------------------------------
// Network recovery
// ---------------------------------------------------------------------------

let onlineHandler: (() => void) | null = null

function setupOnlineHandler() {
  if (onlineHandler) return

  onlineHandler = () => {
    console.log('[useSystemEvents] Network online — reconnecting')
    reconnectAttempts = 0
    if (!eventSource && !intentionallyDisconnected) {
      connectSSE()
    }
  }

  window.addEventListener('online', onlineHandler)
}

function teardownOnlineHandler() {
  if (onlineHandler) {
    window.removeEventListener('online', onlineHandler)
    onlineHandler = null
  }
}

// ---------------------------------------------------------------------------
// Composable
// ---------------------------------------------------------------------------

export function useSystemEvents() {
  // Increment subscriber count on mount
  subscriberCount++

  // Set up global handlers on first subscriber
  if (subscriberCount === 1) {
    setupVisibilityHandler()
    setupOnlineHandler()
    // Auto-connect SSE if not intentionally disconnected
    if (!intentionallyDisconnected && !eventSource) {
      connectSSE()
    }
  }

  // Cleanup on unmount
  onUnmounted(() => {
    subscriberCount--
    if (subscriberCount <= 0) {
      subscriberCount = 0
      disconnectSSE()
      teardownVisibilityHandler()
      teardownOnlineHandler()
    }
  })

  return {
    // State
    connected,
    runningSessions,
    runningTaskIds,
    tunnelConnected,
    currentChatSessionId,

    // Actions
    connectSSE,
    disconnectSSE,
    fullStateSync,

    // Event registration (component-level)
    onEvent: registerUIHandler,
  }
}

// ---------------------------------------------------------------------------
// Exports for direct access (used by other composables without instantiation)
// ---------------------------------------------------------------------------

export {
  runningSessions,
  runningTaskIds,
  tunnelConnected,
  currentChatSessionId,
  connected as systemEventsConnected,
  registerUIHandler,
  dispatchEvent,
  fullStateSync,
}

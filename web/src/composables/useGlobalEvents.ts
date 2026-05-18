import { ref, computed } from 'vue'
import { useReconnect } from './useReconnect'
import { useAppMode } from './useAppMode'

// Event types from server
interface ServerEvent {
    type: string           // "event" | "ping"
    id?: string            // event ID for dedup
    event?: string         // "session_update" | "task_update" | "queue_update"
    data?: {
        session_id?: string
        status?: string
        has_new_messages?: boolean
        task_id?: string
        execution_id?: string
        count?: number
    }
}

// Client message types
type ClientMessage =
    | { type: 'ack'; id: string }
    | { type: 'pong' }

type EventHandler = (event: string, data: ServerEvent['data']) => void

// Module-level singleton state
const connected = ref(false)
const pushAvailable = ref(false)
const pushRegistered = ref(false)
const handlers: EventHandler[] = []
const processedEventIds = new Set<string>()
const MAX_PROCESSED_IDS = 100
let ws: WebSocket | null = null
let heartbeatTimer: ReturnType<typeof setInterval> | null = null
const MISSED_PONG_THRESHOLD = 2
let missedPongs = 0

const { isAppMode } = useAppMode()

const reconnect = useReconnect({
    maxAttempts: 3,
    baseDelay: 2000,
    onReconnect: () => connect(),
})

function addProcessedId(id: string) {
    processedEventIds.add(id)
    // Evict oldest entries when set exceeds limit
    if (processedEventIds.size > MAX_PROCESSED_IDS) {
        const toRemove = processedEventIds.size - MAX_PROCESSED_IDS
        const iter = processedEventIds.values()
        for (let i = 0; i < toRemove; i++) {
            const val = iter.next().value
            if (val !== undefined) processedEventIds.delete(val)
        }
    }
}

function isDuplicate(id: string): boolean {
    return processedEventIds.has(id)
}

/**
 * Check whether push notifications are available.
 * In app mode: query AndroidNative.isPushAvailable() (set by MainActivity after fetchPushConfig).
 * In web mode: fetch from /api/push/config endpoint.
 * Also sets pushRegistered if JPush is available AND has a registration ID.
 */
async function checkPushAvailable() {
    if (isAppMode.value) {
        // Android native bridge — already fetched by MainActivity.fetchPushConfig()
        const native = (window as any).AndroidNative
        pushAvailable.value = !!native?.isPushAvailable?.()
        // JPush registered = available + has registration ID from SDK
        const regId = native?.getPushRegistrationId?.()
        pushRegistered.value = pushAvailable.value && !!regId
    } else {
        // Web mode — check server config
        try {
            const resp = await fetch('/api/push/config')
            if (resp.ok) {
                const data = await resp.json()
                pushAvailable.value = !!data.jpush_enabled && !!data.jpush_app_key
            }
        } catch {
            pushAvailable.value = false
        }
        // Web mode doesn't run JPush SDK, pushRegistered is always false
        pushRegistered.value = false
    }
}

/**
 * Register JPush Registration ID with the server via HTTP.
 * This is a login-level lifecycle event — the server persists the reg ID
 * so it can send push notifications even when the WebSocket is disconnected.
 */
function registerPushId() {
    if (!isAppMode.value) return
    const regId = (window as any).AndroidNative?.getPushRegistrationId?.()
    if (!regId) return
    fetch('/api/push/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ registration_id: regId }),
    }).catch(() => {
        // Non-critical — push will not work until next successful registration
    })
}

function connect() {
    disconnect()

    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${location.host}/api/ai/events/ws`

    ws = new WebSocket(url)

    ws.onopen = () => {
        connected.value = true
        missedPongs = 0
        reconnect.reset()

        // Check push availability (non-blocking)
        checkPushAvailable()

        // Start heartbeat monitoring
        startHeartbeat()
    }

    ws.onmessage = (event) => {
        try {
            const msg: ServerEvent = JSON.parse(event.data)

            if (msg.type === 'ping') {
                send({ type: 'pong' })
                missedPongs = 0
                return
            }

            if (msg.type === 'event' && msg.event) {
                // Dedup check
                if (msg.id && isDuplicate(msg.id)) {
                    return
                }
                if (msg.id) {
                    addProcessedId(msg.id)
                }

                // Dispatch to handlers
                for (const handler of handlers) {
                    handler(msg.event!, msg.data)
                }

                // Send ack
                if (msg.id) {
                    send({ type: 'ack', id: msg.id })
                }
            }
        } catch {
            // Ignore malformed messages
        }
    }

    ws.onclose = () => {
        connected.value = false
        stopHeartbeat()

        if (reconnect.shouldReconnect()) {
            reconnect.scheduleReconnect()
        }
    }

    ws.onerror = () => {
        // onclose will fire after this
    }
}

function disconnect() {
    stopHeartbeat()
    if (ws) {
        ws.onclose = null // prevent reconnect
        ws.close()
        ws = null
    }
    connected.value = false
}

function send(msg: ClientMessage) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(msg))
    }
}

function startHeartbeat() {
    stopHeartbeat()
    missedPongs = 0
    heartbeatTimer = setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
            missedPongs++
            if (missedPongs > MISSED_PONG_THRESHOLD) {
                // Connection seems dead, reconnect
                disconnect()
                if (reconnect.shouldReconnect()) {
                    reconnect.scheduleReconnect()
                }
            }
        }
    }, 35000) // Check every 35s (server pings every 30s)
}

function stopHeartbeat() {
    if (heartbeatTimer) {
        clearInterval(heartbeatTimer)
        heartbeatTimer = null
    }
}

export function useGlobalEvents() {
    // WebSocket connection status: 'connected' | 'reconnecting' | 'disconnected'
    const wsStatus = computed(() => {
        if (connected.value) return 'connected'
        if (reconnect.reconnecting.value) return 'reconnecting'
        return 'disconnected'
    })

    function onEvent(handler: EventHandler) {
        handlers.push(handler)
        return () => {
            const idx = handlers.indexOf(handler)
            if (idx !== -1) handlers.splice(idx, 1)
        }
    }

    // Visibility change: when push notifications are available, disconnect WebSocket
    // on background (push will deliver events). When push is NOT available, keep
    // WebSocket alive in background so real-time events continue to be received.
    function handleVisibilityChange() {
        if (document.visibilityState === 'visible') {
            // Returning to foreground — reconnect and do full state pull
            connect()
            // Emit a custom event that other composables can listen to
            window.dispatchEvent(new CustomEvent('clawbench-foreground'))
        } else {
            // Going to background
            if (pushAvailable.value) {
                // Push notifications available — safe to disconnect WebSocket.
                // Events will be delivered via JPush and replayed on reconnect.
                disconnect()
                reconnect.disable() // prevent auto-reconnect while backgrounded
                // Re-enable reconnect for next foreground
                setTimeout(() => reconnect.reset(), 100)
            }
            // If push is NOT available, keep WebSocket alive in background.
            // The heartbeat monitor will detect dead connections and reconnect.
        }
    }

    function init() {
        document.addEventListener('visibilitychange', handleVisibilityChange)
        // Initial connect
        connect()
        // Register push ID via HTTP (login-level, not per-WS-connection)
        registerPushId()
    }

    function destroy() {
        document.removeEventListener('visibilitychange', handleVisibilityChange)
        disconnect()
    }

    return {
        connected,
        wsStatus,
        pushAvailable,
        pushRegistered,
        connect,
        disconnect,
        onEvent,
        init,
        destroy,
    }
}

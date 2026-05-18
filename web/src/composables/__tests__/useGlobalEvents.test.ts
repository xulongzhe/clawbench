import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { useGlobalEvents } from '@/composables/useGlobalEvents'

// Mock WebSocket that captures constructor calls
let mockWsInstances: MockWebSocket[] = []

class MockWebSocket {
    static CONNECTING = 0
    static OPEN = 1
    static CLOSING = 2
    static CLOSED = 3

    url: string
    readyState: number = MockWebSocket.CONNECTING
    onopen: ((ev: Event) => void) | null = null
    onmessage: ((ev: MessageEvent) => void) | null = null
    onclose: ((ev: CloseEvent) => void) | null = null
    onerror: ((ev: Event) => void) | null = null
    sentMessages: string[] = []

    constructor(url: string) {
        this.url = url
        mockWsInstances.push(this)
    }

    send(data: string) {
        this.sentMessages.push(data)
    }

    close() {
        this.readyState = MockWebSocket.CLOSED
        this.onclose?.(new CloseEvent('close'))
    }

    // Simulate receiving a message from server
    receive(data: object) {
        this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }))
    }

    // Simulate connection open
    simulateOpen() {
        this.readyState = MockWebSocket.OPEN
        this.onopen?.(new Event('open'))
    }
}

function getLatestWs(): MockWebSocket {
    return mockWsInstances[mockWsInstances.length - 1]
}

// Unique ID counter to avoid cross-test dedup conflicts
let idCounter = 0
function nextId(): string {
    return `evt_test_${++idCounter}`
}

describe('useGlobalEvents', () => {
    let originalWebSocket: typeof WebSocket
    let originalFetch: typeof globalThis.fetch
    let events: ReturnType<typeof useGlobalEvents>

    beforeEach(() => {
        mockWsInstances = []
        originalWebSocket = globalThis.WebSocket
        originalFetch = globalThis.fetch
        globalThis.WebSocket = MockWebSocket as any
        // Mock fetch for push config endpoint
        globalThis.fetch = vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ jpush_enabled: false, jpush_app_key: '' }),
        })
        events = useGlobalEvents()
    })

    afterEach(() => {
        events.destroy()
        globalThis.WebSocket = originalWebSocket
        globalThis.fetch = originalFetch
    })

    function connectAndGetWs(): MockWebSocket {
        events.connect()
        const ws = getLatestWs()
        ws.simulateOpen()
        return ws
    }

    describe('event dedup', () => {
        it('should not dispatch duplicate events with same ID', () => {
            const handler = vi.fn()
            events.onEvent(handler)
            const ws = connectAndGetWs()

            const id = nextId()
            const eventData = { session_id: 's1', status: 'completed' }
            ws.receive({ type: 'event', id, event: 'session_update', data: eventData })
            ws.receive({ type: 'event', id, event: 'session_update', data: eventData })

            expect(handler).toHaveBeenCalledTimes(1)
        })

        it('should dispatch events with different IDs', () => {
            const handler = vi.fn()
            events.onEvent(handler)
            const ws = connectAndGetWs()

            ws.receive({ type: 'event', id: nextId(), event: 'session_update', data: {} })
            ws.receive({ type: 'event', id: nextId(), event: 'task_update', data: {} })

            expect(handler).toHaveBeenCalledTimes(2)
        })

        it('should dispatch events without ID (no dedup)', () => {
            const handler = vi.fn()
            events.onEvent(handler)
            const ws = connectAndGetWs()

            ws.receive({ type: 'event', event: 'session_update', data: {} })
            ws.receive({ type: 'event', event: 'session_update', data: {} })

            expect(handler).toHaveBeenCalledTimes(2)
        })
    })

    describe('ping/pong', () => {
        it('should send pong when receiving ping', () => {
            const ws = connectAndGetWs()

            ws.receive({ type: 'ping' })

            expect(ws.sentMessages).toContainEqual(JSON.stringify({ type: 'pong' }))
        })
    })

    describe('ack', () => {
        it('should send ack for events with ID', () => {
            const ws = connectAndGetWs()
            const id = nextId()

            ws.receive({ type: 'event', id, event: 'session_update', data: {} })

            expect(ws.sentMessages).toContainEqual(JSON.stringify({ type: 'ack', id }))
        })

        it('should not send ack for events without ID', () => {
            const ws = connectAndGetWs()
            ws.sentMessages = []

            ws.receive({ type: 'event', event: 'session_update', data: {} })

            const ackMessages = ws.sentMessages.filter(m => {
                try { return JSON.parse(m).type === 'ack' } catch { return false }
            })
            expect(ackMessages).toHaveLength(0)
        })
    })

    describe('onEvent handler management', () => {
        it('should unsubscribe handler when returned function is called', () => {
            const handler = vi.fn()
            const unsub = events.onEvent(handler)
            const ws = connectAndGetWs()

            ws.receive({ type: 'event', id: nextId(), event: 'session_update', data: {} })
            expect(handler).toHaveBeenCalledTimes(1)

            unsub()
            ws.receive({ type: 'event', id: nextId(), event: 'session_update', data: {} })
            expect(handler).toHaveBeenCalledTimes(1) // not called again
        })

        it('should dispatch to multiple handlers', () => {
            const handler1 = vi.fn()
            const handler2 = vi.fn()
            events.onEvent(handler1)
            events.onEvent(handler2)
            const ws = connectAndGetWs()

            ws.receive({ type: 'event', id: nextId(), event: 'session_update', data: {} })

            expect(handler1).toHaveBeenCalledTimes(1)
            expect(handler2).toHaveBeenCalledTimes(1)
        })
    })

    describe('registerPushId', () => {
        it('should skip registration in web mode (no AndroidNative)', () => {
            // In web mode (no AndroidNative), registerPushId is a no-op.
            // init() calls registerPushId() which early-returns if !isAppMode.
            // Verify no push/register fetch call is made.
            events.init()
            const registerCalls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls.filter(
                (call: any[]) => call[0] === '/api/push/register'
            )
            expect(registerCalls).toHaveLength(0)
        })
    })

    describe('event handler receives correct data', () => {
        it('should pass event name and data to handler', () => {
            const handler = vi.fn()
            events.onEvent(handler)
            const ws = connectAndGetWs()

            const data = { session_id: 's1', status: 'completed', has_new_messages: true }
            ws.receive({ type: 'event', id: nextId(), event: 'session_update', data })

            expect(handler).toHaveBeenCalledWith('session_update', data)
        })

        it('should handle task_update events', () => {
            const handler = vi.fn()
            events.onEvent(handler)
            const ws = connectAndGetWs()

            const data = { task_id: 't1', status: 'completed' }
            ws.receive({ type: 'event', id: nextId(), event: 'task_update', data })

            expect(handler).toHaveBeenCalledWith('task_update', data)
        })
    })

    describe('malformed messages', () => {
        it('should ignore non-JSON messages', () => {
            const handler = vi.fn()
            events.onEvent(handler)
            const ws = connectAndGetWs()

            ws.onmessage?.(new MessageEvent('message', { data: 'not json' }))

            expect(handler).not.toHaveBeenCalled()
        })

        it('should ignore messages with unknown type', () => {
            const handler = vi.fn()
            events.onEvent(handler)
            const ws = connectAndGetWs()

            ws.receive({ type: 'unknown_type' })

            expect(handler).not.toHaveBeenCalled()
        })
    })

    describe('connect/disconnect', () => {
        it('should create WebSocket on connect', () => {
            const beforeCount = mockWsInstances.length
            events.connect()
            expect(mockWsInstances.length).toBeGreaterThan(beforeCount)
        })

        it('should close WebSocket on disconnect', () => {
            const ws = connectAndGetWs()
            events.disconnect()
            expect(ws.readyState).toBe(MockWebSocket.CLOSED)
        })
    })

    describe('visibility change with pushAvailable', () => {
        it('should keep WebSocket alive when push is NOT available', () => {
            // init() registers the visibility change handler
            events.init()
            const ws = connectAndGetWs()
            events.pushAvailable.value = false

            // Simulate going to background
            Object.defineProperty(document, 'visibilityState', {
                value: 'hidden',
                writable: true,
                configurable: true,
            })
            document.dispatchEvent(new Event('visibilitychange'))

            // WebSocket should still be open (push not available)
            expect(ws.readyState).toBe(MockWebSocket.OPEN)
        })

        it('should disconnect WebSocket when push IS available', () => {
            events.init()
            const ws = connectAndGetWs()
            events.pushAvailable.value = true

            // Simulate going to background
            Object.defineProperty(document, 'visibilityState', {
                value: 'hidden',
                writable: true,
                configurable: true,
            })
            document.dispatchEvent(new Event('visibilitychange'))

            // WebSocket should be closed (push available to deliver events)
            expect(ws.readyState).toBe(MockWebSocket.CLOSED)
        })

        it('should reconnect on foreground after background with push', () => {
            events.init()
            const ws = connectAndGetWs()
            events.pushAvailable.value = true

            // Go to background (disconnects)
            Object.defineProperty(document, 'visibilityState', {
                value: 'hidden',
                writable: true,
                configurable: true,
            })
            document.dispatchEvent(new Event('visibilitychange'))
            expect(ws.readyState).toBe(MockWebSocket.CLOSED)

            // Come back to foreground
            Object.defineProperty(document, 'visibilityState', {
                value: 'visible',
                writable: true,
                configurable: true,
            })
            document.dispatchEvent(new Event('visibilitychange'))

            // A new WebSocket should be created
            const newWs = getLatestWs()
            expect(newWs).not.toBe(ws)
        })
    })
})

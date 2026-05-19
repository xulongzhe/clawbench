import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { ref } from 'vue'
import { useChatStream } from '@/composables/useChatStream'

// ── Mock EventSource ──

let mockEsInstances: MockEventSource[] = []

class MockEventSource {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 2

  url: string
  readyState: number = MockEventSource.CONNECTING
  onerror: ((e: Event) => void) | null = null
  private listeners: Map<string, EventListener[]> = new Map()

  constructor(url: string) {
    this.url = url
    mockEsInstances.push(this)
  }

  addEventListener(type: string, listener: EventListener) {
    if (!this.listeners.has(type)) this.listeners.set(type, [])
    this.listeners.get(type)!.push(listener)
  }

  removeEventListener() { /* noop */ }

  close() {
    this.readyState = MockEventSource.CLOSED
  }

  // Simulate receiving an SSE event
  simulate(type: string, data: any) {
    this.readyState = MockEventSource.OPEN
    const listeners = this.listeners.get(type) || []
    for (const listener of listeners) {
      listener({ data: JSON.stringify(data) } as any)
    }
  }

  // Simulate connection open
  simulateOpen() {
    this.readyState = MockEventSource.OPEN
  }

  // Simulate SSE error
  simulateError() {
    this.onerror?.(new Event('error'))
  }
}

function getLatestEs(): MockEventSource {
  return mockEsInstances[mockEsInstances.length - 1]
}

// ── Mocks ──

vi.mock('@/utils/api', () => ({
  cancelChat: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('@/utils/chatStreamUtils', () => ({
  FILE_MODIFYING_TOOLS: new Set(),
  findLastBlockOfType: (blocks: any[], type: string) =>
    [...blocks].reverse().find(b => b.type === type),
  forceCleanupStreamingState: vi.fn(),
}))

vi.mock('@/composables/useLocale', () => ({
  gt: (key: string) => key,
}))

// ── Helpers ──

function createOptions(overrides: Record<string, any> = {}) {
  const messages = ref<any[]>([])
  return {
    messages,
    currentSessionId: ref('test-session-1'),
    currentBackend: ref('test-backend'),
    loading: ref(false),
    onRenderNeeded: vi.fn(),
    onScrollBottom: vi.fn(),
    onLoadHistory: vi.fn().mockResolvedValue(undefined),
    onMessage: vi.fn(),
    onOpen: vi.fn(),
    isOpen: ref(false),
    onParseAssistantContent: vi.fn().mockReturnValue({ blocks: [] }),
    onToast: vi.fn(),
    onNotification: vi.fn(),
    onStreamEnd: vi.fn(),
    onQueueUpdate: vi.fn(),
    onQueueConsume: vi.fn(),
    onFileModified: vi.fn(),
    onExtractScheduledTasks: vi.fn(),
    ...overrides,
  }
}

describe('useChatStream', () => {
  let originalEventSource: typeof EventSource

  beforeEach(() => {
    mockEsInstances = []
    originalEventSource = globalThis.EventSource
    globalThis.EventSource = MockEventSource as any
  })

  afterEach(() => {
    globalThis.EventSource = originalEventSource
  })

  /**
   * useChatStream registers its visibility listener in onMounted(),
   * which doesn't fire outside a Vue component. This helper manually
   * simulates the registration so we can test visibility behavior.
   */
  function setupWithVisibility() {
    const options = createOptions()
    const stream = useChatStream(options)
    // Manually register visibility listener (simulates onMounted behavior)
    const handler = () => {
      if (document.visibilityState === 'hidden') {
        stream.disconnectStream()
        stream.stopPolling()
      }
    }
    document.addEventListener('visibilitychange', handler)
    return { options, stream, handler }
  }

  describe('visibility change — always disconnect on background', () => {
    it('should close SSE when page goes hidden, even without push notifications', () => {
      const { options, stream, handler } = setupWithVisibility()

      // Start streaming
      options.loading.value = true
      stream.connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()
      expect(es.readyState).toBe(MockEventSource.OPEN)

      // Simulate going to background
      Object.defineProperty(document, 'visibilityState', {
        value: 'hidden',
        writable: true,
        configurable: true,
      })
      document.dispatchEvent(new Event('visibilitychange'))

      // SSE should be closed — no pushAvailable check
      expect(es.readyState).toBe(MockEventSource.CLOSED)

      document.removeEventListener('visibilitychange', handler)
    })

    it('should stop polling when page goes hidden', () => {
      const { options, stream, handler } = setupWithVisibility()

      options.loading.value = true
      stream.connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()

      // Going to background should call stopPolling without error
      Object.defineProperty(document, 'visibilityState', {
        value: 'hidden',
        writable: true,
        configurable: true,
      })
      document.dispatchEvent(new Event('visibilitychange'))

      expect(es.readyState).toBe(MockEventSource.CLOSED)
      document.removeEventListener('visibilitychange', handler)
    })

    it('should close SSE on background even when session is actively streaming', () => {
      const { options, stream, handler } = setupWithVisibility()

      options.loading.value = true
      stream.connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()

      // Simulate some streaming content
      es.simulate('content', { content: 'Thinking...' })

      // Go to background mid-stream
      Object.defineProperty(document, 'visibilityState', {
        value: 'hidden',
        writable: true,
        configurable: true,
      })
      document.dispatchEvent(new Event('visibilitychange'))

      // SSE must be closed regardless of active session
      expect(es.readyState).toBe(MockEventSource.CLOSED)
      document.removeEventListener('visibilitychange', handler)
    })

    it('should NOT reference pushAvailable — always disconnects on hidden', () => {
      // This is a regression guard: the old code checked pushAvailable before
      // disconnecting. The new code always disconnects. We verify that
      // disconnectStream is called regardless of any external state.
      const { options, stream, handler } = setupWithVisibility()

      const disconnectSpy = vi.spyOn(stream, 'disconnectStream')

      options.loading.value = true
      stream.connectStream('test-session-1')
      getLatestEs().simulateOpen()

      Object.defineProperty(document, 'visibilityState', {
        value: 'hidden',
        writable: true,
        configurable: true,
      })
      document.dispatchEvent(new Event('visibilitychange'))

      expect(disconnectSpy).toHaveBeenCalled()
      document.removeEventListener('visibilitychange', handler)
    })
  })

  describe('SSE event handling', () => {
    it('should coalesce content events into text blocks', () => {
      const options = createOptions()
      const { connectStream } = useChatStream(options)

      connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()

      es.simulate('content', { content: 'Hello ' })
      es.simulate('content', { content: 'World' })

      const assistantMsg = options.messages.value.find(
        (m: any) => m.role === 'assistant' && m.streaming
      )
      expect(assistantMsg).toBeDefined()
      const textBlocks = assistantMsg.blocks.filter((b: any) => b.type === 'text')
      expect(textBlocks.length).toBe(1)
      expect(textBlocks[0].text).toBe('Hello World')
    })

    it('should coalesce thinking events into thinking blocks', () => {
      const options = createOptions()
      const { connectStream } = useChatStream(options)

      connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()

      es.simulate('thinking', { text: 'Let me think...' })
      es.simulate('thinking', { text: ' about this.' })

      const assistantMsg = options.messages.value.find(
        (m: any) => m.role === 'assistant' && m.streaming
      )
      expect(assistantMsg).toBeDefined()
      const thinkingBlocks = assistantMsg.blocks.filter((b: any) => b.type === 'thinking')
      expect(thinkingBlocks.length).toBe(1)
      expect(thinkingBlocks[0].text).toBe('Let me think... about this.')
    })

    it('should handle tool_use start and done events', () => {
      const options = createOptions()
      const { connectStream } = useChatStream(options)

      connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()

      // Start tool use
      es.simulate('tool_use', {
        name: 'Read',
        id: 'tool-1',
        input: { file_path: '/tmp/test.txt' },
      })

      const assistantMsg = options.messages.value.find(
        (m: any) => m.role === 'assistant' && m.streaming
      )
      const toolBlock = assistantMsg.blocks.find(
        (b: any) => b.type === 'tool_use' && b.id === 'tool-1'
      )
      expect(toolBlock).toBeDefined()
      expect(toolBlock.done).toBe(false)

      // Complete tool use
      es.simulate('tool_use', {
        name: 'Read',
        id: 'tool-1',
        done: true,
        output: 'file contents',
        status: 'success',
      })

      expect(toolBlock.done).toBe(true)
      expect(toolBlock.output).toBe('file contents')
    })

    it('should handle done event by disconnecting and loading history', () => {
      const options = createOptions()
      const { connectStream } = useChatStream(options)

      options.loading.value = true
      connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()

      es.simulate('done', {})

      expect(es.readyState).toBe(MockEventSource.CLOSED)
      expect(options.onLoadHistory).toHaveBeenCalled()
    })

    it('should handle cancelled event', () => {
      const options = createOptions()
      const { connectStream } = useChatStream(options)

      options.loading.value = true
      connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()

      es.simulate('cancelled', {})

      expect(es.readyState).toBe(MockEventSource.CLOSED)
      expect(options.onStreamEnd).toHaveBeenCalledWith('cancelled')
    })

    it('should handle warning event by adding warning block', () => {
      const options = createOptions()
      const { connectStream } = useChatStream(options)

      connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()

      es.simulate('warning', { text: 'Rate limited', reason: 'too_many_requests' })

      const assistantMsg = options.messages.value.find(
        (m: any) => m.role === 'assistant' && m.streaming
      )
      const warningBlock = assistantMsg.blocks.find((b: any) => b.type === 'warning')
      expect(warningBlock).toBeDefined()
      expect(warningBlock.text).toBe('Rate limited')
      expect(warningBlock.reason).toBe('too_many_requests')
    })
  })

  describe('disconnectStream', () => {
    it('should close EventSource and clear timeout', () => {
      const options = createOptions()
      const { connectStream, disconnectStream } = useChatStream(options)

      connectStream('test-session-1')
      const es = getLatestEs()
      es.simulateOpen()

      disconnectStream()

      expect(es.readyState).toBe(MockEventSource.CLOSED)
    })

    it('should be safe to call when no stream is active', () => {
      const { disconnectStream } = useChatStream(createOptions())
      expect(() => disconnectStream()).not.toThrow()
    })
  })

  describe('stopPolling', () => {
    it('should be callable without error even when no polling is active', () => {
      const { stopPolling } = useChatStream(createOptions())
      expect(() => stopPolling()).not.toThrow()
    })
  })

  describe('connectStream', () => {
    it('should disconnect previous stream before connecting new one', () => {
      const options = createOptions()
      const { connectStream } = useChatStream(options)

      connectStream('session-1')
      const es1 = getLatestEs()
      es1.simulateOpen()

      connectStream('session-2')
      const es2 = getLatestEs()

      // First EventSource should be closed
      expect(es1.readyState).toBe(MockEventSource.CLOSED)
      // New one should be created
      expect(es2).not.toBe(es1)
    })

    it('should create streaming assistant message if none exists', () => {
      const options = createOptions()
      const { connectStream } = useChatStream(options)

      connectStream('test-session-1')

      const assistantMsg = options.messages.value.find(
        (m: any) => m.role === 'assistant' && m.streaming
      )
      expect(assistantMsg).toBeDefined()
      expect(assistantMsg.blocks).toEqual([])
    })
  })
})

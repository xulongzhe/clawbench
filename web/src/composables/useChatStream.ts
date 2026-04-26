import { onUnmounted, type Ref } from 'vue'
import { cancelChat } from '@/utils/api.ts'

export interface UseChatStreamOptions {
  messages: Ref<any[]>
  currentSessionId: Ref<string>
  currentBackend: Ref<string>
  loading: Ref<boolean>
  inputDisabled: Ref<boolean>
  onRenderNeeded: (forceFull?: boolean) => void
  onScrollBottom: (force?: boolean) => void
  onLoadHistory: () => Promise<void>
  onMessage: () => void
  onOpen: () => void
  isOpen: Ref<boolean>
  createScheduledTask: (proposal: any) => void
  onParseAssistantContent: (content: string) => { blocks: any[]; metadata?: any; cancelled?: boolean; scheduledTask?: any }
  onToast: (msg: string, opts?: any) => void
  onNotification: (title: string, opts?: any) => void
  onPlaySound?: () => void
}

export function useChatStream(options: UseChatStreamOptions) {
  const {
    messages,
    currentSessionId,
    currentBackend,
    loading,
    inputDisabled,
    onRenderNeeded,
    onScrollBottom,
    onLoadHistory,
    onMessage,
    onOpen,
    isOpen,
    createScheduledTask,
    onParseAssistantContent,
    onToast,
    onNotification,
    onPlaySound,
  } = options

  let eventSource: EventSource | null = null
  let reconnectAttempts = 0
  let streamTimeout: ReturnType<typeof setTimeout> | null = null
  let renderTimer: ReturnType<typeof setTimeout> | null = null
  let lastScrollTime = 0
  let pollingInterval: ReturnType<typeof setInterval> | null = null

  const MAX_RECONNECT_ATTEMPTS = 3
  const STREAM_TIMEOUT_MS = 60000 // 60 seconds without any SSE event = try reconnect

  function debouncedRender() {
    if (renderTimer) clearTimeout(renderTimer)
    renderTimer = window.setTimeout(() => {
      onRenderNeeded()
      // 减少scrollBottom调用频率，每500ms最多一次
      if (Date.now() - lastScrollTime > 500) {
        onScrollBottom()
        lastScrollTime = Date.now()
      }
      renderTimer = null
    }, 200) // 增加防抖时间到200ms
  }

  function resetStreamTimeout() {
    if (streamTimeout) clearTimeout(streamTimeout)
    streamTimeout = setTimeout(() => {
      console.warn('SSE stream timeout - no events received, reconnecting')
      // No SSE event received for too long — reconnect instead of killing the session
      disconnectStream()
      // The AI session continues on the backend; just reconnect SSE
      if (currentSessionId.value && loading.value && reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
        reconnectAttempts++
        connectStream(currentSessionId.value)
      } else {
        // Too many reconnect attempts or session no longer active, fall back to polling
        const streamingMsg = messages.value.find(m => m.role === 'assistant' && m.streaming)
        if (streamingMsg) {
          delete streamingMsg.streaming
          onRenderNeeded(true)
        }
        inputDisabled.value = false
        loading.value = false
        pollUntilDone()
      }
    }, STREAM_TIMEOUT_MS)
  }

  function disconnectStream() {
    if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }

  function stopPolling() {
    if (pollingInterval) { clearInterval(pollingInterval); pollingInterval = null }
  }

  function pollUntilDone() {
    stopPolling()
    let jsonParseFailures = 0
    const MAX_JSON_PARSE_FAILURES = 5
    pollingInterval = setInterval(async () => {
      try {
        const resp = await fetch(`/api/ai/chat?session_id=${encodeURIComponent(currentSessionId.value)}`)
        if (!resp.ok) {
          throw new Error(`HTTP ${resp.status}`)
        }
        let data
        try {
          data = await resp.json()
          jsonParseFailures = 0
        } catch {
          jsonParseFailures++
          if (jsonParseFailures >= MAX_JSON_PARSE_FAILURES) {
            console.error('Polling: too many invalid JSON responses, giving up')
            throw new Error('Invalid JSON response')
          }
          console.error('Polling: invalid JSON response')
          return
        }
        if (!data.running) {
          stopPolling()
          messages.value = (data.messages || []).map(msg => {
            if (msg.role === 'assistant') {
              const { blocks, metadata, cancelled, scheduledTask } = onParseAssistantContent(msg.content)
              msg.blocks = blocks
              if (metadata) msg.metadata = metadata
              if (cancelled) msg.cancelled = cancelled
              if (scheduledTask) msg.scheduledTask = scheduledTask
            }
            return msg
          })
          currentSessionId.value = data.sessionId || currentSessionId.value
          onRenderNeeded(true)
          inputDisabled.value = false
          loading.value = false
          onMessage()
          onScrollBottom(true)
          onPlaySound?.()
          // Show toast notification when AI replies and chat panel is not open
          if (!isOpen.value) {
            const lastMsg = messages.value[messages.value.length - 1]
            if (lastMsg?.role === 'assistant') {
              onToast('AI 已回复', { icon: '🤖', duration: 5000, onClick: () => onOpen() })
              onNotification('AI 已回复', {
                body: '点击查看回复详情',
                onClick: () => onOpen()
              })
            }
          }
          return
        }
      } catch (err) {
        console.error('Polling error:', err)
        stopPolling()
        onToast('连接失败，请刷新页面', { icon: '⚠️' })
        inputDisabled.value = false
        loading.value = false
      }
    }, 2000)
  }

  function connectStream(sessionId: string) {
    disconnectStream()
    stopPolling()
    reconnectAttempts = 0

    // Find existing streaming message or create a new one
    let lastIndex = messages.value.findIndex(m => m.role === 'assistant' && m.streaming)
    if (lastIndex === -1) {
      // No streaming message from DB — create empty assistant message
      messages.value.push({
        role: 'assistant',
        content: '',
        blocks: [],
        streaming: true,
        createdAt: new Date().toISOString(),
        backend: currentBackend.value
      })
      lastIndex = messages.value.length - 1
    }
    onScrollBottom()

    // Guard: skip events if session changed or message was removed
    const guard = () => {
      if (currentSessionId.value !== sessionId) return false
      if (!messages.value[lastIndex]) return false
      return true
    }

    eventSource = new EventSource(`/api/ai/chat/stream?session_id=${encodeURIComponent(sessionId)}`)

    // Start stream timeout
    resetStreamTimeout()

    // Track if we've already created a task for this stream's proposal
    let proposalCreated = false

    // Helper: find the most recent block of a given type by searching backward.
    // This handles interleaved thinking/text events correctly — when events
    // alternate, the last block may not be the same type as the incoming event.
    // tool_use blocks act as natural boundaries — text/thinking after a tool_use
    // should not be merged with text/thinking before it.
    const findLastBlockOfType = (blocks: any[], type: string): any | undefined => {
      for (let i = blocks.length - 1; i >= 0; i--) {
        if (blocks[i].type === type) return blocks[i]
        // tool_use blocks are natural boundaries — don't merge across them
        if (blocks[i].type === 'tool_use') return undefined
      }
      return undefined
    }

    eventSource.addEventListener('content', (e) => {
      if (!guard()) return
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      // Coalesce content into the most recent text block
      const blocks = messages.value[lastIndex].blocks
      const existingText = findLastBlockOfType(blocks, 'text')
      if (existingText) {
        existingText.text += data.content
      } else {
        blocks.push({ type: 'text', text: data.content })
      }
      // Detect completed <schedule-proposal> tag during streaming and create task
      const fullText = findLastBlockOfType(blocks, 'text')?.text || ''
      if (!proposalCreated && /<schedule-proposal(\s+confirmed)?>[\s\S]*?<\/schedule-proposal>/.test(fullText)) {
        const match = fullText.match(/<schedule-proposal(\s+confirmed)?>([\s\S]*?)<\/schedule-proposal>/)
        if (match) {
          try {
            const proposal = JSON.parse(match[2].trim())
            proposalCreated = true
            createScheduledTask(proposal)
          } catch (err) {
            console.error('Failed to parse schedule proposal:', err)
          }
        }
      }
      debouncedRender()
    })

    eventSource.addEventListener('thinking', (e) => {
      if (!guard()) return
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      const blocks = messages.value[lastIndex].blocks
      // Coalesce thinking into the most recent thinking block
      const existingThinking = findLastBlockOfType(blocks, 'thinking')
      if (existingThinking) {
        existingThinking.text += data.text
      } else {
        blocks.push({ type: 'thinking', text: data.text })
      }
      onScrollBottom()
    })

    eventSource.addEventListener('tool_use', (e) => {
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      if (!guard()) return
      const blocks = messages.value[lastIndex].blocks
      if (data.done) {
        // Find existing tool block by id and update
        const existing = blocks.find(b => b.type === 'tool_use' && b.id === data.id)
        if (existing) {
          existing.input = data.input || existing.input
          existing.done = true
        }
      } else {
        // New tool call
        blocks.push({ type: 'tool_use', name: data.name, id: data.id, input: data.input || {}, done: false })
      }
      onScrollBottom()
    })

    eventSource.addEventListener('metadata', (e) => {
      if (!guard()) return
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      messages.value[lastIndex].metadata = data
    })

    eventSource.addEventListener('done', () => {
      if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
      disconnectStream()
      // Reload from DB to ensure complete content — SSE events may have been
      // dropped during transmission, so the local state may be incomplete.
      onLoadHistory().finally(() => {
        inputDisabled.value = false
        loading.value = false
        onMessage()
        onScrollBottom(true)
        onPlaySound?.()
        if (!isOpen.value) {
          const lastMsg = messages.value[messages.value.length - 1]
          if (lastMsg?.role === 'assistant') {
            onToast('AI 已回复', { icon: '🤖', duration: 5000, onClick: () => onOpen() })
            onNotification('AI 已回复', {
              body: '点击查看回复详情',
              onClick: () => onOpen()
            })
          }
        }
      })
    })

    eventSource.addEventListener('cancelled', () => {
      if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
      disconnectStream()
      if (!guard()) return
      const msg = messages.value[lastIndex]
      msg.cancelled = true
      delete msg.streaming
      // If no content was received, add error block so the UI shows the error card instead of loading dots
      if ((!msg.blocks || msg.blocks.length === 0) && !msg.content) {
        msg.blocks = [{ type: 'error', text: '用户已中断' }]
      }
      onRenderNeeded()
      inputDisabled.value = false
      loading.value = false
    })

    eventSource.addEventListener('warning', (e) => {
      if (!guard()) return
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      const msg = messages.value[lastIndex]
      // Flush any streaming text before adding warning block
      if (msg.streamingText) {
        msg.blocks.push({ type: 'text', text: msg.streamingText })
        msg.streamingText = ''
      }
      msg.blocks.push({ type: 'warning', text: data.text })
      onRenderNeeded()
    })

    eventSource.addEventListener('error', (e) => {
      if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
      if (!guard()) return
      disconnectStream()
      // Backend reported error (e.g. session not running) — reload from DB for final state
      onLoadHistory().catch(() => {
        if (!guard()) return
        const data = JSON.parse(e.data)
        messages.value[lastIndex].content = `错误: ${data.error}`
        messages.value[lastIndex].blocks = [{ type: 'error', text: data.error }]
        delete messages.value[lastIndex].streaming
        onRenderNeeded()
        inputDisabled.value = false
        loading.value = false
      })
    })

    eventSource.onerror = () => {
      // SSE connection error — reconnect if session is still active
      if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
      disconnectStream()
      if (currentSessionId.value && loading.value && reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
        // AI session likely still running on backend, reconnect SSE
        reconnectAttempts++
        connectStream(currentSessionId.value)
      } else {
        // Too many attempts or session inactive — fall back to polling
        const streamingMsg = messages.value.find(m => m.role === 'assistant' && m.streaming)
        if (streamingMsg) {
          delete streamingMsg.streaming
          onRenderNeeded()
        }
        inputDisabled.value = false
        loading.value = false
        pollUntilDone()
      }
    }
  }

  async function cancelStream() {
    if (!currentSessionId.value || !loading.value) return
    try {
      await cancelChat(currentSessionId.value)
    } catch (err) {
      console.error('Failed to cancel:', err)
      // Force local state reset even if API call fails
      disconnectStream()
      inputDisabled.value = false
      loading.value = false
    }
  }

  // Cleanup on unmount
  onUnmounted(() => {
    disconnectStream()
    stopPolling()
  })

  return {
    connectStream,
    disconnectStream,
    cancelStream,
    stopPolling,
  }
}

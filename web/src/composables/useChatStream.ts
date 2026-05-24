import { onMounted, onUnmounted, type Ref } from 'vue'
import { cancelChat } from '@/utils/api'
import { useReconnect } from './useReconnect'
import { gt } from '@/composables/useLocale'
import { FILE_MODIFYING_TOOLS, findLastBlockOfType, forceCleanupStreamingState as _forceCleanupStreamingState } from '@/utils/chatStreamUtils.ts'

export interface UseChatStreamOptions {
  messages: Ref<any[]>
  currentSessionId: Ref<string>
  currentBackend: Ref<string>
  loading: Ref<boolean>
  onRenderNeeded: (forceFull?: boolean) => void
  onScrollBottom: (force?: boolean) => void
  onLoadHistory: () => Promise<void>
  onMessage: () => void
  onOpen: () => void
  isOpen: Ref<boolean>
  onParseAssistantContent: (content: string) => { blocks: any[]; metadata?: any; cancelled?: boolean }
  onToast: (msg: string, opts?: any) => void
  onNotification: (title: string, opts?: any) => void
  onStreamEnd?: (reason: 'done' | 'cancelled' | 'error') => void
  onQueueUpdate?: (queue: any[]) => void
  onQueueConsume?: () => void
  onFileModified?: (filePath: string) => void
  onExtractScheduledTasks?: (msgs: any[]) => void
}

export function useChatStream(options: UseChatStreamOptions) {
  const {
    messages,
    currentSessionId,
    currentBackend,
    loading,
    onRenderNeeded,
    onScrollBottom,
    onLoadHistory,
    onMessage,
    onOpen,
    isOpen,
    onParseAssistantContent,
    onToast,
    onNotification,
    onStreamEnd,
    onQueueUpdate,
    onQueueConsume,
    onFileModified,
    onExtractScheduledTasks,
  } = options

  let eventSource: EventSource | null = null
  let streamTimeout: ReturnType<typeof setTimeout> | null = null
  let renderTimer: ReturnType<typeof setTimeout> | null = null
  let pollingInterval: ReturnType<typeof setInterval> | null = null
  // Track tool_use timeout timers so we can clean them up
  const toolUseTimeouts: Map<string, ReturnType<typeof setTimeout>> = new Map()

  const STREAM_TIMEOUT_MS = 30000 // 30 seconds without any SSE event = try reconnect
  const TOOL_USE_TIMEOUT_MS = 30000 // 30 seconds without 'done' event = mark as done

  const reconnect = useReconnect({
    maxAttempts: 3,
    baseDelay: 2000,
    onReconnect: () => connectStream(currentSessionId.value, true),
  })

  function debouncedRender() {
    if (renderTimer) clearTimeout(renderTimer)
    // Panel not visible: skip rendering and scrolling — data still accumulates,
    // rendering will catch up when the tab becomes active (loadHistory on re-activate)
    if (!isOpen.value) {
      renderTimer = null
      return
    }
    renderTimer = window.setTimeout(() => {
      onRenderNeeded()
      onScrollBottom()
      renderTimer = null
    }, 80)
  }

  function resetStreamTimeout() {
    if (streamTimeout) clearTimeout(streamTimeout)
    streamTimeout = setTimeout(() => {
      console.warn('SSE stream timeout - no events received, reconnecting')
      // No SSE event received for too long — reconnect instead of killing the session
      disconnectStream()
      // The AI session continues on the backend; just reconnect SSE
      if (currentSessionId.value && loading.value && reconnect.shouldReconnect()) {
        reconnect.scheduleReconnect()
      } else {
        // Too many reconnect attempts or session no longer active, fall back to polling
        forceCleanupStreamingState()
        pollUntilDone()
      }
    }, STREAM_TIMEOUT_MS)
  }

  function disconnectStream() {
    if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
    clearToolUseTimeouts()
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }

  function clearToolUseTimeouts() {
    for (const timer of toolUseTimeouts.values()) {
      clearTimeout(timer)
    }
    toolUseTimeouts.clear()
  }

  /**
   * Clean up streaming state for the current assistant message.
   * Delegates to the extracted pure function, then handles composable-specific
   * cleanup (tool_use timeouts, loading state).
   */
  function forceCleanupStreamingState() {
    clearToolUseTimeouts()
    _forceCleanupStreamingState(messages.value, {
      onRenderNeeded,
      onExtractScheduledTasks,
    })
    loading.value = false
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
        const resp = await fetch(`/api/ai/chat?session_id=${encodeURIComponent(currentSessionId.value)}`, { credentials: 'same-origin' })
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
        // Parse messages from server response
        const latestMsgs = (data.messages || []).map(msg => {
          if (msg.role === 'assistant') {
            const { blocks, metadata, cancelled } = onParseAssistantContent(msg.content)
            msg.blocks = blocks
            if (metadata) msg.metadata = metadata
            if (cancelled) msg.cancelled = cancelled
          } else if (msg.role === 'user' && !msg.blocks) {
            msg.blocks = msg.content ? [{ type: 'text', text: msg.content }] : []
          }
          return msg
        })

        if (!data.running) {
          stopPolling()
          messages.value = latestMsgs
          currentSessionId.value = data.sessionId || currentSessionId.value
          // Only render and scroll when panel is visible
          if (isOpen.value) {
            onRenderNeeded(true)
            onScrollBottom(true)
          }
          loading.value = false
          onMessage()
          onStreamEnd?.('done')
          if (!isOpen.value) {
            const lastMsg = messages.value[messages.value.length - 1]
            if (lastMsg?.role === 'assistant') {
              onToast(gt('chat.stream.aiReplied'), { icon: '🤖', duration: 5000, onClick: () => onOpen() })
              onNotification(gt('chat.stream.aiReplied'), {
                body: gt('chat.stream.clickToViewReply'),
                onClick: () => onOpen()
              })
            }
          }
          return
        }
        // Session still running — update the streaming message with latest content
        // so the user sees progress even while polling (not stuck on stale content)
        const lastAssistant = latestMsgs.findLast(m => m.role === 'assistant')
        if (lastAssistant) {
          lastAssistant.streaming = true
        }
        messages.value = latestMsgs
        currentSessionId.value = data.sessionId || currentSessionId.value
        // Only render and scroll when panel is visible
        if (isOpen.value) {
          onRenderNeeded(true)
          onScrollBottom()
        }
      } catch (err) {
        console.error('Polling error:', err)
        stopPolling()
        // Remove empty assistant placeholder if it still exists
        const emptyIdx = messages.value.findIndex((m: any) => m.role === 'assistant' && !m.content && (!m.blocks || m.blocks.length === 0))
        if (emptyIdx !== -1) messages.value.splice(emptyIdx, 1)
        onToast(gt('chat.stream.connectionFailed'), { icon: '⚠️' })
        loading.value = false
        onRenderNeeded(true)
        onStreamEnd?.('error')
      }
    }, 2000)
  }

  function connectStream(sessionId: string, isRetry = false) {
    disconnectStream()
    stopPolling()
    // Only reset reconnect state for fresh/intentional connections (user action,
    // foreground return, network recovery). Do NOT reset for automatic reconnection
    // attempts — that would clear reconnectAttempts, making maxAttempts useless.
    if (!isRetry) {
      reconnect.reset()
    }

    // Find existing streaming message or create a new one
    let streamingMsg = messages.value.find(m => m.role === 'assistant' && m.streaming)
    if (!streamingMsg) {
      // No streaming message from DB — create empty assistant message
      messages.value.push({
        role: 'assistant',
        content: '',
        blocks: [],
        streaming: true,
        createdAt: new Date().toISOString(),
        backend: currentBackend.value
      })
      // Re-acquire from the reactive array so that all subsequent mutations
      // (blocks.push, text +=, metadata assignment) go through Vue's reactive
      // proxy and trigger UI re-renders. Without this, the local variable
      // still points to the raw object — Vue never sees the changes.
      streamingMsg = messages.value[messages.value.length - 1]
      // Keep renderedContents in sync with messages array
      onRenderNeeded()
    }
    onScrollBottom()

    // Guard: skip events if session changed or message was removed
    const guard = () => {
      if (currentSessionId.value !== sessionId) return false
      if (!messages.value.includes(streamingMsg)) return false
      return true
    }

    eventSource = new EventSource(`/api/ai/chat/stream?session_id=${encodeURIComponent(sessionId)}`, { withCredentials: true })

    // Start stream timeout
    resetStreamTimeout()

    eventSource.addEventListener('resume_split', () => {
      if (!guard()) return
      resetStreamTimeout()
      // AutoResumeBackend detected ExitPlanMode and will auto-resume.
      // Clear the streaming message's blocks so that resume content
      // starts fresh — prevents duplicate rendering of pre-resume
      // content (Issue #60).
      streamingMsg.blocks = []
      debouncedRender()
    })

    eventSource.addEventListener('content', (e) => {
      if (!guard()) return
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      // Coalesce content into the most recent text block
      const blocks = streamingMsg.blocks
      const existingText = findLastBlockOfType(blocks, 'text')
      if (existingText) {
        existingText.text += data.content
      } else {
        blocks.push({ type: 'text', text: data.content })
      }
      // Note: Task creation is now handled by the backend automatically
      debouncedRender()
    })

    eventSource.addEventListener('thinking', (e) => {
      if (!guard()) return
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      const blocks = streamingMsg.blocks
      // Coalesce thinking into the most recent thinking block
      const existingThinking = findLastBlockOfType(blocks, 'thinking')
      if (existingThinking) {
        existingThinking.text += data.text
      } else {
        blocks.push({ type: 'thinking', text: data.text })
      }
      // Skip scroll when panel not visible
      if (isOpen.value) {
        onScrollBottom()
      }
    })

    eventSource.addEventListener('tool_use', (e) => {
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      if (!guard()) return
      const blocks = streamingMsg.blocks
      // Always check for existing block with same ID first — the backend may
      // emit multiple tool_use events for the same call (start + stop), and
      // we should merge them rather than creating duplicates.
      const existing = blocks.find(b => b.type === 'tool_use' && b.id === data.id)
      if (data.done) {
        if (existing) {
          existing.input = data.input || existing.input
          existing.done = true
          if (data.output !== undefined) existing.output = data.output
          if (data.status !== undefined) existing.status = data.status
        }
        // Clear timeout if set
        const timer = toolUseTimeouts.get(data.id)
        if (timer) { clearTimeout(timer); toolUseTimeouts.delete(data.id) }

        // Notify file modification: when a file-modifying tool completes,
        // extract the file_path from its input and call the callback.
        // This provides reliable preview refresh even when fsnotify SSE
        // is disconnected (defense-in-depth with the file watcher).
        if (FILE_MODIFYING_TOOLS.has(data.name) && onFileModified) {
          const input = data.input || existing?.input
          const filePath = input?.file_path
          if (filePath) {
            onFileModified(filePath)
          }
        }
      } else {
        if (existing) {
          // Update existing block with new input data (may be richer than start event)
          if (data.input && Object.keys(data.input).length > 0) {
            existing.input = data.input
          }
          if (data.output !== undefined) existing.output = data.output
          if (data.status !== undefined) existing.status = data.status
        } else {
          // New tool call — start timeout as safety net
          const newBlock = { type: 'tool_use', name: data.name, id: data.id, input: data.input || {}, done: false, output: data.output || '', status: data.status || '' }
          blocks.push(newBlock)
          const timer = setTimeout(() => {
            if (!newBlock.done) {
              console.warn(`tool_use block ${data.id} timed out without 'done', marking as done`)
              newBlock.done = true
              onRenderNeeded()
            }
            toolUseTimeouts.delete(data.id)
          }, TOOL_USE_TIMEOUT_MS)
          toolUseTimeouts.set(data.id, timer)
        }
      }
      // Skip scroll when panel not visible
      if (isOpen.value) {
        onScrollBottom()
      }
    })

    eventSource.addEventListener('tool_result', (e) => {
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      if (!guard()) return
      const blocks = streamingMsg.blocks
      // Find the matching tool_use block and update output/status
      const existing = blocks.find(b => b.type === 'tool_use' && b.id === data.id)
      if (existing) {
        if (data.output !== undefined) existing.output = data.output
        if (data.status !== undefined) existing.status = data.status
      }
      // Skip scroll when panel not visible
      if (isOpen.value) {
        onScrollBottom()
      }
    })

    eventSource.addEventListener('metadata', (e) => {
      if (!guard()) return
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      streamingMsg.metadata = data
    })

    eventSource.addEventListener('done', () => {
      if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
      clearToolUseTimeouts()
      disconnectStream()
      reconnect.reset() // Stream completed — reset reconnect state for future sessions
      // Reload from DB to ensure complete content — SSE events may have been
      // dropped during transmission, so the local state may be incomplete.
      onLoadHistory().finally(() => {
        loading.value = false
        onMessage()
        // Only scroll when panel is visible; loadHistory on
        // re-activate will handle the refresh
        if (isOpen.value) {
          onScrollBottom(true)
        }
        onStreamEnd?.('done')
        if (!isOpen.value) {
          const lastMsg = messages.value[messages.value.length - 1]
          if (lastMsg?.role === 'assistant') {
            onToast(gt('chat.stream.aiReplied'), { icon: '🤖', duration: 5000, onClick: () => onOpen() })
            onNotification(gt('chat.stream.aiReplied'), {
              body: gt('chat.stream.clickToViewReply'),
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
      streamingMsg.cancelled = true
      // If no content was received, add error block so the UI shows the error card instead of loading dots
      if ((!streamingMsg.blocks || streamingMsg.blocks.length === 0) && !streamingMsg.content) {
        streamingMsg.blocks = [{ type: 'error', text: gt('chat.stream.userCancelled') }]
      }
      _forceCleanupStreamingState(messages.value, { onRenderNeeded, onExtractScheduledTasks })
      loading.value = false
      onStreamEnd?.('cancelled')
    })

    eventSource.addEventListener('warning', (e) => {
      if (!guard()) return
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      // Flush any streaming text before adding warning block
      if (streamingMsg.streamingText) {
        streamingMsg.blocks.push({ type: 'text', text: streamingMsg.streamingText })
        streamingMsg.streamingText = ''
      }
      const warningBlock = { type: 'warning', text: data.text }
      if (data.reason) warningBlock.reason = data.reason
      streamingMsg.blocks.push(warningBlock)
      // Skip render when panel not visible — data is accumulated regardless
      if (isOpen.value) {
        onRenderNeeded()
      }
    })

    eventSource.addEventListener('queue_consume', (e) => {
      resetStreamTimeout()
      // Always update pending queue — it's independent of the streaming message
      onQueueConsume?.()
      if (!guard()) return
      const data = JSON.parse(e.data)

      // Add user message bubble (DB message already persisted by backend)
      const userContent = data.text || ''
      messages.value.push({
        role: 'user',
        content: userContent,
        blocks: userContent ? [{ type: 'text', text: userContent }] : [],
        files: (data.files || []).map(p => ({ path: p })),
        createdAt: new Date().toISOString(),
      })

      // Create new streaming assistant placeholder
      messages.value.push({
        role: 'assistant',
        content: '',
        blocks: [],
        streaming: true,
        createdAt: new Date().toISOString(),
        backend: currentBackend.value,
      })
      // Re-acquire from the reactive array so mutations go through
      // Vue's reactive proxy (see connectStream for the same pattern)
      streamingMsg = messages.value[messages.value.length - 1]

      // Skip render/scroll when panel not visible
      if (isOpen.value) {
        onRenderNeeded()
        // Force scroll: queue_done removes the streaming indicator which shrinks layout,
        // making isAtBottom=false even though the user is visually at the bottom.
        // Since new messages are being injected, always scroll to show them.
        onScrollBottom(true)
      }
    })

    eventSource.addEventListener('queue_update', (e) => {
      resetStreamTimeout()
      const data = JSON.parse(e.data)
      // Always update pending queue — it's independent of the streaming message
      onQueueUpdate?.(data.queue || [])
    })

    eventSource.addEventListener('queue_done', () => {
      if (!guard()) return
      resetStreamTimeout()
      // Current streaming message is finalized — clear loading state
      // before the next queued message starts (queue_consume)
      _forceCleanupStreamingState(messages.value, { onRenderNeeded, onExtractScheduledTasks })
      // Skip scroll when panel not visible
      if (isOpen.value) {
        // Re-sync scroll position: removing the streaming indicator and pending
        // messages shrinks the layout, which can make isAtBottom=false even when
        // the user is visually at the bottom. Scroll to ensure isAtBottom stays
        // accurate before queue_consume arrives.
        onScrollBottom()
      }
    })

    eventSource.addEventListener('error', (e) => {
      if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
      if (!guard()) return
      disconnectStream()
      // Backend reported error (e.g. session not running) — reload from DB for final state
      onLoadHistory().catch(() => {
        if (!guard()) return
        const data = JSON.parse(e.data)
        streamingMsg.content = `${gt('chat.stream.errorPrefix')} ${data.error}`
        const errorBlock = { type: 'error', text: data.error }
        if (data.reason) errorBlock.reason = data.reason
        streamingMsg.blocks = [errorBlock]
        _forceCleanupStreamingState(messages.value, { onRenderNeeded, onExtractScheduledTasks })
        loading.value = false
      })
      onStreamEnd?.('error')
    })

    eventSource.onerror = () => {
      // SSE connection error — reconnect if session is still active
      if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
      disconnectStream()
      if (currentSessionId.value && loading.value && reconnect.shouldReconnect()) {
        // AI session likely still running on backend, reconnect SSE
        reconnect.scheduleReconnect()
      } else {
        // Too many attempts or session inactive — fall back to polling
        reconnect.reset() // Clear reconnect state before falling back to polling
        forceCleanupStreamingState()
        pollUntilDone()
      }
    }
  }

  async function cancelStream() {
    if (!currentSessionId.value || !loading.value) return
    try {
      await cancelChat(currentSessionId.value)
      // Backend will send 'cancelled' SSE event which triggers onStreamEnd.
      // If the SSE connection is already dead, forceCleanup won't happen here —
      // the onerror handler or global polling will take over.
    } catch (err) {
      console.error('Failed to cancel:', err)
      // Force local state reset even if API call fails
      disconnectStream()
      forceCleanupStreamingState()
      onStreamEnd?.('cancelled')
    }
  }

  // Network recovery: when the browser regains connectivity after a temporary
  // loss (e.g., WiFi→cellular, tunnel), the SSE connection may be silently dead.
  // The 'online' event lets us reconnect immediately instead of waiting for timeout.
  function handleOnline() {
    if (!loading.value || !currentSessionId.value) return
    // Only reconnect if we have an active EventSource that might be stale
    if (eventSource) {
      console.info('Network recovered, reconnecting SSE stream')
      disconnectStream()
      // connectStream with isRetry=false will reset reconnect state
      connectStream(currentSessionId.value)
    }
  }
  window.addEventListener('online', handleOnline)

  // Visibility change: always close SSE and polling when going to background.
  // Mobile OS will throttle/kill background connections anyway, so keeping SSE
  // alive is a waste of resources. On foreground, ChatPanel's visibility handler
  // calls loadHistory which reconnects the stream if the session is still running.
  function handleStreamVisibility() {
    if (document.visibilityState === 'hidden') {
      disconnectStream()
      stopPolling()
    }
  }

  // Cleanup on unmount
  onMounted(() => {
    document.addEventListener('visibilitychange', handleStreamVisibility)
  })

  onUnmounted(() => {
    disconnectStream()
    stopPolling()
    clearToolUseTimeouts()
    window.removeEventListener('online', handleOnline)
    document.removeEventListener('visibilitychange', handleStreamVisibility)
  })

  return {
    connectStream,
    disconnectStream,
    cancelStream,
    stopPolling,
  }
}

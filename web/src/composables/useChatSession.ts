import { ref, computed, type Ref } from 'vue'
import { useToast } from '@/composables/useToast.ts'
import { useNotification } from '@/composables/useNotification.ts'
import { store } from '@/stores/app.ts'

export interface UseChatSessionOptions {
  currentSessionId: Ref<string>
  messages: Ref<any[]>
  loading: Ref<boolean>
  inputDisabled: Ref<boolean>
  renderedContents: Ref<string[]>
  blockProposals: Record<string, any>
  expandedTools: Ref<Record<string, boolean>>
  switching?: Ref<boolean>
  onParseAssistantContent: (content: string) => any
  onExtractScheduleProposals: (msgs: any[]) => void
  onRenderUpdate: (forceFull: boolean) => void
  onScrollBottom: (force?: boolean) => void
  onConnectStream: (sessionId: string) => void
  onStopPolling: () => void
  onDisconnectStream: () => void
  onMessage: () => void
  onOpen: () => void
  isOpen: Ref<boolean>
  onPlaySound?: () => void
}

export function useChatSession(options: UseChatSessionOptions) {
  const {
    currentSessionId,
    messages,
    loading,
    inputDisabled,
    renderedContents,
    blockProposals,
    expandedTools,
    onParseAssistantContent,
    onExtractScheduleProposals,
    onRenderUpdate,
    onScrollBottom,
    onConnectStream,
    onStopPolling,
    onDisconnectStream,
    onMessage,
    onOpen,
    isOpen,
    onPlaySound,
  } = options

  const toast = useToast()
  const notification = useNotification()

  // Switching state — true while a session switch is in progress (distinct from
  // "loading" which means "AI is generating"). Used to show a fade/placeholder
  // transition so the user sees immediate feedback instead of a frozen UI.
  const switching = ref(false)

  const currentSessionTitle = ref('')
  const currentBackend = ref('')
  const currentAgentId = ref('')
  const sessionDrawerOpen = ref(false)
  const taskDrawerOpen = ref(false)
  const agents = ref([])
  const runningSessions = ref(new Set())
  const lastMsgCount = ref(0)
  let msgCountInterval: ReturnType<typeof setInterval> | null = null
  let globalPollingInterval: ReturnType<typeof setInterval> | null = null

  // Pagination state
  const totalMessages = ref(0)
  const loadingMore = ref(false)
  const hasMore = computed(() => messages.value.length < totalMessages.value)

  const agentHeaderTitle = computed(() => {
    const agent = agents.value.find(a => a.id === currentAgentId.value)
    if (agent) return `${agent.icon} ${agent.name}`
    return 'AI 对话'
  })

  // Guard against concurrent switchSession calls — only the last one wins
  let switchSessionSeq = 0

  async function loadAgents() {
    try {
      const resp = await fetch('/api/agents')
      const data = await resp.json()
      agents.value = data.agents || []
    } catch (err) {
      console.error('Failed to load agents:', err)
    }
  }

  function getAgentIcon(agentId) {
    const agent = agents.value.find(a => a.id === agentId)
    return agent ? agent.icon : '🤖'
  }

  function getAgentName(agentId) {
    const agent = agents.value.find(a => a.id === agentId)
    return agent ? agent.name : (agentId || '助手')
  }

  function parseMessages(rawMsgs) {
    return rawMsgs.map(msg => {
      if (msg.role === 'assistant') {
        const { blocks, metadata, cancelled, scheduledTask } = onParseAssistantContent(msg.content)
        msg.blocks = blocks
        if (metadata) msg.metadata = metadata
        if (cancelled) msg.cancelled = cancelled
        if (scheduledTask) msg.scheduledTask = scheduledTask
        if (msg.streaming) { msg.streaming = true; msg.fromDB = true }
      }
      return msg
    })
  }

  // forceScrollBottom: true = always scroll to bottom (switch session, first load)
  //                   false = only scroll if already near bottom (re-open panel, polling)
  async function loadHistory(forceScrollBottom = true) {
    expandedTools.value = {}
    try {
      // Load agents first so we can resolve agent names
      if (agents.value.length === 0) await loadAgents()
      // Use max of initialMessages and current loaded count to avoid truncating lazy-loaded messages
      const limit = Math.max(store.state.chatInitialMessages, messages.value.length)
      const url = currentSessionId.value
        ? `/api/ai/chat?session_id=${encodeURIComponent(currentSessionId.value)}&limit=${limit}`
        : `/api/ai/chat?limit=${limit}`
      const resp = await fetch(url)
      if (!resp.ok) {
        const errData = await resp.json().catch(() => ({}))
        throw new Error(errData.error || `请求失败 (${resp.status})`)
      }
      const data = await resp.json()
      messages.value = parseMessages(data.messages || [])
      totalMessages.value = data.total || messages.value.length
      currentSessionId.value = data.sessionId || ''
      currentSessionTitle.value = data.sessionTitle || ''
      currentBackend.value = data.backend || ''
      currentAgentId.value = data.agentId || ''
      console.log('loadHistory - agentId:', data.agentId, 'currentAgentId:', currentAgentId.value)
      onExtractScheduleProposals(messages.value)
      onRenderUpdate(true)
      if (data.running) {
        inputDisabled.value = true
        loading.value = true
        stopMsgCountPolling()
        onScrollBottom(true)
        onConnectStream(currentSessionId.value)
      } else {
        inputDisabled.value = false
        loading.value = false
        startMsgCountPolling()
        onScrollBottom(forceScrollBottom)
      }
    } catch (err) {
      console.error('Failed to load chat history:', err)
      toast.show(err.message || '加载聊天记录失败', { icon: '⚠️', type: 'error' })
    }
  }

  async function loadMoreMessages() {
    if (loadingMore.value || !hasMore.value || !currentSessionId.value) return
    loadingMore.value = true
    try {
      const pageSize = store.state.chatPageSize
      // Use cursor-based pagination: pass the created_at of the oldest loaded message
      const oldestMsg = messages.value[0]
      const before = oldestMsg?.createdAt || ''
      const resp = await fetch(`/api/ai/chat?session_id=${encodeURIComponent(currentSessionId.value)}&limit=${pageSize}&before=${encodeURIComponent(before)}`)
      if (!resp.ok) return
      const data = await resp.json()
      const olderMsgs = parseMessages(data.messages || [])
      if (olderMsgs.length > 0) {
        messages.value = [...olderMsgs, ...messages.value]
        totalMessages.value = data.total || totalMessages.value
        onExtractScheduleProposals(olderMsgs)
        onRenderUpdate(true)
      }
    } catch (err) {
      console.error('Failed to load more messages:', err)
    } finally {
      loadingMore.value = false
    }
  }

  async function switchSession(sessionId) {
    // Increment sequence counter — if another switch starts before we finish,
    // our results will be discarded (last writer wins)
    const mySeq = ++switchSessionSeq

    // Mark switching state immediately so UI can show a fade/placeholder
    switching.value = true
    // Immediately lock input to prevent sending messages with stale sessionId.
    // Do NOT set loading=true here — loading means "AI is generating", not
    // "session is switching". Setting it would flash the stop button.
    inputDisabled.value = true

    onDisconnectStream()
    onStopPolling()
    stopMsgCountPolling()
    expandedTools.value = {}
    try {
      // Load agents first so we can resolve agent names
      if (agents.value.length === 0) await loadAgents()
      const limit = store.state.chatInitialMessages
      const resp = await fetch(`/api/ai/chat?session_id=${encodeURIComponent(sessionId)}&limit=${limit}`)
      if (!resp.ok) {
        toast.show('切换会话失败', { icon: '⚠️', type: 'error' })
        inputDisabled.value = false
        switching.value = false
        return
      }
      const data = await resp.json()

      // If another switch happened while we were fetching, discard our results
      // (the newer switch will set switching=false when it completes)
      if (switchSessionSeq !== mySeq) return

      messages.value = parseMessages(data.messages || [])
      totalMessages.value = data.total || messages.value.length
      currentSessionId.value = data.sessionId || sessionId
      currentSessionTitle.value = data.sessionTitle || ''
      currentBackend.value = data.backend || ''
      currentAgentId.value = data.agentId || ''
      onExtractScheduleProposals(messages.value)
      onRenderUpdate(true)
      onScrollBottom(true)
      if (data.running) {
        inputDisabled.value = true
        loading.value = true
        stopMsgCountPolling()
        onConnectStream(sessionId)
      } else {
        inputDisabled.value = false
        loading.value = false
        startMsgCountPolling()
      }
      switching.value = false
    } catch (err) {
      // If another switch happened, don't touch state
      if (switchSessionSeq !== mySeq) return
      console.error('Failed to switch session:', err)
      toast.show('切换会话失败', { icon: '⚠️' })
      inputDisabled.value = false
      switching.value = false
    }
  }

  async function createSession(agentId) {
    try {
      // Load agents first so UI can resolve agent names
      if (agents.value.length === 0) await loadAgents()
      const body = agentId ? { agentId } : {}
      const resp = await fetch('/api/ai/sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const data = await resp.json()
      if (!resp.ok || !data.ok) {
        throw new Error(data.error || `创建失败 (${resp.status})`)
      }
      currentSessionId.value = data.sessionId
      currentSessionTitle.value = ''
      currentBackend.value = data.backend || ''
      currentAgentId.value = data.agentId || agentId || ''
      messages.value = []
      renderedContents.value = []
      totalMessages.value = 0
      Object.keys(blockProposals).forEach(k => delete blockProposals[k])
      inputDisabled.value = false
      loading.value = false
      toast.show('已创建新会话', { icon: '✨', type: 'success', duration: 1500 })
    } catch (err) {
      console.error('Failed to create session:', err)
      toast.show(err.message || '创建会话失败', { icon: '⚠️', type: 'error' })
    }
  }

  async function deleteSession(sessionId, backend) {
    try {
      const resp = await fetch(`/api/ai/session/delete?session_id=${encodeURIComponent(sessionId)}&backend=${encodeURIComponent(backend || '')}`, {
        method: 'DELETE',
      })
      const data = await resp.json()
      if (data.ok) {
        // If deleted current session, switch to another
        if (sessionId === currentSessionId.value) {
          const sessionsResp = await fetch('/api/ai/sessions')
          const sessionsData = await sessionsResp.json()
          if (sessionsData.sessions && sessionsData.sessions.length > 0) {
            await switchSession(sessionsData.sessions[0].id, sessionsData.sessions[0].backend)
          } else {
            // No sessions left, create a default one
            await createSession()
          }
        }
        toast.show('会话已删除', { icon: '🗑️', type: 'success', duration: 2000 })
      }
    } catch (err) {
      console.error('Failed to delete session:', err)
      toast.show('删除会话失败', { icon: '⚠️', type: 'error' })
    }
  }

  function openSessionTab(tab) {
    if (tab === 'tasks') {
      taskDrawerOpen.value = true
    } else {
      sessionDrawerOpen.value = true
    }
  }

  function startMsgCountPolling() {
    stopMsgCountPolling()
    if (!currentSessionId.value) return
    lastMsgCount.value = messages.value.length
    msgCountInterval = setInterval(async () => {
      if (!currentSessionId.value || loading.value) return
      try {
        const resp = await fetch(`/api/ai/chat/count?session_id=${encodeURIComponent(currentSessionId.value)}`)
        if (!resp.ok) return
        const data = await resp.json()
        if (data.count > lastMsgCount.value) {
          lastMsgCount.value = data.count
          // Reload history to pick up new messages (don't force scroll)
          await loadHistory(false)
        }
      } catch (err) {
        // Silently ignore polling errors
      }
    }, 15000)
  }

  function stopMsgCountPolling() {
    if (msgCountInterval) { clearInterval(msgCountInterval); msgCountInterval = null }
  }

  function stopGlobalPolling() {
    if (globalPollingInterval) { clearInterval(globalPollingInterval); globalPollingInterval = null }
  }

  async function startGlobalPolling() {
    stopGlobalPolling()
    globalPollingInterval = setInterval(async () => {
      try {
        const resp = await fetch('/api/ai/sessions')
        const data = await resp.json()
        const sessions = data.sessions || []
        const newRunning = new Set(sessions.filter(s => s.running).map(s => s.id))

        // Check for unread messages in other sessions
        const hasUnreadOther = sessions.some(s => s.unreadCount > 0 && s.id !== currentSessionId.value)
        if (hasUnreadOther) {
          store.state.chatUnread = true
        }

        // Check for completed sessions
        for (const sessionId of runningSessions.value) {
          if (!newRunning.has(sessionId)) {
            if (sessionId === currentSessionId.value) {
              // Current session completed but UI may be stuck in loading state
              // (e.g. done event was dropped) — force reset
              if (loading.value) {
                loadHistory()
              }
            } else {
              // Other session completed
              const session = sessions.find(s => s.id === sessionId)
              if (session) {
                onPlaySound?.()
                toast.show('会话已完成', {
                  icon: '✅',
                  type: 'success',
                  duration: 5000,
                  onClick: () => {
                    switchSession(sessionId, session.backend)
                    onOpen()
                  }
                })
                // Also show browser notification for completed session
                notification.show('会话已完成', {
                  body: '点击查看详情',
                  onClick: () => {
                    switchSession(sessionId, session.backend)
                    onOpen()
                  }
                })
              }
            }
          }
        }

        runningSessions.value = newRunning
      } catch (err) {
        console.error('Global polling error:', err)
      }
    }, 2000)
  }

  function handleVisibilityChange() {
    if (document.visibilityState === 'visible' && loading.value) {
      // Page became visible while streaming - reconnect
      onDisconnectStream()
      onStopPolling()
      loadHistory().catch(() => {
        // loadHistory failed — reset loading state so user isn't stuck
        inputDisabled.value = false
        loading.value = false
      })
    }
  }

  return {
    currentSessionId,
    currentSessionTitle,
    currentBackend,
    currentAgentId,
    sessionDrawerOpen,
    taskDrawerOpen,
    agents,
    runningSessions,
    agentHeaderTitle,
    totalMessages,
    hasMore,
    loadingMore,
    switching,
    loadHistory,
    loadMoreMessages,
    switchSession,
    createSession,
    deleteSession,
    loadAgents,
    getAgentIcon,
    getAgentName,
    openSessionTab,
    startGlobalPolling,
    stopGlobalPolling,
    startMsgCountPolling,
    stopMsgCountPolling,
    handleVisibilityChange,
  }
}

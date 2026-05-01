import { ref, computed } from 'vue'
import { useAgents } from '@/composables/useAgents.ts'

// ───────────────────────────────────────────────────────────
// Module-level singleton state — shared across the whole app.
// Session identity is globally needed (App.vue, QuoteQuestionBar,
// ChatPanel, etc.) but session *interaction* (messages, stream,
// polling) belongs to ChatPanel. This singleton holds only the
// identity layer.
// ───────────────────────────────────────────────────────────

const currentSessionId = ref('')
const currentSessionTitle = ref('')
const currentBackend = ref('')
const currentAgentId = ref('')
const runningSessions = ref(new Set<string>())

// ───────────────────────────────────────────────────────────
// Action callbacks — registered by ChatPanel on mount.
// Inversion of control: singleton owns the identity refs, but
// ChatPanel owns the session *operations*. Other consumers
// (App.vue, QuoteQuestionBar) trigger actions through these
// proxies, which delegate to ChatPanel's implementation.
// ───────────────────────────────────────────────────────────

let _switchSession: ((sessionId: string) => Promise<void>) | null = null
let _createSession: ((agentId?: string) => Promise<void>) | null = null
let _deleteSession: ((sessionId: string, backend?: string) => Promise<void>) | null = null
let _sendMessage: ((text: string, filePaths?: string[]) => Promise<void>) | null = null
let _openChatPanel: (() => void) | null = null

export interface SessionActions {
  switchSession: (sessionId: string) => Promise<void>
  createSession: (agentId?: string) => Promise<void>
  deleteSession: (sessionId: string, backend?: string) => Promise<void>
  sendMessage: (text: string, filePaths?: string[]) => Promise<void>
  openChatPanel: () => void
}

/**
 * Register session action callbacks. Called by ChatPanel on mount.
 * These are stable across the ChatPanel lifecycle — only registered
 * once (guard below). If ChatPanel unmounts and remounts, the old
 * callbacks are replaced.
 */
export function registerSessionActions(actions: SessionActions) {
  _switchSession = actions.switchSession
  _createSession = actions.createSession
  _deleteSession = actions.deleteSession
  _sendMessage = actions.sendMessage
  _openChatPanel = actions.openChatPanel
}

/**
 * Pre-fill session identity from the API. Called by App.vue on mount
 * so that QuoteQuestionBar can display correct session info even
 * before ChatPanel is opened.
 */
export async function initSessionFromAPI() {
  const agents = useAgents()
  try {
    const [chatResp] = await Promise.all([
      fetch('/api/ai/chat?limit=1'),
      agents.loadAgents(),
    ])
    if (chatResp.ok) {
      const data = await chatResp.json()
      if (data.sessionId) {
        currentSessionId.value = data.sessionId
        currentSessionTitle.value = data.sessionTitle || ''
        currentBackend.value = data.backend || ''
        currentAgentId.value = data.agentId || ''
      }
    }
  } catch (_) {
    // Silently ignore — ChatPanel will load properly when opened
  }
}

// ───────────────────────────────────────────────────────────
// Computed helpers
// ───────────────────────────────────────────────────────────

const agentHeaderTitle = computed(() => {
  const { agents, getAgentName } = useAgents()
  const agent = agents.value.find(a => a.id === currentAgentId.value)
  if (agent) return `${agent.icon} ${agent.name}`
  return currentAgentId.value ? `${getAgentName(currentAgentId.value)}` : 'AI 对话'
})

// ───────────────────────────────────────────────────────────
// Public composable
// ───────────────────────────────────────────────────────────

export function useSessionIdentity() {
  /**
   * Switch to a different session. Delegates to ChatPanel's
   * implementation if registered, otherwise falls back to a
   * simple API call (pre-ChatPanel mount scenario).
   */
  async function switchSession(sessionId: string) {
    if (_switchSession) {
      await _switchSession(sessionId)
    }
  }

  /**
   * Create a new session. Delegates to ChatPanel if available,
   * otherwise makes a direct API call and updates identity refs.
   */
  async function createSession(agentId?: string) {
    if (_createSession) {
      await _createSession(agentId)
      return
    }
    // Fallback: direct API call (ChatPanel not yet mounted)
    try {
      const body = agentId ? { agentId } : {}
      const resp = await fetch('/api/ai/sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const data = await resp.json()
      if (data.ok && data.sessionId) {
        currentSessionId.value = data.sessionId
        currentSessionTitle.value = data.title || ''
        currentBackend.value = data.backend || ''
        currentAgentId.value = data.agentId || agentId || ''
      }
    } catch (err) {
      console.error('Failed to create session:', err)
    }
  }

  /**
   * Delete a session. Delegates to ChatPanel if available.
   */
  async function deleteSession(sessionId: string, backend?: string) {
    if (_deleteSession) {
      await _deleteSession(sessionId, backend)
    }
  }

  /**
   * Send a message to the current session. Delegates to ChatPanel
   * if available, otherwise makes a direct API call.
   */
  async function sendMessage(text: string, filePaths?: string[]) {
    if (_sendMessage) {
      await _sendMessage(text, filePaths)
      return
    }
    // Fallback: direct API call (ChatPanel not yet mounted)
    try {
      let sid = currentSessionId.value
      if (!sid) {
        const createResp = await fetch('/api/ai/sessions', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
        })
        const createData = await createResp.json()
        if (createData.ok && createData.sessionId) {
          sid = createData.sessionId
          currentSessionId.value = sid
        }
      }
      const url = sid
        ? `/api/ai/chat?session_id=${encodeURIComponent(sid)}`
        : '/api/ai/chat'
      await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: text, filePaths: filePaths || [] }),
      })
    } catch (err) {
      console.error('Failed to send message:', err)
    }
  }

  /**
   * Open the chat panel. Delegates to App.vue's openDrawer logic
   * via the registered callback.
   */
  function openChatPanel() {
    if (_openChatPanel) {
      _openChatPanel()
    }
  }

  return {
    // Identity refs (read-only for consumers; ChatPanel writes to them)
    currentSessionId,
    currentSessionTitle,
    currentBackend,
    currentAgentId,
    runningSessions,
    agentHeaderTitle,
    // Action proxies
    switchSession,
    createSession,
    deleteSession,
    sendMessage,
    openChatPanel,
    // Registration (for ChatPanel)
    registerSessionActions,
    // Init (for App.vue)
    initSessionFromAPI,
  }
}

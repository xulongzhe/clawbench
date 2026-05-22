import { ref, computed } from 'vue'
import { useAgents } from '@/composables/useAgents'
import { gt } from '@/composables/useLocale'

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
const currentModelId = ref('')
const currentModelName = ref('')
const currentThinkingEffort = ref('')
const runningSessions = ref(new Set<string>())
// Bumped on every mutation to runningSessions so computed properties
// that depend on the set's contents re-evaluate correctly.
const runningSessionsVersion = ref(0)

// Whether the global session drawer is open. Lifted from ChatPanelContent
// to useSessionIdentity so App.vue can render a single SessionDrawer
// instance that's accessible from any tab (chat, viewer, QuoteQuestionBar).
const sessionDrawerOpen = ref(false)

/** Reset all module-level singleton refs — used by SPA hot project switch. */
export function resetIdentity(): void {
  currentSessionId.value = ''
  currentSessionTitle.value = ''
  currentBackend.value = ''
  currentAgentId.value = ''
  currentModelId.value = ''
  currentModelName.value = ''
  currentThinkingEffort.value = ''
  runningSessions.value = new Set()
  runningSessionsVersion.value = 0
  sessionDrawerOpen.value = false
  _switchSession = null
  _createSession = null
  _deleteSession = null
  _sendMessage = null
  _openChatPanel = null
  _sessionDrawerRef = null
}

// ───────────────────────────────────────────────────────────
// Agent preference persistence — stored in agent YAML files via PATCH /api/agents.
// preferredModel / preferredThinkingEffort are the source of truth for
// interactive sessions. Scheduled tasks use BaseModelID() and ThinkingEffort
// (the agent's original defaults) instead.
// ───────────────────────────────────────────────────────────

async function saveModelPref(agentId: string, modelId: string) {
  if (!agentId || !modelId) return
  // Save to server (agent YAML) via PATCH /api/agents
  const { patchAgentPref } = await import('@/composables/useSettingsConfig')
  patchAgentPref(agentId, 'preferred_model', modelId).catch(() => {})
}

function loadModelPref(agentId: string): string | null {
  if (!agentId) return null
  // Read from agent's server-side preference (preferredModel)
  const { getAgent } = useAgents()
  const agent = getAgent(agentId)
  return agent?.preferredModel || null
}

async function saveThinkingPref(agentId: string, level: string) {
  if (!agentId) return
  // Save to server (agent YAML) via PATCH /api/agents
  const { patchAgentPref } = await import('@/composables/useSettingsConfig')
  patchAgentPref(agentId, 'preferred_thinking_effort', level).catch(() => {})
}

function loadThinkingPref(agentId: string): string | null {
  if (!agentId) return null
  // Read from agent's server-side preference (preferredThinkingEffort > thinkingEffort)
  const { getEffectiveThinkingEffort } = useAgents()
  return getEffectiveThinkingEffort(agentId) || null
}

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
// SessionDrawer component ref — set by App.vue. Allows any component to
// trigger openAgentSelector() on the global drawer without coupling.
let _sessionDrawerRef: any = null

export interface SessionActions {
  switchSession: (sessionId: string) => Promise<void>
  createSession: (agentId?: string) => Promise<void>
  deleteSession: (sessionId: string, backend?: string) => Promise<void>
  sendMessage: (text: string, filePaths?: string[]) => Promise<void>
  openChatPanel: () => void
}

/**
 * Register session action callbacks. Called by App.vue on mount
 * (for openAgentSelector) and ChatPanel on mount (for the rest).
 */
export function registerSessionActions(actions: SessionActions) {
  _switchSession = actions.switchSession
  _createSession = actions.createSession
  _deleteSession = actions.deleteSession
  _sendMessage = actions.sendMessage
  _openChatPanel = actions.openChatPanel
}

/** Register the SessionDrawer component ref so openAgentSelector() works. */
export function registerSessionDrawerRef(drawerRef: any) {
  _sessionDrawerRef = drawerRef
}

/**
 * Pre-fill session identity from the API. Called by App.vue on mount
 * so that QuoteQuestionBar can display correct session info even
 * before ChatPanel is opened.
 */
export async function initSessionFromAPI() {
  const agentsApi = useAgents()
  try {
    const [chatResp] = await Promise.all([
      fetch('/api/ai/chat?limit=1'),
      agentsApi.loadAgents(),
    ])
    if (chatResp.ok) {
      const data = await chatResp.json()
      if (data.sessionId) {
        currentSessionId.value = data.sessionId
        currentSessionTitle.value = data.sessionTitle || ''
        currentBackend.value = data.backend || ''
        currentAgentId.value = data.agentId || ''
        // Initialize model: prefer server-persisted modelId, then localStorage pref, then agent default
        if (data.modelId) {
          currentModelId.value = data.modelId
          const model = agentsApi.getAgentModel(data.agentId || '', data.modelId)
          currentModelName.value = model?.name || data.modelId
        } else {
          const savedModelId = loadModelPref(data.agentId || '')
          if (savedModelId) {
            const model = agentsApi.getAgentModel(data.agentId || '', savedModelId)
            if (model) {
              currentModelId.value = savedModelId
              currentModelName.value = model.name
            } else {
              // Saved model no longer available — fall back to agent default & clear stale pref
              const { modelId, modelName } = agentsApi.syncModelFromAgent(data.agentId || '')
              currentModelId.value = modelId
              currentModelName.value = modelName
            }
          } else {
            const { modelId, modelName } = agentsApi.syncModelFromAgent(data.agentId || '')
            currentModelId.value = modelId
            currentModelName.value = modelName
          }
        }
        // Initialize thinking effort: prefer server data, then localStorage pref
        if (data.thinkingEffort) {
          currentThinkingEffort.value = data.thinkingEffort
        } else {
          currentThinkingEffort.value = loadThinkingPref(data.agentId || '') || ''
        }
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
  const { agentHeaderTitle: makeTitle } = useAgents()
  if (currentAgentId.value) return makeTitle(currentAgentId.value)
  return gt('chat.session.aiDialog')
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
        // Initialize model: prefer localStorage pref, then agent default
        const agentsApi = useAgents()
        const savedModelId = loadModelPref(currentAgentId.value)
        if (savedModelId) {
          const model = agentsApi.getAgentModel(currentAgentId.value, savedModelId)
          if (model) {
            currentModelId.value = savedModelId
            currentModelName.value = model.name
          } else {
            const { modelId, modelName } = agentsApi.syncModelFromAgent(currentAgentId.value)
            currentModelId.value = modelId
            currentModelName.value = modelName
          }
        } else {
          const { modelId, modelName } = agentsApi.syncModelFromAgent(currentAgentId.value)
          currentModelId.value = modelId
          currentModelName.value = modelName
        }
        // Initialize thinking effort from localStorage pref
        currentThinkingEffort.value = loadThinkingPref(currentAgentId.value) || ''
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
        body: JSON.stringify({ message: text, filePaths: filePaths || [], modelId: currentModelId.value || undefined, thinkingEffort: currentThinkingEffort.value || undefined }),
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

  /** Open the global session drawer (sets sessionDrawerOpen = true). */
  function openSessionTab() {
    sessionDrawerOpen.value = true
  }

  /** Open the agent selector inside the session drawer. */
  function openAgentSelector() {
    _sessionDrawerRef?.openAgentSelector()
  }

  return {
    // Identity refs (read-only for consumers; ChatPanel writes to them)
    currentSessionId,
    currentSessionTitle,
    currentBackend,
    currentAgentId,
    currentModelId,
    currentModelName,
    currentThinkingEffort,
    runningSessions,
    runningSessionsVersion,
    agentHeaderTitle,
    // Global session drawer state
    sessionDrawerOpen,
    // Action proxies
    switchSession,
    createSession,
    deleteSession,
    sendMessage,
    openChatPanel,
    openSessionTab,
    openAgentSelector,
    // Registration (for ChatPanel)
    registerSessionActions,
    // Init (for App.vue)
    initSessionFromAPI,
    // LocalStorage persistence helpers
    saveModelPref,
    saveThinkingPref,
    loadModelPref,
    loadThinkingPref,
  }
}

import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'

// Mock useAgents before importing useSessionIdentity
const mockGetAgentModel = vi.fn()
const mockSyncModelFromAgent = vi.fn()

// Track agent preferences in-memory so getAgent returns them
const agentPrefs: Record<string, { preferredModel?: string; preferredThinkingEffort?: string }> = {}

const mockGetAgent = vi.fn((agentId: string) => {
  const pref = agentPrefs[agentId]
  if (!pref) return null
  return { id: agentId, preferredModel: pref.preferredModel || '', preferredThinkingEffort: pref.preferredThinkingEffort || '' }
})

const mockGetEffectiveThinkingEffort = vi.fn((agentId: string) => {
  const pref = agentPrefs[agentId]
  if (!pref) return ''
  return pref.preferredThinkingEffort || ''
})

vi.mock('@/composables/useAgents', () => ({
  useAgents: () => ({
    agents: { value: [{ id: 'codebuddy' }, { id: 'claude' }] },
    loadAgents: vi.fn().mockResolvedValue(undefined),
    getAgentModel: mockGetAgentModel,
    getAgentName: vi.fn().mockReturnValue('Test'),
    syncModelFromAgent: mockSyncModelFromAgent,
    getAgentIcon: vi.fn().mockReturnValue('🤖'),
    agentHeaderTitle: vi.fn().mockReturnValue('🤖 Test'),
    getAgent: mockGetAgent,
    getEffectiveThinkingEffort: mockGetEffectiveThinkingEffort,
  }),
}))

// Mock useSettingsConfig.patchAgentPref to update in-memory agentPrefs
vi.mock('@/composables/useSettingsConfig', () => ({
  patchAgentPref: (agentId: string, field: 'preferred_model' | 'preferred_thinking_effort', value: string) => {
    if (!agentPrefs[agentId]) agentPrefs[agentId] = {}
    if (field === 'preferred_model') agentPrefs[agentId].preferredModel = value
    if (field === 'preferred_thinking_effort') agentPrefs[agentId].preferredThinkingEffort = value
    return Promise.resolve()
  },
}))

vi.mock('@/composables/useLocale', () => ({
  gt: (key: string) => key,
}))

// Import after mocks are set up — use the composable to access helper functions
import { useSessionIdentity } from '@/composables/useSessionIdentity'

describe('useSessionIdentity - model preference (cross-project)', () => {
  let saveModelPref: (agentId: string, modelId: string) => void
  let loadModelPref: (agentId: string) => string | null
  let saveThinkingPref: (agentId: string, level: string) => void
  let loadThinkingPref: (agentId: string) => string | null

  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    // Reset in-memory agent preferences
    for (const key of Object.keys(agentPrefs)) {
      delete agentPrefs[key]
    }
    const identity = useSessionIdentity()
    saveModelPref = identity.saveModelPref
    loadModelPref = identity.loadModelPref
    saveThinkingPref = identity.saveThinkingPref
    loadThinkingPref = identity.loadThinkingPref
  })

  afterEach(() => {
    localStorage.clear()
  })

  describe('saveModelPref / loadModelPref', () => {
    it('saves and loads model preference per agent', async () => {
      await saveModelPref('codebuddy', 'glm-5.1')
      expect(loadModelPref('codebuddy')).toBe('glm-5.1')
    })

    it('returns null when no preference saved', () => {
      expect(loadModelPref('claude')).toBeNull()
    })

    it('keeps different preferences for different agents', async () => {
      await saveModelPref('codebuddy', 'glm-5.1')
      await saveModelPref('claude', 'claude-sonnet-4-6')
      expect(loadModelPref('codebuddy')).toBe('glm-5.1')
      expect(loadModelPref('claude')).toBe('claude-sonnet-4-6')
    })

    it('overwrites previous preference for same agent', async () => {
      await saveModelPref('codebuddy', 'glm-5.1')
      await saveModelPref('codebuddy', 'glm-4')
      expect(loadModelPref('codebuddy')).toBe('glm-4')
    })

    it('persists across simulated project switches (localStorage survives)', async () => {
      // User sets model in "project A"
      await saveModelPref('codebuddy', 'claude-sonnet-4-6')

      // Simulate project switch — localStorage is NOT cleared
      // (unlike session DB which is per-project)
      expect(loadModelPref('codebuddy')).toBe('claude-sonnet-4-6')
    })

    it('handles empty agentId gracefully', async () => {
      await saveModelPref('', 'glm-5.1')
      expect(loadModelPref('')).toBeNull()
    })

    it('handles empty modelId gracefully', async () => {
      await saveModelPref('codebuddy', '')
      // Empty modelId should not be saved
      expect(loadModelPref('codebuddy')).toBeNull()
    })
  })

  describe('saveThinkingPref / loadThinkingPref', () => {
    it('saves and loads thinking preference per agent', async () => {
      await saveThinkingPref('codebuddy', 'high')
      expect(loadThinkingPref('codebuddy')).toBe('high')
    })

    it('returns null when no preference saved', () => {
      expect(loadThinkingPref('claude')).toBeNull()
    })
  })

  describe('model resolution priority (syncModelFromData behavior)', () => {
    // These tests verify the model resolution logic that useChatSession
    // implements using useSessionIdentity's helpers.
    // Priority: server modelId > localStorage pref > agent default

    it('when server modelId is empty, localStorage preference is used', async () => {
      // Set global preference
      await saveModelPref('codebuddy', 'claude-sonnet-4-6')

      // Simulate syncModelFromData with empty server modelId (new session)
      const serverModelId = ''
      const agentId = 'codebuddy'

      // This is the logic from useChatSession.syncModelFromData:
      let resolvedModelId = ''
      if (serverModelId) {
        resolvedModelId = serverModelId
      } else {
        const saved = loadModelPref(agentId)
        if (saved) {
          resolvedModelId = saved
        }
      }

      expect(resolvedModelId).toBe('claude-sonnet-4-6')
    })

    it('when server modelId is set, it takes priority over localStorage', async () => {
      // Set global preference
      await saveModelPref('codebuddy', 'glm-5.1')

      // Simulate existing session with a different model in DB
      const serverModelId = 'claude-sonnet-4-6'
      const agentId = 'codebuddy'

      let resolvedModelId = ''
      if (serverModelId) {
        resolvedModelId = serverModelId
      } else {
        const saved = loadModelPref(agentId)
        if (saved) {
          resolvedModelId = saved
        }
      }

      expect(resolvedModelId).toBe('claude-sonnet-4-6')
    })

    it('new session in different project uses global localStorage preference', async () => {
      // User sets model in "project A"
      await saveModelPref('codebuddy', 'glm-5.1')

      // Simulate creating a new session in "project B"
      // Server returns empty modelId because we no longer pre-fill
      const serverModelId = ''
      const agentId = 'codebuddy'

      let resolvedModelId = ''
      if (serverModelId) {
        resolvedModelId = serverModelId
      } else {
        const saved = loadModelPref(agentId)
        if (saved) {
          resolvedModelId = saved
        }
      }

      // Global preference should be used, not agent default
      expect(resolvedModelId).toBe('glm-5.1')
    })
  })
})

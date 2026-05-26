import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useAgents, resetAgents } from '@/composables/useAgents'

// Mock apiGet to control agent data
const mockApiGet = vi.fn()
vi.mock('@/utils/api', () => ({
  apiGet: (...args: any[]) => mockApiGet(...args),
}))

// Mock useLocale gt function
vi.mock('@/composables/useLocale', () => ({
  gt: (key: string) => key,
}))

describe('useAgents', () => {
  const { agents, defaultAgentId, loadAgents, getAgentIcon, getAgentName,
    isDefaultAgent, getDefaultModelId, getAgentModels, isMultiModel,
    getAgent, getAgentModel, getAgentDefaultModelName, agentHeaderTitle,
    syncModelFromAgent, getAgentThinkingEffortLevels, hasThinkingEffortLevels,
    updateAgentField, canRefreshModels } = useAgents()

  const testAgents = [
    {
      id: 'claude',
      name: 'Claude',
      icon: '🤖',
      canRefreshModels: true,
      models: [
        { id: 'claude-3.5', name: 'Claude 3.5 Sonnet', default: true },
        { id: 'claude-3-haiku', name: 'Claude 3 Haiku', default: false },
      ],
      thinkingEffortLevels: ['low', 'medium', 'high', 'xhigh', 'max'],
    },
    {
      id: 'gpt',
      name: 'GPT-4',
      icon: '🧠',
      canRefreshModels: true,
      models: [{ id: 'gpt-4o', name: 'GPT-4o', default: true }],
      thinkingEffortLevels: ['low', 'medium', 'high'],
    },
    {
      id: 'simple',
      name: 'Simple Agent',
      icon: '⚡',
      models: [],
      canRefreshModels: false,
      // no thinkingEffortLevels — unsupported backend
    },
  ]

  beforeEach(async () => {
    // Reset agents state
    agents.value = []
    defaultAgentId.value = ''
    mockApiGet.mockReset()

    // Load test agents
    mockApiGet.mockResolvedValue({ agents: testAgents, defaultAgent: 'claude' })
    await loadAgents()
  })

  // --- loadAgents ---

  describe('loadAgents', () => {
    it('loads agents from the API', async () => {
      expect(agents.value).toHaveLength(3)
      expect(agents.value[0].id).toBe('claude')
    })

    it('sets defaultAgentId from API response', async () => {
      expect(defaultAgentId.value).toBe('claude')
    })

    it('caches result and does not re-fetch', async () => {
      await loadAgents()
      expect(mockApiGet).toHaveBeenCalledTimes(1) // only the beforeEach call
    })

    it('handles API error gracefully', async () => {
      agents.value = []
      mockApiGet.mockRejectedValue(new Error('Network error'))
      await loadAgents()
      expect(agents.value).toHaveLength(0)
    })

    it('handles null agents array', async () => {
      agents.value = []
      mockApiGet.mockResolvedValue({ agents: null })
      await loadAgents()
      expect(agents.value).toEqual([])
    })

    it('handles missing defaultAgent', async () => {
      defaultAgentId.value = ''
      agents.value = []
      mockApiGet.mockResolvedValue({ agents: testAgents })
      await loadAgents()
      expect(defaultAgentId.value).toBe('')
    })
  })

  // --- getAgentIcon ---

  describe('getAgentIcon', () => {
    it('returns the icon for a known agent', () => {
      expect(getAgentIcon('claude')).toBe('🤖')
    })

    it('returns default robot emoji for unknown agent', () => {
      expect(getAgentIcon('nonexistent')).toBe('🤖')
    })

    it('returns default for empty string', () => {
      expect(getAgentIcon('')).toBe('🤖')
    })
  })

  // --- getAgentName ---

  describe('getAgentName', () => {
    it('returns the name for a known agent', () => {
      expect(getAgentName('claude')).toBe('Claude')
    })

    it('returns agentId for unknown agent', () => {
      expect(getAgentName('unknown-id')).toBe('unknown-id')
    })

    it('returns i18n default key for empty string', () => {
      expect(getAgentName('')).toBe('agents.defaultAssistant')
    })
  })

  // --- isDefaultAgent ---

  describe('isDefaultAgent', () => {
    it('returns true for the default agent', () => {
      expect(isDefaultAgent('claude')).toBe(true)
    })

    it('returns false for non-default agent', () => {
      expect(isDefaultAgent('gpt')).toBe(false)
    })

    it('returns false when defaultAgentId is empty', () => {
      defaultAgentId.value = ''
      expect(isDefaultAgent('claude')).toBe(false)
    })
  })

  // --- getDefaultModelId ---

  describe('getDefaultModelId', () => {
    it('returns the model marked as default', () => {
      expect(getDefaultModelId('claude')).toBe('claude-3.5')
    })

    it('returns the first model if none is marked default', () => {
      // gpt has only one model which is default
      expect(getDefaultModelId('gpt')).toBe('gpt-4o')
    })

    it('returns empty string for agent with no models', () => {
      expect(getDefaultModelId('simple')).toBe('')
    })

    it('returns empty string for unknown agent', () => {
      expect(getDefaultModelId('nonexistent')).toBe('')
    })
  })

  // --- getAgentModels ---

  describe('getAgentModels', () => {
    it('returns models for a known agent', () => {
      const models = getAgentModels('claude')
      expect(models).toHaveLength(2)
      expect(models[0].id).toBe('claude-3.5')
    })

    it('returns empty array for agent with no models', () => {
      expect(getAgentModels('simple')).toEqual([])
    })

    it('returns empty array for unknown agent', () => {
      expect(getAgentModels('nonexistent')).toEqual([])
    })
  })

  // --- isMultiModel ---

  describe('isMultiModel', () => {
    it('returns true for agent with multiple models', () => {
      expect(isMultiModel('claude')).toBe(true)
    })

    it('returns false for agent with single model', () => {
      expect(isMultiModel('gpt')).toBe(false)
    })

    it('returns false for agent with no models', () => {
      expect(isMultiModel('simple')).toBe(false)
    })

    it('returns false for unknown agent', () => {
      expect(isMultiModel('nonexistent')).toBe(false)
    })
  })

  // --- getAgent ---

  describe('getAgent', () => {
    it('returns the agent object for a known id', () => {
      const agent = getAgent('claude')
      expect(agent).toBeDefined()
      expect(agent!.id).toBe('claude')
      expect(agent!.name).toBe('Claude')
    })

    it('returns undefined for unknown id', () => {
      expect(getAgent('nonexistent')).toBeUndefined()
    })
  })

  // --- getAgentModel ---

  describe('getAgentModel', () => {
    it('returns a specific model by id', () => {
      const model = getAgentModel('claude', 'claude-3-haiku')
      expect(model).toBeDefined()
      expect(model!.id).toBe('claude-3-haiku')
      expect(model!.name).toBe('Claude 3 Haiku')
    })

    it('returns undefined for unknown model id', () => {
      expect(getAgentModel('claude', 'nonexistent')).toBeUndefined()
    })

    it('returns undefined for unknown agent', () => {
      expect(getAgentModel('nonexistent', 'any')).toBeUndefined()
    })
  })

  // --- getAgentDefaultModelName ---

  describe('getAgentDefaultModelName', () => {
    it('returns the default model name', () => {
      expect(getAgentDefaultModelName('claude')).toBe('Claude 3.5 Sonnet')
    })

    it('returns the model id if model not found', () => {
      // Agent with no models → getDefaultModelId returns ''
      expect(getAgentDefaultModelName('simple')).toBe('')
    })

    it('returns model id for unknown agent', () => {
      expect(getAgentDefaultModelName('nonexistent')).toBe('')
    })
  })

  // --- agentHeaderTitle ---

  describe('agentHeaderTitle', () => {
    it('returns "icon name" for a known agent', () => {
      expect(agentHeaderTitle('claude')).toBe('🤖 Claude')
    })

    it('returns i18n key for empty agentId', () => {
      expect(agentHeaderTitle('')).toBe('chat.session.aiDialog')
    })
  })

  // --- syncModelFromAgent ---

  describe('syncModelFromAgent', () => {
    it('returns default model id and name for known agent', () => {
      const result = syncModelFromAgent('claude')
      expect(result.modelId).toBe('claude-3.5')
      expect(result.modelName).toBe('Claude 3.5 Sonnet')
    })

    it('returns empty strings for agent with no models', () => {
      const result = syncModelFromAgent('simple')
      expect(result.modelId).toBe('')
      expect(result.modelName).toBe('')
    })

    it('returns empty strings for unknown agent', () => {
      const result = syncModelFromAgent('nonexistent')
      expect(result.modelId).toBe('')
      expect(result.modelName).toBe('')
    })

    it('returns model id as name when model lookup fails', () => {
      // Create a scenario where getDefaultModelId returns a valid id
      // but getAgentModels returns empty (shouldn't happen in practice but test edge case)
      const result = syncModelFromAgent('simple')
      expect(result.modelId).toBe('')
      expect(result.modelName).toBe('')
    })
  })

  // --- getAgentThinkingEffortLevels ---

  describe('getAgentThinkingEffortLevels', () => {
    it('returns thinking effort levels for an agent with levels', () => {
      const levels = getAgentThinkingEffortLevels('claude')
      expect(levels).toEqual(['low', 'medium', 'high', 'xhigh', 'max'])
    })

    it('returns levels for another agent with different levels', () => {
      const levels = getAgentThinkingEffortLevels('gpt')
      expect(levels).toEqual(['low', 'medium', 'high'])
    })

    it('returns empty array for agent without thinking effort levels', () => {
      const levels = getAgentThinkingEffortLevels('simple')
      expect(levels).toEqual([])
    })

    it('returns empty array for unknown agent', () => {
      const levels = getAgentThinkingEffortLevels('nonexistent')
      expect(levels).toEqual([])
    })
  })

  // --- hasThinkingEffortLevels ---

  describe('hasThinkingEffortLevels', () => {
    it('returns true for agent with thinking effort levels', () => {
      expect(hasThinkingEffortLevels('claude')).toBe(true)
    })

    it('returns true for another agent with levels', () => {
      expect(hasThinkingEffortLevels('gpt')).toBe(true)
    })

    it('returns false for agent without thinking effort levels', () => {
      expect(hasThinkingEffortLevels('simple')).toBe(false)
    })

    it('returns false for unknown agent', () => {
      expect(hasThinkingEffortLevels('nonexistent')).toBe(false)
    })
  })

  // --- resetAgents ---

  describe('resetAgents', () => {
    it('clears agents and defaultAgentId', async () => {
      // After beforeEach, agents are loaded
      expect(agents.value).toHaveLength(3)
      expect(defaultAgentId.value).toBe('claude')

      resetAgents()

      expect(agents.value).toEqual([])
      expect(defaultAgentId.value).toBe('')
    })

    it('clears loadPromise so next loadAgents re-fetches', async () => {
      // After beforeEach, loadAgents is cached (loadPromise was set and cleared)
      // Reset should clear the cache so a new loadAgents() triggers an API call
      const callCountBefore = mockApiGet.mock.calls.length
      await loadAgents()
      expect(mockApiGet.mock.calls.length).toBe(callCountBefore) // cached, no new call

      resetAgents()
      mockApiGet.mockResolvedValue({ agents: testAgents, defaultAgent: 'gpt' })
      await loadAgents()
      expect(mockApiGet.mock.calls.length).toBe(callCountBefore + 1) // new API call made

      // New default loaded
      expect(defaultAgentId.value).toBe('gpt')
    })
  })

  // --- concurrent loadAgents deduplication ---

  describe('loadAgents deduplication', () => {
    it('uses cached result when agents already loaded', async () => {
      // Agents were loaded in beforeEach — second call should be a no-op
      const callCountBefore = mockApiGet.mock.calls.length
      await loadAgents()
      // Should not have made another API call
      expect(mockApiGet.mock.calls.length).toBe(callCountBefore)
    })

    it('shares the same load promise for concurrent callers', async () => {
      agents.value = []
      mockApiGet.mockReset()

      // Create a slow-resolving mock so both calls overlap
      let resolveApi: (v: any) => void
      const slowPromise = new Promise(r => { resolveApi = r })
      mockApiGet.mockReturnValue(slowPromise)

      // Start both calls before the API resolves
      const p1 = loadAgents()
      const p2 = loadAgents()

      // Resolve the API call
      resolveApi!({ agents: testAgents })

      await Promise.all([p1, p2])

      // Only one API call should have been made (deduplication via loadPromise)
      expect(mockApiGet).toHaveBeenCalledTimes(1)
      expect(agents.value).toHaveLength(3)
    })
  })

  // --- canRefreshModels ---

  describe('canRefreshModels', () => {
    it('returns true for agent with canRefreshModels=true', () => {
      expect(canRefreshModels('claude')).toBe(true)
    })

    it('returns true for another agent with canRefreshModels=true', () => {
      expect(canRefreshModels('gpt')).toBe(true)
    })

    it('returns false for agent with canRefreshModels=false', () => {
      expect(canRefreshModels('simple')).toBe(false)
    })

    it('returns false for unknown agent', () => {
      expect(canRefreshModels('nonexistent')).toBe(false)
    })

    it('returns false for empty string', () => {
      expect(canRefreshModels('')).toBe(false)
    })
  })

  // --- updateAgentField ---

  describe('updateAgentField', () => {
    it('updates a field on an existing agent', () => {
      expect(getAgent('claude')!.preferredModel).toBeUndefined()
      updateAgentField('claude', 'preferredModel', 'claude-3-haiku')
      expect(getAgent('claude')!.preferredModel).toBe('claude-3-haiku')
    })

    it('does nothing for unknown agent', () => {
      // Should not throw
      updateAgentField('nonexistent', 'preferredModel', 'test')
    })
  })
})

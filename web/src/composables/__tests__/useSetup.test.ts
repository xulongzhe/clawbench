import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { useSetup, providerAgentNames, recommendedProviders } from '@/composables/useSetup'

// Mock the api module
vi.mock('@/utils/api', () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
}))

import { apiGet, apiPost } from '@/utils/api'

describe('useSetup', () => {
  beforeEach(() => {
    vi.mocked(apiGet).mockReset()
    vi.mocked(apiPost).mockReset()
  })

  it('returns all expected properties and methods', () => {
    const setup = useSetup()
    expect(setup.status).toBeDefined()
    expect(setup.providers).toBeDefined()
    expect(setup.models).toBeDefined()
    expect(setup.summarizeModelHint).toBeDefined()
    expect(setup.modelsError).toBeDefined()
    expect(setup.loading).toBeDefined()
    expect(setup.completed).toBeDefined()
    expect(typeof setup.checkStatus).toBe('function')
    expect(typeof setup.getProviders).toBe('function')
    expect(typeof setup.scanModels).toBe('function')
    expect(typeof setup.verify).toBe('function')
    expect(typeof setup.complete).toBe('function')
    expect(typeof setup.saveWizardState).toBe('function')
    expect(typeof setup.loadWizardState).toBe('function')
    expect(typeof setup.clearWizardState).toBe('function')
  })

  describe('checkStatus', () => {
    it('calls GET /api/setup/status and returns data', async () => {
      vi.mocked(apiGet).mockResolvedValueOnce({
        needs_setup: true,
        embedded_agent: true,
        agent_version: '0.78.0',
      })
      const setup = useSetup()
      const result = await setup.checkStatus()
      expect(apiGet).toHaveBeenCalledWith('/api/setup/status')
      expect(result.needs_setup).toBe(true)
      expect(result.embedded_agent).toBe(true)
      expect(result.agent_version).toBe('0.78.0')
    })
  })

  describe('getProviders', () => {
    it('calls GET /api/setup/providers and returns provider list', async () => {
      vi.mocked(apiGet).mockResolvedValueOnce({
        providers: [
          { id: 'openai', name: 'OpenAI', envVar: 'OPENAI_API_KEY' },
          { id: 'anthropic', name: 'Anthropic', envVar: 'ANTHROPIC_API_KEY' },
        ],
        custom_url_supported: true,
      })
      const setup = useSetup()
      const result = await setup.getProviders()
      expect(apiGet).toHaveBeenCalledWith('/api/setup/providers')
      expect(result).toHaveLength(2)
      expect(result[0].id).toBe('openai')
    })
  })

  describe('scanModels', () => {
    it('calls POST /api/setup/models and returns models', async () => {
      vi.mocked(apiPost).mockResolvedValueOnce({
        models: [
          { id: 'gpt-4o', name: 'gpt-4o', created: 1700000000 },
          { id: 'gpt-4o-mini', name: 'gpt-4o-mini', created: 1699999999 },
        ],
        summarize_model_hint: 'gpt-4o-mini',
      })
      const setup = useSetup()
      const result = await setup.scanModels('openai', '', 'sk-test')
      expect(apiPost).toHaveBeenCalledWith('/api/setup/models', {
        provider: 'openai',
        custom_url: '',
        api_key: 'sk-test',
      }, expect.any(Object))
      expect(result.models).toHaveLength(2)
      expect(result.summarize_model_hint).toBe('gpt-4o-mini')
    })

    it('handles errors gracefully', async () => {
      vi.mocked(apiPost).mockRejectedValueOnce(new Error('Network error'))
      const setup = useSetup()
      const result = await setup.scanModels('openai', '', 'sk-test')
      expect(result.models).toHaveLength(0)
      expect(result.error).toBe('Network error')
    })
  })

  describe('verify', () => {
    it('calls POST /api/setup/verify and returns result', async () => {
      vi.mocked(apiPost).mockResolvedValueOnce({
        success: true,
        message: '配置验证成功！',
        model: 'gpt-4o',
      })
      const setup = useSetup()
      const result = await setup.verify('openai', '', 'sk-test', 'gpt-4o')
      expect(apiPost).toHaveBeenCalledWith('/api/setup/verify', {
        provider: 'openai',
        custom_url: '',
        api_key: 'sk-test',
        model: 'gpt-4o',
      }, expect.any(Object))
      expect(result.success).toBe(true)
    })

    it('handles verify failure gracefully', async () => {
      vi.mocked(apiPost).mockRejectedValueOnce(new Error('Timeout'))
      const setup = useSetup()
      const result = await setup.verify('openai', '', 'sk-test', 'gpt-4o')
      expect(result.success).toBe(false)
      expect(result.message).toBe('Timeout')
    })
  })

  describe('complete', () => {
    it('calls POST /api/setup/complete and returns result', async () => {
      vi.mocked(apiPost).mockResolvedValueOnce({
        success: true,
        agent: { id: 'openai', name: 'OpenAI' },
        default_agent_id: 'openai',
      })
      const setup = useSetup()
      const result = await setup.complete({
        provider: 'openai',
        custom_url: '',
        api_key: 'sk-test',
        model: 'gpt-4o',
        summarize_model: 'gpt-4o-mini',
        agent_name: 'OpenAI',
        agent_id: 'openai',
      })
      expect(apiPost).toHaveBeenCalledWith('/api/setup/complete', {
        provider: 'openai',
        custom_url: '',
        api_key: 'sk-test',
        model: 'gpt-4o',
        summarize_model: 'gpt-4o-mini',
        agent_name: 'OpenAI',
        agent_id: 'openai',
      })
      expect(result.success).toBe(true)
    })

    it('handles complete failure gracefully', async () => {
      vi.mocked(apiPost).mockRejectedValueOnce(new Error('Server error'))
      const setup = useSetup()
      const result = await setup.complete({
        provider: 'openai',
        custom_url: '',
        api_key: 'sk-test',
        model: 'gpt-4o',
        summarize_model: 'gpt-4o-mini',
        agent_name: 'OpenAI',
        agent_id: 'openai',
      })
      expect(result.success).toBe(false)
    })
  })
})

describe('providerAgentNames', () => {
  it('contains entries for major providers', () => {
    expect(providerAgentNames['openai']).toBeDefined()
    expect(providerAgentNames['anthropic']).toBeDefined()
    expect(providerAgentNames['google']).toBeDefined()
    expect(providerAgentNames['deepseek']).toBeDefined()
  })

  it('each entry has name and id', () => {
    for (const [, value] of Object.entries(providerAgentNames)) {
      expect(value.name).toBeTruthy()
      expect(value.id).toBeTruthy()
      // ID should be lowercase with hyphens only
      expect(value.id).toMatch(/^[a-z0-9-]+$/)
    }
  })

  it('includes _custom entry', () => {
    expect(providerAgentNames['_custom']).toBeDefined()
    expect(providerAgentNames['_custom'].id).toBe('custom-agent')
  })
})

describe('recommendedProviders', () => {
  it('contains the big three providers', () => {
    expect(recommendedProviders).toContain('openai')
    expect(recommendedProviders).toContain('anthropic')
    expect(recommendedProviders).toContain('google')
  })

  it('has exactly 3 entries', () => {
    expect(recommendedProviders).toHaveLength(3)
  })
})

describe('session storage persistence', () => {
  afterEach(() => {
    sessionStorage.clear()
  })

  it('saveWizardState and loadWizardState round-trip', () => {
    const setup = useSetup()
    const state = {
      step: 3,
      provider: 'openai',
      customUrl: '',
      chatModel: 'gpt-4o',
      summarizeModel: 'gpt-4o-mini',
      agentName: 'OpenAI',
      agentId: 'openai',
    }
    setup.saveWizardState(state)
    const loaded = setup.loadWizardState()
    expect(loaded).toEqual(state)
  })

  it('loadWizardState returns null when no state saved', () => {
    const setup = useSetup()
    const loaded = setup.loadWizardState()
    expect(loaded).toBeNull()
  })

  it('clearWizardState removes saved state', () => {
    const setup = useSetup()
    setup.saveWizardState({
      step: 1,
      provider: '',
      customUrl: '',
      chatModel: '',
      summarizeModel: '',
      agentName: '',
      agentId: '',
    })
    setup.clearWizardState()
    const loaded = setup.loadWizardState()
    expect(loaded).toBeNull()
  })
})

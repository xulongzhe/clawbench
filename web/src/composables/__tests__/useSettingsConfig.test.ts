import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useSettingsConfig } from '@/composables/useSettingsConfig'

// Mock api.ts
vi.mock('@/utils/api', () => ({
  apiGet: vi.fn(),
  apiPatch: vi.fn(),
  apiPost: vi.fn(),
}))

// Mock useAgents
const mockGetAgent = vi.fn().mockReturnValue(null)
const mockUpdateAgentField = vi.fn()
vi.mock('@/composables/useAgents', () => ({
  useAgents: () => ({
    getAgent: mockGetAgent,
    updateAgentField: mockUpdateAgentField,
  }),
}))

import { apiGet, apiPatch, apiPost } from '@/utils/api'

const mockedApiGet = vi.mocked(apiGet)
const mockedApiPatch = vi.mocked(apiPatch)
const mockedApiPost = vi.mocked(apiPost)

describe('useSettingsConfig', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads config from API', async () => {
    const mockConfig = {
      server: { port: 20000, log_level: 'info' },
      ssh: { enabled: true, port: 2222 },
    }
    mockedApiGet.mockResolvedValue(mockConfig)

    const { loadConfig, serverConfig } = useSettingsConfig()
    await loadConfig()

    expect(mockedApiGet).toHaveBeenCalledWith('/api/config')
    expect(serverConfig.value).toEqual(mockConfig)
  })

  it('patchConfig calls API and returns restart info', async () => {
    const mockResult = { needsRestart: true, changedColdFields: ['ssh.enabled'] }
    mockedApiPatch.mockResolvedValue(mockResult)

    const { patchConfig } = useSettingsConfig()
    const result = await patchConfig({ ssh: { enabled: false } })

    expect(mockedApiPatch).toHaveBeenCalledWith('/api/config', { ssh: { enabled: false } })
    expect(result.needsRestart).toBe(true)
    expect(result.changedColdFields).toEqual(['ssh.enabled'])
  })

  it('restartServer calls API', async () => {
    mockedApiPost.mockResolvedValue({})

    const { restartServer } = useSettingsConfig()
    await restartServer()

    expect(mockedApiPost).toHaveBeenCalledWith('/api/config/restart', {})
  })

  it('setLocalConfig writes to localStorage and updates reactive', () => {
    const { localConfig, setLocalConfig } = useSettingsConfig()

    setLocalConfig('theme', 'dark')

    expect(localConfig.theme).toBe('dark')
    expect(localStorage.getItem('clawbench-settings-theme')).toBe('"dark"')

    // Clean up
    localStorage.removeItem('clawbench-settings-theme')
  })

  it('getServerValue reads by dot-path', async () => {
    mockedApiGet.mockResolvedValue({ server: { port: 20000 } })

    const { loadConfig, getServerValue } = useSettingsConfig()
    await loadConfig()

    expect(getServerValue('server.port')).toBe(20000)
    expect(getServerValue('server.log_level')).toBeUndefined()
    expect(getServerValue('nonexistent')).toBeUndefined()
  })

  it('localConfig has default keys', () => {
    const { localConfig } = useSettingsConfig()

    // Verify keys exist (values may be overridden by localStorage from other tests)
    expect('theme' in localConfig).toBe(true)
    expect('locale' in localConfig).toBe(true)
    expect('autoSpeech' in localConfig).toBe(true)
    expect('wordWrap' in localConfig).toBe(true)
    expect('lineNumbers' in localConfig).toBe(true)
    expect('showHidden' in localConfig).toBe(true)
    expect('fileView' in localConfig).toBe(true)
    expect('terminalFontSize' in localConfig).toBe(true)
    expect('androidLogCapture' in localConfig).toBe(true)
  })

  it('reads persisted localStorage value via setLocalConfig', () => {
    const { localConfig, setLocalConfig } = useSettingsConfig()

    setLocalConfig('showHidden', true)
    expect(localConfig.showHidden).toBe(true)
    expect(localStorage.getItem('clawbench-settings-showHidden')).toBe('true')

    // Clean up
    localStorage.removeItem('clawbench-settings-showHidden')
  })

  describe('agent preference helpers', () => {
    it('reads agent model preference from agent data', () => {
      const { getAgentModelPref } = useSettingsConfig()

      // No agent data → null
      mockGetAgent.mockReturnValue(null)
      expect(getAgentModelPref('test-agent')).toBeNull()

      // Agent with preferredModel set
      mockGetAgent.mockReturnValue({ id: 'test-agent', preferredModel: 'model-1' })
      expect(getAgentModelPref('test-agent')).toBe('model-1')

      // Agent without preferredModel
      mockGetAgent.mockReturnValue({ id: 'test-agent', preferredModel: '' })
      expect(getAgentModelPref('test-agent')).toBeNull()
    })

    it('reads agent thinking preference from agent data', () => {
      const { getAgentThinkingPref } = useSettingsConfig()

      // No agent data → null
      mockGetAgent.mockReturnValue(null)
      expect(getAgentThinkingPref('test-agent')).toBeNull()

      // Agent with preferredThinkingEffort set
      mockGetAgent.mockReturnValue({ id: 'test-agent', preferredThinkingEffort: 'high' })
      expect(getAgentThinkingPref('test-agent')).toBe('high')

      // Agent without preferredThinkingEffort
      mockGetAgent.mockReturnValue({ id: 'test-agent', preferredThinkingEffort: '' })
      expect(getAgentThinkingPref('test-agent')).toBeNull()
    })

    it('patchAgentPref calls PATCH /api/agents and updates local agent data', async () => {
      const { patchAgentPref } = useSettingsConfig()
      mockedApiPatch.mockResolvedValue({})

      await patchAgentPref('test-agent', 'preferred_model', 'model-1')

      expect(mockedApiPatch).toHaveBeenCalledWith('/api/agents', { id: 'test-agent', preferred_model: 'model-1' })
      expect(mockUpdateAgentField).toHaveBeenCalledWith('test-agent', 'preferredModel', 'model-1')

      // Test preferred_thinking_effort too
      await patchAgentPref('test-agent', 'preferred_thinking_effort', 'high')

      expect(mockedApiPatch).toHaveBeenCalledWith('/api/agents', { id: 'test-agent', preferred_thinking_effort: 'high' })
      expect(mockUpdateAgentField).toHaveBeenCalledWith('test-agent', 'preferredThinkingEffort', 'high')
    })
  })
})

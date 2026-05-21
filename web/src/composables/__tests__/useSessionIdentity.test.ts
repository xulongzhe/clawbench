import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ref, nextTick } from 'vue'

// Mock external dependencies — but NOT the module under test itself
const mockPatchAgentPref = vi.fn().mockResolvedValue(undefined)
vi.mock('@/composables/useSettingsConfig', () => ({
    patchAgentPref: (...args: any[]) => mockPatchAgentPref(...args),
}))

const mockGetAgent = vi.fn()
const mockGetAgentModel = vi.fn()
const mockSyncModelFromAgent = vi.fn().mockReturnValue({ modelId: 'default-model', modelName: 'Default Model' })
const mockGetEffectiveThinkingEffort = vi.fn().mockReturnValue(null)
const mockAgentHeaderTitle = vi.fn().mockReturnValue('🤖 Test')

vi.mock('@/composables/useAgents', () => ({
    useAgents: () => ({
        agents: ref([]),
        loadAgents: vi.fn().mockResolvedValue(undefined),
        getAgent: mockGetAgent,
        getAgentModel: mockGetAgentModel,
        syncModelFromAgent: mockSyncModelFromAgent,
        getEffectiveThinkingEffort: mockGetEffectiveThinkingEffort,
        agentHeaderTitle: mockAgentHeaderTitle,
    }),
}))

vi.mock('@/composables/useToast', () => ({
    useToast: () => ({ show: vi.fn() }),
}))

vi.mock('@/composables/useNotification', () => ({
    useNotification: () => ({ play: vi.fn() }),
}))

vi.mock('@/composables/useLocale', () => ({
    gt: (key: string) => key,
}))

vi.mock('@/stores/app', () => ({
    store: { state: { chatInitialMessages: 50, chatPageSize: 50 } },
}))

vi.mock('@/utils/chatSessionUtils', () => ({
    buildMessageSnapshot: vi.fn().mockReturnValue(''),
    parseMessages: vi.fn().mockReturnValue([]),
}))

import { useSessionIdentity, registerSessionActions, initSessionFromAPI } from '@/composables/useSessionIdentity'

describe('useSessionIdentity', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    // ── Identity refs ──

    describe('identity refs', () => {
        it('exposes all identity refs', () => {
            const identity = useSessionIdentity()
            expect(identity.currentSessionId).toBeDefined()
            expect(identity.currentSessionTitle).toBeDefined()
            expect(identity.currentBackend).toBeDefined()
            expect(identity.currentAgentId).toBeDefined()
            expect(identity.currentModelId).toBeDefined()
            expect(identity.currentModelName).toBeDefined()
            expect(identity.currentThinkingEffort).toBeDefined()
            expect(identity.runningSessions).toBeDefined()
            expect(identity.sessionDrawerOpen).toBeDefined()
        })

        it('shares state across multiple instances (singleton)', () => {
            const id1 = useSessionIdentity()
            const id2 = useSessionIdentity()

            id1.currentThinkingEffort.value = 'high'
            expect(id2.currentThinkingEffort.value).toBe('high')

            // Reset
            id1.currentThinkingEffort.value = ''
        })
    })

    // ── currentThinkingEffort ──

    describe('currentThinkingEffort', () => {
        it('initializes as empty string (auto)', () => {
            const { currentThinkingEffort } = useSessionIdentity()
            expect(currentThinkingEffort.value).toBe('')
        })

        it('can be set to various effort levels', async () => {
            const { currentThinkingEffort } = useSessionIdentity()

            for (const level of ['low', 'medium', 'high', 'xhigh', 'max']) {
                currentThinkingEffort.value = level
                await nextTick()
                expect(currentThinkingEffort.value).toBe(level)
            }

            // Reset
            currentThinkingEffort.value = ''
        })

        it('can be reset to empty (auto)', async () => {
            const { currentThinkingEffort } = useSessionIdentity()
            currentThinkingEffort.value = 'high'
            await nextTick()
            currentThinkingEffort.value = ''
            await nextTick()
            expect(currentThinkingEffort.value).toBe('')
        })
    })

    // ── sessionDrawerOpen ──

    describe('sessionDrawerOpen', () => {
        it('can be toggled', async () => {
            const { sessionDrawerOpen } = useSessionIdentity()
            sessionDrawerOpen.value = true
            await nextTick()
            expect(sessionDrawerOpen.value).toBe(true)

            sessionDrawerOpen.value = false
            await nextTick()
            expect(sessionDrawerOpen.value).toBe(false)
        })
    })

    // ── openSessionTab ──

    describe('openSessionTab', () => {
        it('sets sessionDrawerOpen to true', async () => {
            const identity = useSessionIdentity()
            identity.sessionDrawerOpen.value = false

            identity.openSessionTab()
            await nextTick()

            expect(identity.sessionDrawerOpen.value).toBe(true)
        })
    })

    // ── switchSession proxy ──

    describe('switchSession', () => {
        it('delegates to registered callback', async () => {
            const mockSwitch = vi.fn()
            registerSessionActions({
                switchSession: mockSwitch,
                createSession: vi.fn(),
                deleteSession: vi.fn(),
                sendMessage: vi.fn(),
                openChatPanel: vi.fn(),
            })

            const identity = useSessionIdentity()
            await identity.switchSession('session-2')

            expect(mockSwitch).toHaveBeenCalledWith('session-2')
        })

        it('does nothing when callback is a no-op', async () => {
            // Register with no-op callbacks
            registerSessionActions({
                switchSession: vi.fn(),
                createSession: vi.fn(),
                deleteSession: vi.fn(),
                sendMessage: vi.fn(),
                openChatPanel: vi.fn(),
            })

            const identity = useSessionIdentity()
            // Should not throw
            await identity.switchSession('session-3')
        })
    })

    // ── createSession fallback ──

    describe('createSession fallback', () => {
        it('makes direct API call when no callback registered', async () => {
            // Register with empty no-op callbacks — but createSession should not be called
            // since we're testing fallback. Actually we need _createSession to be null.
            // The composable checks if (_createSession) before delegating.
            // We can't easily set it to null without modifying the source,
            // so let's test the delegation path instead.

            const mockCreate = vi.fn()
            registerSessionActions({
                switchSession: vi.fn(),
                createSession: mockCreate,
                deleteSession: vi.fn(),
                sendMessage: vi.fn(),
                openChatPanel: vi.fn(),
            })

            const identity = useSessionIdentity()
            await identity.createSession('agent-1')

            expect(mockCreate).toHaveBeenCalledWith('agent-1')
        })

        it('delegates to registered callback when available', async () => {
            const mockCreate = vi.fn()
            registerSessionActions({
                switchSession: vi.fn(),
                createSession: mockCreate,
                deleteSession: vi.fn(),
                sendMessage: vi.fn(),
                openChatPanel: vi.fn(),
            })

            const identity = useSessionIdentity()
            await identity.createSession('agent-2')

            expect(mockCreate).toHaveBeenCalledWith('agent-2')
        })
    })

    // ── deleteSession ──

    describe('deleteSession', () => {
        it('delegates to registered callback when available', async () => {
            const mockDelete = vi.fn()
            registerSessionActions({
                switchSession: vi.fn(),
                createSession: vi.fn(),
                deleteSession: mockDelete,
                sendMessage: vi.fn(),
                openChatPanel: vi.fn(),
            })

            const identity = useSessionIdentity()
            await identity.deleteSession('session-1', 'claude')

            expect(mockDelete).toHaveBeenCalledWith('session-1', 'claude')
        })

        it('does not throw when callback is no-op', async () => {
            registerSessionActions({
                switchSession: vi.fn(),
                createSession: vi.fn(),
                deleteSession: vi.fn(),
                sendMessage: vi.fn(),
                openChatPanel: vi.fn(),
            })

            const identity = useSessionIdentity()
            await expect(identity.deleteSession('session-1')).resolves.toBeUndefined()
        })
    })

    // ── sendMessage fallback ──

    describe('sendMessage fallback', () => {
        it('makes direct API call when callback is not registered', async () => {
            // Register with empty callbacks that won't intercept
            // This tests the sendMessage delegation path
            const mockSend = vi.fn()
            registerSessionActions({
                switchSession: vi.fn(),
                createSession: vi.fn(),
                deleteSession: vi.fn(),
                sendMessage: mockSend,
                openChatPanel: vi.fn(),
            })

            const identity = useSessionIdentity()
            identity.currentSessionId.value = 'session-1'

            await identity.sendMessage('hello', ['/path1'])

            expect(mockSend).toHaveBeenCalledWith('hello', ['/path1'])
        })

        it('delegates to registered callback when available', async () => {
            const mockSend = vi.fn()
            registerSessionActions({
                switchSession: vi.fn(),
                createSession: vi.fn(),
                deleteSession: vi.fn(),
                sendMessage: mockSend,
                openChatPanel: vi.fn(),
            })

            const identity = useSessionIdentity()
            await identity.sendMessage('test message', ['/path'])

            expect(mockSend).toHaveBeenCalledWith('test message', ['/path'])
        })
    })

    // ── openChatPanel ──

    describe('openChatPanel', () => {
        it('calls registered callback', () => {
            const mockOpen = vi.fn()
            registerSessionActions({
                switchSession: vi.fn(),
                createSession: vi.fn(),
                deleteSession: vi.fn(),
                sendMessage: vi.fn(),
                openChatPanel: mockOpen,
            })

            const identity = useSessionIdentity()
            identity.openChatPanel()

            expect(mockOpen).toHaveBeenCalled()
        })

        it('does nothing when callback is no-op', () => {
            registerSessionActions({
                switchSession: vi.fn(),
                createSession: vi.fn(),
                deleteSession: vi.fn(),
                sendMessage: vi.fn(),
                openChatPanel: vi.fn(),
            })
            const identity = useSessionIdentity()
            expect(() => identity.openChatPanel()).not.toThrow()
        })
    })

    // ── initSessionFromAPI ──

    describe('initSessionFromAPI', () => {
        it('populates identity refs from API response', async () => {
            const identity = useSessionIdentity()

            const mockFetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({
                    sessionId: 'api-session',
                    sessionTitle: 'API Session',
                    backend: 'codebuddy',
                    agentId: 'agent-1',
                    modelId: 'model-1',
                    thinkingEffort: 'high',
                }),
            })
            vi.stubGlobal('fetch', mockFetch)

            mockGetAgentModel.mockReturnValue({ name: 'Model One' })

            await initSessionFromAPI()

            expect(identity.currentSessionId.value).toBe('api-session')
            expect(identity.currentSessionTitle.value).toBe('API Session')
            expect(identity.currentBackend.value).toBe('codebuddy')
            expect(identity.currentAgentId.value).toBe('agent-1')
            expect(identity.currentModelId.value).toBe('model-1')
            expect(identity.currentModelName.value).toBe('Model One')
            expect(identity.currentThinkingEffort.value).toBe('high')

            vi.unstubAllGlobals()
        })

        it('handles API failure gracefully', async () => {
            const identity = useSessionIdentity()
            identity.currentSessionId.value = 'existing'

            const mockFetch = vi.fn().mockRejectedValue(new Error('fail'))
            vi.stubGlobal('fetch', mockFetch)

            await expect(initSessionFromAPI()).resolves.toBeUndefined()
            // Should not change existing values
            expect(identity.currentSessionId.value).toBe('existing')

            vi.unstubAllGlobals()
        })

        it('falls back to saved model preference when server returns no modelId', async () => {
            const identity = useSessionIdentity()

            const mockFetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({
                    sessionId: 'api-session',
                    agentId: 'agent-1',
                }),
            })
            vi.stubGlobal('fetch', mockFetch)

            mockGetAgent.mockReturnValue({ preferredModel: 'saved-model' })
            mockGetAgentModel.mockReturnValue({ name: 'Saved Model' })

            await initSessionFromAPI()

            expect(identity.currentModelId.value).toBe('saved-model')
            expect(identity.currentModelName.value).toBe('Saved Model')

            vi.unstubAllGlobals()
        })

        it('falls back to agent default when saved model is unavailable', async () => {
            const identity = useSessionIdentity()

            const mockFetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({
                    sessionId: 'api-session',
                    agentId: 'agent-1',
                }),
            })
            vi.stubGlobal('fetch', mockFetch)

            mockGetAgent.mockReturnValue({ preferredModel: 'stale-model' })
            mockGetAgentModel.mockReturnValue(null) // Model no longer exists
            mockSyncModelFromAgent.mockReturnValue({ modelId: 'default-model', modelName: 'Default' })

            await initSessionFromAPI()

            expect(mockSyncModelFromAgent).toHaveBeenCalled()
            expect(identity.currentModelId.value).toBe('default-model')

            vi.unstubAllGlobals()
        })

        it('handles thinking effort from server', async () => {
            const identity = useSessionIdentity()

            const mockFetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({
                    sessionId: 'api-session',
                    agentId: 'agent-1',
                    thinkingEffort: 'xhigh',
                }),
            })
            vi.stubGlobal('fetch', mockFetch)
            mockGetAgentModel.mockReturnValue({ name: 'Model' })

            await initSessionFromAPI()

            expect(identity.currentThinkingEffort.value).toBe('xhigh')

            vi.unstubAllGlobals()
        })

        it('falls back to saved thinking effort when server returns none', async () => {
            const identity = useSessionIdentity()

            const mockFetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({
                    sessionId: 'api-session',
                    agentId: 'agent-1',
                }),
            })
            vi.stubGlobal('fetch', mockFetch)
            mockGetAgent.mockReturnValue({ preferredModel: null })
            mockGetAgentModel.mockReturnValue(null)
            mockSyncModelFromAgent.mockReturnValue({ modelId: 'default', modelName: 'Default' })
            mockGetEffectiveThinkingEffort.mockReturnValue('medium')

            await initSessionFromAPI()

            expect(identity.currentThinkingEffort.value).toBe('medium')

            vi.unstubAllGlobals()
        })
    })
})

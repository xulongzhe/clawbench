import { describe, expect, it, vi, beforeEach } from 'vitest'

// Mock apiGet and apiPost
const mockApiPost = vi.fn()
const mockApiGet = vi.fn()
vi.mock('@/utils/api', () => ({
    apiGet: (...args: any[]) => mockApiGet(...args),
    apiPost: (...args: any[]) => mockApiPost(...args),
}))

// Mock path utils
vi.mock('@/utils/path.ts', () => ({
    baseName: (p: string) => p.split('/').pop() || '',
    dirName: (p: string) => p.split('/').slice(0, -1).join('/') || '/',
}))

// Mock useLocale
vi.mock('@/composables/useLocale', () => ({
    gt: (key: string) => key,
}))

// Mock useToast
vi.mock('@/composables/useToast', () => ({
    useToast: () => ({ show: vi.fn() }),
}))

// Mock useDialog
vi.mock('@/composables/useDialog', () => ({
    useDialog: () => ({ confirm: vi.fn().mockResolvedValue(true) }),
}))

import { store } from '@/stores/app'

describe('store', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        // Reset state to defaults before each test
        store.resetProjectState()
    })

    // ── resetProjectState ──

    describe('resetProjectState', () => {
        it('clears project fields', () => {
            store.state.projectRoot = '/some/project'
            store.state.projectName = 'project'
            store.state.watchDir = '/watch'

            store.resetProjectState()

            expect(store.state.projectRoot).toBe('')
            expect(store.state.projectName).toBe('')
            expect(store.state.watchDir).toBe('')
        })

        it('clears file browser state', () => {
            store.state.currentDir = '/some/dir'
            store.state.dirEntries = [{ name: 'file.ts', type: 'file' }] as any
            store.state.dirLoading = true
            store.state.currentFile = { name: 'file.ts', path: '/file.ts' } as any

            store.resetProjectState()

            expect(store.state.currentDir).toBe('')
            expect(store.state.dirEntries).toEqual([])
            expect(store.state.dirLoading).toBe(false)
            expect(store.state.currentFile).toBeNull()
        })

        it('clears git state', () => {
            store.state.gitBranch = 'main'
            store.state.gitHead = 'abc123'
            store.state.gitDirty = true

            store.resetProjectState()

            expect(store.state.gitBranch).toBe('')
            expect(store.state.gitHead).toBe('')
            expect(store.state.gitDirty).toBe(false)
        })

        it('clears chat/task badges', () => {
            store.state.chatUnread = true
            store.state.chatRunning = true
            store.state.taskUnread = true
            store.state.taskRunning = true
            store.state.taskJustCompleted = true
            store.state.tasks = [{ id: 'task-1' }]

            store.resetProjectState()

            expect(store.state.chatUnread).toBe(false)
            expect(store.state.chatRunning).toBe(false)
            expect(store.state.taskUnread).toBe(false)
            expect(store.state.taskRunning).toBe(false)
            expect(store.state.taskJustCompleted).toBe(false)
            expect(store.state.tasks).toEqual([])
        })

        it('resets config defaults', () => {
            store.state.uploadMaxSizeMB = 999
            store.state.uploadMaxFiles = 99
            store.state.chatInitialMessages = 999
            store.state.chatPageSize = 999
            store.state.chatSessionPageSize = 999
            store.state.chatCollapsedHeight = 999
            store.state.sessionMaxCount = 999
            store.state.recentProjectsMaxCount = 999

            store.resetProjectState()

            expect(store.state.uploadMaxSizeMB).toBe(100)
            expect(store.state.uploadMaxFiles).toBe(20)
            expect(store.state.chatInitialMessages).toBe(20)
            expect(store.state.chatPageSize).toBe(20)
            expect(store.state.chatSessionPageSize).toBe(10)
            expect(store.state.chatCollapsedHeight).toBe(150)
            expect(store.state.sessionMaxCount).toBe(10)
            expect(store.state.recentProjectsMaxCount).toBe(10)
        })
    })

    // ── loadProject ──

    describe('loadProject', () => {
        it('reads recentProjectsMaxCount from watch-dir API', async () => {
            mockApiGet.mockImplementation((url: string) => {
                if (url === '/api/watch-dir') {
                    return { watchDir: '/watch', recentProjectsMaxCount: 5 }
                }
                if (url === '/api/project') {
                    return { path: '/home/user/project' }
                }
                return {}
            })
            mockApiPost.mockResolvedValue({})

            await store.loadProject()

            expect(store.state.recentProjectsMaxCount).toBe(5)
        })

        it('does not update recentProjectsMaxCount when API returns 0 or undefined', async () => {
            store.state.recentProjectsMaxCount = 10

            mockApiGet.mockImplementation((url: string) => {
                if (url === '/api/watch-dir') {
                    return { watchDir: '/watch', recentProjectsMaxCount: 0 }
                }
                if (url === '/api/project') {
                    return { path: '/home/user/project' }
                }
                return {}
            })
            mockApiPost.mockResolvedValue({})

            await store.loadProject()

            // 0 is not > 0, so it stays at the default set by resetProjectState
            expect(store.state.recentProjectsMaxCount).toBe(10)
        })
    })

    // ── setProject ──

    describe('setProject', () => {
        it('calls API and resets project state', async () => {
            // Set some state that should be cleared
            store.state.projectRoot = '/old/project'
            store.state.gitBranch = 'old-branch'
            store.state.chatRunning = true

            mockApiPost.mockResolvedValue({ ok: 'ok', path: '/new/project' })

            const result = await store.setProject('/new/project')

            expect(mockApiPost).toHaveBeenCalledWith('/api/project', { path: '/new/project' })
            // After setProject, resetProjectState should have been called
            expect(store.state.projectRoot).toBe('')
            expect(store.state.gitBranch).toBe('')
            expect(store.state.chatRunning).toBe(false)
            // Returns the path from API response
            expect(result).toBe('/new/project')
        })

        it('returns the input path when API does not return a path', async () => {
            mockApiPost.mockResolvedValue({ ok: 'ok' })

            const result = await store.setProject('/my/project')

            expect(result).toBe('/my/project')
        })
    })
})

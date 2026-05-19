import { describe, expect, it, vi, beforeEach } from 'vitest'

// ────────────────────────────────────────────────────────────
// useTaskHistory composable tests
// Tests ISS-011 (raw fetch → apiGet/apiPut), ISS-015 (locallyReadIds
// to prevent unread flash-back), ISS-016 (AbortController on task change)
// Completion tracking: justCompletedIds, isJustCompleted()
// ────────────────────────────────────────────────────────────

// Mock i18n
vi.mock('@/i18n', () => ({
  default: {
    global: {
      locale: { value: 'en' },
      t: (key: string) => key,
    },
  },
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

// Mock API helpers
const mockApiGet = vi.fn()
const mockApiPut = vi.fn()
vi.mock('@/utils/api.ts', () => ({
  apiGet: (...args: unknown[]) => mockApiGet(...args),
  apiPut: (...args: unknown[]) => mockApiPut(...args),
}))

// Mock composables
const mockToastShow = vi.fn()
vi.mock('@/composables/useToast.ts', () => ({
  useToast: () => ({ show: mockToastShow }),
}))

const mockDialogConfirm = vi.fn()
vi.mock('@/composables/useDialog.ts', () => ({
  useDialog: () => ({ confirm: mockDialogConfirm }),
}))

const mockOpenExecDetail = vi.fn()
const mockLoadTasks = vi.fn()
vi.mock('@/composables/useTaskTab.ts', () => ({
  useTaskTab: () => ({
    openExecDetail: mockOpenExecDetail,
    loadTasks: mockLoadTasks,
  }),
}))

vi.mock('@/composables/useChatRender.ts', () => ({
  useChatRender: () => ({
    parseAssistantContent: (content: string) => ({
      blocks: [{ type: 'text', text: content }],
      metadata: null,
    }),
    formatMessageTime: () => '2m ago',
  }),
}))

// Import after mocks
import { useTaskHistory } from '@/composables/useTaskHistory.ts'
import { ref } from 'vue'

beforeEach(() => {
  mockApiGet.mockReset()
  mockApiPut.mockReset()
  mockToastShow.mockReset()
  mockDialogConfirm.mockReset()
  mockOpenExecDetail.mockReset()
})

// ── Helper ──

function createHistory(taskData: any = { id: 'task-1' }) {
  const task = ref(taskData)

  const history = useTaskHistory({ task })

  return { history, task }
}

// ── Tests ──

describe('useTaskHistory', () => {
  describe('loadExecutions', () => {
    it('calls apiGet with task id', async () => {
      const { history } = createHistory()
      mockApiGet.mockResolvedValue({ executions: [] })

      await history.loadExecutions()

      expect(mockApiGet).toHaveBeenCalledWith('/api/tasks/task-1/executions', expect.objectContaining({}))
    })

    it('populates executions with parsed data', async () => {
      const { history } = createHistory()
      mockApiGet.mockResolvedValue({
        executions: [
          { id: 'e1', content: 'Hello', createdAt: '2026-01-01', isUnread: true },
        ],
      })

      await history.loadExecutions()

      expect(history.executions.value.length).toBe(1)
      expect(history.executions.value[0].id).toBe('e1')
    })
  })

  describe('loadRunningStatus', () => {
    it('calls apiGet with task id', async () => {
      const { history } = createHistory()
      mockApiGet.mockResolvedValue({ runningExecutions: [] })

      await history.loadRunningStatus()

      expect(mockApiGet).toHaveBeenCalledWith('/api/tasks/task-1', expect.objectContaining({}))
    })
  })

  describe('cancelExecution', () => {
    it('calls apiPut with cancel action', async () => {
      const { history } = createHistory()
      mockDialogConfirm.mockResolvedValue(true)
      mockApiPut.mockResolvedValue({})

      await history.cancelExecution('exec-1')

      expect(mockApiPut).toHaveBeenCalledWith('/api/tasks/task-1', {
        action: 'cancel',
        executionId: 'exec-1',
      })
    })

    it('shows success toast on ok', async () => {
      const { history } = createHistory()
      mockDialogConfirm.mockResolvedValue(true)
      mockApiPut.mockResolvedValue({})

      await history.cancelExecution('exec-1')

      expect(mockToastShow).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({ type: 'success' }))
    })

    it('does not cancel if dialog denied', async () => {
      const { history } = createHistory()
      mockDialogConfirm.mockResolvedValue(false)

      await history.cancelExecution('exec-1')

      expect(mockApiPut).not.toHaveBeenCalled()
    })
  })

  describe('markExecRead', () => {
    it('calls apiPut with read action', async () => {
      const { history } = createHistory()
      mockApiPut.mockResolvedValue({})

      await history.markExecRead('exec-1')

      expect(mockApiPut).toHaveBeenCalledWith('/api/tasks/task-1', {
        action: 'read',
        executionId: 'exec-1',
      })
    })

    it('silently ignores errors', async () => {
      const { history } = createHistory()
      mockApiPut.mockRejectedValue(new Error('Failed'))

      // Should not throw
      await history.markExecRead('exec-1')
    })
  })

  describe('openDetail — ISS-015: locallyReadIds', () => {
    it('marks execution as locally read', async () => {
      const { history } = createHistory()
      mockApiGet.mockResolvedValue({
        executions: [{ id: 'e1', content: 'Hello', createdAt: '2026-01-01', isUnread: true }],
      })
      mockApiPut.mockResolvedValue({})

      await history.loadExecutions()

      history.openDetail(history.executions.value[0])

      expect(history.locallyReadIds.has('e1')).toBe(true)
    })

    it('does not re-mark already locally-read execution', async () => {
      const { history } = createHistory()
      mockApiGet.mockResolvedValue({
        executions: [{ id: 'e1', content: 'Hello', createdAt: '2026-01-01', isUnread: true }],
      })
      mockApiPut.mockResolvedValue({})

      await history.loadExecutions()

      history.openDetail(history.executions.value[0])
      expect(mockApiPut).toHaveBeenCalledTimes(1) // read mark

      mockApiPut.mockClear()
      history.openDetail(history.executions.value[0])
      expect(mockApiPut).not.toHaveBeenCalled() // not called again
    })

    it('isUnreadDisplay returns false after local read even if server says unread', async () => {
      const { history } = createHistory()
      mockApiGet.mockResolvedValue({
        executions: [{ id: 'e1', content: 'Hello', createdAt: '2026-01-01', isUnread: true }],
      })
      mockApiPut.mockResolvedValue({})

      await history.loadExecutions()
      history.openDetail(history.executions.value[0])

      expect(history.isUnreadDisplay(history.executions.value[0])).toBe(false)
    })
  })

  describe('AbortController — ISS-016', () => {
    it('provides a signal that changes when task ID changes', async () => {
      const { history, task } = createHistory()
      const signal1 = history.getSignal()

      // Change task ID
      task.value = { id: 'task-2' }
      history.onTaskChange()

      const signal2 = history.getSignal()
      expect(signal1).not.toBe(signal2)
    })
  })

  describe('completion tracking — justCompletedIds', () => {
    it('tracks just-completed execution IDs when running count decreases', async () => {
      const { history } = createHistory()

      // First poll: 1 running execution
      mockApiGet.mockResolvedValue({
        runningExecutions: [{ id: 'session-abc', startedAt: '2026-01-01T00:00:00Z', triggerType: 'auto' }],
      })
      await history.loadRunningStatus()
      expect(history.runningExecutions.value.length).toBe(1)

      // Second poll: 0 running — execution completed
      mockApiGet.mockResolvedValue({ runningExecutions: [] })
      // Also mock loadExecutions so the completion-triggered refresh doesn't fail
      mockApiGet.mockImplementation((url: string) => {
        if (url.includes('/executions')) return Promise.resolve({ executions: [] })
        return Promise.resolve({ runningExecutions: [] })
      })
      await history.loadRunningStatus()

      // isJustCompleted should match the session ID of the just-completed execution
      expect(history.isJustCompleted({ sessionId: 'session-abc' })).toBe(true)
      expect(history.isJustCompleted({ sessionId: 'other-session' })).toBe(false)
    })

    it('auto-clears just-completed IDs after 3 seconds', async () => {
      vi.useFakeTimers()
      const { history } = createHistory()

      // First poll: running
      mockApiGet.mockResolvedValue({
        runningExecutions: [{ id: 'session-abc', startedAt: '2026-01-01T00:00:00Z', triggerType: 'auto' }],
      })
      await history.loadRunningStatus()

      // Second poll: completed
      mockApiGet.mockImplementation((url: string) => {
        if (url.includes('/executions')) return Promise.resolve({ executions: [] })
        return Promise.resolve({ runningExecutions: [] })
      })
      await history.loadRunningStatus()

      expect(history.isJustCompleted({ sessionId: 'session-abc' })).toBe(true)

      // Advance past 3s auto-clear
      vi.advanceTimersByTime(3000)
      expect(history.isJustCompleted({ sessionId: 'session-abc' })).toBe(false)

      vi.useRealTimers()
    })

    it('clears just-completed IDs on task change', async () => {
      const { history, task } = createHistory()

      // First poll: running
      mockApiGet.mockResolvedValue({
        runningExecutions: [{ id: 'session-abc', startedAt: '2026-01-01T00:00:00Z', triggerType: 'auto' }],
      })
      await history.loadRunningStatus()

      // Second poll: completed
      mockApiGet.mockImplementation((url: string) => {
        if (url.includes('/executions')) return Promise.resolve({ executions: [] })
        return Promise.resolve({ runningExecutions: [] })
      })
      await history.loadRunningStatus()

      expect(history.isJustCompleted({ sessionId: 'session-abc' })).toBe(true)

      // Switch task
      task.value = { id: 'task-2' }
      history.onTaskChange()

      expect(history.isJustCompleted({ sessionId: 'session-abc' })).toBe(false)
    })

    it('matches using running execution id field', async () => {
      const { history } = createHistory()

      // Running execution uses 'id' field (session ID)
      mockApiGet.mockResolvedValue({
        runningExecutions: [{ id: 'session-xyz', startedAt: '2026-01-01T00:00:00Z', triggerType: 'manual' }],
      })
      await history.loadRunningStatus()

      // Completed: same session ID
      mockApiGet.mockImplementation((url: string) => {
        if (url.includes('/executions')) return Promise.resolve({ executions: [] })
        return Promise.resolve({ runningExecutions: [] })
      })
      await history.loadRunningStatus()

      // Running exec id='session-xyz' should match completed exec sessionId='session-xyz'
      expect(history.isJustCompleted({ id: 'session-xyz' })).toBe(true)
    })

    it('does not fire just-completed for running executions', async () => {
      const { history } = createHistory()

      // Still running
      mockApiGet.mockResolvedValue({
        runningExecutions: [{ id: 'session-abc', startedAt: '2026-01-01T00:00:00Z', triggerType: 'auto' }],
      })
      await history.loadRunningStatus()

      // Running execution is not "just completed"
      expect(history.isJustCompleted({ sessionId: 'session-abc', status: 'running' })).toBe(false)
    })
  })

  describe('allExecutions dedup — running records not duplicated', () => {
    it('does not duplicate running records from both data sources', async () => {
      const { history } = createHistory()

      // Simulate: DB executions API returns a running record AND completed records
      mockApiGet.mockImplementation((url: string) => {
        if (url.includes('/executions')) {
          return Promise.resolve({
            executions: [
              { id: 1, sessionId: 'session-abc', status: 'running', content: 'working...', createdAt: '2026-01-01T00:00:00Z' },
              { id: 2, sessionId: 'session-xyz', status: 'completed', content: 'done', createdAt: '2025-12-31T00:00:00Z' },
            ],
          })
        }
        // In-memory running executions also returns the same running record
        return Promise.resolve({
          runningExecutions: [{ id: 'session-abc', startedAt: '2026-01-01T00:00:00Z', triggerType: 'manual' }],
        })
      })

      await history.loadExecutions()
      await history.loadRunningStatus()

      // allExecutions should have exactly 2 entries: 1 running (from in-memory) + 1 completed (from DB)
      const all = history.allExecutions.value
      expect(all.length).toBe(2)

      // First should be the running one (from in-memory, normalized)
      expect(all[0].status).toBe('running')
      expect(all[0].id).toBe('session-abc')

      // Second should be the completed one (from DB)
      expect(all[1].status).toBe('completed')
      expect(all[1].sessionId).toBe('session-xyz')
    })

    it('shows all completed records when no running executions', async () => {
      const { history } = createHistory()

      mockApiGet.mockImplementation((url: string) => {
        if (url.includes('/executions')) {
          return Promise.resolve({
            executions: [
              { id: 1, sessionId: 's1', status: 'completed', content: 'done1', createdAt: '2026-01-01' },
              { id: 2, sessionId: 's2', status: 'completed', content: 'done2', createdAt: '2025-12-31' },
            ],
          })
        }
        return Promise.resolve({ runningExecutions: [] })
      })

      await history.loadExecutions()
      await history.loadRunningStatus()

      expect(history.allExecutions.value.length).toBe(2)
      expect(history.allExecutions.value.every(e => e.status === 'completed')).toBe(true)
    })

    it('shows only running when all DB records are running', async () => {
      const { history } = createHistory()

      mockApiGet.mockImplementation((url: string) => {
        if (url.includes('/executions')) {
          return Promise.resolve({
            executions: [
              { id: 1, sessionId: 'session-abc', status: 'running', content: '...', createdAt: '2026-01-01' },
            ],
          })
        }
        return Promise.resolve({
          runningExecutions: [{ id: 'session-abc', startedAt: '2026-01-01T00:00:00Z', triggerType: 'manual' }],
        })
      })

      await history.loadExecutions()
      await history.loadRunningStatus()

      // Only 1 entry, not 2
      expect(history.allExecutions.value.length).toBe(1)
      expect(history.allExecutions.value[0].status).toBe('running')
    })
  })
})

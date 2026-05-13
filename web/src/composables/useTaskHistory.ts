import { ref, reactive, computed, type Ref } from 'vue'
import { apiGet, apiPut } from '@/utils/api.ts'
import { useToast } from '@/composables/useToast.ts'
import { useDialog } from '@/composables/useDialog.ts'
import { useTaskTab } from '@/composables/useTaskTab.ts'
import { useChatRender } from '@/composables/useChatRender.ts'
import { gt } from '@/composables/useLocale'

interface UseTaskHistoryOptions {
  task: Ref<any>
}

export function useTaskHistory(options: UseTaskHistoryOptions) {
  const { task } = options
  const toast = useToast()
  const dialog = useDialog()
  const { openExecDetail } = useTaskTab()

  const chatRender = useChatRender({ messages: ref([]), theme: ref('light'), currentSessionId: ref('') })

  const loading = ref(false)
  const executions = ref<any[]>([])
  const runningExecutions = ref<any[]>([])

  // Unified list: running executions first (normalized to same shape), then completed
  const allExecutions = computed(() => {
    const running = runningExecutions.value.map(exec => ({
      ...exec,
      status: 'running',
      createdAt: exec.startedAt,
    }))
    return [...running, ...executions.value]
  })

  function isRunning(exec: any): boolean {
    return exec.status === 'running'
  }

  // Track previous running count to detect completions
  let prevRunningCount = 0

  // ISS-015: Track locally-read execution IDs to prevent unread flash-back
  const locallyReadIds = reactive(new Set<number>())

  // ISS-016: AbortController for cancelling in-flight requests on task change/unmount
  let abortController = new AbortController()

  function getSignal(): AbortSignal {
    return abortController.signal
  }

  /** Called when task ID changes — aborts in-flight requests and resets state */
  function onTaskChange(): void {
    abortController.abort()
    abortController = new AbortController()
    prevRunningCount = 0
  }

  function isUnreadDisplay(exec: any): boolean {
    return exec.isUnread && !locallyReadIds.has(exec.id)
  }

  async function loadExecutions(): Promise<void> {
    if (!task.value?.id) return
    loading.value = true
    try {
      const data = await apiGet<{ executions: any[] }>(
        `/api/tasks/${task.value.id}/executions`,
        { signal: abortController.signal },
      )
      const rawExecutions = data.executions || []
      executions.value = rawExecutions.map(exec => {
        const { blocks, metadata } = chatRender.parseAssistantContent(exec.content)
        const summary = extractSummary(exec)
        return { ...exec, blocks, metadata, summary }
      })
    } catch (err: any) {
      // Don't report AbortError (expected when switching tasks)
      if (err?.name !== 'AbortError') {
        console.error('Failed to load executions:', err)
      }
    } finally {
      loading.value = false
    }
  }

  async function loadRunningStatus(): Promise<void> {
    if (!task.value?.id) return
    try {
      const data = await apiGet<{ runningExecutions: any[] }>(
        `/api/tasks/${task.value.id}`,
        { signal: abortController.signal },
      )
      const newRunning = data.runningExecutions || []
      const newCount = newRunning.length
      // When running count decreases, an execution just completed — refresh the completed list
      if (prevRunningCount > 0 && newCount < prevRunningCount) {
        loadExecutions()
      }
      prevRunningCount = newCount
      runningExecutions.value = newRunning
    } catch {
      // Silently ignore — polling will retry
    }
  }

  async function cancelExecution(execId: string): Promise<void> {
    if (!task.value?.id) return
    if (!await dialog.confirm(gt('task.exec.confirmCancel'))) return
    try {
      await apiPut(`/api/tasks/${task.value.id}`, {
        action: 'cancel',
        executionId: execId,
      })
      toast.show(gt('task.exec.cancelled'), { type: 'success' })
    } catch (err: any) {
      if (err?.message?.includes('404')) {
        toast.show(gt('task.exec.alreadyFinished'), { type: 'info' })
      }
    }
    await loadRunningStatus()
  }

  async function deleteExecution(execId: number): Promise<void> {
    if (!task.value?.id) return
    if (!await dialog.confirm(gt('task.exec.confirmDeleteExecution'))) return
    try {
      await apiPut(`/api/tasks/${task.value.id}`, {
        action: 'deleteExecution',
        executionId: String(execId),
      })
      toast.show(gt('task.exec.executionDeleted'), { type: 'success' })
    } catch (err: any) {
      toast.show(gt('task.exec.actionFailed'), { type: 'error' })
      return
    }
    await loadExecutions()
  }

  async function deleteAllExecutions(): Promise<void> {
    if (!task.value?.id) return
    if (!await dialog.confirm(gt('task.exec.confirmDeleteAll'), { dangerous: true })) return
    try {
      await apiPut(`/api/tasks/${task.value.id}`, {
        action: 'deleteAllExecutions',
      })
      toast.show(gt('task.exec.allExecutionsDeleted'), { type: 'success' })
    } catch (err: any) {
      toast.show(gt('task.exec.actionFailed'), { type: 'error' })
      return
    }
    await loadExecutions()
  }

  async function markExecRead(execId: string): Promise<void> {
    if (!task.value?.id || !execId) return
    try {
      await apiPut(`/api/tasks/${task.value.id}`, {
        action: 'read',
        executionId: execId,
      })
    } catch {
      // Silently ignore read-mark failures
    }
  }

  function openDetail(exec: any): void {
    if (exec.isUnread && !locallyReadIds.has(exec.id)) {
      locallyReadIds.add(exec.id)
      markExecRead(exec.id)
    }
    openExecDetail(exec.id, exec)
  }

  function extractSummary(exec: any): string {
    const { blocks } = chatRender.parseAssistantContent(exec.content)
    for (const block of blocks) {
      if (block.type === 'text' && block.text) {
        const clean = block.text
          .replace(/<scheduled-task\s+id="[^"]+"\s*\/>/g, '')
          .replace(/[#*`_~\[\]()]/g, '')
          .trim()
        if (clean) {
          return clean.length > 120 ? clean.substring(0, 120) + '...' : clean
        }
      }
    }
    return ''
  }

  return {
    loading,
    executions,
    runningExecutions,
    allExecutions,
    isRunning,
    locallyReadIds,
    loadExecutions,
    loadRunningStatus,
    cancelExecution,
    deleteExecution,
    deleteAllExecutions,
    markExecRead,
    openDetail,
    isUnreadDisplay,
    getSignal,
    onTaskChange,
  }
}

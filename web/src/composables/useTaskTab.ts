import { ref, type Ref } from 'vue'
import { store } from '@/stores/app'

// Module-level singleton refs (shared across all consumers)
const currentView = ref<'list' | 'settings' | 'history'>('list')
const selectedTaskId = ref<number | null>(null)
const selectedExecId = ref<string | null>(null)
const selectedExecData = ref<any>(null)
const execDetailOpen = ref(false)
const formViewOpen = ref(false)
const formMode = ref<'create' | 'edit'>('create')

// Module-level polling timer
let pollingTimer: ReturnType<typeof setInterval> | null = null

// Guard: when markAllTasksRead is in progress, suppress loadTasks
// from overwriting taskUnread back to true (race condition fix)
let markingReadInProgress = false

export function useTaskTab() {
    // --- Navigation methods ---

    function navigateToTaskSettings(taskId: number) {
        selectedTaskId.value = taskId
        currentView.value = 'settings'
        execDetailOpen.value = false
        formViewOpen.value = false
    }

    function navigateToTaskHistory(taskId: number) {
        selectedTaskId.value = taskId
        currentView.value = 'history'
        execDetailOpen.value = false
        formViewOpen.value = false
    }

    function goBack() {
        if (formViewOpen.value) {
            formViewOpen.value = false
        } else if (execDetailOpen.value) {
            execDetailOpen.value = false
            selectedExecId.value = null
        } else if (currentView.value === 'history') {
            currentView.value = 'settings'
        } else {
            currentView.value = 'list'
            selectedTaskId.value = null
        }
    }

    function navigateToList() {
        formViewOpen.value = false
        execDetailOpen.value = false
        selectedExecId.value = null
        currentView.value = 'list'
        selectedTaskId.value = null
    }

    function openExecDetail(execId: string, execData?: any) {
        selectedExecId.value = execId
        selectedExecData.value = execData || null
        execDetailOpen.value = true
    }

    function closeExecDetail() {
        execDetailOpen.value = false
        selectedExecId.value = null
        selectedExecData.value = null
    }

    function openCreateForm() {
        formMode.value = 'create'
        formViewOpen.value = true
    }

    function openEditForm() {
        formMode.value = 'edit'
        formViewOpen.value = true
    }

    function closeForm() {
        formViewOpen.value = false
    }

    // --- Data methods ---

    async function loadTasks() {
        try {
            const resp = await fetch('/api/tasks')
            if (!resp.ok) return
            const data = await resp.json()
            // Race condition guard: if markAllTasksRead is in progress,
            // don't let a stale hasUnread flip taskUnread back to true
            if (!markingReadInProgress) {
                store.state.taskUnread = !!data.hasUnread
            }
            const newTasks = data.tasks || []
            // Derive running state from runningCount
            const hasRunning = newTasks.some((t: any) => t.runningCount > 0)
            store.state.taskRunning = hasRunning
            // Diff-check to avoid unnecessary watcher triggers
            if (
                store.state.tasks.length !== newTasks.length ||
                newTasks.some(
                    (t: any, i: number) =>
                        t.id !== store.state.tasks[i]?.id ||
                        t.status !== store.state.tasks[i]?.status ||
                        t.runCount !== store.state.tasks[i]?.runCount ||
                        t.unreadCount !== store.state.tasks[i]?.unreadCount ||
                        t.runningCount !== store.state.tasks[i]?.runningCount
                )
            ) {
                store.state.tasks = newTasks
            }
        } catch {
            // Silently ignore fetch errors (network down, server restart, etc.)
        }
    }

    async function markAllTasksRead() {
        const unreadTasks = store.state.tasks.filter((t: any) => t.unreadCount > 0)
        if (unreadTasks.length === 0) return
        markingReadInProgress = true
        try {
            await Promise.all(
                unreadTasks.map((t: any) =>
                    fetch(`/api/tasks/${t.id}`, {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ action: 'read' }),
                    }).then(r => {
                        if (!r.ok) throw new Error(`mark read failed: ${r.status}`)
                    })
                )
            )
            // Optimistically clear unread counts in local store
            for (const t of store.state.tasks) {
                if ((t as any).unreadCount > 0) {
                    (t as any).unreadCount = 0
                }
            }
            store.state.taskUnread = false
        } catch {
            // Mark-read failed — don't clear badge, next poll will correct
        } finally {
            markingReadInProgress = false
        }
    }

    // --- Polling ---

    function startTaskPolling() {
        if (pollingTimer !== null) return // guard against double-start
        loadTasks()
        pollingTimer = setInterval(loadTasks, 2000)
    }

    function stopTaskPolling() {
        if (pollingTimer !== null) {
            clearInterval(pollingTimer)
            pollingTimer = null
        }
    }

    return {
        // Navigation state
        currentView: currentView as Ref<'list' | 'settings' | 'history'>,
        selectedTaskId,
        selectedExecId,
        selectedExecData,
        execDetailOpen,
        formViewOpen,
        formMode: formMode as Ref<'create' | 'edit'>,

        // Navigation methods
        navigateToTaskSettings,
        navigateToTaskHistory,
        navigateToList,
        goBack,
        openExecDetail,
        closeExecDetail,
        openCreateForm,
        openEditForm,
        closeForm,

        // Data methods
        loadTasks,
        markAllTasksRead,

        // Polling
        startTaskPolling,
        stopTaskPolling,
    }
}

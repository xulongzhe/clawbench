import { ref, type Ref } from 'vue'
import { store } from '@/stores/app'

// Module-level singleton refs (shared across all consumers)
const currentView = ref<'list' | 'settings' | 'history'>('list')
const selectedTaskId = ref<string | null>(null)
const selectedExecId = ref<string | null>(null)
const selectedExecData = ref<any>(null)
const execDetailOpen = ref(false)
const formViewOpen = ref(false)
const formMode = ref<'create' | 'edit'>('create')

// Module-level polling timer
let pollingTimer: ReturnType<typeof setInterval> | null = null

export function useTaskTab() {
    // --- Navigation methods ---

    function navigateToTaskSettings(taskId: string) {
        selectedTaskId.value = taskId
        currentView.value = 'settings'
        execDetailOpen.value = false
        formViewOpen.value = false
    }

    function navigateToTaskHistory(taskId: string) {
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
        } else {
            currentView.value = 'list'
            selectedTaskId.value = null
        }
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
            store.state.taskUnread = !!data.hasUnread
            const newTasks = data.tasks || []
            // Diff-check to avoid unnecessary watcher triggers
            if (
                store.state.tasks.length !== newTasks.length ||
                newTasks.some(
                    (t: any, i: number) =>
                        t.id !== store.state.tasks[i]?.id ||
                        t.status !== store.state.tasks[i]?.status ||
                        t.runCount !== store.state.tasks[i]?.runCount
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
        await Promise.all(
            unreadTasks.map((t: any) =>
                fetch(`/api/tasks/${t.id}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ action: 'read' }),
                }).catch(() => {})
            )
        )
        store.state.taskUnread = false
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

<template>
  <div class="task-history-page">
    <!-- Header with breadcrumb -->
    <div class="history-header">
      <TaskBreadcrumb
        currentView="history"
        :taskName="task?.name"
        @navigate="onBreadcrumbNavigate"
      />
    </div>
    <!-- History content -->
    <div class="task-history-tab">
    <div v-if="loading" class="history-empty">{{ t('common.loading') }}</div>
    <div v-else-if="executions.length === 0 && runningExecutions.length === 0" class="history-empty">{{ t('task.exec.noExecutions') }}</div>
    <template v-else>
      <!-- Running executions -->
      <div v-for="exec in runningExecutions" :key="exec.id" class="execution-item running" @click.self>
        <div class="execution-row">
          <div class="execution-info">
            <div class="execution-time-row">
              <span class="exec-running-dot"></span>
              <span class="exec-running-label">{{ t('task.exec.running') }}</span>
              <span class="exec-relative-time">{{ chatRender.formatMessageTime(exec.startedAt) }}</span>
              <span v-if="exec.triggerType === 'manual'" class="exec-trigger-type manual">{{ t('task.exec.manual') }}</span>
              <span v-else class="exec-trigger-type auto">{{ t('task.exec.auto') }}</span>
            </div>
          </div>
          <button class="cancel-exec-btn" @click.stop="cancelExecution(exec.id)" :title="t('task.exec.cancel')">
            <Square :size="12" />
          </button>
        </div>
      </div>
      <!-- Completed executions -->
      <div v-for="(exec, idx) in executions" :key="idx" class="execution-item" :class="{ unread: exec.isUnread }" @click="openDetail(exec)">
        <div class="execution-row">
          <div class="execution-info">
            <div class="execution-time-row">
              <span class="exec-absolute-time">{{ formatAbsoluteTime(exec.createdAt) }}</span>
              <span class="exec-relative-time">{{ chatRender.formatMessageTime(exec.createdAt) }}</span>
              <span v-if="exec.isUnread" class="exec-unread-dot"></span>
              <span v-if="exec.triggerType === 'manual'" class="exec-trigger-type manual">{{ t('task.exec.manual') }}</span>
              <span v-else class="exec-trigger-type auto">{{ t('task.exec.auto') }}</span>
            </div>
            <div class="exec-summary-row">
              <div v-if="exec.summary" class="exec-summary">{{ exec.summary }}</div>
              <div v-else class="exec-summary empty">{{ t('task.exec.noTextOutput') }}</div>
            </div>
            <div v-if="exec.metadata" class="exec-meta-row">
              <span v-if="exec.metadata.wallMs" class="exec-meta-tag exec-meta-duration">{{ formatDuration(exec.metadata.wallMs) }}</span>
              <span v-if="exec.metadata.model" class="exec-meta-tag">{{ exec.metadata.model }}</span>
              <span v-if="exec.metadata.inputTokens || exec.metadata.outputTokens" class="exec-meta-tag">{{ formatTokens(exec.metadata) }}</span>
              <span v-if="exec.metadata.costUsd" class="exec-meta-tag">${{ exec.metadata.costUsd.toFixed(4) }}</span>
            </div>
          </div>
          <ChevronRight :size="14" class="exec-chevron" />
        </div>
      </div>
    </template>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronRight, Square } from 'lucide-vue-next'
import TaskBreadcrumb from '@/components/task/TaskBreadcrumb.vue'
import { useChatRender } from '@/composables/useChatRender.ts'
import { useToast } from '@/composables/useToast.ts'
import { useDialog } from '@/composables/useDialog.ts'
import { useTaskTab } from '@/composables/useTaskTab.ts'
import { formatDuration } from '@/utils/format.ts'

const props = defineProps({
  task: Object,
})

const emit = defineEmits(['open-file'])

const { t } = useI18n()
const dialog = useDialog()
const toast = useToast()
const { openExecDetail, goBack } = useTaskTab()

const chatRender = useChatRender({ messages: ref([]), theme: ref('light'), currentSessionId: ref('') })

function formatTokens(meta) {
  const parts = []
  if (meta.inputTokens) parts.push(`${meta.inputTokens.toLocaleString()}↑`)
  if (meta.outputTokens) parts.push(`${meta.outputTokens.toLocaleString()}↓`)
  return parts.join(' ')
}

function formatAbsoluteTime(createdAt) {
  const d = new Date(createdAt)
  const y = d.getFullYear()
  const mo = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const mi = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  return `${y}-${mo}-${day} ${h}:${mi}:${s}`
}

function extractSummary(exec) {
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

const loading = ref(false)
const executions = ref([])
const runningExecutions = ref([])
let pollTimer = null

function startPolling() {
  stopPolling()
  pollTimer = setInterval(loadRunningStatus, 3000)
}

function stopPolling() {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function loadExecutions() {
  if (!props.task?.id) return
  loading.value = true
  try {
    const resp = await fetch(`/api/tasks/${props.task.id}/executions`)
    const data = await resp.json()
    const rawExecutions = data.executions || []
    executions.value = rawExecutions.map(exec => {
      const { blocks, metadata } = chatRender.parseAssistantContent(exec.content)
      const summary = extractSummary(exec)
      return { ...exec, blocks, metadata, summary }
    })
  } catch (err) {
    console.error('Failed to load executions:', err)
  } finally {
    loading.value = false
  }
}

async function loadRunningStatus() {
  if (!props.task?.id) return
  try {
    const resp = await fetch(`/api/tasks/${props.task.id}`)
    if (resp.ok) {
      const data = await resp.json()
      runningExecutions.value = data.runningExecutions || []
    }
  } catch (err) {
    console.error('Failed to load running status:', err)
  }
}

async function cancelExecution(execId) {
  if (!props.task?.id) return
  if (!await dialog.confirm(t('task.exec.confirmCancel'))) return
  try {
    const resp = await fetch(`/api/tasks/${props.task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'cancel', executionId: execId }),
    })
    if (resp.ok) {
      toast.show(t('task.exec.cancelled'), { type: 'success' })
    } else if (resp.status === 404) {
      toast.show(t('task.exec.alreadyFinished'), { type: 'info' })
    }
    await loadRunningStatus()
  } catch (err) {
    console.error('Failed to cancel execution:', err)
  }
}

async function markExecRead(execId) {
  if (!props.task?.id || !execId) return
  try {
    await fetch(`/api/tasks/${props.task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'read', executionId: execId }),
    })
  } catch (err) {
    // Silently ignore read-mark failures
  }
}

function onBreadcrumbNavigate(view) {
  if (view === 'list') {
    goBack()
  }
}

function openDetail(exec) {
  if (exec.isUnread) {
    exec.isUnread = false
    markExecRead(exec.id)
  }
  openExecDetail(exec.id, exec)
}

function onOpenFile(filePath) {
  emit('open-file', filePath)
}

watch(() => props.task?.id, (newId) => {
  if (!newId) {
    stopPolling()
    executions.value = []
    runningExecutions.value = []
    return
  }
  loadExecutions()
  loadRunningStatus()
  startPolling()
}, { immediate: true })

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.task-history-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.history-header {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  flex-shrink: 0;
}

.task-history-tab {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

/* ── Empty state ── */
.history-empty {
  text-align: center;
  padding: 20px 12px;
  color: var(--text-muted, #999);
  font-size: 13px;
}

/* ── Execution items ── */
.execution-item {
  border-bottom: 1px solid var(--border-color, #e5e5e5);
}

.execution-item:last-child {
  border-bottom: none;
}

.execution-item.running {
  background: color-mix(in srgb, var(--success-color, #22c55e) 6%, transparent);
}

.execution-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  cursor: pointer;
  transition: background 0.15s;
}

@media (hover: hover) {
  .execution-item:not(.running) .execution-row:hover {
    background: var(--bg-secondary);
  }
}

.execution-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.execution-time-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.exec-absolute-time {
  font-size: 12px;
  color: var(--text-primary);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.exec-relative-time {
  font-size: 11px;
  color: var(--text-muted, #999);
  white-space: nowrap;
}

/* ── Unread dot ── */
.exec-unread-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-color, #0066cc);
  flex-shrink: 0;
  animation: exec-unread-pulse 1.2s ease-in-out infinite;
}

@keyframes exec-unread-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.7); }
}

.execution-item.unread .exec-absolute-time {
  color: var(--accent-color, #0066cc);
}

.execution-item.unread {
  animation: exec-unread-flash 0.8s ease-in-out infinite;
}

@keyframes exec-unread-flash {
  0%, 100% { background: transparent; }
  50% { background: color-mix(in srgb, var(--accent-color, #0066cc) 6%, transparent); }
}

/* ── Trigger type badges ── */
.exec-trigger-type {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
  white-space: nowrap;
}

.exec-trigger-type.manual {
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
}

.exec-trigger-type.auto {
  background: rgba(34, 197, 94, 0.12);
  color: #22c55e;
}

/* ── Summary ── */
.exec-summary-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.exec-summary {
  font-size: 12px;
  color: var(--text-secondary, #666);
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.exec-summary.empty {
  color: var(--text-muted, #999);
  font-style: italic;
}

/* ── Meta tags ── */
.exec-meta-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.exec-meta-tag {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-secondary, #666);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.exec-meta-duration {
  font-weight: 500;
  color: var(--text-primary);
}

/* ── Chevron ── */
.exec-chevron {
  flex-shrink: 0;
  color: var(--text-muted, #ccc);
}

/* ── Running execution indicator ── */
.exec-running-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--success-color, #22c55e);
  flex-shrink: 0;
  animation: exec-running-pulse 1.5s ease-in-out infinite;
}

@keyframes exec-running-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.4); }
  50% { opacity: 0.7; box-shadow: 0 0 6px 2px rgba(34, 197, 94, 0.2); }
}

.exec-running-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--success-color, #22c55e);
}

/* ── Cancel button ── */
.cancel-exec-btn {
  width: 24px;
  height: 24px;
  border: none;
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.15s;
}

@media (hover: hover) {
  .cancel-exec-btn:hover {
    background: rgba(239, 68, 68, 0.2);
  }
}
</style>

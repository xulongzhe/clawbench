<template>
  <div class="task-history-page">
    <!-- Header with breadcrumb -->
    <div class="history-header">
      <TaskBreadcrumb />
    </div>
    <!-- History content -->
    <div class="task-history-tab">
    <div v-if="loading" class="history-empty">
      <Loader2 class="spin-icon" :size="20" />
      <span>{{ t('common.loading') }}</span>
    </div>
    <div v-else-if="allExecutions.length === 0" class="history-empty">
      <History class="empty-icon" :size="32" />
      <span>{{ t('task.exec.noExecutions') }}</span>
    </div>
    <div v-else class="history-list">
      <div v-if="executions.length > 0" class="clear-all-row">
        <button class="clear-all-btn" @click="deleteAllExecutions">{{ t('task.exec.clearAll') }}</button>
      </div>
      <div v-for="exec in allExecutions" :key="exec.id" class="execution-item" :class="{ running: isRunning(exec), unread: !isRunning(exec) && isUnreadDisplay(exec), 'just-completed': isJustCompleted(exec) }" @click="!isRunning(exec) && openDetail(exec)">
        <div class="execution-row">
          <div class="execution-info">
            <div class="execution-time-row">
              <template v-if="isRunning(exec)">
                <span class="exec-running-dot"></span>
                <span class="exec-running-label">{{ t('task.exec.running') }}</span>
                <span class="exec-relative-time">{{ formatRelativeTime(exec.createdAt) }}</span>
              </template>
              <template v-else>
                <span class="exec-absolute-time">{{ formatAbsoluteTime(exec.createdAt) }}</span>
                <span class="exec-relative-time">{{ formatRelativeTime(exec.createdAt) }}</span>
                <span v-if="isUnreadDisplay(exec)" class="exec-unread-dot"></span>
              </template>
              <span v-if="exec.triggerType === 'manual'" class="exec-trigger-type manual">{{ t('task.exec.manual') }}</span>
              <span v-else class="exec-trigger-type auto">{{ t('task.exec.auto') }}</span>
              <template v-if="!isRunning(exec)">
                <span v-if="exec.status === 'cancelled'" class="exec-status-badge cancelled">{{ t('task.exec.statusCancelled') }}</span>
                <span v-else-if="exec.status === 'failed'" class="exec-status-badge failed">{{ t('task.exec.statusFailed') }}</span>
              </template>
            </div>
            <template v-if="!isRunning(exec)">
              <div class="exec-summary-row">
                <div v-if="exec.preview" class="exec-summary">{{ exec.preview }}</div>
                <div v-else class="exec-summary empty">{{ t('task.exec.noTextOutput') }}</div>
              </div>
              <div v-if="exec.metadata" class="exec-meta-row">
                <span v-if="exec.metadata.wallMs" class="exec-meta-tag exec-meta-duration">{{ formatDuration(exec.metadata.wallMs) }}</span>
                <span v-if="exec.metadata.model" class="exec-meta-tag">{{ exec.metadata.model }}</span>
                <span v-if="exec.metadata.inputTokens || exec.metadata.outputTokens" class="exec-meta-tag">{{ formatTokens(exec.metadata) }}</span>
              </div>
            </template>
          </div>
          <template v-if="isRunning(exec)">
            <button class="cancel-exec-btn" @click.stop="cancelExecution(exec.id)" :title="t('task.exec.cancel')">
              <Square :size="12" />
            </button>
          </template>
          <template v-else>
            <button class="delete-exec-btn" @click.stop="deleteExecution(exec.id)" :title="t('task.delete')">
              <Trash2 :size="14" />
            </button>
          </template>
        </div>
      </div>
    </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Square, Loader2, History, Trash2 } from 'lucide-vue-next'
import TaskBreadcrumb from '@/components/task/TaskBreadcrumb.vue'
import { useTaskHistory } from '@/composables/useTaskHistory.ts'
import { useSystemEvents } from '@/composables/useSystemEvents.ts'
import { formatDuration, formatRelativeTime } from '@/utils/format.ts'

const props = defineProps({
  task: Object,
})

const emit = defineEmits(['open-file'])

const { t } = useI18n()

// Task history composable (ISS-011 + ISS-015 + ISS-016)
const {
  loading,
  allExecutions,
  executions,
  isRunning,
  isJustCompleted,
  locallyReadIds,
  loadExecutions,
  loadRunningStatus,
  cancelExecution,
  deleteExecution,
  deleteAllExecutions,
  openDetail,
  isUnreadDisplay,
  onTaskChange,
} = useTaskHistory({ task: computed(() => props.task) })

// Event-driven execution status updates (replaces 3s polling)
const { registerUIHandler } = useSystemEvents()
const unregisterExecEvents = registerUIHandler('task_exec_update', (payload: any) => {
  if (payload.taskId === props.task?.id) {
    if (payload.status === 'running') {
      // New execution started — refresh running list
      loadRunningStatus()
    } else if (['completed', 'failed', 'cancelled'].includes(payload.status)) {
      // Execution finished — refresh both running and completed lists
      loadRunningStatus()
      loadExecutions()
    }
  }
})

function onOpenFile(filePath) {
  emit('open-file', filePath)
}

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


watch(() => props.task?.id, (newId) => {
  if (!newId) return
  onTaskChange()
  loadExecutions()
  loadRunningStatus()
}, { immediate: true })

onUnmounted(() => {
  unregisterExecEvents()
  onTaskChange() // Abort in-flight requests (ISS-016)
})
</script>

<style scoped>
.task-history-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  background: var(--bg-primary, #ffffff);
}

.history-header {
  display: flex;
  align-items: center;
  padding: 4px 8px;
  flex-shrink: 0;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
}

.task-history-tab {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
  padding: 8px;
}

/* ── Empty state ── */
.history-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  height: 100%;
  color: var(--text-muted, #999);
  font-size: 14px;
}

.spin-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  100% { transform: rotate(360deg); }
}

.empty-icon {
  opacity: 0.5;
}

/* ── Execution items ── */
.history-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.execution-item {
  background: var(--bg-secondary, #f8f9fa);
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 10px;
  overflow: hidden;
  transition: all 0.2s ease;
}

@media (hover: hover) {
  .execution-item:not(.running):hover {
    border-color: var(--accent-color, #0066cc);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
    transform: translateY(-1px);
  }
}

.execution-item:active:not(.running) {
  background: var(--bg-tertiary, #eef1f4);
  transform: translateY(0);
}

.execution-item.running {
  background: color-mix(in srgb, var(--success-color, #16a34a) 5%, var(--bg-secondary, #f8f9fa));
  border-color: color-mix(in srgb, var(--success-color, #16a34a) 30%, transparent);
  animation: exec-card-running 2s ease-in-out infinite;
}

@keyframes exec-card-running {
  0%, 100% { border-color: color-mix(in srgb, var(--success-color, #16a34a) 30%, transparent); }
  50% { border-color: color-mix(in srgb, var(--success-color, #16a34a) 55%, transparent); }
}

.execution-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  cursor: pointer;
}

.execution-item.running .execution-row {
  cursor: default;
}

.execution-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.execution-time-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.exec-absolute-time {
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.exec-relative-time {
  font-size: 12px;
  color: var(--text-muted, #9ca3af);
  white-space: nowrap;
}

/* ── Unread dot ── */
.exec-unread-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-color, #0066cc);
  flex-shrink: 0;
  animation: exec-unread-pulse 1.5s ease-in-out infinite;
}

@keyframes exec-unread-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(0, 102, 204, 0.4); }
  50% { opacity: 0.6; box-shadow: 0 0 6px 2px rgba(0, 102, 204, 0.2); }
}

.execution-item.unread {
  border-left: 3px solid var(--accent-color, #0066cc);
}

/* ── Trigger type badges ── */
.exec-trigger-type {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
  flex-shrink: 0;
  white-space: nowrap;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.exec-trigger-type.manual {
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}

.exec-trigger-type.auto {
  background: rgba(34, 197, 94, 0.12);
  color: #16a34a;
}

/* ── Status badges ── */
.exec-status-badge {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
  margin-left: auto;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}
.exec-status-badge.cancelled {
  background: var(--bg-tertiary, #e5e7eb);
  color: var(--text-secondary, #4b5563);
}
.exec-status-badge.failed {
  background: rgba(239, 68, 68, 0.12);
  color: #dc2626;
}

/* ── Summary ── */
.exec-summary-row {
  display: flex;
  align-items: center;
}

.exec-summary {
  font-size: 13px;
  color: var(--text-secondary, #4b5563);
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.exec-summary.empty {
  color: var(--text-muted, #9ca3af);
  font-style: italic;
}

/* ── Meta tags ── */
.exec-meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 2px;
}

.exec-meta-tag {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--bg-primary, #ffffff);
  border: 1px solid var(--border-color, #e5e7eb);
  color: var(--text-secondary, #6b7280);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
  display: inline-flex;
  align-items: center;
}

.exec-meta-duration {
  font-weight: 600;
  color: var(--text-primary, #111827);
  background: rgba(0, 102, 204, 0.05);
  border-color: rgba(0, 102, 204, 0.1);
}

/* ── Running execution indicator ── */
.exec-running-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #16a34a;
  flex-shrink: 0;
  animation: exec-running-pulse 0.8s ease-in-out infinite;
}

@keyframes exec-running-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(22, 163, 74, 0.5); }
  50% { opacity: 0.7; box-shadow: 0 0 10px 4px rgba(22, 163, 74, 0.3); }
}

/* ── Just-completed execution flash ── */
.execution-item.just-completed {
  animation: exec-just-completed 0.6s ease-out forwards;
}

@keyframes exec-just-completed {
  0% { background: color-mix(in srgb, var(--accent-color, #0066cc) 15%, var(--bg-secondary, #f8f9fa)); transform: translateX(8px); opacity: 0.7; }
  100% { background: var(--bg-secondary, #f8f9fa); transform: translateX(0); opacity: 1; }
}

.exec-running-label {
  font-size: 13px;
  font-weight: 600;
  color: #16a34a;
}

/* ── Cancel button ── */
.cancel-exec-btn {
  width: 32px;
  height: 32px;
  border: none;
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  border-radius: 8px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.2s;
}

@media (hover: hover) {
  .cancel-exec-btn:hover {
    background: rgba(239, 68, 68, 0.2);
    transform: scale(1.05);
  }
}

.cancel-exec-btn:active {
  transform: scale(0.95);
}

/* ── Delete button ── */
.delete-exec-btn {
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--text-muted, #9ca3af);
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.2s;
  opacity: 0;
}

@media (hover: hover) {
  .execution-item:not(.running):hover .delete-exec-btn {
    opacity: 1;
  }
  .delete-exec-btn:hover {
    background: rgba(239, 68, 68, 0.1);
    color: #ef4444;
  }
}

/* Touch devices: always visible but subtle */
@media (hover: none) {
  .delete-exec-btn {
    opacity: 0.5;
  }
}

.delete-exec-btn:active {
  transform: scale(0.9);
}

/* ── Clear all row ── */
.clear-all-row {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 2px;
}

.clear-all-btn {
  border: none;
  background: transparent;
  color: var(--text-muted, #9ca3af);
  font-size: 12px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 6px;
  transition: all 0.2s;
}

@media (hover: hover) {
  .clear-all-btn:hover {
    color: #ef4444;
    background: rgba(239, 68, 68, 0.06);
  }
}

.clear-all-btn:active {
  transform: scale(0.95);
}
</style>

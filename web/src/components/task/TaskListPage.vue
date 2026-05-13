<template>
  <div class="task-list-page">
    <!-- Compact header: breadcrumb + create button -->
    <div class="list-header">
      <TaskBreadcrumb />
      <button class="create-btn" @click="$emit('create')" :title="t('task.form.createTitle')">
        <Plus :size="16" />
      </button>
    </div>
    <div class="task-list-body">
      <div v-if="loading && tasks.length === 0" class="task-loading">
        <Loader2 class="loading-icon" :size="20" />
        <span>{{ t('common.loading') }}</span>
      </div>
      <div v-else-if="tasks.length === 0" class="task-empty">
        <CalendarX class="empty-icon" :size="32" />
        <span>{{ t('task.noTasks') }}</span>
      </div>
      <div v-else class="task-items-container">
        <div
          v-for="task in tasks"
          :key="task.id"
          class="task-item"
          :class="[task.status, { 'has-unread': task.unreadCount > 0 }]"
          @click="$emit('select', task.id)"
        >
          <div class="task-item-main">
            <div class="task-item-header">
              <span class="task-item-icon">{{ getAgentIcon(task.agentId) }}</span>
              <span class="task-item-name">{{ task.name }}</span>
              <span v-if="task.runningCount > 0" class="task-item-running-dot" :title="t('task.exec.running')"></span>
              <span v-if="task.unreadCount > 0" class="task-item-unread">{{ task.unreadCount }}</span>
              <span class="task-item-status" :class="task.status">{{ statusLabel(task.status) }}</span>
            </div>
            <div class="task-item-meta">
              <div class="meta-item cron" :title="task.cronExpr">
                <Clock class="meta-icon" :size="12" />
                <span>{{ humanizeCron(task.cronExpr) }}</span>
              </div>
              <div class="meta-item repeat">
                <Repeat class="meta-icon" :size="12" />
                <span>{{ repeatLabel(task.repeatMode, task.maxRuns) }}</span>
                <span v-if="task.repeatMode !== 'unlimited'" class="task-progress">({{ task.runCount }}/{{ task.maxRuns || 1 }})</span>
              </div>
            </div>
            <div v-if="task.nextRunAt" class="task-item-next">
              <CalendarClock class="meta-icon" :size="12" />
              <span>{{ t('task.nextRun', { time: formatDateTime(task.nextRunAt) }) }}</span>
            </div>
          </div>
          <ChevronRight :size="16" class="task-item-chevron" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Plus, ChevronRight, Loader2, CalendarX, Clock, Repeat, CalendarClock } from 'lucide-vue-next'
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTaskTab } from '@/composables/useTaskTab'
import { useAgents } from '@/composables/useAgents'
import { humanizeCron, repeatLabel, statusLabel, formatDateTime } from '@/utils/format'
import { store } from '@/stores/app'
import TaskBreadcrumb from '@/components/task/TaskBreadcrumb.vue'

const { t } = useI18n()
const { loadTasks } = useTaskTab()
const { loadAgents, getAgentIcon } = useAgents()

const tasks = computed(() => store.state.tasks)
const loading = ref(false)

defineEmits<{
  create: []
  select: [taskId: number]
}>()

async function refresh() {
  loading.value = true
  try {
    await Promise.all([loadTasks(), loadAgents()])
  } finally {
    loading.value = false
  }
}

defineExpose({ refresh })

onMounted(refresh)
</script>

<style scoped>
.task-list-page {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--bg-primary, #ffffff);
}

/* Compact header — matches detail/form/history pages */
.list-header {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  flex-shrink: 0;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  gap: 8px;
}

/* Create button in header toolbar */
.create-btn {
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 14px;
  background: var(--accent-color, #0066cc);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.2s ease;
}

@media (hover: hover) {
  .create-btn:hover {
    background: color-mix(in srgb, var(--accent-color, #0066cc) 85%, black);
    transform: translateY(-1px);
  }
}

.create-btn:active {
  transform: scale(0.9);
}

.task-list-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
}

.task-items-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-loading,
.task-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  height: 100%;
  color: var(--text-muted, #999);
  font-size: 14px;
}

.loading-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  100% { transform: rotate(360deg); }
}

.empty-icon {
  opacity: 0.5;
}

.task-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px;
  background: var(--bg-secondary, #f8f9fa);
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  overflow: hidden;
}

@media (hover: hover) {
  .task-item:hover {
    border-color: var(--accent-color, #0066cc);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
    transform: translateY(-1px);
  }
}

.task-item:active {
  background: var(--bg-tertiary, #eef1f4);
  transform: translateY(0);
}

.task-item.completed {
  opacity: 0.65;
  background: var(--bg-tertiary, #f1f3f5);
}

.task-item-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.task-item-header {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.task-item-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.task-item-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #1a1a1a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.task-item-unread {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 10px;
  font-weight: 600;
  background: #ef4444;
  color: #fff;
  flex-shrink: 0;
  min-width: 16px;
  text-align: center;
  line-height: 1.2;
}

.task-item.has-unread {
  border-left: 3px solid #ef4444;
}

.task-item.has-unread .task-item-icon {
  animation: task-unread-flash 0.8s ease-in-out infinite;
}

@keyframes task-unread-flash {
  0%, 100% { opacity: 1; text-shadow: 0 0 0 transparent; }
  50% { opacity: 0.7; text-shadow: 0 0 8px color-mix(in srgb, var(--accent-color, #0066cc) 40%, transparent); }
}

.task-item-status {
  font-size: 10px;
  padding: 3px 6px;
  border-radius: 4px;
  font-weight: 600;
  flex-shrink: 0;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.task-item-status.active {
  background: rgba(34, 197, 94, 0.12);
  color: #16a34a;
}

.task-item-status.paused {
  background: rgba(234, 179, 8, 0.12);
  color: #ca8a04;
}

.task-item-status.completed {
  background: rgba(156, 163, 175, 0.15);
  color: #6b7280;
}

.task-item-running-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #22c55e;
  flex-shrink: 0;
  animation: task-running-pulse 1.5s ease-in-out infinite;
}

@keyframes task-running-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.4); }
  50% { opacity: 0.8; box-shadow: 0 0 8px 3px rgba(34, 197, 94, 0.2); }
}

.task-item-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--text-secondary, #666);
  min-width: 0;
  flex-wrap: wrap;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.meta-icon {
  color: var(--text-muted, #999);
}

.cron span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 140px;
}

.task-progress {
  color: var(--accent-color, #0066cc);
  font-weight: 500;
  margin-left: 2px;
}

.task-item-next {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--text-muted, #999);
  background: var(--bg-primary, #fff);
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid var(--border-color, #e5e5e5);
  width: fit-content;
}

.task-item-chevron {
  color: var(--text-muted, #cbd5e1);
  flex-shrink: 0;
  margin-left: 8px;
  transition: transform 0.2s;
}

@media (hover: hover) {
  .task-item:hover .task-item-chevron {
    transform: translateX(2px);
    color: var(--accent-color, #0066cc);
  }
}
</style>

<template>
  <div class="task-list-page">
    <div class="task-list-body">
      <div v-if="loading" class="task-loading">{{ t('common.loading') }}</div>
      <div v-else-if="tasks.length === 0" class="task-empty">{{ t('task.noTasks') }}</div>
      <div
        v-for="task in tasks"
        :key="task.id"
        class="task-item"
        :class="[task.status, { 'has-unread': task.unreadCount > 0 }]"
      >
        <div class="task-item-main" @click="$emit('select', task.id)">
          <div class="task-item-info">
            <div class="task-item-header">
              <span class="task-item-icon">{{ getAgentIcon(task.agentId) }}</span>
              <span class="task-item-name">{{ task.name }}</span>
              <span v-if="task.runningCount > 0" class="task-item-running-dot" :title="t('task.exec.running')"></span>
              <span v-if="task.unreadCount > 0" class="task-item-unread">{{ task.unreadCount }}</span>
              <span class="task-item-status" :class="task.status">{{ statusLabel(task.status) }}</span>
            </div>
            <div class="task-item-meta">
              <span class="task-item-cron">{{ humanizeCron(task.cronExpr) }}</span>
              <span class="task-item-repeat">{{ repeatLabel(task.repeatMode, task.maxRuns) }}</span>
              <span v-if="task.repeatMode !== 'unlimited'" class="task-item-progress">{{ task.runCount }}/{{ task.maxRuns || 1 }}</span>
            </div>
            <div v-if="task.nextRunAt" class="task-item-next">
              {{ t('task.nextRun', { time: formatDateTime(task.nextRunAt) }) }}
            </div>
          </div>
          <ChevronRight :size="16" class="task-item-chevron" />
        </div>
        <!-- History button — separate drill-down to execution history -->
        <button
          v-if="task.runCount > 0 || task.runningCount > 0"
          class="task-item-history-btn"
          @click.stop="$emit('select-history', task.id)"
          :title="t('task.exec.title')"
        >
          <Clock :size="14" />
          <span class="history-count">{{ task.runCount }}</span>
        </button>
      </div>
    </div>
    <!-- Fixed FAB -->
    <button class="create-fab" @click="$emit('create')" :title="t('task.form.createTitle')">
      <Plus :size="20" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { Plus, ChevronRight, Clock } from 'lucide-vue-next'
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTaskTab } from '@/composables/useTaskTab'
import { useAgents } from '@/composables/useAgents'
import { humanizeCron, repeatLabel, statusLabel, formatDateTime } from '@/utils/format'
import { store } from '@/stores/app'

const { t } = useI18n()
const { loadTasks, markAllTasksRead } = useTaskTab()
const { loadAgents, getAgentIcon } = useAgents()

const tasks = computed(() => store.state.tasks)
const loading = ref(false)

defineEmits<{
  create: []
  select: [taskId: string]
  'select-history': [taskId: string]
}>()

async function refresh() {
  loading.value = true
  try {
    await Promise.all([loadTasks(), loadAgents()])
    markAllTasksRead()
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
  position: relative;
}

.task-list-body {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}

.task-loading,
.task-empty {
  padding: 32px 12px;
  text-align: center;
  color: var(--text-muted, #999);
  font-size: 13px;
}

/* Floating Action Button — fixed bottom-right */
.create-fab {
  position: absolute;
  bottom: 16px;
  right: 16px;
  width: 44px;
  height: 44px;
  border: none;
  border-radius: 50%;
  background: var(--accent-color, #0066cc);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.18);
  transition: transform 0.15s, opacity 0.15s;
  z-index: 1;
}

.create-fab:active {
  transform: scale(0.92);
}

@media (hover: hover) {
  .create-fab:hover {
    opacity: 0.88;
  }
}

.task-item {
  position: relative;
}

.task-item.completed {
  opacity: 0.5;
}

.task-item-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  cursor: pointer;
  transition: background 0.15s;
  gap: 8px;
}

@media (hover: hover) {
  .task-item-main:hover {
    background: var(--bg-tertiary, rgba(0, 0, 0, 0.03));
  }
}

.task-item-main:active {
  background: var(--bg-tertiary, rgba(0, 0, 0, 0.06));
}

.task-item:not(:last-child)::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 16px;
  right: 16px;
  height: 1px;
  background: var(--border-color, #e5e5e5);
  opacity: 0.5;
}

.task-item-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
  overflow: hidden;
}

.task-item-header {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}

.task-item-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.task-item-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary, #1a1a1a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.task-item-unread {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 8px;
  font-weight: 600;
  background: #ef4444;
  color: #fff;
  flex-shrink: 0;
  min-width: 14px;
  text-align: center;
  line-height: 1.3;
}

.task-item.has-unread .task-item-icon {
  animation: task-unread-flash 0.8s ease-in-out infinite;
}

@keyframes task-unread-flash {
  0%, 100% {
    opacity: 1;
    text-shadow: 0 0 0 transparent;
  }
  50% {
    opacity: 0.7;
    text-shadow: 0 0 8px color-mix(in srgb, var(--accent-color, #0066cc) 40%, transparent);
  }
}

.task-item-status {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
  line-height: 1.4;
}

.task-item-status.active {
  background: rgba(34, 197, 94, 0.12);
  color: #22c55e;
}

.task-item-status.paused {
  background: rgba(234, 179, 8, 0.12);
  color: #eab308;
}

.task-item-status.completed {
  background: var(--bg-tertiary, #e9ecef);
  color: var(--text-muted, #999);
}

.task-item-running-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--success-color, #22c55e);
  flex-shrink: 0;
  animation: task-running-pulse 1.5s ease-in-out infinite;
}

@keyframes task-running-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.4); }
  50% { opacity: 0.7; box-shadow: 0 0 6px 2px rgba(34, 197, 94, 0.2); }
}

.task-item-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-muted, #999);
  min-width: 0;
  overflow: hidden;
  flex-wrap: wrap;
}

.task-item-cron {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 60%;
}

.task-item-next {
  font-size: 10px;
  color: var(--text-muted, #999);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-item-progress {
  font-weight: 500;
  color: var(--accent-color, #0066cc);
  flex-shrink: 0;
}

.task-item-chevron {
  color: var(--text-muted, #999);
  flex-shrink: 0;
}

/* History button — right side, separate click target */
.task-item-history-btn {
  position: absolute;
  right: 16px;
  bottom: 8px;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 2px 6px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted, #999);
  font-size: 10px;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

@media (hover: hover) {
  .task-item-history-btn:hover {
    background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
    color: var(--accent-color, #0066cc);
  }
}

.task-item-history-btn:active {
  background: var(--bg-tertiary, rgba(0, 0, 0, 0.08));
}

.history-count {
  font-weight: 500;
}
</style>

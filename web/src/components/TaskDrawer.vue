<template>
  <BottomSheet ref="bottomSheetRef" :open="open" compact title="定时任务" @close="$emit('close')">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
      </svg>
      <span class="bs-header-title">定时任务</span>
    </template>

    <div class="task-list">
      <div v-if="loading" class="task-loading">加载中...</div>
      <div v-else-if="tasks.length === 0" class="task-empty">暂无定时任务</div>
      <div v-for="task in tasks" :key="task.id" class="task-item" :class="task.status">
        <div class="task-item-main" @click="openTaskDetailDialog(task)">
          <div class="task-item-info">
            <div class="task-item-header">
              <span class="task-item-icon">{{ getAgentIcon(task.agentId) }}</span>
              <span class="task-item-name">{{ task.name }}</span>
              <span class="task-item-status" :class="task.status">{{ taskStatusLabel(task.status) }}</span>
            </div>
            <div class="task-item-meta">
              <span class="task-item-cron">{{ humanizeCron(task.cronExpr) }}</span>
              <span class="task-item-repeat">{{ taskRepeatLabel(task.repeatMode, task.maxRuns) }}</span>
              <span v-if="task.repeatMode !== 'unlimited'" class="task-item-progress">{{ task.runCount }}/{{ task.maxRuns || 1 }}</span>
            </div>
            <div v-if="task.nextRunAt" class="task-item-next">
              下次执行: {{ formatTaskTime(task.nextRunAt) }}
            </div>
          </div>
          <div class="task-item-actions">
            <button v-if="task.status === 'active'" class="task-action-btn pause" @click.stop="pauseTask(task.id)" title="暂停">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>
            </button>
            <button v-if="task.status === 'paused'" class="task-action-btn resume" @click.stop="resumeTask(task.id)" title="恢复">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>
            </button>
            <button class="task-action-btn delete" @click.stop="deleteTask(task.id)" title="删除">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <TaskDetailDialog
      :open="taskDetailOpen"
      :task="selectedTask"
      @close="taskDetailOpen = false"
      @saved="() => { loadTasks(); taskDetailOpen = false }"
    />
  </BottomSheet>
</template>

<script setup>
import { ref, watch } from 'vue'
import BottomSheet from './BottomSheet.vue'
import TaskDetailDialog from './TaskDetailDialog.vue'

const props = defineProps({
  open: Boolean,
})

const emit = defineEmits(['close'])

const bottomSheetRef = ref(null)
const tasks = ref([])
const loading = ref(false)
const taskDetailOpen = ref(false)
const selectedTask = ref(null)
const agents = ref([])

defineExpose({ loadTasks })

async function loadTasks() {
  loading.value = true
  try {
    const resp = await fetch('/api/tasks')
    const data = await resp.json()
    tasks.value = data.tasks || []
  } catch (err) {
    console.error('Failed to load tasks:', err)
  } finally {
    loading.value = false
  }
}

async function loadAgents() {
  try {
    const resp = await fetch('/api/agents')
    const data = await resp.json()
    agents.value = data.agents || []
  } catch (err) {
    console.error('Failed to load agents:', err)
  }
}

function getAgentIcon(agentId) {
  const agent = agents.value.find(a => a.id === agentId)
  return agent ? agent.icon : '🤖'
}

function humanizeCron(expr) {
  const parts = expr.split(' ')
  if (parts.length !== 5) return expr
  const [min, hour, day, month, weekday] = parts
  if (min.startsWith('*/') && hour === '*') return `每 ${min.slice(2)} 分钟`
  if (hour.startsWith('*/') && min === '0') return `每 ${hour.slice(2)} 小时`
  if (min === '0' && !hour.includes('/') && day === '*' && month === '*' && weekday === '*') return `每天 ${hour}:00`
  if (min === '0' && weekday === '1-5') return `工作日 ${hour}:00`
  return expr
}

function taskRepeatLabel(mode, maxRuns) {
  if (mode === 'once') return '单次'
  if (mode === 'limited') return `${maxRuns}次`
  return '不限'
}

function taskStatusLabel(status) {
  if (status === 'active') return '运行中'
  if (status === 'paused') return '已暂停'
  if (status === 'completed') return '已完成'
  return status
}

function formatTaskTime(date) {
  if (!date) return ''
  const d = new Date(date)
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function openTaskDetailDialog(task) {
  selectedTask.value = task
  taskDetailOpen.value = true
}

async function pauseTask(id) {
  await fetch(`/api/tasks/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action: 'pause' }),
  })
  await loadTasks()
}

async function resumeTask(id) {
  await fetch(`/api/tasks/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action: 'resume' }),
  })
  await loadTasks()
}

async function deleteTask(id) {
  if (!confirm('确定删除此任务？')) return
  try {
    await fetch(`/api/tasks/${id}`, { method: 'DELETE' })
    await loadTasks()
  } catch (err) {
    console.error('Failed to delete task:', err)
  }
}

watch(() => props.open, async (val) => {
  if (val) {
    await Promise.all([loadTasks(), loadAgents()])
  }
})
</script>

<style scoped>
.task-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px;
  min-height: 0;
  overflow-y: auto;
  flex: 1;
}

.task-loading,
.task-empty {
  padding: 24px 12px;
  text-align: center;
  color: var(--text-muted, #999);
  font-size: 13px;
}

.task-item {
  border-radius: 6px;
  border: 1px solid var(--border-color, #e5e5e5);
  overflow: hidden;
}

.task-item.completed {
  opacity: 0.6;
}

.task-item-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  cursor: pointer;
}

.task-item-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.task-item-header {
  display: flex;
  align-items: center;
  gap: 4px;
}

.task-item-icon {
  font-size: 14px;
}

.task-item-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary, #1a1a1a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-item-status {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
}

.task-item-status.active {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}

.task-item-status.paused {
  background: rgba(234, 179, 8, 0.15);
  color: #eab308;
}

.task-item-status.completed {
  background: var(--bg-tertiary, #e9ecef);
  color: var(--text-muted, #999);
}

.task-item-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-muted, #999);
}

.task-item-next {
  font-size: 10px;
  color: var(--text-muted, #999);
}

.task-item-progress {
  font-weight: 500;
  color: var(--accent-color, #0066cc);
}

.task-item-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.task-action-btn {
  width: 22px;
  height: 22px;
  border: none;
  background: none;
  color: var(--text-muted, #999);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.15s;
}

.task-action-btn:hover {
  color: var(--text-secondary, #666);
  background: var(--bg-tertiary, #f0f0f0);
}

.task-action-btn.delete:hover {
  color: #dc3545;
  background: var(--bg-tertiary, #f0f0f0);
}
</style>

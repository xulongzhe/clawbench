<template>
  <BottomSheet ref="bottomSheetRef" :open="open" compact no-header @close="$emit('close')">
    <!-- Tab Switcher with drag handle -->
    <div class="tab-bar">
      <div class="drag-handle"></div>
      <div class="tab-switcher">
        <button
          class="tab-btn"
          :class="{ active: activeTab === 'sessions' }"
          @click="activeTab = 'sessions'"
        >
          会话
        </button>
        <button
          class="tab-btn"
          :class="{ active: activeTab === 'tasks' }"
          @click="activeTab = 'tasks'"
        >
          定时任务
        </button>
      </div>
    </div>

    <!-- Sessions Tab -->
    <div v-show="activeTab === 'sessions'" class="tab-content">
      <div class="session-list">
        <div v-if="loading" class="session-loading">加载中...</div>
        <div v-else-if="sessions.length === 0" class="session-empty">暂无会话</div>
        <div
          v-for="session in sessionsWithStatus"
          :key="session.id"
          class="session-item"
          :class="{ active: session.id === currentSessionId, running: session.running }"
          @click="selectSession(session.id, session.backend)"
        >
          <div class="session-item-main">
            <div class="session-item-info">
              <div class="session-item-header">
                <span class="session-item-title">{{ session.title }}</span>
                <span v-if="session.unreadCount > 0" class="session-item-unread">{{ session.unreadCount }}</span>
                <span v-if="session.running" class="session-item-status running">
                  <span class="status-dot"></span>
                  运行中
                </span>
              </div>
              <div class="session-item-meta">
                <span class="session-item-time">{{ formatRelativeTime(session.updatedAt) }}</span>
                <span class="session-item-agent">{{ getAgentIcon(session.agentId) }} {{ getAgentName(session.agentId) }}</span>
                <span class="session-item-backend">{{ session.backend }}</span>
                <span v-if="session.model" class="session-item-model">{{ session.model }}</span>
              </div>
            </div>
            <button class="session-item-delete" @click.stop="deleteSession(session.id)" title="删除">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                <polyline points="3,6 5,6 21,6"/>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Tasks Tab -->
    <div v-show="activeTab === 'tasks'" class="tab-content">
      <div class="task-list">
        <div v-if="taskLoading" class="task-loading">加载中...</div>
        <div v-else-if="tasks.length === 0" class="task-empty">暂无定时任务</div>
        <div v-for="task in tasks" :key="task.id" class="task-item" :class="task.status">
          <div class="task-item-main" @click="openTaskDetailDialog(task)">
            <div class="task-item-info">
              <div class="task-item-header">
                <span class="task-item-icon">{{ getAgentIcon(task.agentId) }}</span>
                <span class="task-item-name">{{ task.name }}</span>
                <span class="task-item-status" :class="task.status">{{ statusLabel(task.status) }}</span>
              </div>
              <div class="task-item-meta">
                <span class="task-item-cron">{{ humanizeCron(task.cronExpr) }}</span>
                <span class="task-item-repeat">{{ repeatLabel(task.repeatMode, task.maxRuns) }}</span>
                <span v-if="task.repeatMode !== 'unlimited'" class="task-item-progress">{{ task.runCount }}/{{ task.maxRuns || 1 }}</span>
              </div>
              <div v-if="task.nextRunAt" class="task-item-next">
                下次执行: {{ formatDateTime(task.nextRunAt) }}
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
    </div>

    <!-- Footer for Sessions Tab only -->
    <template v-if="activeTab === 'sessions'" #footer>
      <div class="session-footer">
        <button class="create-btn" @click="showAgentSelector = true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          新建会话
        </button>

        <!-- Agent selector modal -->
        <div v-if="showAgentSelector" class="agent-selector-modal" @click.self="showAgentSelector = false">
          <div class="agent-selector-content">
            <div class="agent-selector-header">选择智能体</div>
            <div class="agent-list">
              <button
                v-for="agent in agents"
                :key="agent.id"
                class="agent-option"
                :class="{ selected: selectedAgentId === agent.id }"
                @click="createSession(agent.id)"
              >
                <span class="agent-option-icon">{{ agent.icon }}</span>
                <div class="agent-option-detail">
                  <span class="agent-option-name">{{ agent.name }}</span>
                  <span class="agent-option-specialty">{{ agent.specialty }}</span>
                  <div class="agent-option-tags">
                    <span class="agent-tag backend-tag">{{ agent.backend }}</span>
                    <span class="agent-tag model-tag">{{ agent.model }}</span>
                  </div>
                </div>
              </button>
            </div>
            <button class="close-selector-btn" @click="showAgentSelector = false">取消</button>
          </div>
        </div>
      </div>
    </template>
  </BottomSheet>
</template>

<script setup>
import { ref, watch, computed } from 'vue'
import BottomSheet from './BottomSheet.vue'
import TaskDetailDialog from './TaskDetailDialog.vue'
import { useAgents } from '@/composables/useAgents.ts'
import { formatRelativeTime, humanizeCron, repeatLabel, statusLabel, formatDateTime } from '@/utils/helpers.ts'

const props = defineProps({
  open: Boolean,
  currentSessionId: String,
  runningSessionIds: { type: Set, default: () => new Set() },
})

const emit = defineEmits(['close', 'select', 'create', 'delete'])

// BottomSheet ref for animated close
const bottomSheetRef = ref(null)

// Tab management
const activeTab = ref('sessions')

// Sessions
const sessions = ref([])
const loading = ref(false)
const { agents, loadAgents, getAgentIcon, getAgentName } = useAgents()
const selectedAgentId = ref('')
const showAgentSelector = ref(false)

// Tasks
const tasks = ref([])
const taskLoading = ref(false)
const taskDetailOpen = ref(false)
const selectedTask = ref(null)

const sessionsWithStatus = computed(() => {
  return sessions.value.map(s => ({
    ...s,
    running: props.runningSessionIds.has(s.id)
  }))
})

// Expose activeTab for parent component control
defineExpose({
  loadSessions,
  loadAgents,
  loadTasks,
  activeTab,
  setActiveTab: (tab) => { activeTab.value = tab }
})

async function loadSessions() {
  const isInitialLoad = sessions.value.length === 0
  if (isInitialLoad) {
    loading.value = true
  }
  try {
    const resp = await fetch('/api/ai/sessions')
    const data = await resp.json()
    sessions.value = data.sessions || []
  } catch (err) {
    console.error('Failed to load sessions:', err)
    if (isInitialLoad) {
      sessions.value = []
    }
  } finally {
    if (isInitialLoad) {
      loading.value = false
    }
  }
}

function selectSession(sessionId, backend) {
  emit('select', sessionId, backend)
  bottomSheetRef.value?.close()
}

function createSession(agentId) {
  showAgentSelector.value = false
  emit('create', agentId)
  bottomSheetRef.value?.close()
}

async function deleteSession(sessionId) {
  if (!confirm('确定删除此会话及其所有聊天记录?')) return
  const session = sessions.value.find(s => s.id === sessionId)
  emit('delete', sessionId, session?.backend)
}

// Task management functions
async function loadTasks() {
  taskLoading.value = true
  try {
    const resp = await fetch('/api/tasks')
    const data = await resp.json()
    tasks.value = data.tasks || []
  } catch (err) {
    console.error('Failed to load tasks:', err)
  } finally {
    taskLoading.value = false
  }
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
    await Promise.all([loadSessions(), loadAgents(), loadTasks()])
  }
})
</script>

<style scoped>
/* Tab bar - drag handle + segmented control */
.tab-bar {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 6px 6px 4px;
  gap: 4px;
  flex-shrink: 0;
}

.drag-handle {
  width: 28px;
  height: 3px;
  border-radius: 2px;
  background: var(--border-color, #ddd);
}

.tab-switcher {
  display: flex;
  width: 100%;
  gap: 0;
  background: var(--bg-tertiary, #f0f0f0);
  border-radius: 6px;
}

.tab-btn {
  flex: 1;
  padding: 4px 8px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-secondary, #666);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.tab-btn:hover {
  color: var(--text-primary, #1a1a1a);
}

.tab-btn.active {
  background: var(--bg-primary, #fff);
  color: var(--accent-color, #0066cc);
  box-shadow: 0 1px 2px rgba(0,0,0,0.08);
}

.tab-content {
  min-height: 0;
  overflow-y: auto;
  flex: 1;
}

.session-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px;
  min-height: 0;
}

.session-loading,
.session-empty {
  padding: 24px 12px;
  text-align: center;
  color: var(--text-muted, #999);
  font-size: 13px;
}

.session-item {
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s;
  border: 1px solid transparent;
}

.session-item:hover {
  background: var(--bg-secondary, #f8f9fa);
}

.session-item.active {
  background: var(--accent-bg, rgba(0, 102, 204, 0.1));
  border-color: var(--accent-color, #0066cc);
}

.session-item-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.session-item-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.session-item-header {
  display: flex;
  align-items: center;
  gap: 6px;
}

.session-item-meta {
  display: flex;
  align-items: center;
  gap: 6px;
}

.session-item-title {
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  font-weight: 500;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item-agent {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
  background: var(--bg-tertiary, #e9ecef);
  color: var(--text-secondary, #495057);
}

.session-item-backend {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
  background: rgba(0, 102, 204, 0.1);
  color: var(--accent-color, #0066cc);
  text-transform: lowercase;
}

.session-item-model {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
  background: rgba(100, 100, 100, 0.08);
  color: var(--text-muted, #999);
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item.active .session-item-title {
  color: var(--accent-color, #0066cc);
}

.session-item-unread {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 8px;
  font-weight: 600;
  background: #ef4444;
  color: #fff;
  flex-shrink: 0;
  min-width: 14px;
  text-align: center;
}

.session-item-status.running {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  color: #22c55e;
  font-weight: 500;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.session-item.running {
  background: rgba(34, 197, 94, 0.05);
}

.session-item-time {
  font-size: 11px;
  color: var(--text-muted, #999);
}

.session-item-delete {
  width: 22px;
  height: 22px;
  border: none;
  background: none;
  font-size: 16px;
  color: var(--text-muted, #999);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.15s;
  flex-shrink: 0;
}

.session-item-delete:hover {
  color: #dc3545;
  background: var(--bg-tertiary, #f0f0f0);
}

.session-footer {
  display: flex;
  gap: 8px;
  width: 100%;
}

.create-btn {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 6px 12px;
  border: none;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  background: var(--accent-color, #0066cc);
  color: #fff;
  transition: background 0.15s;
}

.create-btn:hover {
  background: #0055aa;
}

/* Agent selector modal */
.agent-selector-modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1001;
}

.agent-selector-content {
  background: var(--bg-primary, #fff);
  border-radius: 12px;
  padding: 20px;
  max-width: 320px;
  width: 90%;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.agent-selector-header {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #1a1a1a);
  margin-bottom: 16px;
  text-align: center;
}

.agent-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.agent-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 8px;
  background: var(--bg-primary, #fff);
  cursor: pointer;
  transition: all 0.15s;
  text-align: left;
}

.agent-option:hover {
  background: var(--bg-secondary, #f8f9fa);
  border-color: var(--accent-color, #0066cc);
}

.agent-option.selected {
  background: rgba(0, 102, 204, 0.1);
  border-color: var(--accent-color, #0066cc);
}

.agent-option-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.agent-option-detail {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.agent-option-name {
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  font-weight: 500;
}

.agent-option-specialty {
  font-size: 11px;
  color: var(--text-secondary, #666);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-option-tags {
  display: flex;
  gap: 4px;
  margin-top: 2px;
}

.agent-tag {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
}

.backend-tag {
  background: rgba(0, 102, 204, 0.1);
  color: var(--accent-color, #0066cc);
  text-transform: lowercase;
}

.model-tag {
  background: rgba(100, 100, 100, 0.08);
  color: var(--text-muted, #999);
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.close-selector-btn {
  width: 100%;
  padding: 8px;
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 6px;
  background: var(--bg-primary, #fff);
  color: var(--text-secondary, #495057);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}

.close-selector-btn:hover {
  background: var(--bg-secondary, #f8f9fa);
  color: var(--text-primary, #1a1a1a);
}

/* Task List Styles */
.task-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px;
  min-height: 0;
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

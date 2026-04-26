<template>
  <BottomSheet ref="bottomSheetRef" :open="open" compact title="会话" @close="$emit('close')">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <rect x="3" y="6" width="18" height="12" rx="2"/><line x1="12" y1="2" x2="12" y2="6"/><circle cx="9" cy="12" r="1" fill="currentColor"/><circle cx="15" cy="12" r="1" fill="currentColor"/><line x1="1" y1="10" x2="3" y2="10"/><line x1="1" y1="14" x2="3" y2="14"/><line x1="21" y1="10" x2="23" y2="10"/><line x1="21" y1="14" x2="23" y2="14"/>
      </svg>
      <span class="bs-header-title">会话</span>
    </template>

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

    <template #footer>
      <div class="session-footer">
        <button class="create-btn" @click="showAgentSelector = true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          新建会话
        </button>
      </div>
    </template>
  </BottomSheet>

  <!-- Agent selector dialog -->
  <ModalDialog :open="showAgentSelector" title="选择智能体" max-width="320px" @close="showAgentSelector = false">
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
    <template #footer>
      <button class="btn btn-secondary" @click="showAgentSelector = false">取消</button>
    </template>
  </ModalDialog>
</template>

<script setup>
import { ref, watch, computed } from 'vue'
import BottomSheet from './BottomSheet.vue'
import ModalDialog from './ModalDialog.vue'
import { useAgents } from '@/composables/useAgents.ts'
import { formatRelativeTime } from '@/utils/helpers.ts'

const props = defineProps({
  open: Boolean,
  currentSessionId: String,
  runningSessionIds: { type: Set, default: () => new Set() },
})

const emit = defineEmits(['close', 'select', 'create', 'delete'])

const bottomSheetRef = ref(null)
const sessions = ref([])
const loading = ref(false)
const { agents, loadAgents, getAgentIcon, getAgentName } = useAgents()
const selectedAgentId = ref('')
const showAgentSelector = ref(false)

const sessionsWithStatus = computed(() => {
  return sessions.value.map(s => ({
    ...s,
    running: props.runningSessionIds.has(s.id)
  }))
})

defineExpose({ loadSessions })

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

watch(() => props.open, async (val) => {
  if (val) {
    await Promise.all([loadSessions(), loadAgents()])
  }
})
</script>

<style scoped>
.session-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px;
  min-height: 0;
  overflow-y: auto;
  flex: 1;
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

/* Agent selector content */
.agent-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  overflow-y: auto;
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

.btn-secondary {
  padding: 5px 14px;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-primary, #1a1a1a);
  transition: background 0.15s;
}

.btn-secondary:hover { background: #e0e0e0; }
</style>

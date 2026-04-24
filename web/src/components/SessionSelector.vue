<template>
  <div v-if="open" class="session-overlay" @click="close">
    <div class="session-drawer" @click.stop>
      <div class="session-header">
        <span class="session-title">选择会话</span>
        <button class="session-close" @click="close">×</button>
      </div>

      <div class="session-list">
        <div v-if="loading" class="session-empty">加载中...</div>
        <div v-else-if="sessions.length === 0" class="session-empty">暂无会话</div>
        <div
          v-for="session in sessions"
          :key="session.id"
          class="session-item"
          :class="{ active: session.id === currentSessionId }"
          @click="selectSession(session.id)"
        >
          <div class="session-item-main">
            <span class="session-item-title">{{ session.title }}</span>
            <span class="session-item-time">{{ formatTime(session.updatedAt) }}</span>
          </div>
        </div>
      </div>

      <div class="session-footer">
        <button class="session-create-btn" @click="createSession">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          新会话
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  open: Boolean,
  currentSessionId: String,
})

const emit = defineEmits(['close', 'select', 'create'])

const sessions = ref([])
const loading = ref(false)

async function loadSessions() {
  loading.value = true
  try {
    const resp = await fetch('/api/ai/sessions')
    const data = await resp.json()
    sessions.value = data.sessions || []
  } catch (err) {
    console.error('Failed to load sessions:', err)
    sessions.value = []
  } finally {
    loading.value = false
  }
}

function close() {
  emit('close')
}

function selectSession(sessionId) {
  emit('select', sessionId)
  close()
}

async function createSession() {
  emit('create')
  close()
}

function formatTime(date) {
  if (!date) return ''
  const d = new Date(date)
  const now = new Date()
  const diff = now - d
  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(diff / 86400000)

  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  if (hours < 24) return `${hours}小时前`
  if (days < 7) return `${days}天前`
  return d.toLocaleDateString('zh-CN')
}

watch(() => props.open, async (val) => {
  if (val) {
    await loadSessions()
  }
})
</script>

<style scoped>
.session-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 1000;
  display: flex;
  align-items: flex-end;
  animation: fadeIn 0.2s;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.session-drawer {
  width: 100%;
  max-width: 400px;
  max-height: 70vh;
  background: var(--bg-primary, #fff);
  border-radius: 16px 16px 0 0;
  display: flex;
  flex-direction: column;
  animation: slideUp 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  margin: 0 auto;
}

@keyframes slideUp {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}

.session-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
}

.session-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary, #1a1a1a);
}

.session-close {
  width: 28px;
  height: 28px;
  border: none;
  background: none;
  font-size: 24px;
  color: var(--text-muted, #999);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: background 0.15s;
}

.session-close:hover {
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-primary, #1a1a1a);
}

.session-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.session-empty {
  padding: 32px 16px;
  text-align: center;
  color: var(--text-muted, #999);
  font-size: 14px;
}

.session-item {
  padding: 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.15s;
  margin-bottom: 4px;
}

.session-item:hover {
  background: var(--bg-secondary, #f8f9fa);
}

.session-item.active {
  background: var(--accent-bg, rgba(0, 102, 204, 0.1));
}

.session-item-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.session-item-title {
  flex: 1;
  font-size: 15px;
  color: var(--text-primary, #1a1a1a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item.active .session-item-title {
  font-weight: 500;
  color: var(--accent-color, #0066cc);
}

.session-item-time {
  font-size: 12px;
  color: var(--text-muted, #999);
  white-space: nowrap;
}

.session-footer {
  padding: 12px;
  border-top: 1px solid var(--border-color, #e5e5e5);
}

.session-create-btn {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px;
  border: none;
  background: var(--accent-color, #0066cc);
  color: #fff;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
}

.session-create-btn:hover {
  background: #0055aa;
}
</style>

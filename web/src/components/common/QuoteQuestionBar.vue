<template>
  <Transition name="quote-bar">
    <div v-if="visible && quoteData" class="quote-question-bar">
      <!-- Collapsed: preview + button -->
      <div v-if="!expanded" class="quote-bar-row">
        <div class="quote-bar-preview">
          <span class="quote-bar-icon">💬</span>
          <span class="quote-bar-text">{{ previewText }}</span>
        </div>
        <button class="quote-bar-btn" @click="expand">
          引用提问
        </button>
      </div>

      <!-- Expanded: session info + input + send -->
      <div v-else class="quote-bar-expanded">
        <div class="qq-session" @click="showSessionPicker = true">
          <span class="qq-session-icon">{{ sessionIcon }}</span>
          <span class="qq-session-name">{{ sessionName }}</span>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
            <polyline points="6 9 12 15 18 9"/>
          </svg>
        </div>
        <div class="qq-input-row">
          <textarea
            ref="inputRef"
            v-model="inputText"
            class="qq-input"
            rows="2"
            placeholder="输入你的问题..."
            @keydown.enter.meta="handleSend"
            @keydown.enter.ctrl="handleSend"
          />
          <div class="qq-actions">
            <button class="qq-action-btn qq-cancel-btn" @click="collapse" title="取消">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
            <button class="qq-action-btn qq-send-btn" :disabled="!canSend" @click="handleSend" title="发送">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                <line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  </Transition>

  <!-- Session picker overlay -->
  <Teleport to="body">
    <div v-if="showSessionPicker" class="qq-picker-overlay" @click="showSessionPicker = false">
      <div class="qq-picker" @click.stop>
        <div class="qq-picker-header">选择会话</div>
        <div class="qq-picker-list">
          <div v-if="loadingSessions" class="qq-picker-empty">加载中...</div>
          <div v-else-if="sessions.length === 0" class="qq-picker-empty">暂无会话</div>
          <div
            v-for="s in sessions"
            :key="s.id"
            class="qq-picker-item"
            :class="{ active: s.id === selectedSessionId }"
            @click="pickSession(s.id)"
          >
            <span class="qq-picker-item-title">{{ s.title || '新会话' }}</span>
            <span class="qq-picker-item-time">{{ formatTime(s.updatedAt) }}</span>
          </div>
        </div>
        <div class="qq-picker-footer">
          <button class="qq-picker-create" @click="createAndPick">+ 新会话</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps({
  visible: Boolean,
  quoteData: Object,
  sessionIcon: { type: String, default: '🤖' },
  sessionName: { type: String, default: 'AI 对话' },
  currentSessionId: { type: String, default: '' },
})
const emit = defineEmits(['send', 'close'])

const expanded = ref(false)
const inputText = ref('')
const inputRef = ref(null)
const showSessionPicker = ref(false)
const sessions = ref([])
const loadingSessions = ref(false)
const selectedSessionId = ref('')

const previewText = computed(() => {
  if (!props.quoteData) return ''
  const text = props.quoteData.text || ''
  return text.length > 60 ? text.slice(0, 60) + '…' : text
})

const canSend = computed(() => inputText.value.trim().length > 0)

// Sync selected session with prop
watch(() => props.currentSessionId, (id) => {
  if (!selectedSessionId.value) selectedSessionId.value = id
}, { immediate: true })

// Load sessions when picker opens
watch(showSessionPicker, async (val) => {
  if (val) {
    loadingSessions.value = true
    try {
      const resp = await fetch('/api/ai/sessions')
      const data = await resp.json()
      sessions.value = data.sessions || []
    } catch {
      sessions.value = []
    } finally {
      loadingSessions.value = false
    }
  }
})

// Reset when bar hides
watch(() => props.visible, (val) => {
  if (!val) {
    expanded.value = false
    inputText.value = ''
    showSessionPicker.value = false
  }
})

async function expand() {
  expanded.value = true
  selectedSessionId.value = props.currentSessionId
  await nextTick()
  inputRef.value?.focus()
}

function collapse() {
  expanded.value = false
  inputText.value = ''
  emit('close')
}

function pickSession(sessionId) {
  selectedSessionId.value = sessionId
  showSessionPicker.value = false
}

async function createAndPick() {
  try {
    const resp = await fetch('/api/ai/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    })
    const data = await resp.json()
    if (data.ok && data.sessionId) {
      selectedSessionId.value = data.sessionId
      showSessionPicker.value = false
    }
  } catch (err) {
    console.error('Failed to create session:', err)
  }
}

function handleSend() {
  if (!canSend.value) return
  emit('send', inputText.value, selectedSessionId.value || undefined)
  expanded.value = false
  inputText.value = ''
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
</script>

<style scoped>
.quote-question-bar {
  position: fixed;
  top: calc(48px + env(safe-area-inset-top, 0px));
  left: 8px;
  right: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  box-shadow: var(--shadow-md);
  z-index: 2400;
  max-width: 400px;
  margin: 0 auto;
  overflow: hidden;
}

/* Collapsed row */
.quote-bar-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
}

.quote-bar-preview {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.quote-bar-icon {
  flex-shrink: 0;
  font-size: 14px;
}

.quote-bar-text {
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.quote-bar-btn {
  flex-shrink: 0;
  padding: 6px 14px;
  border: none;
  border-radius: 8px;
  background: var(--accent-color);
  color: #fff;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.15s;
}

.quote-bar-btn:active {
  opacity: 0.8;
}

/* Expanded */
.quote-bar-expanded {
  padding: 8px 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.qq-session {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  background: var(--bg-tertiary);
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s;
}

.qq-session:active {
  background: var(--bg-secondary);
}

.qq-session-icon {
  font-size: 12px;
}

.qq-session-name {
  flex: 1;
  font-size: 12px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.qq-input-row {
  display: flex;
  align-items: flex-end;
  gap: 6px;
}

.qq-input {
  flex: 1;
  padding: 6px 8px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
  resize: none;
  min-height: 48px;
  max-height: 80px;
  outline: none;
  font-family: inherit;
  line-height: 1.4;
}

.qq-input:focus {
  border-color: var(--accent-color);
}

.qq-actions {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex-shrink: 0;
}

.qq-action-btn {
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: opacity 0.15s;
}

.qq-action-btn:active {
  opacity: 0.7;
}

.qq-cancel-btn {
  background: var(--bg-tertiary);
  color: var(--text-muted);
}

.qq-send-btn {
  background: var(--accent-color);
  color: #fff;
}

.qq-send-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

/* Transition */
.quote-bar-enter-active {
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.quote-bar-leave-active {
  transition: all 0.15s ease-in;
}

.quote-bar-enter-from,
.quote-bar-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

/* Session picker */
.qq-picker-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 3000;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.qq-picker {
  width: 100%;
  max-width: 400px;
  max-height: 60vh;
  background: var(--bg-primary);
  border-radius: 16px 16px 0 0;
  display: flex;
  flex-direction: column;
  animation: qq-picker-up 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes qq-picker-up {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}

.qq-picker-header {
  padding: 14px 16px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-color);
}

.qq-picker-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.qq-picker-empty {
  padding: 24px;
  text-align: center;
  color: var(--text-muted);
  font-size: 14px;
}

.qq-picker-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.15s;
}

.qq-picker-item:active {
  background: var(--bg-tertiary);
}

.qq-picker-item.active {
  background: var(--accent-bg, rgba(0, 102, 204, 0.1));
}

.qq-picker-item-title {
  flex: 1;
  font-size: 14px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.qq-picker-item.active .qq-picker-item-title {
  color: var(--accent-color);
  font-weight: 500;
}

.qq-picker-item-time {
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
  margin-left: 8px;
}

.qq-picker-footer {
  padding: 10px 12px;
  border-top: 1px solid var(--border-color);
}

.qq-picker-create {
  width: 100%;
  padding: 10px;
  border: none;
  border-radius: 8px;
  background: var(--accent-color);
  color: #fff;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
}

.qq-picker-create:active {
  opacity: 0.85;
}
</style>

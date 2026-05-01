<template>
  <Transition name="quote-bar">
    <div v-if="visible && quoteData" ref="barRef" class="quote-question-bar">
      <!-- Preview row — always visible when bar is shown -->
      <div class="quote-bar-row" @click="!expanded && expand()">
        <div class="quote-bar-preview">
          <span class="quote-bar-icon">💬</span>
          <span class="quote-bar-text">{{ previewText }}</span>
        </div>
        <button v-if="!expanded" class="quote-bar-btn" @click.stop="expand">
          对话
        </button>
        <button v-else class="quote-bar-btn quote-bar-btn-collapse" @click.stop="collapse" title="收起">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <polyline points="18 15 12 9 6 15"/>
          </svg>
        </button>
      </div>

      <!-- Expanded: session info + input + send -->
      <div v-if="expanded" class="quote-bar-expanded">
        <div class="qq-session" @click="openSessionDrawer">
          <span class="qq-session-label">{{ sessionLabel }}</span>
          <div class="qq-session-title">
            <HeaderMarquee :text="displaySessionTitle">{{ displaySessionTitle }}</HeaderMarquee>
          </div>
          <svg class="qq-session-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
            <polyline points="6 9 12 15 18 9"/>
          </svg>
        </div>
        <div class="qq-input-container">
          <div class="qq-input-row">
            <button v-if="inputText" class="qq-clear-btn" @click="inputText = ''; collapseTextarea()" title="清空">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
              </svg>
            </button>
            <textarea
              ref="inputRef"
              v-model="inputText"
              class="qq-textarea"
              rows="1"
              placeholder="输入你的问题..."
              @keydown.enter.exact.prevent="handleSend"
              @input="autoResizeTextarea"
            />
            <button class="qq-send-btn" :class="{ disabled: !canSend }" @click="handleSend" title="发送">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                <line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import HeaderMarquee from '@/components/common/HeaderMarquee.vue'

const props = defineProps({
  visible: Boolean,
  quoteData: Object,
  sessionLabel: { type: String, default: 'AI 对话' },
  sessionTitle: { type: String, default: '' },
  currentSessionId: { type: String, default: '' },
})
const emit = defineEmits(['send', 'close', 'pin', 'unpin', 'open-sessions'])

const expanded = ref(false)
const inputText = ref('')
const inputRef = ref(null)
const barRef = ref(null)

const previewText = computed(() => {
  if (!props.quoteData) return ''
  const text = props.quoteData.text || ''
  return text.length > 60 ? text.slice(0, 60) + '…' : text
})

const canSend = computed(() => inputText.value.trim().length > 0)

const displaySessionTitle = computed(() => props.sessionTitle || '新会话')

// Reset when bar hides
watch(() => props.visible, (val) => {
  if (!val) {
    expanded.value = false
    inputText.value = ''
  }
})

// Click outside to close
function onPointerDown(e) {
  if (!props.visible) return
  if (!barRef.value) return
  // Don't close if clicking inside the bar
  if (barRef.value.contains(e.target)) return
  // Don't close if clicking inside a BottomSheet (bs-overlay/bs-panel) or ModalDialog
  if (e.target.closest('.bs-overlay, .bs-panel, .modal-dialog')) return
  emit('close')
}

onMounted(() => {
  document.addEventListener('pointerdown', onPointerDown, true)
})

onUnmounted(() => {
  document.removeEventListener('pointerdown', onPointerDown, true)
})

async function expand() {
  emit('pin')  // Pin bar so selection loss won't auto-hide it
  expanded.value = true
  await nextTick()
  inputRef.value?.focus()
}

function collapse() {
  expanded.value = false
  inputText.value = ''
  collapseTextarea()
  // Don't close the bar — just collapse the input area, keep the preview visible
  // Reset barPinned so selection changes can auto-hide the bar again
  emit('unpin')
}

function autoResizeTextarea() {
  const el = inputRef.value
  if (!el) return
  el.style.height = 'auto'
  const lineHeight = parseFloat(getComputedStyle(el).lineHeight) || 20
  const maxHeight = lineHeight * 3 + 8
  el.style.height = Math.min(el.scrollHeight, maxHeight) + 'px'
}

function collapseTextarea() {
  const el = inputRef.value
  if (!el) return
  el.style.height = 'auto'
}

function openSessionDrawer() {
  // Delegate to the existing SessionDrawer via event
  emit('open-sessions')
}

function handleSend() {
  if (!canSend.value) return
  emit('send', inputText.value)
  // Keep the bar visible after send, just collapse the input area
  expanded.value = false
  inputText.value = ''
  collapseTextarea()
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

.quote-bar-btn-collapse {
  padding: 4px 6px;
  border-radius: 50%;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}

.quote-bar-btn-collapse:active {
  background: var(--bg-secondary);
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

.qq-session-label {
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
}

.qq-session-title {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  color: var(--text-secondary);
}

.qq-session-arrow {
  flex-shrink: 0;
  color: var(--text-muted);
}

/* Input container — same style as ChatInputBar */
.qq-input-container {
  display: flex;
  flex-direction: column;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  overflow: hidden;
  transition: border-color 0.2s;
}

.qq-input-container:focus-within {
  border-color: var(--accent-color);
}

.qq-input-row {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  padding: 4px 6px 6px;
}

.qq-textarea {
  flex: 1;
  padding: 4px 8px;
  border: none;
  background: transparent;
  color: var(--text-primary);
  font-size: 14px;
  line-height: 20px;
  outline: none;
  resize: none;
  overflow-y: auto;
  min-height: 28px;
  max-height: 68px;
  font-family: inherit;
}

.qq-textarea::placeholder {
  color: var(--text-muted);
}

.qq-clear-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted);
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
  flex-shrink: 0;
  align-self: flex-end;
}

.qq-clear-btn:hover {
  color: var(--danger-color);
  background: color-mix(in srgb, var(--danger-color) 8%, transparent);
}

.qq-send-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: var(--accent-color);
  color: #fff;
  border: none;
  border-radius: 50%;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
  flex-shrink: 0;
}

.qq-send-btn:hover {
  background: var(--accent-hover);
}

.qq-send-btn:active {
  opacity: 0.8;
}

.qq-send-btn.disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
</style>

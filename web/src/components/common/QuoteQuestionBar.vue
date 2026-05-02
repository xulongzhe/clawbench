<template>
  <Transition name="quote-bar">
    <div v-if="visible && quoteData" ref="barRef" class="quote-question-bar">

      <!-- Collapsed: quoted snippet (single-line) + 对话 button -->
      <div v-if="!expanded" class="quote-bar-row" @click="expand()">
        <div class="qq-quoted-snippet qq-quoted-snippet--inline">
          <svg class="qq-quoted-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
          </svg>
          <span class="qq-quoted-text qq-quoted-text--single">{{ fullQuoteText }}</span>
        </div>
        <button class="quote-bar-btn" @click.stop="expand">
          对话
        </button>
      </div>

      <!-- Expanded: session selector (top) + quoted snippet + input -->
      <div v-else class="quote-bar-expanded">
        <!-- Top: session selector -->
        <div class="qq-top-row">
          <div class="qq-session" @click="openSessionDrawer">
            <span class="qq-session-label">{{ sessionLabel }}</span>
            <div class="qq-session-title">
              <HeaderMarquee :text="displaySessionTitle">{{ displaySessionTitle }}</HeaderMarquee>
            </div>
            <svg class="qq-session-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
              <polyline points="6 9 12 15 18 9"/>
            </svg>
          </div>
        </div>

        <!-- Quoted snippet -->
        <div class="qq-quoted-snippet">
          <svg class="qq-quoted-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
          </svg>
          <span class="qq-quoted-text">{{ fullQuoteText }}</span>
        </div>

        <!-- Input -->
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

const fullQuoteText = computed(() => {
  if (!props.quoteData) return ''
  const text = props.quoteData.text || ''
  // Show up to 3 lines when expanded; single-line will be truncated via CSS
  return text.length > 150 ? text.slice(0, 150) + '…' : text
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
  emit('pin')
  expanded.value = true
  await nextTick()
  inputRef.value?.focus()
}

function collapse() {
  expanded.value = false
  inputText.value = ''
  collapseTextarea()
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
  emit('open-sessions')
}

function handleSend() {
  if (!canSend.value) return
  emit('send', inputText.value)
  expanded.value = false
  inputText.value = ''
  collapseTextarea()
}
</script>

<style scoped>
.quote-question-bar {
  position: fixed;
  top: calc(var(--header-height, 40px) + 8px + env(safe-area-inset-top, 0px));
  left: 8px;
  right: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 20px;
  box-shadow: var(--shadow-md);
  z-index: 2400;
  max-width: 400px;
  margin: 0 auto;
  overflow: hidden;
}

/* ===== Collapsed row ===== */
.quote-bar-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 8px 10px;
  cursor: pointer;
  transition: background 0.15s;
}

.quote-bar-row:active {
  background: var(--bg-tertiary);
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

/* ===== Expanded panel ===== */
.quote-bar-expanded {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 10px;
}

/* Top row: session selector */
.qq-top-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.qq-session {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 8px;
  background: var(--bg-tertiary);
  border-radius: 20px;
  cursor: pointer;
  transition: background 0.15s;
  flex: 1;
  min-width: 0;
}

.qq-session:active {
  background: var(--bg-primary);
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

/* Quoted snippet block */
.qq-quoted-snippet {
  display: flex;
  align-items: flex-start;
  gap: 5px;
  padding: 6px 8px;
  background: var(--bg-tertiary);
  border-left: 2px solid var(--accent-color);
  border-radius: 0 4px 4px 0;
  margin: 0 2px;
  flex: 1;
  min-width: 0;
}

/* Collapsed inline variant — single row, no flex-start */
.qq-quoted-snippet--inline {
  align-items: center;
  padding: 5px 8px;
  margin: 0;
}

.qq-quoted-icon {
  flex-shrink: 0;
  color: var(--accent-color);
  opacity: 0.6;
  margin-top: 1px;
}

.qq-quoted-snippet--inline .qq-quoted-icon {
  margin-top: 0;
}

.qq-quoted-text {
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary);
  word-break: break-all;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* Collapsed single-line variant */
.qq-quoted-text--single {
  -webkit-line-clamp: 1;
  white-space: nowrap;
  word-break: normal;
}

/* Input container — capsule style */
.qq-input-container {
  display: flex;
  flex-direction: column;
  background: var(--bg-tertiary);
  border: none;
  border-radius: 20px;
  overflow: hidden;
  transition: background 0.2s, box-shadow 0.2s;
}

.qq-input-container:focus-within {
  background: var(--bg-primary);
  box-shadow: 0 0 0 1px var(--accent-color);
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

/* ===== Transitions ===== */
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

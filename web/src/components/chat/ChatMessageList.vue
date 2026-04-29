<template>
  <div class="chat-messages" id="aiChatMessages" ref="messagesRef" @click="handleChatClick" @scroll="handleScroll">
    <!-- Session switch indicator -->
    <Transition name="session-indicator">
      <div v-if="indicatorText" class="session-switch-indicator" :class="indicatorDirection">
        <div class="session-indicator-arrow">
          <svg v-if="indicatorDirection === 'left'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <polyline points="9 18 15 12 9 6"/>
          </svg>
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <polyline points="15 18 9 12 15 6"/>
          </svg>
        </div>
        <span class="session-indicator-text">{{ indicatorText }}</span>
      </div>
    </Transition>

    <!-- Loading indicator for lazy load -->
    <div v-if="loadingMore" class="chat-load-more">
      <span class="chat-load-spinner"></span>
      <span>加载中...</span>
    </div>

    <div class="chat-messages-list">
      <div v-if="messages.length === 0" class="chat-empty">
      <template v-if="currentAgent">
        <div class="agent-welcome">
          <span class="agent-welcome-icon">{{ currentAgent.icon }}</span>
          <div class="agent-welcome-info">
            <span class="agent-welcome-name">{{ currentAgent.name }}</span>
            <span class="agent-welcome-specialty">{{ currentAgent.specialty }}</span>
            <div class="agent-welcome-tags">
              <span class="agent-welcome-tag agent-welcome-backend">{{ currentAgent.backend }}</span>
              <span class="agent-welcome-tag agent-welcome-model">{{ currentAgent.model }}</span>
            </div>
          </div>
        </div>
        <span class="agent-welcome-hint">发送消息开始对话</span>
      </template>
      <span v-else>发送消息开始与 AI 对话</span>
    </div>

    <ChatMessageItem
      v-for="(msg, i) in messages"
      :key="`${msg.createdAt || ''}-${i}`"
      :msg="msg"
      :index="i"
      :expandedTools="expandedTools"
      :blockProposals="blockProposals"
      :agents="agents"
      :renderedContent="renderedContents[i]"
      :shouldCollapse="isCollapsed(i, msg)"
      @toggle-tool="$emit('toggle-tool', $event)"
      @show-metadata="$emit('show-metadata', $event)"
      @file-tag-click="$emit('file-tag-click', $event)"
      @expand="handleExpand"
    />
    </div>
  </div>
</template>

<script setup>
import { ref, nextTick, inject, computed, watch } from 'vue'
import ChatMessageItem from './ChatMessageItem.vue'
import { useDoubleClickCopy } from '@/composables/useDoubleClickCopy.ts'
import { useFilePathAnnotation } from '@/composables/useFilePathAnnotation.ts'

const props = defineProps({
  messages: Array,
  expandedTools: Object,
  blockProposals: Object,
  agents: Array,
  currentAgent: Object,
  renderedContents: Array,
  hasMore: Boolean,
  loadingMore: Boolean,
  indicatorText: String,
  indicatorDirection: String,
})

const emit = defineEmits(['toggle-tool', 'show-metadata', 'file-tag-click', 'file-open', 'load-more'])

const messagesRef = ref(null)
const { handleDblClick } = useDoubleClickCopy()
const { openFilePath } = useFilePathAnnotation()

// Track manually expanded message indices
const expandedSet = ref(new Set())

// Reset expanded state when messages change identity (session switch / reload)
watch(() => props.messages, () => {
  expandedSet.value = new Set()
})

// Compute the last round: last assistant message + its preceding user message
const lastRoundIndices = computed(() => {
  const msgs = props.messages
  if (!msgs || msgs.length === 0) return new Set()

  // Find last assistant message index
  let lastAssistantIdx = -1
  for (let i = msgs.length - 1; i >= 0; i--) {
    if (msgs[i].role === 'assistant') {
      lastAssistantIdx = i
      break
    }
  }

  const indices = new Set()
  if (lastAssistantIdx >= 0) {
    indices.add(lastAssistantIdx)
    // Find the preceding user message
    for (let i = lastAssistantIdx - 1; i >= 0; i--) {
      if (msgs[i].role === 'user') {
        indices.add(i)
        break
      }
    }
  } else {
    // No assistant message — expand last user message
    for (let i = msgs.length - 1; i >= 0; i--) {
      if (msgs[i].role === 'user') {
        indices.add(i)
        break
      }
    }
  }

  return indices
})

function isCollapsed(index, msg) {
  // Last round is always fully expanded (no collapse suggestion)
  if (lastRoundIndices.value.has(index)) return false
  // Manually expanded — don't suggest collapse
  if (expandedSet.value.has(index)) return false
  // Everything else: suggest collapse (ChatMessageItem decides whether content actually overflows)
  return true
}

function handleExpand(index) {
  expandedSet.value = new Set([...expandedSet.value, index])
}

// Inject bottomSheetRef from parent for closing
const chatUI = inject('chatUI', {})

function handleChatClick(event) {
  const btn = (event.target).closest('.chat-file-open-btn')
  if (btn) {
    event.preventDefault()
    event.stopPropagation()
    const filePath = btn.getAttribute('data-file-path')
    if (filePath) {
      openFilePath(filePath)
      chatUI.closeSheet?.()
    }
    return
  }
  handleDblClick(event, (href) => {
    openFilePath(href)
    chatUI.closeSheet?.()
  })
}

let loadMorePending = false
// Track whether the user is at the bottom of the chat.
// When the user scrolls back to the bottom during streaming, auto-scroll resumes.
let isAtBottom = true

const NEAR_BOTTOM_THRESHOLD = 60

function handleScroll() {
  if (!messagesRef.value) return
  const el = messagesRef.value

  // Update isAtBottom state based on current scroll position
  isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_BOTTOM_THRESHOLD

  if (loadMorePending) return
  if (!props.hasMore || props.loadingMore) return
  if (el.scrollTop < 50) {
    loadMorePending = true
    emit('load-more')
    nextTick(() => { loadMorePending = false })
  }
}

function scrollToBottom(force = false) {
  nextTick(() => {
    if (!messagesRef.value) return
    const el = messagesRef.value
    if (force || isAtBottom) {
      el.scrollTop = el.scrollHeight
      isAtBottom = true
      // After forced scroll, re-check after a short delay to handle
      // async content rendering (Mermaid, KaTeX) that changes height
      if (force) {
        setTimeout(() => {
          if (messagesRef.value) {
            messagesRef.value.scrollTop = messagesRef.value.scrollHeight
            isAtBottom = true
          }
        }, 300)
      }
    }
  })
}

defineExpose({
  scrollToBottom,
  messagesRef,
})
</script>

<style scoped>
.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 12px 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  position: relative;
}

/* Message list container */
.chat-messages-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.chat-empty {
  text-align: center;
  padding: 32px 16px;
  color: var(--text-muted);
  font-size: 13px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.agent-welcome {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  max-width: 280px;
  width: 100%;
  text-align: left;
}

.agent-welcome-icon {
  font-size: 28px;
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-tertiary);
  border-radius: 10px;
}

.agent-welcome-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.agent-welcome-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.agent-welcome-specialty {
  font-size: 11px;
  color: var(--text-secondary);
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.agent-welcome-tags {
  display: flex;
  gap: 4px;
  margin-top: 2px;
}

.agent-welcome-tag {
  font-size: 9px;
  padding: 1px 6px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
}

.agent-welcome-backend {
  background: rgba(0, 102, 204, 0.1);
  color: var(--accent-color);
}

.agent-welcome-model {
  background: rgba(100, 100, 100, 0.08);
  color: var(--text-muted);
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-welcome-hint {
  font-size: 12px;
  color: var(--text-muted);
  opacity: 0.7;
}

/* Lazy load indicator */
.chat-load-more {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 0;
  font-size: 12px;
  color: var(--text-muted);
}

.chat-load-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border-color);
  border-top-color: var(--text-secondary);
  border-radius: 50%;
  animation: tool-spin 0.6s linear infinite;
}

@keyframes tool-spin {
  to { transform: rotate(360deg); }
}

/* Session switch indicator */
.session-switch-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 10px 20px;
  background: var(--bg-primary);
  color: var(--text-primary);
  border-radius: 24px;
  font-size: 13px;
  font-weight: 500;
  letter-spacing: 0.3px;
  position: sticky;
  top: 8px;
  z-index: 10;
  margin: 0 auto 12px;
  max-width: 260px;
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-md);
}

.session-indicator-arrow {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  background: var(--accent-color);
  color: #fff;
  border-radius: 50%;
  flex-shrink: 0;
}

.session-switch-indicator.left .session-indicator-arrow {
  animation: arrow-bounce-left 0.4s ease-out;
}

.session-switch-indicator.right .session-indicator-arrow {
  animation: arrow-bounce-right 0.4s ease-out;
}

.session-indicator-text {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-secondary);
}

@keyframes arrow-bounce-left {
  0% { transform: translateX(-8px); opacity: 0; }
  60% { transform: translateX(4px); }
  100% { transform: translateX(0); opacity: 1; }
}

@keyframes arrow-bounce-right {
  0% { transform: translateX(8px); opacity: 0; }
  60% { transform: translateX(-4px); }
  100% { transform: translateX(0); opacity: 1; }
}

.session-switch-indicator.left {
  animation: indicator-slide-left 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.session-switch-indicator.right {
  animation: indicator-slide-right 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}

@keyframes indicator-slide-left {
  from {
    opacity: 0;
    transform: translateX(30px) scale(0.9);
  }
  to {
    opacity: 1;
    transform: translateX(0) scale(1);
  }
}

@keyframes indicator-slide-right {
  from {
    opacity: 0;
    transform: translateX(-30px) scale(0.9);
  }
  to {
    opacity: 1;
    transform: translateX(0) scale(1);
  }
}

.session-indicator-enter-active {
  transition: opacity 0.15s ease-out;
}

.session-indicator-leave-active {
  transition: opacity 0.2s ease-in, transform 0.2s ease-in;
}

.session-indicator-enter-from {
  opacity: 0;
}

.session-indicator-leave-to {
  opacity: 0;
  transform: scale(0.95);
}
</style>

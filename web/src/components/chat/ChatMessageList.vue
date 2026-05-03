<template>
  <div class="chat-messages" id="aiChatMessages" ref="messagesRef" @click="handleChatClick" @scroll="handleScroll">
    <!-- Lazy load feedback -->
    <div class="chat-load-area">
      <Transition name="load-hint-fade">
        <div v-if="loadingMore" class="chat-load-more">
          <span class="chat-load-spinner"></span>
          <span>加载中...</span>
        </div>
        <div v-else-if="hasMore && remainingCount > 0" class="chat-load-hint" @click="emit('load-more')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <polyline points="18 15 12 9 6 15"/>
          </svg>
          <span>还有 {{ remainingCount }} 条更早消息</span>
        </div>
        <div v-else-if="showAllLoaded" class="chat-load-done">
          <span>已加载全部消息</span>
        </div>
      </Transition>
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
      :key="msg.id ? 'db-' + msg.id : 'local-' + i"
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
      @edit-task="$emit('edit-task', $event)"
      @send-message="$emit('send-message', $event)"
      @expand="handleExpand"
    />
    </div>

    <!-- Pending messages (queued while AI is generating) -->
    <div v-if="pendingMessages.length > 0" class="pending-messages-list">
      <PendingMessageItem
        v-for="(msg, i) in pendingMessages"
        :key="'pending-' + i"
        :msg="msg"
        :index="i"
        @remove="$emit('remove-pending', $event)"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, nextTick, inject, computed, watch } from 'vue'
import ChatMessageItem from './ChatMessageItem.vue'
import PendingMessageItem from './PendingMessageItem.vue'
import { useDoubleClickCopy } from '@/composables/useDoubleClickCopy.ts'
import { useFilePathAnnotation } from '@/composables/useFilePathAnnotation.ts'

const props = defineProps({
  messages: Array,
  expandedTools: Object,
  blockProposals: Object,
  agents: Array,
  currentAgent: Object,
  currentSessionId: String,
  renderedContents: Array,
  hasMore: Boolean,
  loadingMore: Boolean,
  totalMessages: { type: Number, default: 0 },
  pendingMessages: { type: Array, default: () => [] },
})

const emit = defineEmits(['toggle-tool', 'show-metadata', 'file-tag-click', 'file-open', 'load-more', 'edit-task', 'send-message', 'remove-pending'])

const messagesRef = ref(null)
const { handleDblClick } = useDoubleClickCopy()
const { openFilePath } = useFilePathAnnotation()

// How many older messages are not yet loaded
const remainingCount = computed(() => {
  if (!props.hasMore) return 0
  return Math.max(0, props.totalMessages - props.messages.length)
})

// "All loaded" brief hint: shown for 2s after last load completes with no more
const showAllLoaded = ref(false)
let allLoadedTimer = null

watch(() => props.hasMore, (hasMore, prevHasMore) => {
  // When transitioning from hasMore=true to hasMore=false (just finished loading all)
  if (!hasMore && prevHasMore && props.messages.length > 0) {
    showAllLoaded.value = true
    clearTimeout(allLoadedTimer)
    allLoadedTimer = setTimeout(() => { showAllLoaded.value = false }, 2000)
  }
})

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

/* Lazy load feedback area */
.chat-load-area {
  position: relative;
  min-height: 0;
}

.chat-load-more,
.chat-load-hint,
.chat-load-done {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 0;
  font-size: 12px;
  color: var(--text-muted);
}

.chat-load-hint {
  cursor: pointer;
  transition: color 0.15s, opacity 0.15s;
  -webkit-tap-highlight-color: transparent;
}

.chat-load-hint:active {
  opacity: 0.6;
}

@media (hover: hover) {
  .chat-load-hint:hover {
    color: var(--text-secondary);
  }
}

.chat-load-done {
  color: var(--text-muted);
  opacity: 0.7;
  font-size: 11px;
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

/* Transition for load hint switching */
.load-hint-fade-enter-active {
  transition: opacity 0.2s ease-out;
}
.load-hint-fade-leave-active {
  transition: opacity 0.15s ease-in;
}
.load-hint-fade-enter-from,
.load-hint-fade-leave-to {
  opacity: 0;
}

/* Pending messages list */
.pending-messages-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 4px;
}

</style>

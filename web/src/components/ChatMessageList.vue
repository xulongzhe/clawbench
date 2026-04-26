<template>
  <div class="chat-messages" id="aiChatMessages" ref="messagesRef" @click="handleChatClick">
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
      @toggle-tool="$emit('toggle-tool', $event)"
      @show-metadata="$emit('show-metadata', $event)"
      @file-tag-click="$emit('file-tag-click', $event)"
    />
  </div>
</template>

<script setup>
import { ref, nextTick, inject } from 'vue'
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
})

const emit = defineEmits(['toggle-tool', 'show-metadata', 'file-tag-click', 'file-open'])

const messagesRef = ref(null)
const { handleDblClick } = useDoubleClickCopy()
const { openFilePath } = useFilePathAnnotation()

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

function scrollToBottom(force = false) {
  nextTick(() => {
    if (!messagesRef.value) return
    const el = messagesRef.value
    if (force || el.scrollHeight - el.scrollTop - el.clientHeight < 60) {
      el.scrollTop = el.scrollHeight
    }
  })
}

defineExpose({
  scrollToBottom,
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
</style>

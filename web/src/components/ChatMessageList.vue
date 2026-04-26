<template>
  <div class="chat-messages" id="aiChatMessages" ref="messagesRef" @click="handleChatClick">
    <div v-if="messages.length === 0" class="chat-empty">
      <span>发送消息开始与 AI 对话</span>
    </div>

    <ChatMessageItem
      v-for="(msg, i) in messages"
      :key="`${msg.createdAt || ''}-${i}`"
      :msg="msg"
      :index="i"
      :expandedTools="expandedTools"
      :expandedThinking="expandedThinking"
      :blockProposals="blockProposals"
      :agents="agents"
      :renderedContent="renderedContents[i]"
      @toggle-tool="$emit('toggle-tool', $event)"
      @toggle-thinking="$emit('toggle-thinking', $event)"
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
  expandedThinking: Object,
  blockProposals: Object,
  agents: Array,
  renderedContents: Array,
})

const emit = defineEmits(['toggle-tool', 'toggle-thinking', 'show-metadata', 'file-tag-click', 'file-open'])

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
}
</style>

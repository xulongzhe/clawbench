<template>
  <div class="exec-detail-page">
    <!-- Header: breadcrumb -->
    <div class="exec-detail-header">
      <TaskBreadcrumb />
    </div>

    <!-- Scrollable message content -->
    <div class="exec-detail-content" ref="contentRef" @click="handleContentClick">
      <ChatMessageItem
        v-if="msgData"
        :msg="msgData"
        :index="0"
        :expandedTools="expandedTools"
        :blockTasks="{}"
        :blockAskQuestions="{}"
        :shouldCollapse="false"
        @toggle-tool="toggleTool"
        @show-tool-detail="handleShowToolDetail"
        @show-metadata="showMetadata"
        @task-card-click="() => {}"
      />
      <div v-else class="exec-detail-empty">{{ t('task.exec.noTextOutput') }}</div>
    </div>

    <!-- Tool Detail Overlay -->
    <ToolDetailOverlay
      :show="toolDetailOverlay.show"
      :toolName="toolDetailOverlay.name"
      :toolSummary="toolDetailOverlay.summary"
      :toolInputHtml="toolDetailOverlay.inputHtml"
      :toolOutputHtml="toolDetailOverlay.outputHtml"
      :toolStatus="toolDetailOverlay.status"
      :toolDone="toolDetailOverlay.done"
      @close="toolDetailOverlay.show = false"
      @file-open="handleFileOpenInOverlay"
    />

    <!-- Metadata Modal -->
    <ChatMetadataModal
      :show="metadataModal.show"
      :data="metadataModal.data"
      :backend="metadataModal.backend"
      :createdAt="metadataModal.createdAt"
      :relatedFile="metadataModal.relatedFile"
      :messageId="metadataModal.messageId"
      :sessionId="metadataModal.sessionId"
      :indexed="metadataModal.indexed"
      :formatDetailTime="chatRender.formatDetailTime"
      @close="metadataModal.show = false"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, provide, onUnmounted, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import TaskBreadcrumb from '@/components/task/TaskBreadcrumb.vue'
import ChatMessageItem from '@/components/chat/ChatMessageItem.vue'
import ToolDetailOverlay from '@/components/chat/ToolDetailOverlay.vue'
import ChatMetadataModal from '@/components/chat/ChatMetadataModal.vue'
import { useChatRender } from '@/composables/useChatRender.ts'
import { useAgents } from '@/composables/useAgents.ts'
import { useFilePathAnnotation } from '@/composables/useFilePathAnnotation.ts'
import { formatToolOutput } from '@/utils/renderToolDetail.ts'

const props = defineProps({
  execDetail: Object,
  taskName: String,
})

const emit = defineEmits(['close', 'open-file'])

const { t } = useI18n()
const theme = inject('theme', ref('light'))
const { openFilePath, verifyFilePaths } = useFilePathAnnotation()
const switchTab = inject('switchTab', () => {})

// ── Agents (for getAgentIcon/getAgentName) ──
const { agents: agentsList, getAgentIcon, getAgentName } = useAgents()

// ── ChatRender — full pipeline for markdown rendering ──
const messages = ref([])
const chatRender = useChatRender({ messages, theme, currentSessionId: ref('') })

// ── Provide dependencies that ChatMessageItem injects ──
provide('chatRender', {
  renderTextBlock: chatRender.renderTextBlock,
  formatMessageTime: chatRender.formatMessageTime,
  toolCallSummary: chatRender.toolCallSummary,
  formatToolInput: chatRender.formatToolInput,
  humanizeCron: chatRender.humanizeCron,
  repeatLabel: chatRender.repeatLabel,
  truncate: chatRender.truncate,
  hasImagesInContent: chatRender.hasImagesInContent,
})
provide('chatSession', { getAgentIcon, getAgentName })
provide('chatUI', { closeSheet: () => emit('close') })
provide('autoSpeech', { isActive: () => false, isGeneratingText: () => false, isPlayingAudio: () => false, speakText: () => {}, stopAudio: () => {} })
provide('layoutRefreshKey', ref(0))

// ── Build a synthetic message object for ChatMessageItem ──
const msgData = computed(() => {
  if (!props.execDetail?.content) return null
  const { blocks } = chatRender.parseAssistantContent(props.execDetail.content)
  if (!blocks || blocks.length === 0) return null
  return {
    id: props.execDetail.id || 'exec',
    role: 'assistant',
    content: props.execDetail.content,
    blocks,
    metadata: props.execDetail.metadata || null,
    createdAt: props.execDetail.createdAt || '',
    streaming: false,
    cancelled: false,
  }
})

// ── Expanded tools state ──
const expandedTools = ref({})

function toggleTool(key) {
  expandedTools.value = { ...expandedTools.value, [key]: !expandedTools.value[key] }
}

// ── Tool Detail Overlay ──
const toolDetailOverlay = ref({
  show: false,
  name: '',
  summary: '',
  inputHtml: '',
  outputHtml: '',
  status: '',
  done: true,
})

function handleShowToolDetail(block) {
  toolDetailOverlay.value = {
    show: true,
    name: block.name || '',
    summary: chatRender.toolCallSummary(block),
    inputHtml: chatRender.formatToolInput(block.input, block.name),
    outputHtml: block.output ? formatToolOutput(block.output, block.name) : '',
    status: block.status || '',
    done: !!block.done,
  }
}

function handleFileOpenInOverlay(filePath) {
  toolDetailOverlay.value.show = false
  openFilePath(filePath)
  emit('close')
  emit('open-file', filePath)
}

// ── Metadata Modal ──
const metadataModal = ref({
  show: false,
  data: {},
  backend: '',
  createdAt: '',
  relatedFile: '',
  messageId: null,
  sessionId: '',
  indexed: false,
})

function showMetadata() {
  const exec = props.execDetail
  if (!exec) return
  metadataModal.value.data = exec.metadata || {}
  metadataModal.value.backend = exec.backend || ''
  metadataModal.value.createdAt = exec.createdAt || ''
  metadataModal.value.relatedFile = ''
  metadataModal.value.messageId = exec.id || null
  metadataModal.value.sessionId = ''
  metadataModal.value.indexed = false
  metadataModal.value.show = true
}

// ── Delegated click handler for .chat-file-open-btn ──
const contentRef = ref(null)

function handleContentClick(event) {
  const btn = event.target.closest('.chat-file-open-btn')
  if (!btn) return
  event.preventDefault()
  event.stopPropagation()
  const filePath = btn.getAttribute('data-file-path')
  if (filePath) {
    openFilePath(filePath)
    emit('close')
    emit('open-file', filePath)
  }
}

// ── Reset state when exec detail changes ──
watch(() => props.execDetail, () => {
  expandedTools.value = {}
  toolDetailOverlay.value.show = false
  metadataModal.value.show = false
  // Verify file path annotations after content re-renders.
  // ChatRender.renderMarkdown calls verifyFilePaths targeting #aiChatMessages,
  // but this component renders outside that container, so non-existent file
  // path buttons are never removed. Run verification against our own container.
  nextTick(() => {
    if (contentRef.value) {
      const paths = [...contentRef.value.querySelectorAll('.chat-file-open-btn[data-file-path]')]
        .map(btn => btn.getAttribute('data-file-path'))
        .filter(Boolean)
      if (paths.length > 0) verifyFilePaths([...new Set(paths)], contentRef.value)
    }
  })
})

onUnmounted(() => {
  // Cleanup is handled by Vue's component unmounting
})
</script>

<style scoped>
.exec-detail-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.exec-detail-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}

.exec-detail-content {
  flex: 1;
  overflow-y: auto;
  padding: 12px 10px;
}

.exec-detail-empty {
  text-align: center;
  padding: 40px 12px;
  color: var(--text-muted, #999);
  font-size: 13px;
}
</style>

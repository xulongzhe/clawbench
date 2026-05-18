<template>
  <div class="exec-detail-page">
    <!-- Header: breadcrumb -->
    <div class="exec-detail-header">
      <TaskBreadcrumb />
    </div>

    <!-- Scrollable message content -->
    <div class="exec-detail-content" ref="contentRef" @click="handleContentClick">
      <!-- Summary / Original tab bar -->
      <div v-if="hasSummary" class="exec-tab-bar">
        <button class="exec-tab-btn" :class="{ active: activeTab === 'summary' }" @click="setTab('summary')">📌 总结</button>
        <button class="exec-tab-btn" :class="{ active: activeTab === 'original' }" @click="setTab('original')">📄 原文</button>
      </div>
      <ChatMessageItem
        v-if="activeMsgData"
        :msg="activeMsgData"
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
      <div v-else-if="execDetail?.status === 'cancelled'" class="exec-cancelled-notice">{{ t('task.exec.cancelledNotice') }}</div>
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
import { useAutoSpeech } from '@/composables/useAutoSpeech.ts'
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
provide('chatUI', { navigateToFileViewer: () => emit('close') })
provide('autoSpeech', useAutoSpeech())
provide('layoutRefreshKey', ref(0))

// ── Summary / Original toggle ──
const hasSummary = computed(() => props.execDetail?.summary != null && props.execDetail.summary !== '')
const activeTab = ref(hasSummary.value ? 'summary' : 'original')

function setTab(tab) {
  activeTab.value = tab
}

// ── Build a synthetic message object for ChatMessageItem (original content) ──
const msgData = computed(() => {
  if (!props.execDetail?.content && props.execDetail?.status !== 'cancelled') return null
  const { blocks } = chatRender.parseAssistantContent(props.execDetail.content || '{}')
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

// ── Build a synthetic message object for ChatMessageItem (summary content) ──
const summaryMsgData = computed(() => {
  if (!props.execDetail?.summary) return null
  const summaryJson = JSON.stringify({ blocks: [{ type: 'text', text: props.execDetail.summary }] })
  const { blocks } = chatRender.parseAssistantContent(summaryJson)
  if (!blocks || blocks.length === 0) return null
  return {
    id: (props.execDetail.id || 'exec') + '-summary',
    role: 'assistant',
    content: summaryJson,
    blocks,
    metadata: props.execDetail.metadata || null,
    createdAt: props.execDetail.createdAt || '',
    streaming: false,
    cancelled: false,
  }
})

// ── Active message data based on tab ──
const activeMsgData = computed(() => {
  if (activeTab.value === 'summary' && summaryMsgData.value) return summaryMsgData.value
  return msgData.value
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
  // Handle commit-hash clicks (span or button)
  const commitEl = event.target.closest('.chat-commit-hash, .chat-commit-open-btn')
  if (commitEl) {
    event.preventDefault()
    event.stopPropagation()
    const sha = commitEl.getAttribute('data-commit-sha')
    if (sha) {
      window.dispatchEvent(new CustomEvent('navigate-to-commit', { detail: { sha } }))
    }
    return
  }
  const btn = event.target.closest('.chat-file-open-btn')
  if (!btn) return
  event.preventDefault()
  event.stopPropagation()
  const filePath = btn.getAttribute('data-file-path')
  if (filePath) {
    openFilePath(filePath)
    emit('open-file', filePath)
  }
}

// ── Reset state when exec detail changes ──
watch(() => props.execDetail, () => {
  expandedTools.value = {}
  toolDetailOverlay.value.show = false
  metadataModal.value.show = false
  activeTab.value = hasSummary.value ? 'summary' : 'original'
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
  background: var(--bg-primary, #ffffff);
}

.exec-detail-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  flex-shrink: 0;
}

.exec-detail-content {
  flex: 1;
  overflow-y: auto;
  padding: 12px 8px;
}

.exec-tab-bar {
  display: flex;
  gap: 4px;
  margin-bottom: 12px;
  background: var(--bg-secondary, #f1f5f9);
  border-radius: 8px;
  padding: 3px;
}

.exec-tab-btn {
  flex: 1;
  border: none;
  background: transparent;
  color: var(--text-secondary, #6b7280);
  font-size: 13px;
  font-weight: 500;
  padding: 6px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  text-align: center;
}

.exec-tab-btn.active {
  background: var(--bg-primary, #ffffff);
  color: var(--text-primary, #1a1a1a);
  font-weight: 600;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

@media (hover: hover) {
  .exec-tab-btn:not(.active):hover {
    color: var(--text-primary, #1a1a1a);
    background: var(--bg-tertiary, #e2e8f0);
  }
}

.exec-detail-empty {
  text-align: center;
  padding: 40px 12px;
  color: var(--text-muted, #999);
  font-size: 14px;
}

.exec-cancelled-notice {
  padding: 3rem 1rem;
  text-align: center;
  color: var(--text-muted, #999);
  font-style: italic;
  font-size: 14px;
}
</style>

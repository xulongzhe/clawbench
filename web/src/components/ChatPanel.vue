<template>
  <BottomSheet ref="bottomSheetRef" :open="open" title="AI 对话" @close="$emit('close')">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
      </svg>
      <span class="bs-header-title">{{ session.agentHeaderTitle.value }}</span>
      <div v-if="session.currentSessionTitle.value" class="bs-header-description">
        <span class="bs-header-description-inner" :title="session.currentSessionTitle.value">
          {{ session.currentSessionTitle.value }}
        </span>
      </div>
      <button class="bs-close" @click.stop="$emit('close')" title="关闭">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    </template>

    <!-- Messages -->
    <ChatMessageList
      ref="messageListRef"
      :messages="messages"
      :expandedTools="render.expandedTools.value"
      :expandedThinking="render.expandedThinking.value"
      :blockProposals="render.blockProposals"
      :agents="session.agents.value"
      :renderedContents="render.renderedContents.value"
      @toggle-tool="render.toggleToolDetail"
      @toggle-thinking="render.toggleThinking"
      @show-metadata="showMetadata"
      @file-tag-click="handleFileTagClick"
    />

    <!-- Unified input container -->
    <ChatInputBar
      ref="inputBarRef"
      :inputDisabled="inputDisabled"
      :loading="loading"
      :currentFile="currentFile"
      :pendingFiles="pendingFiles"
      :attachedFiles="attachedFiles"
      @send="sendMessage"
      @cancel="stream.cancelStream"
      @file-select="handleFileSelect"
      @remove-file="removeFile"
      @add-attached="addAttachedFile"
      @remove-attached="removeAttachedFile"
      @open-session-tab="session.openSessionTab"
      @file-tag-click="handleFileTagClick"
    />

  </BottomSheet>

  <!-- Metadata Modal -->
  <ChatMetadataModal
    :show="metadataModal.show"
    :data="metadataModal.data"
    :backend="metadataModal.backend"
    :createdAt="metadataModal.createdAt"
    :filePath="metadataModal.filePath"
    :formatDetailTime="render.formatDetailTime"
    @close="metadataModal.show = false"
  />

  <!-- Session Drawer -->
  <SessionDrawer
    ref="sessionDrawerRef"
    :open="session.sessionDrawerOpen.value"
    :currentSessionId="session.currentSessionId.value"
    :runningSessionIds="session.runningSessions.value"
    @close="session.sessionDrawerOpen.value = false"
    @select="session.switchSession"
    @create="session.createSession"
    @delete="session.deleteSession"
  />

  <!-- Task Drawer -->
  <TaskDrawer
    ref="taskDrawerRef"
    :open="session.taskDrawerOpen.value"
    @close="session.taskDrawerOpen.value = false"
  />
</template>

<script setup>
import { ref, watch, onUnmounted, onMounted, inject, provide, toRef } from 'vue'
import BottomSheet from './BottomSheet.vue'
import SessionDrawer from './SessionDrawer.vue'
import TaskDrawer from './TaskDrawer.vue'
import ChatMetadataModal from './ChatMetadataModal.vue'
import ChatInputBar from './ChatInputBar.vue'
import ChatMessageList from './ChatMessageList.vue'
import { useChatRender } from '@/composables/useChatRender.ts'
import { useChatStream } from '@/composables/useChatStream.ts'
import { useChatSession } from '@/composables/useChatSession.ts'
import { useToast } from '@/composables/useToast.ts'
import { useFilePathAnnotation } from '@/composables/useFilePathAnnotation.ts'
import { useNotification } from '@/composables/useNotification.ts'
import { useFileUpload } from '@/composables/useFileUpload.ts'

const props = defineProps({
    open: Boolean,
    currentFile: Object,
})
const emit = defineEmits(['close', 'open', 'message'])

const messages = ref([])
const inputDisabled = ref(true)
const loading = ref(false)
const currentSessionId = ref('')
const sessionDrawerRef = ref(null)
const bottomSheetRef = ref(null)
const inputBarRef = ref(null)
const messageListRef = ref(null)
const metadataModal = ref({
  show: false,
  data: {},
  backend: '',
  createdAt: '',
  filePath: ''
})
const toast = useToast()
const notification = useNotification()
const theme = inject('theme', ref('light'))
const { openFilePath } = useFilePathAnnotation()

function handleFileTagClick(filePath) {
    if (filePath) {
        openFilePath(filePath)
        bottomSheetRef.value?.close()
    }
}

const render = useChatRender({ messages, theme, currentSessionId })

const session = useChatSession({
  currentSessionId,
  messages,
  loading,
  inputDisabled,
  renderedContents: render.renderedContents,
  blockProposals: render.blockProposals,
  expandedTools: render.expandedTools,
  expandedThinking: render.expandedThinking,
  onParseAssistantContent: (content) => render.parseAssistantContent(content),
  onExtractScheduleProposals: (msgs) => render.extractScheduleProposals(msgs),
  onRenderUpdate: (forceFull) => render.updateRenderedContents(forceFull),
  onScrollBottom: (force) => scrollBottom(force),
  onConnectStream: (sessionId) => stream.connectStream(sessionId),
  onStopPolling: () => stream.stopPolling(),
  onDisconnectStream: () => stream.disconnectStream(),
  onMessage: () => emit('message'),
  onOpen: () => emit('open'),
  isOpen: toRef(props, 'open'),
})


const stream = useChatStream({
  messages,
  currentSessionId: session.currentSessionId,
  currentBackend: session.currentBackend,
  loading,
  inputDisabled,
  onRenderNeeded: (forceFull) => render.updateRenderedContents(forceFull),
  onScrollBottom: (force) => scrollBottom(force),
  onLoadHistory: () => session.loadHistory(),
  onMessage: () => emit('message'),
  onOpen: () => emit('open'),
  isOpen: toRef(props, 'open'),
  createScheduledTask: (proposal) => render.createScheduledTask(proposal),
  onParseAssistantContent: (content) => render.parseAssistantContent(content),
  onToast: (msg, opts) => toast.show(msg, opts),
  onNotification: (title, opts) => notification.show(title, opts),
})

provide('chatRender', {
  renderTextBlock: render.renderTextBlock,
  formatMessageTime: render.formatMessageTime,
  toolCallSummary: render.toolCallSummary,
  formatToolInput: render.formatToolInput,
  humanizeCron: render.humanizeCron,
  repeatLabel: render.repeatLabel,
  truncate: render.truncate,
  hasImagesInContent: render.hasImagesInContent,
})
provide('chatSession', { getAgentIcon: session.getAgentIcon, getAgentName: session.getAgentName })
provide('chatUI', { closeSheet: () => bottomSheetRef.value?.close() })

// 子抽屉跟随聊天框关闭
watch(() => props.open, (val) => {
  if (!val) {
    session.sessionDrawerOpen.value = false
    session.taskDrawerOpen.value = false
  }
})

const { pendingFiles, attachedFiles, handleFileSelect, removeFile, addAttachedFile, removeAttachedFile, cleanupPreviewUrls, clearPendingFiles } = useFileUpload({ inputDisabled })

async function sendMessage(text) {
    const inputText = text !== undefined ? text : (inputBarRef.value?.inputText?.trim() || '')
    const hasFiles = pendingFiles.value.length > 0 || attachedFiles.value.length > 0

    if ((!inputText && !hasFiles) || inputDisabled.value) return

    const filePaths = attachedFiles.value.length > 0 ? [...attachedFiles.value] : []
    const uploadedFiles = pendingFiles.value.map(f => ({ path: f.path }))
    const projectFiles = filePaths.map(p => ({ path: p }))

    messages.value.push({
        role: 'user',
        content: inputText,
        filePath: filePaths.length > 0 ? filePaths[0] : '',
        files: [...uploadedFiles, ...projectFiles],
        createdAt: new Date().toISOString()
    })

    render.updateRenderedContents()

    attachedFiles.value = []
    inputBarRef.value?.clearInput()
    clearPendingFiles()

    inputDisabled.value = true
    loading.value = true
    scrollBottom(true)

    try {
        // Use currentAgentId as-is (backend will use default agent if empty)
        const effectiveAgentId = session.currentAgentId.value

        const url = session.currentSessionId.value
            ? `/api/ai/chat?session_id=${encodeURIComponent(session.currentSessionId.value)}`
            : '/api/ai/chat'
        const resp = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ message: text, filePaths, files: [...uploadedFiles, ...projectFiles].map(f => f.path), agentId: effectiveAgentId }),
        })
        const data = await resp.json()
        if (!resp.ok) {
            throw new Error(data.error || 'Unknown error')
        }
        // Update session ID if backend created a new one
        if (data.sessionId && !session.currentSessionId.value) {
            session.currentSessionId.value = data.sessionId
        }
        // Session already running — another request is in progress
        if (data.running) {
            loading.value = true
            inputDisabled.value = true
            stream.connectStream(session.currentSessionId.value)
            return
        }
        stream.connectStream(session.currentSessionId.value)
    } catch (err) {
        stream.stopPolling()
        stream.disconnectStream()
        messages.value.push({ role: 'assistant', content: `错误: ${err.message}`, file_path: '' })
        inputDisabled.value = false
        loading.value = false
        toast.show('发送失败，请重试', { icon: '⚠️' })
        // Clear session ID on error to prevent using invalid session
        if (err.message?.includes('Session backend not found') || err.message?.includes('session not found')) {
            session.currentSessionId.value = ''
        }
    }
}

function scrollBottom(force = false) {
    messageListRef.value?.scrollToBottom(force)
}

function showMetadata(msg) {
    metadataModal.value.data = msg.metadata || {}
    metadataModal.value.backend = msg.backend || ''
    metadataModal.value.createdAt = msg.createdAt || ''
    metadataModal.value.filePath = msg.filePath || ''
    metadataModal.value.show = true
}

// Start global polling when component mounts
onMounted(() => {
    // Request notification permission on mount
    notification.requestPermission().catch(err => {
        console.warn('Failed to request notification permission:', err)
    })

    session.startGlobalPolling()
    document.addEventListener('visibilitychange', session.handleVisibilityChange)
})

// Cleanup preview URLs on unmount
onUnmounted(() => {
    cleanupPreviewUrls()
    stream.disconnectStream()
    stream.stopPolling()
    session.stopGlobalPolling()
    session.stopMsgCountPolling()
    document.removeEventListener('visibilitychange', session.handleVisibilityChange)
    notification.closeAll()
})

watch(() => props.open, async (val) => {
    if (val) {
        await session.loadHistory()
    }
})
</script>

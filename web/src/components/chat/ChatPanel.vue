<template>
  <BottomSheet ref="bottomSheetRef" :open="open" title="AI 对话" @close="$emit('close')">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
      </svg>
      <span class="bs-header-title">{{ session.agentHeaderTitle.value }}</span>
      <div v-if="session.currentSessionTitle.value" class="bs-header-description">
        <HeaderMarquee :text="session.currentSessionTitle.value">{{ session.currentSessionTitle.value }}</HeaderMarquee>
      </div>
    </template>

    <!-- Messages -->
    <ChatMessageList
      ref="messageListRef"
      :messages="messages"
      :expandedTools="render.expandedTools.value"
      :blockProposals="render.blockProposals"
      :agents="agentsList"
      :currentAgent="currentAgent"
      :currentSessionId="identity.currentSessionId.value"
      :renderedContents="render.renderedContents.value"
      :hasMore="session.hasMore.value"
      :loadingMore="session.loadingMore.value"
      :indicatorText="swipeSession.indicatorText.value"
      :indicatorDirection="swipeSession.indicatorDirection.value"
      @touchstart="swipeSession.onTouchStart"
      @touchend="swipeSession.onTouchEnd"
      @toggle-tool="render.toggleToolDetail"
      @show-metadata="showMetadata"
      @file-tag-click="handleFileTagClick"
      @load-more="handleLoadMore"
    />

    <!-- Session switching overlay — placed here to cover the entire message area -->
    <Transition name="session-switch-fade">
      <div v-if="session.switching.value" class="session-switch-overlay">
        <div class="session-switch-spinner"></div>
      </div>
    </Transition>

    <!-- Unified input container -->
    <ChatInputBar
      ref="inputBarRef"
      :inputDisabled="inputDisabled"
      :loading="loading"
      :currentFile="currentFile"
      :pendingFiles="pendingFiles"
      :attachedFiles="attachedFiles"
      :messages="messages"
      :autoSpeechEnabled="autoSpeech.enabled.value"
      :currentSessionId="identity.currentSessionId.value"
      :chatUnread="store.state.chatUnread"
      :chatRunning="store.state.chatRunning"
      :quickSend="store.state.chatQuickSend"
      @send="sendMessage"
      @cancel="stream.cancelStream"
      @file-select="handleFileSelect"
      @file-drop="handleFileDrop"
      @remove-file="removeFile"
      @add-attached="addAttachedFile"
      @remove-attached="removeAttachedFile"
      @open-session-tab="session.openSessionTab"
      @file-tag-click="handleFileTagClick"
      @toggle-auto-speech="autoSpeech.toggle"
      @create-session="handleCreateSession"
      @delete-session="handleDeleteSession"
    />

  </BottomSheet>

  <!-- Metadata Modal -->
  <ChatMetadataModal
    :show="metadataModal.show"
    :data="metadataModal.data"
    :backend="metadataModal.backend"
    :createdAt="metadataModal.createdAt"
    :filePath="metadataModal.filePath"
    :messageId="metadataModal.messageId"
    :formatDetailTime="render.formatDetailTime"
    @close="metadataModal.show = false"
  />

  <!-- Session Drawer -->
  <SessionDrawer
    ref="sessionDrawerRef"
    :open="session.sessionDrawerOpen.value"
    :currentSessionId="identity.currentSessionId.value"
    :runningSessionIds="identity.runningSessions.value"
    @close="session.sessionDrawerOpen.value = false"
    @select="session.switchSession"
    @create="handleCreateSession"
    @delete="handleDeleteSessionById"
  />

  <!-- Task Drawer -->
  <TaskDrawer
    ref="taskDrawerRef"
    :open="session.taskDrawerOpen.value"
    @close="session.taskDrawerOpen.value = false"
  />
</template>

<script setup>
import { ref, computed, watch, onUnmounted, onMounted, inject, provide, toRef, nextTick } from 'vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import HeaderMarquee from '@/components/common/HeaderMarquee.vue'
import SessionDrawer from '@/components/session/SessionDrawer.vue'
import TaskDrawer from '@/components/task/TaskDrawer.vue'
import ChatMetadataModal from './ChatMetadataModal.vue'
import ChatInputBar from './ChatInputBar.vue'
import ChatMessageList from './ChatMessageList.vue'
import { useChatRender } from '@/composables/useChatRender.ts'
import { useChatStream } from '@/composables/useChatStream.ts'
import { useChatSession } from '@/composables/useChatSession.ts'
import { useSessionIdentity } from '@/composables/useSessionIdentity.ts'
import { useAgents } from '@/composables/useAgents.ts'
import { useToast } from '@/composables/useToast.ts'
import { useFilePathAnnotation } from '@/composables/useFilePathAnnotation.ts'
import { useNotification } from '@/composables/useNotification.ts'
import { useFileUpload } from '@/composables/useFileUpload.ts'
import { playNotificationSound } from '@/composables/useNotificationSound.ts'
import { useAutoSpeech } from '@/composables/useAutoSpeech.ts'
import { useSwipeSession } from '@/composables/useSwipeSession.ts'
import { store } from '@/stores/app.ts'

const props = defineProps({
    open: Boolean,
    currentFile: Object,
})
const emit = defineEmits(['close', 'open', 'message'])

// ── Singletons ──
const identity = useSessionIdentity()
const { agents: agentsList, getAgentIcon, getAgentName } = useAgents()

const messages = ref([])
const inputDisabled = ref(true)
const loading = ref(false)
// Incremented when the panel reopens, so ChatMessageItem can re-check
// overflow after being hidden (display:none gives scrollHeight=0).
const layoutRefreshKey = ref(0)
const currentAgent = computed(() => {
  const agentId = identity.currentAgentId.value
  if (!agentId) return null
  return agentsList.value.find(a => a.id === agentId) || null
})
const sessionDrawerRef = ref(null)
const bottomSheetRef = ref(null)
const inputBarRef = ref(null)
const messageListRef = ref(null)
const metadataModal = ref({
  show: false,
  data: {},
  backend: '',
  createdAt: '',
  filePath: '',
  messageId: null
})
const toast = useToast()
const notification = useNotification()
const autoSpeech = useAutoSpeech()
const theme = inject('theme', ref('light'))
const { openFilePath } = useFilePathAnnotation()

function handleFileTagClick(filePath) {
    if (filePath) {
        openFilePath(filePath)
        bottomSheetRef.value?.close()
    }
}

const render = useChatRender({ messages, theme, currentSessionId: identity.currentSessionId })

const session = useChatSession({
  currentSessionId: identity.currentSessionId,
  messages,
  loading,
  inputDisabled,
  renderedContents: render.renderedContents,
  blockProposals: render.blockProposals,
  expandedTools: render.expandedTools,
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
  onStreamDone: playNotificationSound,
})

const swipeSession = useSwipeSession({
  currentSessionId: identity.currentSessionId,
  switchSession: session.switchSession,
})

// onStreamDone: fires when current session stream completes
// Plays notification sound and triggers auto-speech if enabled
function onStreamDone() {
  playNotificationSound()
  if (autoSpeech.enabled.value) {
    const lastMsg = messages.value[messages.value.length - 1]
    if (lastMsg?.role === 'assistant') {
      const textBlocks = (lastMsg.blocks || []).filter(b => b.type === 'text')
      const fullText = textBlocks.map(b => b.text || '').join('\n')
      if (fullText.trim() && lastMsg.id) {
        autoSpeech.speakMessage(lastMsg.id, fullText.trim())
      }
    }
  }
}

const stream = useChatStream({
  messages,
  currentSessionId: identity.currentSessionId,
  currentBackend: identity.currentBackend,
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
  onStreamDone,
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
provide('chatSession', { getAgentIcon, getAgentName })
provide('chatUI', { closeSheet: () => bottomSheetRef.value?.close() })
provide('autoSpeech', autoSpeech)
provide('layoutRefreshKey', layoutRefreshKey)

// Register session actions with the identity singleton so that
// App.vue / QuoteQuestionBar can trigger ChatPanel operations.
identity.registerSessionActions({
  switchSession: session.switchSession,
  createSession: async (agentId) => {
    cleanupActiveStream()
    await session.createSession(agentId)
  },
  deleteSession: async (sessionId, backend) => {
    cleanupActiveStream()
    await session.deleteSession(sessionId, backend)
  },
  sendMessage: (text, filePaths) => sendMessage(text, filePaths),
  openChatPanel: () => emit('open'),
})

// 子抽屉跟随聊天框关闭；面板打开时刷新渲染（修复 display:none 期间的过时布局状态）
watch(() => props.open, async (val) => {
  if (!val) {
    session.sessionDrawerOpen.value = false
    session.taskDrawerOpen.value = false
  } else {
    // Re-open: load history (with overlay) and fix stale layout state from v-show display:none
    await session.loadHistory(false, true)
    // Bump layoutRefreshKey AFTER loadHistory so ChatMessageItem re-checks
    // collapse state with the fresh messages and valid scrollHeight.
    nextTick(() => {
      layoutRefreshKey.value++
    })
  }
})

const { pendingFiles, attachedFiles, handleFileSelect, handleFileDrop, removeFile, addAttachedFile, removeAttachedFile, cleanupPreviewUrls, clearPendingFiles } = useFileUpload({ inputDisabled })

// Clean up streaming state when user wants to interact with session management
// (new session, delete session) while AI is still generating
function cleanupActiveStream() {
  if (!loading.value) return
  stream.disconnectStream()
  stream.stopPolling()
  const streamingMsg = messages.value.find(m => m.role === 'assistant' && m.streaming)
  if (streamingMsg) {
    delete streamingMsg.streaming
    if (streamingMsg.blocks) {
      for (const block of streamingMsg.blocks) {
        if (block.type === 'tool_use' && !block.done) block.done = true
      }
    }
  }
  render.updateRenderedContents(true)
}

async function handleCreateSession(agentId) {
  cleanupActiveStream()
  await session.createSession(agentId)
}

async function handleDeleteSession() {
  if (!identity.currentSessionId.value) return
  cleanupActiveStream()
  await session.deleteSession(identity.currentSessionId.value, identity.currentBackend.value)
}

async function handleDeleteSessionById(sessionId, backend) {
  cleanupActiveStream()
  await session.deleteSession(sessionId, backend)
}

async function sendMessage(text, extraFilePaths) {
    const inputText = text !== undefined ? text : (inputBarRef.value?.inputText?.trim() || '')
    const hasFiles = pendingFiles.value.length > 0 || attachedFiles.value.length > 0

    if ((!inputText && !hasFiles) || inputDisabled.value) return

    // Merge attached files from the input bar with extra file paths (e.g. from quote-question)
    const filePaths = [...(extraFilePaths || []), ...(attachedFiles.value.length > 0 ? attachedFiles.value : [])]
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
        const effectiveAgentId = identity.currentAgentId.value

        const url = identity.currentSessionId.value
            ? `/api/ai/chat?session_id=${encodeURIComponent(identity.currentSessionId.value)}`
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
        if (data.sessionId && !identity.currentSessionId.value) {
            identity.currentSessionId.value = data.sessionId
        }
        // Session already running — another request is in progress
        if (data.running) {
            loading.value = true
            inputDisabled.value = true
            stream.connectStream(identity.currentSessionId.value)
            return
        }
        stream.connectStream(identity.currentSessionId.value)
    } catch (err) {
        stream.stopPolling()
        stream.disconnectStream()
        messages.value.push({ role: 'assistant', content: `错误: ${err.message}`, file_path: '' })
        inputDisabled.value = false
        loading.value = false
        toast.show('发送失败，请重试', { icon: '⚠️', type: 'error' })
        // Clear session ID on error to prevent using invalid session
        if (err.message?.includes('Session backend not found') || err.message?.includes('session not found')) {
            identity.currentSessionId.value = ''
        }
    }
}

function scrollBottom(force = false) {
    messageListRef.value?.scrollToBottom(force)
}

async function handleLoadMore() {
    const el = messageListRef.value?.messagesRef
    if (!el) return
    const oldScrollHeight = el.scrollHeight
    await session.loadMoreMessages()
    // Wait for DOM update + one frame for async rendering (Mermaid, KaTeX)
    await nextTick()
    await new Promise(resolve => requestAnimationFrame(resolve))
    const newScrollHeight = el.scrollHeight
    el.scrollTop = newScrollHeight - oldScrollHeight
}

function showMetadata(msg) {
    metadataModal.value.data = msg.metadata || {}
    metadataModal.value.backend = msg.backend || ''
    metadataModal.value.createdAt = msg.createdAt || ''
    metadataModal.value.filePath = msg.filePath || ''
    metadataModal.value.messageId = msg.id || null
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
</script>

<style scoped>
/* Make .bs-body a positioning context so the switching overlay covers
   the message+input area only (not the header above it). */
:deep(.bs-body) {
  position: relative;
}

/* Session switch overlay — covers the entire body area (messages + input) */
.session-switch-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-primary);
  z-index: 5;
  opacity: 0.85;
}

.session-switch-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: session-switch-spin 0.7s linear infinite;
}

@keyframes session-switch-spin {
  to { transform: rotate(360deg); }
}

.session-switch-fade-enter-active {
  transition: opacity 0.12s ease-out;
}
.session-switch-fade-leave-active {
  transition: opacity 0.18s ease-in;
}
.session-switch-fade-enter-from,
.session-switch-fade-leave-to {
  opacity: 0;
}
</style>

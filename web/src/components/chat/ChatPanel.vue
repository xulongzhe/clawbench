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
      :totalMessages="session.totalMessages.value"
      :pendingMessages="pendingMessages.value"
      @touchstart="swipeSession.onTouchStart"
      @touchend="swipeSession.onTouchEnd"
      @toggle-tool="render.toggleToolDetail"
      @show-metadata="showMetadata"
      @file-tag-click="handleFileTagClick"
      @load-more="handleLoadMore"
      @edit-task="openTaskEdit"
      @send-message="handleToolSendMessage"
      @remove-pending="handleRemovePending"
    />

    <!-- Session switching overlay — placed here to cover the entire message area -->
    <Transition name="session-switch-fade">
      <div v-if="session.switching.value" class="session-switch-overlay">
        <div class="session-switch-spinner"></div>
      </div>
    </Transition>

    <!-- Session swipe indicator — floats above the message area -->
    <Transition name="session-indicator">
      <div v-if="swipeSession.indicatorText.value" class="session-switch-indicator" :class="swipeSession.indicatorDirection.value">
        <div class="session-indicator-arrow">
          <svg v-if="swipeSession.indicatorDirection.value === 'left'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <polyline points="9 18 15 12 9 6"/>
          </svg>
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <polyline points="15 18 9 12 15 6"/>
          </svg>
        </div>
        <span class="session-indicator-text">{{ swipeSession.indicatorText.value }}</span>
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
      :taskUnread="store.state.taskUnread"
      :quickSend="store.state.chatQuickSend"
      :pendingCount="pendingMessages.value.length"
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
      @show-agent-selector="handleShowAgentSelector"
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

  <!-- Task Edit Dialog (opened from schedule-proposal card) -->
  <TaskFormDialog
    :open="taskEditOpen"
    mode="edit"
    :task="taskEditData"
    @close="taskEditOpen = false"
    @saved="handleTaskEditSaved"
  />
</template>

<script setup>
import { ref, computed, watch, onUnmounted, onMounted, inject, provide, toRef, nextTick } from 'vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import HeaderMarquee from '@/components/common/HeaderMarquee.vue'
import SessionDrawer from '@/components/session/SessionDrawer.vue'
import TaskDrawer from '@/components/task/TaskDrawer.vue'
import TaskFormDialog from '@/components/task/TaskFormDialog.vue'
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
// Pending message queue: messages enqueued while AI is generating,
// consumed automatically when the current stream ends normally.
const pendingMessages = ref([])
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

// Task edit dialog (opened from schedule-proposal card)
const taskEditOpen = ref(false)
const taskEditData = ref(null)

async function openTaskEdit(taskId) {
  try {
    const resp = await fetch(`/api/tasks/${taskId}`)
    if (resp.ok) {
      taskEditData.value = await resp.json()
      taskEditOpen.value = true
    }
  } catch (err) {
    console.error('Failed to load task for editing:', err)
  }
}

function handleTaskEditSaved() {
  taskEditOpen.value = false
  taskDrawerRef.value?.loadTasks()
}

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

// onStreamEnd: fires when current session stream completes with a reason
// - 'done': normal completion → consume pending queue, play sound, auto-speech
// - 'cancelled': user cancelled → clear queue (user chose to stop)
// - 'error': error occurred → pause queue, let user decide
function onStreamEnd(reason) {
  if (reason === 'done') {
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
    // Consume pending queue after a tick so loading=false has settled
    nextTick(() => consumeQueue())
  } else if (reason === 'cancelled') {
    // User explicitly cancelled — clear the pending queue
    pendingMessages.value = []
  }
  // 'error': don't touch the queue — user can decide to continue or clear
}

const stream = useChatStream({
  messages,
  currentSessionId: identity.currentSessionId,
  currentBackend: identity.currentBackend,
  loading,
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
  onStreamEnd,
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

// ── Pending message queue ──

/** Enqueue a message for later delivery while AI is generating. */
function enqueueMessage(text, extraFilePaths) {
  const inputText = text !== undefined ? text : ''
  const filePaths = [...(extraFilePaths || []), ...(attachedFiles.value.length > 0 ? attachedFiles.value : [])]
  const uploadedFiles = pendingFiles.value.map(f => ({ path: f.path }))
  const projectFiles = filePaths.map(p => ({ path: p }))

  pendingMessages.value.push({
    text: inputText,
    filePaths,
    files: [...uploadedFiles, ...projectFiles].map(f => f.path),
    createdAt: new Date().toISOString(),
  })

  // Clear input state after enqueueing
  attachedFiles.value = []
  inputBarRef.value?.clearInput()
  clearPendingFiles()
  scrollBottom(true)
}

/** Consume the next pending message from the queue (called after stream ends normally). */
function consumeQueue() {
  if (pendingMessages.value.length === 0) return
  if (loading.value) return // safety: don't send if somehow still loading
  const next = pendingMessages.value.shift()
  sendMessageNow(next.text, next.filePaths, next.files)
}

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
  // Clear pending queue on forced cleanup (session switch / delete)
  pendingMessages.value = []
}

async function handleCreateSession(agentId) {
  cleanupActiveStream()
  await session.createSession(agentId)
}

function handleShowAgentSelector() {
  sessionDrawerRef.value?.openAgentSelector()
}

async function handleDeleteSession() {
  if (!identity.currentSessionId.value) return
  const deletedId = identity.currentSessionId.value
  cleanupActiveStream()
  await session.deleteSession(deletedId, identity.currentBackend.value)
  inputBarRef.value?.deleteDraft(deletedId)
}

async function handleDeleteSessionById(sessionId, backend) {
  cleanupActiveStream()
  await session.deleteSession(sessionId, backend)
  inputBarRef.value?.deleteDraft(sessionId)
}

async function sendMessage(text, extraFilePaths) {
    const inputText = text !== undefined ? text : (inputBarRef.value?.inputText?.trim() || '')
    const hasFiles = pendingFiles.value.length > 0 || attachedFiles.value.length > 0

    if ((!inputText && !hasFiles) || inputDisabled.value) return

    // If AI is generating, enqueue the message instead of sending immediately
    if (loading.value) {
      enqueueMessage(inputText, extraFilePaths)
      return
    }

    // Merge attached files from the input bar with extra file paths (e.g. from quote-question)
    const filePaths = [...(extraFilePaths || []), ...(attachedFiles.value.length > 0 ? attachedFiles.value : [])]
    const uploadedFiles = pendingFiles.value.map(f => ({ path: f.path }))
    const projectFiles = filePaths.map(p => ({ path: p }))
    const allFiles = [...uploadedFiles, ...projectFiles].map(f => f.path)

    // Clear input state before async request
    attachedFiles.value = []
    inputBarRef.value?.clearInput()
    clearPendingFiles()

    await sendMessageNow(inputText, filePaths, allFiles)
}

/** Actually send a message to the backend (no queue check). */
async function sendMessageNow(text, filePaths, files) {
    messages.value.push({
        role: 'user',
        content: text || '',
        filePath: filePaths.length > 0 ? filePaths[0] : '',
        files: (files || []).map(p => ({ path: p })),
        createdAt: new Date().toISOString()
    })

    render.updateRenderedContents()
    loading.value = true
    scrollBottom(true)

    try {
        const effectiveAgentId = identity.currentAgentId.value

        const url = identity.currentSessionId.value
            ? `/api/ai/chat?session_id=${encodeURIComponent(identity.currentSessionId.value)}`
            : '/api/ai/chat'
        const resp = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ message: text, filePaths, files: files || [], agentId: effectiveAgentId }),
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
            stream.connectStream(identity.currentSessionId.value)
            return
        }
        stream.connectStream(identity.currentSessionId.value)
    } catch (err) {
        stream.stopPolling()
        stream.disconnectStream()
        messages.value.push({ role: 'assistant', content: `错误: ${err.message}`, file_path: '' })
        loading.value = false
        toast.show('发送失败，请重试', { icon: '⚠️', type: 'error' })
        // Clear session ID on error to prevent using invalid session
        if (err.message?.includes('Session backend not found') || err.message?.includes('session not found')) {
            identity.currentSessionId.value = ''
        }
    }
}

/** Handle a tool-triggered message send (e.g. AskUserQuestion answer).
 *  If the AI stream is still running, enqueues the message for delivery after stream ends. */
async function handleToolSendMessage(text) {
    if (!text) return
    if (loading.value) {
      enqueueMessage(text)
    } else {
      await sendMessage(text)
    }
}

/** Remove a pending message from the queue by index. */
function handleRemovePending(index) {
  pendingMessages.value.splice(index, 1)
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

/* Session swipe indicator — floats at top of message area */
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
  position: absolute;
  top: 48px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
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
    transform: translateX(-50%) translateX(30px) scale(0.9);
  }
  to {
    opacity: 1;
    transform: translateX(-50%) scale(1);
  }
}

@keyframes indicator-slide-right {
  from {
    opacity: 0;
    transform: translateX(-50%) translateX(-30px) scale(0.9);
  }
  to {
    opacity: 1;
    transform: translateX(-50%) scale(1);
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
  transform: translateX(-50%) scale(0.95);
}
</style>

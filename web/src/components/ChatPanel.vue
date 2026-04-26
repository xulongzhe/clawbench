<template>
  <BottomSheet ref="bottomSheetRef" :open="open" title="AI 对话" @close="$emit('close')">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
      </svg>
      <span class="bs-header-title">{{ agentHeaderTitle }}</span>
      <div v-if="currentSessionTitle" class="bs-header-description">
        <span class="bs-header-description-inner" :title="currentSessionTitle">
          {{ currentSessionTitle }}
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
    <div class="chat-messages" id="aiChatMessages" ref="messagesRef" @click="handleChatClick">
      <div v-if="messages.length === 0" class="chat-empty">
        <span>发送消息开始与 AI 对话</span>
      </div>

      <div v-for="(msg, i) in messages" :key="`${msg.createdAt || ''}-${i}`"
        class="chat-message" :class="[msg.role, { 'has-metadata': msg.role === 'assistant' && msg.metadata }]">

        <div v-if="msg.role === 'user' && msg.files && msg.files.length > 0 && !hasImagesInContent(msg.content)" class="chat-files">
          <template v-for="(f, idx) in msg.files" :key="idx">
            <span v-if="isUploadPath(normalizeFileEntry(f).path)" class="chat-file-attachment attachment-upload" @click="handleFileTagClick(normalizeFileEntry(f).path)" title="打开文件">
              <svg v-if="isImageFile(normalizeFileEntry(f).path)" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="12" height="12">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                <polyline points="14 2 14 8 20 8"/>
                <circle cx="10" cy="13" r="2"/>
                <path d="m20 17-3.1-3.1a2 2 0 0 0-2.8 0L9 19"/>
              </svg>
              <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="12" height="12">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                <polyline points="14 2 14 8 20 8"/>
              </svg>
              <span class="chat-file-name">{{ getFileName(normalizeFileEntry(f).path) }}</span>
            </span>
            <span v-else class="chat-file-attachment attachment-ref" @click="handleFileTagClick(normalizeFileEntry(f).path)" title="打开文件">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="12" height="12">
                <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
              </svg>
              <span class="chat-file-name">{{ getFileName(normalizeFileEntry(f).path) }}</span>
            </span>
          </template>
        </div>

        <!-- Scheduled task trigger banner -->
        <div v-if="msg.role === 'assistant' && msg.scheduledTask" class="chat-scheduled-banner">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
          </svg>
          <span class="scheduled-label">定时触发</span>
          <span class="scheduled-task-name">{{ msg.scheduledTask.taskName }}</span>
          <span class="scheduled-sep">·</span>
          <span class="scheduled-agent">{{ getAgentIcon(msg.scheduledTask.agentId) }} {{ getAgentName(msg.scheduledTask.agentId) }}</span>
          <span class="scheduled-sep">·</span>
          <span class="scheduled-cron">{{ msg.scheduledTask.cronExpr }}</span>
        </div>

        <!-- Message content -->
        <template v-if="msg.role === 'assistant' && msg.blocks">
          <template v-for="(block, bi) in msg.blocks" :key="bi">
            <!-- Thinking block -->
            <div v-if="block.type === 'thinking'" class="chat-thinking" :class="{ expanded: expandedThinking[`${i}-${bi}`] }" @click="toggleThinking(`${i}-${bi}`)">
              <div class="thinking-header">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                  <circle cx="12" cy="12" r="10"/>
                  <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/>
                </svg>
                <span class="thinking-label">Thinking</span>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12" class="thinking-chevron">
                  <polyline points="6 9 12 15 18 9"/>
                </svg>
              </div>
              <pre v-if="expandedThinking[`${i}-${bi}`]" class="thinking-text" @click.stop>{{ block.text }}</pre>
            </div>
            <!-- Tool use block -->
            <template v-else-if="block.type === 'tool_use'">
              <div class="chat-tool-call" :class="{ done: block.done }" @click="toggleToolDetail(`${i}-${bi}`)">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12" class="tool-icon">
                  <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>
                </svg>
                <span class="tool-name">{{ block.name }}</span>
                <span v-if="toolCallSummary(block)" class="tool-summary">{{ toolCallSummary(block) }}</span>
                <span v-if="!block.done" class="tool-spinner"></span>
                <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12" class="tool-check">
                  <polyline points="20 6 9 17 4 12"/>
                </svg>
              </div>
              <pre v-if="block.input && Object.keys(block.input).length && expandedTools[`${i}-${bi}`]" class="tool-detail" @click.stop v-html="formatToolInput(block.input)"></pre>
            </template>
            <!-- Error block -->
            <div v-else-if="block.type === 'error'" class="chat-error-card">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14" class="error-icon">
                <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
                <line x1="12" y1="9" x2="12" y2="13"/>
                <line x1="12" y1="17" x2="12.01" y2="17"/>
              </svg>
              <span class="error-text">{{ block.text }}</span>
            </div>
            <!-- Warning block (e.g. CLI stderr on success) -->
            <div v-else-if="block.type === 'warning'" class="chat-warning-card">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14" class="warning-icon">
                <circle cx="12" cy="12" r="10"/>
                <line x1="12" y1="8" x2="12" y2="12"/>
                <line x1="12" y1="16" x2="12.01" y2="16"/>
              </svg>
              <span class="warning-text">{{ block.text }}</span>
            </div>
            <!-- Text block -->
            <div v-else-if="block.type === 'text'" v-html="renderTextBlock(block.text, msg.id, bi)"></div>
            <!-- Schedule proposal card (inline in message) -->
            <div v-if="block.type === 'text' && blockProposals[`${msg.id}-${bi}`]" class="schedule-proposal-card confirmed">
              <div class="proposal-header confirmed">📋 定时任务已创建</div>
              <div class="proposal-body">
                <div class="proposal-row"><strong>任务：</strong>{{ blockProposals[`${msg.id}-${bi}`].proposal.name }}</div>
                <div class="proposal-row"><strong>频率：</strong>{{ humanizeCron(blockProposals[`${msg.id}-${bi}`].proposal.cron_expr) }}</div>
                <div class="proposal-row"><strong>执行者：</strong>{{ getAgentIcon(blockProposals[`${msg.id}-${bi}`].proposal.agent_id) }} {{ getAgentName(blockProposals[`${msg.id}-${bi}`].proposal.agent_id) }}</div>
                <div class="proposal-row"><strong>重复：</strong>{{ repeatLabel(blockProposals[`${msg.id}-${bi}`].proposal.repeat_mode, blockProposals[`${msg.id}-${bi}`].proposal.max_runs) }}</div>
                <div class="proposal-row"><strong>提示词：</strong>{{ truncate(blockProposals[`${msg.id}-${bi}`].proposal.prompt, 80) }}</div>
              </div>
            </div>
          </template>
          <!-- Loading dots while AI is still streaming -->
          <div v-if="msg.streaming || msg.blocks.length === 0" class="placeholder-dots"><span></span><span></span><span></span></div>
          <!-- Cancelled marker -->
          <div v-if="msg.cancelled" class="chat-cancelled-mark">已中断</div>
        </template>
        <!-- User message or legacy plain text -->
        <div v-else-if="msg.role === 'user' || msg.content" v-html="renderedContents[i]"></div>

        <!-- Bottom bar for assistant messages with metadata -->
        <div v-if="msg.role === 'assistant' && msg.metadata" class="chat-meta-bar">
          <span class="chat-meta-info">
            <span v-if="msg.backend">{{ msg.backend }}</span>
            <span v-if="msg.metadata.model" class="chat-meta-sep">{{ msg.metadata.model }}</span>
            <span v-if="msg.createdAt" class="chat-meta-sep">{{ formatMessageTime(msg.createdAt) }}</span>
          </span>
          <button class="chat-info-btn" @click="showMetadata(msg)" title="查看详情">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <circle cx="12" cy="12" r="10"/>
              <line x1="12" y1="16" x2="12" y2="12"/>
              <line x1="12" y1="8" x2="12.01" y2="8"/>
            </svg>
          </button>
        </div>
      </div>

    </div>

    <!-- Unified input container -->
    <div class="chat-input-container">
      <input type="file" ref="fileInputRef" @change="handleFileSelect" style="display:none" multiple />
      <!-- Toolbar -->
      <div class="chat-toolbar">
        <button class="chat-toolbar-btn" @click="$refs.fileInputRef.click()" :disabled="inputDisabled" title="上传文件">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
            <polyline points="17 8 12 3 7 8"/>
            <line x1="12" y1="3" x2="12" y2="15"/>
          </svg>
        </button>
        <button class="chat-toolbar-btn"
          @click="addAttachedFile(props.currentFile?.path)"
          :disabled="inputDisabled || !props.currentFile?.path || attachedFiles.includes(props.currentFile?.path)"
          :title="!props.currentFile?.path ? '无当前文件' : attachedFiles.includes(props.currentFile?.path) ? '已附带此文件' : '附带当前文件'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
          </svg>
        </button>
        <button class="chat-toolbar-btn" @click="openSessionTab('sessions')" title="会话管理">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
          </svg>
        </button>
        <button class="chat-toolbar-btn" @click="openSessionTab('tasks')" title="定时任务">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
          </svg>
        </button>
      </div>
      <!-- Attachment tags -->
      <div v-if="attachedFiles.length > 0 || pendingFiles.length > 0" class="chat-attachment-tags">
        <span v-for="(filePath, idx) in attachedFiles" :key="'att-' + filePath" class="chat-file-attachment attachment-ref" @click="handleFileTagClick(filePath)" title="打开文件">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="12" height="12">
            <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
          </svg>
          <span class="chat-file-name">{{ getFileName(filePath) }}</span>
          <button class="attachment-tag-remove" @click.stop="removeAttachedFile(idx)" title="移除">×</button>
        </span>
        <span v-for="(f, idx) in pendingFiles" :key="'upload-' + idx" class="chat-file-attachment attachment-upload">
          <svg v-if="f.isImage" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="12" height="12">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <circle cx="10" cy="13" r="2"/>
            <path d="m20 17-3.1-3.1a2 2 0 0 0-2.8 0L9 19"/>
          </svg>
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="12" height="12">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
          </svg>
          <span class="chat-file-name">{{ getFileName(f.path) }}</span>
          <button class="attachment-tag-remove" @click.stop="removeFile(idx)" title="移除">×</button>
        </span>
      </div>
      <!-- Input row -->
      <div class="chat-input-row">
        <textarea class="chat-textarea"
          ref="textareaRef"
          v-model="inputText"
          :disabled="inputDisabled"
          :placeholder="pendingFiles.length > 0 ? '添加描述（可选）...' : '输入消息...'"
          rows="1"
          @keydown.enter.exact.prevent="sendMessage"
          @input="autoResizeTextarea"
          @blur="collapseTextarea"
          @dblclick="inputText = ''"></textarea>
        <button v-if="loading" class="chat-stop-btn" @click="cancelStream" title="停止生成">
          <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
        </button>
        <button v-else class="chat-send-btn" @click="sendMessage" :class="{ disabled: inputDisabled && pendingFiles.length === 0 && attachedFiles.length === 0 }" title="发送">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <line x1="22" y1="2" x2="11" y2="13"/>
            <polygon points="22 2 15 22 11 13 2 9 22 2"/>
          </svg>
        </button>
      </div>
    </div>

  </BottomSheet>

  <!-- Metadata Modal -->
  <Teleport to="body">
    <div v-if="metadataModal.show" class="metadata-modal-overlay" @click="metadataModal.show = false">
      <div class="metadata-modal" @click.stop>
        <div class="metadata-modal-header">
          <h3>响应详情</h3>
          <button class="metadata-close-btn" @click="metadataModal.show = false">×</button>
        </div>
        <div class="metadata-content">
          <div v-if="metadataModal.createdAt" class="metadata-item">
            <span class="metadata-label">时间:</span>
            <span class="metadata-value">{{ formatDetailTime(metadataModal.createdAt) }}</span>
          </div>
          <div v-if="metadataModal.filePath" class="metadata-item">
            <span class="metadata-label">关联文件:</span>
            <span class="metadata-value metadata-value-copyable" @click="copyValue(metadataModal.filePath, $event)">{{ metadataModal.filePath }}</span>
          </div>
          <div v-if="metadataModal.backend" class="metadata-item">
            <span class="metadata-label">后端:</span>
            <span class="metadata-value">{{ metadataModal.backend }}</span>
          </div>
          <div v-if="metadataModal.data.model" class="metadata-item">
            <span class="metadata-label">模型:</span>
            <span class="metadata-value">{{ metadataModal.data.model }}</span>
          </div>
          <div v-if="metadataModal.data.inputTokens" class="metadata-item">
            <span class="metadata-label">输入Token:</span>
            <span class="metadata-value">{{ metadataModal.data.inputTokens.toLocaleString() }}</span>
          </div>
          <div v-if="metadataModal.data.outputTokens" class="metadata-item">
            <span class="metadata-label">输出Token:</span>
            <span class="metadata-value">{{ metadataModal.data.outputTokens.toLocaleString() }}</span>
          </div>
          <div v-if="metadataModal.data.durationMs" class="metadata-item">
            <span class="metadata-label">耗时:</span>
            <span class="metadata-value">{{ (metadataModal.data.durationMs / 1000).toFixed(2) }}s</span>
          </div>
          <div v-if="metadataModal.data.costUsd" class="metadata-item">
            <span class="metadata-label">成本:</span>
            <span class="metadata-value">${{ metadataModal.data.costUsd.toFixed(6) }}</span>
          </div>
          <div v-if="metadataModal.data.sessionId" class="metadata-item metadata-copyable" @click="copyValue(metadataModal.data.sessionId, $event)">
            <span class="metadata-label">会话ID:</span>
            <div class="metadata-value-wrap">
              <span class="metadata-value metadata-session-id metadata-value-copyable">{{ metadataModal.data.sessionId }}</span>
              <button class="metadata-copy-btn" @click.stop="copyValue(metadataModal.data.sessionId, $event)" title="复制">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                </svg>
              </button>
            </div>
          </div>
          <div v-if="metadataModal.data.stopReason" class="metadata-item">
            <span class="metadata-label">停止原因:</span>
            <span class="metadata-value">{{ metadataModal.data.stopReason }}</span>
          </div>
          <div v-if="metadataModal.data.isError" class="metadata-item">
            <span class="metadata-label">错误:</span>
            <span class="metadata-value metadata-error">{{ metadataModal.data.errorMessage || '未知错误' }}</span>
          </div>
        </div>
      </div>
    </div>
  </Teleport>


  <!-- Session Drawer -->
  <SessionDrawer
    ref="sessionDrawerRef"
    :open="sessionDrawerOpen"
    :currentSessionId="currentSessionId"
    :runningSessionIds="runningSessions"
    @close="sessionDrawerOpen = false"
    @select="switchSession"
    @create="createSession"
    @delete="deleteSession"
  />

  <!-- Task Drawer -->
  <TaskDrawer
    ref="taskDrawerRef"
    :open="taskDrawerOpen"
    @close="taskDrawerOpen = false"
  />
</template>

<script setup>
import { ref, reactive, computed, watch, nextTick, onUnmounted, onMounted, inject } from 'vue'
import BottomSheet from './BottomSheet.vue'
import SessionDrawer from './SessionDrawer.vue'
import TaskDrawer from './TaskDrawer.vue'
import { escapeHtml, baseName, splitPath } from '@/utils/helpers.ts'
import { cancelChat } from '@/utils/api.ts'
import { marked, DOMPurify, hljs, mermaid } from '@/utils/globals.ts'
import { renderKatexInString, renderMermaidInElement } from '@/composables/useMarkdownRenderer.ts'
import { useToast } from '@/composables/useToast.ts'
import { useDoubleClickCopy } from '@/composables/useDoubleClickCopy.ts'
import { useFilePathAnnotation } from '@/composables/useFilePathAnnotation.ts'
import { useNotification } from '@/composables/useNotification.ts'
import { store } from '@/stores/app.ts'

const props = defineProps({
    open: Boolean,
    currentFile: Object,
})
const emit = defineEmits(['close', 'open', 'message'])

const messages = ref([])
const renderedContents = ref([])
const renderCache = new Map() // key: message content, value: rendered HTML
const RENDER_CACHE_MAX = 200

function trimRenderCache() {
    if (renderCache.size > RENDER_CACHE_MAX) {
        const keys = renderCache.keys()
        for (let i = 0; i < renderCache.size - RENDER_CACHE_MAX; i++) {
            renderCache.delete(keys.next().value)
        }
    }
}
const inputText = ref('')
const inputDisabled = ref(true)
const loading = ref(false)
const messagesRef = ref(null)
const fileInputRef = ref(null)
const sessionDrawerRef = ref(null)
const bottomSheetRef = ref(null)
const pendingFiles = ref([])
const attachedFiles = ref([]) // 附带的文件路径列表
const textareaRef = ref(null)
const metadataModal = ref({
  show: false,
  data: {},
  backend: '',
  createdAt: '',
  filePath: ''
})
let pollingInterval = null
let globalPollingInterval = null
let eventSource = null
let renderTimer = null
let streamTimeout = null // Timeout for detecting stale SSE connections
let reconnectAttempts = 0
const MAX_RECONNECT_ATTEMPTS = 3
const STREAM_TIMEOUT_MS = 60000 // 60 seconds without any SSE event = try reconnect
let lastScrollTime = 0 // Track last scroll time to reduce frequency
const toast = useToast()
const notification = useNotification()
const theme = inject('theme', ref('light'))
watch(theme, () => {
    renderCache.clear()
    updateRenderedContents(true)
})
const { handleDblClick } = useDoubleClickCopy()
const { annotateFilePaths, verifyFilePaths, openFilePath } = useFilePathAnnotation()

function handleFileTagClick(filePath) {
    if (filePath) {
        openFilePath(filePath)
        bottomSheetRef.value?.close()
    }
}

function handleChatClick(event) {
    // Check for file-open button click first
    const btn = (event.target).closest('.chat-file-open-btn')
    if (btn) {
        event.preventDefault()
        event.stopPropagation()
        const filePath = btn.getAttribute('data-file-path')
        if (filePath) {
            openFilePath(filePath)
            // Use BottomSheet's close method to trigger the exit animation
            bottomSheetRef.value?.close()
        }
        return
    }
    // Handle <a> link clicks (relative paths) + double-click copy
    handleDblClick(event, (href) => {
        openFilePath(href)
        bottomSheetRef.value?.close()
    })
}

function copyValue(value, event) {
    const wrap = event.currentTarget.closest('.metadata-value-wrap') || event.currentTarget
    const btn = wrap.querySelector?.('.metadata-copy-btn')
    const txt = wrap.querySelector?.('.metadata-session-id')
    const doCopy = () => {
        if (btn) { btn.classList.add('copied'); setTimeout(() => btn.classList.remove('copied'), 800) }
        if (txt) { txt.classList.add('copied'); setTimeout(() => txt.classList.remove('copied'), 800) }
        toast.show('已复制', { icon: '📋', duration: 1500 })
    }
    if (navigator.clipboard?.writeText) {
        navigator.clipboard.writeText(value).then(doCopy).catch(() => {
            const ta = document.createElement('textarea')
            ta.value = value
            ta.style.cssText = 'position:fixed;opacity:0'
            document.body.appendChild(ta)
            ta.select()
            document.execCommand('copy')
            document.body.removeChild(ta)
            doCopy()
        })
    } else {
        const ta = document.createElement('textarea')
        ta.value = value
        ta.style.cssText = 'position:fixed;opacity:0'
        document.body.appendChild(ta)
        ta.select()
        document.execCommand('copy')
        document.body.removeChild(ta)
        doCopy()
    }
}

// Track running sessions globally
const runningSessions = ref(new Set())

// Schedule proposals stored per message ID (stable key, not index-based)
const blockProposals = reactive({})

// Message count polling for scheduled task results
let msgCountInterval = null
const lastMsgCount = ref(0)

// Session management
const currentSessionId = ref('')
const currentSessionTitle = ref('')
const currentBackend = ref('')
const currentAgentId = ref('')
const sessionDrawerOpen = ref(false)
const taskDrawerOpen = ref(false)
const agents = ref([])

const agentHeaderTitle = computed(() => {
    const agent = agents.value.find(a => a.id === currentAgentId.value)
    if (agent) return `${agent.icon} ${agent.name}`
    return 'AI 对话'
})

// Extract schedule proposals from loaded messages for display only.
// Task creation happens exclusively during streaming (see renderTextBlock).
// Historical messages should never trigger task creation.
function extractScheduleProposals(msgs) {
    for (const msg of msgs) {
        if (msg.role === 'assistant' && msg.blocks && !msg.streaming) {
            for (let bi = 0; bi < msg.blocks.length; bi++) {
                const block = msg.blocks[bi]
                if (block.type === 'text') {
                    const proposalKey = `${msg.id}-${bi}`
                    if (blockProposals[proposalKey]) continue // already tracked
                    const proposalMatch = block.text.match(/<schedule-proposal(\s+confirmed)?>([\s\S]*?)<\/schedule-proposal>/)
                    if (proposalMatch) {
                        try {
                            const proposal = JSON.parse(proposalMatch[2].trim())
                            blockProposals[proposalKey] = { proposal }
                        } catch (e) {
                            console.error('Failed to parse schedule proposal:', e)
                        }
                    }
                }
            }
        }
    }
}

async function loadHistory() {
    expandedTools.value = {}
    expandedThinking.value = {}
    try {
        // Load agents first so we can resolve agent names
        if (agents.value.length === 0) await loadAgents()
        const url = currentSessionId.value
            ? `/api/ai/chat?session_id=${encodeURIComponent(currentSessionId.value)}`
            : '/api/ai/chat'
        const resp = await fetch(url)
        if (!resp.ok) {
            const errData = await resp.json().catch(() => ({}))
            throw new Error(errData.error || `请求失败 (${resp.status})`)
        }
        const data = await resp.json()
        messages.value = (data.messages || []).map(msg => {
            if (msg.role === 'assistant') {
                const { blocks, metadata, cancelled, scheduledTask } = parseAssistantContent(msg.content)
                msg.blocks = blocks
                if (metadata) msg.metadata = metadata
                if (cancelled) msg.cancelled = cancelled
                if (scheduledTask) msg.scheduledTask = scheduledTask
                if (msg.streaming) { msg.streaming = true; msg.fromDB = true }
            }
            return msg
        })
        renderCache.clear()
        currentSessionId.value = data.sessionId || ''
        currentSessionTitle.value = data.sessionTitle || ''
        currentBackend.value = data.backend || ''
        currentAgentId.value = data.agentId || ''
        console.log('loadHistory - agentId:', data.agentId, 'currentAgentId:', currentAgentId.value)
        extractScheduleProposals(messages.value)
        updateRenderedContents(true)
        if (data.running) {
            inputDisabled.value = true
            loading.value = true
            stopMsgCountPolling()
            scrollBottom()
            connectStream(currentSessionId.value)
        } else {
            inputDisabled.value = false
            loading.value = false
            startMsgCountPolling()
        }
    } catch (err) {
        console.error('Failed to load chat history:', err)
        toast.show(err.message || '加载聊天记录失败', { icon: '⚠️' })
    }
}

async function switchSession(sessionId) {
    disconnectStream()
    stopPolling()
    stopMsgCountPolling()
    expandedTools.value = {}
    expandedThinking.value = {}
    try {
        // Load agents first so we can resolve agent names
        if (agents.value.length === 0) await loadAgents()
        const resp = await fetch(`/api/ai/chat?session_id=${encodeURIComponent(sessionId)}`)
        if (!resp.ok) {
            toast.show('切换会话失败', { icon: '⚠️' })
            return
        }
        const data = await resp.json()
        messages.value = (data.messages || []).map(msg => {
            if (msg.role === 'assistant') {
                const { blocks, metadata, cancelled, scheduledTask } = parseAssistantContent(msg.content)
                msg.blocks = blocks
                if (metadata) msg.metadata = metadata
                if (cancelled) msg.cancelled = cancelled
                if (scheduledTask) msg.scheduledTask = scheduledTask
                if (msg.streaming) { msg.streaming = true; msg.fromDB = true }
            }
            return msg
        })
        renderCache.clear()
        currentSessionId.value = data.sessionId || sessionId
        currentSessionTitle.value = data.sessionTitle || ''
        currentBackend.value = data.backend || ''
        currentAgentId.value = data.agentId || ''
        extractScheduleProposals(messages.value)
        updateRenderedContents(true)
        if (data.running) {
            inputDisabled.value = true
            loading.value = true
            stopMsgCountPolling()
            scrollBottom()
            connectStream(sessionId)
        } else {
            inputDisabled.value = false
            loading.value = false
            startMsgCountPolling()
        }
        scrollBottom()
    } catch (err) {
        console.error('Failed to switch session:', err)
        toast.show('切换会话失败', { icon: '⚠️' })
    }
}

async function createSession(agentId) {
    try {
        const body = agentId ? { agentId } : {}
        const resp = await fetch('/api/ai/sessions', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        })
        const data = await resp.json()
        if (!resp.ok || !data.ok) {
            throw new Error(data.error || `创建失败 (${resp.status})`)
        }
        currentSessionId.value = data.sessionId
        currentSessionTitle.value = ''
        currentBackend.value = data.backend || ''
        currentAgentId.value = data.agentId || agentId || ''
        messages.value = []
        renderedContents.value = []
        Object.keys(blockProposals).forEach(k => delete blockProposals[k])
        inputDisabled.value = false
        loading.value = false
        toast.show('已创建新会话', { icon: '✨', duration: 1500 })
    } catch (err) {
        console.error('Failed to create session:', err)
        toast.show(err.message || '创建会话失败', { icon: '⚠️' })
    }
}

async function loadAgents() {
    try {
        const resp = await fetch('/api/agents')
        const data = await resp.json()
        agents.value = data.agents || []
    } catch (err) {
        console.error('Failed to load agents:', err)
    }
}

function getAgentIcon(agentId) {
    const agent = agents.value.find(a => a.id === agentId)
    return agent ? agent.icon : '🤖'
}

function getAgentName(agentId) {
    const agent = agents.value.find(a => a.id === agentId)
    return agent ? agent.name : (agentId || '助手')
}

function humanizeCron(expr) {
    const parts = expr.split(' ')
    if (parts.length !== 5) return expr
    const [min, hour, day, month, weekday] = parts
    if (min.startsWith('*/') && hour === '*') return `每 ${min.slice(2)} 分钟`
    if (hour.startsWith('*/') && min === '0') return `每 ${hour.slice(2)} 小时`
    if (min === '0' && !hour.includes('/') && day === '*' && month === '*' && weekday === '*') return `每天 ${hour}:00`
    if (min === '0' && weekday === '1-5') return `工作日 ${hour}:00`
    return expr
}

function repeatLabel(mode, maxRuns) {
    if (mode === 'once') return '单次执行'
    if (mode === 'limited') return `${maxRuns} 次后停止`
    return '不限次数'
}

function truncate(str, len) {
    if (!str) return ''
    const runes = [...str]
    return runes.length > len ? runes.slice(0, len).join('') + '...' : str
}

async function deleteSession(sessionId, backend) {
    try {
        const resp = await fetch(`/api/ai/session/delete?session_id=${encodeURIComponent(sessionId)}&backend=${encodeURIComponent(backend || '')}`, {
            method: 'DELETE',
        })
        const data = await resp.json()
        if (data.ok) {
            // If deleted current session, switch to another
            if (sessionId === currentSessionId.value) {
                const sessionsResp = await fetch('/api/ai/sessions')
                const sessionsData = await sessionsResp.json()
                if (sessionsData.sessions && sessionsData.sessions.length > 0) {
                    await switchSession(sessionsData.sessions[0].id, sessionsData.sessions[0].backend)
                } else {
                    // No sessions left, create a default one
                    await createSession()
                }
            }
            // Refresh session list in SessionManager
            sessionDrawerRef.value?.loadSessions()
            toast.show('会话已删除', { icon: '🗑️', duration: 2000 })
        }
    } catch (err) {
        console.error('Failed to delete session:', err)
        toast.show('删除会话失败', { icon: '⚠️' })
    }
}

function openSessionTab(tab) {
    if (tab === 'tasks') {
        taskDrawerOpen.value = true
    } else {
        sessionDrawerOpen.value = true
    }
}

function stopPolling() {
    if (pollingInterval) { clearInterval(pollingInterval); pollingInterval = null }
}

function stopGlobalPolling() {
    if (globalPollingInterval) { clearInterval(globalPollingInterval); globalPollingInterval = null }
}

// Poll for new messages from scheduled tasks when idle
function startMsgCountPolling() {
    stopMsgCountPolling()
    if (!currentSessionId.value) return
    lastMsgCount.value = messages.value.length
    msgCountInterval = setInterval(async () => {
        if (!currentSessionId.value || loading.value) return
        try {
            const resp = await fetch(`/api/ai/chat/count?session_id=${encodeURIComponent(currentSessionId.value)}`)
            if (!resp.ok) return
            const data = await resp.json()
            if (data.count > lastMsgCount.value) {
                lastMsgCount.value = data.count
                // Reload history to pick up new messages
                await loadHistory()
            }
        } catch (err) {
            // Silently ignore polling errors
        }
    }, 15000)
}

function stopMsgCountPolling() {
    if (msgCountInterval) { clearInterval(msgCountInterval); msgCountInterval = null }
}

async function cancelStream() {
    if (!currentSessionId.value || !loading.value) return
    try {
        await cancelChat(currentSessionId.value)
    } catch (err) {
        console.error('Failed to cancel:', err)
        // Force local state reset even if API call fails
        disconnectStream()
        inputDisabled.value = false
        loading.value = false
    }
}

function disconnectStream() {
    if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
    if (eventSource) {
        eventSource.close()
        eventSource = null
    }
}

function resetStreamTimeout() {
    if (streamTimeout) clearTimeout(streamTimeout)
    streamTimeout = setTimeout(() => {
        console.warn('SSE stream timeout - no events received, reconnecting')
        // No SSE event received for too long — reconnect instead of killing the session
        disconnectStream()
        // The AI session continues on the backend; just reconnect SSE
        if (currentSessionId.value && loading.value && reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
            reconnectAttempts++
            connectStream(currentSessionId.value)
        } else {
            // Too many reconnect attempts or session no longer active, fall back to polling
            const streamingMsg = messages.value.find(m => m.role === 'assistant' && m.streaming)
            if (streamingMsg) {
                delete streamingMsg.streaming
                updateRenderedContents(true)
            }
            inputDisabled.value = false
            loading.value = false
            pollUntilDone()
        }
    }, STREAM_TIMEOUT_MS)
}

function debouncedRender() {
    if (renderTimer) clearTimeout(renderTimer)
    renderTimer = window.setTimeout(() => {
        updateRenderedContents()
        // 减少scrollBottom调用频率，每500ms最多一次
        if (Date.now() - lastScrollTime > 500) {
            scrollBottom()
            lastScrollTime = Date.now()
        }
        renderTimer = null
    }, 200) // 增加防抖时间到200ms
}

// Global polling to monitor all running sessions
async function startGlobalPolling() {
    stopGlobalPolling()
    globalPollingInterval = setInterval(async () => {
        try {
            const resp = await fetch('/api/ai/sessions')
            const data = await resp.json()
            const sessions = data.sessions || []
            const newRunning = new Set(sessions.filter(s => s.running).map(s => s.id))
            
            // Check for completed sessions
            for (const sessionId of runningSessions.value) {
                if (!newRunning.has(sessionId)) {
                    if (sessionId === currentSessionId.value) {
                        // Current session completed but UI may be stuck in loading state
                        // (e.g. done event was dropped) — force reset
                        if (loading.value) {
                            loadHistory()
                        }
                    } else {
                        // Other session completed
                        const session = sessions.find(s => s.id === sessionId)
                        if (session) {
                            toast.show('会话已完成', {
                                icon: '✅',
                                duration: 5000,
                                onClick: () => {
                                    switchSession(sessionId, session.backend)
                                    emit('open')
                                }
                            })
                            // Also show browser notification for completed session
                            notification.show('会话已完成', {
                                body: '点击查看详情',
                                onClick: () => {
                                    switchSession(sessionId, session.backend)
                                    emit('open')
                                }
                            })
                        }
                    }
                }
            }
            
            runningSessions.value = newRunning
        } catch (err) {
            console.error('Global polling error:', err)
        }
    }, 2000)
}

async function pollUntilDone() {
    stopPolling()
    // Add current session to running set
    runningSessions.value = new Set([...runningSessions.value, currentSessionId.value])
    let jsonParseFailures = 0
    const MAX_JSON_PARSE_FAILURES = 5
    pollingInterval = setInterval(async () => {
        try {
            const resp = await fetch(`/api/ai/chat?session_id=${encodeURIComponent(currentSessionId.value)}`)
            if (!resp.ok) {
                // Non-2xx response — server error, stop polling
                throw new Error(`HTTP ${resp.status}`)
            }
            let data
            try {
                data = await resp.json()
                jsonParseFailures = 0 // Reset on success
            } catch {
                // JSON parse failed — server returned non-JSON (e.g. HTML error page)
                jsonParseFailures++
                if (jsonParseFailures >= MAX_JSON_PARSE_FAILURES) {
                    // Too many failures, give up
                    console.error('Polling: too many invalid JSON responses, giving up')
                    throw new Error('Invalid JSON response')
                }
                console.error('Polling: invalid JSON response')
                return
            }
            if (!data.running) {
                stopPolling()
                const updated = new Set(runningSessions.value)
                updated.delete(currentSessionId.value)
                runningSessions.value = updated
                messages.value = (data.messages || []).map(msg => {
                    if (msg.role === 'assistant') {
                        const { blocks, metadata, cancelled, scheduledTask } = parseAssistantContent(msg.content)
                        msg.blocks = blocks
                        if (metadata) msg.metadata = metadata
                        if (cancelled) msg.cancelled = cancelled
                        if (scheduledTask) msg.scheduledTask = scheduledTask
                    }
                    return msg
                })
                currentSessionId.value = data.sessionId || currentSessionId.value
                currentSessionTitle.value = data.sessionTitle || currentSessionTitle.value
                updateRenderedContents(true)
                inputDisabled.value = false
                loading.value = false
                emit('message')
                scrollBottom()
                // Show toast notification when AI replies and chat panel is not open
                if (!props.open) {
                    const lastMsg = messages.value[messages.value.length - 1]
                    if (lastMsg?.role === 'assistant') {
                        toast.show('AI 已回复', { icon: '🤖', duration: 5000, onClick: () => emit('open') })
                        // Also show browser notification
                        notification.show('AI 已回复', {
                            body: '点击查看回复详情',
                            onClick: () => emit('open')
                        })
                    }
                }
                return
            }
        } catch (err) {
            console.error('Polling error:', err)
            stopPolling()
            toast.show('连接失败，请刷新页面', { icon: '⚠️' })
            inputDisabled.value = false
            loading.value = false
        }
    }, 2000)
}

function connectStream(sessionId) {
    disconnectStream()
    stopPolling()
    reconnectAttempts = 0

    // Find existing streaming message or create a new one
    let lastIndex = messages.value.findIndex(m => m.role === 'assistant' && m.streaming)
    if (lastIndex === -1) {
        // No streaming message from DB — create empty assistant message
        messages.value.push({
            role: 'assistant',
            content: '',
            blocks: [],
            streaming: true,
            createdAt: new Date().toISOString(),
            backend: currentBackend.value
        })
        lastIndex = messages.value.length - 1
    }
    scrollBottom()

    // Guard: skip events if session changed or message was removed
    const guard = () => {
        if (currentSessionId.value !== sessionId) return false
        if (!messages.value[lastIndex]) return false
        return true
    }

    // Initialize currentText from existing text block (for reconnection)
    let currentText = ''
    const existingBlocks = messages.value[lastIndex]?.blocks || []
    if (existingBlocks.length > 0) {
        const lastBlock = existingBlocks[existingBlocks.length - 1]
        if (lastBlock?.type === 'text') {
            currentText = lastBlock.text || ''
        }
    }

    eventSource = new EventSource(`/api/ai/chat/stream?session_id=${encodeURIComponent(sessionId)}`)

    // Start stream timeout
    resetStreamTimeout()

    // Track if we've already created a task for this stream's proposal
    let proposalCreated = false

    eventSource.addEventListener('content', (e) => {
        if (!guard()) return
        resetStreamTimeout()
        const data = JSON.parse(e.data)
        currentText += data.content
        // Update or add text block at the end
        const blocks = messages.value[lastIndex].blocks
        const lastBlock = blocks[blocks.length - 1]
        if (lastBlock && lastBlock.type === 'text') {
            lastBlock.text = currentText
        } else {
            blocks.push({ type: 'text', text: currentText })
        }
        // Detect completed <schedule-proposal> tag during streaming and create task
        if (!proposalCreated && /<schedule-proposal(\s+confirmed)?>[\s\S]*?<\/schedule-proposal>/.test(currentText)) {
            const match = currentText.match(/<schedule-proposal(\s+confirmed)?>([\s\S]*?)<\/schedule-proposal>/)
            if (match) {
                try {
                    const proposal = JSON.parse(match[2].trim())
                    proposalCreated = true
                    createScheduledTask(proposal)
                } catch (err) {
                    console.error('Failed to parse schedule proposal:', err)
                }
            }
        }
        debouncedRender()
    })

    eventSource.addEventListener('thinking', (e) => {
        if (!guard()) return
        resetStreamTimeout()
        const data = JSON.parse(e.data)
        // Flush any pending text
        currentText = ''
        const blocks = messages.value[lastIndex].blocks
        // Append or extend thinking block
        const lastBlock = blocks[blocks.length - 1]
        if (lastBlock && lastBlock.type === 'thinking') {
            lastBlock.text += data.text
        } else {
            blocks.push({ type: 'thinking', text: data.text })
        }
        scrollBottom()
    })

    eventSource.addEventListener('tool_use', (e) => {
        resetStreamTimeout()
        const data = JSON.parse(e.data)
        if (!guard()) return
        // Flush any pending text
        currentText = ''
        const blocks = messages.value[lastIndex].blocks
        if (data.done) {
            // Find existing tool block by id and update
            const existing = blocks.find(b => b.type === 'tool_use' && b.id === data.id)
            if (existing) {
                existing.input = data.input || existing.input
                existing.done = true
            }
        } else {
            // New tool call
            blocks.push({ type: 'tool_use', name: data.name, id: data.id, input: data.input || {}, done: false })
        }
        scrollBottom()
    })

    eventSource.addEventListener('metadata', (e) => {
        if (!guard()) return
        resetStreamTimeout()
        const data = JSON.parse(e.data)
        messages.value[lastIndex].metadata = data
    })

    eventSource.addEventListener('done', () => {
        if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
        disconnectStream()
        // Reload from DB to ensure complete content — SSE events may have been
        // dropped during transmission, so the local state may be incomplete.
        loadHistory().finally(() => {
            inputDisabled.value = false
            loading.value = false
            emit('message')
            scrollBottom()
            if (!props.open) {
                const lastMsg = messages.value[messages.value.length - 1]
                if (lastMsg?.role === 'assistant') {
                    toast.show('AI 已回复', { icon: '🤖', duration: 5000, onClick: () => emit('open') })
                    // Also show browser notification
                    notification.show('AI 已回复', {
                        body: '点击查看回复详情',
                        onClick: () => emit('open')
                    })
                }
            }
        })
    })

    eventSource.addEventListener('cancelled', () => {
        if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
        disconnectStream()
        if (!guard()) return
        const msg = messages.value[lastIndex]
        msg.cancelled = true
        delete msg.streaming
        // If no content was received, add error block so the UI shows the error card instead of loading dots
        if ((!msg.blocks || msg.blocks.length === 0) && !msg.content) {
            msg.blocks = [{ type: 'error', text: '用户已中断' }]
        }
        updateRenderedContents()
        inputDisabled.value = false
        loading.value = false
    })

    eventSource.addEventListener('warning', (e) => {
        if (!guard()) return
        resetStreamTimeout()
        const data = JSON.parse(e.data)
        const msg = messages.value[lastIndex]
        // Flush any streaming text before adding warning block
        if (msg.streamingText) {
            msg.blocks.push({ type: 'text', text: msg.streamingText })
            msg.streamingText = ''
        }
        msg.blocks.push({ type: 'warning', text: data.text })
        updateRenderedContents()
    })

    eventSource.addEventListener('error', (e) => {
        if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
        if (!guard()) return
        disconnectStream()
        // Backend reported error (e.g. session not running) — reload from DB for final state
        loadHistory().catch(() => {
            if (!guard()) return
            const data = JSON.parse(e.data)
            messages.value[lastIndex].content = `错误: ${data.error}`
            messages.value[lastIndex].blocks = [{ type: 'error', text: data.error }]
            delete messages.value[lastIndex].streaming
            updateRenderedContents()
            inputDisabled.value = false
            loading.value = false
        })
    })

    eventSource.onerror = () => {
        // SSE connection error — reconnect if session is still active
        if (streamTimeout) { clearTimeout(streamTimeout); streamTimeout = null }
        disconnectStream()
        if (currentSessionId.value && loading.value && reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
            // AI session likely still running on backend, reconnect SSE
            reconnectAttempts++
            connectStream(currentSessionId.value)
        } else {
            // Too many attempts or session inactive — fall back to polling
            const streamingMsg = messages.value.find(m => m.role === 'assistant' && m.streaming)
            if (streamingMsg) {
                delete streamingMsg.streaming
                updateRenderedContents()
            }
            inputDisabled.value = false
            loading.value = false
            pollUntilDone()
        }
    }
}

async function handleFileSelect(e) {
    const files = Array.from(e.target.files || [])
    if (files.length === 0) return
    e.target.value = ''

    const maxFiles = store.state.uploadMaxFiles
    const remaining = maxFiles - pendingFiles.value.length
    if (remaining <= 0) {
        toast.show(`最多上传 ${maxFiles} 个文件`, { icon: '⚠️' })
        return
    }

    const toUpload = files.slice(0, remaining)
    if (files.length > remaining) {
        toast.show(`已选择 ${files.length} 个文件，但仅剩 ${remaining} 个名额`, { icon: '⚠️' })
    }

    const maxSizeBytes = store.state.uploadMaxSizeMB * 1024 * 1024

    for (const file of toUpload) {
        if (file.size > maxSizeBytes) {
            toast.show(`${file.name} 超过 ${store.state.uploadMaxSizeMB}MB 限制`, { icon: '⚠️' })
            continue
        }

        const isImage = file.type.startsWith('image/')
        const previewUrl = isImage ? URL.createObjectURL(file) : null

        const formData = new FormData()
        formData.append('file', file)

        try {
            const resp = await fetch('/api/upload/file', {
                method: 'POST',
                body: formData
            })
            const data = await resp.json()
            if (data.ok) {
                pendingFiles.value.push({
                    path: data.path,
                    previewUrl,
                    isImage
                })
            } else {
                if (previewUrl) URL.revokeObjectURL(previewUrl)
                toast.show('上传失败: ' + (data.error || '未知错误'), { icon: '⚠️' })
            }
        } catch (err) {
            if (previewUrl) URL.revokeObjectURL(previewUrl)
            toast.show('上传失败: ' + err.message, { icon: '⚠️' })
        }
    }
}

function removeFile(index) {
    const f = pendingFiles.value[index]
    if (f?.previewUrl) {
        URL.revokeObjectURL(f.previewUrl)
    }
    pendingFiles.value.splice(index, 1)
}

function addAttachedFile(filePath) {
    if (filePath && !attachedFiles.value.includes(filePath)) {
        attachedFiles.value.push(filePath)
    }
}

function removeAttachedFile(index) {
    attachedFiles.value.splice(index, 1)
}

function autoResizeTextarea() {
    const el = textareaRef.value
    if (!el) return
    el.style.height = 'auto'
    const lineHeight = parseFloat(getComputedStyle(el).lineHeight) || 20
    const maxHeight = lineHeight * 3 + 8 // ~3 lines + padding
    el.style.height = Math.min(el.scrollHeight, maxHeight) + 'px'
}

function collapseTextarea() {
    const el = textareaRef.value
    if (!el) return
    el.style.height = 'auto'
}

async function sendMessage() {
    const text = inputText.value.trim()
    const hasFiles = pendingFiles.value.length > 0 || attachedFiles.value.length > 0

    if ((!text && !hasFiles) || inputDisabled.value) return

    const filePaths = attachedFiles.value.length > 0 ? [...attachedFiles.value] : []
    const uploadedFiles = pendingFiles.value.map(f => ({ path: f.path }))
    const projectFiles = filePaths.map(p => ({ path: p }))

    messages.value.push({
        role: 'user',
        content: text,
        filePath: filePaths.length > 0 ? filePaths[0] : '',
        files: [...uploadedFiles, ...projectFiles],
        createdAt: new Date().toISOString()
    })

    updateRenderedContents()

    inputText.value = ''
    attachedFiles.value = []
    nextTick(() => collapseTextarea())
    // Clear pending files
    pendingFiles.value.forEach(f => {
        if (f.previewUrl) URL.revokeObjectURL(f.previewUrl)
    })
    pendingFiles.value = []

    inputDisabled.value = true
    loading.value = true
    scrollBottom(true)

    try {
        // Use currentAgentId as-is (backend will use default agent if empty)
        const effectiveAgentId = currentAgentId.value

        const url = currentSessionId.value
            ? `/api/ai/chat?session_id=${encodeURIComponent(currentSessionId.value)}`
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
        if (data.sessionId && !currentSessionId.value) {
            currentSessionId.value = data.sessionId
        }
        // Session already running — another request is in progress
        if (data.running) {
            loading.value = true
            inputDisabled.value = true
            connectStream(currentSessionId.value)
            return
        }
        connectStream(currentSessionId.value)
    } catch (err) {
        stopPolling()
        disconnectStream()
        messages.value.push({ role: 'assistant', content: `错误: ${err.message}`, file_path: '' })
        inputDisabled.value = false
        loading.value = false
        toast.show('发送失败，请重试', { icon: '⚠️' })
        // Clear session ID on error to prevent using invalid session
        if (err.message?.includes('Session backend not found') || err.message?.includes('session not found')) {
            currentSessionId.value = ''
        }
    }
}

function scrollBottom(force = false) {
    nextTick(() => {
        if (!messagesRef.value) return
        const el = messagesRef.value
        // Only auto-scroll if user is near the bottom, or force=true (e.g. user sent a message)
        if (force || el.scrollHeight - el.scrollTop - el.clientHeight < 60) {
            el.scrollTop = el.scrollHeight
        }
    })
}

function parseAssistantContent(content) {
    if (!content) return { blocks: [], metadata: null, scheduledTask: null }
    try {
        const parsed = JSON.parse(content)
        if (parsed.blocks && Array.isArray(parsed.blocks)) {
            // Mark tool_use blocks from DB as done (they're complete when loaded from storage)
            return {
                blocks: parsed.blocks.map(b => {
                    if (b.type === 'tool_use') b.done = true
                    return b
                }),
                metadata: parsed.metadata || null,
                cancelled: parsed.cancelled || false,
                scheduledTask: parsed.scheduledTask || null
            }
        }
    } catch {}
    // Plain text fallback
    return { blocks: [{ type: 'text', text: content }], metadata: null, scheduledTask: null }
}

function renderMarkdown(text) {
    let html = marked.parse((text || '').trim())
    html = renderKatexInString(html)
    html = DOMPurify.sanitize(html, { ADD_TAGS: ['math', 'button'], ADD_ATTR: ['data-file-path', 'title'] })
    html = html.replace(/<table>/g, '<div class="table-wrap"><table>').replace(/<\/table>/g, '</table></div>')
    html = html.replace(/<img([^>]*)>/g, (match, attrs) => {
        let cleanAttrs = attrs.replace(/\s*style="[^"]*"/i, '').replace(/\s*class="[^"]*"/i, '')
        return `<img${cleanAttrs} style="max-width: 200px; max-height: 200px; object-fit: cover; border-radius: 6px; margin: 4px 0; cursor: pointer;" class="chat-img-thumbnail">`
    })
    const audioExts = ['.mp3', '.wav', '.ogg', '.m4a', '.aac', '.flac', '.wma', '.opus']
    html = html.replace(/<a href="([^"]+)">([^<]*)<\/a>/g, (match, href, text) => {
        const lower = href.toLowerCase()
        if (audioExts.some(ext => lower.endsWith(ext))) {
            return `<div class="chat-audio-wrapper"><audio src="${href}" controls class="chat-audio-player"></audio></div>`
        }
        return match
    })
    const { html: annotatedHtml, detectedPaths } = annotateFilePaths(html, { projectRoot: store.state.projectRoot })
    html = annotatedHtml
    if (detectedPaths.length > 0) {
        const uniquePaths = [...new Set(detectedPaths)]
        nextTick(() => {
            const el = document.getElementById('aiChatMessages')
            if (el) verifyFilePaths(uniquePaths, el)
        })
    }
    return html
}

function renderTextBlock(text, msgId, blockIdx) {
    // Pure rendering — no side effects. Task creation is handled in SSE content event.
    const proposalMatch = text.match(/<schedule-proposal(\s+confirmed)?>([\s\S]*?)<\/schedule-proposal>/)
    if (proposalMatch) {
        const proposalKey = `${msgId}-${blockIdx}`
        if (!blockProposals[proposalKey]) {
            try {
                const proposal = JSON.parse(proposalMatch[2].trim())
                blockProposals[proposalKey] = { proposal }
            } catch (e) {
                console.error('Failed to parse schedule proposal:', e)
            }
        }
        const cleanText = text.replace(/<schedule-proposal(\s+confirmed)?>[\s\S]*?<\/schedule-proposal>/, '').trim()
        return cleanText ? renderMarkdown(cleanText) : ''
    }
    return renderMarkdown(text)
}

async function createScheduledTask(proposal) {
    try {
        const body = { ...proposal, session_id: currentSessionId.value || undefined }
        const resp = await fetch('/api/tasks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        })
        const data = await resp.json()
        if (resp.ok && data.ok) {
            toast.show('定时任务已创建', { icon: '✅', duration: 2000 })
        } else {
            toast.show('任务创建失败: ' + (data.error || resp.statusText), { icon: '⚠️' })
        }
    } catch (err) {
        toast.show('任务创建失败: ' + err.message, { icon: '⚠️' })
    }
}

function renderMsg(msg) {
    return renderMarkdown(msg.content)
}

function hasImagesInContent(content) {
    return content && content.includes('![')
}

function normalizeFileEntry(f) {
    if (typeof f === 'string') return { path: f }
    return { path: f.path || '' }
}

function isUploadPath(path) {
    return path.startsWith('.clawbench/uploads/') || path.startsWith('.clawbench\\uploads\\')
}

function isImageFile(path) {
    if (!path) return false
    const imageExts = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg', '.bmp', '.ico', '.tiff', '.tif', '.avif']
    const lower = path.toLowerCase()
    return imageExts.some(ext => lower.endsWith(ext))
}

function getFileName(path) {
    return baseName(path)
}

// Generate a human-readable summary for a tool call block
function toolCallSummary(block) {
    if (!block.input) return ''
    const obj = block.input
    if (obj.file_path) return baseName(obj.file_path)
    if (obj.command) return obj.command.length > 60 ? obj.command.slice(0, 57) + '...' : obj.command
    if (obj.path) return baseName(obj.path)
    if (obj.src_path && obj.dst_path) return `${baseName(obj.src_path)} → ${baseName(obj.dst_path)}`
    const firstVal = Object.values(obj)[0]
    if (typeof firstVal === 'string' && firstVal.length < 80) return firstVal
    return ''
}

// Toggle tool call detail expansion
const expandedTools = ref({})
const expandedThinking = ref({})
function toggleToolDetail(key) {
    expandedTools.value[key] = !expandedTools.value[key]
}
function toggleThinking(key) {
    expandedThinking.value[key] = !expandedThinking.value[key]
}

function formatToolInput(input) {
    if (!input) return ''
    try {
        const json = JSON.stringify(input, null, 2)
        return hljs.highlight(json, { language: 'json' }).value
    } catch {
        return JSON.stringify(input, null, 2)
    }
}

function updateRenderedContents(forceFullRender = false) {
    if (forceFullRender) {
        // 全量渲染：用于加载历史、主题变化、切换会话等场景
        renderedContents.value = messages.value.map(msg => {
            if (msg.role === 'assistant' && msg.blocks) {
                return ''
            }
            const key = msg.content || ''
            if (key && renderCache.has(key)) {
                return renderCache.get(key)
            }
            const html = renderMsg(msg)
            if (key) {
                renderCache.set(key, html)
                trimRenderCache()
            }
            return html
        })
        nextTick(() => {
            const el = document.getElementById('aiChatMessages')
            if (el) renderMermaidInElement(el, 'chat-mermaid')
        })
    } else {
        // 增量渲染：只渲染新增的消息（流式输出场景）
        const startIdx = renderedContents.value.length
        const newMsgs = messages.value.slice(startIdx)
        
        if (newMsgs.length === 0) return
        
        // 只渲染新消息
        const newContents = newMsgs.map(msg => {
            if (msg.role === 'assistant' && msg.blocks) {
                return ''
            }
            const key = msg.content || ''
            if (key && renderCache.has(key)) {
                return renderCache.get(key)
            }
            const html = renderMsg(msg)
            if (key) {
                renderCache.set(key, html)
                trimRenderCache()
            }
            return html
        })
        
        renderedContents.value = [...renderedContents.value, ...newContents]
        
        // 只为新增内容渲染Mermaid（如果它们在视口中）
        nextTick(() => {
            const el = document.getElementById('aiChatMessages')
            if (el) {
                // 使用IntersectionObserver只为可见的mermaid块渲染
                const newBlocks = el.querySelectorAll(`.chat-message:nth-last-child(n+${startIdx + 1}) pre.mermaid:not([data-rendered])`)
                if (newBlocks.length > 0) {
                    renderMermaidInElement(el, 'chat-mermaid', newBlocks)
                }
            }
        })
    }
}

function formatMessageTime(createdAt) {
    const date = new Date(createdAt)
    const now = new Date()
    const diffMs = now - date
    const diffMins = Math.floor(diffMs / 60000)

    if (diffMins < 1) return '刚刚'
    if (diffMins < 60) return `${diffMins}分钟前`

    const diffHours = Math.floor(diffMins / 60)
    if (diffHours < 24) return `${diffHours}小时前`

    const diffDays = Math.floor(diffHours / 24)
    if (diffDays < 7) return `${diffDays}天前`

    // More than a week ago, show date
    const month = date.getMonth() + 1
    const day = date.getDate()
    const hour = date.getHours().toString().padStart(2, '0')
    const minute = date.getMinutes().toString().padStart(2, '0')
    return `${month}/${day} ${hour}:${minute}`
}

function formatDetailTime(createdAt) {
    const date = new Date(createdAt)
    const year = date.getFullYear()
    const month = (date.getMonth() + 1).toString().padStart(2, '0')
    const day = date.getDate().toString().padStart(2, '0')
    const hour = date.getHours().toString().padStart(2, '0')
    const minute = date.getMinutes().toString().padStart(2, '0')
    const second = date.getSeconds().toString().padStart(2, '0')
    return `${year}-${month}-${day} ${hour}:${minute}:${second}`
}

function showMetadata(msg) {
    metadataModal.value.data = msg.metadata || {}
    metadataModal.value.backend = msg.backend || ''
    metadataModal.value.createdAt = msg.createdAt || ''
    metadataModal.value.filePath = msg.filePath || ''
    metadataModal.value.show = true
}

// Handle page visibility change (mobile lock/unlock)
function handleVisibilityChange() {
    if (document.visibilityState === 'visible' && loading.value) {
        // Page became visible while streaming - reconnect
        disconnectStream()
        stopPolling()
        loadHistory().catch(() => {
            // loadHistory failed — reset loading state so user isn't stuck
            inputDisabled.value = false
            loading.value = false
        })
    }
}

// Start global polling when component mounts
onMounted(() => {
    // Request notification permission on mount
    notification.requestPermission().catch(err => {
        console.warn('Failed to request notification permission:', err)
    })

    startGlobalPolling()
    document.addEventListener('visibilitychange', handleVisibilityChange)
})

// Cleanup preview URLs on unmount
onUnmounted(() => {
    pendingFiles.value.forEach(f => {
        if (f.previewUrl) URL.revokeObjectURL(f.previewUrl)
    })
    stopPolling()
    stopGlobalPolling()
    stopMsgCountPolling()
    document.removeEventListener('visibilitychange', handleVisibilityChange)
    notification.closeAll()
})

watch(() => props.open, async (val) => {
    if (val) {
        await loadHistory()
    }
})
</script>

<style scoped>
.chat-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.chat-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.chat-header-row .bs-title {
  font-weight: 600;
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  flex-shrink: 0;
}

.ai-backend-label {
  font-size: 11px !important;
  color: var(--text-muted, #999) !important;
  background: var(--bg-tertiary, #f0f0f0) !important;
  padding: 2px 8px !important;
  border-radius: 10px !important;
  text-transform: capitalize !important;
  flex-shrink: 0;
}

.ai-backend-label.clickable {
  cursor: pointer;
  transition: all 0.15s;
}

.ai-backend-label.clickable:hover {
  color: var(--text-primary, #1a1a1a) !important;
  background: var(--bg-secondary, #e0e0e0) !important;
}

.chat-session-title {
  flex: 1;
  font-size: 13px;
  font-weight: 400;
  color: var(--text-muted, #999);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

/* ── Messages ── */
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

.placeholder-dots {
  display: flex;
  gap: 4px;
  align-items: center;
  padding: 8px 0 4px;
}
.placeholder-dots span {
  width: 7px; height: 7px;
  border-radius: 50%;
  background: var(--text-muted, #999);
  animation: dot-bounce 1.2s infinite ease-in-out;
}
.placeholder-dots span:nth-child(1) { animation-delay: 0s; }
.placeholder-dots span:nth-child(2) { animation-delay: 0.2s; }
.placeholder-dots span:nth-child(3) { animation-delay: 0.4s; }

@keyframes dot-bounce {
  0%, 80%, 100% { transform: scale(0.6); opacity: 0.4; }
  40% { transform: scale(1); opacity: 1; }
}

/* ── Unified input container ── */
.chat-input-container {
  display: flex;
  flex-direction: column;
  background: var(--bg-primary, #fff);
  flex-shrink: 0;
  margin: 0 8px 8px;
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 12px;
  overflow: hidden;
}

/* Toolbar row */
.chat-toolbar {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px 6px 0;
}

.chat-toolbar-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted, #999);
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
  flex-shrink: 0;
}

.chat-toolbar-btn:hover:not(:disabled) {
  color: var(--accent-color, #0066cc);
  background: var(--bg-tertiary, #f0f0f0);
}

.chat-toolbar-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Attachment tags row */
.chat-attachment-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 4px 8px;
}

/* Input area attachment tags */
.chat-attachment-tags .chat-file-attachment {
  max-width: 120px;
}

.chat-attachment-tags .attachment-upload {
  background: color-mix(in srgb, var(--accent-color, #0066cc) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--accent-color, #0066cc) 20%, transparent);
  color: var(--accent-color, #0066cc);
  cursor: default;
}

.chat-attachment-tags .attachment-upload .chat-file-name {
  color: var(--accent-color, #0066cc);
}

.chat-attachment-tags .attachment-upload svg {
  stroke: var(--accent-color, #0066cc);
}

.chat-attachment-tags .attachment-upload:hover {
  background: color-mix(in srgb, var(--accent-color, #0066cc) 18%, transparent);
}

.chat-attachment-tags .attachment-ref {
  background: color-mix(in srgb, var(--text-muted, #999) 8%, transparent);
  border: 1px dashed var(--text-secondary, #666);
  color: var(--text-secondary, #666);
}

.chat-attachment-tags .attachment-ref .chat-file-name {
  color: var(--text-secondary, #666);
}

.chat-attachment-tags .attachment-ref svg {
  stroke: var(--text-secondary, #666);
}

.chat-attachment-tags .attachment-ref:hover {
  background: color-mix(in srgb, var(--text-muted, #999) 15%, transparent);
}

.attachment-tag-remove {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted, #999);
  padding: 0;
  font-size: 14px;
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 2px;
  transition: color 0.15s, background 0.15s;
}

.attachment-tag-remove:hover {
  color: var(--danger-color, #dc3545);
  background: color-mix(in srgb, var(--danger-color, #dc3545) 10%, transparent);
}

/* Input row */
.chat-input-row {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  padding: 4px 6px 6px;
}

.chat-textarea {
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
  max-height: 68px; /* ~3 lines */
  font-family: inherit;
}

.chat-textarea::placeholder {
  color: var(--text-muted, #999);
}

.chat-textarea:disabled {
  opacity: 0.5;
}

.chat-send-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: var(--accent-color, #0066cc);
  color: #fff;
  border: none;
  border-radius: 50%;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
  flex-shrink: 0;
}
.chat-send-btn:hover { background: #0055aa; }
.chat-send-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.chat-send-btn.disabled { opacity: 0.5; cursor: not-allowed; }

/* Stop button */
.chat-stop-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: var(--danger-color, #dc3545);
  color: #fff;
  border: none;
  border-radius: 50%;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
  flex-shrink: 0;
}
.chat-stop-btn:hover { opacity: 0.85; }

/* Cancelled marker */
.chat-cancelled-mark {
  display: inline-block;
  font-size: 11px;
  color: var(--text-muted, #999);
  background: var(--bg-tertiary, #f0f0f0);
  padding: 2px 8px;
  border-radius: 4px;
  margin-top: 4px;
}

.chat-error-card {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  margin: 2px 0;
  border-left: 3px solid #ef4444;
  background: rgba(239, 68, 68, 0.08);
}

.chat-error-card .error-icon {
  flex-shrink: 0;
  color: #ef4444;
}

.chat-error-card .error-text {
  font-size: 12px;
  font-weight: 500;
  color: #dc2626;
}

:root[data-theme="dark"] .chat-error-card {
  border-left-color: #f87171;
  background: rgba(248, 113, 113, 0.1);
}

:root[data-theme="dark"] .chat-error-card .error-icon {
  color: #f87171;
}

:root[data-theme="dark"] .chat-error-card .error-text {
  color: #fca5a5;
}

.chat-warning-card {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  margin: 2px 0;
  border-left: 3px solid #f59e0b;
  background: rgba(245, 158, 11, 0.08);
}

.chat-warning-card .warning-icon {
  flex-shrink: 0;
  color: #f59e0b;
}

.chat-warning-card .warning-text {
  font-size: 12px;
  font-weight: 500;
  color: #d97706;
  white-space: pre-wrap;
  word-break: break-word;
}

:root[data-theme="dark"] .chat-warning-card {
  border-left-color: #fbbf24;
  background: rgba(251, 191, 36, 0.1);
}

:root[data-theme="dark"] .chat-warning-card .warning-icon {
  color: #fbbf24;
}

:root[data-theme="dark"] .chat-warning-card .warning-text {
  color: #fcd34d;
}

/* Audio player in chat */
.chat-audio-wrapper {
  margin: 8px 0;
}

.chat-audio-player {
  width: 100%;
  max-width: 280px;
  height: 36px;
  border-radius: var(--radius-sm);
  outline: none;
}

/* Style images in user messages - now handled by inline styles */
/* .chat-message.user img rules removed to allow inline thumbnail styles */

/* ── File attachment in messages ── */
.chat-files {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin: 4px 0;
}

/* Common file tag styles - shared by both current file and uploaded attachments */
.chat-file-tag,
.chat-file-attachment {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  border-radius: 8px;
  padding: 1px 6px;
  margin-bottom: 4px;
  font-size: 11px;
  text-decoration: none;
  cursor: pointer;
  transition: opacity 0.15s;
  white-space: nowrap;
  max-width: 120px;
}

.chat-file-tag-icon,
.chat-file-attachment svg {
  flex-shrink: 0;
}

.chat-file-tag-path,
.chat-file-name {
  font-family: monospace;
  flex: 1;
  min-width: 0;
  overflow-x: auto;
  overflow-y: hidden;
  white-space: nowrap;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.chat-file-tag-path::-webkit-scrollbar,
.chat-file-name::-webkit-scrollbar {
  display: none;
}

/* Upload tags in input area: not clickable */
.chat-attachment-tags .attachment-upload {
  cursor: default;
}

/* User message: common colors */
.chat-message.user .chat-file-tag,
.chat-message.user .chat-file-attachment {
  color: rgba(255, 255, 255, 0.95);
}

.chat-message.user .chat-file-tag-path,
.chat-message.user .chat-file-name {
  color: rgba(255, 255, 255, 0.95);
}

.chat-message.user .chat-file-tag-icon,
.chat-message.user .chat-file-attachment svg {
  stroke: rgba(255, 255, 255, 0.95);
}

/* User message: uploaded - solid border */
.chat-message.user .attachment-upload {
  background: rgba(255, 255, 255, 0.15);
  border: 1px solid rgba(255, 255, 255, 0.35);
}

/* User message: referenced - dashed border */
.chat-message.user .attachment-ref {
  background: rgba(255, 255, 255, 0.15);
  border: 1px dashed rgba(255, 255, 255, 0.6);
}

.chat-message.user .attachment-ref:hover,
.chat-message.user .chat-file-tag:hover {
  background: rgba(255, 255, 255, 0.25);
}

/* Assistant message: common colors */
.chat-message.assistant .chat-file-tag,
.chat-message.assistant .chat-file-attachment {
  color: var(--text-secondary);
}

.chat-message.assistant .chat-file-tag-path,
.chat-message.assistant .chat-file-name {
  color: var(--text-secondary);
}

.chat-message.assistant .chat-file-tag-icon,
.chat-message.assistant .chat-file-attachment svg {
  stroke: var(--text-secondary);
}

/* Assistant message: uploaded - solid border */
.chat-message.assistant .attachment-upload {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
}

/* Assistant message: referenced - dashed border */
.chat-message.assistant .attachment-ref {
  background: color-mix(in srgb, var(--text-muted, #999) 8%, transparent);
  border: 1px dashed var(--text-secondary);
}

.chat-message.assistant .attachment-ref:hover,
.chat-message.assistant .chat-file-tag:hover {
  background: var(--bg-secondary);
}

/* Tool calls display */
.chat-tool-call {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  color: var(--text-secondary);
  background: var(--bg-secondary);
  padding: 3px 8px;
  border-radius: 4px;
  cursor: pointer;
  width: 100%;
  margin-top: 4px;
  overflow: hidden;
}

/* Thinking block */
.chat-thinking {
  background: color-mix(in srgb, var(--accent-color, #0066cc) 6%, transparent);
  border: 1px solid color-mix(in srgb, var(--accent-color, #0066cc) 15%, transparent);
  border-radius: 6px;
  margin: 4px 0;
  cursor: pointer;
  overflow: hidden;
}

.thinking-header {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 5px 8px;
  font-size: 12px;
  color: var(--text-secondary);
}

.thinking-label {
  font-weight: 500;
}

.thinking-chevron {
  margin-left: auto;
  transition: transform 0.2s;
}

.chat-thinking.expanded .thinking-chevron {
  transform: rotate(180deg);
}

.thinking-text {
  margin: 0;
  padding: 6px 8px;
  font-size: 11px;
  line-height: 1.5;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
  border-top: 1px solid color-mix(in srgb, var(--accent-color, #0066cc) 10%, transparent);
  max-height: 200px;
  overflow-y: auto;
  font-family: inherit;
}

.chat-tool-call:hover {
  background: color-mix(in srgb, var(--bg-secondary) 80%, var(--text-secondary));
}

.chat-tool-call .tool-icon {
  opacity: 0.6;
  flex-shrink: 0;
}

.chat-tool-call .tool-name {
  font-weight: 500;
}

.chat-tool-call .tool-summary {
  color: var(--text-tertiary, #888);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chat-tool-call .tool-check {
  color: #22c55e;
  flex-shrink: 0;
}

.tool-detail {
  margin: 2px 0 4px 0;
  padding: 6px 8px;
  font-size: 11px;
  line-height: 1.4;
  background: var(--bg-primary);
  border-radius: 4px;
  border: 1px solid var(--border-color);
  white-space: pre;
  overflow-x: auto;
  max-height: 150px;
  cursor: default;
}

.tool-spinner {
  width: 10px;
  height: 10px;
  border: 1.5px solid var(--border-color);
  border-top-color: var(--text-secondary);
  border-radius: 50%;
  animation: tool-spin 0.6s linear infinite;
  flex-shrink: 0;
}

@keyframes tool-spin {
  to { transform: rotate(360deg); }
}

/* Image thumbnails in user messages */
.chat-image-thumb {
  max-width: 80px;
  max-height: 80px;
  object-fit: cover;
  border-radius: 6px;
  display: block;
}

/* Image thumbnail style */
.chat-message .chat-img-thumbnail {
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
}

.chat-message .chat-img-thumbnail:hover {
  transform: scale(1.02);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.schedule-proposal-card {
  margin: 8px 0;
  border: 1px solid var(--accent-color, #0066cc);
  border-radius: 8px;
  overflow: hidden;
  background: var(--bg-primary, #fff);
}

.schedule-proposal-card.confirmed {
  border-color: #4caf50;
  opacity: 0.85;
}

.proposal-header {
  background: var(--accent-color, #0066cc);
  color: #fff;
  padding: 8px 12px;
  font-size: 13px;
  font-weight: 600;
}

.proposal-header.confirmed {
  background: #4caf50;
}

.proposal-body {
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.6;
}

.proposal-row {
  margin-bottom: 4px;
}

.proposal-row strong {
  color: var(--text-secondary, #666);
}


</style>

<style>
/* Chat message - non-scoped for v-html penetration */
.chat-message {
    padding: 8px 12px;
    border-radius: var(--radius-md);
    font-size: 13px;
    line-height: 1.4;
    min-width: 0;
    word-wrap: break-word;
    overflow-wrap: break-word;
    word-break: break-word;
    max-width: 100%;
    box-sizing: border-box;
}

.chat-message.user {
    background: var(--user-msg-color);
    color: white;
    align-self: flex-end;
    border-radius: 16px 16px 0 16px;
}

.chat-message.assistant {
    background: var(--bg-tertiary);
    color: var(--text-primary);
    align-self: stretch;
    border-radius: 16px 16px 16px 0;
    position: relative;
}

.chat-message.assistant pre {
    padding: 10px;
    margin: 6px 0;
    border-radius: var(--radius-sm);
}

.chat-message.assistant code {
    padding: 2px 6px;
    font-size: 13px;
}

.chat-message.assistant h1,
.chat-message.assistant h2,
.chat-message.assistant h3 {
    margin: 6px 0 3px;
    font-weight: 600;
}

.chat-message.assistant h1 { font-size: 16px; }
.chat-message.assistant h2 { font-size: 14px; }
.chat-message.assistant h3 { font-size: 13px; }

.chat-message.assistant p {
    margin: 3px 0;
}

.chat-message.assistant ul,
.chat-message.assistant ol {
    margin: 6px 0;
}

.chat-message.assistant blockquote {
    margin: 6px 0;
    padding: 5px 10px;
}

.chat-message.assistant img {
    margin: 6px 0;
}

.chat-message.assistant hr {
    margin: 8px 0;
}

.chat-message.assistant .table-wrap {
    overflow-x: auto;
    border: none;
    border-radius: 6px;
    margin: 0.75em 0;
}

.chat-message.assistant table {
    display: block;
    margin: 0;
}

.chat-message.assistant th {
    font-size: 13px;
    color: var(--text-primary);
}

.chat-message.assistant td {
    white-space: nowrap;
}

/* Mermaid diagram thumbnail */
.chat-message .mermaid {
  max-width: 200px;
  max-height: 200px;
  overflow: hidden;
  border-radius: 6px;
  margin: 4px 0;
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
  background: var(--bg-secondary);
  padding: 8px;
}

.chat-message .mermaid:hover {
  transform: scale(1.02);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.chat-message .mermaid svg {
  max-width: 100%;
  max-height: 184px;
  height: auto;
}

/* Chat Meta Bar — contains model/duration info + detail button */
.chat-meta-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 4px;
    gap: 6px;
}

.chat-meta-info {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--text-secondary);
    opacity: 0.7;
    min-width: 0;
    overflow: hidden;
}

.chat-meta-sep::before {
    content: '·';
    margin-right: 6px;
}

/* Scheduled Task Trigger Banner */
.chat-scheduled-banner {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    margin-bottom: 6px;
    border-radius: var(--radius-sm, 6px);
    background: color-mix(in srgb, var(--accent-color, #0066cc) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--accent-color, #0066cc) 15%, transparent);
    font-size: 11px;
    color: var(--accent-color, #0066cc);
    flex-wrap: wrap;
}

.chat-scheduled-banner svg {
    flex-shrink: 0;
    opacity: 0.7;
}

.scheduled-label {
    font-weight: 600;
    white-space: nowrap;
}

.scheduled-task-name {
    font-weight: 500;
    opacity: 0.85;
}

.scheduled-sep {
    opacity: 0.4;
}

.scheduled-agent,
.scheduled-cron {
    opacity: 0.7;
    white-space: nowrap;
}

/* Chat Info Button */
.chat-info-btn {
    flex-shrink: 0;
    width: 22px;
    height: 22px;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.5;
    transition: opacity 0.2s, background 0.2s;
}

.chat-info-btn:hover {
    opacity: 1;
    background: var(--bg-tertiary);
}

.chat-info-btn svg {
    width: 14px;
    height: 14px;
}

/* Metadata Modal */
.metadata-modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2500;
    animation: fadeIn 0.15s ease;
}

.metadata-modal {
    background: var(--bg-primary);
    border-radius: 8px;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.15);
    max-width: 480px;
    width: 90%;
    max-height: 80vh;
    overflow: hidden;
    animation: slideUp 0.2s ease;
}

.metadata-modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-color);
}

.metadata-modal-header h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
}

.metadata-close-btn {
    width: 28px;
    height: 28px;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    font-size: 24px;
    cursor: pointer;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.15s;
}

.metadata-close-btn:hover {
    background: var(--bg-tertiary);
}

.metadata-content {
    padding: 16px 20px;
    overflow-y: auto;
    max-height: calc(80vh - 60px);
}

.metadata-item {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 10px 0;
    border-bottom: 1px solid var(--border-color);
}

.metadata-item:last-child {
    border-bottom: none;
}

.metadata-label {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary);
    min-width: 90px;
    flex-shrink: 0;
}

.metadata-value {
    font-size: 13px;
    color: var(--text-primary);
    word-break: break-all;
}

.metadata-session-id {
    font-family: monospace;
    font-size: 12px;
    background: var(--bg-tertiary);
    padding: 2px 6px;
    border-radius: 3px;
}

.metadata-value-wrap {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
}

.metadata-value-copyable {
    cursor: pointer;
}

.metadata-copyable {
    user-select: none;
}

.metadata-copyable:hover {
    background: var(--bg-tertiary, #f5f5f5);
}

.metadata-value-copyable:hover {
    color: var(--accent-color, #4a90d9);
}

.metadata-value-copyable.copied {
    color: #22c55e;
}

.metadata-error {
    color: #ef4444;
    word-break: break-all;
}

.metadata-copy-btn {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    background: none;
    border: none;
    cursor: pointer;
    color: var(--text-muted, #999);
    padding: 2px;
    border-radius: 3px;
    transition: color 0.15s, background 0.15s;
}

.metadata-copy-btn:hover {
    color: var(--accent-color, #4a90d9);
    background: var(--bg-tertiary, #f0f0f0);
}

.metadata-copy-btn.copied {
    color: #22c55e;
}

@keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
}

@keyframes slideUp {
    from {
        opacity: 0;
        transform: translateY(10px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}
</style>

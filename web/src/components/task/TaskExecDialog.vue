<template>
  <ModalDialog :open="open" :title="view === 'detail' ? '' : t('task.exec.title')" full-height @close="handleClose">
    <template #header>
      <!-- Detail view: back arrow + time -->
      <template v-if="view === 'detail' && selectedExec">
        <button class="back-btn" @click.stop="view = 'list'" :title="t('nav.prevFile')">
          <ChevronLeft :size="16" />
        </button>
        <span class="modal-title">{{ formatAbsoluteTime(selectedExec.createdAt) }}</span>
      </template>
      <!-- List view: icon + task name -->
      <template v-else>
        <Clock :size="16" class="modal-header-icon" />
        <span class="modal-title">{{ task?.name || t('task.exec.title') }}</span>
      </template>
    </template>

    <!-- List view -->
    <div v-if="view === 'list'" class="executions-content">
      <div v-if="loading" class="dialog-empty">{{ t('common.loading') }}</div>
      <div v-else-if="executions.length === 0 && runningExecutions.length === 0" class="dialog-empty">{{ t('task.exec.noExecutions') }}</div>
      <!-- Running executions (virtual items) -->
      <div v-for="exec in runningExecutions" :key="exec.id" class="execution-item running" @click.self>
        <div class="execution-row">
          <div class="execution-info">
            <div class="execution-time-row">
              <span class="exec-running-dot"></span>
              <span class="exec-running-label">{{ t('task.exec.running') }}</span>
              <span class="exec-relative-time">{{ chatRender.formatMessageTime(exec.startedAt) }}</span>
              <span v-if="exec.triggerType === 'manual'" class="exec-trigger-type manual">{{ t('task.exec.manual') }}</span>
              <span v-else class="exec-trigger-type auto">{{ t('task.exec.auto') }}</span>
            </div>
          </div>
          <button class="cancel-exec-btn" @click.stop="cancelExecution(exec.id)" :title="t('task.exec.cancel')">
            <Square :size="12" />
          </button>
        </div>
      </div>
      <!-- Completed executions -->
      <div v-for="(exec, idx) in executions" :key="idx" class="execution-item" :class="{ unread: exec.isUnread }" @click="openDetail(exec)">
        <div class="execution-row">
          <div class="execution-info">
            <div class="execution-time-row">
              <span class="exec-absolute-time">{{ formatAbsoluteTime(exec.createdAt) }}</span>
              <span class="exec-relative-time">{{ chatRender.formatMessageTime(exec.createdAt) }}</span>
              <span v-if="exec.isUnread" class="exec-unread-dot"></span>
            </div>
            <div class="exec-summary-row">
              <div v-if="exec.summary" class="exec-summary">{{ exec.summary }}</div>
              <div v-else class="exec-summary empty">{{ t('task.exec.noTextOutput') }}</div>
              <span v-if="exec.triggerType === 'manual'" class="exec-trigger-type manual">{{ t('task.exec.manual') }}</span>
            </div>
            <div v-if="exec.metadata" class="exec-meta-row">
              <span v-if="exec.metadata.wallMs" class="exec-meta-tag exec-meta-duration">{{ formatDuration(exec.metadata.wallMs) }}</span>
              <span v-if="exec.metadata.model" class="exec-meta-tag">{{ exec.metadata.model }}</span>
              <span v-if="exec.metadata.inputTokens || exec.metadata.outputTokens" class="exec-meta-tag">{{ formatTokens(exec.metadata) }}</span>
              <span v-if="exec.metadata.costUsd" class="exec-meta-tag">${{ exec.metadata.costUsd.toFixed(4) }}</span>
            </div>
          </div>
          <ChevronRight :size="14" class="exec-chevron" />
        </div>
      </div>
    </div>

    <!-- Detail view: reuse chat-message.assistant for consistent markdown/inline styles -->
    <div v-if="view === 'detail' && selectedExec" class="chat-message assistant detail-content">
      <ContentBlocks
        :blocks="selectedExec.blocks"
        msgId="exec-detail"
        :msgIndex="0"
        :expandedTools="expandedTools"
        :blockTasks="{}"
        :renderTextBlock="chatRender.renderTextBlock"
        :formatToolInput="chatRender.formatToolInput"
        :toolCallSummary="chatRender.toolCallSummary"
        @toggle-tool="toggleTool"
        @show-tool-detail="handleShowToolDetail"
      />
      <!-- Execution metadata bar -->
      <div v-if="selectedExec.metadata" class="exec-detail-meta">
        <span v-if="selectedExec.metadata.wallMs" class="exec-meta-tag exec-meta-duration">{{ t('chat.metadata.wallDuration') }} {{ formatDuration(selectedExec.metadata.wallMs) }}</span>
        <span v-if="selectedExec.metadata.model" class="exec-meta-tag">{{ selectedExec.metadata.model }}</span>
        <span v-if="selectedExec.metadata.inputTokens" class="exec-meta-tag">{{ t('chat.metadata.inputTokens') }} {{ selectedExec.metadata.inputTokens.toLocaleString() }}</span>
        <span v-if="selectedExec.metadata.outputTokens" class="exec-meta-tag">{{ t('chat.metadata.outputTokens') }} {{ selectedExec.metadata.outputTokens.toLocaleString() }}</span>
        <span v-if="selectedExec.metadata.costUsd" class="exec-meta-tag">{{ t('chat.metadata.cost') }} ${{ selectedExec.metadata.costUsd.toFixed(6) }}</span>
        <span v-if="selectedExec.metadata.stopReason" class="exec-meta-tag">{{ selectedExec.metadata.stopReason }}</span>
      </div>
    </div>

    <template #footer>
      <button class="btn btn-primary" @click="triggerTask" :disabled="triggering || runningExecutions.length > 0">
        <template v-if="runningExecutions.length > 0">{{ t('task.exec.running') }}...</template>
        <template v-else-if="triggering">{{ t('task.exec.executing') }}</template>
        <template v-else>{{ t('task.exec.executeNow') }}</template>
      </button>
      <button class="btn btn-secondary" @click="handleClose">{{ t('common.close') }}</button>
    </template>
  </ModalDialog>

  <!-- Tool Detail Overlay (for viewing tool input/output in execution history) -->
  <ToolDetailOverlay
    :show="toolDetailOverlay.show"
    :toolName="toolDetailOverlay.name"
    :toolSummary="toolDetailOverlay.summary"
    :toolInputHtml="toolDetailOverlay.inputHtml"
    :toolOutputHtml="toolDetailOverlay.outputHtml"
    :toolStatus="toolDetailOverlay.status"
    :toolDone="toolDetailOverlay.done"
    @close="toolDetailOverlay.show = false"
  />
</template>

<script setup>
import { ref, watch, onUnmounted, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { Clock, ChevronLeft, ChevronRight, Square } from 'lucide-vue-next'
import ModalDialog from '@/components/common/ModalDialog.vue'
import ContentBlocks from '@/components/chat/ContentBlocks.vue'
import ToolDetailOverlay from '@/components/chat/ToolDetailOverlay.vue'
import { useChatRender } from '@/composables/useChatRender.ts'
import { formatToolOutput } from '@/utils/renderToolDetail.ts'
import { useToast } from '@/composables/useToast.ts'
import { useDialog } from '@/composables/useDialog.ts'
import { formatDuration } from '@/utils/format.ts'

const props = defineProps({
  open: Boolean,
  task: Object,
})

const emit = defineEmits(['close'])

const { t } = useI18n()
const dialog = useDialog()

function formatTokens(meta) {
  const parts = []
  if (meta.inputTokens) parts.push(`${meta.inputTokens.toLocaleString()}↑`)
  if (meta.outputTokens) parts.push(`${meta.outputTokens.toLocaleString()}↓`)
  return parts.join(' ')
}

const loading = ref(false)
const triggering = ref(false)
const executions = ref([])
const runningExecutions = ref([])
const expandedTools = ref({})
const view = ref('list')  // 'list' | 'detail'
const selectedExec = ref(null)
let pollTimer = null

// Create chatRender instance for rendering execution blocks
const renderTheme = inject('theme', ref('light'))
const chatRender = useChatRender({ messages: ref([]), theme: renderTheme, currentSessionId: ref('') })
const toast = useToast()

function toggleTool(key) {
  expandedTools.value = { ...expandedTools.value, [key]: !expandedTools.value[key] }
}

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
  const { formatToolInput } = chatRender
  toolDetailOverlay.value = {
    show: true,
    name: block.name || '',
    summary: chatRender.toolCallSummary(block),
    inputHtml: formatToolInput(block.input, block.name),
    outputHtml: block.output ? formatToolOutput(block.output, block.name) : '',
    status: block.status || '',
    done: !!block.done,
  }
}

function formatAbsoluteTime(createdAt) {
  const d = new Date(createdAt)
  const y = d.getFullYear()
  const mo = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const mi = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  return `${y}-${mo}-${day} ${h}:${mi}:${s}`
}

function extractSummary(blocks) {
  for (const block of blocks) {
    if (block.type === 'text' && block.text) {
      // Strip schedule-proposal tags and markdown
      const clean = block.text
        .replace(/<scheduled-task\s+id="[^"]+"\s*\/>/g, '')
        .replace(/[#*`_~\[\]()]/g, '')
        .trim()
      if (clean) {
        return clean.length > 80 ? clean.substring(0, 80) + '...' : clean
      }
    }
  }
  return ''
}

function openDetail(exec) {
  selectedExec.value = exec
  expandedTools.value = {}
  view.value = 'detail'
}

function handleClose() {
  view.value = 'list'
  selectedExec.value = null
  emit('close')
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(loadRunningStatus, 3000)
}

function stopPolling() {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function loadRunningStatus() {
  if (!props.task?.id) return
  try {
    const resp = await fetch(`/api/tasks/${props.task.id}`)
    if (resp.ok) {
      const data = await resp.json()
      runningExecutions.value = data.runningExecutions || []
    }
  } catch (err) {
    console.error('Failed to load running status:', err)
  }
}

async function cancelExecution(execId) {
  if (!props.task?.id) return
  if (!await dialog.confirm(t('task.exec.confirmCancel'))) return
  try {
    const resp = await fetch(`/api/tasks/${props.task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'cancel', executionId: execId }),
    })
    if (resp.ok) {
      toast.show(t('task.exec.cancelled'), { type: 'success' })
    } else if (resp.status === 404) {
      // Already finished, just refresh
      toast.show(t('task.exec.alreadyFinished'), { type: 'info' })
    }
    // Immediately refresh running status
    await loadRunningStatus()
  } catch (err) {
    console.error('Failed to cancel execution:', err)
  }
}

async function triggerTask() {
  if (!props.task?.id || triggering.value) return
  if (runningExecutions.value.length > 0) return
  triggering.value = true
  try {
    const resp = await fetch(`/api/tasks/${props.task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'trigger' }),
    })
    if (resp.ok) {
      toast.show(t('task.exec.triggered', { name: props.task.name }), { type: 'success' })
      // Immediately check for new running execution
      await loadRunningStatus()
    } else if (resp.status === 409) {
      toast.show(t('task.exec.alreadyRunning'), { type: 'warning' })
      await loadRunningStatus()
    }
  } catch (err) {
    console.error('Failed to trigger task:', err)
  } finally {
    triggering.value = false
  }
}

async function loadExecutions() {
  if (!props.task?.id) return
  loading.value = true
  try {
    const resp = await fetch(`/api/tasks/${props.task.id}/executions`)
    const data = await resp.json()
    const rawExecutions = data.executions || []
    executions.value = rawExecutions.map(exec => {
      const { blocks, metadata } = chatRender.parseAssistantContent(exec.content)
      const summary = extractSummary(blocks)
      return { ...exec, blocks, metadata, summary }
    })
  } catch (err) {
    console.error('Failed to load executions:', err)
  } finally {
    loading.value = false
  }
}

async function markTaskRead() {
  if (!props.task?.id) return
  try {
    await fetch(`/api/tasks/${props.task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'read' }),
    })
  } catch (err) {
    console.error('Failed to mark task as read:', err)
  }
}

watch(() => props.open, (isOpen) => {
  if (!isOpen) return
  view.value = 'list'
  selectedExec.value = null
  expandedTools.value = {}
  loadExecutions()
  loadRunningStatus()
  markTaskRead()
  startPolling()
})

watch(() => props.open, (isOpen) => {
  if (!isOpen) {
    stopPolling()
    runningExecutions.value = []
  }
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.executions-content {
  flex: 1;
  overflow-y: auto;
  padding: 2px 0;
}

.detail-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto !important;
  /* Override .chat-message.assistant styles that conflict with modal context */
  background: transparent !important;
  border-radius: 0 !important;
  align-self: stretch !important;
}

.execution-item {
  border-bottom: 1px solid var(--border-color, #e5e5e5);
}

.execution-item:last-child {
  border-bottom: none;
}

.execution-item.running {
  background: color-mix(in srgb, var(--success-color, #22c55e) 6%, transparent);
}

.execution-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  cursor: pointer;
  transition: background 0.15s;
}

.execution-row:hover {
  background: var(--bg-secondary);
}

.execution-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.exec-meta-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.exec-meta-tag {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-secondary, #666);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.exec-meta-duration {
  font-weight: 500;
  color: var(--text-primary);
}

.execution-time-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.exec-absolute-time {
  font-size: 12px;
  color: var(--text-primary);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.exec-relative-time {
  font-size: 11px;
  color: var(--text-muted, #999);
  white-space: nowrap;
}

.exec-unread-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-color, #0066cc);
  flex-shrink: 0;
  animation: exec-unread-pulse 1.2s ease-in-out infinite;
}

.exec-trigger-type {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
  white-space: nowrap;
}

.exec-trigger-type.manual {
  background: rgba(59, 130, 246, 0.12);
  color: #3b82f6;
}

.exec-trigger-type.auto {
  background: rgba(34, 197, 94, 0.12);
  color: #22c55e;
}

@keyframes exec-unread-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.7); }
}

.exec-summary-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.exec-summary {
  font-size: 12px;
  color: var(--text-secondary, #666);
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.exec-summary.empty {
  color: var(--text-muted, #999);
  font-style: italic;
}

.execution-item.unread .exec-absolute-time {
  color: var(--accent-color, #0066cc);
}

.execution-item.unread {
  animation: exec-unread-flash 0.8s ease-in-out infinite;
}

@keyframes exec-unread-flash {
  0%, 100% {
    background: transparent;
  }
  50% {
    background: color-mix(in srgb, var(--accent-color, #0066cc) 6%, transparent);
  }
}

.exec-chevron {
  flex-shrink: 0;
  color: var(--text-muted, #ccc);
}

.exec-detail-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 8px 12px;
  border-top: 1px solid var(--border-color, #e5e5e5);
  margin-top: 4px;
}

.exec-detail-meta .exec-meta-tag {
  font-size: 11px;
}

/* Running execution indicator */
.exec-running-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--success-color, #22c55e);
  flex-shrink: 0;
  animation: exec-running-pulse 1.5s ease-in-out infinite;
}

@keyframes exec-running-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.4); }
  50% { opacity: 0.7; box-shadow: 0 0 6px 2px rgba(34, 197, 94, 0.2); }
}

.exec-running-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--success-color, #22c55e);
}

.cancel-exec-btn {
  width: 24px;
  height: 24px;
  border: none;
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.15s;
}

.cancel-exec-btn:hover {
  background: rgba(239, 68, 68, 0.2);
}

.back-btn {
  width: 22px;
  height: 22px;
  border: none;
  background: none;
  color: var(--accent-color, #0066cc);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: background 0.15s;
}

.back-btn:hover {
  background: rgba(0, 102, 204, 0.1);
}

.dialog-empty {
  text-align: center;
  padding: 20px 12px;
  color: var(--text-muted, #999);
  font-size: 13px;
}

/* Buttons */
.btn {
  padding: 5px 14px;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
}

.btn-primary {
  background: var(--accent-color, #0066cc);
  color: #fff;
}

.btn-primary:hover { background: #0055aa; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.btn-secondary {
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-primary, #1a1a1a);
}

.btn-secondary:hover { background: #e0e0e0; }
</style>

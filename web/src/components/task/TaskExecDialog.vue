<template>
  <ModalDialog :open="open" :title="view === 'detail' ? '' : t('task.exec.title')" @close="handleClose">
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
      <div v-else-if="executions.length === 0" class="dialog-empty">{{ t('task.exec.noExecutions') }}</div>
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
        :blockProposals="{}"
        :renderTextBlock="chatRender.renderTextBlock"
        :formatToolInput="chatRender.formatToolInput"
        :toolCallSummary="chatRender.toolCallSummary"
        @toggle-tool="toggleTool"
      />
    </div>

    <template #footer>
      <button class="btn btn-primary" @click="triggerTask" :disabled="triggering">
        {{ triggering ? t('task.exec.executing') : t('task.exec.executeNow') }}
      </button>
      <button class="btn btn-secondary" @click="handleClose">{{ t('common.close') }}</button>
    </template>
  </ModalDialog>
</template>

<script setup>
import { ref, watch, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { Clock, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import ModalDialog from '@/components/common/ModalDialog.vue'
import ContentBlocks from '@/components/chat/ContentBlocks.vue'
import { useChatRender } from '@/composables/useChatRender.ts'
import { useToast } from '@/composables/useToast.ts'

const props = defineProps({
  open: Boolean,
  task: Object,
})

const emit = defineEmits(['close'])

const { t } = useI18n()

const loading = ref(false)
const triggering = ref(false)
const executions = ref([])
const expandedTools = ref({})
const view = ref('list')  // 'list' | 'detail'
const selectedExec = ref(null)

// Create chatRender instance for rendering execution blocks
const renderTheme = inject('theme', ref('light'))
const chatRender = useChatRender({ messages: ref([]), theme: renderTheme, currentSessionId: ref('') })
const toast = useToast()

function toggleTool(key) {
  expandedTools.value = { ...expandedTools.value, [key]: !expandedTools.value[key] }
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
        .replace(/<schedule-proposal>[\s\S]*?<\/schedule-proposal>/g, '')
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

async function triggerTask() {
  if (!props.task?.id || triggering.value) return
  triggering.value = true
  try {
    const resp = await fetch(`/api/tasks/${props.task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'trigger' }),
    })
    if (resp.ok) {
      toast.show(t('task.exec.triggered', { name: props.task.name }), { type: 'success' })
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
      const { blocks } = chatRender.parseAssistantContent(exec.content)
      const summary = extractSummary(blocks)
      return { ...exec, blocks, summary }
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
  markTaskRead()
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

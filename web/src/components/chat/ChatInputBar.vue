<template>
  <div class="chat-input-wrapper">
    <!-- Top action bar (above input box) -->
    <div class="chat-top-actions">
      <div class="chat-action-group">
        <span class="chat-group-label" :title="t('chat.actions.session')">
          <MessageSquare :size="12" />
        </span>
        <button class="chat-action-btn" :class="{ 'has-unread': chatUnread, 'has-running': chatRunning && !chatUnread }"
          @click="$emit('open-session-tab', 'sessions')"
          :title="t('chat.actions.session')">
          <List :size="14" />
        </button>
        <button class="chat-action-btn"
          @click="handleCreateClick"
          @contextmenu.prevent="emit('create-session')"
          :title="t('chat.create.selectAgentOrLongPress')">
          <Plus :size="14" />
        </button>
        <button class="chat-action-btn chat-action-btn-delete" :class="{ disabled: !currentSessionId }"
          @click="handleDelete"
          :title="currentSessionId ? t('chat.actions.deleteCurrentSession') : t('chat.actions.noSessionToDelete')">
          <Trash2 :size="14" />
        </button>
      </div>
      <button class="chat-action-btn" :class="{ 'has-unread': taskUnread }"
        @click="$emit('open-session-tab', 'tasks')"
        :title="t('chat.actions.scheduledTasks')">
        <Calendar :size="14" />
      </button>
      <button class="chat-action-btn" :class="{ active: autoSpeechEnabled }"
        @click="$emit('toggle-auto-speech')"
        :title="t('chat.actions.autoSpeech')">
        <Volume2 :size="14" />
      </button>
      <!-- Model chip (show for all agents, click only for multi-model) -->
      <button class="chat-action-btn model-chip" ref="modelChipRef"
        :class="{ clickable: isMultiModel(currentAgentId) }"
        @click.stop="toggleModelMenu"
        :title="isMultiModel(currentAgentId) ? t('chat.actions.switchModel') : currentModelName">
        <Cpu :size="14" />
        <span class="chat-action-label">{{ currentModelName }}</span>
        <ChevronDown v-if="isMultiModel(currentAgentId)" :size="10" />
      </button>
    </div>
    <!-- Input container -->
    <div class="chat-input-container" :class="{ 'drag-over': isDragOver }"
      @dragenter="onDragEnter"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop">
      <input type="file" ref="fileInputRef" @change="onFileSelect" style="display:none" multiple />
      <!-- Drop overlay -->
      <div v-if="isDragOver" class="drop-overlay">
        <Upload :size="24" :stroke-width="1.5" />
        <span>{{ t('chat.attach.dropToUpload') }}</span>
      </div>
      <!-- Upload progress bars -->
      <div v-if="uploadingFiles.length > 0" class="chat-upload-progress">
        <div v-for="(f, idx) in uploadingFiles" :key="'prog-' + idx" class="upload-progress-item">
          <div class="upload-progress-bar" :style="{ width: f.progress + '%' }"></div>
        </div>
      </div>
      <!-- Attachment tags -->
      <div v-if="attachedFiles.length > 0 || pendingFiles.length > 0" class="chat-attachment-tags">
        <span v-for="(filePath, idx) in attachedFiles" :key="'att-' + filePath" class="chat-file-attachment attachment-ref" @click="$emit('file-tag-click', filePath)" :title="t('chat.attach.openFile')">
          <Folder v-if="isDirPath(filePath)" :size="12" :stroke-width="1.5" />
          <Paperclip v-else :size="12" :stroke-width="1.5" />
          <span class="chat-file-name">{{ getFileName(filePath) }}</span>
          <button class="attachment-tag-remove" @click.stop="$emit('remove-attached', idx)" :title="t('common.remove')">×</button>
        </span>
        <span v-for="(f, idx) in pendingFiles" :key="'upload-' + idx" class="chat-file-attachment attachment-upload" :class="{ 'is-uploading': f.uploading }">
          <FileImage v-if="f.isImage" :size="12" :stroke-width="1.5" />
          <FileText v-else :size="12" :stroke-width="1.5" />
          <span class="chat-file-name">{{ getFileName(f.path) || t('chat.attach.uploading') }}</span>
          <span v-if="f.uploading" class="attachment-progress-pct">{{ f.progress }}%</span>
          <button class="attachment-tag-remove" @click.stop="$emit('remove-file', idx)" :title="t('common.remove')">×</button>
        </span>
      </div>
      <!-- Input row: attach + clear + textarea + stop + send -->
      <div class="chat-input-row">
        <div class="attach-menu-wrapper" ref="attachMenuRef">
          <button class="chat-attach-btn" @click.stop="toggleAttachMenu" :disabled="inputDisabled" :title="t('chat.actions.attachment')">
            <Paperclip :size="16" />
          </button>
        </div>
        <button v-if="inputText && !loading" class="chat-clear-btn" @click="inputText = ''" :title="t('chat.input.clearInput')">
          <XCircle :size="16" />
        </button>
        <textarea class="chat-textarea"
          ref="textareaRef"
          v-model="inputText"
          :disabled="inputDisabled"
          :placeholder="pendingFiles.length > 0 ? t('chat.input.placeholderOptional') : loading ? t('chat.input.placeholderQueue') : t('chat.input.placeholder')"
          rows="1"
          @keydown.enter.exact.prevent="$emit('send', inputText.trim())"
          @blur="autoResizeTextarea"></textarea>
        <button v-if="!stopPrimed" class="chat-send-btn" ref="sendBtnRef" :class="{ queued: loading }" @click.stop="handleSendClick" :title="loading ? t('chat.input.enqueue') : t('chat.input.send')">
          <!-- Queue mode: inbox with down arrow (enqueue) -->
          <Inbox v-if="loading" :size="16" />
          <!-- Normal mode: paper plane (send) -->
          <Send v-else :size="16" />
        </button>
        <button v-if="loading" class="chat-stop-btn" :class="{ primed: stopPrimed }" @click="handleStopClick" :title="stopPrimed ? t('chat.input.confirmStop') : t('chat.input.stopGenerating')">
          <Square :size="16" fill="currentColor" />
        </button>
      </div>
      <!-- Teleported attach menu (avoids overflow:hidden clipping) -->
      <PopupMenu v-model:show="showAttachMenu" :target-element="attachMenuRef?.querySelector('.chat-attach-btn')" :max-width="200" :max-height="280" :menu-items-count="attachMenuItemCount">
        <!-- Current file group -->
        <template v-if="currentFile?.path && !attachedFiles.includes(currentFile.path)">
          <div class="attach-menu-group-title">{{ t('chat.attach.currentFile') }}</div>
          <button class="attach-menu-item" @click="handleAttachFile(currentFile.path)">
            <FileText :size="14" :stroke-width="1.5" />
            <span class="attach-menu-item-name">{{ getFileName(currentFile.path) }}</span>
          </button>
        </template>
        <!-- Current directory group -->
        <template v-if="currentDir && !attachedFiles.includes(currentDir)">
          <div class="attach-menu-group-title">{{ t('chat.attach.currentDir') }}</div>
          <button class="attach-menu-item" @click="handleAttachFile(currentDir)">
            <Folder :size="14" :stroke-width="1.5" />
            <span class="attach-menu-item-name">{{ getFileName(currentDir) }}</span>
          </button>
        </template>
        <!-- Recently referenced group -->
        <template v-if="recentReferencedFiles.length > 0">
          <div class="attach-menu-group-title">{{ t('chat.attach.recentReferences') }}</div>
          <button v-for="item in recentReferencedFiles" :key="item.path" class="attach-menu-item" @click="handleAttachFile(item.path)">
            <FileText :size="14" :stroke-width="1.5" />
            <span class="attach-menu-item-name">{{ getFileName(item.path) }}</span>
            <span class="attach-menu-item-count">×{{ item.count }}</span>
          </button>
        </template>
        <!-- Separator + Upload -->
        <div v-if="hasFileGroups" class="attach-menu-separator"></div>
        <button class="attach-menu-item" @click="handleUploadClick">
          <Upload :size="14" :stroke-width="1.5" />
          <span>{{ t('chat.attach.uploadFile') }}</span>
        </button>
      </PopupMenu>
      <!-- Teleported quick-send menu -->
      <PopupMenu v-model:show="showQuickMenu" :target-element="sendBtnRef" anchor="right" :max-width="260" :max-height="280" :menu-items-count="quickSendItems.length + 1">
        <div class="quick-send-title">{{ t('chat.quickSend.title') }}</div>
        <button v-for="item in quickSendItems" :key="item.id" class="quick-send-item" @click="handleQuickSend(item.command)">
          {{ item.label }}
        </button>
        <div class="quick-send-divider" />
        <button class="quick-send-item" @click="showQuickMenu = false; quickSendStore.showEditDialog.value = true">
          ⚙️ {{ t('chat.quickSend.edit') }}
        </button>
      </PopupMenu>
      <!-- Teleported model switcher menu -->
      <PopupMenu v-model:show="showModelMenu" :target-element="modelChipRef" :max-width="220" :max-height="320" :menu-items-count="agentModels.length">
        <div class="model-menu-title">{{ t('chat.modelSwitcher.title') }}</div>
        <button v-for="m in agentModels" :key="m.id" class="model-menu-item" :class="{ active: m.id === currentModelId }" @click="selectModel(m)">
          <Check v-if="m.id === currentModelId" :size="14" />
          <span v-else class="model-menu-check-spacer"></span>
          <span>{{ m.name }}</span>
        </button>
      </PopupMenu>
      <QuickSendDialog :open="props.active && quickSendStore.showEditDialog.value" @close="quickSendStore.showEditDialog.value = false" />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, watch, onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessageSquare, List, Plus, Trash2, Calendar, Volume2, Upload, Paperclip, FileImage, FileText, Folder, XCircle, Inbox, Send, Square, Cpu, ChevronDown, Check } from 'lucide-vue-next'
import { baseName } from '@/utils/path.ts'
import PopupMenu from '@/components/common/PopupMenu.vue'
import QuickSendDialog from '@/components/chat/QuickSendDialog.vue'
import { useDialog } from '@/composables/useDialog.ts'
import { useQuickSend } from '@/composables/useQuickSend'

const { t } = useI18n()
const dialog = useDialog()
const quickSendStore = useQuickSend()
const { items: quickSendItems, fetchItems } = quickSendStore

const props = defineProps({
  inputDisabled: Boolean,
  loading: Boolean,
  currentFile: Object,
  currentDir: String,
  pendingFiles: Array,
  attachedFiles: Array,
  messages: Array,
  autoSpeechEnabled: Boolean,
  currentSessionId: String,
  chatUnread: Boolean,
  chatRunning: Boolean,
  taskUnread: Boolean,
  currentModelId: String,
  currentModelName: String,
  agentModels: { type: Array, default: () => [] },
  isMultiModel: { type: Function, default: () => false },
  currentAgentId: String,
  active: Boolean,
})

const emit = defineEmits([
  'send',
  'cancel',
  'file-select',
  'file-drop',
  'remove-file',
  'add-attached',
  'remove-attached',
  'open-session-tab',
  'file-tag-click',
  'toggle-auto-speech',
  'create-session',
  'show-agent-selector',
  'delete-session',
  'switch-model',
])

const inputText = ref('')
const textareaRef = ref(null)
const fileInputRef = ref(null)
const isDragOver = ref(false)
const dragCounter = ref(0)
const showAttachMenu = ref(false)
const attachMenuRef = ref(null)
const showQuickMenu = ref(false)
const sendBtnRef = ref(null)
const showModelMenu = ref(false)
const modelChipRef = ref(null)

// Stop button two-click confirmation state
const stopPrimed = ref(false)
let stopPrimeTimer = null

function handleStopClick() {
  if (!stopPrimed.value) {
    // First click: enter confirmation state (solid ■ → hollow ✕)
    stopPrimed.value = true
    clearTimeout(stopPrimeTimer)
    stopPrimeTimer = setTimeout(() => { stopPrimed.value = false }, 1500)
  } else {
    // Second click: confirmed — execute stop
    stopPrimed.value = false
    clearTimeout(stopPrimeTimer)
    emit('cancel')
  }
}

// Per-session draft cache: save input text when switching away, restore when switching back
const draftCache = new Map()

watch(() => props.currentSessionId, (newId, oldId) => {
  // Save draft from the old session
  if (oldId) {
    const text = inputText.value
    if (text) {
      draftCache.set(oldId, text)
    } else {
      draftCache.delete(oldId)
    }
  }
  // Restore draft for the new session (or clear if none)
  inputText.value = newId ? (draftCache.get(newId) || '') : ''
  // autoResizeTextarea is called automatically by the inputText watcher
})

const uploadingFiles = computed(() => props.pendingFiles.filter(f => f.uploading))

const hasInputContent = computed(() => inputText.value.trim() || props.pendingFiles.length > 0 || props.attachedFiles.length > 0)

// Extract recently referenced files from message history
const recentReferencedFiles = computed(() => {
  if (!props.messages || props.messages.length === 0) return []
  const countMap = new Map()
  for (const msg of props.messages) {
    if (msg.role !== 'user' || !msg.files) continue
    for (const f of msg.files) {
      // Backend returns string[], local push uses [{path: "..."}]
      const p = typeof f === 'string' ? f : f?.path
      if (!p) continue
      countMap.set(p, (countMap.get(p) || 0) + 1)
    }
  }
  // Exclude current file and already attached files
  const exclude = new Set([...props.attachedFiles])
  if (props.currentFile?.path) exclude.add(props.currentFile.path)
  return [...countMap.entries()]
    .filter(([path]) => !exclude.has(path))
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([path, count]) => ({ path, count }))
})

const hasFileGroups = computed(() => {
  const hasCurrent = props.currentFile?.path && !props.attachedFiles.includes(props.currentFile.path)
  const hasDir = props.currentDir && !props.attachedFiles.includes(props.currentDir)
  return hasCurrent || hasDir || recentReferencedFiles.value.length > 0
})

const attachMenuItemCount = computed(() => {
  let count = recentReferencedFiles.value.length
  if (props.currentFile?.path && !props.attachedFiles.includes(props.currentFile.path)) count++
  if (props.currentDir && !props.attachedFiles.includes(props.currentDir)) count++
  count++ // Upload file button
  return count
})

function handleCreateClick(e) {
  // On desktop, click = show agent selector (short tap equivalent)
  if (e.detail === 0) return
  emit('show-agent-selector')
}

async function handleDelete() {
  if (!props.currentSessionId) return
  if (await dialog.confirm(t('chat.delete.confirm'), { dangerous: true })) {
    emit('delete-session')
  }
}

function getFileName(path) {
  return baseName(path)
}

function isDirPath(filePath) {
  return props.currentDir && filePath === props.currentDir
}

function autoResizeTextarea() {
  const el = textareaRef.value
  if (!el) return
  el.style.height = 'auto'
  const computed = getComputedStyle(el)
  const lineHeight = parseFloat(computed.lineHeight) || 20
  const paddingTop = parseFloat(computed.paddingTop) || 0
  const paddingBottom = parseFloat(computed.paddingBottom) || 0
  const maxContentHeight = lineHeight * 3
  const maxHeight = maxContentHeight + paddingTop + paddingBottom
  el.style.height = Math.min(el.scrollHeight, maxHeight) + 'px'
}

// Watch inputText changes (both user input and programmatic changes like draft restore)
// to ensure textarea height stays in sync with content
watch(inputText, () => nextTick(() => autoResizeTextarea()))

function onFileSelect(e) {
  emit('file-select', e)
}

function onDragEnter(e) {
  e.preventDefault()
  dragCounter.value++
  isDragOver.value = true
}

function onDragOver(e) {
  e.preventDefault()
}

function onDragLeave(e) {
  e.preventDefault()
  dragCounter.value--
  if (dragCounter.value <= 0) {
    dragCounter.value = 0
    isDragOver.value = false
  }
}

function onDrop(e) {
  e.preventDefault()
  dragCounter.value = 0
  isDragOver.value = false
  const files = Array.from(e.dataTransfer?.files || [])
  if (files.length > 0) {
    emit('file-drop', files)
  }
}

function clearInput() {
  inputText.value = ''
  // Also clear the draft cache for current session so it doesn't linger
  if (props.currentSessionId) {
    draftCache.delete(props.currentSessionId)
  }
}

function handleAttachFile(filePath) {
  emit('add-attached', filePath)
}

function handleUploadClick() {
  showAttachMenu.value = false
  if (fileInputRef.value) {
    // Clear previous selection BEFORE opening picker to prevent stale
    // file data on Android WebView when user cancels the picker
    fileInputRef.value.value = ''
    fileInputRef.value.click()
  }
}

function toggleAttachMenu() {
  showAttachMenu.value = !showAttachMenu.value
}

function handleSendClick() {
  if (inputText.value.trim()) {
    emit('send', inputText.value.trim())
  } else if (props.pendingFiles.length > 0 || props.attachedFiles.length > 0) {
    emit('send', '')
  } else {
    toggleQuickMenu()
  }
}

function handleQuickSend(text) {
  emit('send', text)
}

function toggleQuickMenu() {
  showQuickMenu.value = !showQuickMenu.value
}

function toggleModelMenu() {
  showModelMenu.value = !showModelMenu.value
}

function selectModel(model) {
  showModelMenu.value = false
  emit('switch-model', model)
}

// Menu mutual exclusion: opening one closes the others
watch(showAttachMenu, (v) => { if (v) { showQuickMenu.value = false; showModelMenu.value = false } })
watch(showQuickMenu, (v) => { if (v) { showAttachMenu.value = false; showModelMenu.value = false } })
watch(showModelMenu, (v) => { if (v) { showAttachMenu.value = false; showQuickMenu.value = false } })

onMounted(() => {
  fetchItems()
})

onBeforeUnmount(() => {
  clearTimeout(stopPrimeTimer)
})

// Reset stop confirmation state when loading ends (AI finished or cancelled)
watch(() => props.loading, (val) => {
  if (!val) {
    stopPrimed.value = false
    clearTimeout(stopPrimeTimer)
  }
})

defineExpose({
  clearInput,
  inputText,
  deleteDraft: (sessionId) => { draftCache.delete(sessionId) },
})
</script>

<style scoped>
/* Outer wrapper: top actions + input box stacked vertically */
.chat-input-wrapper {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  margin: 0 8px 8px;
  padding-top: 8px;
  border-top: 1px solid var(--border-color, #e5e5e5);
}

/* Top action bar (above input box, compact) */
.chat-top-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 4px 6px;
}

/* Session button group */
.chat-action-group {
  display: inline-flex;
  align-items: stretch;
  border-radius: 20px;
  overflow: hidden;
  border: 1px solid var(--border-color, #e5e5e5);
}

.chat-action-group .chat-action-btn {
    border-radius: 0;
}

.chat-action-group .chat-action-btn:first-child {
    border-radius: 0;
}

/* Group label: subtle icon identifying the button group */
.chat-group-label {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 5px 6px 5px 8px;
    color: var(--text-muted, #999);
    background: var(--bg-tertiary, #f0f0f0);
    pointer-events: none;
    user-select: none;
}

.chat-action-group .chat-action-btn:last-child {
    border-radius: 0 999px 999px 0;
}

.chat-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted, #999);
  padding: 5px 8px;
  border-radius: 4px;
  font-size: 11px;
  line-height: 1;
  transition: color 0.15s, background 0.15s, transform 0.1s;
  -webkit-tap-highlight-color: transparent;
  user-select: none;
}

@media (hover: hover) {
  .chat-action-btn:hover {
    color: var(--accent-color, #0066cc);
    background: var(--bg-tertiary, #f0f0f0);
  }
}

.chat-action-btn:active {
  color: var(--accent-color, #0066cc);
  background: color-mix(in srgb, var(--accent-color, #0066cc) 15%, transparent);
  transform: scale(0.92);
}

.chat-action-btn.active {
  color: var(--accent-color, #0066cc);
  background: color-mix(in srgb, var(--accent-color, #0066cc) 10%, transparent);
}

.chat-action-btn.active:active {
  background: color-mix(in srgb, var(--accent-color, #0066cc) 25%, transparent);
  transform: scale(0.92);
}

.chat-action-btn-delete:not(.disabled) {
  color: var(--text-muted, #999);
}

@media (hover: hover) {
  .chat-action-btn-delete:not(.disabled):hover {
    color: var(--danger-color, #dc3545);
    background: color-mix(in srgb, var(--danger-color, #dc3545) 10%, transparent);
  }
}

.chat-action-btn-delete:not(.disabled):active {
  color: var(--danger-color, #dc3545);
  background: color-mix(in srgb, var(--danger-color, #dc3545) 18%, transparent);
  transform: scale(0.92);
}

.chat-action-btn-delete.disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* Unread session indicator — fast flash on the button (GPU-safe: opacity + box-shadow) */
.chat-action-btn.has-unread {
    position: relative;
    color: var(--accent-color, #0066cc);
    background: color-mix(in srgb, var(--accent-color, #0066cc) 16%, transparent);
    animation: unread-flash 0.8s ease-in-out infinite;
}

.chat-action-btn.has-unread:active {
    animation: none;
    background: color-mix(in srgb, var(--accent-color, #0066cc) 30%, transparent);
    transform: scale(0.92);
}

@keyframes unread-flash {
    0%, 100% {
        opacity: 0.6;
        box-shadow: 0 0 0 0 color-mix(in srgb, var(--accent-color, #0066cc) 0%, transparent);
    }
    50% {
        opacity: 1;
        box-shadow: 0 0 10px 2px color-mix(in srgb, var(--accent-color, #0066cc) 25%, transparent);
    }
}

/* Running session indicator — horizontal sweep light */
.chat-action-btn.has-running {
    position: relative;
    overflow: hidden;
    color: var(--accent-color, #0066cc);
    background: color-mix(in srgb, var(--accent-color, #0066cc) 8%, transparent);
}

.chat-action-btn.has-running:active {
    background: color-mix(in srgb, var(--accent-color, #0066cc) 25%, transparent);
    transform: scale(0.92);
}

.chat-action-btn.has-running::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    width: 60%;
    height: 100%;
    transform: translateX(-160%);
    background: linear-gradient(90deg, transparent, rgba(255,255,255,0.35), transparent);
    animation: sweep-light 2s ease-in-out infinite;
}

@keyframes sweep-light {
    0% { transform: translateX(-60%); }
    100% { transform: translateX(160%); }
}

.chat-action-btn svg {
  flex-shrink: 0;
}

.chat-action-label {
  font-size: 11px;
  line-height: 1;
}

/* Unified input container */
.chat-input-container {
  display: flex;
  flex-direction: column;
  background: var(--bg-tertiary, #f0f0f0);
  flex: 1;
  min-width: 0;
  border: none;
  border-radius: 20px;
  overflow: hidden;
  position: relative;
  transition: background 0.2s, box-shadow 0.2s;
}

.chat-input-container:focus-within {
  background: var(--bg-primary, #fff);
  box-shadow: 0 0 0 1px var(--accent-color, #0066cc);
}

.chat-input-container.drag-over {
  background: var(--bg-primary, #fff);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent-color, #0066cc) 40%, transparent);
}

/* Drop overlay */
.drop-overlay {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: color-mix(in srgb, var(--accent-color, #0066cc) 8%, var(--bg-primary, #fff));
  color: var(--accent-color, #0066cc);
  font-size: 13px;
  font-weight: 500;
  border-radius: 20px;
  pointer-events: none;
}

/* Upload progress bars at top of input */
.chat-upload-progress {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 8px 0;
}

.upload-progress-item {
  height: 3px;
  background: color-mix(in srgb, var(--accent-color, #0066cc) 15%, transparent);
  border-radius: 2px;
  overflow: hidden;
}

.upload-progress-bar {
  height: 100%;
  background: var(--accent-color, #0066cc);
  border-radius: 2px;
  transition: width 0.15s ease;
}

/* Uploading state for attachment tag */
.attachment-upload.is-uploading {
  opacity: 0.7;
}

.attachment-progress-pct {
  font-size: 10px;
  color: var(--accent-color, #0066cc);
  font-variant-numeric: tabular-nums;
}

/* Attach button (inside input row) */
.attach-menu-wrapper {
  position: relative;
  flex-shrink: 0;
}

.chat-attach-btn {
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
}

.chat-attach-btn:hover:not(:disabled) {
  color: var(--accent-color, #0066cc);
  background: var(--bg-tertiary, #f0f0f0);
}

.chat-attach-btn:disabled {
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

/* Base attachment tag styles */
.chat-file-attachment {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: 8px;
  padding: 1px 6px;
  margin-bottom: 4px;
  font-size: 12px;
  text-decoration: none;
  cursor: pointer;
  transition: opacity 0.15s;
  white-space: nowrap;
  max-width: 200px;
}

.chat-file-attachment svg {
  flex-shrink: 0;
}

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

.chat-file-name::-webkit-scrollbar {
  display: none;
}

/* Input area attachment tags */
.chat-attachment-tags .chat-file-attachment {
  max-width: 200px;
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
  gap: 2px;
  padding: 4px 6px 6px;
}

/* Clear input button (next to attach button) */
.chat-clear-btn {
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
  align-self: flex-end;
}

.chat-clear-btn:hover {
  color: var(--danger-color, #dc3545);
  background: color-mix(in srgb, var(--danger-color, #dc3545) 8%, transparent);
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
  max-height: calc(20px * 3 + 4px + 4px); /* 3 lines + padding-top + padding-bottom */
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
  transition: background 0.15s, opacity 0.15s, transform 0.15s;
  flex-shrink: 0;
}
.chat-send-btn:hover { background: #0055aa; }
.chat-send-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.chat-send-btn.disabled { opacity: 0.5; cursor: not-allowed; }

/* Send button in queue mode: orange to distinguish from normal send */
.chat-send-btn.queued {
  background: #e67e22;
}
.chat-send-btn.queued:hover { background: #d35400; }

/* Stop button — default: dim red solid */
.chat-stop-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: color-mix(in srgb, var(--danger-color, #dc3545) 40%, transparent);
  color: color-mix(in srgb, #fff 60%, var(--danger-color, #dc3545));
  border: none;
  border-radius: 50%;
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.34, 1.56, 0.64, 1);
  flex-shrink: 0;
}
.chat-stop-btn:active { opacity: 0.75; }

/* Stop button — primed (first click, awaiting confirmation): bright red + heartbeat */
.chat-stop-btn.primed {
  background: var(--danger-color, #dc3545);
  color: #fff;
  transform: scale(1.15);
  animation: stop-heartbeat 0.8s ease-in-out infinite;
}

/* Pressed in primed state: scale feedback */
.chat-stop-btn.primed:active {
  transform: scale(1.0);
  animation: none;
}

@keyframes stop-heartbeat {
  0%, 100% { box-shadow: 0 0 0 0 rgba(220, 53, 69, 0.5); }
  50%      { box-shadow: 0 0 0 8px rgba(220, 53, 69, 0); }
}

/* Model switcher chip */
.model-chip {
  font-variant-numeric: tabular-nums;
}

.model-chip:not(.clickable) {
  cursor: default;
}

.model-chip:not(.clickable):hover {
  background: none;
  color: var(--text-muted, #999);
}

.model-chip:not(.clickable):active {
  transform: none;
}

.model-chip .chat-action-label {
  max-width: 80px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

</style>

<!-- Unscoped styles for teleported menu content (PopupMenu uses Teleport to body, scoped styles won't reach it) -->
<style>
/* Attach menu content styles */
.attach-menu-group-title {
  padding: 4px 10px 1px;
  font-size: 10px;
  color: var(--text-muted, #999);
  font-weight: 500;
  letter-spacing: 0.3px;
}

.attach-menu-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  width: 100%;
  border: none;
  background: none;
  color: var(--text-primary);
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  text-align: left;
}

.attach-menu-item:hover {
  background: var(--accent-color, #0066cc);
  color: #fff;
}

.attach-menu-item svg {
  flex-shrink: 0;
  width: 12px;
  height: 12px;
}

.attach-menu-item-name {
  font-family: monospace;
  font-size: 11px;
  min-width: 0;
  overflow-x: auto;
  overflow-y: hidden;
  white-space: nowrap;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.attach-menu-item-name::-webkit-scrollbar {
  display: none;
}

.attach-menu-item-count {
  margin-left: auto;
  font-size: 10px;
  color: var(--text-muted, #999);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.attach-menu-item:hover .attach-menu-item-count {
  color: rgba(255, 255, 255, 0.7);
}

.attach-menu-separator {
  height: 1px;
  background: var(--border-color, #e5e5e5);
  margin: 3px 6px;
}

/* Quick-send menu content styles */
.quick-send-title {
  padding: 6px 14px 2px;
  font-size: 11px;
  color: var(--text-muted, #999);
  font-weight: 500;
  letter-spacing: 0.3px;
}

.quick-send-item {
  display: block;
  width: 100%;
  padding: 8px 14px;
  border: none;
  background: none;
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
  text-align: left;
  transition: background 0.12s, color 0.12s;
}

.quick-send-item:hover {
  background: var(--accent-color, #0066cc);
  color: #fff;
}

.quick-send-divider {
  height: 1px;
  background: var(--border-color, #e5e5e5);
  margin: 3px 6px;
}

/* Model switcher menu content styles */
.model-menu-title {
  padding: 4px 10px 1px;
  font-size: 10px;
  color: var(--text-muted, #999);
  font-weight: 500;
  letter-spacing: 0.3px;
}

.model-menu-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  width: 100%;
  border: none;
  background: none;
  color: var(--text-primary);
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  text-align: left;
}

.model-menu-item:hover {
  background: var(--accent-color, #0066cc);
  color: #fff;
}

.model-menu-item.active {
  color: var(--accent-color, #0066cc);
  font-weight: 500;
}

.model-menu-item.active:hover {
  color: #fff;
}

.model-menu-item svg {
  flex-shrink: 0;
  width: 14px;
  height: 14px;
}

.model-menu-check-spacer {
  display: inline-block;
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}
</style>

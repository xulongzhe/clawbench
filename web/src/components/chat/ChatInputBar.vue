<template>
  <div class="chat-input-wrapper">
    <!-- Top action bar (above input box) -->
    <div class="chat-top-actions">
      <div class="chat-action-group">
        <button class="chat-action-btn" @click="$emit('open-session-tab', 'sessions')" title="切换会话">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <rect x="3" y="6" width="18" height="12" rx="2"/><line x1="12" y1="2" x2="12" y2="6"/><circle cx="9" cy="12" r="1" fill="currentColor"/><circle cx="15" cy="12" r="1" fill="currentColor"/><line x1="1" y1="10" x2="3" y2="10"/><line x1="1" y1="14" x2="3" y2="14"/><line x1="21" y1="10" x2="23" y2="10"/><line x1="21" y1="14" x2="23" y2="14"/>
          </svg>
          <span class="chat-action-label">切换</span>
        </button>
        <button class="chat-action-btn" @click="$emit('create-session')" title="新建会话">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <path d="M12 5v14M5 12h14"/>
          </svg>
          <span class="chat-action-label">新建</span>
        </button>
        <button class="chat-action-btn chat-action-btn-danger" :class="{ disabled: !currentSessionId }" @click="handleDelete" :title="currentSessionId ? '删除当前会话' : '无会话可删除'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
          </svg>
          <span class="chat-action-label">删除</span>
        </button>
      </div>
      <button class="chat-action-btn" @click="$emit('open-session-tab', 'tasks')" title="定时任务">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
        </svg>
        <span class="chat-action-label">定时</span>
      </button>
      <button class="chat-action-btn" :class="{ active: autoSpeechEnabled }" @click="$emit('toggle-auto-speech')" title="自动朗读">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
          <path d="M15.54 8.46a5 5 0 0 1 0 7.07"/>
          <path d="M19.07 4.93a10 10 0 0 1 0 14.14"/>
        </svg>
        <span class="chat-action-label">朗读</span>
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
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="24" height="24">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
          <polyline points="17 8 12 3 7 8"/>
          <line x1="12" y1="3" x2="12" y2="15"/>
        </svg>
        <span>松开上传文件</span>
      </div>
      <!-- Upload progress bars -->
      <div v-if="uploadingFiles.length > 0" class="chat-upload-progress">
        <div v-for="(f, idx) in uploadingFiles" :key="'prog-' + idx" class="upload-progress-item">
          <div class="upload-progress-bar" :style="{ width: f.progress + '%' }"></div>
        </div>
      </div>
      <!-- Attachment tags -->
      <div v-if="attachedFiles.length > 0 || pendingFiles.length > 0" class="chat-attachment-tags">
        <span v-for="(filePath, idx) in attachedFiles" :key="'att-' + filePath" class="chat-file-attachment attachment-ref" @click="$emit('file-tag-click', filePath)" title="打开文件">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="12" height="12">
            <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
          </svg>
          <span class="chat-file-name">{{ getFileName(filePath) }}</span>
          <button class="attachment-tag-remove" @click.stop="$emit('remove-attached', idx)" title="移除">×</button>
        </span>
        <span v-for="(f, idx) in pendingFiles" :key="'upload-' + idx" class="chat-file-attachment attachment-upload" :class="{ 'is-uploading': f.uploading }">
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
          <span class="chat-file-name">{{ getFileName(f.path) || '上传中...' }}</span>
          <span v-if="f.uploading" class="attachment-progress-pct">{{ f.progress }}%</span>
          <button class="attachment-tag-remove" @click.stop="$emit('remove-file', idx)" title="移除">×</button>
        </span>
      </div>
      <!-- Input row: attach + clear + textarea + send/stop -->
      <div class="chat-input-row">
        <div class="attach-menu-wrapper" ref="attachMenuRef">
          <button class="chat-attach-btn" @click.stop="toggleAttachMenu" :disabled="inputDisabled" title="附件">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
              <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
            </svg>
          </button>
        </div>
        <button v-if="inputText && !loading" class="chat-clear-btn" @click="inputText = ''; collapseTextarea()" title="清空输入">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <circle cx="12" cy="12" r="10"/>
            <line x1="15" y1="9" x2="9" y2="15"/>
            <line x1="9" y1="9" x2="15" y2="15"/>
          </svg>
        </button>
        <textarea class="chat-textarea"
          ref="textareaRef"
          v-model="inputText"
          :disabled="inputDisabled"
          :placeholder="pendingFiles.length > 0 ? '添加描述（可选）...' : '输入消息...'"
          rows="1"
          @keydown.enter.exact.prevent="$emit('send', inputText.trim())"
          @input="autoResizeTextarea"
          @blur="collapseTextarea"></textarea>
        <button v-if="loading" class="chat-stop-btn" @click="$emit('cancel')" title="停止生成">
          <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
          <span class="stop-btn-pulse"></span>
        </button>
        <button v-else class="chat-send-btn" @click="$emit('send', inputText.trim())" :class="{ disabled: inputDisabled && pendingFiles.length === 0 && attachedFiles.length === 0 }" title="发送">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <line x1="22" y1="2" x2="11" y2="13"/>
            <polygon points="22 2 15 22 11 13 2 9 22 2"/>
          </svg>
        </button>
      </div>
      <!-- Teleported attach menu (avoids overflow:hidden clipping) -->
      <Teleport to="body">
        <div v-if="showAttachMenu" class="attach-menu" :style="menuStyle" @click.stop>
          <!-- Current file group -->
          <template v-if="currentFile?.path && !attachedFiles.includes(currentFile.path)">
            <div class="attach-menu-group-title">当前文件</div>
            <button class="attach-menu-item" @click="handleAttachFile(currentFile.path)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                <polyline points="14 2 14 8 20 8"/>
              </svg>
              <span class="attach-menu-item-name">{{ getFileName(currentFile.path) }}</span>
            </button>
          </template>
          <!-- Recently referenced group -->
          <template v-if="recentReferencedFiles.length > 0">
            <div class="attach-menu-group-title">最近引用</div>
            <button v-for="item in recentReferencedFiles" :key="item.path" class="attach-menu-item" @click="handleAttachFile(item.path)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                <polyline points="14 2 14 8 20 8"/>
              </svg>
              <span class="attach-menu-item-name">{{ getFileName(item.path) }}</span>
              <span class="attach-menu-item-count">×{{ item.count }}</span>
            </button>
          </template>
          <!-- Separator + Upload -->
          <div v-if="hasFileGroups" class="attach-menu-separator"></div>
          <button class="attach-menu-item" @click="handleUploadClick">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="14" height="14">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <polyline points="17 8 12 3 7 8"/>
              <line x1="12" y1="3" x2="12" y2="15"/>
            </svg>
            <span>上传文件</span>
          </button>
        </div>
      </Teleport>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { baseName } from '@/utils/helpers.ts'

const props = defineProps({
  inputDisabled: Boolean,
  loading: Boolean,
  currentFile: Object,
  pendingFiles: Array,
  attachedFiles: Array,
  messages: Array,
  autoSpeechEnabled: Boolean,
  currentSessionId: String,
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
  'delete-session',
])

const inputText = ref('')
const textareaRef = ref(null)
const fileInputRef = ref(null)
const isDragOver = ref(false)
const dragCounter = ref(0)
const showAttachMenu = ref(false)
const attachMenuRef = ref(null)
const menuStyle = ref({})

const uploadingFiles = computed(() => props.pendingFiles.filter(f => f.uploading))

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
  return hasCurrent || recentReferencedFiles.value.length > 0
})

function handleDelete() {
  if (!props.currentSessionId) return
  if (confirm('确认删除当前会话？此操作不可撤销。')) {
    emit('delete-session')
  }
}

function getFileName(path) {
  return baseName(path)
}

function autoResizeTextarea() {
  const el = textareaRef.value
  if (!el) return
  el.style.height = 'auto'
  const lineHeight = parseFloat(getComputedStyle(el).lineHeight) || 20
  const maxHeight = lineHeight * 3 + 8
  el.style.height = Math.min(el.scrollHeight, maxHeight) + 'px'
}

function collapseTextarea() {
  const el = textareaRef.value
  if (!el) return
  el.style.height = 'auto'
}

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
  nextTick(() => collapseTextarea())
}

function handleAttachFile(filePath) {
  showAttachMenu.value = false
  emit('add-attached', filePath)
}

function handleUploadClick() {
  showAttachMenu.value = false
  fileInputRef.value?.click()
}

function toggleAttachMenu() {
  if (showAttachMenu.value) {
    showAttachMenu.value = false
    return
  }
  // Calculate menu position from the button's bounding rect
  const btn = attachMenuRef.value?.querySelector('.chat-attach-btn')
  if (btn) {
    const rect = btn.getBoundingClientRect()
    menuStyle.value = {
      position: 'fixed',
      bottom: `${window.innerHeight - rect.top + 4}px`,
      left: `${rect.left}px`,
    }
  }
  showAttachMenu.value = true
}

// Close menu on outside click
function handleClickOutside(e) {
  // The teleported menu is outside attachMenuRef, so check both
  const menuEl = document.querySelector('.attach-menu')
  if (menuEl && menuEl.contains(e.target)) return
  if (attachMenuRef.value && attachMenuRef.value.contains(e.target)) return
  showAttachMenu.value = false
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})

defineExpose({
  clearInput,
  inputText,
})
</script>

<style scoped>
/* Outer wrapper: top actions + input box stacked vertically */
.chat-input-wrapper {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  margin: 0 8px 8px;
}

/* Top action bar (above input box, compact) */
.chat-top-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 4px 4px;
}

/* Session button group */
.chat-action-group {
  display: inline-flex;
  align-items: center;
  gap: 1px;
  background: var(--bg-tertiary, #f0f0f0);
  border-radius: 6px;
  padding: 1px;
}

.chat-action-group .chat-action-btn {
  border-radius: 5px;
}

.chat-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted, #999);
  padding: 3px 6px;
  border-radius: 4px;
  font-size: 11px;
  line-height: 1;
  transition: color 0.15s, background 0.15s;
}

@media (hover: hover) {
  .chat-action-btn:hover {
    color: var(--accent-color, #0066cc);
    background: var(--bg-tertiary, #f0f0f0);
  }
}

.chat-action-btn.active {
  color: var(--accent-color, #0066cc);
  background: color-mix(in srgb, var(--accent-color, #0066cc) 10%, transparent);
}

.chat-action-btn-danger:not(.disabled) {
  color: var(--danger-color, #dc3545);
}

@media (hover: hover) {
  .chat-action-btn-danger:not(.disabled):hover {
    color: #fff;
    background: var(--danger-color, #dc3545);
  }
}

.chat-action-btn-danger.disabled {
  opacity: 0.4;
  cursor: not-allowed;
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
  background: var(--bg-primary, #fff);
  flex: 1;
  min-width: 0;
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 12px;
  overflow: hidden;
  position: relative;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.chat-input-container.drag-over {
  border-color: var(--accent-color, #0066cc);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent-color, #0066cc) 20%, transparent);
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
  border-radius: 12px;
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

/* Attach menu (teleported to body, uses fixed positioning) */
.attach-menu {
  position: fixed;
  background: var(--bg-secondary, #fff);
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 8px;
  box-shadow: 0 -4px 12px rgba(0, 0, 0, 0.12);
  z-index: 9999;
  min-width: 140px;
  max-width: 200px;
  padding: 3px 0;
}

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
  max-height: 68px;
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
  position: relative;
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
  animation: heartbeat 1.4s ease-in-out infinite;
}
.chat-stop-btn:hover { opacity: 0.85; }

.stop-btn-pulse {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  animation: pulse-ring 1.4s ease-out infinite;
}

@keyframes heartbeat {
  0%, 100% { transform: scale(1); }
  14% { transform: scale(1.12); }
  28% { transform: scale(1); }
  42% { transform: scale(1.08); }
  56% { transform: scale(1); }
}

@keyframes pulse-ring {
  0% { box-shadow: 0 0 0 0 rgba(220, 53, 69, 0.45); }
  70% { box-shadow: 0 0 0 8px rgba(220, 53, 69, 0); }
  100% { box-shadow: 0 0 0 0 rgba(220, 53, 69, 0); }
}
</style>

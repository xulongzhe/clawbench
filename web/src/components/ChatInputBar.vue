<template>
  <div class="chat-input-container">
    <input type="file" ref="fileInputRef" @change="onFileSelect" style="display:none" multiple />
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
        @click="$emit('add-attached', currentFile?.path)"
        :disabled="inputDisabled || !currentFile?.path || attachedFiles.includes(currentFile?.path)"
        :title="!currentFile?.path ? '无当前文件' : attachedFiles.includes(currentFile?.path) ? '已附带此文件' : '附带当前文件'">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
        </svg>
      </button>
      <button class="chat-toolbar-btn" @click="$emit('open-session-tab', 'sessions')" title="会话管理">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
        </svg>
      </button>
      <button class="chat-toolbar-btn" @click="$emit('open-session-tab', 'tasks')" title="定时任务">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
        </svg>
      </button>
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
        <button class="attachment-tag-remove" @click.stop="$emit('remove-file', idx)" title="移除">×</button>
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
        @keydown.enter.exact.prevent="$emit('send', inputText.trim())"
        @input="autoResizeTextarea"
        @blur="collapseTextarea"
        @dblclick="inputText = ''"></textarea>
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
  </div>
</template>

<script setup>
import { ref, nextTick } from 'vue'
import { baseName } from '@/utils/helpers.ts'

const props = defineProps({
  inputDisabled: Boolean,
  loading: Boolean,
  currentFile: Object,
  pendingFiles: Array,
  attachedFiles: Array,
})

const emit = defineEmits([
  'send',
  'cancel',
  'file-select',
  'remove-file',
  'add-attached',
  'remove-attached',
  'open-session-tab',
  'file-tag-click',
])

const inputText = ref('')
const textareaRef = ref(null)
const fileInputRef = ref(null)

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

function clearInput() {
  inputText.value = ''
  nextTick(() => collapseTextarea())
}

defineExpose({
  clearInput,
  inputText,
})
</script>

<style scoped>
/* Unified input container */
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

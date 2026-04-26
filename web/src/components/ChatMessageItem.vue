<template>
  <div class="chat-message" :class="[msg.role, { 'has-metadata': msg.role === 'assistant' && msg.metadata }]">

    <div v-if="msg.role === 'user' && msg.files && msg.files.length > 0 && !hasImagesInContent(msg.content)" class="chat-files">
      <template v-for="(f, idx) in msg.files" :key="idx">
        <span v-if="isUploadPath(normalizeFileEntry(f).path)" class="chat-file-attachment attachment-upload" @click="$emit('file-tag-click', normalizeFileEntry(f).path)" title="打开文件">
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
        <span v-else class="chat-file-attachment attachment-ref" @click="$emit('file-tag-click', normalizeFileEntry(f).path)" title="打开文件">
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
        <div v-if="block.type === 'thinking'" class="chat-thinking" :class="{ expanded: thinkingExpanded[`${index}-${bi}`] }" @click.stop="toggleThinking(`${index}-${bi}`)">
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
          <pre v-if="thinkingExpanded[`${index}-${bi}`]" class="thinking-text">{{ block.text }}</pre>
        </div>
        <!-- Tool use block -->
        <template v-else-if="block.type === 'tool_use'">
          <div class="chat-tool-call" :class="{ done: block.done }" @click.stop="$emit('toggle-tool', `${index}-${bi}`)">
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
          <pre v-if="block.input && Object.keys(block.input).length && expandedTools[`${index}-${bi}`]" class="tool-detail" @click.stop v-html="formatToolInput(block.input)"></pre>
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
    <div v-else-if="msg.role === 'user' || msg.content" v-html="renderedContent"></div>

    <!-- Bottom bar for assistant messages with metadata -->
    <div v-if="msg.role === 'assistant' && msg.metadata" class="chat-meta-bar">
      <span class="chat-meta-info">
        <span v-if="msg.backend">{{ msg.backend }}</span>
        <span v-if="msg.metadata.model" class="chat-meta-sep">{{ msg.metadata.model }}</span>
        <span v-if="msg.createdAt" class="chat-meta-sep">{{ formatMessageTime(msg.createdAt) }}</span>
      </span>
      <button class="chat-info-btn" @click="$emit('show-metadata', msg)" title="查看详情">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="16" x2="12" y2="12"/>
          <line x1="12" y1="8" x2="12.01" y2="8"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, inject } from 'vue'
import { baseName } from '@/utils/helpers.ts'

const props = defineProps({
  msg: Object,
  index: Number,
  expandedTools: Object,
  blockProposals: Object,
  agents: Array,
  renderedContent: String,
})

const emit = defineEmits(['toggle-tool', 'show-metadata', 'file-tag-click'])

const thinkingExpanded = ref({})

function toggleThinking(key) {
  thinkingExpanded.value = { ...thinkingExpanded.value, [key]: !thinkingExpanded.value[key] }
}

const chatRender = inject('chatRender', {})
const chatSession = inject('chatSession', {})

const { renderTextBlock, formatMessageTime, toolCallSummary, formatToolInput, humanizeCron, repeatLabel, truncate, hasImagesInContent } = chatRender
const { getAgentIcon, getAgentName } = chatSession

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
</script>

<style scoped>
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

.chat-thinking .thinking-text {
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
</style>

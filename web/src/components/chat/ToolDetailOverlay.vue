<template>
  <BottomSheet :open="show" auto @close="$emit('close')">
    <template #header>
      <div class="tool-detail-header" :data-category="category">
        <component :is="headerIcon" :size="14" class="tool-detail-header-icon" />
        <span class="tool-detail-header-name">{{ toolName }}</span>
        <span v-if="toolSummary" class="tool-detail-header-summary">{{ toolSummary }}</span>
        <span v-if="!toolDone" class="tool-detail-spinner"></span>
        <XCircle v-else-if="toolStatus === 'error'" :size="14" color="#ef4444" class="tool-detail-status" />
        <CheckCircle2 v-else :size="14" color="#22c55e" class="tool-detail-status" />
      </div>
    </template>
    <div class="tool-detail-body" @click="handleBodyClick">
      <div v-html="toolInputHtml"></div>
      <!-- Tool output section -->
      <div v-if="toolOutputHtml" class="tool-output-section">
        <div class="tool-output-header">
          <span class="tool-output-label">output</span>
          <span v-if="toolStatus === 'error'" class="tool-output-status tool-output-error">error</span>
          <span v-else class="tool-output-status tool-output-success">ok</span>
        </div>
        <div class="tool-output-body" v-html="toolOutputHtml"></div>
      </div>
    </div>
  </BottomSheet>
</template>

<script setup>
import { computed } from 'vue'
import { CheckCircle2, XCircle } from 'lucide-vue-next'
import BottomSheet from '@/components/common/BottomSheet.vue'
import { getToolIcon } from '@/utils/icons'
import { handleToolAction } from '@/utils/renderToolDetail.ts'
import { useAppMode } from '@/composables/useAppMode.ts'
import { usePortForward } from '@/composables/usePortForward.ts'
import { useToast } from '@/composables/useToast.ts'
import { isLocalhostUrl, parseLocalhostUrl } from '@/composables/useLocalhostAnnotation.ts'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  show: { type: Boolean, default: false },
  toolName: { type: String, default: '' },
  toolSummary: { type: String, default: '' },
  toolInputHtml: { type: String, default: '' },
  toolOutputHtml: { type: String, default: '' },
  toolStatus: { type: String, default: '' },
  toolDone: { type: Boolean, default: true },
})

const emit = defineEmits(['close', 'file-open', 'send-message'])

const category = computed(() => getToolIcon(props.toolName).category)
const headerIcon = computed(() => getToolIcon(props.toolName).icon)

const { isAppMode } = useAppMode()
const { ensurePortRegistered, openPort } = usePortForward()
const toast = useToast()
const { t } = useI18n()
let urlOpening = false

async function openLocalhostUrl(element, port, protocol) {
  if (urlOpening) return
  urlOpening = true
  element.classList.add('loading')

  try {
    await ensurePortRegistered(port, protocol)
    openPort(port, protocol)
  } catch (err) {
    toast.show(t('chat.localhost.openFailed'), { type: 'error' })
  } finally {
    urlOpening = false
    element.classList.remove('loading')
  }
}

function handleBodyClick(event) {
  if (props.toolName && handleToolAction(props.toolName, event, emit)) return

  // Handle localhost URL open buttons — bottom sheet is teleported to <body>,
  // ChatMessageList's handleChatClick won't see these clicks.
  if (isAppMode.value) {
    const urlBtn = event.target.closest('.chat-url-open-btn')
    if (urlBtn) {
      event.preventDefault()
      event.stopPropagation()
      const port = parseInt(urlBtn.getAttribute('data-port') || '0')
      const protocol = urlBtn.getAttribute('data-protocol') || 'http'
      if (port > 0) {
        openLocalhostUrl(urlBtn, port, protocol)
      }
      return
    }

    // Intercept <a> clicks on localhost URLs in tool output
    const anchor = event.target.closest('a[href]')
    if (anchor) {
      const href = anchor.getAttribute('href') || ''
      if (isLocalhostUrl(href)) {
        event.preventDefault()
        event.stopPropagation()
        const parsed = parseLocalhostUrl(href)
        if (parsed) {
          openLocalhostUrl(anchor, parsed.port, parsed.protocol)
        }
        return
      }
    }
  }

  // Handle commit-hash clicks (span or button) — bottom sheet is teleported to <body>,
  // ChatMessageList's handleChatClick won't see these clicks.
  const commitEl = event.target.closest('.chat-commit-hash, .chat-commit-open-btn')
  if (commitEl) {
    const sha = commitEl.getAttribute('data-commit-sha')
    if (sha) {
      window.dispatchEvent(new CustomEvent('navigate-to-commit', { detail: { sha } }))
    }
    return
  }
  // Handle file-open buttons
  const fileBtn = event.target.closest('.chat-file-open-btn')
  if (fileBtn) {
    const filePath = fileBtn.getAttribute('data-file-path')
    if (filePath) emit('file-open', filePath)
    return
  }
  event.stopPropagation()
}
</script>

<style scoped>
/* Header — tool-specific accent colors */
.tool-detail-header {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 1;
  --tool-accent: var(--text-muted);
}

.tool-detail-header[data-category="file"]     { --tool-accent: var(--accent-color); }
.tool-detail-header[data-category="bash"]     { --tool-accent: #10b981; }
.tool-detail-header[data-category="search"]   { --tool-accent: #8b5cf6; }
.tool-detail-header[data-category="task"]     { --tool-accent: #f59e0b; }
.tool-detail-header[data-category="plan"]     { --tool-accent: var(--accent-color); }
.tool-detail-header[data-category="agent"]    { --tool-accent: #ec4899; }
.tool-detail-header[data-category="skill"]    { --tool-accent: #06b6d4; }
.tool-detail-header[data-category="ask"]      { --tool-accent: #f97316; }
.tool-detail-header[data-category="fallback"] { --tool-accent: var(--text-muted); }

:root[data-theme="dark"] .tool-detail-header[data-category="bash"]   { --tool-accent: #34d399; }
:root[data-theme="dark"] .tool-detail-header[data-category="search"] { --tool-accent: #a78bfa; }
:root[data-theme="dark"] .tool-detail-header[data-category="task"]   { --tool-accent: #fbbf24; }
:root[data-theme="dark"] .tool-detail-header[data-category="agent"]  { --tool-accent: #f472b6; }
:root[data-theme="dark"] .tool-detail-header[data-category="skill"]  { --tool-accent: #22d3ee; }
:root[data-theme="dark"] .tool-detail-header[data-category="ask"]    { --tool-accent: #fb923c; }

.tool-detail-header-icon {
  color: color-mix(in srgb, var(--tool-accent) 80%, transparent);
  flex-shrink: 0;
}

.tool-detail-header-name {
  font-weight: 600;
  color: var(--tool-accent);
  font-size: 13px;
  flex-shrink: 0;
}

.tool-detail-header-summary {
  color: var(--text-tertiary, #888);
  font-size: 12px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.tool-detail-status {
  flex-shrink: 0;
  margin-left: auto;
}

.tool-detail-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid var(--border-color);
  border-top-color: var(--tool-accent);
  border-radius: 50%;
  animation: tool-spin 0.6s linear infinite;
  flex-shrink: 0;
  margin-left: auto;
}

@keyframes tool-spin {
  to { transform: rotate(360deg); }
}

/* Body */
.tool-detail-body {
  padding: 12px 14px;
  overflow-y: auto;
  overflow-x: hidden;
  font-size: 12px;
  line-height: 1.5;
  flex: 1;
  cursor: default;
}

/* Tint the bottom sheet header with tool accent color */
:deep(.bs-header) {
  --tool-accent: var(--text-muted);
  background: color-mix(in srgb, var(--tool-accent) 5%, transparent);
  border-bottom-color: color-mix(in srgb, var(--tool-accent) 15%, var(--border-color));
}
</style>

<style>
/* Non-scoped styles for v-html penetration — tool detail rendering in bottom sheet */
.tool-detail-body .tool-output-section {
  margin-top: 8px;
  border-top: 1px solid var(--border-color);
  padding-top: 8px;
}

.tool-detail-body .tool-output-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}

.tool-detail-body .tool-output-label {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(34, 197, 94, 0.12);
  color: #16a34a;
  font-weight: 600;
}

:root[data-theme="dark"] .tool-detail-body .tool-output-label {
  background: rgba(74, 222, 128, 0.15);
  color: #4ade80;
}

.tool-detail-body .tool-output-status {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  font-weight: 600;
}

.tool-detail-body .tool-output-success {
  background: rgba(34, 197, 94, 0.12);
  color: #16a34a;
}

:root[data-theme="dark"] .tool-detail-body .tool-output-success {
  background: rgba(74, 222, 128, 0.15);
  color: #4ade80;
}

.tool-detail-body .tool-output-error {
  background: rgba(239, 68, 68, 0.12);
  color: #dc2626;
}

:root[data-theme="dark"] .tool-detail-body .tool-output-error {
  background: rgba(248, 113, 113, 0.15);
  color: #fca5a5;
}

.tool-detail-body .tool-output-body {
  overflow-y: auto;
  font-size: 12px;
  line-height: 1.5;
}

.tool-detail-body .tool-output-body pre {
  margin: 0;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.tool-detail-body .tool-output-default pre {
  background: var(--bg-tertiary);
  border-radius: 4px;
  padding: 8px 10px;
}

/* File header */
.tool-detail-body .tool-file-header {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin-bottom: 6px;
  padding-bottom: 6px;
  padding-right: 22px;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}

.tool-detail-body .tool-file-header .chat-file-open-btn {
  position: absolute;
  top: 0;
  right: 0;
  flex-shrink: 0;
}

/* Base style for file-open buttons in tool detail */
.tool-detail-body .chat-file-open-btn {
  background: none;
  border: none;
  padding: 2px;
  cursor: pointer;
  color: var(--text-muted, #999);
  border-radius: 3px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s, background 0.15s;
}

.tool-detail-body .chat-file-open-btn:hover {
  color: var(--accent-color, #4a90d9);
  background: var(--bg-tertiary, #f0f0f0);
}

.tool-detail-body .tool-file-path {
  font-family: 'SF Mono', 'Fira Code', Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  color: var(--accent-color);
  word-break: break-all;
  flex: 1;
  min-width: 0;
}

/* Edit diff */
.tool-detail-body .edit-diff-view {
  display: flex;
  flex-direction: column;
  font-size: 12px;
  line-height: 1.6;
}

.tool-detail-body .edit-diff-replace-all {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(245, 158, 11, 0.12);
  color: #d97706;
  font-weight: 600;
  white-space: nowrap;
}

.tool-detail-body .edit-diff-scroll {
  overflow-x: auto;
}

.tool-detail-body .edit-diff-body {
  white-space: pre;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  line-height: 1.6;
  min-width: max-content;
}

.tool-detail-body .edit-diff-del {
  background: rgba(239, 68, 68, 0.08);
  color: #dc2626;
  white-space: pre;
}

.tool-detail-body .edit-diff-add {
  background: rgba(34, 197, 94, 0.08);
  color: #16a34a;
  white-space: pre;
}

:root[data-theme="dark"] .tool-detail-body .edit-diff-del {
  background: rgba(248, 113, 113, 0.1);
  color: #fca5a5;
}

:root[data-theme="dark"] .tool-detail-body .edit-diff-add {
  background: rgba(74, 222, 128, 0.1);
  color: #86efac;
}

:root[data-theme="dark"] .tool-detail-body .edit-diff-replace-all {
  background: rgba(251, 191, 36, 0.15);
  color: #fbbf24;
}

/* File preview */
.tool-detail-body .file-preview-view {
  display: flex;
  flex-direction: column;
  font-size: 12px;
  line-height: 1.6;
}

.tool-detail-body .file-preview-body {
  white-space: pre;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  line-height: 1.6;
  overflow-x: auto;
}

.tool-detail-body .file-preview-line {
  white-space: pre;
  color: var(--text-primary);
}

.tool-detail-body .file-preview-meta {
  white-space: normal;
  color: var(--text-muted, #999);
  font-style: italic;
  padding: 4px 0;
}

/* File write */
.tool-detail-body .file-write-view {
  display: flex;
  flex-direction: column;
  font-size: 12px;
  line-height: 1.6;
}

.tool-detail-body .file-write-badge {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
  font-weight: 600;
  white-space: nowrap;
}

:root[data-theme="dark"] .tool-detail-body .file-write-badge {
  background: rgba(96, 165, 250, 0.15);
  color: #93c5fd;
}

.tool-detail-body .file-write-body {
  white-space: pre;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  line-height: 1.6;
  overflow-x: auto;
}

.tool-detail-body .file-write-line {
  white-space: pre;
  color: var(--text-primary);
}

/* JSON fallback */
.tool-detail-body .tool-json-body {
  white-space: pre;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  line-height: 1.5;
  overflow-x: auto;
}

.tool-detail-body .tool-json-body code {
  font-family: inherit;
}

/* Bash terminal */
.tool-detail-body .bash-terminal-view {
  white-space: normal;
}

.tool-detail-body .bash-terminal-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 6px;
  white-space: pre-wrap;
  word-break: break-word;
}

.tool-detail-body .bash-terminal-body {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  line-height: 1.6;
  background: var(--bg-tertiary);
  border-radius: 6px;
  padding: 8px 10px;
  white-space: pre-wrap;
  word-break: break-word;
}

.tool-detail-body .bash-prompt {
  color: #16a34a;
  font-weight: 700;
  margin-right: 4px;
}

:root[data-theme="dark"] .tool-detail-body .bash-prompt {
  color: #4ade80;
}

.tool-detail-body .bash-command {
  color: var(--text-primary);
}

/* Bash output */
.tool-detail-body .bash-output-body pre {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  line-height: 1.6;
  background: var(--bg-tertiary);
  border-radius: 6px;
  padding: 8px 10px;
  white-space: pre-wrap;
  word-break: break-word;
}

/* Grep search */
.tool-detail-body .grep-search-view {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  line-height: 1.5;
}

.tool-detail-body .grep-pattern-row,
.tool-detail-body .grep-path-row {
  display: flex;
  align-items: flex-start;
  gap: 6px;
}

.tool-detail-body .grep-label {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(139, 92, 246, 0.12);
  color: #7c3aed;
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
  line-height: 1.5;
}

:root[data-theme="dark"] .tool-detail-body .grep-label {
  background: rgba(167, 139, 250, 0.15);
  color: #a78bfa;
}

.tool-detail-body .grep-pattern-text,
.tool-detail-body .grep-path-text {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text-primary);
}

.tool-detail-body .grep-mode-tag {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(139, 92, 246, 0.08);
  color: #8b5cf6;
  font-weight: 500;
  align-self: flex-start;
}

:root[data-theme="dark"] .tool-detail-body .grep-mode-tag {
  background: rgba(167, 139, 250, 0.12);
  color: #a78bfa;
}

/* Glob pattern */
.tool-detail-body .glob-pattern-view {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  line-height: 1.5;
}

.tool-detail-body .glob-pattern-row,
.tool-detail-body .glob-path-row {
  display: flex;
  align-items: flex-start;
  gap: 6px;
}

.tool-detail-body .glob-label {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(139, 92, 246, 0.12);
  color: #7c3aed;
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
  line-height: 1.5;
}

:root[data-theme="dark"] .tool-detail-body .glob-label {
  background: rgba(167, 139, 250, 0.15);
  color: #a78bfa;
}

.tool-detail-body .glob-pattern-text,
.tool-detail-body .glob-path-text {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text-primary);
}

/* WebSearch */
.tool-detail-body .web-search-view {
  font-size: 12px;
  line-height: 1.5;
}

.tool-detail-body .web-search-query {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  color: var(--text-primary);
}

.tool-detail-body .web-search-icon {
  flex-shrink: 0;
  font-size: 14px;
  line-height: 1.4;
}

.tool-detail-body .web-search-text {
  white-space: pre-wrap;
  word-break: break-word;
}

/* WebFetch */
.tool-detail-body .web-fetch-view {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  line-height: 1.5;
}

.tool-detail-body .web-fetch-url-row {
  display: flex;
  align-items: flex-start;
  gap: 6px;
}

.tool-detail-body .web-fetch-label {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(139, 92, 246, 0.12);
  color: #7c3aed;
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
  line-height: 1.5;
}

:root[data-theme="dark"] .tool-detail-body .web-fetch-label {
  background: rgba(167, 139, 250, 0.15);
  color: #a78bfa;
}

.tool-detail-body .web-fetch-link {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  color: var(--accent-color);
  text-decoration: none;
  word-break: break-all;
}

.tool-detail-body .web-fetch-link:hover {
  text-decoration: underline;
}

.tool-detail-body .web-fetch-text {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text-primary);
}

.tool-detail-body .web-fetch-prompt {
  color: var(--text-secondary);
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
}

/* Agent call */
.tool-detail-body .agent-call-view {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  line-height: 1.5;
}

.tool-detail-body .agent-call-header {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.tool-detail-body .agent-type-badge {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(236, 72, 153, 0.12);
  color: #db2777;
  font-weight: 600;
  white-space: nowrap;
}

:root[data-theme="dark"] .tool-detail-body .agent-type-badge {
  background: rgba(244, 114, 182, 0.15);
  color: #f472b6;
}

.tool-detail-body .agent-call-desc {
  color: var(--text-primary);
  font-weight: 500;
}

.tool-detail-body .agent-call-prompt {
  color: var(--text-secondary);
  font-size: 12px;
  white-space: normal;
  word-break: break-word;
  padding: 6px 10px;
  background: var(--bg-tertiary);
  border-radius: 6px;
  font-family: inherit;
  line-height: 1.6;
}
.tool-detail-body .agent-call-prompt p:first-child {
  margin-top: 0;
}
.tool-detail-body .agent-call-prompt p:last-child {
  margin-bottom: 0;
}
.tool-detail-body .agent-call-prompt h1,
.tool-detail-body .agent-call-prompt h2,
.tool-detail-body .agent-call-prompt h3,
.tool-detail-body .agent-call-prompt h4 {
  font-size: 13px;
  font-weight: 600;
  margin: 8px 0 4px;
  color: var(--text-primary);
}
.tool-detail-body .agent-call-prompt ul,
.tool-detail-body .agent-call-prompt ol {
  margin: 4px 0;
  padding-left: 20px;
}
.tool-detail-body .agent-call-prompt li {
  margin: 2px 0;
}
.tool-detail-body .agent-call-prompt code {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 11px;
  background: color-mix(in srgb, var(--text-secondary) 8%, transparent);
  padding: 1px 4px;
  border-radius: 3px;
}
.tool-detail-body .agent-call-prompt pre {
  margin: 4px 0;
  padding: 6px 8px;
  background: var(--bg-secondary);
  border-radius: 4px;
  overflow-x: auto;
}
.tool-detail-body .agent-call-prompt pre code {
  background: none;
  padding: 0;
  font-size: 12px;
}
.tool-detail-body .agent-call-prompt strong {
  font-weight: 600;
  color: var(--text-primary);
}
.tool-detail-body .agent-call-prompt hr {
  border: none;
  border-top: 1px solid var(--border-color);
  margin: 6px 0;
}

/* Skill call */
.tool-detail-body .skill-call-view {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  line-height: 1.5;
}

.tool-detail-body .skill-call-header {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tool-detail-body .skill-call-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.tool-detail-body .skill-call-name {
  font-weight: 600;
  color: #0891b2;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
}

:root[data-theme="dark"] .tool-detail-body .skill-call-name {
  color: #22d3ee;
}

.tool-detail-body .skill-call-args {
  color: var(--text-secondary);
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
  padding: 6px 10px;
  background: var(--bg-tertiary);
  border-radius: 6px;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  line-height: 1.5;
}

/* Thinking content in overlay — plain text (legacy) */
.tool-detail-body .thinking-overlay-text {
  margin: 0;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text-secondary);
}

/* Thinking content in overlay — markdown rendered */
.tool-detail-body .thinking-overlay-md {
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-secondary);
  word-break: break-word;
}
.tool-detail-body .thinking-overlay-md p {
  margin: 0 0 0.5em;
}
.tool-detail-body .thinking-overlay-md p:last-child {
  margin-bottom: 0;
}
.tool-detail-body .thinking-overlay-md pre {
  margin: 0.5em 0;
  padding: 8px;
  background: var(--bg-tertiary, rgba(0,0,0,0.04));
  border-radius: 4px;
  overflow-x: auto;
  font-size: 12px;
}
.tool-detail-body .thinking-overlay-md code {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
}
.tool-detail-body .thinking-overlay-md :not(pre) > code {
  padding: 1px 4px;
  background: var(--bg-tertiary, rgba(0,0,0,0.06));
  border-radius: 3px;
}
.tool-detail-body .thinking-overlay-md ul,
.tool-detail-body .thinking-overlay-md ol {
  margin: 0.3em 0;
  padding-left: 1.5em;
}
.tool-detail-body .thinking-overlay-md li {
  margin: 0.15em 0;
}
.tool-detail-body .thinking-overlay-md blockquote {
  margin: 0.5em 0;
  padding-left: 0.8em;
  border-left: 3px solid var(--border-color, rgba(0,0,0,0.12));
  color: var(--text-secondary);
}
.tool-detail-body .thinking-overlay-md h1,
.tool-detail-body .thinking-overlay-md h2,
.tool-detail-body .thinking-overlay-md h3 {
  margin: 0.6em 0 0.3em;
  font-size: 1em;
  font-weight: 600;
}
.tool-detail-body .thinking-overlay-md a {
  color: var(--accent-color, #0066cc);
}
.tool-detail-body .thinking-overlay-md table {
  border-collapse: collapse;
  margin: 0.5em 0;
  font-size: 12px;
}
.tool-detail-body .thinking-overlay-md th,
.tool-detail-body .thinking-overlay-md td {
  border: 1px solid var(--border-color, rgba(0,0,0,0.12));
  padding: 4px 8px;
}
.tool-detail-body .thinking-overlay-md th {
  background: var(--bg-tertiary, rgba(0,0,0,0.04));
  font-weight: 600;
}

/* ── Localhost URL open button in tool output (same pattern as ChatMessageItem) ── */
.tool-detail-body .chat-url-open-btn {
  background: none;
  border: none;
  padding: 2px;
  cursor: pointer;
  color: var(--text-muted, #999);
  border-radius: 3px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s, background 0.15s;
  font-size: 12px;
  line-height: 1;
  vertical-align: baseline;
}

.tool-detail-body .chat-url-open-btn:hover {
  color: var(--accent-color, #4a90d9);
  background: var(--bg-tertiary, #f0f0f0);
}

.tool-detail-body .chat-url-open-btn.loading {
  opacity: 0.5;
  pointer-events: none;
}

.tool-detail-body .chat-url-open-btn.loading::after {
  content: '';
  width: 8px;
  height: 8px;
  border: 1.5px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: url-btn-spin 0.6s linear infinite;
  margin-left: 2px;
  display: inline-block;
}

/* Localhost <a> links in tool output pre blocks */
.tool-detail-body pre a[href] {
  color: var(--accent-color, #4a90d9);
  text-decoration: none;
}

.tool-detail-body pre a[href]:hover {
  text-decoration: underline;
}
</style>

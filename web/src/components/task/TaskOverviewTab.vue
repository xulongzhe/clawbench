<template>
  <div class="task-overview">
    <!-- Scrollable content -->
    <div class="overview-scroll">
      <!-- Info card -->
      <div class="overview-card">
        <!-- Task ID -->
        <div class="overview-row">
          <span class="overview-label">ID</span>
          <span class="overview-value task-id-value" @click="copyId" :title="t('common.copy')">{{ task.id }}</span>
        </div>
        <!-- Status -->
        <div class="overview-row">
          <span class="overview-label">{{ t('chat.contentBlocks.status') }}</span>
          <span class="overview-value">
            <span class="status-dot" :class="task.status"></span>
            <span :class="['status-text', task.status]">{{ statusText }}</span>
          </span>
        </div>
        <!-- Frequency -->
        <div class="overview-row">
          <span class="overview-label">{{ t('chat.contentBlocks.frequency') }}</span>
          <span class="overview-value">{{ humanizeCron(task.cronExpr) }}</span>
        </div>
        <!-- Agent -->
        <div class="overview-row">
          <span class="overview-label">{{ t('chat.contentBlocks.executor') }}</span>
          <span class="overview-value">
            <span class="agent-icon">{{ getAgentIcon(task.agentId) }}</span>
            <span class="agent-name">{{ getAgentName(task.agentId) }}</span>
          </span>
        </div>
        <!-- Repeat mode -->
        <div class="overview-row">
          <span class="overview-label">{{ t('chat.contentBlocks.repeat') }}</span>
          <span class="overview-value">{{ repeatLabel(task.repeatMode, task.maxRuns) }}</span>
        </div>
        <!-- Run count -->
        <div v-if="task.runCount > 0" class="overview-row">
          <span class="overview-label">{{ t('chat.contentBlocks.statusExecutions', { count: task.runCount }) }}</span>
        </div>
        <!-- Next run -->
        <div v-if="task.nextRunAt" class="overview-row">
          <span class="overview-label">{{ t('chat.contentBlocks.nextRun') }}</span>
          <span class="overview-value">{{ formatDateTime(task.nextRunAt) }}</span>
        </div>
      </div>

      <!-- Prompt preview card -->
      <div class="overview-card">
        <div class="prompt-header" @click="promptExpanded = !promptExpanded">
          <span class="overview-label">{{ t('task.form.prompt') }}</span>
          <span class="prompt-toggle">{{ promptExpanded ? '▾' : '▸' }}</span>
        </div>
        <div v-if="promptExpanded" class="prompt-body markdown-body" v-html="renderedPrompt"></div>
        <div v-else class="prompt-body collapsed">
          <div class="prompt-preview-text" v-html="renderedPrompt"></div>
          <div class="prompt-fade"></div>
        </div>
      </div>
    </div>

    <!-- Fixed bottom action bar -->
    <div class="overview-actions">
      <button class="action-btn" @click="$emit('edit')">
        <Pencil :size="12" />
        <span class="action-text">{{ t('common.edit') }}</span>
      </button>
      <button v-if="task.runCount > 0 || task.runningCount > 0" class="action-btn" @click="$emit('history')">
        <Clock :size="12" />
        <span class="action-text">{{ t('task.history') }}</span>
      </button>
      <span class="actions-spacer"></span>
      <template v-if="task.status === 'active'">
        <button class="action-btn accent" :disabled="actionLoading" @click="triggerTask">
          <Zap :size="12" />
          <span class="action-text">{{ t('task.run') }}</span>
        </button>
        <button class="action-btn warn" :disabled="actionLoading" @click="pauseTask">
          <Pause :size="12" />
          <span class="action-text">{{ t('task.pause') }}</span>
        </button>
        <button class="action-btn danger" :disabled="actionLoading" @click="deleteTask">
          <Trash2 :size="12" />
          <span class="action-text">{{ t('task.delete') }}</span>
        </button>
      </template>
      <template v-else-if="task.status === 'paused'">
        <button class="action-btn accent" :disabled="actionLoading" @click="triggerTask">
          <Zap :size="12" />
          <span class="action-text">{{ t('task.run') }}</span>
        </button>
        <button class="action-btn success" :disabled="actionLoading" @click="resumeTask">
          <Play :size="12" />
          <span class="action-text">{{ t('task.resume') }}</span>
        </button>
        <button class="action-btn danger" :disabled="actionLoading" @click="deleteTask">
          <Trash2 :size="12" />
          <span class="action-text">{{ t('task.delete') }}</span>
        </button>
      </template>
      <template v-else-if="task.status === 'completed'">
        <button class="action-btn danger" :disabled="actionLoading" @click="deleteTask">
          <Trash2 :size="12" />
          <span class="action-text">{{ t('task.delete') }}</span>
        </button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { Pencil, Pause, Play, Zap, Trash2, Clock } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { useTaskOverview } from '@/composables/useTaskOverview.ts'
import { useMarkdownRenderer } from '@/composables/useMarkdownRenderer'
import { useAgents } from '@/composables/useAgents'
import { humanizeCron, repeatLabel, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const { renderMarkdown } = useMarkdownRenderer()
const { getAgentIcon, getAgentName } = useAgents()

const props = defineProps<{
  task: any
}>()

const emit = defineEmits<{
  (e: 'deleted'): void
  (e: 'edit'): void
  (e: 'history'): void
}>()

// Task overview composable (ISS-011 + ISS-014)
const { actionLoading, triggerTask, pauseTask, resumeTask, deleteTask } = useTaskOverview({
  task: computed(() => props.task),
  emit: {
    deleted: () => emit('deleted'),
    edit: () => emit('edit'),
    history: () => emit('history'),
  },
})

const promptExpanded = ref(true)

function copyId() {
  if (props.task.id) {
    navigator.clipboard.writeText(String(props.task.id)).catch(() => {})
  }
}

const statusText = computed(() => {
  if (props.task.runningCount > 0) return t('chat.contentBlocks.statusRunning')
  const map: Record<string, string> = {
    active: t('chat.contentBlocks.statusActive'),
    paused: t('chat.contentBlocks.statusPaused'),
    completed: t('chat.contentBlocks.statusCompleted'),
  }
  return map[props.task.status] || props.task.status
})

const renderedPrompt = computed(() => {
  return renderMarkdown(props.task.prompt || '', { sanitize: true })
})
</script>

<style scoped>
.task-overview {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.overview-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.overview-card {
  background: var(--bg-secondary, #f5f5f5);
  border-radius: 10px;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.overview-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 22px;
}

.overview-label {
  font-size: 12px;
  color: var(--text-muted, #999);
  flex-shrink: 0;
}

.overview-value {
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  display: flex;
  align-items: center;
  gap: 5px;
  text-align: right;
  word-break: break-word;
}

.task-id-value {
  font-size: 13px;
  font-family: monospace;
  color: var(--text-muted, #999);
  cursor: pointer;
  user-select: all;
  padding: 1px 4px;
  border-radius: 3px;
  transition: background 0.15s;
}

.task-id-value:active {
  background: var(--bg-tertiary, rgba(0, 0, 0, 0.06));
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
  align-self: center;
}

.status-dot.active {
  background: #22c55e;
}

.status-dot.paused {
  background: #eab308;
}

.status-dot.completed {
  background: var(--text-muted, #999);
}

.status-dot.running {
  background: #22c55e;
  animation: task-running-pulse 1.5s ease-in-out infinite;
}

@keyframes task-running-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.4); }
  50% { opacity: 0.7; box-shadow: 0 0 6px 2px rgba(34, 197, 94, 0.2); }
}

.status-text.active {
  color: #22c55e;
}

.status-text.paused {
  color: #eab308;
}

.status-text.completed {
  color: var(--text-muted, #999);
}

.status-text.running {
  color: #22c55e;
}

.agent-icon {
  font-size: 14px;
}

.agent-name {
  font-size: 13px;
}

/* Prompt card */
.prompt-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  user-select: none;
}

.prompt-toggle {
  font-size: 12px;
  color: var(--text-muted, #999);
}

/* Expanded: use global .markdown-body styles (content.css + markdown-common.css) */
.prompt-body.markdown-body {
  /* Override .markdown-body's own overflow-y: auto — scroll is on parent .overview-scroll */
  overflow-y: visible;
  max-width: 100%;
  padding: 6px 0 0;
  margin: 0;
  background: transparent;
}

.prompt-body.collapsed {
  position: relative;
  overflow: hidden;
  max-height: 4.5em;
}

.prompt-preview-text {
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary, #666);
}

.prompt-preview-text :deep(p) {
  margin: 0 0 4px;
}

.prompt-preview-text :deep(p:last-child) {
  margin-bottom: 0;
}

.prompt-fade {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 2em;
  background: linear-gradient(transparent, var(--bg-secondary, #f5f5f5));
  pointer-events: none;
}

/* Fixed bottom action bar */
.overview-actions {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 6px 12px;
  border-top: none;
  background: transparent;
  flex-shrink: 0;
}

.actions-spacer {
  flex: 1;
}

.action-btn {
  height: 26px;
  border: none;
  border-radius: 13px;
  background: var(--bg-tertiary, rgba(0, 0, 0, 0.06));
  color: var(--text-secondary, #666);
  cursor: pointer;
  transition: all 0.15s;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 0 8px;
  flex-shrink: 0;
  font-size: 11px;
  white-space: nowrap;
}

.action-text {
  line-height: 1;
}

.action-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

@media (hover: hover) {
  .action-btn:hover:not(:disabled) {
    background: rgba(0, 0, 0, 0.1);
    color: var(--text-primary, #1a1a1a);
  }
}

.action-btn:active:not(:disabled) {
  transform: scale(0.95);
}

.action-btn.accent {
  background: var(--accent-color, #0066cc);
  color: #fff;
}

@media (hover: hover) {
  .action-btn.accent:hover:not(:disabled) {
    opacity: 0.85;
    background: var(--accent-color, #0066cc);
    color: #fff;
  }
}

.action-btn.warn {
  background: rgba(234, 179, 8, 0.15);
  color: #c9970a;
}

@media (hover: hover) {
  .action-btn.warn:hover:not(:disabled) {
    background: rgba(234, 179, 8, 0.25);
    color: #b5890a;
  }
}

.action-btn.success {
  background: rgba(34, 197, 94, 0.15);
  color: #1a9e50;
}

@media (hover: hover) {
  .action-btn.success:hover:not(:disabled) {
    background: rgba(34, 197, 94, 0.25);
    color: #168a44;
  }
}

.action-btn.danger {
  background: rgba(220, 53, 69, 0.1);
  color: #c4293c;
}

@media (hover: hover) {
  .action-btn.danger:hover:not(:disabled) {
    background: rgba(220, 53, 69, 0.18);
  }
}
</style>

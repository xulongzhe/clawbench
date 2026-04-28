<template>
  <div class="chat-message" :class="[msg.role, { 'has-metadata': msg.role === 'assistant' && msg.metadata }]">

    <!-- Collapsible content wrapper -->
    <div ref="wrapperRef" class="msg-content-wrapper" :class="{ collapsed }" :style="collapsed ? { maxHeight: store.state.chatCollapsedHeight + 'px' } : {}">
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
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
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
            <div class="chat-tool-call" :class="{ done: block.done, incomplete: block.done && !hasToolResult(block) }" :data-category="getToolDisplay(block).category" @click.stop="$emit('toggle-tool', `${index}-${bi}`)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12" class="tool-icon">
                <path :d="getToolDisplay(block).icon"/>
              </svg>
              <span class="tool-name">{{ block.name }}</span>
              <span v-if="toolCallSummary(block)" class="tool-summary">{{ toolCallSummary(block) }}</span>
              <!-- Loading: spinner -->
              <span v-if="!block.done" class="tool-spinner"></span>
              <!-- Done with result: green check -->
              <svg v-else-if="hasToolResult(block)" viewBox="0 0 24 24" fill="none" stroke="#22c55e" stroke-width="2" width="14" height="14" class="tool-check">
                <circle cx="12" cy="12" r="10"/>
                <polyline points="8 12 11 15 16 9"/>
              </svg>
              <!-- Done without result: yellow warning -->
              <svg v-else viewBox="0 0 24 24" fill="none" stroke="#f59e0b" stroke-width="2" width="14" height="14" class="tool-warn">
                <circle cx="12" cy="12" r="10"/>
                <line x1="12" y1="8" x2="12" y2="12"/>
                <line x1="12" y1="16" x2="12.01" y2="16"/>
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
          <!-- Text block: streaming uses throttled render to avoid UI freeze -->
          <div v-else-if="block.type === 'text'" v-html="getBlockHtml(bi, block)"></div>
          <!-- Schedule proposal card (inline in message) -->
          <div v-if="block.type === 'text' && blockProposals[`${msg.id}-${bi}`]"
               class="schedule-proposal-card"
               :class="{ confirmed: blockProposals[`${msg.id}-${bi}`].confirmed, failed: !blockProposals[`${msg.id}-${bi}`].confirmed }">
            <div class="proposal-header"
                 :class="blockProposals[`${msg.id}-${bi}`].confirmed ? 'confirmed' : 'failed'">
              {{ blockProposals[`${msg.id}-${bi}`].confirmed ? '📋 定时任务已创建' : '⚠️ 任务创建失败' }}
            </div>
            <div class="proposal-body">
              <div class="proposal-row"><strong>任务：</strong>{{ blockProposals[`${msg.id}-${bi}`].proposal.name }}</div>
              <div class="proposal-row"><strong>频率：</strong>{{ humanizeCron(blockProposals[`${msg.id}-${bi}`].proposal.cron_expr) }}</div>
              <div class="proposal-row"><strong>执行者：</strong>{{ getAgentIcon(blockProposals[`${msg.id}-${bi}`].proposal.agent_id) }} {{ getAgentName(blockProposals[`${msg.id}-${bi}`].proposal.agent_id) }}</div>
              <div class="proposal-row"><strong>重复：</strong>{{ repeatLabel(blockProposals[`${msg.id}-${bi}`].proposal.repeat_mode, blockProposals[`${msg.id}-${bi}`].proposal.max_runs) }}</div>
              <div class="proposal-row"><strong>提示词：</strong>{{ truncate(blockProposals[`${msg.id}-${bi}`].proposal.prompt, 80) }}</div>
            </div>
          </div>
        </template>
        <!-- Loading dots while AI is still streaming (not when cancelled) -->
        <div v-if="msg.streaming && !msg.cancelled" class="placeholder-dots"><span></span><span></span><span></span></div>
        <!-- Cancelled marker -->
        <div v-if="msg.cancelled" class="chat-cancelled-mark">已中断</div>
      </template>
      <!-- User message or legacy plain text -->
      <div v-else-if="msg.role === 'user' || msg.content" v-html="renderedContent"></div>
    </div>

    <!-- Collapse overlay + expand button -->
    <div v-if="collapsed" class="msg-collapse-overlay" @click="manuallyExpanded = true; $emit('expand', index)">
      <div class="msg-collapse-gradient"></div>
      <button class="msg-expand-btn">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <polyline points="6 9 12 15 18 9"/>
        </svg>
        展开全文
      </button>
    </div>

    <!-- Bottom bar for assistant messages -->
    <div v-if="msg.role === 'assistant' && msgText" class="chat-meta-bar">
      <span class="chat-meta-info">
        <span v-if="msg.backend">{{ msg.backend }}</span>
        <span v-if="msg.metadata?.model" class="chat-meta-sep">{{ msg.metadata.model }}</span>
        <span v-if="msg.createdAt" class="chat-meta-sep">{{ formatMessageTime(msg.createdAt) }}</span>
      </span>
      <div class="chat-meta-actions">
        <button v-if="!msg.streaming" ref="speakBtnRef" class="chat-info-btn chat-speak-btn" :class="{ active: autoSpeech.isActive(msgText), loading: autoSpeech.isGeneratingText(msgText) }" @click.stop="handleSpeak">
          <!-- Generating state -->
          <template v-if="autoSpeech.isGeneratingText(msgText)">
            <svg class="speak-spinner" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <path d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM12 6v6l4 2"/>
            </svg>
            <span>总结中</span>
          </template>
          <!-- Playing state -->
          <template v-else-if="autoSpeech.isPlayingAudio(msgText)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/>
            </svg>
            <span>朗读中</span>
          </template>
          <!-- Default state -->
          <template v-else>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
              <path d="M15.54 8.46a5 5 0 0 1 0 7.07"/>
              <path d="M19.07 4.93a10 10 0 0 1 0 14.14"/>
            </svg>
            <span>朗读</span>
          </template>
        </button>
        <button v-if="msg.metadata" class="chat-info-btn" @click="$emit('show-metadata', msg)" title="查看详情">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="16" x2="12" y2="12"/>
            <line x1="12" y1="8" x2="12.01" y2="8"/>
          </svg>
        </button>
      </div>
    </div>
    <!-- Bottom bar for user messages -->
    <div v-if="msg.role === 'user'" class="chat-meta-bar chat-meta-bar-user">
      <span class="chat-meta-info">
        <span v-if="msg.createdAt">{{ formatMessageTime(msg.createdAt) }}</span>
      </span>
      <button class="chat-info-btn chat-info-btn-user" @click="$emit('show-metadata', msg)" title="查看详情">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="16" x2="12" y2="12"/>
          <line x1="12" y1="8" x2="12.01" y2="8"/>
        </svg>
      </button>
    </div>

    <!-- TTS Popover: shows AI-summarized text being read aloud -->
    <!-- TtsPopover removed - status now shown in meta bar -->
  </div>
</template>

<script setup>
import { ref, inject, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { baseName } from '@/utils/helpers.ts'
import { store } from '@/stores/app.ts'
import { useAutoSpeech } from '@/composables/useAutoSpeech.ts'

// Tool display configuration: icon SVG paths + category for color
const TOOL_DISPLAY = {
  'Read':          { icon: 'M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7z M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z', category: 'file' },
  'Write':         { icon: 'M17 3a2.83 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z', category: 'file' },
  'Edit':          { icon: 'M12 3v18M3 12h18', category: 'file' },
  'Bash':          { icon: 'M4 17l6-6-6-6M12 19h8', category: 'bash' },
  'WebSearch':     { icon: 'M11 3a8 8 0 1 0 0 16 8 8 0 0 0 0-16zM21 21l-4.35-4.35', category: 'search' },
  'WebFetch':      { icon: 'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z', category: 'search' },
  'TaskCreate':    { icon: 'M12 5v14M5 12h14', category: 'task' },
  'TaskUpdate':    { icon: 'M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7 M18.5 2.5a2.12 2.12 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z', category: 'task' },
  'TaskList':      { icon: 'M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01', category: 'task' },
  'TaskGet':       { icon: 'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM12 6a6 6 0 1 0 0 12 6 6 0 0 0 0-12zM12 10a2 2 0 1 0 0 4 2 2 0 0 0 0-4z', category: 'task' },
  'EnterPlanMode': { icon: 'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM16.24 7.76l-2.12 6.36-6.36 2.12 2.12-6.36 6.36-2.12z', category: 'plan' },
  'ExitPlanMode':  { icon: 'M22 11.08V12a10 10 0 1 1-5.93-9.14M22 4L12 14.01l-3-3', category: 'plan' },
  'Agent':         { icon: 'M12 8V4H8 M12 8V4h4 M8 4a4 4 0 0 0-4 4v2 M16 4a4 4 0 0 1 4 4v2 M9 16h6 M10 20a2 2 0 1 0 0-4 2 2 0 0 0 0 4z', category: 'agent' },
  'SendMessage':   { icon: 'M22 2l-7 20-4-9-9-4 20-7z', category: 'agent' },
  'Skill':         { icon: 'M12 2l2.4 7.2L22 12l-7.6 2.8L12 22l-2.4-7.2L2 12l7.6-2.8z', category: 'skill' },
}
const FALLBACK_TOOL_DISPLAY = { icon: 'M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z', category: 'fallback' }

function getToolDisplay(block) {
  const name = (block.name || '').toLowerCase()
  const entry = Object.entries(TOOL_DISPLAY).find(([k]) => k.toLowerCase() === name)
  return entry ? entry[1] : FALLBACK_TOOL_DISPLAY
}

// Check if a tool_use block has meaningful result data
function hasToolResult(block) {
  return block.input && typeof block.input === 'object' && Object.keys(block.input).length > 0
}

const props = defineProps({
  msg: Object,
  index: Number,
  expandedTools: Object,
  blockProposals: Object,
  agents: Array,
  renderedContent: String,
  shouldCollapse: Boolean,
})

const emit = defineEmits(['toggle-tool', 'show-metadata', 'file-tag-click', 'expand'])

const autoSpeech = useAutoSpeech()
const thinkingExpanded = ref({})
const wrapperRef = ref(null)
const overflows = ref(false)
const manuallyExpanded = ref(false)
const speakBtnRef = ref(null)

// Extract text content from message blocks for TTS
const msgText = computed(() => {
  if (props.msg?.role !== 'assistant') return ''
  const blocks = props.msg?.blocks || []
  return blocks.filter(b => b.type === 'text').map(b => b.text || '').join('\n').trim()
})

// Handle speak button click: play or stop (no popover)
function handleSpeak() {
  if (autoSpeech.isActive(msgText.value)) {
    autoSpeech.stopAudio()
  } else if (msgText.value) {
    autoSpeech.speakText(msgText.value)
  }
}

function checkOverflow() {
  if (!wrapperRef.value) return
  overflows.value = wrapperRef.value.scrollHeight > store.state.chatCollapsedHeight
}

// Check overflow after mount and when content changes
onMounted(() => nextTick(checkOverflow))
watch(() => props.renderedContent, () => nextTick(checkOverflow))
watch(() => props.msg?.blocks?.length, () => nextTick(checkOverflow))
watch(() => props.msg?.streaming, () => nextTick(checkOverflow))

const collapsed = computed(() => {
  if (!props.shouldCollapse) return false
  if (props.msg?.streaming) return false
  if (manuallyExpanded.value) return false
  return overflows.value
})

const chatRender = inject('chatRender', {})
const chatSession = inject('chatSession', {})

const { renderTextBlock, formatMessageTime, toolCallSummary, formatToolInput, humanizeCron, repeatLabel, truncate, hasImagesInContent } = chatRender
const { getAgentIcon, getAgentName } = chatSession

// ── Throttled streaming render ──
// During streaming, block.text changes on every SSE event (~50ms).
// Running full markdown→KaTeX→DOMPurify on each change freezes the UI.
// Instead, cache rendered HTML in a ref and throttle updates to ~300ms.
const blockHtmlCache = ref({})          // { [blockIdx]: html }
let _throttleTimer = null
let _throttlePending = false
const THROTTLE_MS = 300

function flushBlockHtml() {
  _throttleTimer = null
  if (!_throttlePending) return
  _throttlePending = false
  const newCache = {}
  for (let i = 0; i < (props.msg?.blocks?.length || 0); i++) {
    const block = props.msg.blocks[i]
    if (block.type === 'text') {
      newCache[i] = renderTextBlock(block.text, props.msg.id, i)
    }
  }
  blockHtmlCache.value = newCache
}

function getBlockHtml(bi, block) {
  // Non-streaming: render immediately (result is stable, Vue skips DOM update if same)
  if (!props.msg?.streaming) {
    return renderTextBlock(block.text, props.msg.id, bi)
  }
  // Streaming: serve cached HTML; schedule a throttled refresh
  if (blockHtmlCache.value[bi] !== undefined) {
    // Kick off throttled update if not already pending
    if (!_throttleTimer) {
      // First change or throttle window elapsed — update immediately
      const newCache = { ...blockHtmlCache.value }
      newCache[bi] = renderTextBlock(block.text, props.msg.id, bi)
      blockHtmlCache.value = newCache
      _throttleTimer = setTimeout(flushBlockHtml, THROTTLE_MS)
    } else {
      _throttlePending = true
    }
    return blockHtmlCache.value[bi]
  }
  // Cache miss (first render of this block) — render now
  const html = renderTextBlock(block.text, props.msg.id, bi)
  blockHtmlCache.value = { ...blockHtmlCache.value, [bi]: html }
  return html
}

// When streaming ends, clear throttle and force full re-render
watch(() => props.msg?.streaming, (streaming, wasStreaming) => {
  if (wasStreaming && !streaming) {
    if (_throttleTimer) { clearTimeout(_throttleTimer); _throttleTimer = null }
    _throttlePending = false
    blockHtmlCache.value = {}  // clear cache so next getBlockHtml renders fresh
  }
})

function toggleThinking(key) {
  thinkingExpanded.value = { ...thinkingExpanded.value, [key]: !thinkingExpanded.value[key] }
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

onUnmounted(() => {
  if (_throttleTimer) { clearTimeout(_throttleTimer); _throttleTimer = null }
})
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
  padding: 3px 8px;
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
  --tool-accent: var(--text-muted);
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  color: var(--text-secondary);
  background: color-mix(in srgb, var(--tool-accent) 6%, var(--bg-secondary));
  border: 1px solid color-mix(in srgb, var(--tool-accent) 15%, var(--border-color));
  padding: 3px 8px;
  border-radius: 4px;
  cursor: pointer;
  width: 100%;
  margin-top: 4px;
  overflow: hidden;
}

/* Category accent colors */
.chat-tool-call[data-category="file"]     { --tool-accent: var(--accent-color); }
.chat-tool-call[data-category="bash"]     { --tool-accent: #10b981; }
.chat-tool-call[data-category="search"]   { --tool-accent: #8b5cf6; }
.chat-tool-call[data-category="task"]     { --tool-accent: #f59e0b; }
.chat-tool-call[data-category="plan"]     { --tool-accent: var(--accent-color); }
.chat-tool-call[data-category="agent"]    { --tool-accent: #ec4899; }
.chat-tool-call[data-category="skill"]    { --tool-accent: #06b6d4; }
.chat-tool-call[data-category="fallback"] { --tool-accent: var(--text-muted); }

.chat-tool-call:hover {
  background: color-mix(in srgb, var(--tool-accent) 12%, var(--bg-secondary));
}

.chat-tool-call .tool-icon {
  color: var(--tool-accent);
  opacity: 0.8;
  flex-shrink: 0;
}

.chat-tool-call .tool-name {
  font-weight: 600;
  color: var(--tool-accent);
  font-size: 11px;
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
  flex-shrink: 0;
  margin-left: auto;
}

.chat-tool-call .tool-warn {
  flex-shrink: 0;
  margin-left: auto;
}

/* Incomplete tool call: session ended before result arrived */
.chat-tool-call.incomplete {
  --tool-accent: #f59e0b;
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
  border-top-color: var(--tool-accent);
  border-radius: 50%;
  animation: tool-spin 0.6s linear infinite;
  flex-shrink: 0;
  margin-left: auto;
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

.schedule-proposal-card.failed {
  border-color: #f44336;
  opacity: 0.9;
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

.proposal-header.failed {
  background: #f44336;
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

/* ── Collapse styles ── */
.msg-content-wrapper {
  position: relative;
  transition: max-height 0.3s ease;
}

.msg-content-wrapper.collapsed {
  overflow: hidden;
}

.msg-collapse-overlay {
  position: relative;
  margin-top: -40px;
  padding-top: 40px;
  cursor: pointer;
}

.msg-collapse-gradient {
  position: absolute;
  inset: 0;
  background: linear-gradient(to bottom, transparent 0%, var(--bg-tertiary) 80%);
  pointer-events: none;
}

.chat-message.user .msg-collapse-gradient {
  background: linear-gradient(to bottom, transparent 0%, var(--user-msg-color) 80%);
}

.msg-expand-btn {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  width: 100%;
  padding: 6px 0;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: color 0.2s;
}

.msg-expand-btn:hover {
  color: var(--accent-color, #0066cc);
}

.msg-expand-btn svg {
  flex-shrink: 0;
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
    min-width: 22px;
    height: 22px;
    padding: 0 6px;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    opacity: 0.5;
    transition: opacity 0.2s, background 0.2s;
    font-size: 11px;
}

.chat-info-btn:hover {
    opacity: 1;
    background: var(--bg-tertiary);
}

.chat-info-btn svg {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
}

.chat-info-btn span {
    white-space: nowrap;
}

/* Speak button specific styles */
.chat-speak-btn {
    min-width: auto;
    padding: 0 8px;
}

.chat-speak-btn.active {
    opacity: 1;
    color: var(--accent-color, #0066cc);
}

.chat-speak-btn.active:hover {
    background: color-mix(in srgb, var(--accent-color, #0066cc) 10%, transparent);
}

/* Meta bar action buttons container */
.chat-meta-actions {
    display: flex;
    align-items: center;
    gap: 2px;
}

/* Speak button active state */
.chat-speak-btn.active {
    opacity: 1;
    color: var(--accent-color, #0066cc);
}

.chat-speak-btn.active:hover {
    background: color-mix(in srgb, var(--accent-color, #0066cc) 10%, transparent);
}

/* Speak button loading spinner animation */
.chat-speak-btn.loading .speak-spinner {
    animation: speak-spin 1s linear infinite;
}

/* Speak button loading spinner animation */
.chat-speak-btn.loading .speak-spinner {
    animation: speak-spin 1s linear infinite;
}

@keyframes speak-spin {
    to { transform: rotate(360deg); }
}

/* User message meta bar */
.chat-meta-bar-user {
    opacity: 0.6;
    transition: opacity 0.2s;
}

.chat-meta-bar-user:hover {
    opacity: 1;
}

.chat-info-btn-user {
    color: rgba(255, 255, 255, 0.7);
}

.chat-info-btn-user:hover {
    color: rgba(255, 255, 255, 0.9);
    background: rgba(255, 255, 255, 0.1);
}

.chat-meta-bar-user .chat-meta-info {
    color: rgba(255, 255, 255, 0.7);
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
    min-width: 0;
    overflow-wrap: break-word;
}

.chat-message.assistant pre {
    padding: 10px;
    margin: 6px 0;
    border-radius: var(--radius-sm);
    overflow-x: auto;
    max-width: 100%;
    box-sizing: border-box;
    word-break: normal;
    word-wrap: normal;
    white-space: pre;
}

.chat-message.assistant pre code {
    white-space: pre;
    word-break: normal;
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

.chat-message.assistant a {
    word-break: break-all;
    overflow-wrap: break-word;
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

/* Dark mode tool accent adjustments */
:root[data-theme="dark"] .chat-tool-call[data-category="bash"]   { --tool-accent: #34d399; }
:root[data-theme="dark"] .chat-tool-call[data-category="search"] { --tool-accent: #a78bfa; }
:root[data-theme="dark"] .chat-tool-call[data-category="task"]   { --tool-accent: #fbbf24; }
:root[data-theme="dark"] .chat-tool-call[data-category="agent"]  { --tool-accent: #f472b6; }
:root[data-theme="dark"] .chat-tool-call[data-category="skill"]  { --tool-accent: #22d3ee; }
</style>

<template>
  <div class="file-header-bar">
    <span class="lang-badge" :style="badgeStyle">{{ badgeLabel }}</span>
    <div class="file-name-wrap">
      <span class="file-path-hint" style="cursor:pointer" @click="$emit('showDetails')" :title="file.name">{{ file.name }}</span>
    </div>
    <div class="header-actions">
      <a :href="'/api/local-file/' + encodeURIComponent(file.path)" :download="file.name" class="image-download-btn" title="下载">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
          <polyline points="7,10 12,15 17,10"/>
          <line x1="12" y1="15" x2="12" y2="3"/>
        </svg>
      </a>
      <button class="file-header-btn" @click="confirmDelete" title="删除">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
          <polyline points="3,6 5,6 21,6"/>
          <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>
          <path d="M10 11v6M14 11v6"/>
          <path d="M9 6V4h6v2"/>
        </svg>
      </button>

      <!-- Markdown toggle button (code icon) -->
      <button v-if="isMarkdown" class="file-header-btn" @click="$emit('toggleView')" title="切换视图">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
          <polyline points="16 18 22 12 16 6"/>
          <polyline points="8 6 2 12 8 18"/>
        </svg>
      </button>

      <!-- Git history button -->
      <button
        class="file-header-btn"
        @click="$emit('openGitHistory')"
        title="代码历史"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
            <circle cx="12" cy="12" r="10"/>
            <polyline points="12 6 12 12 16 14"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { getFileType } from '@/utils/helpers.ts'

const props = defineProps({
    file: Object,
    viewMode: String,
})
const emit = defineEmits(['delete', 'toggleView', 'showDetails', 'openGitHistory'])

const fileType = computed(() => props.file ? getFileType(props.file.name) : null)
const isMarkdown = computed(() => fileType.value?.isMarkdown || false)

const badgeLabel = computed(() => {
    if (!props.file) return ''
    return getFileType(props.file.name)?.label || 'TXT'
})

const badgeColor = computed(() => {
    if (!props.file) return '#8b8b8b'
    return getFileType(props.file.name)?.color || '#8b8b8b'
})

const badgeStyle = computed(() => ({
    background: badgeColor.value + '22',
    color: badgeColor.value,
    border: `1px solid ${badgeColor.value}44`,
}))

function confirmDelete() {
    if (!confirm(`确定要删除"${props.file?.name}"吗？`)) return
    emit('delete', props.file?.path)
}
</script>

<style scoped>
.file-header-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-left: none;
    border-right: none;
    border-radius: 0;
    z-index: 10;
    border-bottom: none;
    font-size: 13px;
    position: sticky;
    top: 0;
    left: 0;
    min-width: 0;
}

.file-name-wrap {
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
}

.lang-badge {
    font-size: 11px;
    font-weight: 700;
    padding: 3px 8px;
    border-radius: 5px;
    flex-shrink: 0;
    font-family: monospace;
    letter-spacing: 0.5px;
}

.file-path-hint {
    flex: 0 0 auto;
    max-width: 100%;
    color: var(--text-muted);
    font-family: monospace;
    font-size: 12px;
    overflow-x: auto;
    white-space: nowrap;
    cursor: pointer;
    transition: color 0.15s;
    scrollbar-width: none;
}
.file-path-hint::-webkit-scrollbar {
    display: none;
}
.file-path-hint:hover {
    color: var(--accent-color);
}
.file-path-hint.copied {
    color: #22c55e;
}

.header-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: auto;
    flex-shrink: 0;
}

.file-header-btn {
    margin-left: auto;
    padding: 4px 12px;
    border: 1px solid var(--border-color);
    background: var(--bg-tertiary);
    border-radius: var(--radius-sm);
    font-size: 12px;
    cursor: pointer;
    color: var(--text-primary);
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 4px;
}
.file-header-btn:hover {
    background: var(--accent-color);
    color: #fff;
    border-color: var(--accent-color);
}
.file-header-btn svg {
    width: 13px;
    height: 13px;
}
.file-header-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
    pointer-events: none;
}
.file-header-btn:disabled:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
    border-color: var(--border-color);
}

.image-download-btn {
    padding: 4px 8px;
    border: 1px solid var(--border-color);
    background: var(--bg-tertiary);
    border-radius: var(--radius-sm);
    color: var(--text-primary);
    text-decoration: none;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    transition: all 0.15s ease;
    flex-shrink: 0;
}
.image-download-btn:hover {
    background: var(--accent-color);
    color: #fff;
    border-color: var(--accent-color);
}
.image-download-btn svg {
    width: 13px;
    height: 13px;
}
</style>

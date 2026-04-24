<template>
  <BottomSheet :open="open" @close="handleClose">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <circle cx="11" cy="11" r="8"/>
        <line x1="21" y1="21" x2="16.65" y2="16.65"/>
      </svg>
      <span class="bs-header-title">搜索文件</span>
      <div v-if="file?.path" class="bs-header-description">
        <span class="bs-header-description-inner" :title="file.path">
          {{ file.path }}
        </span>
      </div>
      <button class="bs-close" @click.stop="handleClose" title="关闭">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    </template>

    <div class="search-body">
      <div class="search-input-row">
        <input
          ref="inputRef"
          v-model="query"
          class="search-input"
          placeholder="输入关键字搜索…"
          type="text"
          @keydown.enter="jumpToFirst"
          @dblclick="query = ''"
        />
      </div>

      <div class="search-content">
        <div v-if="!file?.content" class="search-empty">无文件内容</div>
        <div v-else-if="!query.trim()" class="search-empty">输入关键字搜索</div>
        <div v-else-if="results.length === 0" class="search-empty">未找到 "{{ query }}"</div>
        <div v-else class="search-results">
          <div class="search-results-count">{{ results.length }} 处匹配</div>
          <div
            v-for="r in results"
            :key="r.line"
            class="search-result-item"
            @click="jumpTo(r.line)"
          >
            <span class="search-result-lnum">{{ r.line }}</span>
            <span class="search-result-text" v-html="r.highlighted" />
          </div>
        </div>
      </div>
    </div>
  </BottomSheet>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import BottomSheet from './BottomSheet.vue'

const props = defineProps({
  file: Object,
  open: Boolean,
})
const emit = defineEmits(['close', 'jump'])

const query = ref('')
const inputRef = ref(null)

watch(() => props.open, async (val) => {
  if (val) {
    await nextTick()
    inputRef.value?.focus()
  }
})

// Clear query only when the file changes
watch(() => props.file?.path, () => {
  query.value = ''
})

function handleClose() {
  emit('close')
}

function escapeHtml(text) {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function highlightText(text, q) {
  if (!q) return escapeHtml(text)
  const escaped = escapeHtml(text)
  const re = new RegExp(q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g')
  return escaped.replace(re, '<mark>$&</mark>')
}

const results = computed(() => {
  const content = props.file?.content
  if (!content || !query.value.trim()) return []
  const q = query.value.trim()
  const lines = content.split('\n')
  const out = []
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].includes(q)) {
      out.push({
        line: i + 1,
        text: lines[i],
        highlighted: highlightText(lines[i], q),
      })
    }
  }
  return out
})

function jumpTo(line) {
  emit('jump', line)
  emit('close')
}

function jumpToFirst() {
  if (results.value.length > 0) {
    jumpTo(results.value[0].line)
  }
}
</script>

<style scoped>
.search-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #212529);
}

.search-body {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.search-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  background: var(--bg-secondary, #f8f9fa);
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  border: 1px solid var(--border-color, #dee2e6);
  border-radius: 6px;
  outline: none;
  padding: 6px 10px;
  font-size: 13px;
  background: var(--bg-primary, #fff);
  color: var(--text-primary, #212529);
}

.search-input:focus {
  border-color: var(--accent-color, #4a90d9);
}

.search-input::placeholder {
  color: var(--text-muted, #999);
}

.search-content {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.search-empty {
  padding: 24px;
  text-align: center;
  color: var(--text-muted, #999);
  font-size: 13px;
}

.search-results {
  flex: 1;
  overflow-y: auto;
}

.search-results-count {
  padding: 6px 14px;
  font-size: 11px;
  color: var(--text-muted, #999);
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  background: var(--bg-secondary, #f8f9fa);
  flex-shrink: 0;
}

.search-result-item {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 5px 14px;
  cursor: pointer;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
  border-bottom: 1px solid var(--border-color, #f0f0f0);
  transition: background 0.1s;
}

.search-result-item:hover {
  background: var(--bg-secondary, #f8f9fa);
}

.search-result-lnum {
  color: var(--text-muted, #999);
  min-width: 32px;
  text-align: right;
  flex-shrink: 0;
  user-select: none;
}

.search-result-text {
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--text-primary, #212529);
}

.search-result-text :deep(mark) {
  background: rgba(255, 230, 0, 0.5);
  color: inherit;
  border-radius: 2px;
  padding: 0 1px;
}
</style>

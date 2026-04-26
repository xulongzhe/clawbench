<template>
  <BottomSheet :open="open" compact @close="handleClose">
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
            v-for="(r, idx) in results"
            :key="viewMode === 'rendered' ? idx : r.line"
            class="search-result-item"
            @click="jumpTo(r)"
          >
            <span class="search-result-lnum">{{ viewMode === 'rendered' ? idx + 1 : r.line }}</span>
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
  viewMode: String, // 'rendered' | 'raw' | undefined
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

// Clear query when the file changes
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

// --- Rendered mode: search DOM text ---

const BLOCK_TAGS = new Set(['P', 'LI', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'BLOCKQUOTE', 'TD', 'TH', 'DT', 'DD', 'PRE', 'FIGCAPTION', 'DIV'])

function findBlockAncestor(node) {
  let el = node.parentElement
  while (el) {
    if (BLOCK_TAGS.has(el.tagName) && el.closest('.markdown-body')) {
      return el
    }
    el = el.parentElement
  }
  return node.parentElement
}

function searchRenderedContent(q) {
  const container = document.querySelector('.markdown-body')
  if (!container) return []

  const out = []
  const seen = new Set()
  const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT, null)

  while (walker.nextNode()) {
    const textNode = walker.currentNode
    const text = textNode.textContent
    if (!text || !text.includes(q)) continue

    const block = findBlockAncestor(textNode)
    if (!block || seen.has(block)) continue
    seen.add(block)

    const fullText = block.textContent.trim()
    const idx = fullText.indexOf(q)
    const start = Math.max(0, idx - 30)
    const end = Math.min(fullText.length, idx + q.length + 30)
    const snippet = (start > 0 ? '...' : '') + fullText.slice(start, end) + (end < fullText.length ? '...' : '')

    out.push({
      line: out.length,
      text: snippet,
      highlighted: highlightText(snippet, q),
      _blockEl: block,
    })
  }
  return out
}

// --- Raw mode: search source lines ---

function searchRawContent(q) {
  const content = props.file?.content
  if (!content) return []
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
}

const results = computed(() => {
  if (!props.file?.content || !query.value.trim()) return []
  const q = query.value.trim()
  if (props.viewMode === 'rendered') {
    return searchRenderedContent(q)
  }
  return searchRawContent(q)
})

function jumpTo(result) {
  if (props.viewMode === 'rendered') {
    scrollToRenderedMatch(result)
    emit('close')
  } else {
    emit('jump', result.line)
    emit('close')
  }
}

function scrollToRenderedMatch(result) {
  const block = result._blockEl
  if (!block) return
  block.scrollIntoView({ behavior: 'smooth', block: 'center' })
  block.classList.add('line-flash')
  block.addEventListener('animationend', () => block.classList.remove('line-flash'), { once: true })
}

function jumpToFirst() {
  if (results.value.length > 0) {
    jumpTo(results.value[0])
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
  flex-shrink: 0;
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

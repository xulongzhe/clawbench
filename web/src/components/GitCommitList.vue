<template>
  <div class="drilldown-page">
    <div v-if="!(commits.length === 0 && untracked) && isGit" class="drilldown-header">
      <div class="drilldown-title">
        <span v-if="commits.length > 0" class="drilldown-count">
          {{ searchLoading ? '加载中…' : filteredCommits.length + (hasMore && !commitSearch ? '+' : '') + ' 条' + countLabel }}
        </span>
        <span v-else-if="!isGit" class="drilldown-count">未初始化</span>
        <span v-else-if="!untracked" class="drilldown-count">加载中…</span>
      </div>
      <SearchInput v-if="commits.length > 0" v-model="commitSearch" :placeholder="searchPlaceholder" />
    </div>
    <div class="drilldown-body" ref="bodyRef">
      <div v-if="loading" class="git-history-loading">
        <div class="spinner" style="width:24px;height:24px;border-width:2px;" />
      </div>
      <div v-else-if="searchLoading" class="git-history-loading">
        <div class="spinner" style="width:24px;height:24px;border-width:2px;" />
      </div>
      <div v-else-if="error" class="git-history-error">{{ error }}</div>
      <div v-else-if="!isGit" class="git-history-empty">
        <div class="init-git-prompt">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="40" height="40" style="color:#ccc;margin-bottom:12px;">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="8" x2="12" y2="16"/>
            <line x1="8" y1="12" x2="16" y2="12"/>
          </svg>
          <div style="font-size:14px;color:var(--text-muted,#999);margin-bottom:12px;">尚未初始化 Git 仓库</div>
          <button class="init-git-btn" @click.stop="$emit('init-git')" :disabled="initLoading">
            <span v-if="initLoading" class="spinner" style="width:14px;height:14px;border-width:2px;" />
            <span v-else>初始化 Git</span>
          </button>
        </div>
      </div>
      <div v-else-if="commits.length === 0 && untracked" class="git-history-empty">
        <div class="empty-state-card">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="36" height="36" style="color:var(--text-muted);">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <line x1="9" y1="15" x2="15" y2="15"/>
          </svg>
          <div class="empty-state-title">此文件未被 Git 跟踪</div>
          <div class="empty-state-desc">文件尚未纳入版本控制，无历史记录</div>
          <div class="empty-state-hint">
            <code>git add {{ '&lt;文件名&gt;' }}</code> 可将其添加到跟踪
          </div>
        </div>
      </div>
      <div v-else-if="commits.length === 0" class="git-history-empty">暂无提交记录</div>
      <div v-else class="commit-list-container">
        <!-- Graph SVG - hidden during search because filtering breaks lane continuity -->
        <GitGraph
          v-if="!isSearching"
          class="commit-list-graph"
          :commits="filteredCommits"
          :row-height="46"
          :collapsed="graphCollapsed"
          @update:collapsed="graphCollapsed = $event"
        />
        <!-- Commit rows -->
        <div class="commit-list-content" ref="contentRef" @touchstart="onTouchStart" @touchend="onTouchEnd">
          <div
            v-for="c in filteredCommits"
            :key="c.sha"
            class="drilldown-item"
            @click="$emit('select', c)"
          >
            <div class="git-commit-info">
              <div class="git-commit-msg">{{ c.msg }}</div>
              <div class="git-commit-meta">
                <span>{{ formatDate(c.date) }}</span>
                <span v-if="c.author"> · {{ c.author }}</span>
              </div>
            </div>
          </div>
          <div ref="listRef">
            <div v-if="hasMore && loadingMore" class="git-load-more">
              <div class="spinner" style="width:20px;height:20px;border-width:2px;" />
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import GitGraph from './GitGraph.vue'
import SearchInput from './SearchInput.vue'

const props = defineProps({
  commits: { type: Array, default: () => [] },
  isGit: { type: Boolean, default: true },
  hasMore: { type: Boolean, default: false },
  loadingMore: { type: Boolean, default: false },
  searchLoading: { type: Boolean, default: false },
  initLoading: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  untracked: { type: Boolean, default: false },
  countLabel: { type: String, default: '提交记录' },
  searchPlaceholder: { type: String, default: '搜索提交信息…' },
})

const emit = defineEmits(['select', 'search', 'load-more', 'init-git'])

const commitSearch = ref('')
const listRef = ref(null)
const contentRef = ref(null)
const observer = ref(null)
const graphCollapsed = ref(false)

// Touch swipe handling for graph toggle
const SWIPE_THRESHOLD = 50
let touchStartX = 0
let touchStartY = 0
let touchStartTime = 0

function onTouchStart(e) {
  touchStartX = e.touches[0].clientX
  touchStartY = e.touches[0].clientY
  touchStartTime = Date.now()
}

function onTouchEnd(e) {
  const dx = e.changedTouches[0].clientX - touchStartX
  const dy = e.changedTouches[0].clientY - touchStartY
  const dt = Date.now() - touchStartTime
  // Only trigger on primarily horizontal swipes, fast enough
  if (dt > 500 || Math.abs(dy) > Math.abs(dx) || Math.abs(dx) < SWIPE_THRESHOLD) return
  if (dx < 0) {
    // Swipe left: hide graph
    graphCollapsed.value = true
  } else {
    // Swipe right: show graph
    graphCollapsed.value = false
  }
}

const isSearching = computed(() => commitSearch.value.trim().length > 0)

const filteredCommits = computed(() => {
  const q = commitSearch.value.trim().toLowerCase()
  if (!q) return props.commits
  return props.commits.filter(c => c.msg.toLowerCase().includes(q))
})

function formatDate(dateStr) {
  if (!dateStr) return ''
  try {
    const d = new Date(dateStr)
    return d.toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return dateStr
  }
}

watch(commitSearch, (q) => {
  emit('search', q)
})

function setupObserver() {
  observer.value = new IntersectionObserver((entries) => {
    if (entries[0].isIntersecting && props.hasMore && !props.loadingMore) {
      emit('load-more')
    }
  }, { threshold: 0.1, rootMargin: '100px' })
}

function observeList() {
  if (observer.value && listRef.value) {
    observer.value.observe(listRef.value)
  }
}

function unobserveList() {
  if (observer.value) {
    observer.value.disconnect()
  }
}

onMounted(() => {
  setupObserver()
})

onUnmounted(() => {
  unobserveList()
})

defineExpose({ observeList, unobserveList, commitSearch })
</script>

<style scoped>
.drilldown-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.drilldown-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 14px;
  height: var(--header-height, 44px);
  border-bottom: 1px solid var(--border-color, #dee2e6);
  background: var(--bg-secondary, #f8f9fa);
  flex-shrink: 0;
  gap: 8px;
}

.drilldown-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #212529);
  overflow: hidden;
  flex: 1;
  min-width: 0;
}

.drilldown-count {
  font-size: 10px;
  font-weight: 700;
  background: var(--bg-tertiary, #e9ecef);
  color: var(--text-muted, #999);
  padding: 1px 6px;
  border-radius: 10px;
  flex-shrink: 0;
}

.drilldown-body {
  flex: 1;
  overflow-y: auto;
}

/* ─── Commit list: Graph + Info ─────────────────────────────────────────── */

.commit-list-container {
  position: relative;
  display: flex;
}

.commit-list-graph {
  position: sticky;
  left: 0;
  z-index: 1;
  flex-shrink: 0;
}

.commit-list-content {
  flex: 1;
  min-width: 0;
}

.drilldown-item {
  display: flex;
  align-items: center;
  padding: 11px 14px;
  cursor: pointer;
  transition: background 0.15s;
  border-bottom: 1px solid var(--border-color, #dee2e6);
  height: 46px;
  box-sizing: border-box;
}

.drilldown-item:hover {
  background: var(--bg-secondary, #f8f9fa);
}

.drilldown-item:active {
  background: var(--bg-tertiary, #e9ecef);
}

.git-commit-info {
  flex: 1;
  min-width: 0;
}

.git-commit-msg {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: inherit;
}

.git-commit-meta {
  font-size: 11px;
  color: var(--text-muted, #999);
  margin-top: 2px;
}

.git-load-more {
  padding: 20px 14px;
  display: flex;
  justify-content: center;
  min-height: 60px;
}

.git-history-loading {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.git-history-error,
.git-history-empty {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted, #999);
    font-size: 14px;
}

.empty-state-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 32px 24px;
    gap: 8px;
}

.empty-state-title {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary);
}

.empty-state-desc {
    font-size: 13px;
    color: var(--text-muted);
}

.empty-state-hint {
    font-size: 12px;
    color: var(--text-muted);
    margin-top: 4px;
}

.empty-state-hint code {
    background: var(--bg-tertiary);
    padding: 2px 6px;
    border-radius: 4px;
    font-family: monospace;
    font-size: 11px;
}

.init-git-prompt {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
}

.init-git-btn {
  padding: 8px 20px;
  border: 1px solid var(--accent-color, #4a90d9);
  border-radius: 6px;
  background: var(--accent-color, #4a90d9);
  color: #fff;
  font-size: 14px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  transition: opacity 0.15s;
}

.init-git-btn:hover:not(:disabled) {
  opacity: 0.85;
}

.init-git-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>

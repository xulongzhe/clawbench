<template>
  <div class="drilldown-page">
    <div class="drilldown-header">
      <div class="drilldown-title">
        <span v-if="commits.length > 0" class="drilldown-count">
          {{ searchLoading ? '加载中…' : filteredCommits.length + (hasMore && !commitSearch ? '+' : '') + ' 条' + countLabel }}
        </span>
        <span v-else-if="!isGit" class="drilldown-count">未初始化</span>
        <span v-else class="drilldown-count">加载中…</span>
      </div>
      <input
        v-if="commits.length > 0"
        v-model="commitSearch"
        class="drilldown-search"
        :placeholder="searchPlaceholder"
        type="text"
      />
    </div>
    <div class="drilldown-body">
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
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="40" height="40" style="color:#ccc;margin-bottom:12px;">
          <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/>
        </svg>
        <div style="font-size:14px;color:var(--text-muted,#999);margin-bottom:4px;">此文件未被 Git 跟踪</div>
        <div style="font-size:12px;color:var(--text-muted,#aaa);">使用 <code style="background:#f0f0f0;padding:1px 4px;border-radius:3px;">git add &lt;文件名&gt;</code> 将其添加到跟踪</div>
      </div>
      <div v-else-if="commits.length === 0" class="git-history-empty">暂无提交记录</div>
      <div v-else class="drilldown-list">
        <div
          v-for="c in filteredCommits"
          :key="c.sha"
          class="drilldown-item"
          @click="$emit('select', c)"
        >
          <div :class="c.isWT ? 'git-commit-dot-wt' : 'git-commit-dot'" />
          <div class="git-commit-info">
            <div class="git-commit-msg">{{ c.msg }}</div>
            <div class="git-commit-meta">
              <span>{{ formatDate(c.date) }}</span>
              <span v-if="c.author"> · {{ c.author }}</span>
            </div>
          </div>
          <svg class="drilldown-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <polyline points="9 18 15 12 9 6"/>
          </svg>
        </div>
        <div ref="listRef">
          <div v-if="hasMore && loadingMore" class="git-load-more">
            <div class="spinner" style="width:20px;height:20px;border-width:2px;" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'

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
const observer = ref(null)

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

.drilldown-search {
  padding: 4px 8px;
  border: 1px solid var(--border-color, #dee2e6);
  border-radius: 6px;
  background: var(--bg-primary, #ffffff);
  color: var(--text-primary, #212529);
  font-size: 12px;
  outline: none;
  width: 140px;
  flex-shrink: 0;
}

.drilldown-search:focus {
  border-color: var(--accent-color, #4a90d9);
}

.drilldown-search::placeholder {
  color: var(--text-muted, #999);
}

.drilldown-body {
  flex: 1;
  overflow-y: auto;
}

.drilldown-list {
  padding: 6px 0;
}

.drilldown-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 14px;
  cursor: pointer;
  transition: background 0.15s;
  border-bottom: 1px solid var(--border-color, #dee2e6);
}

.drilldown-item:hover {
  background: var(--bg-secondary, #f8f9fa);
}

.drilldown-item:active {
  background: var(--bg-tertiary, #e9ecef);
}

.drilldown-arrow {
  margin-left: auto;
  flex-shrink: 0;
  color: var(--text-muted, #999);
}

.git-commit-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent-color, #4a90d9);
  flex-shrink: 0;
  margin-top: 5px;
}

.git-commit-dot-wt {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #f59e0b;
  flex-shrink: 0;
  margin-top: 5px;
  box-shadow: 0 0 0 2px rgba(245, 158, 11, 0.3);
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

<template>
  <BottomSheet :open="open" @close="handleClose">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <line x1="6" y1="3" x2="6" y2="15"/>
        <circle cx="18" cy="6" r="3"/>
        <circle cx="6" cy="18" r="3"/>
        <path d="M15 6a9 9 0 0 0-9 9V3"/>
      </svg>
      <span class="bs-header-title">{{ mode === 'file' ? '文件历史' : '项目历史' }}</span>
      <div v-if="mode === 'project' && store.state.projectRoot" class="bs-header-description">
        <span class="bs-header-description-inner" :title="store.state.projectRoot">
          {{ store.state.projectRoot }}
        </span>
      </div>
      <div v-else-if="mode === 'file' && file?.path" class="bs-header-description">
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

    <!-- Loading (initial) -->
    <div v-if="loading" class="git-history-loading">
      <div class="spinner" style="width:24px;height:24px;border-width:2px;margin:0 auto;" />
    </div>

    <!-- Error -->
    <div v-else-if="error" class="git-history-error">
      {{ error }}
    </div>

    <!-- View: commit list (shared by both modes) -->
    <GitCommitList
      v-else-if="currentView === 'commits'"
      ref="commitListRef"
      :commits="commits"
      :is-git="isGit"
      :has-more="hasMore"
      :loading-more="loadingMore"
      :search-loading="searchLoading"
      :init-loading="initLoading"
      :loading="false"
      :error="''"
      :untracked="untracked"
      :count-label="mode === 'file' ? '记录' : '提交记录'"
      @select="onCommitSelect"
      @search="onSearch"
      @load-more="loadMoreCommits"
      @init-git="initGitRepo"
    />

    <!-- View: file list for selected commit (project mode only) -->
    <div v-else-if="currentView === 'files'" class="drilldown-page">
      <div class="drilldown-header">
        <div class="drilldown-title">
          <span class="drilldown-count">{{ files.length }} 个变更文件</span>
        </div>
        <button class="drilldown-back-btn" @click="drillBack('commits')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <polyline points="15 18 9 12 15 6"/>
          </svg>
          返回
        </button>
      </div>
      <GitCommitMeta :commit="selectedCommit" :is-working-tree="isWorkingTree" />
      <div class="drilldown-body">
        <div v-if="filesLoading" class="git-history-loading">
          <div class="spinner" style="width:24px;height:24px;border-width:2px;" />
        </div>
        <div v-else-if="files.length === 0" class="git-history-empty">此提交无文件变更</div>
        <div v-else class="drilldown-list">
          <div
            v-for="f in files"
            :key="f.path + '-' + f.type + (f.staged ? '-s' : '')"
            class="drilldown-item"
            @click="drillToFile(f)"
          >
            <span class="git-file-icon">
              <svg v-if="f.type === 'A'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" width="14" height="14">
                <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
              </svg>
              <svg v-else-if="f.type === 'D'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" width="14" height="14">
                <line x1="5" y1="12" x2="19" y2="12"/>
              </svg>
              <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                <polyline points="14 2 14 8 20 8"/>
              </svg>
            </span>
            <span class="git-file-type-badge" :class="badgeClass(f)">{{ fileTypeLabel(f.type, f.staged) }}</span>
            <span class="git-file-path">{{ f.path }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- View: diff (shared by both modes) -->
    <div v-else-if="currentView === 'diff'" class="drilldown-page">
      <div class="drilldown-header">
        <div class="drilldown-title">
          <span class="drilldown-count">{{ diffTitle }}</span>
        </div>
        <button class="drilldown-back-btn" @click="drillBack(diffBackTarget)">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
            <polyline points="15 18 9 12 15 6"/>
          </svg>
          返回
        </button>
      </div>
      <div class="drilldown-body">
        <GitCommitMeta :commit="selectedCommit" :is-working-tree="isWorkingTree" />
        <GitDiffView
          :loading="diffState.loading"
          :empty="diffState.empty"
          :html="diffState.html"
          :no-wrap="mode === 'project'"
        />
      </div>
    </div>
  </BottomSheet>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import BottomSheet from './BottomSheet.vue'
import GitCommitList from './GitCommitList.vue'
import GitCommitMeta from './GitCommitMeta.vue'
import GitDiffView from './GitDiffView.vue'
import { renderDiff } from '@/utils/diff.ts'
import { store } from '@/stores/app.ts'

const props = defineProps({
  open: Boolean,
  mode: {
    type: String,
    default: 'project', // 'project' | 'file'
  },
  file: Object, // { path, name } — used when mode === 'file'
})

const emit = defineEmits(['close', 'open-file'])

// ─── Unified state ─────────────────────────────────────────────────────────

const loading = ref(false)
const error = ref('')
const commits = ref([])
const hasMore = ref(false)
const searchLoading = ref(false)
const loadingMore = ref(false)
const isGit = ref(false)
const initLoading = ref(false)
const untracked = ref(false)

const currentView = ref('commits') // 'commits' | 'files' | 'diff'
const selectedSHA = ref(null)

// Files view (project mode only)
const filesLoading = ref(false)
const files = ref([])
const selectedFilePath = ref(null)

// Unified diff state
const diffState = ref({ loading: false, empty: false, html: '' })

// Working tree
const wtFiles = ref([])

const commitListRef = ref(null)

const selectedCommit = computed(() => {
  return commits.value.find(c => c.sha === selectedSHA.value) || null
})
const isWorkingTree = computed(() => selectedSHA.value === 'HEAD')

const diffTitle = computed(() => {
  if (mode === 'file') return '比较报告'
  return selectedFilePath.value || ''
})

const diffBackTarget = computed(() => {
  return props.mode === 'file' ? 'commits' : 'files'
})

const mode = computed(() => props.mode)

// ─── Helpers ────────────────────────────────────────────────────────────────

function fileTypeLabel(t, staged) {
  const labels = { A: '新增', M: '修改', D: '删除', R: '重命名', '?': '未跟踪' }
  const base = labels[t] || t
  return staged ? '已暂存·' + base : base
}

function badgeClass(f) {
  const typeMap = { A: 'A', M: 'M', D: 'D', R: 'R', '?': 'U' }
  const cls = typeMap[f.type] || 'M'
  return 'badge-' + cls + (f.staged ? ' badge-staged' : '')
}

function resetState() {
  commits.value = []
  files.value = []
  hasMore.value = false
  selectedSHA.value = null
  selectedFilePath.value = null
  diffState.value = { loading: false, empty: false, html: '' }
  currentView.value = 'commits'
  error.value = ''
  commitSearch.value = ''
  isGit.value = false
  untracked.value = false
  wtFiles.value = []
}

// Expose commitSearch for the search watcher
const commitSearch = ref('')

// ─── Data loading ───────────────────────────────────────────────────────────

async function loadProjectHistory() {
  loading.value = true
  error.value = ''
  commits.value = []
  hasMore.value = false
  selectedSHA.value = null
  files.value = []
  selectedFilePath.value = null
  wtFiles.value = []
  isGit.value = true

  try {
    const resp = await fetch('/api/git/project-history')
    if (!resp.ok) {
      const data = await resp.json()
      error.value = data.error || '加载历史记录失败'
      return
    }
    const data = await resp.json()

    if (!data.isGit) {
      isGit.value = false
      return
    }

    isGit.value = true

    // Check working tree changes
    const wtResp = await fetch('/api/git/working-tree')
    let loadedWtFiles = []
    if (wtResp.ok) {
      const wt = await wtResp.json()
      loadedWtFiles = wt.files || []
      wtFiles.value = loadedWtFiles
    }

    const histCommits = data.commits || []

    // Prepend working tree entry if there are uncommitted changes
    if (loadedWtFiles.length > 0) {
      commits.value = [{ sha: 'HEAD', msg: '工作区变更', date: '', author: '', isWT: true }, ...histCommits]
    } else {
      commits.value = histCommits
    }
    hasMore.value = data.hasMore
  } catch {
    error.value = '加载历史记录失败'
  } finally {
    loading.value = false
  }
}

async function loadFileHistory(filePath) {
  loading.value = true
  error.value = ''
  commits.value = []
  selectedSHA.value = null
  isGit.value = true
  untracked.value = false

  try {
    const resp = await fetch(`/api/git/history?path=${encodeURIComponent(filePath)}`)
    if (!resp.ok) {
      const data = await resp.json()
      error.value = data.error || '加载历史记录失败'
      return
    }
    const hist = await resp.json()
    if (!hist.isGit) {
      isGit.value = false
      loading.value = false
      return
    }
    isGit.value = true
    untracked.value = !!hist.untracked
    commits.value = hist.commits || []
  } catch {
    error.value = '加载历史记录失败'
  } finally {
    loading.value = false
  }
}

async function loadMoreCommits() {
  if (loadingMore.value || !hasMore.value || !isGit.value) return
  loadingMore.value = true
  try {
    // Count only git commits (exclude WT node) for the skip parameter,
    // since WT is a frontend-only entry not present in git log output.
    const gitCount = commits.value.filter(c => !c.isWT).length
    const resp = await fetch(`/api/git/project-history?skip=${gitCount}`)
    if (!resp.ok) return
    const data = await resp.json()
    commits.value.push(...(data.commits || []))
    hasMore.value = data.hasMore
  } catch {
    // ignore
  } finally {
    loadingMore.value = false
  }
}

// When searching, auto-load all commits so filtering covers the full history
async function onSearch(q) {
  if (!q.trim() || !isGit.value || props.mode === 'file') return
  searchLoading.value = true
  try {
    while (hasMore.value) {
      const gitCount = commits.value.filter(c => !c.isWT).length
      const resp = await fetch(`/api/git/project-history?skip=${gitCount}`)
      if (!resp.ok) break
      const data = await resp.json()
      commits.value.push(...(data.commits || []))
      hasMore.value = data.hasMore
    }
  } finally {
    searchLoading.value = false
  }
}

async function initGitRepo() {
  isGit.value = true
  initLoading.value = true
  try {
    const resp = await fetch('/api/git/init', { method: 'POST' })
    if (resp.ok) {
      if (props.mode === 'file') {
        await loadFileHistory(props.file?.path)
      } else {
        await loadProjectHistory()
      }
    }
  } catch {
    // ignore
  } finally {
    initLoading.value = false
  }
}

// ─── Drill-down navigation ──────────────────────────────────────────────────

function onCommitSelect(c) {
  selectedSHA.value = c.sha

  if (props.mode === 'project') {
    // Project mode: commit → files list
    currentView.value = 'files'
    if (c.sha === 'HEAD') {
      filesLoading.value = true
      files.value = wtFiles.value
      filesLoading.value = false
    } else {
      loadCommitFiles(c.sha).catch(() => {})
    }
  } else {
    // File mode: commit → diff
    currentView.value = 'diff'
    loadDiff()
  }
}

function drillBack(view) {
  if (view === 'commits') {
    selectedSHA.value = null
    files.value = []
    selectedFilePath.value = null
    diffState.value = { loading: false, empty: false, html: '' }
  } else if (view === 'files') {
    selectedFilePath.value = null
    diffState.value = { loading: false, empty: false, html: '' }
  }
  currentView.value = view
}

function drillToFile(f) {
  selectedFilePath.value = f.path
  currentView.value = 'diff'
  loadDiff()
}

// ─── Diff loading ───────────────────────────────────────────────────────────

async function loadCommitFiles(sha) {
  filesLoading.value = true
  files.value = []
  try {
    const resp = await fetch(`/api/git/commit-files?sha=${encodeURIComponent(sha)}`)
    if (!resp.ok) { files.value = []; return }
    files.value = await resp.json()
  } catch {
    files.value = []
  } finally {
    filesLoading.value = false
  }
}

async function loadDiff() {
  diffState.value = { loading: true, empty: false, html: '' }

  try {
    let resp
    if (props.mode === 'project') {
      resp = await fetch(
        `/api/git/file-diff?sha=${encodeURIComponent(selectedSHA.value)}&path=${encodeURIComponent(selectedFilePath.value)}`
      )
    } else {
      resp = await fetch(
        `/api/git/diff?path=${encodeURIComponent(props.file.path)}&commit=${encodeURIComponent(selectedSHA.value)}`
      )
    }
    if (!resp.ok) {
      diffState.value = { loading: false, empty: true, html: '' }
      return
    }
    const data = await resp.json()
    if (data.empty) {
      diffState.value = { loading: false, empty: true, html: '' }
    } else {
      const filePath = props.mode === 'project' ? selectedFilePath.value : props.file.path
      diffState.value = { loading: false, empty: false, html: renderDiff(data.diff || '', filePath) }
    }
  } catch {
    diffState.value = { loading: false, empty: true, html: '' }
  }
}

// ─── Lifecycle ──────────────────────────────────────────────────────────────

function handleClose() {
  emit('close')
}

watch(() => props.open, async (val) => {
  if (!val) {
    resetState()
    commitListRef.value?.unobserveList()
    return
  }

  if (props.mode === 'file' && props.file?.path) {
    await loadFileHistory(props.file.path)
  } else {
    await loadProjectHistory()
  }
  // Start observing after content loads
  setTimeout(() => commitListRef.value?.observeList(), 100)
})
</script>

<style scoped>
.git-history-loading {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.git-history-error {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted, #999);
  font-size: 14px;
}

/* ─── Drill-down shared ────────────────────────────────────────────────── */

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

.drilldown-back-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 10px;
  border: 1px solid var(--border-color, #dee2e6);
  border-radius: 6px;
  background: var(--bg-primary, #ffffff);
  color: var(--accent-color, #4a90d9);
  font-size: 13px;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.15s, border-color 0.15s;
}

.drilldown-back-btn:hover {
  background: var(--bg-secondary, #f8f9fa);
  border-color: var(--accent-color, #4a90d9);
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

.git-history-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted, #999);
  font-size: 14px;
}

/* ─── File list (project mode) ────────────────────────────────────────── */

.git-file-icon {
  flex-shrink: 0;
  color: var(--text-muted, #999);
  display: flex;
  align-items: center;
}

.git-file-type-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 5px;
  border-radius: 4px;
  flex-shrink: 0;
  letter-spacing: 0.02em;
}

.badge-A { background: color-mix(in srgb, var(--color-green, #16a34a) 15%, transparent); color: var(--color-green, #16a34a); }
.badge-M { background: color-mix(in srgb, var(--color-yellow, #a16207) 15%, transparent); color: var(--color-yellow, #a16207); }
.badge-D { background: color-mix(in srgb, var(--color-red, #dc2626) 15%, transparent); color: var(--color-red, #dc2626); }
.badge-R { background: color-mix(in srgb, var(--color-purple, #7c3aed) 15%, transparent); color: var(--color-purple, #7c3aed); }
.badge-U { background: var(--bg-tertiary, #f0f0f0); color: var(--text-muted, #999); }
.badge-staged { border: 1px solid var(--accent-color, #4a90d9); }

.git-file-path {
  color: var(--text-primary, #212529);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

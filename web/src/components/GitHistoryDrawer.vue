<template>
  <BottomSheet :open="open" @close="handleClose">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <circle cx="12" cy="12" r="10"/>
        <polyline points="12 6 12 12 16 14"/>
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

    <!-- Loading -->
    <div v-if="loading" class="git-history-loading">
      <div class="spinner" style="width:24px;height:24px;border-width:2px;margin:0 auto;" />
    </div>

    <!-- Error -->
    <div v-else-if="error" class="git-history-error">
      {{ error }}
    </div>

    <!-- Project mode: three-level drill-down -->
    <template v-else-if="mode === 'project'">

      <!-- Level 1: commit list -->
      <div v-if="projectView === 'commits'" class="drilldown-page">
        <div class="drilldown-header">
          <div class="drilldown-title">
            <span class="drilldown-count" v-if="commits.length > 0">{{ searchLoading ? '加载中…' : filteredCommits.length + (hasMore && !commitSearch ? '+' : '') + ' 条提交记录' }}</span>
            <span v-else-if="!isGit" class="drilldown-count">未初始化</span>
            <span v-else class="drilldown-count">加载中…</span>
          </div>
          <input
            v-if="commits.length > 0"
            v-model="commitSearch"
            class="drilldown-search"
            placeholder="搜索提交信息…"
            type="text"
            @dblclick="commitSearch = ''"
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
              <button class="init-git-btn" @click.stop="initGitRepo" :disabled="initLoading">
                <span v-if="initLoading" class="spinner" style="width:14px;height:14px;border-width:2px;" />
                <span v-else>初始化 Git</span>
              </button>
            </div>
          </div>
          <div v-else-if="commits.length === 0" class="git-history-empty">暂无提交记录</div>
          <div v-else class="drilldown-list">
            <div
              v-for="c in filteredCommits"
              :key="c.sha"
              class="drilldown-item"
              @click="drillToCommit(c)"
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

      <!-- Level 2: file list for selected commit -->
      <div v-else-if="projectView === 'files'" class="drilldown-page">
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
        <!-- Commit metadata panel -->
        <div v-if="selectedCommit || isWorkingTree" class="diff-meta-panel">
          <template v-if="isWorkingTree">
            <div class="diff-meta-row diff-meta-row-msg">
              <span class="diff-meta-label">说明</span>
              <span class="diff-meta-value">工作区变更</span>
            </div>
          </template>
          <template v-else>
            <div class="diff-meta-row">
              <span class="diff-meta-label">SHA</span>
              <span class="diff-meta-value diff-meta-sha">{{ selectedCommit.sha.substring(0, 8) }}</span>
            </div>
            <div class="diff-meta-row">
              <span class="diff-meta-label">作者</span>
              <span class="diff-meta-value">{{ selectedCommit.author }}</span>
            </div>
            <div class="diff-meta-row">
              <span class="diff-meta-label">时间</span>
              <span class="diff-meta-value">{{ formatDate(selectedCommit.date) }}</span>
            </div>
            <div class="diff-meta-row diff-meta-row-msg">
              <span class="diff-meta-label">说明</span>
              <span class="diff-meta-value">{{ selectedCommit.msg }}</span>
            </div>
          </template>
        </div>
        <div class="drilldown-body">
          <div v-if="filesLoading" class="git-history-loading">
            <div class="spinner" style="width:24px;height:24px;border-width:2px;" />
          </div>
          <div v-else-if="files.length === 0" class="git-history-empty">此提交无文件变更</div>
          <div v-else class="drilldown-list">
            <div
              v-for="f in files"
              :key="f.path"
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
              <span class="git-file-type-badge" :class="'badge-' + f.type">{{ fileTypeLabel(f.type) }}</span>
              <span class="git-file-path">{{ f.path }}</span>
              <svg class="drilldown-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
                <polyline points="9 18 15 12 9 6"/>
              </svg>
            </div>
          </div>
        </div>
      </div>

      <!-- Level 3: diff view -->
      <div v-else-if="projectView === 'diff'" class="drilldown-page">
        <div class="drilldown-header">
          <div class="drilldown-title">
            <span class="drilldown-count">{{ selectedFilePath }}</span>
          </div>
          <button class="drilldown-back-btn" @click="drillBack('files')">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <polyline points="15 18 9 12 15 6"/>
            </svg>
            返回
          </button>
        </div>
        <div class="drilldown-body">
          <div v-if="fileDiffLoading" class="git-history-loading">
            <div class="spinner" style="width:24px;height:24px;border-width:2px;" />
          </div>
          <div v-else-if="fileDiffEmpty" class="git-history-empty">无变更</div>
          <div v-else class="git-diff-scroll no-wrap" v-html="fileDiffHtml" />
        </div>
      </div>

    </template>

    <!-- File mode: two-level drill-down -->
    <template v-else-if="mode === 'file'">

      <!-- Level 1: commit list -->
      <div v-if="fileView === 'commits'" class="drilldown-page">
        <div class="drilldown-header">
          <div class="drilldown-title">
            <span class="drilldown-count" v-if="commits.length > 0">{{ searchLoading ? '加载中…' : filteredCommits.length + ' 条记录' }}</span>
            <span v-else-if="!fileIsGit" class="drilldown-count">未初始化</span>
            <span v-else class="drilldown-count">加载中…</span>
          </div>
          <input
            v-if="commits.length > 0"
            v-model="commitSearch"
            class="drilldown-search"
            placeholder="搜索提交信息…"
            type="text"
            @dblclick="commitSearch = ''"
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
          <div v-else-if="!fileIsGit" class="git-history-empty">
            <div class="init-git-prompt">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="40" height="40" style="color:#ccc;margin-bottom:12px;">
                <circle cx="12" cy="12" r="10"/>
                <line x1="12" y1="8" x2="12" y2="16"/>
                <line x1="8" y1="12" x2="16" y2="12"/>
              </svg>
              <div style="font-size:14px;color:var(--text-muted,#999);margin-bottom:12px;">尚未初始化 Git 仓库</div>
              <button class="init-git-btn" @click.stop="initGitRepo" :disabled="initLoading">
                <span v-if="initLoading" class="spinner" style="width:14px;height:14px;border-width:2px;" />
                <span v-else>初始化 Git</span>
              </button>
            </div>
          </div>
          <div v-else-if="commits.length === 0 && fileUntracked" class="git-history-empty">
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
              @click="drillToFileCommit(c)"
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
          </div>
        </div>
      </div>

      <!-- Level 2: diff view -->
      <div v-else-if="fileView === 'diff'" class="drilldown-page">
        <div class="drilldown-header">
          <div class="drilldown-title">
            <span class="drilldown-count">比较报告</span>
          </div>
          <button class="drilldown-back-btn" @click="drillBackFile()">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <polyline points="15 18 9 12 15 6"/>
            </svg>
            返回
          </button>
        </div>
        <div class="drilldown-body">
          <div v-if="selectedCommit || isWorkingTree" class="diff-meta-panel">
            <template v-if="isWorkingTree">
              <div class="diff-meta-row diff-meta-row-msg">
                <span class="diff-meta-label">说明</span>
                <span class="diff-meta-value">工作区变更</span>
              </div>
            </template>
            <template v-else>
              <div class="diff-meta-row">
                <span class="diff-meta-label">SHA</span>
                <span class="diff-meta-value diff-meta-sha">{{ selectedCommit.sha.substring(0, 8) }}</span>
              </div>
              <div class="diff-meta-row">
                <span class="diff-meta-label">作者</span>
                <span class="diff-meta-value">{{ selectedCommit.author }}</span>
              </div>
              <div class="diff-meta-row">
                <span class="diff-meta-label">时间</span>
                <span class="diff-meta-value">{{ formatDate(selectedCommit.date) }}</span>
              </div>
              <div class="diff-meta-row diff-meta-row-msg">
                <span class="diff-meta-label">说明</span>
                <span class="diff-meta-value">{{ selectedCommit.msg }}</span>
              </div>
            </template>
          </div>
          <div v-if="diffLoading" class="git-history-loading">
            <div class="spinner" style="width:24px;height:24px;border-width:2px;" />
          </div>
          <div v-else-if="diffEmpty" class="git-history-empty">无变更</div>
          <div v-else class="git-diff-scroll" v-html="diffHtml" />
        </div>
      </div>

    </template>
  </BottomSheet>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import BottomSheet from './BottomSheet.vue'
import { renderDiff, detectLang, highlightLine } from '@/utils/diff.ts'
import { escapeHtml } from '@/utils/helpers.ts'
import { store } from '@/stores/app.ts'

const props = defineProps({
    open: Boolean,
    mode: {
        type: String,
        default: 'project', // 'project' | 'file'
    },
    file: Object,  // { path, name } — used when mode === 'file'
})

const emit = defineEmits(['close', 'open-file'])

// Shared state
const loading = ref(false)
const error = ref('')
const commits = ref([])
const commitSearch = ref('')
const filteredCommits = computed(() => {
    const q = commitSearch.value.trim().toLowerCase()
    if (!q) return commits.value
    return commits.value.filter(c => c.msg.toLowerCase().includes(q))
})
const hasMore = ref(false)
const searchLoading = ref(false)
const loadingMore = ref(false)
const selectedSHA = ref(null)
const isGit = ref(false)
const initLoading = ref(false)
const listRef = ref(null)
const observer = ref(null)
const selectedCommit = computed(() => {
    return commits.value.find(c => c.sha === selectedSHA.value) || null
})
const isWorkingTree = computed(() => selectedSHA.value === 'HEAD')
const workingTreeEntry = computed(() => ({
    sha: 'HEAD',
    msg: '工作区变更',
    date: '',
    author: '',
    isWT: true,
}))

// Working tree: uses selectedSHA = 'HEAD' to distinguish
const projectWTFiles = ref([])
const projectHasWT = ref(false)
const fileHasWT = ref(false)

// Project mode: drill-down views ('commits' | 'files' | 'diff')
const projectView = ref('commits')
const filesLoading = ref(false)
const files = ref([])
const selectedFilePath = ref(null)
const fileDiffLoading = ref(false)
const fileDiffEmpty = ref(false)
const fileDiffHtml = ref('')

// File mode: drill-down view ('commits' | 'diff')
const fileView = ref('commits')
const diffLoading = ref(false)
const diffEmpty = ref(false)
const diffHtml = ref('')
const fileIsGit = ref(true)
const fileUntracked = ref(false)

function formatDate(dateStr) {
    if (!dateStr) return ''
    try {
        const d = new Date(dateStr)
        return d.toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
    } catch {
        return dateStr
    }
}

function fileTypeLabel(t) {
    return { A: '新增', M: '修改', D: '删除', R: '重命名' }[t] || t
}

// ─── Project mode ────────────────────────────────────────────────────────────

async function loadProjectHistory() {
    loading.value = true
    error.value = ''
    commits.value = []
    hasMore.value = false
    selectedSHA.value = null
    files.value = []
    selectedFilePath.value = null
    projectHasWT.value = false
    projectWTFiles.value = []
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
            loading.value = false
            return
        }

        isGit.value = true

        // Check working tree changes
        const wtResp = await fetch('/api/git/working-tree')
        let wtFiles = []
        if (wtResp.ok) {
            const wt = await wtResp.json()
            projectHasWT.value = wt.hasUncommitted
            wtFiles = wt.files || []
            projectWTFiles.value = wtFiles
        }

        const histCommits = data.commits || []

        // Prepend working tree entry
        if (wtFiles.length > 0) {
            commits.value = [{ sha: 'HEAD', msg: '工作区变更', date: '', author: '', isWT: true }, ...histCommits]
        } else {
            commits.value = histCommits
        }
        hasMore.value = data.hasMore
    } catch (e) {
        error.value = '加载历史记录失败'
    } finally {
        loading.value = false
    }
}

async function loadMoreCommits() {
    if (loadingMore.value || !hasMore.value || !isGit.value) return
    loadingMore.value = true
    try {
        const resp = await fetch(`/api/git/project-history?skip=${commits.value.length}`)
        if (!resp.ok) return
        const data = await resp.json()
        commits.value.push(...(data.commits || []))
        hasMore.value = data.hasMore
    } catch (e) {
        // ignore
    } finally {
        loadingMore.value = false
    }
}

// When searching, auto-load all commits so filtering covers the full history
watch(commitSearch, async (q) => {
    if (!q.trim() || !isGit.value) return
    searchLoading.value = true
    try {
        while (hasMore.value) {
            const resp = await fetch(`/api/git/project-history?skip=${commits.value.length}`)
            if (!resp.ok) break
            const data = await resp.json()
            commits.value.push(...(data.commits || []))
            hasMore.value = data.hasMore
        }
    } finally {
        searchLoading.value = false
    }
})

async function initGitRepo() {
    if (props.mode === 'file') {
        fileIsGit.value = true
    } else {
        isGit.value = true
    }
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
    } catch (e) {
        // ignore
    } finally {
        initLoading.value = false
    }
}

// ─── Project mode: drill-down ─────────────────────────────────────────────

function drillToCommit(c) {
    selectedSHA.value = c.sha
    projectView.value = 'files'
    if (c.sha === 'HEAD') {
        // Working tree: use pre-loaded file list
        filesLoading.value = true
        files.value = projectWTFiles.value
        filesLoading.value = false
    } else {
        // await so Promise rejections are caught here, not in the Vue click handler
        loadCommitFiles(c.sha).catch(() => {})
    }
}

function drillBack(view) {
    if (view === 'commits') {
        selectedSHA.value = null
        files.value = []
        selectedFilePath.value = null
        fileDiffHtml.value = ''
    } else if (view === 'files') {
        selectedFilePath.value = null
        fileDiffHtml.value = ''
    }
    projectView.value = view
}

async function loadCommitFiles(sha) {
    filesLoading.value = true
    files.value = []
    try {
        const resp = await fetch(`/api/git/commit-files?sha=${encodeURIComponent(sha)}`)
        if (!resp.ok) { files.value = []; return }
        files.value = await resp.json()
    } catch (e) {
        files.value = []
    } finally {
        filesLoading.value = false
    }
}

function drillToFile(f) {
    selectedFilePath.value = f.path
    projectView.value = 'diff'
    loadFileDiff(f.path)
}

async function loadFileDiff(filePath) {
    fileDiffLoading.value = true
    fileDiffEmpty.value = false
    fileDiffHtml.value = ''
    try {
        const resp = await fetch(
            `/api/git/file-diff?sha=${encodeURIComponent(selectedSHA.value)}&path=${encodeURIComponent(filePath)}`
        )
        if (!resp.ok) { fileDiffEmpty.value = true; return }
        const data = await resp.json()
        if (data.empty) {
            fileDiffEmpty.value = true
        } else {
            fileDiffHtml.value = renderDiff(data.diff || '', filePath)
        }
    } catch (e) {
        fileDiffEmpty.value = true
    } finally {
        fileDiffLoading.value = false
    }
}

// ─── File mode ───────────────────────────────────────────────────────────────

async function loadFileHistory(filePath) {
    loading.value = true
    error.value = ''
    commits.value = []
    selectedSHA.value = null
    diffHtml.value = ''
    diffEmpty.value = false
    fileView.value = 'commits'
    fileHasWT.value = false
    fileIsGit.value = true
    fileUntracked.value = false

    try {
        // Check working tree changes for this file (also tells us if it's a git repo)
        const wtResp = await fetch(`/api/git/working-tree?path=${encodeURIComponent(filePath)}`)
        if (wtResp.ok) {
            const wt = await wtResp.json()
            if (!wt.isGit) {
                fileIsGit.value = false
                loading.value = false
                return
            }
            if (wt.hasUncommitted) {
                fileHasWT.value = true
                commits.value.push({ sha: 'HEAD', msg: '工作区变更', date: '', author: '', isWT: true })
            }
        }

        const resp = await fetch(`/api/git/history?path=${encodeURIComponent(filePath)}`)
        if (!resp.ok) {
            const data = await resp.json()
            error.value = data.error || '加载历史记录失败'
            return
        }
        const hist = await resp.json()
        fileUntracked.value = !!hist.untracked
        commits.value.push(...(hist.commits || []))
    } catch (e) {
        error.value = '加载历史记录失败'
    } finally {
        loading.value = false
    }
}

function drillToFileCommit(c) {
    selectedSHA.value = c.sha
    fileView.value = 'diff'
    loadFileDiffForMode()
}

function drillBackFile() {
    selectedSHA.value = null
    diffHtml.value = ''
    diffEmpty.value = false
    fileView.value = 'commits'
}

async function loadFileDiffForMode() {
    diffLoading.value = true
    diffHtml.value = ''
    diffEmpty.value = false
    try {
        const resp = await fetch(
            `/api/git/diff?path=${encodeURIComponent(props.file.path)}&commit=${encodeURIComponent(selectedSHA.value)}`
        )
        const data = await resp.json()
        if (data.empty) {
            diffEmpty.value = true
        } else {
            diffHtml.value = renderDiff(data.diff || '', props.file.path)
        }
    } catch (e) {
        diffEmpty.value = true
    } finally {
        diffLoading.value = false
    }
}

function handleClose() {
    emit('close')
}

function setupIntersectionObserver() {
    observer.value = new IntersectionObserver((entries) => {
        if (entries[0].isIntersecting && hasMore.value && !loadingMore.value) {
            loadMoreCommits()
        }
    }, {
        threshold: 0.1,
        rootMargin: '100px'
    })
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
    setupIntersectionObserver()
})

onUnmounted(() => {
    unobserveList()
})

watch(() => props.open, async (val) => {
    if (!val) {
        commits.value = []
        files.value = []
        hasMore.value = false
        selectedSHA.value = null
        commitSearch.value = ''
        selectedFilePath.value = null
        fileDiffHtml.value = ''
        diffHtml.value = ''
        diffEmpty.value = false
        projectView.value = 'commits'
        fileView.value = 'commits'
        unobserveList()
        return
    }

    if (props.mode === 'file' && props.file?.path) {
        fileView.value = 'commits'
        await loadFileHistory(props.file.path)
    } else {
        projectView.value = 'commits'
        await loadProjectHistory()
    }
    // Start observing after content loads
    setTimeout(() => observeList(), 100)
})
</script>

<style scoped>
.git-history-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary, #212529);
    overflow-x: auto;
    scrollbar-width: none;
    flex: 1;
}
.git-history-title::-webkit-scrollbar {
    display: none;
}

.git-history-filename {
    font-size: 12px;
    font-weight: 400;
    color: var(--accent-color, #4a90d9);
    background: var(--bg-tertiary, #e9ecef);
    padding: 2px 8px;
    border-radius: 10px;
    white-space: nowrap;
    flex-shrink: 0;
    cursor: default;
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

/* ─── Project mode: drill-down ────────────────────────────────────────── */

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

.git-commit-dot-wt {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #f59e0b;
    flex-shrink: 0;
    margin-top: 5px;
    box-shadow: 0 0 0 2px rgba(245, 158, 11, 0.3);
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

.drilldown-arrow {
    margin-left: auto;
    flex-shrink: 0;
    color: var(--text-muted, #999);
}

.diff-meta-panel {
    padding: 12px 14px;
    border-bottom: 1px solid var(--border-color, #dee2e6);
    background: var(--bg-secondary, #f8f9fa);
    display: flex;
    flex-direction: column;
    gap: 5px;
    flex-shrink: 0;
}

.diff-meta-row {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    font-size: 13px;
}

.diff-meta-label {
    color: var(--text-muted, #999);
    flex-shrink: 0;
    width: 36px;
    padding-top: 1px;
}

.diff-meta-value {
    color: var(--text-primary, #212529);
    word-break: break-all;
}

.diff-meta-sha {
    font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
    font-size: 12px;
    color: var(--accent-color, #4a90d9);
}

.diff-meta-row-msg .diff-meta-value {
    font-weight: 500;
}

.git-commit-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--accent-color, #4a90d9);
    flex-shrink: 0;
    margin-top: 5px;
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

.badge-A { background: #dcfce7; color: #16a34a; }
.badge-M { background: #fef9c3; color: #a16207; }
.badge-D { background: #fee2e2; color: #dc2626; }
.badge-R { background: #ede9fe; color: #7c3aed; }

.git-file-path {
    color: var(--text-primary, #212529);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.git-diff-scroll {
    padding: 12px;
}

.git-diff-scroll :deep(.diff-card-view) {
    display: flex;
    flex-direction: column;
    gap: 12px;
}

.git-diff-scroll :deep(.diff-hunk-loc) {
    font-size: 12px;
    color: var(--text-muted, #999);
    padding: 0 4px;
}

.git-diff-scroll :deep(.diff-card-pair) {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.git-diff-scroll :deep(.diff-card) {
    overflow-x: auto;
    font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, 'Courier New', monospace;
    font-size: 12px;
    line-height: 1.6;
}

.git-diff-scroll :deep(.diff-card-add) {
    background: rgba(34, 197, 94, 0.08);
    border-left: 3px solid #22c55e;
}

.git-diff-scroll :deep(.diff-card-del) {
    background: rgba(239, 68, 68, 0.08);
    border-left: 3px solid #ef4444;
}

.git-diff-scroll :deep(.diff-card-label) {
    font-size: 11px;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 0;
    font-family: system-ui, sans-serif;
    letter-spacing: 0.02em;
    display: inline-block;
    margin-bottom: 4px;
}

.git-diff-scroll :deep(.diff-card-add .diff-card-label) {
    background: rgba(34, 197, 94, 0.15);
    color: #16a34a;
}

.git-diff-scroll :deep(.diff-card-del .diff-card-label) {
    background: rgba(239, 68, 68, 0.15);
    color: #dc2626;
}

.git-diff-scroll :deep(.diff-card-line) {
    padding: 2px 10px;
    white-space: pre;
    color: inherit;
}

.git-diff-scroll :deep(.diff-raw) {
    font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, 'Courier New', monospace;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    word-break: break-all;
    color: var(--text-primary, #212529);
    margin: 0;
}

</style>

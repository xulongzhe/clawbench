<template>
  <div>
    <!-- Loading state: show nothing while checking auth -->
    <div v-if="isAuthenticated === null" style="display:none" />

    <!-- Login -->
    <LoginView v-else-if="!isAuthenticated" @login-success="handleLoginSuccess" />

    <!-- Main app -->
    <div v-else class="app-container">
      <AppHeader
        :project-root="projectRoot"
        :theme="theme"
        @toggle-theme="toggleTheme"
        @open-project-dialog="handleOpenProjectDialog"
      />

      <FileManager
        :entries="dirEntries"
        :current-dir="currentDir"
        :current-file="currentFile"
        :open="sidebarOpen"
        :show-hidden="showHidden"
        :sort-field="sortField"
        :sort-dir="sortDir"
        @close="sidebarOpen = false"
        @navigate-dir="handleNavigateDir"
        @select-file="handleSelectFile"
        @toggle-sort="handleToggleSort"
        @toggle-hidden="toggleHidden"
        @rename="handleRename"
        @delete="handleDelete"
        @refresh="handleRefresh"
      />

      <main class="main-content">
        <div
          class="content-area"
          id="contentArea"
          :style="contentSwipeStyle"
        >
          <WelcomeView v-if="!currentFile" />
          <FileViewer
            v-if="currentFile"
            :file="currentFile"
            :toc-open="tocOpen"
            :search-open="searchOpen"
            :markdown-view-mode="markdownViewMode"
            @delete="handleDelete(currentFile?.path)"
            @show-details="detailsOpen = true"
            @open-git-history="openDrawer('fileHistory')"
            @toggle-toc="openDrawer('toc')"
            @toggle-search="currentFile?.content && openDrawer('search')"
            @toggle-view="markdownViewMode = markdownViewMode === 'rendered' ? 'raw' : 'rendered'"
          />
        </div>
      </main>

      <Lightbox />

      <ChatPanel
        :open="chatOpen"
        :current-file="currentFile"
        @close="chatOpen = false"
        @open="chatOpen = true"
        @message="handleChatMessage()"
        :initial-tab="initialChatTab"
      />

      <GitHistoryDrawer
        :open="projectHistoryOpen"
        mode="project"
        @close="projectHistoryOpen = false"
        @open-file="handleSelectFile"
      />

      <GitHistoryDrawer
        :open="fileHistoryOpen"
        mode="file"
        :file="currentFile"
        @close="fileHistoryOpen = false"
        @open-file="handleSelectFile"
      />

      <TocDrawer
        :file="tocFile"
        :open="tocOpen"
        @close="tocOpen = false"
        @jump="scrollToLine"
      />

      <SearchDrawer
        :file="currentFile"
        :open="searchOpen"
        :view-mode="currentFileIsMarkdown ? markdownViewMode : undefined"
        @close="searchOpen = false"
        @jump="scrollToLine"
      />

      <ProjectDialog
        :open="projectDialogOpen"
        @close="projectDialogOpen = false"
      />

      <FileDetailsDialog
        :file="currentFile"
        :open="detailsOpen"
        @close="detailsOpen = false"
      />

      <!-- Bottom dock -->
      <div
        v-if="isAuthenticated"
        class="bottom-dock"
        @touchstart="swipeHandlers.handleTouchStart"
        @touchmove="swipeHandlers.handleTouchMove"
        @touchend="swipeHandlers.handleTouchEnd"
      >
        <button class="dock-btn" :class="{ active: chatOpen }" @click.stop="openDrawer('chat')" title="AI 对话">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
          </svg>
          <span v-if="chatUnread" class="dock-badge"></span>
        </button>
        <button class="dock-btn" :class="{ active: sidebarOpen }" @click.stop="openDrawer('sidebar')" title="文件管理器">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
          </svg>
        </button>
        <button class="dock-btn" :class="{ active: projectHistoryOpen || fileHistoryOpen }" @click.stop="toggleHistoryDrawer" title="历史">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="6" y1="3" x2="6" y2="15"/>
            <circle cx="18" cy="6" r="3"/>
            <circle cx="6" cy="18" r="3"/>
            <path d="M15 6a9 9 0 0 0-9 9V3"/>
          </svg>
        </button>
        <button class="dock-btn" @click.stop="handleRefresh" title="刷新">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="23,4 23,10 17,10"/>
            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
          </svg>
        </button>
      </div>
    </div>

    <!-- Toast - always rendered regardless of auth state -->
    <ToastNotification :toast="toast" />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, provide, nextTick } from 'vue'
import AppHeader from './components/AppHeader.vue'
import FileManager from './components/FileManager.vue'
import WelcomeView from './components/WelcomeView.vue'
import FileViewer from './components/FileViewer.vue'
import Lightbox from './components/Lightbox.vue'
import ChatPanel from './components/ChatPanel.vue'
import ProjectDialog from './components/ProjectDialog.vue'
import LoginView from './components/LoginView.vue'
import TocDrawer from './components/TocDrawer.vue'
import FileDetailsDialog from './components/FileDetailsDialog.vue'
import GitHistoryDrawer from './components/GitHistoryDrawer.vue'
import SearchDrawer from './components/SearchDrawer.vue'
import ToastNotification from './components/ToastNotification.vue'
import { useToast } from './composables/useToast.ts'
import { useSwipeNavigation } from './composables/useSwipeNavigation.ts'
import { store } from './stores/app.ts'
import { initMermaid, reRenderMermaid, getFileType } from './utils/helpers.ts'
import 'highlight.js/styles/github.css'
import 'highlight.js/styles/github-dark.css'
import './assets/hljs-light-override.css'

// Auth
const isAuthenticated = ref(null)


// Git history drawers
const projectHistoryOpen = ref(false)
const fileHistoryOpen = ref(false)

// File details dialog
const detailsOpen = ref(false)

const searchOpen = ref(false)

// Markdown view mode (lifted from FileViewer so SearchDrawer can access it)
const markdownViewMode = ref('rendered')

// Chat
const chatOpen = ref(false)
const initialChatTab = ref(null)
const chatUnread = ref(false)

// Global toast
const toast = useToast()
provide('toast', toast)


// TOC state
const tocOpen = ref(false)

// Sidebar state (文件管理器)
const sidebarOpen = ref(false)
const showHidden = ref(JSON.parse(localStorage.getItem('clawbenchShowHidden') || 'false'))
const sortField = ref(null)
const sortDir = ref('asc')

// 抽屉互斥：打开一个时关闭其他（瞬间关闭，无动画）
const drawerStates = {
  chat: chatOpen,
  sidebar: sidebarOpen,
  projectHistory: projectHistoryOpen,
  fileHistory: fileHistoryOpen,
  toc: tocOpen,
  search: searchOpen,
  details: detailsOpen,
}

function openDrawer(name, tab = null) {
  // 如果已打开，则关闭
  if (drawerStates[name].value) {
    drawerStates[name].value = false
    return
  }
  // 清除聊天未读角标
  if (name === 'chat') chatUnread.value = false
  // 关闭其他抽屉
  Object.entries(drawerStates).forEach(([key, ref]) => {
    if (key !== name && ref.value) {
      ref.value = false
    }
  })
  // 如果是 chat 抽屉，设置 initial tab
  if (name === 'chat' && tab) {
    initialChatTab.value = tab
  } else {
    initialChatTab.value = null
  }
  // 打开目标抽屉
  drawerStates[name].value = true
}

function toggleHistoryDrawer() {
  // 如果任一历史抽屉打开，关闭它
  if (projectHistoryOpen.value || fileHistoryOpen.value) {
    projectHistoryOpen.value = false
    fileHistoryOpen.value = false
  } else {
    openDrawer('projectHistory')
  }
}

async function handleLoginSuccess() {
    isAuthenticated.value = true
    initMermaid()
    await store.loadProject()
    await store.loadFiles('')
}

// Project dialog
const projectDialogOpen = ref(false)

function handleOpenProjectDialog() {
    projectDialogOpen.value = true
}

// Theme
const theme = ref(localStorage.getItem('theme') ||
    (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'))

// Sync sidebar state from store
const dirEntries = computed(() => store.state.dirEntries)
const currentDir = computed(() => store.state.currentDir)
const currentFile = computed(() => store.state.currentFile)
const currentFileIsMarkdown = computed(() => {
    const f = currentFile.value
    if (!f) return false
    const ft = getFileType(f.name)
    return ft?.isMarkdown || false
})
const projectRoot = computed(() => store.state.projectRoot)

// These must be defined after currentFile since they reference it
const tocFile = computed(() => {
    const f = currentFile.value
    if (!f || f.isImage || f.isPdf || f.isAudio || !f.content) return null
    const ft = getFileType(f.name)
    if (ft.isImage || ft.isAudio) return null
    return f
})


const tocFabVisible = computed(() => !!tocFile.value)

// File history navigation
const canGoBack = computed(() => store.canNavigateBack())
const canGoForward = computed(() => store.canNavigateForward())

function handleGoBack() {
    if (canGoBack.value) store.navigateToPrevFile()
}

function handleGoForward() {
    if (canGoForward.value) store.navigateToNextFile()
}

// Close dialogs when file changes
watch(() => currentFile.value, (f) => {
    tocOpen.value = false
    detailsOpen.value = false
    markdownViewMode.value = 'rendered'
})

function toggleHidden() {
    showHidden.value = !showHidden.value
    localStorage.setItem('clawbenchShowHidden', JSON.stringify(showHidden.value))
    store.loadFiles(store.state.currentDir)
}

function handleToggleSort(field) {
    if (sortField.value === field) {
        sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    } else {
        sortField.value = field
        sortDir.value = 'asc'
    }
}

async function handleNavigateDir(path) {
    await store.navigateToDir(path)
}

async function handleSelectFile(path) {
    await store.selectFile(path)
}

async function handleRename({ path, name }) {
    await store.renameFile(path, name)
}

async function handleDelete(path) {
    await store.deleteFile(path)
}

function handleChatMessage() {
    handleRefresh()
    if (!chatOpen.value) chatUnread.value = true
}

async function handleRefresh() {
    sortField.value = null
    sortDir.value = 'asc'
    await store.loadFiles(currentDir.value)
    if (currentFile.value) {
        await store.selectFile(currentFile.value.path)
        if (store.state.currentFile?.error) {
            store.state.currentFile = null
        }
    }
}

function scrollToLine(line) {
    nextTick(() => {
        const el = document.querySelector(`.code-line[data-line="${line}"]`)
        if (!el) return
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
        el.classList.add('line-flash')
        el.addEventListener('animationend', () => el.classList.remove('line-flash'), { once: true })
    })
}

function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    applyTheme(theme.value)
}

function applyTheme(t) {
    document.documentElement.setAttribute('data-theme', t)
    localStorage.setItem('theme', t)
    // Toggle highlight.js theme via attribute selector (both CSS files are bundled)
    document.documentElement.setAttribute('data-hljs-theme', t)
    initMermaid()
    reRenderMermaid()
}

provide('theme', theme)
provide('applyTheme', applyTheme)

// Swipe navigation for bottom dock
const swipeHandlers = useSwipeNavigation(
  () => store.navigateToPrevFile(),
  () => store.navigateToNextFile(),
  50
)

const contentSwipeStyle = computed(() => {
  const offset = swipeHandlers.swipeOffset.value
  if (offset === 0 && !swipeHandlers.settling.value) return {}
  const transition = swipeHandlers.settling.value ? 'transform 0.25s ease-out' : 'none'
  return {
    transform: `translateX(${offset}px)`,
    transition,
  }
})

function handleOpenSidebar() {
    openDrawer('sidebar')
}

onMounted(async () => {
    window.addEventListener('open-sidebar', handleOpenSidebar)
    applyTheme(theme.value)
    let resp
    try {
        resp = await fetch('/api/me')
    } catch (_) {
        isAuthenticated.value = false
        toast.show('无法连接到服务器，请检查后端服务是否启动', { icon: '⚠️', duration: 0, onClick: () => location.reload() })
        return
    }
    if (resp.ok) {
        isAuthenticated.value = true
    } else if (resp.status === 401 || resp.status === 403) {
        isAuthenticated.value = false
        return
    } else {
        isAuthenticated.value = false
        toast.show('服务器响应异常，后端服务可能未正确启动', { icon: '⚠️', duration: 0, onClick: () => location.reload() })
        return
    }
    initMermaid()
    // Check unread chat messages on startup
    try {
        const sr = await fetch('/api/ai/sessions')
        if (sr.ok) {
            const sd = await sr.json()
            if (sd.sessions?.some(s => s.unreadCount > 0)) {
                chatUnread.value = true
            }
        }
    } catch (_) {}
    try {
        await store.loadProject()
    } catch (_) {
        toast.show('项目加载失败，后端服务可能未正确启动', { icon: '⚠️', duration: 0, onClick: () => location.reload() })
        return
    }
    try {
        await store.loadFiles('')
    } catch (_) {
        toast.show('文件列表加载失败', { icon: '⚠️', duration: 6000 })
    }
    const lastFile = localStorage.getItem('clawbenchLastFile_' + store.state.projectRoot)
    if (lastFile && lastFile !== store.state.currentFile?.path) {
        const lastSlash = lastFile.lastIndexOf('/')
        store.state.currentDir = lastSlash > 0 ? lastFile.slice(0, lastSlash) : ''
        await store.loadFiles(store.state.currentDir)
        await store.selectFile(lastFile)
        if (store.state.currentFile?.error) {
            store.state.currentFile = null
        }
    }
})

onUnmounted(() => {
    window.removeEventListener('open-sidebar', handleOpenSidebar)
})
</script>

<style scoped>
.bottom-dock {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    padding: 6px 16px;
    padding-bottom: calc(6px + env(safe-area-inset-bottom, 0));
    background: var(--bg-primary);
    border-top: 1px solid var(--border-color);
    -webkit-tap-highlight-color: transparent;
    user-select: none;
}

.dock-btn {
    position: relative;
    width: 40px;
    height: 40px;
    border: none;
    border-radius: 12px;
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.2s, color 0.2s, transform 0.15s;
}

.dock-btn:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
}

.dock-btn:active {
    transform: scale(0.92);
}

.dock-btn.active {
    background: var(--accent-color);
    color: #fff;
}

.dock-btn.active:hover {
    background: var(--accent-hover);
    color: #fff;
}

.dock-btn svg {
    width: 18px;
    height: 18px;
}

.dock-btn.disabled {
    opacity: 0.3;
    cursor: default;
    pointer-events: none;
}

.dock-badge {
    position: absolute;
    top: 4px;
    right: 4px;
    width: 8px;
    height: 8px;
    background: var(--danger-color, #dc3545);
    border-radius: 50%;
    pointer-events: none;
    animation: badge-pulse 2s ease-in-out infinite;
}

@keyframes badge-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
}
</style>

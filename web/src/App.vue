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
        :open="fileManagerOpen"
        :show-hidden="showHidden"
        :sort-field="sortField"
        :sort-dir="sortDir"
        @close="fileManagerOpen = false"
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

      <PortForwardBrowser v-if="isAppMode" ref="portForwardBrowserRef" />

      <ChatPanel
        :open="chatOpen"
        :current-file="currentFile"
        @close="chatOpen = false"
        @open="chatOpen = true"
        @message="handleChatMessage()"
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

      <ProxyPanel
        v-if="isAppMode"
        :open="proxyOpen"
        @close="proxyOpen = false"
      />

      <!-- Quote question floating bar — uses session identity singleton -->
      <QuoteQuestionBar
        :visible="quoteQuestion.visible.value"
        :quoteData="quoteQuestion.quoteData.value"
        :sessionLabel="sessionIdentity.agentHeaderTitle.value"
        :sessionTitle="sessionIdentity.currentSessionTitle.value"
        :currentSessionId="sessionIdentity.currentSessionId.value"
        @send="quoteQuestion.sendMessage($event, sessionIdentity.currentSessionId.value)"
        @close="quoteQuestion.closeSheet()"
        @pin="quoteQuestion.pinBar()"
        @unpin="quoteQuestion.unpinBar()"
        @open-sessions="handleQuoteOpenSessions"
      />

      <!-- Session drawer for quote-question session switching -->
      <SessionDrawer
        :open="quoteSessionDrawerOpen"
        :currentSessionId="sessionIdentity.currentSessionId.value"
        :runningSessionIds="sessionIdentity.runningSessions.value"
        @close="quoteSessionDrawerOpen = false"
        @select="handleQuoteSessionSelect"
        @create="handleQuoteSessionCreate"
        @delete="handleQuoteSessionDelete"
      />

      <!-- Bottom dock -->
      <div v-if="isAuthenticated" class="bottom-dock-wrapper">
        <div class="bottom-dock">
          <button
            class="dock-nav-btn"
            :class="{ disabled: !canGoBack }"
            @click.stop="handleGoBack"
            title="上一个文件"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="15 18 9 12 15 6"/>
            </svg>
          </button>

          <div class="dock-center">
            <button class="dock-btn" :class="{ active: chatOpen, 'has-unread': store.state.chatUnread && !chatOpen, 'has-running': store.state.chatRunning && !chatOpen && !store.state.chatUnread }" @click.stop="openDrawer('chat')" title="会话">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
              </svg>
            </button>
            <button class="dock-btn" :class="{ active: fileManagerOpen }" @click.stop="openDrawer('fileManager')" title="文件管理器">
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
            <button v-if="isAppMode" class="dock-btn" :class="{ active: proxyOpen }" @click.stop="openDrawer('proxy')" title="端口转发">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 2L2 7l10 5 10-5-10-5z"/>
                <path d="M2 17l10 5 10-5"/>
                <path d="M2 12l10 5 10-5"/>
              </svg>
            </button>
            <button class="dock-btn" @click.stop="handleRefresh" title="刷新">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="23,4 23,10 17,10"/>
                <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
              </svg>
            </button>
          </div>

          <button
            class="dock-nav-btn"
            :class="{ disabled: !canGoForward }"
            @click.stop="handleGoForward"
            title="下一个文件"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6"/>
            </svg>
          </button>
        </div>
        <div class="dock-safe-area"></div>
      </div>
    </div>

    <!-- Toast - always rendered regardless of auth state -->
    <ToastNotification :toast="toast" />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, provide, nextTick } from 'vue'
import AppHeader from './components/common/AppHeader.vue'
import FileManager from './components/file/FileManager.vue'
import WelcomeView from './components/WelcomeView.vue'
import FileViewer from './components/file/FileViewer.vue'
import Lightbox from './components/media/Lightbox.vue'
import ChatPanel from './components/chat/ChatPanel.vue'
import ProjectDialog from './components/ProjectDialog.vue'
import LoginView from './components/LoginView.vue'
import TocDrawer from './components/TocDrawer.vue'
import FileDetailsDialog from './components/file/FileDetailsDialog.vue'
import GitHistoryDrawer from './components/git/GitHistoryDrawer.vue'
import SearchDrawer from './components/common/SearchDrawer.vue'
import ToastNotification from './components/common/ToastNotification.vue'
import SessionDrawer from './components/session/SessionDrawer.vue'
import ProxyPanel from './components/proxy/ProxyPanel.vue'
import PortForwardBrowser from './components/proxy/PortForwardBrowser.vue'
import QuoteQuestionBar from './components/common/QuoteQuestionBar.vue'
import { useQuoteQuestion } from './composables/useQuoteQuestion.ts'
import { useSessionIdentity } from './composables/useSessionIdentity.ts'
import { useToast } from './composables/useToast.ts'
import { useAppMode } from './composables/useAppMode.ts'
import { usePortForward, setOpenPortBrowser } from './composables/usePortForward.ts'
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

// Global toast
const toast = useToast()
provide('toast', toast)

// Session identity singleton — single source of truth for session state
const sessionIdentity = useSessionIdentity()

// TOC state
const tocOpen = ref(false)

// FileManager state
const fileManagerOpen = ref(false)
const showHidden = ref(JSON.parse(localStorage.getItem('clawbenchShowHidden') || 'false'))
const sortField = ref(null)
const sortDir = ref('asc')

// App mode & port forwarding
const { isAppMode } = useAppMode()
const { syncToNative } = usePortForward()
const proxyOpen = ref(false)

// Quote question feature
const quoteQuestion = useQuoteQuestion()
const quoteSessionDrawerOpen = ref(false)

// Open session drawer directly when user clicks session info in QuoteQuestionBar
function handleQuoteOpenSessions() {
  quoteSessionDrawerOpen.value = true
}

function handleQuoteSessionSelect(sessionId) {
  sessionIdentity.switchSession(sessionId)
  quoteSessionDrawerOpen.value = false
}

function handleQuoteSessionCreate(agentId) {
  sessionIdentity.createSession(agentId)
  quoteSessionDrawerOpen.value = false
}

function handleQuoteSessionDelete(sessionId, backend) {
  sessionIdentity.deleteSession(sessionId, backend)
}

// 抽屉互斥：打开一个时关闭其他（瞬间关闭，无动画）
const drawerStates = {
  chat: chatOpen,
  fileManager: fileManagerOpen,
  projectHistory: projectHistoryOpen,
  fileHistory: fileHistoryOpen,
  toc: tocOpen,
  search: searchOpen,
  details: detailsOpen,
  proxy: proxyOpen,
}

function openDrawer(name, tab = null) {
  // 如果已打开，则关闭
  if (drawerStates[name].value) {
    drawerStates[name].value = false
    return
  }
  // 清除聊天未读角标
  if (name === 'chat') store.state.chatUnread = false
  // 关闭其他抽屉
  Object.entries(drawerStates).forEach(([key, ref]) => {
    if (key !== name && ref.value) {
      ref.value = false
    }
  })
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

// Sync fileManager state from store
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
    if (!chatOpen.value) store.state.chatUnread = true
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

function handleOpenFileManager() {
    openDrawer('fileManager')
}

onMounted(async () => {
    window.addEventListener('open-file-manager', handleOpenFileManager)
    applyTheme(theme.value)
    let resp
    try {
        resp = await fetch('/api/me')
    } catch (_) {
        isAuthenticated.value = false
        if (isAppMode.value && window.AndroidNative?.showServerDialog) {
            toast.show('无法连接到服务器，点击重新配置', {
                icon: '⚠️', type: 'error', duration: 0,
                onClick: () => window.AndroidNative.showServerDialog()
            })
        } else {
            toast.show('无法连接到服务器，请检查后端服务是否启动', { icon: '⚠️', type: 'error', duration: 0, onClick: () => location.reload() })
        }
        return
    }
    if (resp.ok) {
        isAuthenticated.value = true
    } else if (resp.status === 401 || resp.status === 403) {
        // Android app mode: try auto-login with saved password
        if (isAppMode.value && window.AndroidNative?.getPassword?.()) {
            const savedPwd = window.AndroidNative.getPassword()
            if (savedPwd) {
                try {
                    const loginRes = await fetch('/login', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ password: savedPwd })
                    })
                    if (loginRes.ok) {
                        isAuthenticated.value = true
                        // Re-save password for SSH tunnel (in case SharedPreferences was cleared)
                        if (window.AndroidNative?.setSSHPassword) {
                            window.AndroidNative.setSSHPassword(savedPwd)
                        }
                        // Continue with normal initialization below
                    } else {
                        // Auto-login failed (password changed), show login form
                        isAuthenticated.value = false
                        return
                    }
                } catch (_) {
                    isAuthenticated.value = false
                    return
                }
            } else {
                isAuthenticated.value = false
                return
            }
        } else {
            isAuthenticated.value = false
            return
        }
    } else {
        isAuthenticated.value = false
        toast.show('服务器响应异常，后端服务可能未正确启动', { icon: '⚠️', type: 'error', duration: 0, onClick: () => location.reload() })
        return
    }
    initMermaid()
    // Pre-fill session identity from API so QuoteQuestionBar shows correct info
    // even before ChatPanel is opened
    await sessionIdentity.initSessionFromAPI()
    // Check unread chat messages on startup
    try {
        const sr = await fetch('/api/ai/sessions')
        if (sr.ok) {
            const sd = await sr.json()
            if (sd.sessions?.some(s => s.unreadCount > 0)) {
                store.state.chatUnread = true
            }
        }
    } catch (_) {}
    // Sync port forwarding to Android native layer
    if (isAppMode.value) {
      syncToNative().catch(() => {})
    }
    try {
        await store.loadProject()
    } catch (_) {
        toast.show('项目加载失败，后端服务可能未正确启动', { icon: '⚠️', type: 'error', duration: 0, onClick: () => location.reload() })
        return
    }
    try {
        await store.loadFiles('')
    } catch (_) {
        toast.show('文件列表加载失败', { icon: '⚠️', type: 'error', duration: 6000 })
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
    window.removeEventListener('open-file-manager', handleOpenFileManager)
})
</script>

<style scoped>
.bottom-dock-wrapper {
    flex-shrink: 0;
    -webkit-tap-highlight-color: transparent;
    user-select: none;
}

.bottom-dock {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 8px;
    background: var(--bg-primary);
    border-top: 1px solid color-mix(in srgb, var(--border-color) 40%, transparent);
    border-bottom: 1px solid color-mix(in srgb, var(--border-color) 40%, transparent);
}

.dock-safe-area {
    height: env(safe-area-inset-bottom, 0px);
}

.dock-center {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 16px;
}

.dock-nav-btn {
    width: 28px;
    height: 28px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text-tertiary);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.15s, color 0.15s;
    flex-shrink: 0;
}

.dock-nav-btn:hover:not(.disabled) {
    background: var(--bg-tertiary);
    color: var(--text-secondary);
}

.dock-nav-btn:active:not(.disabled) {
    background: var(--bg-secondary);
    color: var(--text-primary);
}

.dock-nav-btn.disabled {
    opacity: 0.2;
    cursor: default;
    pointer-events: none;
}

.dock-nav-btn svg {
    width: 14px;
    height: 14px;
}

.dock-btn {
    position: relative;
    width: 34px;
    height: 34px;
    border: none;
    border-radius: 50%;
    background: var(--bg-tertiary);
    color: var(--text-secondary);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.2s, color 0.2s, transform 0.15s;
}

.dock-btn:hover {
    background: var(--bg-secondary);
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
    width: 16px;
    height: 16px;
}

.dock-btn.disabled {
    opacity: 0.3;
    cursor: default;
    pointer-events: none;
}

/* Unread indicator — fast flash on dock button */
.dock-btn.has-unread {
    animation: dock-unread-flash 0.8s ease-in-out infinite;
}

@keyframes dock-unread-flash {
    0%, 100% {
        box-shadow: 0 0 0 0 color-mix(in srgb, var(--accent-color, #0066cc) 0%, transparent);
    }
    50% {
        box-shadow: 0 0 8px 3px color-mix(in srgb, var(--accent-color, #0066cc) 40%, transparent);
    }
}

/* Running indicator — spinning border light on white glow */
.dock-btn.has-running {
    position: relative;
    isolation: isolate;
    overflow: hidden;
    border-color: transparent;
    box-shadow: 0 0 6px 2px rgba(255, 255, 255, 0.2);
}
.dock-btn.has-running::before {
    content: '';
    position: absolute;
    inset: -2px;
    border-radius: inherit;
    background: conic-gradient(from 0deg, transparent 0%, rgba(255,255,255,0.6) 10%, var(--accent-color, #0066cc) 22%, rgba(255,255,255,0.4) 34%, transparent 50%);
    animation: dock-spin-light 1.2s linear infinite;
    z-index: -2;
}
.dock-btn.has-running::after {
    content: '';
    position: absolute;
    inset: 1.5px;
    border-radius: inherit;
    background: var(--bg-tertiary);
    z-index: -1;
}

@keyframes dock-spin-light {
    to { transform: rotate(360deg); }
}
</style>

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

      <main class="main-content">
        <div class="content-area" id="contentArea">
          <!-- Chat Tab -->
          <TabPanel tabId="chat" :activeTab="activeTab">
            <template #header>
              <MessageSquare :size="16" class="bs-header-icon" />
              <span class="bs-header-title">{{ sessionIdentity.agentHeaderTitle.value }}</span>
              <div v-if="sessionIdentity.currentSessionTitle.value" class="bs-header-description">
                <HeaderMarquee :text="sessionIdentity.currentSessionTitle.value">{{ sessionIdentity.currentSessionTitle.value }}</HeaderMarquee>
              </div>
            </template>
            <ChatPanelContent
              :active="activeTab === 'chat'"
              :current-file="currentFile"
              :current-dir="currentDir"
              @open="switchTab('chat')"
              @open-file="handleSelectFile"
              @task-card-click="onTaskCardClick"
            />
          </TabPanel>

          <!-- File Browse Tab -->
          <TabPanel tabId="browse" :activeTab="activeTab">
            <template #header>
              <Folder :size="16" class="bs-header-icon" />
              <span class="bs-header-title">{{ t('file.manager') }}</span>
            </template>
            <FileManagerContent
              :entries="dirEntries"
              :current-dir="currentDir"
              :current-file="currentFile"
              :show-hidden="showHidden"
              :sort-field="sortField"
              :sort-dir="sortDir"
              :dir-loading="store.state.dirLoading"
              @navigate-dir="handleNavigateDir"
              @select-file="handleBrowseSelectFile"
              @toggle-sort="handleToggleSort"
              @toggle-hidden="toggleHidden"
              @rename="handleRename"
              @delete="handleDelete"
              @batch-delete="handleBatchDelete"
              @refresh="handleRefresh"
              @open-terminal="handleOpenTerminal"
            />
          </TabPanel>

          <!-- File Viewer Tab -->
          <TabPanel tabId="viewer" :activeTab="activeTab" :noHeader="true">
            <div class="viewer-panel">
              <WelcomeView v-if="!currentFile" />
              <FileViewer
                v-if="currentFile"
                ref="fileViewerRef"
                :file="currentFile"
                :toc-open="tocOpen"
                :search-open="searchOpen"
                :markdown-view-mode="markdownViewMode"
                @delete="handleDelete(currentFile?.path)"
                @show-details="detailsOpen = true"
                @open-git-history="openFileHistory"
                @toggle-toc="tocOpen = !tocOpen"
                @toggle-search="currentFile?.content && (searchOpen = !searchOpen)"
                @toggle-view="markdownViewMode = markdownViewMode === 'rendered' ? 'raw' : 'rendered'"
                @refresh="handleRefresh"
              />
            </div>
            <!-- Auxiliary overlays for viewer tab — open only when viewer tab is active -->
            <TocDrawer
              :file="tocFile"
              :pdf-outline="pdfOutline"
              :open="activeTab === 'viewer' && tocOpen"
              @close="tocOpen = false"
              @jump="scrollToLine"
              @jump-page="handleJumpPdfPage"
            />
            <SearchDrawer
              :file="currentFile"
              :open="activeTab === 'viewer' && searchOpen"
              :view-mode="currentFileIsMarkdown ? markdownViewMode : undefined"
              @close="searchOpen = false"
              @jump="scrollToLine"
            />
            <GitHistoryDrawer
              :open="activeTab === 'viewer' && fileHistoryOpen"
              mode="file"
              :file="currentFile"
              @close="fileHistoryOpen = false"
              @open-file="handleSelectFile"
            />
          </TabPanel>

          <!-- History Tab -->
          <TabPanel tabId="history" :activeTab="activeTab">
            <template #header>
              <GitBranch :size="16" class="bs-header-icon" />
              <span class="bs-header-title">{{ t('git.history.projectHistory') }}</span>
            </template>
            <GitHistoryContent
              mode="project"
              @open-file="handleSelectFile"
            />
          </TabPanel>

          <!-- Proxy Tab -->
          <TabPanel tabId="proxy" :activeTab="activeTab">
            <template #header>
              <EthernetPort :size="16" class="bs-header-icon" />
              <span class="bs-header-title">{{ t('proxy.title') }}</span>
            </template>
            <ProxyPanelContent />
          </TabPanel>

          <!-- Terminal Tab -->
          <TabPanel tabId="terminal" :activeTab="activeTab" :noHeader="true">
            <TerminalPanelContent
              :requested-cwd="terminalRequestedCwd"
              :active="activeTab === 'terminal'"
            />
          </TabPanel>

          <!-- Tasks Tab -->
          <TabPanel tabId="tasks" :activeTab="activeTab">
            <template #header>
              <Clock :size="16" class="bs-header-icon" />
              <span class="bs-header-title">{{ t('nav.tasks') }}</span>
            </template>
            <TaskTab :active="activeTab === 'tasks'" @open-file="handleTaskOpenFile" />
          </TabPanel>
        </div>
      </main>

      <Lightbox />

      <ProjectDialog
        :open="projectDialogOpen"
        @close="projectDialogOpen = false"
      />

      <FileDetailsDialog
        :file="currentFile"
        :open="activeTab === 'viewer' && detailsOpen"
        @close="detailsOpen = false"
      />

      <!-- Quote question floating bar -->
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

      <!-- Bottom dock (tab bar) -->
      <div v-if="isAuthenticated" class="bottom-dock-wrapper">
        <div class="bottom-dock">
          <div class="dock-center">
            <button class="dock-btn" :class="{ active: activeTab === 'chat', 'has-unread': store.state.chatUnread && activeTab !== 'chat', 'has-running': store.state.chatRunning && activeTab !== 'chat' && !store.state.chatUnread }" @click.stop="switchTab('chat')" :title="t('nav.chat')">
              <MessageSquare />
            </button>
            <button class="dock-btn" :class="{ active: activeTab === 'browse' }" @click.stop="switchTab('browse')" :title="t('nav.fileManager')">
              <FolderOpen />
            </button>
            <button class="dock-btn" :class="{ active: activeTab === 'viewer' }" @click.stop="switchTab('viewer')" :title="t('nav.fileViewer')">
              <FileText />
            </button>
            <button class="dock-btn" :class="{ active: activeTab === 'history' }" @click.stop="switchTab('history')" :title="t('nav.history')">
              <GitBranch />
            </button>
            <button class="dock-btn" :class="{ active: activeTab === 'tasks', 'has-unread': store.state.taskUnread && activeTab !== 'tasks' }" @click.stop="switchTab('tasks')" :title="t('nav.tasks')">
              <Clock />
            </button>
            <button class="dock-btn" :class="{ active: activeTab === 'proxy' }" @click.stop="switchTab('proxy')" :title="t('nav.portForward')">
              <EthernetPort />
            </button>
            <button class="dock-btn" :class="{ active: activeTab === 'terminal' }" @click.stop="handleDockTerminal" :title="t('terminal.title')">
              <TerminalIcon />
            </button>
          </div>
        </div>
        <div class="dock-safe-area"></div>
      </div>
    </div>

    <ToastNotification :toast="toast" />
    <DialogOverlay />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, provide, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessageSquare, Folder, FolderOpen, FileText, GitBranch, EthernetPort, Terminal as TerminalIcon, Clock } from 'lucide-vue-next'
import AppHeader from './components/common/AppHeader.vue'
import TabPanel from './components/common/TabPanel.vue'
import WelcomeView from './components/WelcomeView.vue'
import FileViewer from './components/file/FileViewer.vue'
import Lightbox from './components/media/Lightbox.vue'
import ChatPanelContent from './components/chat/ChatPanelContent.vue'
import FileManagerContent from './components/file/FileManagerContent.vue'
import GitHistoryContent from './components/git/GitHistoryContent.vue'
import ProxyPanelContent from './components/proxy/ProxyPanelContent.vue'
import TerminalPanelContent from './components/terminal/TerminalPanelContent.vue'
import ProjectDialog from './components/ProjectDialog.vue'
import LoginView from './components/LoginView.vue'
import TocDrawer from './components/TocDrawer.vue'
import FileDetailsDialog from './components/file/FileDetailsDialog.vue'
import GitHistoryDrawer from './components/git/GitHistoryDrawer.vue'
import SearchDrawer from './components/common/SearchDrawer.vue'
import ToastNotification from './components/common/ToastNotification.vue'
import DialogOverlay from './components/common/DialogOverlay.vue'
import SessionDrawer from './components/session/SessionDrawer.vue'
import QuoteQuestionBar from './components/common/QuoteQuestionBar.vue'
import HeaderMarquee from './components/common/HeaderMarquee.vue'
import TaskTab from '@/components/task/TaskTab.vue'
import { useQuoteQuestion } from './composables/useQuoteQuestion.ts'
import { useTaskTab } from '@/composables/useTaskTab.ts'
import { useSessionIdentity } from './composables/useSessionIdentity.ts'
import { useToast } from './composables/useToast.ts'
import { useAppMode } from './composables/useAppMode.ts'
import { usePortForward } from './composables/usePortForward.ts'
import { useFileWatch } from './composables/useFileWatch.ts'
import { refreshCurrentFile } from './composables/useFileRefresh.ts'
import { store } from './stores/app.ts'
import { initMermaid, reRenderMermaid } from './utils/mermaid.ts'
import { getFileType } from './utils/fileType.ts'
import 'highlight.js/styles/github.css'
import 'highlight.js/styles/github-dark.css'
import './assets/hljs-light-override.css'

const isAuthenticated = ref(null)
const { t } = useI18n()

const activeTab = ref('chat')

function switchTab(tab) {
  if (activeTab.value === tab) return
  activeTab.value = tab
  if (tab === 'chat') {
    store.state.chatUnread = false
  }
  if (tab === 'tasks') {
    store.state.taskUnread = false
  }
}

const detailsOpen = ref(false)
const tocOpen = ref(false)
const searchOpen = ref(false)
const fileHistoryOpen = ref(false)

function openFileHistory() {
  fileHistoryOpen.value = true
}

const markdownViewMode = ref('rendered')

const toast = useToast()
provide('toast', toast)

const sessionIdentity = useSessionIdentity()

const showHidden = ref(JSON.parse(localStorage.getItem('clawbenchShowHidden') || 'false'))
const sortField = ref(null)
const sortDir = ref('asc')

useFileWatch({
  fileManagerOpen: computed(() => activeTab.value === 'browse'),
  currentDir: computed(() => store.state.currentDir),
  currentFile: computed(() => store.state.currentFile),
})

const { isAppMode } = useAppMode()
const { syncToNative } = usePortForward()
const { startTaskPolling, stopTaskPolling, navigateToTaskSettings } = useTaskTab()
const terminalRequestedCwd = ref(null)

const quoteQuestion = useQuoteQuestion()
const quoteSessionDrawerOpen = ref(false)

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

async function handleLoginSuccess() {
    isAuthenticated.value = true
    initMermaid()
    await store.loadProject()
    await store.loadFiles('')
}

const projectDialogOpen = ref(false)

function handleOpenProjectDialog() {
    projectDialogOpen.value = true
}

const theme = ref(localStorage.getItem('theme') ||
    (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'))

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

const tocFile = computed(() => {
    const f = currentFile.value
    if (!f || f.isImage || f.isAudio) return null
    // PDF: pass file even without content (outline comes from pdfOutline prop)
    if (f.isPdf) return f
    if (!f.content) return null
    const ft = getFileType(f.name)
    if (ft.isImage || ft.isAudio) return null
    return f
})

// PDF TOC integration
const fileViewerRef = ref(null)
const pdfOutline = computed(() => fileViewerRef.value?.pdfOutline || [])
function handleJumpPdfPage(pageNum) {
    fileViewerRef.value?.pdfScrollToPage(pageNum)
}

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
        if (sortDir.value === 'asc') {
            sortDir.value = 'desc'
        } else {
            sortField.value = null
            sortDir.value = 'asc'
        }
    } else {
        sortField.value = field
        sortDir.value = 'asc'
    }
}

async function handleNavigateDir(path) {
    if (store.state.dirLoading) return
    await store.navigateToDir(path)
}

async function handleSelectFile(path) {
    await store.selectFile(path)
}

async function handleBrowseSelectFile(path) {
    await store.selectFile(path)
    activeTab.value = 'viewer'
}

async function handleTaskOpenFile(filePath) {
    await store.selectFile(filePath)
    switchTab('viewer')
}

function onTaskCardClick(taskId) {
    navigateToTaskSettings(taskId)
    switchTab('tasks')
}

async function handleRename({ path, name }) {
    await store.renameFile(path, name)
}

async function handleDelete(path) {
    await store.deleteFile(path)
}

async function handleBatchDelete(paths) {
    await store.deleteFiles(paths)
}

async function handleRefresh() {
    await refreshCurrentFile({ loadDir: true, clearOnError: true })
}

function handleDockTerminal() {
    terminalRequestedCwd.value = null
    switchTab('terminal')
}

function handleOpenTerminal(cwd) {
    terminalRequestedCwd.value = cwd || null
    activeTab.value = 'terminal'
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
    document.documentElement.setAttribute('data-hljs-theme', t)
    initMermaid()
    reRenderMermaid()
}

provide('theme', theme)
provide('applyTheme', applyTheme)
provide('activeTab', activeTab)
provide('switchTab', switchTab)

function handleOpenFileManager() {
    activeTab.value = 'browse'
}

function playQuoteEmitAnimation(e) {
  const { from, to } = e?.detail ?? {}
  if (!from || !to) return
  const x0 = from.x, y0 = from.y, x1 = to.x, y1 = to.y
  const mx = (x0 + x1) / 2
  const my = Math.min(y0, y1) - 30
  const dot = document.createElement('div')
  dot.className = 'quote-emit-dot'
  dot.style.cssText = `
    position: fixed; width: 8px; height: 8px; border-radius: 50%;
    background: var(--accent-color, #0066cc);
    box-shadow: 0 0 10px 3px color-mix(in srgb, var(--accent-color, #0066cc) 50%, transparent);
    z-index: 9999; pointer-events: none; left: 0; top: 0; will-change: transform, opacity;
  `
  document.body.appendChild(dot)
  const duration = 420, start = performance.now()
  function animate(now) {
    const t = Math.min((now - start) / duration, 1)
    const ease = 1 - Math.pow(1 - t, 3)
    const x = (1 - ease) ** 2 * x0 + 2 * (1 - ease) * ease * mx + ease ** 2 * x1
    const y = (1 - ease) ** 2 * y0 + 2 * (1 - ease) * ease * my + ease ** 2 * y1
    const scale = t < 0.1 ? t / 0.1 : t > 0.85 ? 1 - (t - 0.85) / 0.15 : 1
    const opacity = t < 0.08 ? t / 0.08 : t > 0.7 ? 1 - (t - 0.7) / 0.3 : 1
    dot.style.transform = `translate(${x - 4}px, ${y - 4}px) scale(${scale})`
    dot.style.opacity = opacity
    if (t < 1) requestAnimationFrame(animate)
    else {
      dot.remove()
      const chatDockBtn = document.querySelector('.dock-center')?.querySelector('.dock-btn')
      if (chatDockBtn) {
        chatDockBtn.classList.add('quote-emit-receive')
        chatDockBtn.addEventListener('animationend', () => chatDockBtn.classList.remove('quote-emit-receive'), { once: true })
      }
    }
  }
  requestAnimationFrame(animate)
}

onMounted(async () => {
    startTaskPolling()
    window.addEventListener('open-file-manager', handleOpenFileManager)
    window.addEventListener('quote-sent', playQuoteEmitAnimation)
    applyTheme(theme.value)
    let resp
    try {
        resp = await fetch('/api/me')
    } catch (_) {
        isAuthenticated.value = false
        if (isAppMode.value && window.AndroidNative?.showServerDialog) {
            toast.show(t('toast.serverUnreachableApp'), { icon: '⚠️', type: 'error', duration: 0, onClick: () => window.AndroidNative.showServerDialog() })
        } else {
            toast.show(t('toast.serverUnreachableWeb'), { icon: '⚠️', type: 'error', duration: 0, onClick: () => location.reload() })
        }
        return
    }
    if (resp.ok) {
        isAuthenticated.value = true
    } else if (resp.status === 401 || resp.status === 403) {
        if (isAppMode.value && window.AndroidNative?.getPassword?.()) {
            const savedPwd = window.AndroidNative.getPassword()
            if (savedPwd) {
                try {
                    const loginRes = await fetch('/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ password: savedPwd }) })
                    if (loginRes.ok) {
                        isAuthenticated.value = true
                        if (window.AndroidNative?.setSSHPassword) window.AndroidNative.setSSHPassword(savedPwd)
                    } else { isAuthenticated.value = false; return }
                } catch (_) { isAuthenticated.value = false; return }
            } else { isAuthenticated.value = false; return }
        } else { isAuthenticated.value = false; return }
    } else {
        isAuthenticated.value = false
        toast.show(t('toast.serverError'), { icon: '⚠️', type: 'error', duration: 0, onClick: () => location.reload() })
        return
    }
    initMermaid()
    await sessionIdentity.initSessionFromAPI()
    try {
        const sr = await fetch('/api/ai/sessions')
        if (sr.ok) { const sd = await sr.json(); if (sd.sessions?.some(s => s.unreadCount > 0)) store.state.chatUnread = true }
    } catch (_) {}
    if (isAppMode.value) syncToNative().catch(() => {})
    try { await store.loadProject() } catch (_) {
        toast.show(t('toast.projectLoadFailed'), { icon: '⚠️', type: 'error', duration: 0, onClick: () => location.reload() }); return
    }
    try { await store.loadFiles('') } catch (_) {
        toast.show(t('toast.fileListLoadFailed'), { icon: '⚠️', type: 'error', duration: 6000 })
    }
    const lastFile = localStorage.getItem('clawbenchLastFile_' + store.state.projectRoot)
    if (lastFile && lastFile !== store.state.currentFile?.path) {
        const lastSlash = lastFile.lastIndexOf('/')
        store.state.currentDir = lastSlash > 0 ? lastFile.slice(0, lastSlash) : ''
        await store.loadFiles(store.state.currentDir)
        await store.selectFile(lastFile)
        if (store.state.currentFile?.error) store.state.currentFile = null
        // 不再自动跳转到 viewer，保持默认 tab（chat）
        // 用户切到 browse 时可以直接看到上次打开的文件
    }
})

onUnmounted(() => {
    stopTaskPolling()
    window.removeEventListener('open-file-manager', handleOpenFileManager)
    window.removeEventListener('quote-sent', playQuoteEmitAnimation)
})
</script>

<style scoped>
.viewer-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.bottom-dock-wrapper {
    flex-shrink: 0;
    -webkit-tap-highlight-color: transparent;
    user-select: none;
}

.bottom-dock {
    display: flex;
    align-items: center;
    justify-content: center;
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
    gap: 8px;
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
}

.dock-btn.has-unread {
    animation: dock-unread-flash 0.8s ease-in-out infinite;
}

@keyframes dock-unread-flash {
    0%, 100% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--accent-color, #0066cc) 0%, transparent); }
    50% { box-shadow: 0 0 8px 3px color-mix(in srgb, var(--accent-color, #0066cc) 40%, transparent); }
}

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

.dock-btn.quote-emit-receive {
    animation: quote-emit-pulse 0.4s ease-out;
}

@keyframes quote-emit-pulse {
    0% { transform: scale(1); box-shadow: 0 0 0 0 color-mix(in srgb, var(--accent-color, #0066cc) 60%, transparent); }
    40% { transform: scale(1.25); box-shadow: 0 0 14px 4px color-mix(in srgb, var(--accent-color, #0066cc) 40%, transparent); }
    100% { transform: scale(1); box-shadow: 0 0 0 0 color-mix(in srgb, var(--accent-color, #0066cc) 0%, transparent); }
}
</style>

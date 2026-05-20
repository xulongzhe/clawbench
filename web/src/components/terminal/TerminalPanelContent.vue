<template>
  <div class="terminal-panel" :style="panelStyle">
    <!-- Header -->
    <div class="terminal-header" @click.self="focusTerminal">
      <div class="terminal-header-left">
        <TerminalIcon :size="14" />
        <span class="terminal-title">{{ t('terminal.title') }}</span>
        <span v-if="currentCwd" class="terminal-cwd" :title="currentCwd">{{ shortCwd }}</span>
      </div>
      <div class="terminal-header-right">
        <span class="terminal-font-size" @click="applyFontSize(DEFAULT_FONT_SIZE)" :title="t('terminal.resetFontSize')">{{ fontSize }}</span>
        <span class="terminal-status-dot" :class="connectionState"></span>
      </div>
    </div>

    <!-- Terminal viewport -->
    <div ref="terminalContainer" class="terminal-container" @click.self="focusTerminal">
      <!-- Rebuild overlay -->
      <div v-if="rebuilding" class="terminal-rebuild-overlay">
        <span class="terminal-rebuild-spinner"></span>
        <span>{{ t('terminal.rebuilding') }}</span>
      </div>

      <!-- Directory mismatch overlay -->
      <div v-if="showReopenPrompt" class="terminal-error-overlay">
        <p>{{ t('terminal.directoryMismatch') }}</p>
        <div class="terminal-prompt-actions">
          <button class="terminal-reconnect-btn" @click="dismissReopenPrompt">{{ t('terminal.continueHere') }}</button>
          <button class="terminal-reconnect-btn" @click="handleRebuild">{{ t('terminal.reopenHere') }}</button>
        </div>
      </div>

      <!-- Error overlay -->
      <div v-if="showErrorOverlay" class="terminal-error-overlay">
        <p>{{ errorDisplayMessage }}</p>
        <button v-if="canReconnect" class="terminal-reconnect-btn" @click="handleReconnect">{{ t('terminal.reconnect') }}</button>
      </div>

      <!-- Gesture hint overlay -->
      <Transition name="gesture-hint">
        <div v-if="gestureHint" class="gesture-hint">{{ gestureHint }}</div>
      </Transition>

      <!-- xterm.js will mount here -->
    </div>

    <!-- Virtual key toolbar -->
    <div class="terminal-toolbar">
      <!-- Symbol bar (toggleable, above main toolbar) -->
      <div v-if="showSymbolBar" class="symbol-bar">
        <div class="symbol-bar-scroll">
          <button v-for="sym in symbolKeys" :key="sym" class="toolbar-btn btn-symbol" @click="handleSymbolClick(sym)">{{ sym }}</button>
        </div>
      </div>

      <!-- Main toolbar row -->
      <div class="main-toolbar-row">
        <button class="toolbar-btn modifier gesture-toggle" :class="{ active: gestures.enabled.value }" @click="gestures.toggle(); focusTerminal()" @contextmenu.prevent :title="t('terminal.gestures')">
          <HandIcon :size="14" />
        </button>
        <button class="toolbar-btn modifier gesture-toggle" :class="{ active: showSymbolBar }" @click="toggleSymbolBar()" @contextmenu.prevent :title="t('terminal.symbols')">
          <HashIcon :size="14" />
        </button>
        <div class="toolbar-scroll">
          <!-- Group: Modifiers -->
          <div class="key-group">
            <button v-if="!gestures.enabled.value" class="toolbar-btn btn-modifier" @click="terminalKeys.sendEscape(); focusTerminal()" title="Esc">Esc</button>
            <button v-if="!gestures.enabled.value" class="toolbar-btn btn-modifier" @click="terminalKeys.sendTab(); focusTerminal()" title="Tab">Tab</button>
            <button class="toolbar-btn btn-modifier modifier" :class="{ active: terminalKeys.activeModifiers.value.ctrl !== 'inactive', locked: terminalKeys.activeModifiers.value.ctrl === 'locked' }" @click="handleModifier('ctrl')" @contextmenu.prevent title="Ctrl">Ctl</button>
            <button class="toolbar-btn btn-modifier modifier" :class="{ active: terminalKeys.activeModifiers.value.alt !== 'inactive', locked: terminalKeys.activeModifiers.value.alt === 'locked' }" @click="handleModifier('alt')" @contextmenu.prevent title="Alt">Alt</button>
            <button class="toolbar-btn btn-modifier modifier" :class="{ active: terminalKeys.activeModifiers.value.shift !== 'inactive', locked: terminalKeys.activeModifiers.value.shift === 'locked' }" @click="handleModifier('shift')" @contextmenu.prevent title="Shift"><ShiftIcon :size="14" /></button>
          </div>
          <!-- Group: Shortcuts (Ctrl+C / Ctrl+Z) -->
          <div class="key-group">
            <button class="toolbar-btn btn-modifier shortcut" @click="terminalKeys.sendCtrlC(); focusTerminal()" title="Ctrl+C">⌃C</button>
            <button class="toolbar-btn btn-modifier shortcut" @click="terminalKeys.sendCtrlZ(); focusTerminal()" title="Ctrl+Z">⌃Z</button>
          </div>
          <!-- Group: Navigation -->
          <div class="key-group">
            <button class="toolbar-btn btn-nav" @click="terminalKeys.sendHome(); focusTerminal()" title="Home">Home</button>
            <button class="toolbar-btn btn-nav" @click="terminalKeys.sendEnd(); focusTerminal()" title="End">End</button>
            <button v-if="!gestures.enabled.value" class="toolbar-btn btn-nav" @click="terminalKeys.sendPageUp(); focusTerminal()" title="Page Up">PgUp</button>
            <button v-if="!gestures.enabled.value" class="toolbar-btn btn-nav" @click="terminalKeys.sendPageDown(); focusTerminal()" title="Page Down">PgDn</button>
          </div>
          <!-- Group: Arrow keys -->
          <div v-show="!gestures.enabled.value" class="key-group">
            <button class="toolbar-btn btn-arrow" @click="terminalKeys.sendArrowUp(); focusTerminal()" title="↑">↑</button>
            <button class="toolbar-btn btn-arrow" @click="terminalKeys.sendArrowDown(); focusTerminal()" title="↓">↓</button>
            <button class="toolbar-btn btn-arrow" @click="terminalKeys.sendArrowLeft(); focusTerminal()" title="←">←</button>
            <button class="toolbar-btn btn-arrow" @click="terminalKeys.sendArrowRight(); focusTerminal()" title="→">→</button>
          </div>
          <!-- Group: Actions -->
          <div class="key-group">
            <button ref="cmdBtnRef" class="toolbar-btn btn-action" @click="showCommands = !showCommands" :title="t('terminal.quickCommands')">
              <ZapIcon :size="14" />
            </button>
            <button class="toolbar-btn btn-action" @click="handleCopyOutput" :title="t('terminal.copyOutput')">
              <CopyIcon :size="14" />
            </button>
            <button class="toolbar-btn btn-action" @click="handleRebuild" :title="t('terminal.rebuildSession')">
              <RefreshCwIcon :size="14" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Quick commands popup -->
    <PopupMenu v-model:show="showCommands" :target-element="cmdBtnRef" :max-width="220" :max-height="280" :menu-items-count="visibleCommands.length + 1">
      <div class="quick-send-title">{{ t('terminal.quickCommands') }}</div>
      <button v-for="cmd in visibleCommands" :key="cmd.id" class="quick-send-item" @click="executeCommand(cmd)">
        {{ cmd.label }}
      </button>
      <div class="quick-send-divider" />
      <button class="quick-send-item" @click="openEditDialog">
        ⚙️ {{ t('terminal.editCommands') }}
      </button>
    </PopupMenu>

    <!-- Quick command edit dialog — only open when terminal tab is active -->
    <QuickCommandDialog :open="props.active && showEditDialog" @close="showEditDialog = false" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'

import PopupMenu from '@/components/common/PopupMenu.vue'
import QuickCommandDialog from '@/components/terminal/QuickCommandDialog.vue'
import { useTerminalSession } from '@/composables/useTerminalSession'
import { useTerminalViewport } from '@/composables/useTerminalViewport'
import { useTerminalKeys } from '@/composables/useTerminalKeys'
import { shouldPreventTerminalContextMenu, useTerminalGestures } from '@/composables/useTerminalGestures'
import { useToast } from '@/composables/useToast'
import { useQuickCommands } from '@/composables/useQuickCommands'
import { useAppMode } from '@/composables/useAppMode'
import { store } from '@/stores/app'
import { resolveTerminalCwd, shouldPromptForTerminalReopen } from './terminalCwd'
import {
  DEFAULT_FONT_SIZE,
  shortCwd as shortCwdUtil,
  canReconnect as canReconnectUtil,
  errorDisplayMessage as errorDisplayMessageUtil,
  showErrorOverlay as showErrorOverlayUtil,
} from '@/utils/terminalFontUtils'
import { localConfig, setLocalConfig } from '@/composables/useSettingsConfig'
import {
  ALL_SYMBOLS,
  loadSymbolFreqs,
  saveSymbolFreqs,
  sortSymbolsByFreq as sortSymbolsByFreqUtil,
  incrementSymbolFreq,
} from '@/utils/terminalSymbolFreq'
import type { SymbolFreqs } from '@/utils/terminalSymbolFreq'

import { Terminal as TerminalIcon, Copy as CopyIcon, Zap as ZapIcon, Hand as HandIcon, RefreshCw as RefreshCwIcon, ArrowUpFromLine as ShiftIcon, Hash as HashIcon } from 'lucide-vue-next'

const props = defineProps<{
  requestedCwd?: string | null
  active?: boolean
}>()

const emit = defineEmits<{
  open: []
}>()

const { t } = useI18n()
const toast = useToast()

// Font size with persistence via settings config
const fontSize = ref<number>(localConfig.terminalFontSize || DEFAULT_FONT_SIZE)

function applyFontSize(size: number) {
  const MIN = 8, MAX = 28
  const clamped = Math.max(MIN, Math.min(MAX, size))
  fontSize.value = clamped
  setLocalConfig('terminalFontSize', clamped)
  if (xterm.value) {
    xterm.value.options.fontSize = clamped
    viewport.fitTerminal()
  }
}

// Refs
const terminalContainer = ref<HTMLElement | null>(null)
const gestureHint = ref('')
let gestureHintTimer: ReturnType<typeof setTimeout> | null = null
const xterm = ref<Terminal | null>(null)
const fitAddon = ref<FitAddon | null>(null)
const showCommands = ref(false)
const cmdBtnRef = ref<HTMLElement | null>(null)
const rebuilding = ref(false)
const showReopenPrompt = ref(false)
const showSymbolBar = ref(false)

// Symbol bar with recency-weighted frequency sorting (exponential decay)
const symbolKeys = ref<string[]>([...ALL_SYMBOLS])

// Sort symbols by decayed score (descending)
function sortSymbolsByFreq() {
  const freqs = loadSymbolFreqs()
  const now = Date.now()
  symbolKeys.value = sortSymbolsByFreqUtil(freqs, now)
}

// Increment symbol: apply decay to old score, add 1, update timestamp
function handleSymbolClick(sym: string) {
  const freqs = loadSymbolFreqs()
  const now = Date.now()
  const updated = incrementSymbolFreq(freqs, sym, now)
  saveSymbolFreqs(updated)
  session.sendInput(sym)
  focusTerminal()
}

// Toggle symbol bar — re-sort on open
function toggleSymbolBar() {
  showSymbolBar.value = !showSymbolBar.value
  if (showSymbolBar.value) {
    sortSymbolsByFreq()
  }
  focusTerminal()
}

function computeCwd(): string {
  return resolveTerminalCwd({
    currentFilePath: store.state.currentFile?.path,
    currentDir: store.state.currentDir,
    requestedCwd: props.requestedCwd,
  })
}

function targetAbsoluteCwd(): string {
  const root = store.state.projectRoot.replace(/\/+$/, '')
  const cwd = computeCwd()
  return cwd ? `${root}/${cwd}` : root
}

// Terminal session
const getWsUrl = () => {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const cwd = computeCwd()
  const cwdParam = cwd ? `?cwd=${encodeURIComponent(cwd)}` : ''
  return `${proto}//${location.host}/api/terminal/ws${cwdParam}`
}

const session = useTerminalSession(getWsUrl)
const { connectionState, errorMessage, errorCode, currentCwd } = session

// Quick commands composable (module-level singleton)
const {
  visibleCommands,
  autoExecCommand,
  fetchCommands,
  showEditDialog,
} = useQuickCommands()

// Terminal viewport (also syncs keyboardHeight to useTerminalKeyboard singleton)
const viewport = useTerminalViewport(xterm, terminalContainer)

// Terminal keys
const terminalKeys = useTerminalKeys(session.sendInput)

let touchScrollRemainder = 0

function handleTerminalTouchScroll(deltaY: number) {
  const term = xterm.value
  if (!term) return

  const lineHeightOption = typeof term.options.lineHeight === 'number' ? term.options.lineHeight : 1
  const rowHeight = Math.max(1, fontSize.value * lineHeightOption)
  touchScrollRemainder += deltaY / rowHeight
  const lines = Math.trunc(touchScrollRemainder)
  if (lines === 0) return

  term.scrollLines(-lines)
  touchScrollRemainder -= lines
}

// Terminal gestures (Termius-style: swipe arrows, long-press Esc, double-tap Tab, pinch zoom)
const gestures = useTerminalGestures(terminalContainer, {
  sendArrowUp: terminalKeys.sendArrowUp,
  sendArrowDown: terminalKeys.sendArrowDown,
  sendArrowLeft: terminalKeys.sendArrowLeft,
  sendArrowRight: terminalKeys.sendArrowRight,
  sendPageUp: terminalKeys.sendPageUp,
  sendPageDown: terminalKeys.sendPageDown,
  sendEscape: terminalKeys.sendEscape,
  sendTab: terminalKeys.sendTab,
  onPinchZoom: (delta: number) => applyFontSize(fontSize.value + delta),
  onTouchScroll: handleTerminalTouchScroll,
  onGestureHint: (symbol: string) => {
    gestureHint.value = symbol
    if (gestureHintTimer) clearTimeout(gestureHintTimer)
    gestureHintTimer = setTimeout(() => { gestureHint.value = '' }, 600)
  },
})

// Volume key → arrow key mapping (Android app mode only)
// When the terminal panel is open, volume up/down are remapped to arrow up/down
// via the Android native bridge. On close, the default volume behavior is restored.
const { isAppMode } = useAppMode()

function enableVolumeKeys() {
  if (!isAppMode.value) return
  const native = (window as any).AndroidNative
  if (native?.setVolumeKeyMode) {
    native.setVolumeKeyMode(true)
  }
}

function disableVolumeKeys() {
  if (!isAppMode.value) return
  const native = (window as any).AndroidNative
  if (native?.setVolumeKeyMode) {
    native.setVolumeKeyMode(false)
  }
}

// Register the global callback that Android calls via evaluateJavascript
// when a volume key is pressed while volumeKeyMode is active.
;(window as any).__onVolumeKey = (direction: 'up' | 'down') => {
  if (direction === 'up') {
    terminalKeys.sendArrowUp()
  } else {
    terminalKeys.sendArrowDown()
  }
}

// Computed
const shortCwd = computed(() => shortCwdUtil(currentCwd.value))

const showErrorOverlay = computed(() => showErrorOverlayUtil(connectionState.value))

const canReconnect = computed(() => canReconnectUtil(errorCode.value))

const errorDisplayMessage = computed(() => errorDisplayMessageUtil(errorCode.value, errorMessage.value, t('terminal.websocketFailed')))

const panelStyle = computed(() => ({
  '--keyboard-height': `${viewport.keyboardHeight.value}px`,
}))

// Theme
function getXtermTheme() {
  const isDark = document.documentElement.getAttribute('data-theme') === 'dark'
  return isDark ? darkTheme : lightTheme
}

const darkTheme = {
  background: '#1e1e2e',
  foreground: '#cdd6f4',
  cursor: '#f5e0dc',
  cursorAccent: '#1e1e2e',
  selectionBackground: '#585b7066',
  black: '#45475a', red: '#f38ba8', green: '#a6e3a1', yellow: '#f9e2af',
  blue: '#89b4fa', magenta: '#f5c2e7', cyan: '#94e2d5', white: '#bac2de',
  brightBlack: '#585b70', brightRed: '#f38ba8', brightGreen: '#a6e3a1',
  brightYellow: '#f9e2af', brightBlue: '#89b4fa', brightMagenta: '#f5c2e7',
  brightCyan: '#94e2d5', brightWhite: '#a6adc8',
}

const lightTheme = {
  background: '#eff1f5',
  foreground: '#4c4f69',
  cursor: '#dc8a78',
  cursorAccent: '#eff1f5',
  selectionBackground: '#acb0be66',
  black: '#bcc0cc', red: '#d20f39', green: '#40a02b', yellow: '#df8e1d',
  blue: '#1e66f5', magenta: '#ea76cb', cyan: '#179299', white: '#4c4f69',
  brightBlack: '#9ca0b0', brightRed: '#d20f39', brightGreen: '#40a02b',
  brightYellow: '#df8e1d', brightBlue: '#1e66f5', brightMagenta: '#ea76cb',
  brightCyan: '#179299', brightWhite: '#6c6f85',
}

// Initialize xterm
function initTerminal() {
  if (xterm.value) return

  const term = new Terminal({
    theme: getXtermTheme(),
    fontSize: fontSize.value,
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    cursorBlink: true,
    convertEol: true,
    scrollback: 5000,
    rightClickSelectsWord: true,
  })

  const fit = new FitAddon()
  term.loadAddon(fit)
  term.loadAddon(new WebLinksAddon())

  // Store addons for later access
  ;(term as any).fitAddon = fit
  fitAddon.value = fit

  // Handle terminal input
  term.onData((data) => {
    const processed = terminalKeys.processInput(data)
    session.sendInput(processed)
  })

  // Send resize to backend when terminal dimensions change
  term.onResize(({ cols, rows }) => {
    session.sendResize(cols, rows)
  })

  // Set up session callbacks
  session.setCallbacks({
    onOutput: (data) => {
      term.write(data)
    },
    onReplay: (data) => {
      // Clear terminal before replaying to avoid conflicts between
      // stale buffer content and ANSI sequences in the replay data
      term.clear()
      term.write(data)
    },
    onStatus: (status) => {
      // Auto-execute quick command on every connect/reconnect
      if (autoExecCommand.value) {
        session.sendInput(autoExecCommand.value.command + '\r')
      }
    },
    onExit: (code) => {
      toast.show(t('terminal.ptyExited'), { type: 'info' })
    },
    onError: (message, code) => {
      // Error displayed via overlay
    },
  })

  xterm.value = term
}

// Mount terminal to DOM
async function mountTerminal() {
  if (!xterm.value || !terminalContainer.value) return

  // Only open if not already
  if (xterm.value.element) return

  xterm.value.open(terminalContainer.value)

  // Ctrl+Wheel to zoom font size (desktop)
  const container = terminalContainer.value
  const wheelHandler = (e: WheelEvent) => {
    if (e.ctrlKey || e.metaKey) {
      e.preventDefault()
      const delta = e.deltaY < 0 ? 1 : -1
      applyFontSize(fontSize.value + delta)
    }
  }
  container.addEventListener('wheel', wheelHandler, { passive: false })
  // Store for cleanup
  ;(container as any).__terminalWheelHandler = wheelHandler

  // Suppress native context menu only while terminal gestures are active.
  // When gestures are disabled, long-press must be able to open the platform
  // selection/copy UI instead of being reduced to a vibration.
  const contextMenuHandler = (e: Event) => {
    if (shouldPreventTerminalContextMenu(gestures.enabled.value)) {
      e.preventDefault()
    }
  }
  container.addEventListener('contextmenu', contextMenuHandler)
  ;(container as any).__terminalContextMenuHandler = contextMenuHandler

  await nextTick()
  viewport.startWatching()
  gestures.attach()
  focusTerminal()

  // Connect WebSocket
  try {
    await session.connect()
  } catch (err) {
    console.error('terminal: connection failed', err)
  }
}

function focusTerminal() {
  xterm.value?.focus()
}

// Lifecycle: expose init/mount for parent to call
async function activate() {
  emit('open')
  initTerminal()
  enableVolumeKeys()
  await nextTick()
  await mountTerminal()
}

function deactivate() {
  disableVolumeKeys()
  session.disconnect()
  terminalKeys.reset()
  showCommands.value = false
  showReopenPrompt.value = false
  cleanupTerminal()
}

// Auto-activate/deactivate when the terminal tab becomes active/inactive.
// This replaces the old BottomSheet watch(() => props.open, ...) lifecycle.
watch(() => props.active, async (isActive) => {
  if (isActive) {
    emit('open')
    initTerminal()
    enableVolumeKeys()
    await nextTick()
    await mountTerminal()
  } else {
    disableVolumeKeys()
    session.disconnect()
    terminalKeys.reset()
    showCommands.value = false
    showReopenPrompt.value = false
    cleanupTerminal()
  }
}, { immediate: true })

// Watch target cwd changes. Do not automatically rebuild: a terminal may be
// running a long-lived command, so changing files/directories must only show a
// prompt and wait for explicit user confirmation before closing the PTY.
watch([
  () => props.requestedCwd,
  () => store.state.currentDir,
  () => store.state.currentFile?.path,
  currentCwd,
], () => {
  if (connectionState.value !== 'connected') return
  showReopenPrompt.value = shouldPromptForTerminalReopen(currentCwd.value, targetAbsoluteCwd())
})

// Watch theme changes
let themeObserver: MutationObserver | null = null

onMounted(async () => {
  // Load quick commands from API
  await fetchCommands()

  // Watch for theme changes
  themeObserver = new MutationObserver(() => {
    if (xterm.value) {
      xterm.value.options.theme = getXtermTheme()
    }
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme'],
  })
})

onBeforeUnmount(() => {
  themeObserver?.disconnect()
  viewport.stopWatching()
  gestures.detach()
  disableVolumeKeys()
  delete (window as any).__onVolumeKey
  session.disconnect()
  cleanupTerminal()
})

// Actions
function handleModifier(key: 'ctrl' | 'alt' | 'shift') {
  terminalKeys.toggleModifier(key, false)
  focusTerminal()
}

function handleReconnect() {
  session.disconnect()
  session.connect().then(() => {
    focusTerminal()
  }).catch(() => {
    // Error will be shown via overlay
  })
}

function cleanupTerminal() {
  touchScrollRemainder = 0
  // Remove event handlers from container
  if (terminalContainer.value) {
    const wheelH = (terminalContainer.value as any).__terminalWheelHandler
    if (wheelH) terminalContainer.value.removeEventListener('wheel', wheelH)
    const ctxH = (terminalContainer.value as any).__terminalContextMenuHandler
    if (ctxH) terminalContainer.value.removeEventListener('contextmenu', ctxH)
  }
  // Dispose xterm — next open will create a fresh instance
  xterm.value?.dispose()
  xterm.value = null
  fitAddon.value = null
}

function dismissReopenPrompt() {
  showReopenPrompt.value = false
  focusTerminal()
}

async function handleRebuild() {
  terminalKeys.reset()
  showCommands.value = false
  showReopenPrompt.value = false
  rebuilding.value = true

  await rebuildSession()
}

async function rebuildSession() {
  // Close specific session via HTTP API (ensures PTY is dead)
  try {
    const sid = session.sessionId.value
    const url = sid ? `/api/terminal/close?session=${encodeURIComponent(sid)}` : '/api/terminal/close'
    await fetch(url, { method: 'POST' })
  } catch {
    // Reconnect below will surface any remaining terminal errors.
  }

  // Reset session state (closes WS, clears errors, resets reconnect counter, clears sessionId)
  session.reset()

  // Clear terminal display
  if (xterm.value) {
    xterm.value.clear()
  }

  // Reconnect with current cwd — backend will create a new session
  try {
    await session.connect()
    focusTerminal()
  } catch {
    // Error will be shown via overlay
  } finally {
    rebuilding.value = false
  }
}

function handleCopyOutput() {
  if (!xterm.value) return
  const buffer = xterm.value.buffer.active
  const lines: string[] = []
  for (let i = 0; i < buffer.length; i++) {
    const line = buffer.getLine(i)?.translateToString(true)
    if (line) lines.push(line)
  }
  const text = lines.filter(l => l.trim()).join('\n')
  navigator.clipboard.writeText(text).catch(() => {})
  toast.show(t('common.copied'), { type: 'success', duration: 1500 })
  focusTerminal()
}

function executeCommand(cmd: { id: number; label: string; command: string }) {
  session.sendInput(cmd.command + '\r')
  showCommands.value = false
  focusTerminal()
}

function openEditDialog() {
  showCommands.value = false
  showEditDialog.value = true
}

// Expose for parent component — keyboardHeight lets App.vue hide chrome when terminal has soft keyboard
defineExpose({ activate, deactivate, keyboardHeight: viewport.keyboardHeight })
</script>

<style scoped>
.terminal-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  position: relative;
}

.terminal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 8px;
  height: 28px;
  border-bottom: none;
  flex-shrink: 0;
  background: var(--bg-secondary);
  gap: 8px;
  position: relative;
  z-index: 2;
}

.terminal-header-left {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.terminal-title {
  font-weight: 600;
  font-size: 13px;
  white-space: nowrap;
  color: var(--text-primary);
}

.terminal-cwd {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.terminal-header-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.terminal-font-size {
  font-size: 11px;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
  cursor: pointer;
  padding: 1px 4px;
  border-radius: 4px;
  min-width: 20px;
  text-align: center;
}

.terminal-font-size:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

.terminal-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-muted);
}

.terminal-status-dot.connected {
  background: var(--color-green);
}

.terminal-status-dot.connecting,
.terminal-status-dot.reconnecting {
  background: var(--color-yellow);
  animation: status-blink 1s ease-in-out infinite;
}

.terminal-status-dot.disconnected,
.terminal-status-dot.error {
  background: var(--text-muted);
}

@keyframes status-blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.terminal-container {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
  background: #1e1e2e;
}

/* Keep the terminal scrollbar as a thin position indicator instead of a wide rail. */
.terminal-container :deep(.xterm-scrollable-element > .scrollbar.vertical),
.terminal-container :deep(.xterm-scrollbar) {
  width: 2px !important;
  right: 1px !important;
  background: transparent !important;
}

.terminal-container :deep(.xterm-scrollable-element > .scrollbar > .slider) {
  width: 2px !important;
  left: 0 !important;
  border-radius: 999px !important;
}

[data-theme="dark"] .terminal-container {
  background: #1e1e2e;
}

:root:not([data-theme="dark"]) .terminal-container {
  background: #eff1f5;
}

.terminal-rebuild-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: rgba(0, 0, 0, 0.6);
  color: rgba(255, 255, 255, 0.8);
  font-size: 13px;
  z-index: 8;
  user-select: none;
  -webkit-user-select: none;
}

.terminal-rebuild-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: rgba(255, 255, 255, 0.8);
  border-radius: 50%;
  animation: terminal-spin 0.6s linear infinite;
}

@keyframes terminal-spin {
  to { transform: rotate(360deg); }
}

.gesture-hint {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 48px;
  color: rgba(255, 255, 255, 0.7);
  text-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
  pointer-events: none;
  z-index: 5;
  user-select: none;
  -webkit-user-select: none;
}

.gesture-hint-enter-active {
  transition: opacity 0.1s ease;
}
.gesture-hint-leave-active {
  transition: opacity 0.4s ease;
}
.gesture-hint-enter-from,
.gesture-hint-leave-to {
  opacity: 0;
}

.terminal-error-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.8);
  color: #fff;
  z-index: 10;
  padding: 20px;
  text-align: center;
}

.terminal-prompt-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: center;
}

.terminal-reconnect-btn {
  margin-top: 12px;
  padding: 6px 16px;
  border: 1px solid rgba(255, 255, 255, 0.4);
  border-radius: 6px;
  background: transparent;
  color: #fff;
  cursor: pointer;
  font-size: 13px;
}

.terminal-reconnect-btn:hover {
  background: rgba(255, 255, 255, 0.1);
}

.terminal-toolbar {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  background: var(--bg-secondary);
  border-top: 1px solid color-mix(in srgb, var(--border-color) 40%, transparent);
  --toolbar-key-hover: color-mix(in srgb, var(--text-primary) 7%, transparent);
  --toolbar-key-active: color-mix(in srgb, var(--text-primary) 12%, transparent);
  --toolbar-key-text: color-mix(in srgb, var(--text-primary) 72%, transparent);
  --toolbar-key-muted: color-mix(in srgb, var(--text-muted) 72%, transparent);
  --toolbar-key-selected-bg: color-mix(in srgb, var(--text-primary) 14%, transparent);
  --toolbar-key-selected-text: var(--text-primary);
  --toolbar-divider: color-mix(in srgb, var(--border-color) 48%, transparent);
  --toolbar-scrollbar-track: color-mix(in srgb, var(--border-color) 20%, transparent);
  --toolbar-scrollbar-thumb: color-mix(in srgb, var(--text-muted) 46%, transparent);
  --toolbar-scrollbar-thumb-hover: color-mix(in srgb, var(--text-primary) 58%, transparent);
}

[data-theme="dark"] .terminal-toolbar {
  background: var(--bg-secondary);
  --toolbar-key-hover: color-mix(in srgb, var(--text-primary) 9%, transparent);
  --toolbar-key-active: color-mix(in srgb, var(--text-primary) 16%, transparent);
  --toolbar-key-text: color-mix(in srgb, var(--text-primary) 64%, transparent);
  --toolbar-key-muted: color-mix(in srgb, var(--text-muted) 64%, transparent);
  --toolbar-key-selected-bg: color-mix(in srgb, var(--text-primary) 18%, transparent);
  --toolbar-key-selected-text: var(--text-primary);
  --toolbar-divider: color-mix(in srgb, var(--border-color) 52%, transparent);
  --toolbar-scrollbar-track: color-mix(in srgb, var(--border-color) 30%, transparent);
  --toolbar-scrollbar-thumb: color-mix(in srgb, var(--text-muted) 54%, transparent);
  --toolbar-scrollbar-thumb-hover: color-mix(in srgb, var(--text-primary) 68%, transparent);
}

/* Symbol bar: full-width scrollable row above the main toolbar */
.symbol-bar {
  padding: 3px 6px 0;
  background: color-mix(in srgb, var(--text-primary) 3%, transparent);
  border-radius: 6px 6px 0 0;
}

.symbol-bar-scroll {
  display: flex;
  align-items: center;
  gap: 3px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: thin;
  scrollbar-color: var(--toolbar-scrollbar-thumb) transparent;
}
.symbol-bar-scroll::-webkit-scrollbar {
  height: 2px;
}
.symbol-bar-scroll::-webkit-scrollbar-track {
  background: transparent;
}
.symbol-bar-scroll::-webkit-scrollbar-thumb {
  background: var(--toolbar-scrollbar-thumb);
  border-radius: 999px;
}
.symbol-bar-scroll:hover::-webkit-scrollbar-thumb {
  background: var(--toolbar-scrollbar-thumb-hover);
}

/* Main toolbar row: gesture toggles + scrollable key groups */
.main-toolbar-row {
  display: flex;
  align-items: center;
  padding: 4px 6px;
  gap: 2px;
}

.gesture-toggle {
  flex-shrink: 0;
  margin-right: 2px;
}

.toolbar-scroll {
  display: flex;
  align-items: center;
  gap: 0;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  flex: 1;
  min-width: 0;
  padding-bottom: 1px;
  scrollbar-width: thin;
  scrollbar-color: var(--toolbar-scrollbar-thumb) transparent;
}
.toolbar-scroll::-webkit-scrollbar {
  height: 2px;
}
.toolbar-scroll::-webkit-scrollbar-track {
  background: linear-gradient(90deg,
    transparent 0,
    var(--toolbar-scrollbar-track) 14px,
    var(--toolbar-scrollbar-track) calc(100% - 14px),
    transparent 100%);
}
.toolbar-scroll::-webkit-scrollbar-thumb {
  background: var(--toolbar-scrollbar-thumb);
  border-radius: 999px;
  transition: background 140ms ease;
}
.toolbar-scroll:hover::-webkit-scrollbar-thumb {
  background: var(--toolbar-scrollbar-thumb-hover);
}

.key-group {
  display: flex;
  align-items: center;
  gap: 3px;
}

.key-group + .key-group {
  position: relative;
  margin-left: 6px;
}

.key-group + .key-group::before {
  content: '';
  position: absolute;
  left: -4px;
  width: 1px;
  height: 16px;
  border-radius: 999px;
  background: var(--toolbar-divider);
}

.toolbar-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 32px;
  padding: 0 5px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--toolbar-key-text);
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.01em;
  cursor: pointer;
  flex-shrink: 0;
  user-select: none;
  -webkit-user-select: none;
  touch-action: manipulation;
  transition:
    background 140ms ease,
    color 140ms ease,
    transform 90ms ease;
}

.toolbar-btn:hover {
  background: var(--toolbar-key-hover);
}

.toolbar-btn:active {
  background: var(--toolbar-key-active);
  transform: translateY(1px) scale(0.98);
}

.toolbar-btn:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--text-primary) 36%, transparent);
  outline-offset: 2px;
}

.toolbar-btn.modifier.active {
  background: var(--toolbar-key-selected-bg);
  color: var(--toolbar-key-selected-text);
}

.toolbar-btn.modifier.locked {
  background: var(--toolbar-key-selected-bg);
  color: var(--toolbar-key-selected-text);
  box-shadow: inset 0 -2px 0 color-mix(in srgb, var(--toolbar-key-selected-text) 36%, transparent);
}

.toolbar-btn.shortcut {
  background: transparent;
  color: var(--toolbar-key-text);
  font-weight: 800;
  font-size: 11px;
}

.toolbar-btn.shortcut:active {
  background: var(--toolbar-key-active);
}

.toolbar-btn.danger {
  color: var(--toolbar-key-text);
  opacity: 0.78;
}

.toolbar-btn.danger:hover {
  opacity: 1;
  background: var(--toolbar-key-hover);
}

/* Gesture toggle keeps a compact anchor shape outside the scroll row. */
.toolbar-btn.gesture-toggle {
  min-width: 32px;
  border-radius: 9px;
}

/* Mobile: adjust toolbar for soft keyboard */
@media (max-width: 768px) {
  .main-toolbar-row {
    padding-bottom: max(4px, env(safe-area-inset-bottom));
  }
}

/* Touch device: prevent sticky hover */
@media (hover: none) {
  .toolbar-btn:hover {
    background: transparent;
  }
  .toolbar-btn.shortcut:hover {
    background: transparent;
  }
  .toolbar-btn.modifier.active:hover,
  .toolbar-btn.modifier.locked:hover {
    background: var(--toolbar-key-selected-bg);
  }
  .toolbar-btn:active {
    background: var(--toolbar-key-active);
  }
}

/* Button groups share one borderless system; class hooks remain semantic. */
.toolbar-btn.btn-modifier,
.toolbar-btn.btn-nav,
.toolbar-btn.btn-arrow,
.toolbar-btn.btn-symbol,
.toolbar-btn.btn-action {
  background: transparent;
}

.toolbar-btn.btn-symbol {
  color: var(--toolbar-key-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 15px;
  font-weight: 700;
}
</style>

<style>
/* Quick commands popup divider (unscoped because PopupMenu teleports to body) */
.quick-send-divider {
  height: 1px;
  background: var(--border-color);
  margin: 4px 0;
}
</style>

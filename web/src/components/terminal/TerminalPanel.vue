<template>
  <BottomSheet ref="bottomSheetRef" :open="open" @close="$emit('close')" noHeader>
    <div class="terminal-panel" :style="panelStyle">
      <!-- Header -->
      <div class="terminal-header" @click.self="focusTerminal">
        <div class="terminal-header-left">
          <TerminalIcon :size="16" />
          <span class="terminal-title">{{ t('terminal.title') }}</span>
          <span v-if="currentCwd" class="terminal-cwd" :title="currentCwd">{{ shortCwd }}</span>
        </div>
        <div class="terminal-header-right">
          <span class="terminal-font-size" @click="applyFontSize(DEFAULT_FONT_SIZE)" :title="t('terminal.resetFontSize')">{{ fontSize }}</span>
          <span v-if="connectionState === 'connected'" class="terminal-status connected">{{ t('terminal.connected') }}</span>
          <span v-else-if="connectionState === 'connecting' || connectionState === 'reconnecting'" class="terminal-status connecting">{{ t('terminal.reconnecting') }}</span>
          <span v-else class="terminal-status disconnected">{{ t('terminal.disconnected') }}</span>
        </div>
      </div>

      <!-- Terminal viewport -->
      <div ref="terminalContainer" class="terminal-container" @click.self="focusTerminal">
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
        <button class="toolbar-btn modifier gesture-toggle" :class="{ active: gestures.enabled.value }" @pointerdown.prevent="gestures.toggle(); focusTerminal()" @contextmenu.prevent :title="t('terminal.gestures')">
          <HandIcon :size="14" />
        </button>
        <div class="toolbar-scroll">
          <button class="toolbar-btn" @pointerdown.prevent="terminalKeys.sendEscape(); focusTerminal()" :title="'Esc'">Esc</button>
          <button class="toolbar-btn" @pointerdown.prevent="terminalKeys.sendTab(); focusTerminal()" :title="'Tab'">Tab</button>
          <button class="toolbar-btn modifier" :class="{ active: terminalKeys.activeModifiers.value.ctrl !== 'inactive', locked: terminalKeys.activeModifiers.value.ctrl === 'locked' }" @pointerdown.prevent="handleModifier('ctrl')" @contextmenu.prevent>Ctl</button>
          <button class="toolbar-btn modifier" :class="{ active: terminalKeys.activeModifiers.value.alt !== 'inactive', locked: terminalKeys.activeModifiers.value.alt === 'locked' }" @pointerdown.prevent="handleModifier('alt')" @contextmenu.prevent>Alt</button>
          <button class="toolbar-btn" @pointerdown.prevent="terminalKeys.sendCtrlC(); focusTerminal()" :title="'Ctrl+C'">C-C</button>
          <button class="toolbar-btn" @pointerdown.prevent="session.sendInput('/'); focusTerminal()">/</button>
          <button class="toolbar-btn" @pointerdown.prevent="session.sendInput('-'); focusTerminal()">-</button>
          <button class="toolbar-btn" @pointerdown.prevent="session.sendInput('|'); focusTerminal()">|</button>
          <button class="toolbar-btn" @pointerdown.prevent="session.sendInput('_'); focusTerminal()">_</button>
          <button v-if="quickCommands.length > 0" class="toolbar-btn" @click="showCommands = !showCommands" :title="t('terminal.quickCommands')">
            <ListIcon :size="14" />
          </button>
          <button class="toolbar-btn" @click="handleCopyOutput" :title="t('terminal.copyOutput')">
            <CopyIcon :size="14" />
          </button>
          <button class="toolbar-btn" @click="handleRebuild" :title="t('terminal.rebuildSession')">
            <RefreshCwIcon :size="14" />
          </button>
          <button class="toolbar-btn danger" @click="handleClose" :title="t('terminal.closeProcess')">
            <XIcon :size="14" />
          </button>
        </div>
      </div>

      <!-- Quick commands popup -->
      <div v-if="showCommands" class="terminal-commands-popup">
        <div v-for="(cmd, i) in quickCommands" :key="i" class="command-item" @click="executeCommand(cmd)">
          {{ cmd.label }}
        </div>
        <div v-if="quickCommands.length === 0" class="command-empty">{{ t('terminal.noQuickCommands') }}</div>
      </div>
    </div>
  </BottomSheet>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'

import BottomSheet from '@/components/common/BottomSheet.vue'
import { useTerminalSession } from '@/composables/useTerminalSession'
import { useTerminalViewport } from '@/composables/useTerminalViewport'
import { useTerminalKeys } from '@/composables/useTerminalKeys'
import { useTerminalGestures } from '@/composables/useTerminalGestures'
import { useToast } from '@/composables/useToast'
import { store } from '@/stores/app'

import { Terminal as TerminalIcon, Copy as CopyIcon, X as XIcon, List as ListIcon, Hand as HandIcon, RefreshCw as RefreshCwIcon } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
  open: []
}>()

const { t } = useI18n()
const toast = useToast()

// Font size with persistence
const FONT_SIZE_KEY = 'clawbench-terminal-font-size'
const DEFAULT_FONT_SIZE = 12
const MIN_FONT_SIZE = 8
const MAX_FONT_SIZE = 28

const fontSize = ref(DEFAULT_FONT_SIZE)

function loadFontSize(): number {
  const saved = localStorage.getItem(FONT_SIZE_KEY)
  if (saved) {
    const n = parseInt(saved, 10)
    if (n >= MIN_FONT_SIZE && n <= MAX_FONT_SIZE) return n
  }
  return DEFAULT_FONT_SIZE
}

function applyFontSize(size: number) {
  fontSize.value = Math.max(MIN_FONT_SIZE, Math.min(MAX_FONT_SIZE, size))
  localStorage.setItem(FONT_SIZE_KEY, String(fontSize.value))
  if (xterm.value) {
    xterm.value.options.fontSize = fontSize.value
    viewport.fitTerminal()
  }
}

// Refs
const bottomSheetRef = ref<InstanceType<typeof BottomSheet> | null>(null)
const terminalContainer = ref<HTMLElement | null>(null)
const gestureHint = ref('')
let gestureHintTimer: ReturnType<typeof setTimeout> | null = null
const xterm = ref<Terminal | null>(null)
const fitAddon = ref<FitAddon | null>(null)
const showCommands = ref(false)
const quickCommands = ref<{ label: string; command: string }[]>([])

// Compute cwd from file manager's current directory (not the opened file)
function computeCwd(): string {
  return store.state.currentDir || ''
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

// Terminal viewport
const viewport = useTerminalViewport(xterm, terminalContainer)

// Terminal keys
const terminalKeys = useTerminalKeys(session.sendInput)

// Terminal gestures (Termius-style: swipe arrows, double-tap Tab, pinch zoom)
const gestures = useTerminalGestures(terminalContainer, {
  sendArrowUp: terminalKeys.sendArrowUp,
  sendArrowDown: terminalKeys.sendArrowDown,
  sendArrowLeft: terminalKeys.sendArrowLeft,
  sendArrowRight: terminalKeys.sendArrowRight,
  onPinchZoom: (delta: number) => applyFontSize(fontSize.value + delta),
  onGestureHint: (symbol: string) => {
    gestureHint.value = symbol
    if (gestureHintTimer) clearTimeout(gestureHintTimer)
    gestureHintTimer = setTimeout(() => { gestureHint.value = '' }, 600)
  },
})

// Computed
const shortCwd = computed(() => {
  if (!currentCwd.value) return ''
  const parts = currentCwd.value.split('/')
  return parts.length > 2 ? '.../' + parts.slice(-2).join('/') : currentCwd.value
})

const showErrorOverlay = computed(() => {
  return connectionState.value === 'error' || connectionState.value === 'disconnected'
})

const canReconnect = computed(() => {
  // terminal_disabled means the feature is turned off — no point reconnecting
  if (errorCode.value === 'terminal_disabled') return false
  // All other errors are retryable (session_in_use is no longer possible —
  // backend now kicks the old client and lets the new one take over)
  return true
})

const errorDisplayMessage = computed(() => {
  if (errorCode.value === 'terminal_disabled') return t('terminal.disabled')
  if (errorCode.value === 'shell_start_failed') return t('terminal.shellStartFailed')
  return errorMessage.value || t('terminal.websocketFailed')
})

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
    fontSize: loadFontSize(),
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    cursorBlink: true,
    convertEol: true,
    scrollback: 5000,
    selectionStyle: 'line',
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
      // Terminal is ready
    },
    onExit: (code) => {
      toast.show(t('terminal.ptyExited'))
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

  // Suppress native context menu (paste/clipboard popup on mobile)
  // but don't stopPropagation — xterm.js needs contextmenu for word selection
  const contextMenuHandler = (e: Event) => {
    e.preventDefault()
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

// Watch open/close
watch(() => props.open, async (isOpen) => {
  if (isOpen) {
    emit('open')
    initTerminal()
    await nextTick()
    await mountTerminal()
  }
}, { immediate: true })

// Watch theme changes
let themeObserver: MutationObserver | null = null

onMounted(async () => {
  // Load quick commands from config API
  try {
    const resp = await fetch('/api/terminal/config')
    if (resp.ok) {
      const data = await resp.json()
      quickCommands.value = data.quick_commands || []
    }
  } catch {
    // Config not available — no quick commands
  }

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
  session.disconnect()
  // Cleanup wheel handler and contextmenu handler
  if (terminalContainer.value) {
    const wheelH = (terminalContainer.value as any).__terminalWheelHandler
    if (wheelH) terminalContainer.value.removeEventListener('wheel', wheelH)
    const ctxH = (terminalContainer.value as any).__terminalContextMenuHandler
    if (ctxH) terminalContainer.value.removeEventListener('contextmenu', ctxH)
  }
  xterm.value?.dispose()
  xterm.value = null
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

function handleClose() {
  session.sendClose()
  terminalKeys.reset()
  showCommands.value = false
}

async function handleRebuild() {
  terminalKeys.reset()
  showCommands.value = false

  // Close session via HTTP API (synchronous — ensures PTY is dead and m.session = nil)
  try {
    await fetch('/api/terminal/close', { method: 'POST' })
  } catch {
    // Ignore — best effort
  }

  // Reset session state (closes WS, clears errors, resets reconnect counter)
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
  toast.show(t('common.copied') || 'Copied')
  focusTerminal()
}

function executeCommand(cmd: { label: string; command: string }) {
  session.sendInput(cmd.command + '\r')
  showCommands.value = false
  focusTerminal()
}
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
  padding: 6px 12px;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  background: var(--bg-secondary);
  gap: 8px;
}

.terminal-header-left {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.terminal-title {
  font-weight: 600;
  font-size: 14px;
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

.terminal-status {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 8px;
}

.terminal-status.connected {
  color: var(--color-green);
}

.terminal-status.connecting {
  color: var(--color-yellow);
}

.terminal-status.disconnected {
  color: var(--text-muted);
}

.terminal-container {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
  background: #1e1e2e;
}

[data-theme="dark"] .terminal-container {
  background: #1e1e2e;
}

:root:not([data-theme="dark"]) .terminal-container {
  background: #eff1f5;
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
  align-items: center;
  padding: 4px 6px;
  gap: 3px;
  border-top: 1px solid var(--border-color);
  flex-shrink: 0;
  background: var(--bg-secondary);
}

.gesture-toggle {
  flex-shrink: 0;
  margin-right: 2px;
}

.toolbar-scroll {
  display: flex;
  align-items: center;
  gap: 3px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  flex: 1;
  min-width: 0;
  /* Hide scrollbar for cleaner look */
  scrollbar-width: none;
}
.toolbar-scroll::-webkit-scrollbar {
  display: none;
}

.toolbar-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 32px;
  padding: 0 6px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--bg-key);
  color: var(--text-primary);
  font-size: 12px;
  cursor: pointer;
  flex-shrink: 0;
  user-select: none;
  -webkit-user-select: none;
  touch-action: manipulation;
}

.toolbar-btn:hover {
  background: var(--bg-tertiary);
}

.toolbar-btn:active {
  background: var(--bg-key-active);
}

.toolbar-btn.modifier.active {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: #fff;
}

.toolbar-btn.modifier.locked {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
  color: #fff;
}

.toolbar-btn.danger {
  color: var(--color-red);
  border-color: var(--color-red);
  opacity: 0.7;
}

.toolbar-btn.danger:hover {
  opacity: 1;
  background: var(--bg-tertiary);
}

.terminal-commands-popup {
  position: absolute;
  bottom: 44px;
  right: 6px;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  box-shadow: var(--shadow-md);
  z-index: 20;
  min-width: 160px;
  max-height: 200px;
  overflow-y: auto;
}

.command-item {
  padding: 8px 12px;
  cursor: pointer;
  font-size: 13px;
  white-space: nowrap;
  color: var(--text-primary);
}

.command-item:hover {
  background: var(--bg-tertiary);
}

.command-empty {
  padding: 12px;
  color: var(--text-muted);
  font-size: 13px;
  text-align: center;
}

/* Mobile: adjust toolbar for soft keyboard */
@media (max-width: 768px) {
  .terminal-toolbar {
    padding-bottom: max(4px, env(safe-area-inset-bottom));
  }
}

/* Touch device: prevent sticky hover */
@media (hover: none) {
  .toolbar-btn:hover {
    background: var(--bg-key);
  }
  .toolbar-btn:active {
    background: var(--bg-key-active);
  }
}
</style>

import { ref } from 'vue'

// Module-level singleton — shared between ChatInputBar (activator) and App.vue (reader)
const chatKeyboardHeight = ref(0)

/**
 * Reactive soft-keyboard height for the chat input.
 *
 * On iOS WKWebView there is no adjustResize — window.innerHeight stays the same
 * when the keyboard opens, so fixed-position elements extend behind the keyboard.
 * This composable uses the visualViewport API to detect the keyboard height and
 * expose it reactively so App.vue can compensate.
 *
 * On Android (adjustResize) and desktop, this always returns 0 — those platforms
 * handle keyboard avoidance natively.
 */
export function useChatKeyboard() {
  function activate() {
    clearDeactivateTimer()
    startWatching()
  }

  /**
   * Debounced deactivate — wait a short period after blur before clearing
   * the keyboard height. This prevents a flash where the keyboard is still
   * animating closed (visualViewport still reports a reduced height) but
   * we've already set height=0, causing a brief layout jump.
   */
  function debounceDeactivate() {
    clearDeactivateTimer()
    deactivateTimer = setTimeout(() => {
      deactivateTimer = null
      deactivate()
    }, DEACTIVATE_DELAY_MS)
  }

  function deactivate() {
    clearDeactivateTimer()
    stopWatching()
    chatKeyboardHeight.value = 0
  }

  return { chatKeyboardHeight, activate, deactivate, debounceDeactivate }
}

// ── Internal ──

let watching = false
let deactivateTimer: ReturnType<typeof setTimeout> | null = null
const DEACTIVATE_DELAY_MS = 150

function clearDeactivateTimer() {
  if (deactivateTimer) {
    clearTimeout(deactivateTimer)
    deactivateTimer = null
  }
}

function updateKeyboardHeight() {
  const vv = window.visualViewport
  if (!vv) return

  // keyboardHeight = space taken by the keyboard relative to the layout viewport
  const height = window.innerHeight - vv.height - vv.offsetTop
  chatKeyboardHeight.value = Math.max(height, 0)
}

function onVisualViewportResize() {
  updateKeyboardHeight()
}

function startWatching() {
  if (watching) return
  watching = true
  updateKeyboardHeight()
  if (window.visualViewport) {
    window.visualViewport.addEventListener('resize', onVisualViewportResize)
  }
}

function stopWatching() {
  if (!watching) return
  watching = false
  if (window.visualViewport) {
    window.visualViewport.removeEventListener('resize', onVisualViewportResize)
  }
}

/**
 * Generic composable for SSE/WebSocket reconnection with exponential backoff.
 * Extracts the common reconnect logic shared across useTerminalSession,
 * useChatStream, and useFileWatch.
 */
import { ref } from 'vue'

export interface ReconnectOptions {
  maxAttempts?: number           // default 3
  baseDelay?: number             // default 2000 (ms)
  onReconnect: () => void        // callback to reconnect
  getFatalError?: () => boolean | null // return non-null = fatal, null = safe to reconnect
}

export function useReconnect(options: ReconnectOptions) {
  let reconnectAttempts = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let disabled = false

  // Reactive reconnecting state — true when a reconnect is scheduled or in-progress
  const reconnecting = ref(false)

  function hasActiveAttempts(): boolean {
    return reconnectAttempts < (options.maxAttempts ?? 3)
  }

  function scheduleReconnect() {
    reconnecting.value = true
    const delay = (options.baseDelay ?? 2000) * (reconnectAttempts + 1)
    reconnectTimer = setTimeout(() => {
      reconnectAttempts++
      options.onReconnect()
    }, delay)
  }

  function reset() {
    reconnectAttempts = 0
    disabled = false
    reconnecting.value = false
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = null
  }

  function disable() {
    disabled = true
    reconnecting.value = false
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = null
  }

  function shouldReconnect(): boolean {
    if (disabled) return false
    const fatalError = options.getFatalError?.()
    if (fatalError !== undefined && fatalError !== null) return false
    return hasActiveAttempts()
  }

  return {
    reconnecting,
    scheduleReconnect,
    reset,
    disable,
    shouldReconnect,
    getAttempts: () => reconnectAttempts,
  }
}
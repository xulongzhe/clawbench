import { ref } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/utils/api.ts'
import { useAppMode } from './useAppMode.ts'
import { gt } from '@/composables/useLocale'

interface ForwardedPort {
  port: number
  name: string
  protocol: string
  autoDetect: boolean
  active: boolean
}

interface DetectedPort {
  port: number
  protocol: string
  processName: string
  processArgs: string
}

export interface SSHConnectionStats {
  connected: boolean
  clientCount: number
  activeChannels: number
  lastConnectedAt?: string
}

export interface SSHInfo {
  enabled: boolean
  host: string
  port: number
  username: string
  fingerprint: string
  command: string
  connectionStats: SSHConnectionStats | null
}

export type TunnelStatus = 'unknown' | 'ok' | 'disconnected' | 'degraded'

// Module-level shared state
const ports = ref<ForwardedPort[]>([])
const detectedPorts = ref<DetectedPort[]>([])
const loading = ref(false)
const sshInfo = ref<SSHInfo | null>(null)
const tunnelStatus = ref<TunnelStatus>('unknown')
const tunnelMessage = ref('')
const tunnelChecking = ref(false)

// Auto-refresh interval when tunnel is unhealthy
let tunnelPollTimer: ReturnType<typeof setInterval> | null = null

/** Returns true if any registered port has an active backend */
function hasActivePorts(): boolean {
  return ports.value.some(p => p.active)
}

/**
 * Determines tunnel status from port state.
 * `hasPorts` indicates whether there are any registered ports.
 * When there are ports but none are active, the tunnel is degraded.
 * When there are no ports, or at least one is active, the tunnel is OK.
 */
function tunnelStatusFromPorts(hasPorts: boolean): 'ok' | 'degraded' {
  const anyActive = hasActivePorts()
  if (hasPorts && !anyActive) return 'degraded'
  return 'ok'
}

/**
 * Manages port forwarding state: list of forwarded ports, CRUD operations,
 * auto-detection, and registration with Android native layer.
 */
export function usePortForward() {
  const { isAppMode } = useAppMode()

  async function loadPorts(silent = false) {
    if (!silent) loading.value = true
    try {
      const data = await apiGet<{ ports: ForwardedPort[] }>('/api/proxy/ports')
      ports.value = data.ports || []
    } finally {
      if (!silent) loading.value = false
    }
  }

  async function registerPort(port: number, name?: string, protocol?: string) {
    await apiPost('/api/proxy/ports', { port, name: name || '', protocol: protocol || 'http' })
    // Register with Android native layer
    if (isAppMode.value) {
      ;(window as any).AndroidNative?.addForwardedPort(port)
    }
    // Silent refresh: don't flicker the port list with loading state
    await Promise.all([loadPorts(true), loadSSHInfo()])
  }

  async function unregisterPort(port: number) {
    await apiDelete(`/api/proxy/ports?port=${port}`)
    if (isAppMode.value) {
      ;(window as any).AndroidNative?.removeForwardedPort(port)
    }
    await Promise.all([loadPorts(true), loadSSHInfo()])
  }

  async function detectPorts() {
    const data = await apiGet<{ ports: DetectedPort[] }>('/api/proxy/detect')
    detectedPorts.value = data.ports || []
  }

  /** Sync all registered ports to Android native on initial load.
   *  If the server has no registered ports, stop the native service
   *  to avoid an idle foreground service draining battery. */
  async function syncToNative() {
    if (!isAppMode.value) return
    await loadPorts()
    if (ports.value.length === 0) {
      // No ports on server — stop the native service (clears stale SharedPreferences)
      ;(window as any).AndroidNative?.stopPortForwardService()
      return
    }
    for (const p of ports.value) {
      ;(window as any).AndroidNative?.addForwardedPort(p.port)
    }
  }

  /** Fetch SSH tunnel connection info from server */
  async function loadSSHInfo() {
    try {
      const data = await apiGet<SSHInfo>('/api/ssh/info')
      sshInfo.value = data
    } catch {
      sshInfo.value = null
    }
  }

  /** Check SSH tunnel health and determine status */
  async function checkTunnelHealth() {
    tunnelChecking.value = true
    tunnelStatus.value = 'unknown'
    tunnelMessage.value = ''

    await Promise.all([loadPorts(), loadSSHInfo()])

    const info = sshInfo.value
    // No SSH configured — skip tunnel check (web mode without SSH)
    if (!info?.enabled) {
      tunnelChecking.value = false
      return
    }

    // In app mode: prefer native SSH tunnel status
    if (isAppMode.value) {
      const nativeConnected = getNativeTunnelStatus()
      if (nativeConnected === true) {
        // Native says connected — trust it regardless of server-side connCount
        const hasPorts = ports.value.length > 0
        const status = tunnelStatusFromPorts(hasPorts)
        if (status === 'degraded') {
          tunnelStatus.value = 'degraded'
          tunnelMessage.value = gt('portForward.tunnelDegraded')
          tunnelChecking.value = false
          startTunnelPoll()
          return
        }
        tunnelStatus.value = 'ok'
        tunnelChecking.value = false
        stopTunnelPoll()
        return
      } else if (nativeConnected === false) {
        tunnelStatus.value = 'disconnected'
        tunnelMessage.value = gt('portForward.tunnelDisconnected')
        tunnelChecking.value = false
        startTunnelPoll()
        return
      }
    }

    // Native status unavailable — fall back to server-side connection stats
    const stats = info.connectionStats
    if (!stats) {
      tunnelChecking.value = false
      return
    }

    if (!stats.connected) {
      // Server says disconnected, but check if any ports are actually active
      // (health check passes = tunnel is working despite connCount=0)
      if (hasActivePorts()) {
        tunnelStatus.value = 'ok'
        tunnelChecking.value = false
        stopTunnelPoll()
        return
      }
      tunnelStatus.value = 'disconnected'
      tunnelMessage.value = gt('portForward.tunnelDisconnected')
      tunnelChecking.value = false
      startTunnelPoll()
      return
    }

    // SSH is connected — check if any ports have active backends
    const hasPorts = ports.value.length > 0
    if (tunnelStatusFromPorts(hasPorts) === 'degraded') {
      tunnelStatus.value = 'degraded'
      tunnelMessage.value = gt('portForward.tunnelDegraded')
      tunnelChecking.value = false
      startTunnelPoll()
      return
    }

    tunnelStatus.value = 'ok'
    tunnelChecking.value = false
    stopTunnelPoll()
  }

  /**
   * Query Android native layer for SSH tunnel connection status.
   * Returns true (connected), false (disconnected), or null (unavailable/not app mode).
   */
  function getNativeTunnelStatus(): boolean | null {
    if (!isAppMode.value) return null
    const native = (window as any).AndroidNative
    if (!native || typeof native.isTunnelConnected !== 'function') return null
    try {
      const result = native.isTunnelConnected()
      if (typeof result === 'boolean') return result
      return null
    } catch {
      return null
    }
  }

  /** Start polling tunnel health every 5s while unhealthy */
  function startTunnelPoll() {
    if (tunnelPollTimer) return
    tunnelPollTimer = setInterval(async () => {
      // Check native status first (fast, no network)
      const nativeConnected = getNativeTunnelStatus()
      if (nativeConnected === true) {
        await loadPorts()
        const hasPorts = ports.value.length > 0
        if (tunnelStatusFromPorts(hasPorts) === 'ok') {
          tunnelStatus.value = 'ok'
          tunnelMessage.value = ''
          stopTunnelPoll()
        } else {
          tunnelStatus.value = 'degraded'
          tunnelMessage.value = gt('portForward.tunnelDegraded')
        }
        return
      }

      // Fall back to server-side check
      await loadSSHInfo()
      const info = sshInfo.value
      const stats = info?.connectionStats
      if (stats?.connected) {
        // Re-check full health (ports + ssh)
        await loadPorts()
        const hasPorts = ports.value.length > 0
        if (tunnelStatusFromPorts(hasPorts) === 'ok') {
          tunnelStatus.value = 'ok'
          tunnelMessage.value = ''
          stopTunnelPoll()
        } else {
          tunnelStatus.value = 'degraded'
          tunnelMessage.value = gt('portForward.tunnelDegraded')
        }
      } else {
        // Server says disconnected — still check if ports are actually active
        await loadPorts()
        if (hasActivePorts()) {
          tunnelStatus.value = 'ok'
          tunnelMessage.value = ''
          stopTunnelPoll()
        }
      }
    }, 5000)
  }

  /** Stop the tunnel health polling */
  function stopTunnelPoll() {
    if (tunnelPollTimer) {
      clearInterval(tunnelPollTimer)
      tunnelPollTimer = null
    }
  }

  /** Open a forwarded port — in app mode opens sandbox browser, otherwise window.open */
  function openPort(targetPort: number, protocol?: string) {
    const scheme = protocol === 'https' ? 'https' : 'http'
    if (isAppMode.value) {
      const native = (window as any).AndroidNative
      // Prefer sandbox browser (isolated process), fall back to external browser
      if (native?.openInSandbox) {
        native.openInSandbox(targetPort, scheme)
      } else if (native?.openInBrowser) {
        native.openInBrowser(targetPort, scheme)
      }
    } else {
      window.open(`${scheme}://localhost:${targetPort}`, '_blank')
    }
  }

  /** Open a forwarded port in external/system browser */
  function openInExternalBrowser(targetPort: number, protocol?: string) {
    const scheme = protocol === 'https' ? 'https' : 'http'
    if (isAppMode.value) {
      const native = (window as any).AndroidNative
      if (native?.openInBrowser) {
        native.openInBrowser(targetPort, scheme)
      }
    } else {
      window.open(`${scheme}://localhost:${targetPort}`, '_blank')
    }
  }

  return {
    ports,
    detectedPorts,
    loading,
    isAppMode,
    sshInfo,
    tunnelStatus,
    tunnelMessage,
    tunnelChecking,
    loadPorts,
    registerPort,
    unregisterPort,
    detectPorts,
    syncToNative,
    loadSSHInfo,
    checkTunnelHealth,
    openPort,
    openInExternalBrowser,
  }
}

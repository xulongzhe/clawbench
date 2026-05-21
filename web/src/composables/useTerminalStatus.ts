import { ref } from 'vue'
import { apiGet } from '@/utils/api'

// Module-level singleton — shared across all callers
const terminalRuntimeEnabled = ref<boolean | null>(null)

/**
 * Lightweight composable for terminal runtime availability.
 *
 * Unlike `getServerValueWithDefault('terminal.enabled')` which reads the
 * *config* value (optimistically updated before restart), this queries the
 * actual server runtime: `/api/terminal/status` returns `enabled: false`
 * when the terminal manager is nil (e.g. config says true but server hasn't
 * restarted yet). Mirrors the SSH pattern where `sshInfo.enabled` comes from
 * the live `/api/ssh/info` endpoint.
 */
export function useTerminalStatus() {
  async function loadTerminalStatus() {
    try {
      const data = await apiGet<{ enabled: boolean }>('/api/terminal/status')
      terminalRuntimeEnabled.value = data.enabled ?? false
    } catch {
      terminalRuntimeEnabled.value = false
    }
  }

  return { terminalRuntimeEnabled, loadTerminalStatus }
}

import { reactive, ref } from 'vue'
import { apiGet, apiPatch, apiPost } from '@/utils/api'

const LOCAL_PREFIX = 'clawbench-settings-'

const localDefaults: Record<string, any> = {
  theme: 'auto',
  locale: 'zh',
  autoSpeech: false,
  showHidden: false,
  wordWrap: true,
  lineNumbers: false,
  fileView: 'list',
  terminalFontSize: 14,
  androidLogCapture: false,
}

function readLocal(key: string): any {
  try {
    const saved = localStorage.getItem(LOCAL_PREFIX + key)
    if (saved !== null) return JSON.parse(saved)
  } catch { /* ignore */ }
  return localDefaults[key] ?? null
}

function writeLocal(key: string, value: any) {
  try {
    localStorage.setItem(LOCAL_PREFIX + key, JSON.stringify(value))
  } catch { /* ignore */ }
}

// Build reactive local config from defaults + localStorage
const localConfig = reactive<Record<string, any>>({})
for (const key of Object.keys(localDefaults)) {
  localConfig[key] = readLocal(key)
}

const serverConfig = ref<Record<string, any>>({})

export function useSettingsConfig() {
  async function loadConfig() {
    try {
      const data = await apiGet<Record<string, any>>('/api/config')
      serverConfig.value = data
    } catch {
      // Silently ignore — server may be unreachable
    }
  }

  async function patchConfig(changes: Record<string, any>): Promise<{ needsRestart: boolean; changedColdFields: string[] }> {
    const result = await apiPatch<{ needsRestart?: boolean; changedColdFields?: string[] }>('/api/config', changes)
    // Merge patched values into local cache
    Object.assign(serverConfig.value, changes)
    return {
      needsRestart: result.needsRestart ?? false,
      changedColdFields: result.changedColdFields ?? [],
    }
  }

  async function restartServer() {
    await apiPost('/api/config/restart', {})
  }

  function setLocalConfig(key: string, value: any) {
    localConfig[key] = value
    writeLocal(key, value)
  }

  /** Read a server config value by dot-path (e.g. "server.port") */
  function getServerValue(dotPath: string): any {
    const parts = dotPath.split('.')
    let current: any = serverConfig.value
    for (const p of parts) {
      if (current == null || typeof current !== 'object') return undefined
      current = current[p]
    }
    return current
  }

  /** Write a server config value by dot-path and patch the server */
  async function setServerValue(dotPath: string, value: any): Promise<{ needsRestart: boolean; changedColdFields: string[] }> {
    const parts = dotPath.split('.')
    const changes: Record<string, any> = {}
    // Build nested object for patch (e.g. "server.port" → { server: { port: val } })
    let obj: any = changes
    for (let i = 0; i < parts.length - 1; i++) {
      obj[parts[i]] = {}
      obj = obj[parts[i]]
    }
    obj[parts[parts.length - 1]] = value

    // Also update local cache immediately
    let current: any = serverConfig.value
    for (let i = 0; i < parts.length - 1; i++) {
      if (current[parts[i]] == null) current[parts[i]] = {}
      current = current[parts[i]]
    }
    current[parts[parts.length - 1]] = value

    return patchConfig(changes)
  }

  return {
    serverConfig,
    localConfig,
    loadConfig,
    patchConfig,
    restartServer,
    setLocalConfig,
    getServerValue,
    setServerValue,
  }
}

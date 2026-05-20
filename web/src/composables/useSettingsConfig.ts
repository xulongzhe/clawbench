import { reactive, ref } from 'vue'
import { apiGet, apiPatch, apiPost } from '@/utils/api'
import i18n, { STORAGE_KEY as LOCALE_KEY, setLocaleCookie } from '@/i18n'
import { useAgents } from '@/composables/useAgents'

const LOCAL_PREFIX = 'clawbench-settings-'

/**
 * Mapping from settings key → legacy localStorage key + write format.
 * Each entry tells setLocalConfig() how to also write to the key that
 * the actual feature reads from, so changes take effect immediately.
 */
const legacyKeys: Record<string, {
  key: string                    // legacy localStorage key
  format: 'raw' | 'json'        // raw = string value, json = JSON.stringify
  sideEffect?: (value: any) => void  // runtime side-effect for immediate effect
}> = {
  theme: {
    key: 'theme',
    format: 'raw',
    sideEffect(value: string) {
      // Resolve 'auto' to actual theme
      const resolved = value === 'auto'
        ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
        : value
      document.documentElement.setAttribute('data-theme', resolved)
      document.documentElement.setAttribute('data-hljs-theme', resolved)
      // Notify App.vue to sync its `theme` ref (used by provide/inject for chat rendering)
      window.dispatchEvent(new CustomEvent('clawbench-theme-change', { detail: resolved }))
    },
  },
  locale: {
    key: LOCALE_KEY,  // 'clawbench-locale'
    format: 'raw',
    sideEffect(value: string) {
      i18n.global.locale.value = value as any
      setLocaleCookie(value)
    },
  },
  autoSpeech: {
    key: 'clawbench-auto-speech',
    format: 'raw',
    sideEffect(value: boolean) {
      // Notify useAutoSpeech singleton to sync its `enabled` ref
      window.dispatchEvent(new CustomEvent('clawbench-autospeech-change', { detail: value }))
    },
  },
  showHidden: {
    key: 'clawbenchShowHidden',
    format: 'json',
    sideEffect(value: boolean) {
      // Notify App.vue to sync its `showHidden` ref
      window.dispatchEvent(new CustomEvent('clawbench-showhidden-change', { detail: value }))
    },
  },
  wordWrap: {
    key: 'clawbench-word-wrap',
    format: 'raw',
  },
  lineNumbers: {
    key: 'clawbench-line-numbers',
    format: 'raw',
  },
  fileView: {
    key: 'clawbench-file-view',
    format: 'raw',
  },
  terminalFontSize: {
    key: 'clawbench-terminal-font-size',
    format: 'raw',
  },
  androidLogCapture: {
    // No legacy key — this is a new setting
  },
}

/** Read initial value from legacy key (falls back to our own prefixed key, then default) */
function readLocalValue(settingsKey: string, defaultValue: any): any {
  const legacy = legacyKeys[settingsKey]
  // Try legacy key first (it's the source of truth if already set)
  if (legacy?.key) {
    try {
      const saved = localStorage.getItem(legacy.key)
      if (saved !== null) {
        if (legacy.format === 'json') {
          return JSON.parse(saved)
        }
        // Raw format: may need type coercion
        if (defaultValue === true || defaultValue === false) {
          return saved === 'true'
        }
        if (typeof defaultValue === 'number') {
          const n = Number(saved)
          if (!isNaN(n)) return n
        }
        return saved
      }
    } catch { /* ignore */ }
  }
  // Fallback: try our own prefixed key
  try {
    const saved = localStorage.getItem(LOCAL_PREFIX + settingsKey)
    if (saved !== null) return JSON.parse(saved)
  } catch { /* ignore */ }
  return defaultValue
}

const localDefaults: Record<string, any> = {
  theme: 'auto',
  locale: 'zh',
  autoSpeech: false,
  showHidden: false,
  wordWrap: false,
  lineNumbers: true,
  fileView: 'list',
  terminalFontSize: 12,
  androidLogCapture: false,
}

// Build reactive local config from legacy localStorage + defaults
const localConfig = reactive<Record<string, any>>({})
for (const key of Object.keys(localDefaults)) {
  localConfig[key] = readLocalValue(key, localDefaults[key])
}

const serverConfig = ref<Record<string, any>>({})

/**
 * Server config defaults mirroring backend ApplyDefaults() in internal/model/defaults.go.
 * Used as fallback when the API hasn't loaded yet, so items always display meaningful values.
 */
const serverDefaults: Record<string, any> = {
  'chat.initial_messages': 20,
  'chat.page_size': 20,
  'chat.collapsed_height': 150,
  'chat.system_prompt_interval': 10,
  'session.max_count': 10,
  'upload.max_size_mb': 100,
  'upload.max_files': 20,
  'terminal.enabled': true,
  'terminal.idle_timeout': '10m',
  'terminal.max_sessions': 10,
  'terminal.buffer_lines': 2000,
  'default_agent': '',
  'tts.engine': 'edge',
  'tts.format': '',
  'tts.summarize_backend': 'simple',
  'tts.speed': 1.0,
  'tts.max_cache_files': 100,
  'rag.enabled': false,
  'rag.ollama_base_url': 'http://localhost:11434',
  'rag.ollama_model': 'bge-m3',
  'rag.chunk_size': 512,
  'rag.search_limit': 5,
  'rag.retention_days': 90,
  'proxy.enabled': true,
  'proxy.allowed_ports': '1024-65535',
  'ssh.enabled': true,
  'ssh.port': 0,
  'push.jpush.enabled': false,
  'tts.piper.noise_scale': 0.667,
  'tts.piper.length_scale': 1.0,
  'tts.piper.sentence_silence': 0.2,
  'tts.kokoro.lang': 'cmn',
  'tts.moss_nano.voice': 'Junhao',
  'tts.moss_nano.backend': 'onnx',
  'tts.api.format': 'openai',
  'tasks.summarize_backend': '',
  'tasks.summarize_model': '',
}

// ── Agent preference helpers ──────────────────────────────
// Agent model and thinking effort preferences are stored server-side
// in agent YAML files via PATCH /api/agents.

/** Patch an agent's preferred_model or preferred_thinking_effort on the server. */
async function patchAgentPref(agentId: string, field: 'preferred_model' | 'preferred_thinking_effort', value: string): Promise<void> {
  await apiPatch('/api/agents', { id: agentId, [field]: value })
  // Also update the agent object in useAgents so the UI reflects immediately
  const { updateAgentField } = useAgents()
  updateAgentField(agentId, field === 'preferred_model' ? 'preferredModel' : 'preferredThinkingEffort', value)
}

/** Read the preferred model ID for an agent from the server-side agent data. */
function getAgentModelPref(agentId: string): string | null {
  const { getAgent } = useAgents()
  const agent = getAgent(agentId)
  return agent?.preferredModel || null
}

/** Read the preferred thinking effort for an agent from the server-side agent data. */
function getAgentThinkingPref(agentId: string): string | null {
  const { getAgent } = useAgents()
  const agent = getAgent(agentId)
  return agent?.preferredThinkingEffort || null
}

export function useSettingsConfig() {
  async function loadConfig() {
    try {
      const data = await apiGet<Record<string, any>>('/api/config')
      serverConfig.value = data
    } catch {
      // Server may be unreachable — keep existing cached values
    }
  }

  async function patchConfig(changes: Record<string, any>): Promise<{ needsRestart: boolean; changedColdFields: string[] }> {
    const result = await apiPatch<{ needsRestart?: boolean; changedColdFields?: string[] }>('/api/config', changes)
    // Merge patched values into local cache after successful response
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

    // Write to our own prefixed key (for persistence)
    try {
      localStorage.setItem(LOCAL_PREFIX + key, JSON.stringify(value))
    } catch { /* ignore */ }

    // Write to the legacy key that the actual feature reads from
    const legacy = legacyKeys[key]
    if (legacy?.key) {
      try {
        if (legacy.format === 'json') {
          localStorage.setItem(legacy.key, JSON.stringify(value))
        } else {
          localStorage.setItem(legacy.key, String(value))
        }
      } catch { /* ignore */ }
    }

    // Run side-effect for immediate runtime change
    if (legacy?.sideEffect) {
      legacy.sideEffect(value)
    }
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

  /** Read a server config value by dot-path, falling back to built-in defaults */
  function getServerValueWithDefault(dotPath: string): any {
    const value = getServerValue(dotPath)
    if (value !== undefined) return value
    return serverDefaults[dotPath]
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

    // Save old value for rollback on failure
    const oldValue = getServerValue(dotPath)

    // Optimistic local cache update
    let current: any = serverConfig.value
    for (let i = 0; i < parts.length - 1; i++) {
      if (current[parts[i]] == null) current[parts[i]] = {}
      current = current[parts[i]]
    }
    current[parts[parts.length - 1]] = value

    try {
      return await patchConfig(changes)
    } catch (err) {
      // Rollback local cache on failure
      let rollbackTarget: any = serverConfig.value
      for (let i = 0; i < parts.length - 1; i++) {
        if (rollbackTarget[parts[i]] == null) break
        rollbackTarget = rollbackTarget[parts[i]]
      }
      if (rollbackTarget && typeof rollbackTarget === 'object') {
        rollbackTarget[parts[parts.length - 1]] = oldValue
      }
      throw err
    }
  }

  return {
    serverConfig,
    localConfig,
    loadConfig,
    patchConfig,
    restartServer,
    setLocalConfig,
    getServerValue,
    getServerValueWithDefault,
    setServerValue,
    patchAgentPref,
    getAgentModelPref,
    getAgentThinkingPref,
  }
}

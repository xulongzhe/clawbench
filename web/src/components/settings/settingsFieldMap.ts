/**
 * Centralized settings item definitions — the single source of truth.
 *
 * Used by:
 * - SettingsCategory.vue (renders the UI)
 * - SettingsRestartDialog.vue (translates changed_cold_fields via serverFieldToLabelKey)
 *
 * Adding a new setting? Add it here. Both the category page and the restart
 * dialog will pick it up automatically — no manual sync required.
 */

export interface DependsOn {
  key: string
  value?: any
  values?: any[]
}

export interface ItemSpec {
  labelKey: string
  key: string
  type: 'switch' | 'select' | 'number' | 'text' | 'slider' | 'action' | 'info' | 'header' | 'password'
  source: 'server' | 'local'
  needsRestart?: boolean
  options?: { labelKey: string; value: any }[]
  min?: number
  max?: number
  step?: number
  dependsOn?: DependsOn
  sectionHeader?: string
}

/**
 * Complete category → items mapping.
 * The `agents` category is built dynamically at runtime, so it's an empty array here.
 */
export const categoryItems: Record<string, ItemSpec[]> = {
  appearance: [
    { labelKey: 'settings.items.theme', key: 'theme', type: 'select', source: 'local', options: [
      { labelKey: 'settings.items.themeAuto', value: 'auto' },
      { labelKey: 'settings.items.themeLight', value: 'light' },
      { labelKey: 'settings.items.themeDark', value: 'dark' },
    ]},
    { labelKey: 'settings.items.locale', key: 'locale', type: 'select', source: 'local', options: [
      { labelKey: 'settings.items.localeZh', value: 'zh' },
      { labelKey: 'settings.items.localeEn', value: 'en' },
    ]},
  ],
  chat: [
    { labelKey: 'settings.items.defaultAgent', key: 'default_agent', type: 'select', source: 'server', needsRestart: true },
    { labelKey: 'settings.items.autoSpeech', key: 'autoSpeech', type: 'switch', source: 'local' },
    { labelKey: 'settings.items.chatInitialMessages', key: 'chat.initial_messages', type: 'number', source: 'server' },
    { labelKey: 'settings.items.chatPageSize', key: 'chat.page_size', type: 'number', source: 'server' },
    { labelKey: 'settings.items.chatCollapsedHeight', key: 'chat.collapsed_height', type: 'number', source: 'server' },
    { labelKey: 'settings.items.chatSystemPromptInterval', key: 'chat.system_prompt_interval', type: 'number', source: 'server' },
    { labelKey: 'settings.items.sessionMaxCount', key: 'session.max_count', type: 'number', source: 'server' },
  ],
  agents: [],  // Dynamically built in computed items
  files: [
    { labelKey: 'settings.items.showHidden', key: 'showHidden', type: 'switch', source: 'local' },
    { labelKey: 'settings.items.wordWrap', key: 'wordWrap', type: 'switch', source: 'local' },
    { labelKey: 'settings.items.lineNumbers', key: 'lineNumbers', type: 'switch', source: 'local' },
    { labelKey: 'settings.items.fileView', key: 'fileView', type: 'select', source: 'local', options: [
      { labelKey: 'settings.items.fileViewList', value: 'list' },
      { labelKey: 'settings.items.fileViewGrid', value: 'grid' },
    ]},
    { labelKey: 'settings.items.uploadMaxSize', key: 'upload.max_size_mb', type: 'number', source: 'server' },
    { labelKey: 'settings.items.uploadMaxFiles', key: 'upload.max_files', type: 'number', source: 'server' },
  ],
  terminal: [
    { labelKey: 'settings.items.terminalFontSize', key: 'terminalFontSize', type: 'slider', source: 'local', min: 10, max: 24, step: 1 },
    { labelKey: 'settings.items.terminalEnabled', key: 'terminal.enabled', type: 'switch', source: 'server', needsRestart: true },
    { labelKey: 'settings.items.terminalIdleTimeout', key: 'terminal.idle_timeout', type: 'text', source: 'server' },
    { labelKey: 'settings.items.terminalMaxSessions', key: 'terminal.max_sessions', type: 'number', source: 'server' },
    { labelKey: 'settings.items.terminalBufferLines', key: 'terminal.buffer_lines', type: 'number', source: 'server' },
  ],
  tts: [
    // Engine selection (always shown)
    { labelKey: 'settings.items.ttsEngine', key: 'tts.engine', type: 'select', source: 'server', needsRestart: true, options: [
      { labelKey: 'settings.items.ttsEngineEdge', value: 'edge' },
      { labelKey: 'settings.items.ttsEngineMinimax', value: 'minimax' },
      { labelKey: 'settings.items.ttsEnginePiper', value: 'piper' },
      { labelKey: 'settings.items.ttsEngineKokoro', value: 'kokoro' },
      { labelKey: 'settings.items.ttsEngineMossNano', value: 'moss-nano' },
    ]},
    // Common fields (always shown)
    { labelKey: 'settings.items.ttsVoice', key: 'tts.voice', type: 'text', source: 'server' },
    { labelKey: 'settings.items.ttsSpeed', key: 'tts.speed', type: 'slider', source: 'server', min: 0.5, max: 3, step: 0.1 },
    // Minimax-specific
    { labelKey: 'settings.items.ttsModel', key: 'tts.tts_model', type: 'text', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'minimax' } },
    { labelKey: 'settings.items.ttsFormat', key: 'tts.format', type: 'select', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'minimax' }, options: [
      { labelKey: 'settings.items.ttsFormatDefault', value: '' },
      { labelKey: 'settings.items.ttsFormatMp3', value: 'mp3' },
      { labelKey: 'settings.items.ttsFormatWav', value: 'wav' },
      { labelKey: 'settings.items.ttsFormatPcm', value: 'pcm' },
    ]},
    // Piper sub-config
    { labelKey: 'settings.items.piperModelPath', key: 'tts.piper.model_path', type: 'text', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'piper' }, sectionHeader: 'settings.items.ttsPiperHeader' },
    { labelKey: 'settings.items.piperNoiseScale', key: 'tts.piper.noise_scale', type: 'number', source: 'server', min: 0, max: 1, step: 0.001,
      dependsOn: { key: 'tts.engine', value: 'piper' } },
    { labelKey: 'settings.items.piperLengthScale', key: 'tts.piper.length_scale', type: 'number', source: 'server', min: 0.1, max: 5, step: 0.1,
      dependsOn: { key: 'tts.engine', value: 'piper' } },
    { labelKey: 'settings.items.piperSentenceSilence', key: 'tts.piper.sentence_silence', type: 'number', source: 'server', min: 0, max: 5, step: 0.1,
      dependsOn: { key: 'tts.engine', value: 'piper' } },
    // Kokoro sub-config
    { labelKey: 'settings.items.kokoroModelPath', key: 'tts.kokoro.model_path', type: 'text', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'kokoro' }, sectionHeader: 'settings.items.ttsKokoroHeader' },
    { labelKey: 'settings.items.kokoroVoicesPath', key: 'tts.kokoro.voices_path', type: 'text', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'kokoro' } },
    { labelKey: 'settings.items.kokoroLang', key: 'tts.kokoro.lang', type: 'text', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'kokoro' } },
    // MossNano sub-config
    { labelKey: 'settings.items.mossNanoModelDir', key: 'tts.moss_nano.model_dir', type: 'text', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'moss-nano' }, sectionHeader: 'settings.items.ttsMossNanoHeader' },
    { labelKey: 'settings.items.mossNanoPromptSpeech', key: 'tts.moss_nano.prompt_speech', type: 'text', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'moss-nano' } },
    { labelKey: 'settings.items.mossNanoVoice', key: 'tts.moss_nano.voice', type: 'text', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'moss-nano' } },
    { labelKey: 'settings.items.mossNanoBackend', key: 'tts.moss_nano.backend', type: 'select', source: 'server',
      dependsOn: { key: 'tts.engine', value: 'moss-nano' }, options: [
      { labelKey: 'settings.items.mossNanoBackendOnnx', value: 'onnx' },
      { labelKey: 'settings.items.mossNanoBackendPytorch', value: 'pytorch' },
    ]},
    // Summarization
    { labelKey: 'settings.items.ttsSummarizeBackend', key: 'tts.summarize_backend', type: 'select', source: 'server',
      sectionHeader: 'settings.items.ttsSummarizeHeader', options: [
      { labelKey: 'settings.items.ttsSummarizeSimple', value: 'simple' },
      { labelKey: 'settings.items.ttsSummarizeApi', value: 'api' },
      { labelKey: 'settings.items.ttsSummarizeClaude', value: 'claude' },
      { labelKey: 'settings.items.ttsSummarizeCodebuddy', value: 'codebuddy' },
      { labelKey: 'settings.items.ttsSummarizeGemini', value: 'gemini' },
      { labelKey: 'settings.items.ttsSummarizeOpencode', value: 'opencode' },
      { labelKey: 'settings.items.ttsSummarizeCodex', value: 'codex' },
      { labelKey: 'settings.items.ttsSummarizeQoder', value: 'qoder' },
      { labelKey: 'settings.items.ttsSummarizeVecli', value: 'vecli' },
      { labelKey: 'settings.items.ttsSummarizeDeepseek', value: 'deepseek' },
      { labelKey: 'settings.items.ttsSummarizePi', value: 'pi' },
      { labelKey: 'settings.items.ttsSummarizeMmxcli', value: 'mmx-cli' },
    ]},
    { labelKey: 'settings.items.ttsSummarizeModel', key: 'tts.summarize_model', type: 'text', source: 'server' },
    // API sub-config (when summarize_backend=api)
    { labelKey: 'settings.items.apiBaseUrl', key: 'tts.api.base_url', type: 'text', source: 'server',
      dependsOn: { key: 'tts.summarize_backend', value: 'api' }, sectionHeader: 'settings.items.ttsApiHeader' },
    { labelKey: 'settings.items.apiKey', key: 'tts.api.key', type: 'password', source: 'server',
      dependsOn: { key: 'tts.summarize_backend', value: 'api' } },
    { labelKey: 'settings.items.apiFormat', key: 'tts.api.format', type: 'select', source: 'server',
      dependsOn: { key: 'tts.summarize_backend', value: 'api' }, options: [
      { labelKey: 'settings.items.apiFormatOpenai', value: 'openai' },
      { labelKey: 'settings.items.apiFormatAnthropic', value: 'anthropic' },
    ]},
    { labelKey: 'settings.items.apiModel', key: 'tts.api.model', type: 'text', source: 'server',
      dependsOn: { key: 'tts.summarize_backend', value: 'api' } },
    // Cache
    { labelKey: 'settings.items.ttsMaxCacheFiles', key: 'tts.max_cache_files', type: 'number', source: 'server',
      sectionHeader: 'settings.items.ttsCacheHeader' },
    // Tasks summarize
    { labelKey: 'settings.items.tasksSummarizeBackend', key: 'tasks.summarize_backend', type: 'select', source: 'server',
      sectionHeader: 'settings.items.tasksHeader', options: [
      { labelKey: 'settings.items.tasksSummarizeDisabled', value: '' },
      { labelKey: 'settings.items.ttsSummarizeSimple', value: 'simple' },
      { labelKey: 'settings.items.ttsSummarizeApi', value: 'api' },
      { labelKey: 'settings.items.ttsSummarizeClaude', value: 'claude' },
      { labelKey: 'settings.items.ttsSummarizeCodebuddy', value: 'codebuddy' },
      { labelKey: 'settings.items.ttsSummarizeGemini', value: 'gemini' },
      { labelKey: 'settings.items.ttsSummarizeOpencode', value: 'opencode' },
      { labelKey: 'settings.items.ttsSummarizeCodex', value: 'codex' },
      { labelKey: 'settings.items.ttsSummarizeQoder', value: 'qoder' },
      { labelKey: 'settings.items.ttsSummarizeVecli', value: 'vecli' },
      { labelKey: 'settings.items.ttsSummarizeDeepseek', value: 'deepseek' },
      { labelKey: 'settings.items.ttsSummarizePi', value: 'pi' },
      { labelKey: 'settings.items.ttsSummarizeMmxcli', value: 'mmx-cli' },
    ]},
    { labelKey: 'settings.items.tasksSummarizeModel', key: 'tasks.summarize_model', type: 'text', source: 'server' },
  ],
  rag: [
    { labelKey: 'settings.items.ragOllamaUrl', key: 'rag.ollama_base_url', type: 'text', source: 'server' },
    { labelKey: 'settings.items.ragOllamaModel', key: 'rag.ollama_model', type: 'text', source: 'server' },
    { labelKey: 'settings.items.ragChunkSize', key: 'rag.chunk_size', type: 'number', source: 'server' },
    { labelKey: 'settings.items.ragSearchLimit', key: 'rag.search_limit', type: 'number', source: 'server' },
    { labelKey: 'settings.items.ragSearchPoolSize', key: 'rag.search_pool_size', type: 'number', source: 'server' },
    { labelKey: 'settings.items.ragRetentionDays', key: 'rag.retention_days', type: 'number', source: 'server' },
  ],
  network: [
    { labelKey: 'settings.items.proxyEnabled', key: 'proxy.enabled', type: 'switch', source: 'server', needsRestart: true, sectionHeader: 'settings.items.proxyHeader' },
    { labelKey: 'settings.items.proxyAllowedPorts', key: 'proxy.allowed_ports', type: 'text', source: 'server' },
    { labelKey: 'settings.items.portForwardEnabled', key: 'port_forward.enabled', type: 'switch', source: 'server', needsRestart: true, sectionHeader: 'settings.items.portForwardHeader' },
    { labelKey: 'settings.items.portForwardPort', key: 'port_forward.port', type: 'number', source: 'server', needsRestart: true },
    { labelKey: 'settings.items.pushEnabled', key: 'push.jpush.enabled', type: 'switch', source: 'server', needsRestart: true, sectionHeader: 'settings.items.pushHeader' },
    { labelKey: 'settings.items.pushAppKey', key: 'push.jpush.app_key', type: 'text', source: 'server' },
  ],
  android: [
    { labelKey: 'settings.items.androidLogCapture', key: 'androidLogCapture', type: 'switch', source: 'local' },
    { labelKey: 'settings.items.reconfigureServer', key: 'reconfigureServer', type: 'action', source: 'local' },
  ],
  about: [
    { labelKey: 'settings.items.aboutServerVersion', key: 'serverVersion', type: 'info', source: 'server' },
    { labelKey: 'settings.items.aboutAppVersion', key: 'appVersion', type: 'info', source: 'local' },
    { labelKey: 'settings.items.serverRestart', key: 'restart', type: 'action', source: 'server' },
  ],
}

/** Build and return the mapping from server config dot-path keys to i18n label keys. */
export function getServerFieldToLabelKey(): Record<string, string> {
  const map: Record<string, string> = {}
  for (const items of Object.values(categoryItems)) {
    for (const item of items) {
      if (item.source === 'server') {
        map[item.key] = item.labelKey
      }
    }
  }
  return map
}

/** Pre-computed singleton — used by SettingsRestartDialog to translate field paths. */
export const serverFieldToLabelKey: Record<string, string> = getServerFieldToLabelKey()

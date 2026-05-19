<template>
  <div class="settings-category">
    <SettingsItem
      v-for="item in items"
      :key="item.key"
      :label="item.label"
      :type="item.type"
      :model-value="getItemValue(item)"
      :options="item.options"
      :min="item.min"
      :max="item.max"
      :step="item.step"
      :needs-restart="item.needsRestart"
      :force-close="openEditorKey !== null && openEditorKey !== item.key"
      @update:model-value="(v: any) => handleUpdate(item, v)"
      @click="handleClick(item)"
      @edit-toggle="(open: boolean) => handleEditToggle(item.key, open)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import SettingsItem from './SettingsItem.vue'
import { useSettingsConfig } from '@/composables/useSettingsConfig'
import { useAgents } from '@/composables/useAgents'

interface ItemSpec {
  labelKey: string
  key: string
  type: 'switch' | 'select' | 'number' | 'text' | 'slider' | 'action' | 'info'
  source: 'server' | 'local'
  needsRestart?: boolean
  options?: { labelKey: string; value: any }[]
  min?: number
  max?: number
  step?: number
}

const props = defineProps<{
  categoryId: string
}>()

const emit = defineEmits<{
  navigate: [categoryId: string]
  restartNeeded: [changedFields: string[]]
}>()

const { t } = useI18n()
const { localConfig, setLocalConfig, getServerValueWithDefault, setServerValue, getAgentModelPref, setAgentModelPref, getAgentThinkingPref, setAgentThinkingPref } = useSettingsConfig()
const { agents, loadAgents, getAgentModels, getAgentThinkingEffortLevels, hasThinkingEffortLevels, getDefaultModelId } = useAgents()

const openEditorKey = ref<string | null>(null)

// Load agents when this category is shown
watch(() => props.categoryId, (id) => {
  if (id === 'chat' || id === 'agents') loadAgents()
}, { immediate: true })

const categoryItems: Record<string, ItemSpec[]> = {
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
    { labelKey: 'settings.items.autoSpeech', key: 'autoSpeech', type: 'switch', source: 'local' },
    { labelKey: 'settings.items.chatInitialMessages', key: 'chat.initial_messages', type: 'number', source: 'server' },
    { labelKey: 'settings.items.chatPageSize', key: 'chat.page_size', type: 'number', source: 'server' },
    { labelKey: 'settings.items.chatCollapsedHeight', key: 'chat.collapsed_height', type: 'number', source: 'server' },
    { labelKey: 'settings.items.chatSystemPromptInterval', key: 'chat.system_prompt_interval', type: 'number', source: 'server' },
    { labelKey: 'settings.items.sessionMaxCount', key: 'session.max_count', type: 'number', source: 'server', needsRestart: true },
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
    { labelKey: 'settings.items.ttsEngine', key: 'tts.engine', type: 'select', source: 'server', needsRestart: true, options: [
      { labelKey: 'settings.items.ttsEngineEdge', value: 'edge' },
      { labelKey: 'settings.items.ttsEngineMinimax', value: 'minimax' },
      { labelKey: 'settings.items.ttsEnginePiper', value: 'piper' },
      { labelKey: 'settings.items.ttsEngineKokoro', value: 'kokoro' },
      { labelKey: 'settings.items.ttsEngineMossNano', value: 'moss-nano' },
    ]},
    { labelKey: 'settings.items.ttsModel', key: 'tts.tts_model', type: 'text', source: 'server' },
    { labelKey: 'settings.items.ttsFormat', key: 'tts.format', type: 'select', source: 'server', options: [
      { labelKey: 'settings.items.ttsFormatDefault', value: '' },
      { labelKey: 'settings.items.ttsFormatMp3', value: 'mp3' },
      { labelKey: 'settings.items.ttsFormatWav', value: 'wav' },
      { labelKey: 'settings.items.ttsFormatPcm', value: 'pcm' },
    ]},
    { labelKey: 'settings.items.ttsSummarizeBackend', key: 'tts.summarize_backend', type: 'select', source: 'server', options: [
      { labelKey: 'settings.items.ttsSummarizeSimple', value: 'simple' },
      { labelKey: 'settings.items.ttsSummarizeApi', value: 'api' },
      { labelKey: 'settings.items.ttsSummarizeClaude', value: 'claude' },
      { labelKey: 'settings.items.ttsSummarizeCodebuddy', value: 'codebuddy' },
    ]},
    { labelKey: 'settings.items.ttsSummarizeModel', key: 'tts.summarize_model', type: 'text', source: 'server' },
    { labelKey: 'settings.items.ttsSpeed', key: 'tts.speed', type: 'slider', source: 'server', min: 0.5, max: 2, step: 0.1 },
    { labelKey: 'settings.items.ttsVoice', key: 'tts.voice', type: 'text', source: 'server' },
    { labelKey: 'settings.items.ttsMaxCacheFiles', key: 'tts.max_cache_files', type: 'number', source: 'server' },
  ],
  rag: [
    { labelKey: 'settings.items.ragEnabled', key: 'rag.enabled', type: 'switch', source: 'server', needsRestart: true },
    { labelKey: 'settings.items.ragOllamaUrl', key: 'rag.ollama_base_url', type: 'text', source: 'server' },
    { labelKey: 'settings.items.ragOllamaModel', key: 'rag.ollama_model', type: 'text', source: 'server' },
    { labelKey: 'settings.items.ragChunkSize', key: 'rag.chunk_size', type: 'number', source: 'server' },
    { labelKey: 'settings.items.ragSearchLimit', key: 'rag.search_limit', type: 'number', source: 'server' },
    { labelKey: 'settings.items.ragRetentionDays', key: 'rag.retention_days', type: 'number', source: 'server' },
  ],
  network: [
    { labelKey: 'settings.items.proxyEnabled', key: 'proxy.enabled', type: 'switch', source: 'server', needsRestart: true },
    { labelKey: 'settings.items.proxyAllowedPorts', key: 'proxy.allowed_ports', type: 'text', source: 'server' },
    { labelKey: 'settings.items.sshEnabled', key: 'ssh.enabled', type: 'switch', source: 'server', needsRestart: true },
    { labelKey: 'settings.items.sshPort', key: 'ssh.port', type: 'number', source: 'server', needsRestart: true },
    { labelKey: 'settings.items.pushEnabled', key: 'push.jpush.enabled', type: 'switch', source: 'server', needsRestart: true },
    { labelKey: 'settings.items.pushAppKey', key: 'push.jpush.app_key', type: 'text', source: 'server' },
  ],
  about: [
    { labelKey: 'settings.items.serverRestart', key: 'restart', type: 'action', source: 'server' },
  ],
}

// Resolve i18n labels at runtime, and dynamically inject agent options
const items = computed(() => {
  // For the 'agents' category, dynamically build items from the agent list
  if (props.categoryId === 'agents') {
    const result: any[] = []
    for (const agent of agents.value) {
      // Agent group header — just shows the agent icon + name as a label
      result.push({
        key: `agent-header-${agent.id}`,
        label: `${agent.icon} ${agent.name}`,
        labelKey: '',
        type: 'info' as const,
        source: 'local' as const,
        modelValue: '',
      })
      // Model preference (only if agent has multiple models)
      const models = getAgentModels(agent.id)
      if (models.length > 1) {
        const savedModel = getAgentModelPref(agent.id)
        const currentModel = savedModel || getDefaultModelId(agent.id)
        result.push({
          key: `agent-model-${agent.id}`,
          label: t('settings.items.agentModel'),
          labelKey: 'settings.items.agentModel',
          type: 'select' as const,
          source: 'local' as const,
          modelValue: currentModel,
          options: models.map((m: any) => ({
            label: m.name || m.id,
            value: m.id,
          })),
        })
      }
      // Thinking effort preference (only if agent supports it)
      if (hasThinkingEffortLevels(agent.id)) {
        const levels = getAgentThinkingEffortLevels(agent.id)
        const savedThinking = getAgentThinkingPref(agent.id)
        const currentThinking = savedThinking || agent.thinkingEffort || ''
        result.push({
          key: `agent-thinking-${agent.id}`,
          label: t('settings.items.agentThinking'),
          labelKey: 'settings.items.agentThinking',
          type: 'select' as const,
          source: 'local' as const,
          modelValue: currentThinking,
          options: levels.map((level: string) => ({
            label: level,
            value: level,
          })),
        })
      }
    }
    return result
  }

  const raw = categoryItems[props.categoryId] ?? []
  return raw.map(item => {
    // Dynamically build options for default_agent from the agents list
    let resolvedOptions = item.options
    if (item.key === 'default_agent') {
      resolvedOptions = agents.value.map(a => ({
        labelKey: '',
        value: a.id,
        label: `${a.icon} ${a.name}`,
      }))
    }

    return {
      ...item,
      label: t(item.labelKey),
      options: resolvedOptions?.map(opt => ({
        ...opt,
        label: opt.label || resolveOptionLabel(item.key, opt),
      })),
    }
  })
})

function resolveOptionLabel(itemKey: string, opt: { labelKey: string; value: any }): string {
  // Special cases where option labels differ from item label
  if (itemKey === 'theme') {
    const themeLabels: Record<string, string> = {
      auto: t('settings.items.themeAuto'),
      light: t('settings.items.themeLight'),
      dark: t('settings.items.themeDark'),
    }
    return themeLabels[opt.value] ?? String(opt.value)
  }
  if (itemKey === 'locale') {
    const localeLabels: Record<string, string> = {
      zh: t('settings.items.localeZh'),
      en: t('settings.items.localeEn'),
    }
    return localeLabels[opt.value] ?? String(opt.value)
  }
  if (itemKey === 'fileView') {
    const viewLabels: Record<string, string> = {
      list: t('settings.items.fileViewList'),
      grid: t('settings.items.fileViewGrid'),
    }
    return viewLabels[opt.value] ?? String(opt.value)
  }
  // For all other select options, the labelKey is already set to the i18n key
  if (opt.labelKey) return t(opt.labelKey)
  return String(opt.value)
}

function getItemValue(item: any): any {
  // Agent model/thinking prefs are handled specially
  if (item.key?.startsWith('agent-model-')) {
    return item.modelValue
  }
  if (item.key?.startsWith('agent-thinking-')) {
    return item.modelValue
  }
  if (item.key?.startsWith('agent-header-')) {
    return ''
  }
  if (item.source === 'local') {
    return localConfig[item.key]
  }
  return getServerValueWithDefault(item.key)
}

async function handleUpdate(item: any, value: any) {
  // Agent model preference
  if (item.key?.startsWith('agent-model-')) {
    const agentId = item.key.replace('agent-model-', '')
    setAgentModelPref(agentId, value)
    return
  }
  // Agent thinking preference
  if (item.key?.startsWith('agent-thinking-')) {
    const agentId = item.key.replace('agent-thinking-', '')
    setAgentThinkingPref(agentId, value)
    return
  }
  if (item.source === 'local') {
    setLocalConfig(item.key, value)
    return
  }
  // Server config: auto-save immediately
  try {
    const result = await setServerValue(item.key, value)
    if (result.needsRestart && result.changedColdFields.length > 0) {
      emit('restartNeeded', result.changedColdFields)
    }
  } catch {
    // Silently ignore — value will revert on next config load
  }
}

function handleClick(item: any) {
  if (item.key === 'restart') {
    emit('restartNeeded', [])
  }
}

function handleEditToggle(key: string, open: boolean) {
  if (open) {
    openEditorKey.value = key
  } else if (openEditorKey.value === key) {
    openEditorKey.value = null
  }
}
</script>

<style scoped>
.settings-category {
  padding: 8px 0;
  background: var(--bg-secondary);
  min-height: 100%;
}
</style>

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
      @update:model-value="(v: any) => handleUpdate(item, v)"
      @click="handleClick(item)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import SettingsItem from './SettingsItem.vue'
import { useSettingsConfig } from '@/composables/useSettingsConfig'

interface ItemSpec {
  label: string
  key: string
  type: 'switch' | 'select' | 'number' | 'text' | 'slider' | 'action'
  source: 'server' | 'local'
  needsRestart?: boolean
  options?: { label: string; value: any }[]
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

const { localConfig, setLocalConfig, getServerValue, setServerValue } = useSettingsConfig()

const categoryItems: Record<string, ItemSpec[]> = {
  appearance: [
    { label: '主题', key: 'theme', type: 'select', source: 'local', options: [
      { label: '跟随系统', value: 'auto' },
      { label: '浅色', value: 'light' },
      { label: '深色', value: 'dark' },
    ]},
    { label: '语言', key: 'locale', type: 'select', source: 'local', options: [
      { label: '中文', value: 'zh' },
      { label: 'English', value: 'en' },
    ]},
  ],
  chat: [
    { label: '自动语音', key: 'autoSpeech', type: 'switch', source: 'local' },
  ],
  agents: [
    { label: '默认模型', key: 'default_agent', type: 'select', source: 'server', needsRestart: true, options: [] },
  ],
  fileManager: [
    { label: '显示隐藏文件', key: 'showHidden', type: 'switch', source: 'local' },
  ],
  fileViewer: [
    { label: '自动换行', key: 'wordWrap', type: 'switch', source: 'local' },
    { label: '行号', key: 'lineNumbers', type: 'switch', source: 'local' },
    { label: '默认视图', key: 'fileView', type: 'select', source: 'local', options: [
      { label: '列表', value: 'list' },
      { label: '网格', value: 'grid' },
    ]},
  ],
  terminal: [
    { label: '字体大小', key: 'terminalFontSize', type: 'slider', source: 'local', min: 10, max: 24, step: 1 },
  ],
  tts: [
    { label: 'TTS引擎', key: 'tts.engine', type: 'select', source: 'server', needsRestart: true, options: [
      { label: 'MiniMax', value: 'minimax' },
      { label: 'Edge TTS', value: 'edge' },
      { label: 'Piper', value: 'piper' },
    ]},
    { label: '语速', key: 'tts.speed', type: 'slider', source: 'server', min: 0.5, max: 2, step: 0.1 },
  ],
  rag: [
    { label: '启用RAG', key: 'rag.enabled', type: 'switch', source: 'server', needsRestart: true },
    { label: 'Ollama地址', key: 'rag.ollama_base_url', type: 'text', source: 'server' },
    { label: 'Embedding模型', key: 'rag.ollama_model', type: 'text', source: 'server' },
  ],
  proxy: [
    { label: '启用端口转发', key: 'proxy.enabled', type: 'switch', source: 'server', needsRestart: true },
  ],
  ssh: [
    { label: '启用SSH隧道', key: 'ssh.enabled', type: 'switch', source: 'server', needsRestart: true },
    { label: 'SSH端口', key: 'ssh.port', type: 'number', source: 'server', needsRestart: true },
  ],
  push: [
    { label: '启用JPush', key: 'push.jpush.enabled', type: 'switch', source: 'server', needsRestart: true },
    { label: 'AppKey', key: 'push.jpush.app_key', type: 'text', source: 'server' },
  ],
  android: [
    { label: '日志捕获', key: 'androidLogCapture', type: 'switch', source: 'local' },
  ],
  server: [
    { label: '端口', key: 'server.port', type: 'number', source: 'server', needsRestart: true },
    { label: '日志级别', key: 'server.log_level', type: 'select', source: 'server', needsRestart: true, options: [
      { label: 'Debug', value: 'debug' },
      { label: 'Info', value: 'info' },
      { label: 'Warn', value: 'warn' },
      { label: 'Error', value: 'error' },
    ]},
    { label: '重启服务器', key: 'restart', type: 'action', source: 'server' },
  ],
  about: [
    { label: '版本', key: 'version', type: 'text', source: 'server' },
  ],
}

const items = computed(() => categoryItems[props.categoryId] ?? [])

function getItemValue(item: ItemSpec): any {
  if (item.source === 'local') {
    return localConfig[item.key]
  }
  return getServerValue(item.key)
}

async function handleUpdate(item: ItemSpec, value: any) {
  if (item.source === 'local') {
    setLocalConfig(item.key, value)
    return
  }
  try {
    const result = await setServerValue(item.key, value)
    if (result.needsRestart && result.changedColdFields.length > 0) {
      emit('restartNeeded', result.changedColdFields)
    }
  } catch {
    // Silently ignore
  }
}

function handleClick(item: ItemSpec) {
  if (item.key === 'restart') {
    emit('restartNeeded', [])
  }
}
</script>

<style scoped>
.settings-category {
  padding: 0 0 8px;
}
</style>

<template>
  <div class="settings-restart-overlay" @click.self="$emit('later')">
    <div class="settings-restart-dialog">
      <div class="settings-restart-dialog__header">{{ t('settings.restartConfirmTitle') }}</div>
      <p class="settings-restart-dialog__message">{{ t('settings.restartConfirmMessage') }}</p>
      <ul v-if="changedFields.length > 0" class="settings-restart-dialog__list">
        <li v-for="field in displayFields" :key="field">{{ field }}</li>
      </ul>
      <div class="settings-restart-dialog__actions">
        <button class="settings-restart-dialog__btn settings-restart-dialog__btn--later" @click="$emit('later')">
          {{ t('settings.restartLater') }}
        </button>
        <button class="settings-restart-dialog__btn settings-restart-dialog__btn--restart" @click="$emit('restart')">
          {{ t('settings.restartNow') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  changedFields: string[]
}>()

defineEmits<{
  restart: []
  later: []
}>()

const { t } = useI18n()

// Map server dot-path keys to i18n label keys for user-friendly display
const fieldToLabelKey: Record<string, string> = {
  'default_agent': 'settings.items.defaultAgent',
  'chat.initial_messages': 'settings.items.chatInitialMessages',
  'chat.page_size': 'settings.items.chatPageSize',
  'chat.collapsed_height': 'settings.items.chatCollapsedHeight',
  'chat.system_prompt_interval': 'settings.items.chatSystemPromptInterval',
  'session.max_count': 'settings.items.sessionMaxCount',
  'upload.max_size_mb': 'settings.items.uploadMaxSize',
  'upload.max_files': 'settings.items.uploadMaxFiles',
  'terminal.enabled': 'settings.items.terminalEnabled',
  'terminal.idle_timeout': 'settings.items.terminalIdleTimeout',
  'terminal.max_sessions': 'settings.items.terminalMaxSessions',
  'terminal.buffer_lines': 'settings.items.terminalBufferLines',
  'tts.engine': 'settings.items.ttsEngine',
  'tts.format': 'settings.items.ttsFormat',
  'tts.summarize_backend': 'settings.items.ttsSummarizeBackend',
  'tts.summarize_model': 'settings.items.ttsSummarizeModel',
  'tts.speed': 'settings.items.ttsSpeed',
  'tts.voice': 'settings.items.ttsVoice',
  'tts.max_cache_files': 'settings.items.ttsMaxCacheFiles',
  'rag.enabled': 'settings.items.ragEnabled',
  'rag.ollama_base_url': 'settings.items.ragOllamaUrl',
  'rag.ollama_model': 'settings.items.ragOllamaModel',
  'rag.chunk_size': 'settings.items.ragChunkSize',
  'rag.search_limit': 'settings.items.ragSearchLimit',
  'rag.retention_days': 'settings.items.ragRetentionDays',
  'proxy.enabled': 'settings.items.proxyEnabled',
  'proxy.allowed_ports': 'settings.items.proxyAllowedPorts',
  'ssh.enabled': 'settings.items.sshEnabled',
  'ssh.port': 'settings.items.sshPort',
  'push.jpush.enabled': 'settings.items.pushEnabled',
  'push.jpush.app_key': 'settings.items.pushAppKey',
  'tasks.summarize_backend': 'settings.items.tasksSummarizeBackend',
  'tasks.summarize_model': 'settings.items.tasksSummarizeModel',
}

const displayFields = computed(() =>
  props.changedFields.map(key => {
    const labelKey = fieldToLabelKey[key]
    return labelKey ? t(labelKey) : key
  })
)
</script>

<style scoped>
.settings-restart-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
  -webkit-backdrop-filter: blur(4px);
  backdrop-filter: blur(4px);
}

.settings-restart-dialog {
  background: var(--bg-primary);
  border-radius: 14px;
  padding: 20px;
  margin: 24px;
  max-width: 320px;
  width: 100%;
  box-shadow: var(--shadow-md);
}

.settings-restart-dialog__header {
  font-size: 17px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
  text-align: center;
}

.settings-restart-dialog__message {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0 0 12px;
  text-align: center;
}

.settings-restart-dialog__list {
  margin: 0 0 20px;
  padding-left: 20px;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.6;
}

.settings-restart-dialog__list li {
  margin-bottom: 2px;
}

.settings-restart-dialog__actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.settings-restart-dialog__btn {
  width: 100%;
  padding: 12px 16px;
  border: none;
  border-radius: 10px;
  font-size: 16px;
  font-weight: 500;
  cursor: pointer;
  text-align: center;
}

.settings-restart-dialog__btn--later {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

@media (hover: hover) {
  .settings-restart-dialog__btn--later:hover {
    background: var(--bg-secondary);
  }
}

.settings-restart-dialog__btn--later:active {
  background: var(--bg-tertiary);
}

.settings-restart-dialog__btn--restart {
  background: var(--accent-color);
  color: #fff;
}

@media (hover: hover) {
  .settings-restart-dialog__btn--restart:hover {
    background: var(--accent-hover);
  }
}

.settings-restart-dialog__btn--restart:active {
  background: var(--accent-hover);
}
</style>

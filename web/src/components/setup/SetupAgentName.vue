<template>
  <div class="setup-step setup-agent-name">
    <h3 class="step-title">{{ t('setup.nameYourAgent') }}</h3>
    <p class="step-desc">{{ t('setup.nameYourAgentHint') }}</p>

    <!-- Agent icon preview -->
    <div class="agent-preview">
      <span class="agent-preview-icon">🥧</span>
      <span class="agent-preview-name">{{ agentNameValue || t('setup.agentNamePlaceholder') }}</span>
    </div>

    <!-- Agent Name -->
    <div class="input-group">
      <label class="input-label">{{ t('setup.agentName') }}</label>
      <input
        v-model="agentNameValue"
        type="text"
        class="setup-input"
        :placeholder="t('setup.agentNamePlaceholder')"
        maxlength="50"
        autocomplete="off"
      />
    </div>

    <!-- Agent ID -->
    <div class="input-group">
      <label class="input-label">
        {{ t('setup.agentId') }}
        <span class="input-label-hint">{{ t('setup.agentIdHint') }}</span>
      </label>
      <input
        v-model="agentIdValue"
        type="text"
        class="setup-input setup-input--mono"
        :placeholder="t('setup.agentIdPlaceholder')"
        maxlength="50"
        autocomplete="off"
      />
    </div>

    <!-- Error -->
    <div v-if="error" class="setup-error">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
      </svg>
      {{ error }}
    </div>

    <!-- Navigation -->
    <div class="step-nav">
      <button class="setup-btn-secondary" @click="$emit('back')">{{ t('setup.back') }}</button>
      <button class="setup-btn-primary complete-btn" :disabled="!canProceed || completing" @click="handleComplete">
        <span v-if="completing" class="btn-spinner"></span>
        {{ t('setup.completeSetup') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  agentName: string
  agentId: string
  completing: boolean
}>()

const emit = defineEmits<{
  'update:agentName': [value: string]
  'update:agentId': [value: string]
  complete: []
  back: []
}>()

const { t } = useI18n()
const error = ref('')

const agentNameValue = computed({
  get: () => props.agentName,
  set: (v) => emit('update:agentName', v),
})

const agentIdValue = computed({
  get: () => props.agentId,
  set: (v) => emit('update:agentId', v.replace(/[^a-z0-9-]/g, '-')),
})

const canProceed = computed(() => {
  return props.agentName.trim() !== '' && props.agentId.trim() !== ''
})

function handleComplete() {
  if (!canProceed.value) return
  error.value = ''
  emit('complete')
}
</script>

<style scoped>
.setup-agent-name {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.step-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.step-desc {
  font-size: 13px;
  color: var(--text-muted);
  margin: -8px 0 0;
}

.agent-preview {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border-radius: var(--radius-md, 10px);
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
}

.agent-preview-icon {
  font-size: 28px;
}

.agent-preview-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.input-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.input-label-hint {
  font-size: 11px;
  font-weight: 400;
  color: var(--text-muted);
}

.setup-input {
  width: 100%;
  padding: 10px 14px;
  border: 1.5px solid var(--border-color);
  border-radius: var(--radius-md, 10px);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
  box-sizing: border-box;
  transition: border-color 0.2s;
}

.setup-input:focus {
  border-color: var(--accent-color);
}

.setup-input--mono {
  font-family: monospace;
  font-size: 13px;
}

.complete-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.btn-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>

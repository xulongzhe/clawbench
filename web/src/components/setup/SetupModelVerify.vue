<template>
  <div class="setup-step setup-model-verify">
    <h3 class="step-title">{{ t('setup.selectModel') }}</h3>

    <!-- Chat model selection -->
    <div class="model-group">
      <label class="model-label">
        {{ t('setup.chatModel') }}
      </label>

      <!-- Manual input for empty model list -->
      <div v-if="models.length === 0 && !modelsLoading" class="input-wrap">
        <input
          v-model="chatModelValue"
          type="text"
          class="setup-input"
          :placeholder="t('setup.enterModelId')"
          autocomplete="off"
        />
      </div>

      <!-- Model list -->
      <div v-else class="model-select-wrap">
        <select v-model="chatModelValue" class="model-select" :disabled="modelsLoading">
          <option v-for="m in models" :key="m.id" :value="m.id">
            {{ m.name || m.id }}
            <template v-if="m.context_length"> ({{ formatContext(m.context_length) }})</template>
          </option>
        </select>
      </div>
    </div>

    <!-- Summarize model selection -->
    <div class="model-group">
      <label class="model-label">
        {{ t('setup.summarizeModel') }}
      </label>

      <div v-if="models.length === 0 && !modelsLoading" class="input-wrap">
        <input
          v-model="summarizeModelValue"
          type="text"
          class="setup-input"
          :placeholder="t('setup.enterModelId')"
          autocomplete="off"
        />
      </div>

      <div v-else class="model-select-wrap">
        <select v-model="summarizeModelValue" class="model-select" :disabled="modelsLoading">
          <option v-for="m in models" :key="m.id" :value="m.id">
            {{ m.name || m.id }}
          </option>
        </select>
      </div>
    </div>

    <!-- Models loading/error -->
    <div v-if="modelsLoading" class="model-loading">
      <span class="btn-spinner"></span>
      <span>{{ t('setup.loadingModels') }}</span>
    </div>
    <div v-else-if="modelsError" class="setup-error">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
        <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
      </svg>
      {{ modelsError }}
    </div>

    <!-- Verification section -->
    <div class="verify-section">
      <button
        class="setup-btn-secondary verify-btn"
        :disabled="!chatModel || verifying || modelsLoading"
        @click="handleVerify"
      >
        <span v-if="verifying" class="btn-spinner"></span>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
          <polyline points="22 4 12 14.01 9 11.01"/>
        </svg>
        {{ verifying ? t('setup.verifying') : t('setup.verifyConfig') }}
      </button>

      <!-- Verify result -->
      <div v-if="verifyResult" class="verify-result" :class="{ 'verify-success': verifyResult.success, 'verify-fail': !verifyResult.success }">
        <svg v-if="verifyResult.success" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
          <polyline points="22 4 12 14.01 9 11.01"/>
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
        </svg>
        <span>{{ verifyResult.message }}</span>
      </div>
    </div>

    <!-- Navigation -->
    <div class="step-nav">
      <button class="setup-btn-secondary" @click="$emit('back')">{{ t('setup.back') }}</button>
      <button
        class="setup-btn-primary"
        :disabled="!canProceed"
        @click="$emit('next')"
      >
        {{ t('setup.next') }}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <path d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  models: { id: string; name: string; context_length?: number; cost_tier?: string }[]
  modelsLoading: boolean
  modelsError: string
  summarizeModelHint: string
  chatModel: string
  summarizeModel: string
  verifying: boolean
  verifyResult: { success: boolean; message: string } | null
}>()

const emit = defineEmits<{
  'update:chatModel': [value: string]
  'update:summarizeModel': [value: string]
  verify: []
  next: []
  back: []
}>()

const { t } = useI18n()

const chatModelValue = computed({
  get: () => props.chatModel,
  set: (v) => emit('update:chatModel', v),
})

const summarizeModelValue = computed({
  get: () => props.summarizeModel,
  set: (v) => emit('update:summarizeModel', v),
})

const canProceed = computed(() => {
  return props.chatModel &&
    props.summarizeModel &&
    props.verifyResult?.success === true
})

function formatContext(tokens: number): string {
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M`
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(0)}K`
  return String(tokens)
}

function handleVerify() {
  emit('verify')
}
</script>

<style scoped>
.setup-model-verify {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.model-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.model-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
}

.model-select-wrap {
  position: relative;
}

.model-select {
  width: 100%;
  padding: 8px 10px;
  border: 1.5px solid var(--border-color);
  border-radius: var(--radius-sm, 6px);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  box-sizing: border-box;
  appearance: none;
  cursor: pointer;
}

.model-select:focus {
  border-color: var(--accent-color);
}

.setup-input {
  width: 100%;
  padding: 8px 10px;
  border: 1.5px solid var(--border-color);
  border-radius: var(--radius-sm, 6px);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  box-sizing: border-box;
}

.setup-input:focus {
  border-color: var(--accent-color);
}

.model-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-muted);
  padding: 4px 0;
}

.verify-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.verify-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  padding: 8px 12px;
  font-size: 13px;
}

.verify-result {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 10px;
  border-radius: var(--radius-sm, 6px);
  font-size: 12px;
}

.verify-success {
  background: color-mix(in srgb, var(--color-green, #16a34a) 10%, var(--bg-primary));
  border: 1px solid color-mix(in srgb, var(--color-green, #16a34a) 25%, var(--border-color));
  color: var(--color-green, #16a34a);
}

.verify-fail {
  background: color-mix(in srgb, var(--color-red, #dc2626) 8%, var(--bg-primary));
  border: 1px solid color-mix(in srgb, var(--color-red, #dc2626) 20%, var(--border-color));
  color: var(--color-red, #dc2626);
}

.btn-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid color-mix(in srgb, currentColor 30%, transparent);
  border-top-color: currentColor;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
  flex-shrink: 0;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>

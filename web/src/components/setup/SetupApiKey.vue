<template>
  <div class="setup-step setup-apikey">
    <h3 class="step-title">{{ t('setup.enterApiKey') }}</h3>

    <!-- Custom URL input (only for _custom provider) -->
    <div v-if="provider === '_custom'" class="input-group">
      <label class="input-label">{{ t('setup.customBaseUrl') }}</label>
      <div class="input-wrap">
        <svg class="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
          <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
        </svg>
        <input
          v-model="customUrlModel"
          type="url"
          class="setup-input"
          :class="{ 'setup-input--warning': urlWarning }"
          :placeholder="apiFormatModel === 'anthropic' ? 'https://api.example.com/v1/messages' : 'https://api.example.com/v1/chat/completions'"
          autocomplete="off"
        />
      </div>
      <div v-if="urlWarning" class="url-warning">{{ urlWarning }}</div>
    </div>

    <!-- API Format selector (only for _custom provider) -->
    <div v-if="provider === '_custom'" class="input-group">
      <label class="input-label">{{ t('setup.apiFormat') }}</label>
      <div class="format-toggle">
        <button
          class="format-btn"
          :class="{ 'format-btn--active': apiFormatModel === 'openai' }"
          @click="apiFormatModel = 'openai'"
        >OpenAI</button>
        <button
          class="format-btn"
          :class="{ 'format-btn--active': apiFormatModel === 'anthropic' }"
          @click="apiFormatModel = 'anthropic'"
        >Anthropic</button>
      </div>
    </div>

    <!-- API Key input -->
    <div class="input-group">
      <label class="input-label">{{ t('setup.apiKey') }}</label>
      <div class="input-wrap">
        <svg class="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
          <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
        </svg>
        <input
          ref="apiKeyInputRef"
          v-model="apiKeyModel"
          :type="showKey ? 'text' : 'password'"
          class="setup-input"
          :placeholder="provider === '_custom' ? t('setup.apiKeyPlaceholder') : providerEnvVar"
          autocomplete="off"
          @keydown.enter="handleNext"
        />
        <button class="input-toggle" @click="showKey = !showKey" :title="showKey ? t('setup.hideKey') : t('setup.showKey')">
          <svg v-if="showKey" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/>
            <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/>
            <line x1="1" y1="1" x2="23" y2="23"/>
          </svg>
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
            <circle cx="12" cy="12" r="3"/>
          </svg>
        </button>
      </div>
    </div>

    <!-- Error -->
    <div v-if="error" class="setup-error">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
        <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
      </svg>
      {{ error }}
    </div>

    <!-- Navigation -->
    <div class="step-nav">
      <button class="setup-btn-secondary" @click="$emit('back')">{{ t('setup.back') }}</button>
      <button class="setup-btn-primary" :disabled="!canProceed" @click="handleNext">
        {{ t('setup.next') }}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <path d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  provider: string
  providerName: string
  providerEnvVar: string
  customUrl: string
  apiKey: string
  apiFormat: string
}>()

const emit = defineEmits<{
  'update:customUrl': [value: string]
  'update:apiKey': [value: string]
  'update:apiFormat': [value: string]
  next: []
  back: []
}>()

const { t } = useI18n()

const customUrlModel = computed({
  get: () => props.customUrl,
  set: (v) => emit('update:customUrl', v),
})

const apiKeyModel = computed({
  get: () => props.apiKey,
  set: (v) => emit('update:apiKey', v),
})

const apiFormatModel = computed({
  get: () => props.apiFormat,
  set: (v) => emit('update:apiFormat', v),
})

const showKey = ref(false)
const error = ref('')
const apiKeyInputRef = ref<HTMLInputElement | null>(null)

// URL format validation for custom URL
const OPENAI_SUFFIX = '/chat/completions'
const ANTHROPIC_SUFFIX = '/v1/messages'

const urlWarning = computed(() => {
  if (props.provider !== '_custom' || !props.customUrl.trim()) return ''
  const url = props.customUrl.trim()
  if (props.apiFormat === 'anthropic') {
    if (!url.endsWith(ANTHROPIC_SUFFIX)) {
      return t('setup.urlFormatWarning.anthropic')
    }
  } else {
    if (!url.endsWith(OPENAI_SUFFIX)) {
      return t('setup.urlFormatWarning.openai')
    }
  }
  return ''
})

// Auto-fix URL suffix when switching API format
watch(() => props.apiFormat, (newFmt, oldFmt) => {
  if (props.provider !== '_custom' || !props.customUrl.trim()) return
  const url = props.customUrl.trim()
  const oldSuffix = oldFmt === 'anthropic' ? ANTHROPIC_SUFFIX : OPENAI_SUFFIX
  const newSuffix = newFmt === 'anthropic' ? ANTHROPIC_SUFFIX : OPENAI_SUFFIX
  if (url.endsWith(oldSuffix)) {
    emit('update:customUrl', url.slice(0, -oldSuffix.length) + newSuffix)
  }
})

const canProceed = computed(() => {
  if (props.provider === '_custom') {
    return props.customUrl.trim() !== '' && props.apiKey.trim() !== ''
  }
  return props.apiKey.trim() !== ''
})

function handleNext() {
  if (!canProceed.value) return
  error.value = ''
  emit('next')
}

// Auto-focus API key input
onMounted(() => {
  nextTick(() => {
    if (props.provider !== '_custom' && apiKeyInputRef.value) {
      apiKeyInputRef.value.focus()
    }
  })
})
</script>

<style scoped>
.setup-apikey {
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

.input-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.input-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
}

.input-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.input-icon {
  position: absolute;
  left: 10px;
  width: 16px;
  height: 16px;
  color: var(--text-muted);
  pointer-events: none;
}

.setup-input {
  width: 100%;
  padding: 9px 38px 9px 32px;
  border: 1.5px solid var(--border-color);
  border-radius: var(--radius-sm, 6px);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  box-sizing: border-box;
  transition: border-color 0.2s;
}

.setup-input:focus {
  border-color: var(--accent-color);
}

.format-toggle {
  display: flex;
  gap: 0;
  border: 1.5px solid var(--border-color);
  border-radius: var(--radius-sm, 6px);
  overflow: hidden;
}

.format-btn {
  flex: 1;
  padding: 7px 0;
  border: none;
  background: var(--bg-primary);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.format-btn:first-child {
  border-right: 1px solid var(--border-color);
}

.format-btn--active {
  background: color-mix(in srgb, var(--accent-color) 12%, var(--bg-primary));
  color: var(--accent-color);
  font-weight: 600;
}

.input-toggle {
  position: absolute;
  right: 6px;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
}

.input-toggle:hover {
  color: var(--text-secondary);
}

.setup-input--warning {
  border-color: #e6a817;
}

.url-warning {
  font-size: 11px;
  color: #c08b00;
  line-height: 1.4;
  padding: 0 2px;
}
</style>

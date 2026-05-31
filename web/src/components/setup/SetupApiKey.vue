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
          :class="{ 'setup-input--error': urlError, 'setup-input--detected': detectedFormat }"
          placeholder="https://api.example.com/v1/chat/completions"
          autocomplete="off"
        />
      </div>
      <div v-if="detectedFormat" class="url-detected">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
          <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
          <polyline points="22 4 12 14.01 9 11.01"/>
        </svg>
        {{ detectedFormat === 'openai' ? 'OpenAI' : 'Anthropic' }} API
      </div>
      <div v-if="urlError" class="url-warning">{{ urlError }}</div>
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

// Auto-detect API format from URL path
const OPENAI_SUFFIX = '/chat/completions'
const ANTHROPIC_SUFFIX = '/v1/messages'

function detectFormat(url: string): '' | 'openai' | 'anthropic' {
  if (!url.trim()) return ''
  const trimmed = url.trim()
  if (trimmed.endsWith(OPENAI_SUFFIX)) return 'openai'
  if (trimmed.endsWith(ANTHROPIC_SUFFIX)) return 'anthropic'
  return ''
}

const detectedFormat = computed(() => {
  if (props.provider !== '_custom') return ''
  return detectFormat(props.customUrl)
})

const urlError = computed(() => {
  if (props.provider !== '_custom' || !props.customUrl.trim()) return ''
  const url = props.customUrl.trim()
  // Scheme validation
  if (!url.startsWith('http://') && !url.startsWith('https://')) {
    return t('setup.urlError.invalidScheme')
  }
  // Hostname validation
  try {
    const parsed = new URL(url)
    if (!parsed.hostname) {
      return t('setup.urlError.invalidHost')
    }
  } catch {
    return t('setup.urlError.invalidUrl')
  }
  // Auto-detect format from URL — error if neither suffix matches
  const fmt = detectFormat(url)
  if (!fmt) {
    return t('setup.urlError.unrecognizedFormat')
  }
  return ''
})

// Auto-sync apiFormat when URL changes (detected from path)
watch(detectedFormat, (fmt) => {
  if (fmt && fmt !== props.apiFormat) {
    emit('update:apiFormat', fmt)
  }
})

const canProceed = computed(() => {
  if (props.provider === '_custom') {
    return props.customUrl.trim() !== '' && props.apiKey.trim() !== '' && !urlError.value
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

.setup-input--error {
  border-color: var(--color-red, #dc2626);
}

.setup-input--detected {
  border-color: var(--color-green, #16a34a);
}

.url-detected {
  font-size: 11px;
  color: var(--color-green, #16a34a);
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 2px;
}

.url-warning {
  font-size: 11px;
  color: var(--color-red, #dc2626);
  line-height: 1.4;
  padding: 0 2px;
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
</style>

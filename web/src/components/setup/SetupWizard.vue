<template>
  <div class="setup-wizard">
    <!-- Decorative background (same as LoginView) -->
    <div class="setup-bg-gradient"></div>
    <div class="setup-bg-grid"></div>

    <div class="setup-content">
      <!-- Progress bar -->
      <div class="setup-progress">
        <div
          v-for="i in totalSteps"
          :key="i"
          class="progress-dot"
          :class="{
            'progress-dot--active': i === step,
            'progress-dot--done': i < step
          }"
        />
      </div>

      <!-- Step content -->
      <div class="setup-step-container">
        <Transition name="setup-slide" mode="out-in">
          <!-- Step 1: Welcome -->
          <SetupWelcome
            v-if="step === 1"
            key="welcome"
            :embedded-agent="embeddedAgent"
            :agent-version="agentVersion"
            @next="goToStep(2)"
          />

          <!-- Step 2: Provider -->
          <SetupProvider
            v-else-if="step === 2"
            key="provider"
            v-model="provider"
            :providers="providersList"
            @next="handleProviderNext"
            @back="goToStep(1)"
          />

          <!-- Step 3: API Key -->
          <SetupApiKey
            v-else-if="step === 3"
            key="apikey"
            :provider="provider"
            :provider-name="currentProviderName"
            :provider-env-var="currentProviderEnvVar"
            v-model:custom-url="customUrl"
            v-model:api-key="apiKey"
            @next="handleApiKeyNext"
            @back="goToStep(2)"
          />

          <!-- Step 4: Model + Verify -->
          <SetupModelVerify
            v-else-if="step === 4"
            key="model"
            :models="modelsList"
            :models-loading="modelsLoading"
            :models-error="modelsErrorMsg"
            :summarize-model-hint="summarizeModelHintVal"
            v-model:chat-model="chatModel"
            v-model:summarize-model="summarizeModel"
            :verifying="verifying"
            :verify-result="verifyResult"
            @verify="handleVerify"
            @next="goToStep(5)"
            @back="goToStep(3)"
          />

          <!-- Step 5: Agent Name -->
          <SetupAgentName
            v-else-if="step === 5"
            key="name"
            v-model:agent-name="agentName"
            v-model:agent-id="agentId"
            :completing="completing"
            @complete="handleComplete"
            @back="goToStep(4)"
          />
        </Transition>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSetup, providerAgentNames } from '@/composables/useSetup'
import { useToast } from '@/composables/useToast'
import { store } from '@/stores/app'
import SetupWelcome from './SetupWelcome.vue'
import SetupProvider from './SetupProvider.vue'
import SetupApiKey from './SetupApiKey.vue'
import SetupModelVerify from './SetupModelVerify.vue'
import SetupAgentName from './SetupAgentName.vue'

const emit = defineEmits<{
  complete: []
}>()

const { t } = useI18n()
const toast = useToast()
const {
  checkStatus,
  getProviders,
  scanModels,
  verify: verifyApi,
  complete: completeApi,
  saveWizardState,
  loadWizardState,
  clearWizardState,
} = useSetup()

// ── Wizard state ──

const totalSteps = 5
const step = ref(1)

const provider = ref('')
const customUrl = ref('')
const apiKey = ref('')  // NEVER persisted to sessionStorage
const chatModel = ref('')
const summarizeModel = ref('')
const agentName = ref('')
const agentId = ref('')

// ── Async state ──

const embeddedAgent = ref(false)
const agentVersion = ref('')
const providersList = ref<{ id: string; name: string; envVar: string }[]>([])
const modelsList = ref<{ id: string; name: string; context_length?: number; cost_tier?: string }[]>([])
const modelsLoading = ref(false)
const modelsErrorMsg = ref('')
const summarizeModelHintVal = ref('')
const verifying = ref(false)
const verifyResult = ref<{ success: boolean; message: string } | null>(null)
const completing = ref(false)

// ── Computed helpers ──

const currentProviderName = computed(() => {
  if (provider.value === '_custom') return t('setup.customUrl')
  return providersList.value.find(p => p.id === provider.value)?.name || provider.value
})

const currentProviderEnvVar = computed(() => {
  if (provider.value === '_custom') return ''
  return providersList.value.find(p => p.id === provider.value)?.envVar || ''
})

// ── Step navigation ──

function goToStep(n: number) {
  step.value = n
  persistState()
}

function persistState() {
  saveWizardState({
    step: step.value,
    provider: provider.value,
    customUrl: customUrl.value,
    chatModel: chatModel.value,
    summarizeModel: summarizeModel.value,
    agentName: agentName.value,
    agentId: agentId.value,
  })
}

// ── Step handlers ──

async function handleProviderNext() {
  // Auto-generate agent name/id from provider
  const names = providerAgentNames[provider.value]
  if (names) {
    if (!agentName.value || Object.values(providerAgentNames).some(n => n.name === agentName.value)) {
      agentName.value = names.name
    }
    if (!agentId.value || Object.values(providerAgentNames).some(n => n.id === agentId.value)) {
      agentId.value = names.id
    }
  }
  goToStep(3)
}

async function handleApiKeyNext() {
  goToStep(4)
  // Auto-fetch models when entering step 4
  await fetchModels()
}

async function fetchModels() {
  modelsLoading.value = true
  modelsErrorMsg.value = ''
  modelsList.value = []
  chatModel.value = ''
  summarizeModel.value = ''
  verifyResult.value = null

  const result = await scanModels(provider.value, customUrl.value, apiKey.value)
  modelsList.value = result.models || []
  summarizeModelHintVal.value = result.summarize_model_hint || ''
  if (result.error) {
    modelsErrorMsg.value = result.error
  }
  if (modelsList.value.length > 0) {
    // Default: first model for chat, hint for summarize
    chatModel.value = modelsList.value[0].id
    summarizeModel.value = summarizeModelHintVal.value || modelsList.value[0].id
  }
  persistState()
}

async function handleVerify() {
  if (!chatModel.value) return
  verifying.value = true
  verifyResult.value = null
  const result = await verifyApi(provider.value, customUrl.value, apiKey.value, chatModel.value)
  verifyResult.value = result
  verifying.value = false
}

async function handleComplete() {
  completing.value = true
  const result = await completeApi({
    provider: provider.value === '_custom' ? '' : provider.value,
    custom_url: provider.value === '_custom' ? customUrl.value : '',
    api_key: apiKey.value,
    model: chatModel.value,
    summarize_model: summarizeModel.value,
    agent_name: agentName.value,
    agent_id: agentId.value,
  })
  completing.value = false

  if (result.success) {
    clearWizardState()
    // Reload agents so the app picks up the new one
    try {
      await store.loadProject()
    } catch { /* best effort */ }
    emit('complete')
  } else {
    toast.show(t('setup.completeFailed'), { icon: '⚠️', type: 'error', duration: 4000 })
  }
}

// ── Restore state from sessionStorage on mount ──

onMounted(async () => {
  // Check setup status
  try {
    const status = await checkStatus()
    embeddedAgent.value = status.embedded_agent
    agentVersion.value = status.agent_version || ''
  } catch {
    embeddedAgent.value = false
  }

  // Load providers
  try {
    providersList.value = await getProviders()
  } catch { /* will show empty list */ }

  // Restore wizard state (API key is NOT restored for security)
  const saved = loadWizardState()
  if (saved) {
    step.value = saved.step || 1
    provider.value = saved.provider || ''
    customUrl.value = saved.customUrl || ''
    chatModel.value = saved.chatModel || ''
    summarizeModel.value = saved.summarizeModel || ''
    agentName.value = saved.agentName || ''
    agentId.value = saved.agentId || ''
  }
})

// ── Auto-derive agentId from agentName ──

watch(agentName, (name) => {
  // Only auto-derive if the user hasn't manually changed agentId
  const expected = providerAgentNames[provider.value]?.id || ''
  if (!agentId.value || agentId.value === expected || agentId.value === name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')) {
    agentId.value = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
  }
})
</script>

<style scoped>
.setup-wizard {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-primary);
  position: relative;
  overflow: hidden;
}

/* Decorative background (mirrors LoginView) */
.setup-bg-gradient {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse 60% 50% at 20% 20%, color-mix(in srgb, var(--accent-color) 8%, transparent), transparent),
    radial-gradient(ellipse 50% 60% at 80% 80%, color-mix(in srgb, var(--accent-color) 6%, transparent), transparent);
  pointer-events: none;
}

.setup-bg-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(color-mix(in srgb, var(--border-color) 30%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--border-color) 30%, transparent) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(ellipse 70% 70% at center, black, transparent);
  -webkit-mask-image: radial-gradient(ellipse 70% 70% at center, black, transparent);
  opacity: 0.4;
  pointer-events: none;
}

.setup-content {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 420px;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Progress dots */
.setup-progress {
  display: flex;
  justify-content: center;
  gap: 8px;
  padding-bottom: 4px;
}

.progress-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--bg-tertiary);
  transition: background 0.3s, transform 0.3s;
}

.progress-dot--active {
  background: var(--accent-color);
  transform: scale(1.2);
}

.progress-dot--done {
  background: color-mix(in srgb, var(--accent-color) 50%, var(--bg-tertiary));
}

/* Step container */
.setup-step-container {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 14px;
  padding: 24px;
  box-shadow: var(--shadow-sm);
  min-height: 280px;
  display: flex;
  flex-direction: column;
}

/* Slide transition */
.setup-slide-enter-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.setup-slide-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.setup-slide-enter-from {
  opacity: 0;
  transform: translateX(20px);
}
.setup-slide-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}

/* Shared button styles (used by child step components via global) */
</style>

<style>
/* Global setup wizard button styles — shared by all step components */
.setup-btn-primary {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 11px 20px;
  border: none;
  border-radius: var(--radius-md, 10px);
  background: var(--accent-color);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s, transform 0.1s, opacity 0.2s;
}

.setup-btn-primary:hover:not(:disabled) {
  background: var(--accent-hover);
}

.setup-btn-primary:active:not(:disabled) {
  transform: scale(0.98);
}

.setup-btn-primary:disabled {
  opacity: 0.5;
  cursor: default;
}

.setup-btn-secondary {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 11px 20px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md, 10px);
  background: var(--bg-primary);
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s, border-color 0.2s;
}

.setup-btn-secondary:hover:not(:disabled) {
  background: var(--bg-tertiary);
  border-color: var(--text-muted);
}

.setup-btn-secondary:disabled {
  opacity: 0.5;
  cursor: default;
}

.step-nav {
  display: flex;
  gap: 10px;
  margin-top: auto;
  padding-top: 16px;
}

.step-nav .setup-btn-secondary {
  flex: 0 0 auto;
}

.step-nav .setup-btn-primary {
  flex: 1;
}

.setup-error {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: var(--radius-md, 10px);
  background: color-mix(in srgb, var(--color-red, #dc2626) 8%, var(--bg-primary));
  border: 1px solid color-mix(in srgb, var(--color-red, #dc2626) 20%, var(--border-color));
  color: var(--color-red, #dc2626);
  font-size: 13px;
}
</style>

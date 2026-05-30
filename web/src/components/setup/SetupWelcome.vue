<template>
  <div class="setup-step setup-welcome">
    <div class="welcome-icon">
      <span class="welcome-emoji">🥧</span>
      <div class="welcome-ring"></div>
    </div>
    <h2 class="welcome-title">{{ t('setup.welcomeTitle') }}</h2>
    <p class="welcome-desc">{{ t('setup.welcomeDesc') }}</p>
    <p v-if="agentVersion" class="welcome-version">Pi v{{ agentVersion }}</p>
    <button class="setup-btn-primary" @click="$emit('next')" :disabled="!embeddedAgent">
      <span>{{ t('setup.configureAgent') }}</span>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18">
        <path d="M5 12h14M12 5l7 7-7 7"/>
      </svg>
    </button>
    <p v-if="!embeddedAgent" class="welcome-hint">{{ t('setup.noEmbeddedAgent') }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  embeddedAgent: boolean
  agentVersion: string
}>()

defineEmits<{
  next: []
}>()

const { t } = useI18n()
</script>

<style scoped>
.setup-welcome {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 16px;
  padding: 24px 0;
}

.welcome-icon {
  position: relative;
  width: 96px;
  height: 96px;
  margin-bottom: 8px;
}

.welcome-emoji {
  font-size: 64px;
  line-height: 96px;
  display: block;
  position: relative;
  z-index: 1;
}

.welcome-ring {
  position: absolute;
  inset: -4px;
  border-radius: 50%;
  border: 2px solid color-mix(in srgb, var(--accent-color) 30%, transparent);
  animation: ring-pulse 3s ease-in-out infinite;
}

@keyframes ring-pulse {
  0%, 100% { opacity: 0.4; transform: scale(1); }
  50% { opacity: 0.8; transform: scale(1.04); }
}

.welcome-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.welcome-desc {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0;
  max-width: 320px;
  line-height: 1.6;
}

.welcome-version {
  font-size: 12px;
  color: var(--text-muted);
  margin: 0;
  padding: 4px 12px;
  background: var(--bg-tertiary);
  border-radius: 20px;
}

.welcome-hint {
  font-size: 13px;
  color: var(--color-red, #dc2626);
  margin: 8px 0 0;
}
</style>

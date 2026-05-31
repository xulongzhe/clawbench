<template>
  <div class="setup-step setup-welcome">
    <div class="welcome-icon">
      <span class="welcome-emoji">🥧</span>
    </div>
    <h2 class="welcome-title">{{ t('setup.welcomeTitle') }}</h2>
    <p class="welcome-desc">{{ t('setup.welcomeDesc') }}</p>
    <p v-if="agentVersion" class="welcome-version">Pi v{{ agentVersion }}</p>
    <button class="setup-btn-primary" @click="$emit('next')" :disabled="!embeddedAgent">
      <span>{{ t('setup.configureAgent') }}</span>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
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
  gap: 10px;
  padding: 12px 0;
}

.welcome-icon {
  position: relative;
  width: 64px;
  height: 64px;
}

.welcome-emoji {
  font-size: 48px;
  line-height: 64px;
  display: block;
}

.welcome-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.welcome-desc {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0;
  max-width: 300px;
  line-height: 1.5;
}

.welcome-version {
  font-size: 11px;
  color: var(--text-muted);
  margin: 0;
  padding: 2px 10px;
  background: var(--bg-tertiary);
  border-radius: 12px;
}

.welcome-hint {
  font-size: 12px;
  color: var(--color-red, #dc2626);
  margin: 4px 0 0;
}
</style>

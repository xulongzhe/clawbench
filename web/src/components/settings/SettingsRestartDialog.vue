<template>
  <div class="settings-restart-overlay" @click.self="$emit('later')">
    <div class="settings-restart-dialog">
      <div class="settings-restart-dialog__header">{{ t('settings.restartConfirmTitle') }}</div>
      <p class="settings-restart-dialog__message">{{ t('settings.restartConfirmMessage') }}</p>
      <ul v-if="changedFields.length > 0" class="settings-restart-dialog__list">
        <li v-for="field in changedFields" :key="field">{{ field }}</li>
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
import { useI18n } from 'vue-i18n'

defineProps<{
  changedFields: string[]
}>()

defineEmits<{
  restart: []
  later: []
}>()

const { t } = useI18n()
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

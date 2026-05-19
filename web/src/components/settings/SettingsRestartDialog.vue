<template>
  <div class="settings-restart-overlay" @click.self="$emit('later')">
    <div class="settings-restart-dialog">
      <div class="settings-restart-dialog__header">以下配置需重启后生效</div>
      <ul v-if="changedFields.length > 0" class="settings-restart-dialog__list">
        <li v-for="field in changedFields" :key="field">{{ field }}</li>
      </ul>
      <div class="settings-restart-dialog__actions">
        <button class="settings-restart-dialog__btn settings-restart-dialog__btn--later" @click="$emit('later')">
          稍后
        </button>
        <button class="settings-restart-dialog__btn settings-restart-dialog__btn--restart" @click="$emit('restart')">
          立即重启
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  changedFields: string[]
}>()

defineEmits<{
  restart: []
  later: []
}>()
</script>

<style scoped>
.settings-restart-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
}

.settings-restart-dialog {
  background: var(--bg-primary, #fff);
  border-radius: 12px;
  padding: 20px;
  margin: 24px;
  max-width: 320px;
  width: 100%;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
}

.settings-restart-dialog__header {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary, #1a1a1a);
  margin-bottom: 12px;
}

.settings-restart-dialog__list {
  margin: 0 0 16px;
  padding-left: 20px;
  font-size: 14px;
  color: var(--text-secondary, #666);
}

.settings-restart-dialog__list li {
  margin-bottom: 4px;
}

.settings-restart-dialog__actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

.settings-restart-dialog__btn {
  padding: 8px 16px;
  border: none;
  border-radius: 8px;
  font-size: 14px;
  cursor: pointer;
}

.settings-restart-dialog__btn--later {
  background: var(--bg-secondary, #f0f0f0);
  color: var(--text-primary, #1a1a1a);
}

.settings-restart-dialog__btn--restart {
  background: #4caf50;
  color: #fff;
}

.settings-restart-dialog__btn--restart:active {
  background: #43a047;
}

[data-theme="dark"] .settings-restart-dialog {
  background: var(--bg-primary, #2a2a2a);
}

[data-theme="dark"] .settings-restart-dialog__header {
  color: var(--text-primary, #e0e0e0);
}

[data-theme="dark"] .settings-restart-dialog__list {
  color: var(--text-secondary, #aaa);
}

[data-theme="dark"] .settings-restart-dialog__btn--later {
  background: var(--bg-secondary, #333);
  color: var(--text-primary, #e0e0e0);
}
</style>

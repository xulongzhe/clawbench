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
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
  -webkit-backdrop-filter: blur(4px);
  backdrop-filter: blur(4px);
}

.settings-restart-dialog {
  background: #fff;
  border-radius: 14px;
  padding: 20px;
  margin: 24px;
  max-width: 320px;
  width: 100%;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
}

.settings-restart-dialog__header {
  font-size: 17px;
  font-weight: 600;
  color: var(--text-primary, #1a1a1a);
  margin-bottom: 12px;
  text-align: center;
}

.settings-restart-dialog__list {
  margin: 0 0 20px;
  padding-left: 20px;
  font-size: 14px;
  color: var(--text-secondary, #8e8e93);
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
  background: #e9e9ea;
  color: var(--text-primary, #1a1a1a);
}

@media (hover: hover) {
  .settings-restart-dialog__btn--later:hover {
    background: #ddd;
  }
}

.settings-restart-dialog__btn--later:active {
  background: #d1d1d2;
}

.settings-restart-dialog__btn--restart {
  background: #007aff;
  color: #fff;
}

@media (hover: hover) {
  .settings-restart-dialog__btn--restart:hover {
    background: #0066d6;
  }
}

.settings-restart-dialog__btn--restart:active {
  background: #005ec2;
}

/* Dark mode */
[data-theme="dark"] .settings-restart-dialog {
  background: #2c2c2e;
}

[data-theme="dark"] .settings-restart-dialog__header {
  color: var(--text-primary, #e0e0e0);
}

[data-theme="dark"] .settings-restart-dialog__list {
  color: var(--text-secondary, #8e8e93);
}

[data-theme="dark"] .settings-restart-dialog__btn--later {
  background: #3a3a3c;
  color: var(--text-primary, #e0e0e0);
}

@media (hover: hover) {
  [data-theme="dark"] .settings-restart-dialog__btn--later:hover {
    background: #48484a;
  }
}

[data-theme="dark"] .settings-restart-dialog__btn--later:active {
  background: #4a4a4c;
}

[data-theme="dark"] .settings-restart-dialog__btn--restart {
  background: #0a84ff;
}

@media (hover: hover) {
  [data-theme="dark"] .settings-restart-dialog__btn--restart:hover {
    background: #0070e0;
  }
}

[data-theme="dark"] .settings-restart-dialog__btn--restart:active {
  background: #0062c4;
}
</style>

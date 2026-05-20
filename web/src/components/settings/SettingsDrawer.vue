<template>
  <BottomSheet ref="bottomSheetRef" :open="open" @close="handleClose">
    <template #header>
      <button v-if="navStack.length > 0" class="settings-back-btn" @click.stop="popNav">
        <ChevronLeft :size="18" />
      </button>
      <Settings :size="16" class="bs-header-icon" />
      <span class="bs-header-title">{{ headerTitle }}</span>
      <button class="settings-close-btn" @click.stop="handleClose">
        <X :size="16" />
      </button>
    </template>

    <div class="settings-body">
      <SettingsIndex v-if="navStack.length === 0" @navigate="pushNav" />
      <SettingsCategory
        v-else
        :category-id="currentCategory!"
        @restart-needed="handleRestartNeeded"
      />
      <SettingsRestartDialog
        v-if="restartDialogVisible"
        :changed-fields="changedColdFields"
        @restart="handleRestart"
        @later="restartDialogVisible = false"
      />
    </div>

    <template #footer>
      <button class="settings-restart-btn" :class="{ 'settings-restart-btn--pending': needsRestart }" :disabled="restarting" @click="handleRestart">
        <RefreshCw :size="14" class="settings-restart-btn__icon" :class="{ 'settings-restart-btn__icon--spin': restarting }" />
        <span>{{ restarting ? t('settings.restarting') : (needsRestart ? t('settings.restartPending') : t('settings.restartServer')) }}</span>
      </button>
    </template>
  </BottomSheet>
  <!-- Restart loading overlay (same as SettingsPage) -->
  <Teleport to="body">
    <div v-if="restartingOverlay" class="restart-overlay">
      <div class="restart-overlay__content">
        <div class="restart-overlay__spinner"></div>
        <div class="restart-overlay__text">{{ t('settings.restartingPleaseWait') }}</div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ChevronLeft, Settings, X, RefreshCw } from 'lucide-vue-next'
import BottomSheet from '@/components/common/BottomSheet.vue'
import SettingsIndex from './SettingsIndex.vue'
import SettingsCategory from './SettingsCategory.vue'
import SettingsRestartDialog from './SettingsRestartDialog.vue'
import { useSettingsNavigation } from '@/composables/useSettingsNavigation'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const {
  t, loadConfig,
  navStack, currentCategory, pushNav, popNav, resetState,
  restartDialogVisible, changedColdFields, needsRestart,
  restarting, restartingOverlay,
  handleRestartNeeded, handleRestart,
} = useSettingsNavigation()

const bottomSheetRef = ref<InstanceType<typeof BottomSheet> | null>(null)

const headerTitle = computed(() => {
  if (navStack.value.length === 0) return t('nav.settings')
  return currentCategory.value ? t(`settings.categories.${currentCategory.value}`) : ''
})

function handleClose() {
  emit('close')
}

// Load config and reset state when drawer opens
watch(() => props.open, (val) => {
  if (val) {
    loadConfig()
    resetState()
  }
})
</script>

<style scoped>
.settings-body {
  position: relative;
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.settings-back-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: none;
  color: var(--text-primary);
  cursor: pointer;
  border-radius: 6px;
  padding: 0;
  flex-shrink: 0;
}

@media (hover: hover) {
  .settings-back-btn:hover {
    background: var(--bg-tertiary);
  }
}

.settings-back-btn:active {
  background: var(--bg-tertiary);
}

.settings-close-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: none;
  color: var(--text-secondary);
  cursor: pointer;
  border-radius: 6px;
  padding: 0;
  flex-shrink: 0;
  margin-left: auto;
}

@media (hover: hover) {
  .settings-close-btn:hover {
    background: var(--bg-tertiary);
  }
}

.settings-close-btn:active {
  background: var(--bg-tertiary);
}

/* Restart footer button */
.settings-restart-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  padding: 10px 16px;
  border: none;
  border-radius: 10px;
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  text-align: center;
  transition: background 0.2s, color 0.2s, box-shadow 0.2s;
}

.settings-restart-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.settings-restart-btn--pending {
  background: var(--accent-color);
  color: #fff;
  animation: restart-pulse 2s ease-in-out infinite;
}

@keyframes restart-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(74, 144, 217, 0.4); }
  50% { box-shadow: 0 0 12px 4px rgba(74, 144, 217, 0.2); }
}

[data-theme="dark"] .settings-restart-btn--pending {
  animation: restart-pulse-dark 2s ease-in-out infinite;
}

@keyframes restart-pulse-dark {
  0%, 100% { box-shadow: 0 0 0 0 rgba(88, 166, 255, 0.4); }
  50% { box-shadow: 0 0 12px 4px rgba(88, 166, 255, 0.2); }
}

@media (hover: hover) {
  .settings-restart-btn:hover:not(:disabled):not(.settings-restart-btn--pending) {
    background: var(--bg-secondary);
  }
  .settings-restart-btn.settings-restart-btn--pending:hover:not(:disabled) {
    background: var(--accent-hover);
  }
}

.settings-restart-btn:active:not(.settings-restart-btn--pending) {
  background: var(--bg-secondary);
}

.settings-restart-btn:active.settings-restart-btn--pending:not(:disabled) {
  background: var(--accent-hover);
}

.settings-restart-btn__icon--spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Restart loading overlay (same as SettingsPage) */
.restart-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
}

.restart-overlay__content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  padding: 40px 48px;
  border-radius: 16px;
  background: var(--bg-primary);
  box-shadow: var(--shadow-md);
}

.restart-overlay__spinner {
  width: 36px;
  height: 36px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.restart-overlay__text {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
}
</style>

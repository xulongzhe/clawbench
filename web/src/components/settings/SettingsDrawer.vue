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
        <span>{{ restarting ? '重启中…' : (needsRestart ? '重启生效' : '重启服务器') }}</span>
      </button>
    </template>
  </BottomSheet>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ChevronLeft, Settings, X, RefreshCw } from 'lucide-vue-next'
import BottomSheet from '@/components/common/BottomSheet.vue'
import SettingsIndex from './SettingsIndex.vue'
import SettingsCategory from './SettingsCategory.vue'
import SettingsRestartDialog from './SettingsRestartDialog.vue'
import { useSettingsConfig } from '@/composables/useSettingsConfig'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const { loadConfig, restartServer } = useSettingsConfig()

const bottomSheetRef = ref<InstanceType<typeof BottomSheet> | null>(null)
const navStack = ref<string[]>([])
const restartDialogVisible = ref(false)
const changedColdFields = ref<string[]>([])
const needsRestart = ref(false)
const restarting = ref(false)

const currentCategory = computed(() => {
  return navStack.value.length > 0 ? navStack.value[navStack.value.length - 1] : null
})

const categoryLabels: Record<string, string> = {
  appearance: '外观',
  chat: '聊天',
  agents: 'Agent偏好',
  files: '文件',
  terminal: '终端',
  tts: 'TTS语音',
  rag: 'RAG记忆',
  network: '网络',
  about: '关于',
}

const headerTitle = computed(() => {
  if (navStack.value.length === 0) return '设置'
  return categoryLabels[currentCategory.value!] ?? currentCategory.value!
})

function pushNav(categoryId: string) {
  navStack.value.push(categoryId)
}

function popNav() {
  if (navStack.value.length > 0) {
    navStack.value.pop()
  }
}

function handleClose() {
  emit('close')
}

function handleRestartNeeded(fields: string[]) {
  changedColdFields.value = fields
  needsRestart.value = true
  // Also show the dialog with details
  restartDialogVisible.value = true
}

async function handleRestart() {
  restartDialogVisible.value = false
  restarting.value = true
  try {
    await restartServer()
    needsRestart.value = false
  } catch {
    // Ignore
  } finally {
    restarting.value = false
  }
}

// Load config and reset state when drawer opens
watch(() => props.open, (val) => {
  if (val) {
    loadConfig()
    navStack.value = []
    needsRestart.value = false
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
  color: var(--text-primary, #1a1a1a);
  cursor: pointer;
  border-radius: 6px;
  padding: 0;
  flex-shrink: 0;
}

@media (hover: hover) {
  .settings-back-btn:hover {
    background: rgba(0, 0, 0, 0.04);
  }
}

.settings-back-btn:active {
  background: rgba(0, 0, 0, 0.08);
}

.settings-close-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: none;
  color: var(--text-secondary, #8e8e93);
  cursor: pointer;
  border-radius: 6px;
  padding: 0;
  flex-shrink: 0;
  margin-left: auto;
}

@media (hover: hover) {
  .settings-close-btn:hover {
    background: rgba(0, 0, 0, 0.04);
  }
}

.settings-close-btn:active {
  background: rgba(0, 0, 0, 0.08);
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
  background: var(--bg-tertiary, #e9e9ea);
  color: var(--text-secondary, #8e8e93);
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
  background: #007aff;
  color: #fff;
  animation: restart-pulse 2s ease-in-out infinite;
}

@keyframes restart-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(0, 122, 255, 0.4); }
  50% { box-shadow: 0 0 12px 4px rgba(0, 122, 255, 0.2); }
}

@media (hover: hover) {
  .settings-restart-btn:hover:not(:disabled):not(.settings-restart-btn--pending) {
    background: var(--bg-secondary, #f0f0f0);
  }
  .settings-restart-btn.settings-restart-btn--pending:hover:not(:disabled) {
    background: #0066d6;
  }
}

.settings-restart-btn:active:not(.settings-restart-btn--pending) {
  background: var(--bg-secondary, #e0e0e0);
}

.settings-restart-btn:active.settings-restart-btn--pending:not(:disabled) {
  background: #005ec2;
}

.settings-restart-btn__icon--spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Dark mode */
[data-theme="dark"] .settings-back-btn {
  color: var(--text-primary, #e0e0e0);
}

@media (hover: hover) {
  [data-theme="dark"] .settings-back-btn:hover {
    background: rgba(255, 255, 255, 0.06);
  }
}

[data-theme="dark"] .settings-back-btn:active {
  background: rgba(255, 255, 255, 0.1);
}

[data-theme="dark"] .settings-close-btn {
  color: var(--text-secondary, #8e8e93);
}

@media (hover: hover) {
  [data-theme="dark"] .settings-close-btn:hover {
    background: rgba(255, 255, 255, 0.06);
  }
}

[data-theme="dark"] .settings-close-btn:active {
  background: rgba(255, 255, 255, 0.1);
}

[data-theme="dark"] .settings-restart-btn {
  background: #2c2c2e;
  color: var(--text-secondary, #8e8e93);
}

[data-theme="dark"] .settings-restart-btn--pending {
  background: #0a84ff;
  color: #fff;
}

@keyframes restart-pulse-dark {
  0%, 100% { box-shadow: 0 0 0 0 rgba(10, 132, 255, 0.4); }
  50% { box-shadow: 0 0 12px 4px rgba(10, 132, 255, 0.2); }
}

[data-theme="dark"] .settings-restart-btn--pending {
  animation: restart-pulse-dark 2s ease-in-out infinite;
}

@media (hover: hover) {
  [data-theme="dark"] .settings-restart-btn:hover:not(:disabled):not(.settings-restart-btn--pending) {
    background: #3a3a3c;
  }
  [data-theme="dark"] .settings-restart-btn.settings-restart-btn--pending:hover:not(:disabled) {
    background: #0070e0;
  }
}

[data-theme="dark"] .settings-restart-btn:active.settings-restart-btn--pending:not(:disabled) {
  background: #0062c4;
}
</style>

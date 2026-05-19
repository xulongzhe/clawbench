<template>
  <Transition name="settings-drawer">
    <div v-if="show" class="settings-drawer" @click.self="$emit('close')">
      <div class="settings-drawer__panel">
        <div class="settings-drawer__header">
          <button class="settings-drawer__back" @click="handleBack">
            <ChevronLeft :size="20" />
          </button>
          <span class="settings-drawer__title">{{ headerTitle }}</span>
        </div>
        <div class="settings-drawer__body">
          <SettingsIndex v-if="navStack.length === 0" @navigate="pushNav" />
          <SettingsCategory
            v-else
            :category-id="currentCategory!"
            @restart-needed="handleRestartNeeded"
          />
        </div>
        <SettingsRestartDialog
          v-if="restartDialogVisible"
          :changed-fields="changedColdFields"
          @restart="handleRestart"
          @later="restartDialogVisible = false"
        />
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ChevronLeft } from 'lucide-vue-next'
import SettingsIndex from './SettingsIndex.vue'
import SettingsCategory from './SettingsCategory.vue'
import SettingsRestartDialog from './SettingsRestartDialog.vue'
import { useSettingsConfig } from '@/composables/useSettingsConfig'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const { loadConfig, restartServer } = useSettingsConfig()

const navStack = ref<string[]>([])
const restartDialogVisible = ref(false)
const changedColdFields = ref<string[]>([])

const currentCategory = computed(() => {
  return navStack.value.length > 0 ? navStack.value[navStack.value.length - 1] : null
})

const categoryLabels: Record<string, string> = {
  appearance: '外观',
  chat: '聊天',
  agents: 'Agent偏好',
  fileManager: '文件管理',
  fileViewer: '文件查看器',
  terminal: '终端',
  tts: 'TTS语音',
  rag: 'RAG记忆',
  proxy: '端口转发',
  ssh: 'SSH隧道',
  push: '推送',
  android: 'Android',
  server: '服务器',
  about: '关于',
}

const headerTitle = computed(() => {
  if (navStack.value.length === 0) return '设置'
  return categoryLabels[currentCategory.value!] ?? currentCategory.value!
})

function pushNav(categoryId: string) {
  navStack.value.push(categoryId)
}

function handleBack() {
  if (navStack.value.length > 0) {
    navStack.value.pop()
  } else {
    emit('close')
  }
}

function handleRestartNeeded(fields: string[]) {
  changedColdFields.value = fields
  restartDialogVisible.value = true
}

async function handleRestart() {
  restartDialogVisible.value = false
  try {
    await restartServer()
  } catch {
    // Ignore
  }
}

// Load config when drawer opens
watch(() => props.show, (val) => {
  if (val) {
    loadConfig()
    navStack.value = []
  }
})
</script>

<style scoped>
.settings-drawer {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  justify-content: flex-end;
}

.settings-drawer__panel {
  width: 100vw;
  max-width: 420px;
  height: 100%;
  background: var(--bg-primary, #fff);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.settings-drawer__header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 8px;
  border-bottom: 1px solid var(--border-color, #e0e0e0);
  flex-shrink: 0;
}

.settings-drawer__back {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: none;
  background: none;
  color: var(--text-primary, #1a1a1a);
  cursor: pointer;
  border-radius: 8px;
  padding: 0;
}

.settings-drawer__back:active {
  background: var(--active-bg, rgba(0, 0, 0, 0.05));
}

.settings-drawer__title {
  font-size: 17px;
  font-weight: 600;
  color: var(--text-primary, #1a1a1a);
}

.settings-drawer__body {
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

/* Slide animation */
.settings-drawer-enter-active,
.settings-drawer-leave-active {
  transition: opacity 0.25s ease;
}

.settings-drawer-enter-active .settings-drawer__panel,
.settings-drawer-leave-active .settings-drawer__panel {
  transition: transform 0.25s ease;
}

.settings-drawer-enter-from {
  opacity: 0;
}

.settings-drawer-enter-from .settings-drawer__panel {
  transform: translateX(100%);
}

.settings-drawer-leave-to {
  opacity: 0;
}

.settings-drawer-leave-to .settings-drawer__panel {
  transform: translateX(100%);
}

[data-theme="dark"] .settings-drawer__panel {
  background: var(--bg-primary, #1e1e1e);
}

[data-theme="dark"] .settings-drawer__header {
  border-bottom-color: var(--border-color, #333);
}

[data-theme="dark"] .settings-drawer__back {
  color: var(--text-primary, #e0e0e0);
}

[data-theme="dark"] .settings-drawer__title {
  color: var(--text-primary, #e0e0e0);
}
</style>

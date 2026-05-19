<template>
  <div class="settings-index">
    <div
      v-for="cat in categories"
      :key="cat.id"
      class="settings-index__row"
      @click="$emit('navigate', cat.id)"
    >
      <div class="settings-index__left">
        <component :is="cat.icon" class="settings-index__icon" :size="18" />
        <span class="settings-index__label">{{ cat.label }}</span>
      </div>
      <ChevronRight class="settings-index__arrow" :size="18" />
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  Palette,
  MessageSquare,
  Bot,
  FolderOpen,
  FileText,
  Terminal,
  Volume2,
  Brain,
  Network,
  Shield,
  Bell,
  Smartphone,
  Server,
  Info,
  ChevronRight,
} from 'lucide-vue-next'

defineEmits<{
  navigate: [categoryId: string]
}>()

const categories = [
  { id: 'appearance', label: '外观', icon: Palette },
  { id: 'chat', label: '聊天', icon: MessageSquare },
  { id: 'agents', label: 'Agent偏好', icon: Bot },
  { id: 'fileManager', label: '文件管理', icon: FolderOpen },
  { id: 'fileViewer', label: '文件查看器', icon: FileText },
  { id: 'terminal', label: '终端', icon: Terminal },
  { id: 'tts', label: 'TTS语音', icon: Volume2 },
  { id: 'rag', label: 'RAG记忆', icon: Brain },
  { id: 'proxy', label: '端口转发', icon: Network },
  { id: 'ssh', label: 'SSH隧道', icon: Shield },
  { id: 'push', label: '推送', icon: Bell },
  { id: 'android', label: 'Android', icon: Smartphone },
  { id: 'server', label: '服务器', icon: Server },
  { id: 'about', label: '关于', icon: Info },
]
</script>

<style scoped>
.settings-index {
  padding: 0 0 8px;
}

.settings-index__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 13px 16px;
  cursor: pointer;
  gap: 12px;
}

.settings-index__row:active {
  background: var(--active-bg, rgba(0, 0, 0, 0.05));
}

.settings-index__left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.settings-index__icon {
  flex-shrink: 0;
  color: var(--text-secondary, #666);
}

.settings-index__label {
  font-size: 15px;
  color: var(--text-primary, #1a1a1a);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.settings-index__arrow {
  flex-shrink: 0;
  color: var(--text-tertiary, #999);
}

[data-theme="dark"] .settings-index__icon {
  color: var(--text-secondary, #aaa);
}

[data-theme="dark"] .settings-index__label {
  color: var(--text-primary, #e0e0e0);
}

[data-theme="dark"] .settings-index__arrow {
  color: var(--text-tertiary, #777);
}

[data-theme="dark"] .settings-index__row:active {
  background: var(--active-bg, rgba(255, 255, 255, 0.08));
}
</style>

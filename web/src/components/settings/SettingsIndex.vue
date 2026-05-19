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
  padding: 8px 0;
  background: var(--settings-bg, #f2f2f7);
  min-height: 100%;
}

.settings-index__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 0 16px;
  cursor: pointer;
  gap: 12px;
  background: #fff;
  position: relative;
}

/* Grouped card: first row rounded top, last row rounded bottom */
.settings-index__row:first-child {
  border-radius: 12px 12px 0 0;
}

.settings-index__row:last-child {
  border-radius: 0 0 12px 12px;
}

.settings-index__row:only-child {
  border-radius: 12px;
}

/* Row separator (not on last) */
.settings-index__row:not(:last-child)::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 48px;
  right: 0;
  height: 0.5px;
  background: var(--border-color, #c6c6c6);
}

@media (hover: hover) {
  .settings-index__row:hover {
    background: #f7f7f7;
  }
}

.settings-index__row:active {
  background: #ececec;
}

.settings-index__left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.settings-index__icon {
  flex-shrink: 0;
  color: var(--text-secondary, #8e8e93);
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
  color: var(--text-tertiary, #c7c7cc);
}

/* Dark mode */
[data-theme="dark"] .settings-index {
  background: var(--settings-bg, #000);
}

[data-theme="dark"] .settings-index__row {
  background: #1c1c1e;
}

[data-theme="dark"] .settings-index__row:not(:last-child)::after {
  background: var(--border-color, #38383a);
}

@media (hover: hover) {
  [data-theme="dark"] .settings-index__row:hover {
    background: #2c2c2e;
  }
}

[data-theme="dark"] .settings-index__row:active {
  background: #3a3a3c;
}

[data-theme="dark"] .settings-index__icon {
  color: var(--text-secondary, #8e8e93);
}

[data-theme="dark"] .settings-index__label {
  color: var(--text-primary, #e0e0e0);
}

[data-theme="dark"] .settings-index__arrow {
  color: var(--text-tertiary, #48484a);
}
</style>

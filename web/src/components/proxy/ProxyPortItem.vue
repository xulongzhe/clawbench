<template>
  <div class="proxy-port-item" :class="{ inactive: !active && !tunnelDisconnected }">
    <div class="port-info">
      <div class="port-main">
        <span class="port-number">{{ port }}</span>
        <span v-if="host" class="port-host">{{ host }}</span>
        <span class="port-protocol" :class="protocol">{{ protocol }}</span>
        <span
          class="port-status"
          :class="statusClass"
          :title="statusTitle"
        ></span>
        <span v-if="name" class="port-name">{{ name }}</span>
      </div>
    </div>
    <div class="port-actions">
      <button class="port-action-btn sandbox" @click.stop="$emit('open', port, protocol, host)" :title="t('proxy.openInSandbox')">
        <Box :size="14" />
      </button>
      <button class="port-action-btn open" @click.stop="$emit('openExternal', port, protocol, host)" :title="t('proxy.openInBrowser')">
        <ExternalLink :size="14" />
      </button>
      <button class="port-action-btn delete" @click.stop="$emit('remove', port, host)" :title="t('common.delete')">
        <Trash2 :size="14" />
      </button>
    </div>
  </div>
</template>

<script setup>
import { Box, ExternalLink, Trash2 } from 'lucide-vue-next'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps({
  port: { type: Number, required: true },
  host: { type: String, default: '' },
  name: { type: String, default: '' },
  protocol: { type: String, default: 'http' },
  active: { type: Boolean, default: false },
  tunnelDisconnected: { type: Boolean, default: false },
})

defineEmits(['open', 'openExternal', 'remove'])

const statusClass = computed(() => {
  if (props.active) return 'active'
  if (props.tunnelDisconnected) return 'tunnel-down'
  return 'inactive'
})

const statusTitle = computed(() => {
  if (props.active) return t('proxy.portItem.active')
  if (props.tunnelDisconnected) return t('proxy.portItem.tunnelDown')
  return t('proxy.portItem.inactive')
})
</script>

<style scoped>
.proxy-port-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid var(--border-color, #e5e5e5);
}

.proxy-port-item.inactive {
  opacity: 0.6;
}

.port-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.port-main {
  display: flex;
  align-items: center;
  gap: 6px;
}

.port-number {
  font-size: 16px;
  font-weight: 600;
  font-family: monospace;
  color: var(--text-primary, #1a1a1a);
}

.port-host {
  font-size: 10px;
  font-family: monospace;
  font-weight: 500;
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(107, 114, 128, 0.1);
  color: var(--text-secondary, #666);
}

.port-protocol {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 4px;
  border-radius: 3px;
  text-transform: uppercase;
  line-height: 1;
}

.port-protocol.http {
  background: rgba(34, 197, 94, 0.12);
  color: #16a34a;
}

.port-protocol.https {
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}

.port-status {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.port-status.active {
  background: #22c55e;
  box-shadow: 0 0 4px rgba(34, 197, 94, 0.4);
}

.port-status.inactive {
  background: #9ca3af;
}

.port-status.tunnel-down {
  background: #ef4444;
  box-shadow: 0 0 4px rgba(239, 68, 68, 0.4);
  animation: pulse-red 2s ease-in-out infinite;
}

@keyframes pulse-red {
  0%, 100% {
    box-shadow: 0 0 4px rgba(239, 68, 68, 0.4);
  }
  50% {
    box-shadow: 0 0 8px rgba(239, 68, 68, 0.7);
  }
}

.port-name {
  font-size: 13px;
  color: var(--text-secondary, #666);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.port-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.port-action-btn {
  width: 26px;
  height: 26px;
  border: none;
  background: none;
  color: var(--text-muted, #999);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.15s;
}

.port-action-btn:hover {
  color: var(--text-secondary, #666);
  background: var(--bg-tertiary, #f0f0f0);
}

.port-action-btn.open:hover {
  color: var(--accent-color, #0066cc);
  background: var(--bg-tertiary, #f0f0f0);
}

.port-action-btn.sandbox:hover {
  color: #8b5cf6;
  background: var(--bg-tertiary, #f0f0f0);
}

.port-action-btn.delete:hover {
  color: #dc3545;
  background: var(--bg-tertiary, #f0f0f0);
}
</style>

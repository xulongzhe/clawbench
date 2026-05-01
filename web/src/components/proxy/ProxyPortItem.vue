<template>
  <div class="proxy-port-item" :class="{ inactive: !active && !tunnelDisconnected }">
    <div class="port-info">
      <div class="port-main">
        <span class="port-number">{{ port }}</span>
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
      <button class="port-action-btn open" @click.stop="$emit('open', port, protocol)" title="打开">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
          <polyline points="15 3 21 3 21 9"/>
          <line x1="10" y1="14" x2="21" y2="3"/>
        </svg>
      </button>
      <button class="port-action-btn delete" @click.stop="$emit('remove', port)" title="删除">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="3 6 5 6 21 6"/>
          <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  port: { type: Number, required: true },
  name: { type: String, default: '' },
  protocol: { type: String, default: 'http' },
  active: { type: Boolean, default: false },
  tunnelDisconnected: { type: Boolean, default: false },
})

defineEmits(['open', 'remove'])

const statusClass = computed(() => {
  if (props.active) return 'active'
  if (props.tunnelDisconnected) return 'tunnel-down'
  return 'inactive'
})

const statusTitle = computed(() => {
  if (props.active) return '活跃'
  if (props.tunnelDisconnected) return '隧道断开'
  return '离线'
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

.port-action-btn.delete:hover {
  color: #dc3545;
  background: var(--bg-tertiary, #f0f0f0);
}
</style>

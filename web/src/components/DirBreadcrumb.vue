<template>
  <div v-if="parts.length > 0" class="dir-breadcrumb">
    <span class="crumb" @click="$emit('navigate', '')">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
        <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>
        <polyline points="9 22 9 12 15 12 15 22"/>
      </svg>
    </span>
    <template v-for="(part, i) in parts" :key="i">
      <span class="crumb-sep">›</span>
      <span
        class="crumb"
        :class="{ current: i === parts.length - 1 }"
        @click="i < parts.length - 1 && $emit('navigate', parts.slice(0, i + 1).join('/'))"
      >{{ part }}</span>
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { splitPath } from '@/utils/helpers.ts'

const props = defineProps({
  path: { type: String, default: '' },
})

defineEmits(['navigate'])

const parts = computed(() => {
  if (!props.path || props.path === '.') return []
  return splitPath(props.path)
})
</script>

<style scoped>
.dir-breadcrumb {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow-x: auto;
  font-size: 13px;
  color: var(--text-muted, #999);
  scrollbar-width: none;
}
.dir-breadcrumb::-webkit-scrollbar {
  display: none;
}

.crumb {
  padding: 3px 6px;
  border-radius: 4px;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s;
  display: flex;
  align-items: center;
}

.crumb:hover {
  background: var(--bg-secondary, #e0e0e0);
  color: var(--accent-color, #4a90d9);
}

.crumb.current {
  font-weight: 600;
  color: var(--text-primary, #1a1a1a);
  cursor: default;
}

.crumb.current:hover {
  background: none;
  color: var(--text-primary, #1a1a1a);
}

.crumb-sep {
  color: var(--text-muted, #999);
  font-size: 11px;
}
</style>

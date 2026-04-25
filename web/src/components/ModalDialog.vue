<template>
  <Teleport to="body">
    <div v-show="open" class="modal-overlay" :style="{ zIndex }" @click.self="$emit('close')">
      <div class="modal-dialog" :style="{ maxWidth, maxHeight: maxHeightValue }" @click.stop>
        <div class="modal-header">
          <span class="modal-title">{{ title }}</span>
          <button class="modal-close" @click="$emit('close')" title="关闭">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>
        <div class="modal-body">
          <slot />
        </div>
        <div v-if="$slots.footer" class="modal-footer">
          <slot name="footer" />
        </div>
        <slot name="after" />
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  open: Boolean,
  title: { type: String, default: '' },
  maxWidth: { type: String, default: '600px' },
  fullHeight: Boolean,
  zIndex: { type: Number, default: 2100 },
})

defineEmits(['close'])

const maxHeightValue = computed(() =>
  props.fullHeight ? 'none' : 'calc(100dvh - 64px)'
)
</script>

<style>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.85);
  z-index: 2100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
}

.modal-dialog {
  background: var(--bg-secondary, #fff);
  border-radius: var(--radius-md, 10px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  width: 100%;
  max-width: 600px;
  max-height: calc(100dvh - 64px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.modal-dialog[style*="max-height: none"] {
  max-height: none;
  height: calc(100dvh - 48px);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  flex-shrink: 0;
}

.modal-title {
  font-weight: 600;
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
}

.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted, #999);
  padding: 2px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
}

.modal-close:hover {
  color: var(--text-primary, #1a1a1a);
  background: var(--bg-tertiary, #f0f0f0);
}

.modal-body {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.modal-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-top: 1px solid var(--border-color, #e5e5e5);
  flex-shrink: 0;
  justify-content: flex-end;
}
</style>

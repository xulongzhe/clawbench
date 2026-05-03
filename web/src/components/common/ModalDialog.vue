<template>
  <Teleport to="body">
    <div
      v-if="everOpened"
      v-show="open || leaving"
      class="modal-overlay"
      :class="{ 'modal-leaving': leaving }"
      :style="{ zIndex }"
      @click.self="handleClose"
    >
      <div class="modal-dialog" :class="{ 'modal-leaving': leaving }" @click.stop>
        <div class="modal-header" @click="handleClose">
          <slot name="header">
            <span class="modal-title">{{ title }}</span>
          </slot>
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
import { ref, watch } from 'vue'

const props = defineProps({
  open: Boolean,
  title: { type: String, default: '' },
  zIndex: { type: Number, default: 2100 },
})

const emit = defineEmits(['close'])

const leaving = ref(false)
const everOpened = ref(false)
let leaveTimer = null

watch(() => props.open, (val) => {
  clearTimeout(leaveTimer)
  if (val) {
    everOpened.value = true
    leaving.value = false
  } else if (leaving.value) {
    // Close triggered externally while animating — cancel animation, hide now
    leaving.value = false
  }
})

function handleClose() {
  if (leaving.value) return
  leaving.value = true
  leaveTimer = setTimeout(() => {
    leaving.value = false
    leaveTimer = null
    emit('close')
  }, 250)
}

defineExpose({
  close: handleClose,
})
</script>

<style>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 2100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 44px 2px 48px;
  animation: modal-fadeIn 0.2s ease;
}

.modal-overlay.modal-leaving {
  animation: modal-fadeOut 0.25s ease forwards;
}

.modal-dialog {
  background: var(--bg-secondary, #fff);
  border-radius: 12px;
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: modal-scaleIn 0.25s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.modal-dialog.modal-leaving {
  animation: modal-scaleOut 0.25s ease forwards;
}

@keyframes modal-fadeIn {
  from { opacity: 0; }
  to   { opacity: 1; }
}

@keyframes modal-fadeOut {
  from { opacity: 1; }
  to   { opacity: 0; }
}

@keyframes modal-scaleIn {
  from {
    opacity: 0;
    transform: translateY(24px) scale(0.94);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes modal-scaleOut {
  from {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
  to {
    opacity: 0;
    transform: translateY(24px) scale(0.94);
  }
}

.modal-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  flex-shrink: 0;
  cursor: pointer;
}

.modal-header-icon {
  flex-shrink: 0;
  color: var(--text-primary, #1a1a1a);
  display: flex;
  align-items: center;
}

.modal-title {
  font-weight: 600;
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
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

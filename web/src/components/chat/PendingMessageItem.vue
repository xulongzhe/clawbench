<template>
  <div class="pending-message">
    <div class="pending-bubble">
      <span class="pending-text">{{ msg.text || '(附件消息)' }}</span>
      <button v-if="index > 0" class="pending-remove" @click="$emit('remove', index)" title="移除">×</button>
    </div>
    <span class="pending-hint">
      <span class="pending-spinner"></span>
      排队中
    </span>
  </div>
</template>

<script setup>
defineProps({
  msg: Object,
  index: Number,
})
defineEmits(['remove'])
</script>

<style scoped>
.pending-message {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  animation: pending-fade-in 0.25s ease-out;
}

.pending-bubble {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: color-mix(in srgb, var(--accent-color, #0066cc) 10%, transparent);
  border: 1px dashed color-mix(in srgb, var(--accent-color, #0066cc) 30%, transparent);
  border-radius: 12px 12px 4px 12px;
  padding: 6px 10px;
  max-width: 85%;
}

.pending-text {
  font-size: 13px;
  color: var(--text-secondary);
  word-break: break-word;
  white-space: pre-wrap;
}

.pending-remove {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted, #999);
  padding: 0 2px;
  font-size: 14px;
  line-height: 1;
  flex-shrink: 0;
  transition: color 0.15s;
}

.pending-remove:hover {
  color: var(--danger-color, #dc3545);
}

.pending-hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  color: var(--text-muted);
  padding-right: 4px;
}

.pending-spinner {
  width: 10px;
  height: 10px;
  border: 1.5px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: pending-spin 0.6s linear infinite;
}

@keyframes pending-spin {
  to { transform: rotate(360deg); }
}

@keyframes pending-fade-in {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>

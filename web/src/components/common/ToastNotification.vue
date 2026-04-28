<template>
  <Teleport to="body">
    <Transition name="toast">
      <div v-if="toast.visible.value" :class="['toast', `toast-${toast.type.value}`]" @click="toast.onClick.value ? (toast.onClick.value(), toast.dismiss()) : toast.dismiss()">
        <span v-if="toast.icon.value" class="toast-icon">{{ toast.icon.value }}</span>
        <span class="toast-text">{{ toast.message.value }}</span>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
defineProps({
    toast: {
        type: Object,
        required: true,
    },
})
</script>

<style>
.toast {
    position: fixed;
    top: 60px;
    left: 50%;
    transform: translateX(-50%);
    background: var(--accent-color, #4a90d9);
    color: #fff;
    border-radius: 24px;
    padding: 10px 18px;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 500;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
    cursor: pointer;
    z-index: 9999;
    white-space: normal;
    width: fit-content;
    min-width: 200px;
    max-width: 95vw;
    text-align: left;
    line-height: 1.4;
    -webkit-tap-highlight-color: transparent;
    user-select: none;
    transition: opacity 0.1s, transform 0.1s;
}

.toast-error {
    background: rgba(220, 38, 38, 0.9);
}

[data-theme="dark"] .toast-error {
    background: rgba(185, 28, 28, 0.9);
}

.toast-success {
    background: #27ae60;
}

[data-theme="dark"] .toast-success {
    background: #1e8449;
}

.toast-info {
    background: #3498db;
}

[data-theme="dark"] .toast-info {
    background: #2c3e50;
}

[data-theme="dark"] .toast {
    background: #1a3a5c;
    color: #e6edf3;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.5);
}

.toast:active {
    opacity: 0.8;
    transform: translateX(-50%) scale(0.97);
}

.toast-icon {
    font-size: 16px;
}

.toast-text {
    flex: 1;
}

.toast-enter-active,
.toast-leave-active {
    transition: opacity 0.25s ease, transform 0.25s ease;
}

.toast-enter-from,
.toast-leave-to {
    opacity: 0;
    transform: translateX(-50%) translateY(-12px);
}
</style>

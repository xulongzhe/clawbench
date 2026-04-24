<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="bs-overlay"
      :class="{ 'bs-leaving': leaving, 'bs-instant': instant }"
      @click.self="handleClose"
    >
      <div class="bs-panel" :class="{ 'bs-leaving': leaving, 'bs-instant': instant, 'bs-compact': compact }">
        <!-- Header -->
        <div v-if="!noHeader" class="bs-header">
          <slot name="header">
            <span class="bs-title">{{ title }}</span>
          </slot>
        </div>
        <!-- Body -->
        <div class="bs-body">
          <slot />
        </div>
        <!-- Footer slot -->
        <footer v-if="$slots.footer" class="bs-footer">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  open: Boolean,
  title: {
    type: String,
    default: '',
  },
  instant: Boolean,  // 立即关闭，无动画
  compact: Boolean,  // 紧凑模式，高度自适应内容，最大50%，无圆角
  noHeader: Boolean, // 隐藏Header
})

const emit = defineEmits(['close'])

const leaving = ref(false)
const justOpened = ref(false)

// Prevent click-through when just opened
watch(() => props.open, (val) => {
  if (val) {
    justOpened.value = true
    setTimeout(() => {
      justOpened.value = false
    }, 100)
  }
})

function handleClose() {
  if (leaving.value || justOpened.value) return  // prevent double-trigger or click-through
  if (props.instant) {
    emit('close')
    return
  }
  leaving.value = true
  setTimeout(() => {
    leaving.value = false
    emit('close')
  }, 250)  // match animation duration
}
</script>

<style>
/* ── BottomSheet base styles ── */

.bs-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: var(--dock-height, 52px);
  background: rgba(0, 0, 0, 0.5);
  z-index: 1000;
  display: flex;
  align-items: flex-end;
  animation: bs-fadeIn 0.2s ease;
}

.bs-overlay.bs-leaving {
  animation: bs-fadeOut 0.25s ease forwards;
}

.bs-panel {
  position: fixed;
  bottom: var(--dock-height, 52px);
  left: 0;
  right: 0;
  top: 0;
  background: var(--bg-secondary, #fff);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: bs-slideUp 0.25s ease;
}

/* Compact mode - auto height, no border-radius */
.bs-panel.bs-compact {
  top: auto;
  height: auto;
  max-height: 50%;
  border-radius: 0;
  box-shadow: 0 -4px 20px rgba(0, 0, 0, 0.15);
}

.bs-panel.bs-compact .bs-header {
  border-radius: 0;
}

.bs-panel.bs-leaving {
  animation: bs-slideDown 0.25s ease forwards;
}

@keyframes bs-slideUp {
  from { transform: translateY(100%); }
  to   { transform: translateY(0); }
}

@keyframes bs-slideDown {
  from { transform: translateY(0); }
  to   { transform: translateY(100%); }
}

@keyframes bs-fadeIn {
  from { opacity: 0; }
  to   { opacity: 1; }
}

@keyframes bs-fadeOut {
  from { opacity: 1; }
  to   { opacity: 0; }
}

/* Instant close (no animation) */
.bs-overlay.bs-instant {
  animation: none;
}

.bs-panel.bs-instant {
  animation: none;
}

.bs-overlay.bs-instant.bs-leaving {
  display: none;
}

.bs-panel.bs-instant.bs-leaving {
  display: none;
}

/* ── Unified Drawer Header ── */
.bs-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px;
  height: 36px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  background: var(--bg-secondary, #f8f9fa);
  flex-shrink: 0;
  user-select: none;
}

.bs-header-icon {
  flex-shrink: 0;
  color: var(--text-primary, #1a1a1a);
  display: flex;
  align-items: center;
}

.bs-header-title {
  font-weight: 600;
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  flex-shrink: 0;
  white-space: nowrap;
}

.bs-header-description {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  color: var(--text-muted, #999);
  white-space: nowrap;
  overflow-x: auto;
  overflow-y: hidden;
  cursor: grab;
  display: flex;
  align-items: center;
  touch-action: pan-x;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.bs-header-description::-webkit-scrollbar {
  display: none;
}

.bs-header-description:active {
  cursor: grabbing;
}

.bs-header-description-inner {
  padding-left: 8px;
}

.bs-close {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted, #999);
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
  flex-shrink: 0;
  margin-left: auto;
}

.bs-close:hover {
  color: var(--text-primary, #1a1a1a);
  background: var(--bg-tertiary, #f0f0f0);
}

/* ── Body ── */
.bs-body {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

/* Compact mode body - scrollable */
.bs-panel.bs-compact .bs-body {
  overflow-y: auto;
}

/* ── Footer ── */
.bs-panel > .bs-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 8px 12px;
  border-top: 1px solid var(--border-color, #e5e5e5);
  flex-shrink: 0;
  gap: 8px;
}

/* Compact mode footer — add bottom padding for dock bar clearance */
.bs-panel.bs-compact > .bs-footer {
    padding-bottom: calc(8px + env(safe-area-inset-bottom, 0px));
}
</style>

<template>
  <Teleport to="body">
    <Transition name="tts-popover">
      <div v-if="visible" class="tts-popover-backdrop" @click="$emit('close')">
        <div ref="popoverRef" class="tts-popover" :style="positionStyle" @click.stop>
          <!-- Status bar -->
          <div class="tts-popover-status">
            <span v-if="isGenerating" class="tts-status-indicator generating">
              <svg class="tts-spin-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
                <path d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM12 6v6l4 2"/>
              </svg>
              <span>总结中...</span>
            </span>
            <span v-else-if="isPlaying" class="tts-status-indicator playing">
              <span class="tts-equalizer">
                <span></span><span></span><span></span>
              </span>
              <span>朗读摘要</span>
            </span>
            <span v-else class="tts-status-indicator idle">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
                <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
                <path d="M15.54 8.46a5 5 0 0 1 0 7.07"/>
              </svg>
              <span>朗读摘要</span>
            </span>
            <button class="tts-popover-close" @click="$emit('close')" title="关闭">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
                <line x1="18" y1="6" x2="6" y2="18"/>
                <line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>
          <!-- Text content -->
          <div class="tts-popover-text">{{ text }}</div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, watch, nextTick, onBeforeUnmount } from 'vue'

const props = defineProps({
  visible: Boolean,
  text: { type: String, default: '' },
  anchorEl: { type: Object, default: null },  // HTMLElement
  isPlaying: Boolean,
  isGenerating: Boolean,
})

defineEmits(['close'])

const popoverRef = ref(null)
const positionStyle = ref({})
const isPositioned = ref(false) // Track if initial position is set

function updatePosition(skipHeightReset = false) {
  if (!props.visible || !props.anchorEl || !popoverRef.value) return

  const anchorRect = props.anchorEl.getBoundingClientRect()
  const vh = window.innerHeight
  const vw = window.innerWidth

  // Edge padding from viewport
  const edgePadding = 20

  // Calculate available space above and below the anchor
  const spaceAbove = anchorRect.top - edgePadding
  const spaceBelow = vh - anchorRect.bottom - edgePadding

  // Decide placement: prefer above, fall back to below if not enough room
  let placeAbove = spaceAbove >= 80 // need at least status bar height
  if (!placeAbove && spaceBelow < 80) {
    // Neither side has enough room — pick the larger side
    placeAbove = spaceAbove >= spaceBelow
  }

  const availableHeight = placeAbove ? spaceAbove : spaceBelow

  // Only recalculate height on initial positioning or content change
  if (!skipHeightReset) {
    // Temporarily remove height constraint to measure natural size
    popoverRef.value.style.maxHeight = 'none'
    const popoverRect = popoverRef.value.getBoundingClientRect()
    const naturalHeight = popoverRect.height
    const naturalWidth = popoverRect.width

    // Clamp popover height to available space (leave room for padding)
    const clampedHeight = Math.min(naturalHeight, availableHeight - edgePadding)
    popoverRef.value.style.maxHeight = `${Math.max(clampedHeight, 60)}px`
  }

  const popoverRect = popoverRef.value.getBoundingClientRect()
  const naturalWidth = popoverRect.width
  const currentHeight = popoverRect.height

  let top, left

  if (placeAbove) {
    top = anchorRect.top - currentHeight - 12
  } else {
    top = anchorRect.bottom + 12
  }

  // Center horizontally
  left = anchorRect.left + anchorRect.width / 2 - naturalWidth / 2

  // Clamp horizontal within viewport
  left = Math.max(edgePadding, Math.min(left, vw - naturalWidth - edgePadding))

  // Final vertical clamp
  top = Math.max(edgePadding, Math.min(top, vh - currentHeight - edgePadding))

  positionStyle.value = {
    position: 'fixed',
    top: `${top}px`,
    left: `${left}px`,
  }
}

// Update position when visible changes or anchor moves
watch(() => props.visible, async (val) => {
  if (val) {
    isPositioned.value = false
    await nextTick()
    updatePosition()
    isPositioned.value = true
  }
})

// Re-position when text content changes (summary arriving mid-display)
watch(() => props.text, async () => {
  if (props.visible) {
    await nextTick()
    updatePosition()
  }
})

// Update position on scroll/resize while open (skip height reset to preserve scroll position)
let scrollHandler = null

watch(() => props.visible, (val) => {
  if (val) {
    scrollHandler = () => updatePosition(true)
    window.addEventListener('scroll', scrollHandler, true)
    window.addEventListener('resize', scrollHandler)
  } else {
    cleanup()
  }
})

function cleanup() {
  if (scrollHandler) {
    window.removeEventListener('scroll', scrollHandler, true)
    window.removeEventListener('resize', scrollHandler)
    scrollHandler = null
  }
}

onBeforeUnmount(cleanup)
</script>

<style scoped>
/* Backdrop: transparent overlay to catch outside clicks */
.tts-popover-backdrop {
  position: fixed;
  inset: 0;
  z-index: 2200;
}

/* Popover card */
.tts-popover {
  position: fixed;
  width: min(360px, 90vw);
  background: var(--bg-secondary, #fff);
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: var(--radius-md, 10px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12), 0 2px 8px rgba(0, 0, 0, 0.06);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

:root[data-theme="dark"] .tts-popover {
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4), 0 2px 8px rgba(0, 0, 0, 0.2);
}

/* Status bar */
.tts-popover-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  flex-shrink: 0;
}

.tts-status-indicator {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 500;
}

.tts-status-indicator.generating {
  color: var(--accent-color, #0066cc);
}

.tts-status-indicator.playing {
  color: #22c55e;
}

.tts-status-indicator.idle {
  color: var(--text-muted, #999);
}

/* Spinning icon for generating state */
.tts-spin-icon {
  animation: tts-spin 1s linear infinite;
}

@keyframes tts-spin {
  to { transform: rotate(360deg); }
}

/* Equalizer animation for playing state */
.tts-equalizer {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 12px;
}

.tts-equalizer span {
  width: 3px;
  background: #22c55e;
  border-radius: 1px;
  animation: tts-eq-bar 0.8s ease-in-out infinite;
}

.tts-equalizer span:nth-child(1) {
  height: 4px;
  animation-delay: 0s;
}

.tts-equalizer span:nth-child(2) {
  height: 8px;
  animation-delay: 0.15s;
}

.tts-equalizer span:nth-child(3) {
  height: 6px;
  animation-delay: 0.3s;
}

@keyframes tts-eq-bar {
  0%, 100% { transform: scaleY(0.5); }
  50% { transform: scaleY(1.2); }
}

/* Close button */
.tts-popover-close {
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

.tts-popover-close:hover {
  color: var(--text-primary, #1a1a1a);
  background: var(--bg-tertiary, #f0f0f0);
}

/* Text content */
.tts-popover-text {
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-primary, #1a1a1a);
  white-space: pre-wrap;
  word-break: break-word;
  overflow-y: auto;
  flex: 1;
  overscroll-behavior: contain;
}

/* Transition animations */
.tts-popover-enter-active {
  transition: opacity 150ms ease-out, transform 150ms ease-out;
}

.tts-popover-leave-active {
  transition: opacity 100ms ease-in, transform 100ms ease-in;
}

.tts-popover-enter-from {
  opacity: 0;
  transform: scale(0.95);
}

.tts-popover-leave-to {
  opacity: 0;
  transform: scale(0.95);
}
</style>

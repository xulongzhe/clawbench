<template>
  <Teleport to="body">
    <div id="lightbox" class="lightbox" :style="{ display: lightboxVisible ? 'flex' : 'none' }">
      <div class="lightbox-backdrop" @click="close" />
      <div class="lightbox-toolbar">
        <div v-if="currentFileName" class="lb-filename">{{ currentFileName }}</div>
        <button class="lb-btn" @click="resetAndRefresh" title="Reset & Reload">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
        </button>
        <button class="lb-btn lb-close" @click="close" title="Close">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
      <div
        class="lightbox-content"
        :class="{ grabbing: isDragging, 'slide-left': slideDirection === 'left', 'slide-right': slideDirection === 'right' }"
        ref="contentRef"
        @wheel.prevent="handleWheel"
        @mousedown="handleMouseDown"
        @touchstart.passive="handleTouchStart"
        @touchmove="handleTouchMove"
        @touchend="handleTouchEnd"
        @touchcancel="handleTouchEnd"
      >
        <img
          v-if="currentUrl && !currentSvg"
          :src="currentUrl"
          :style="imgStyle"
          draggable="false"
          @mousedown.prevent
        />
        <div v-if="currentSvg" :style="imgStyle" v-html="currentSvg" />
      </div>
      <div class="lightbox-bottom-bar">
        <template v-if="showNav">
          <button class="lb-btn lb-nav-btn" @click="navigatePrev" title="Previous">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>
          </button>
          <span class="lb-counter">{{ currentIndex + 1 }}/{{ siblingFiles.length }}</span>
          <button class="lb-btn lb-nav-btn" @click="navigateNext" title="Next">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
          </button>
        </template>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, provide, onMounted, onUnmounted } from 'vue'
import { store } from '@/stores/app.ts'
import { baseName, getFileType } from '@/utils/helpers.ts'

const lightboxVisible = ref(false)
const currentUrl = ref('')
const currentSvg = ref('')
const currentFilePath = ref('')
const scale = ref(1)
const tx = ref(0)
const ty = ref(0)
const lastTx = ref(0)
const lastTy = ref(0)
const isDragging = ref(false)
const dragStartX = ref(0)
const dragStartY = ref(0)
const contentRef = ref(null)

// Navigation state
const siblingFiles = ref([])
const currentIndex = ref(-1)
const slideDirection = ref('') // '', 'left', 'right'

// Touch state
const pinchStartDist = ref(0)
const pinchStartScale = ref(1)
const touchStartX = ref(0)
const touchStartY = ref(0)
const touchLastX = ref(0)
const touchLastY = ref(0)
const hasMoved = ref(false)

const showNav = computed(() => currentIndex.value >= 0 && siblingFiles.value.length > 1)

const currentFileName = computed(() => {
    if (!currentFilePath.value) return ''
    return baseName(currentFilePath.value)
})

// Computed style for transform
const imgStyle = computed(() => ({
    transform: `translate(${tx.value}px, ${ty.value}px) scale(${scale.value})`,
    transition: isDragging.value ? 'none' : 'transform 0.1s ease-out'
}))

function getMediaType(filePath) {
    if (!filePath) return null
    const ft = getFileType(filePath)
    if (ft.isImage) return 'image'
    if (ft.isAudio) return 'audio'
    if (ft.isVideo) return 'video'
    return null
}

function buildSiblingList(filePath) {
    if (!filePath) { siblingFiles.value = []; currentIndex.value = -1; return }
    const mediaType = getMediaType(filePath)
    if (!mediaType) { siblingFiles.value = []; currentIndex.value = -1; return }

    const entries = store.state.dirEntries || []
    const siblings = entries.filter(e => {
        if (e.type === 'dir') return false
        return getMediaType(e.name) === mediaType
    })

    siblingFiles.value = siblings

    const fileName = baseName(filePath)
    const dir = filePath.substring(0, filePath.lastIndexOf('/'))
    const idx = siblings.findIndex(e => {
        const entryPath = dir ? dir + '/' + e.name : e.name
        return entryPath === filePath || e.name === fileName
    })
    currentIndex.value = idx
}

function navigatePrev() {
    if (!showNav.value) return
    const newIdx = (currentIndex.value - 1 + siblingFiles.value.length) % siblingFiles.value.length
    navigateToIndex(newIdx, 'right')
}

function navigateNext() {
    if (!showNav.value) return
    const newIdx = (currentIndex.value + 1) % siblingFiles.value.length
    navigateToIndex(newIdx, 'left')
}

function navigateToIndex(newIdx, direction) {
    const entry = siblingFiles.value[newIdx]
    if (!entry) return

    const currentDir = store.state.currentDir || ''
    const entryPath = currentDir ? currentDir + '/' + entry.name : entry.name

    // Animate slide direction
    slideDirection.value = direction
    setTimeout(() => { slideDirection.value = '' }, 200)

    // Reset transform for new image
    scale.value = 1
    tx.value = 0
    ty.value = 0
    lastTx.value = 0
    lastTy.value = 0

    currentIndex.value = newIdx
    currentFilePath.value = entryPath

    // Build URL for the new file
    const url = '/api/local-file/' + encodeURIComponent(entryPath)
    currentUrl.value = url + '?t=' + Date.now()
    currentSvg.value = ''

    // Sync with store
    store.selectFile(entryPath)
}

function open(url, svg = '') {
    currentUrl.value = svg ? '' : url + (url.includes('?') ? '&' : '?') + 't=' + Date.now()
    currentSvg.value = svg
    lightboxVisible.value = true
    scale.value = 1
    tx.value = 0
    ty.value = 0
    lastTx.value = 0
    lastTy.value = 0
    pinchStartDist.value = 0
    pinchStartScale.value = 1
    isDragging.value = false
    hasMoved.value = false
    slideDirection.value = ''

    // Build navigation from store's current file
    if (!svg && store.state.currentFile?.path) {
        currentFilePath.value = store.state.currentFile.path
        buildSiblingList(store.state.currentFile.path)
    } else {
        currentFilePath.value = ''
        siblingFiles.value = []
        currentIndex.value = -1
    }

    document.body.style.overflow = 'hidden'
}

function openSvg(svgContent) {
    open('', svgContent)
}

function close() {
    lightboxVisible.value = false
    currentUrl.value = ''
    currentFilePath.value = ''
    siblingFiles.value = []
    currentIndex.value = -1
    document.body.style.overflow = ''
}

function resetAndRefresh() {
    scale.value = 1
    tx.value = 0
    ty.value = 0
    lastTx.value = 0
    lastTy.value = 0
    if (currentUrl.value) {
        const base = currentUrl.value.replace(/[?&]t=\d+/, '')
        currentUrl.value = base + (base.includes('?') ? '&' : '?') + 't=' + Date.now()
    }
}

function handleWheel(e) {
    const delta = e.deltaY > 0 ? 0.85 : 1.2
    const newScale = Math.min(Math.max(scale.value * delta, 0.1), 10)
    if (newScale < 1 && scale.value >= 1) { tx.value = 0; ty.value = 0; lastTx.value = 0; lastTy.value = 0 }
    scale.value = newScale
}

// Mouse events
function handleMouseDown(e) {
    if (e.button !== 0) return // Only left click
    if (scale.value <= 1) return
    e.preventDefault()
    isDragging.value = true
    dragStartX.value = e.clientX - lastTx.value
    dragStartY.value = e.clientY - lastTy.value
}

function handleMouseMove(e) {
    if (!isDragging.value) return
    e.preventDefault()
    tx.value = e.clientX - dragStartX.value
    ty.value = e.clientY - dragStartY.value
}

function handleMouseUp() {
    if (isDragging.value) {
        isDragging.value = false
        lastTx.value = tx.value
        lastTy.value = ty.value
    }
}

// Touch events
function handleTouchStart(e) {
    if (e.touches.length === 2) {
        // Pinch to zoom
        pinchStartDist.value = Math.hypot(
            e.touches[0].clientX - e.touches[1].clientX,
            e.touches[0].clientY - e.touches[1].clientY
        )
        pinchStartScale.value = scale.value
        isDragging.value = false
    } else if (e.touches.length === 1) {
        touchStartX.value = e.touches[0].clientX
        touchStartY.value = e.touches[0].clientY
        touchLastX.value = e.touches[0].clientX
        touchLastY.value = e.touches[0].clientY
        hasMoved.value = false

        if (scale.value > 1) {
            isDragging.value = true
            dragStartX.value = e.touches[0].clientX - lastTx.value
            dragStartY.value = e.touches[0].clientY - lastTy.value
        }
    }
}

function handleTouchMove(e) {
    if (e.touches.length === 2) {
        // Pinch zoom
        e.preventDefault()
        const dist = Math.hypot(
            e.touches[0].clientX - e.touches[1].clientX,
            e.touches[0].clientY - e.touches[1].clientY
        )
        if (pinchStartDist.value > 0) {
            const s = dist / pinchStartDist.value
            scale.value = Math.min(Math.max(pinchStartScale.value * s, 0.1), 10)
        }
    } else if (e.touches.length === 1) {
        touchLastX.value = e.touches[0].clientX
        touchLastY.value = e.touches[0].clientY

        if (isDragging.value) {
            e.preventDefault()
            const dx = e.touches[0].clientX - touchStartX.value
            const dy = e.touches[0].clientY - touchStartY.value

            // Check if finger moved significantly
            if (Math.abs(dx) > 5 || Math.abs(dy) > 5) {
                hasMoved.value = true
            }

            tx.value = e.touches[0].clientX - dragStartX.value
            ty.value = e.touches[0].clientY - dragStartY.value
        }
    }
}

function handleTouchEnd(e) {
    // Check for swipe navigation (only at scale 1)
    if (scale.value === 1 && showNav.value && !hasMoved.value) {
        const dx = touchStartX.value - touchLastX.value
        const dy = touchStartY.value - touchLastY.value
        const absDx = Math.abs(dx)
        const absDy = Math.abs(dy)

        if (absDx > 50 && absDx > absDy) {
            if (dx > 0) navigateNext()  // Swipe left → next
            else navigatePrev()          // Swipe right → prev
            isDragging.value = false
            pinchStartDist.value = 0
            return
        }
    }

    // If zoomed out below 1, reset
    if (scale.value < 1) {
        scale.value = 1
        tx.value = 0
        ty.value = 0
        lastTx.value = 0
        lastTy.value = 0
    } else {
        lastTx.value = tx.value
        lastTy.value = ty.value
    }

    isDragging.value = false
    pinchStartDist.value = 0
}

provide('openLightbox', open)
provide('openSvgLightbox', openSvg)

onMounted(() => {
    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
    // Listen for clicks on images and mermaid diagrams to open lightbox
    document.addEventListener('click', (e) => {
        const img = e.target.closest('.markdown-body img, .chat-message img, .image-preview-img')
        if (img) { e.preventDefault(); open(img.src); return }
        const mermaidDiv = e.target.closest('.markdown-body .mermaid, .chat-message .mermaid')
        if (mermaidDiv) {
            e.preventDefault()
            const svg = mermaidDiv.querySelector('svg')
            if (svg) openSvg(svg.outerHTML)
        }
    })
})

onUnmounted(() => {
    document.removeEventListener('mousemove', handleMouseMove)
    document.removeEventListener('mouseup', handleMouseUp)
})
</script>

<style scoped>
.lightbox {
    position: fixed;
    inset: 0;
    z-index: 3000;
    display: flex;
    align-items: center;
    justify-content: center;
    touch-action: none;
    overscroll-behavior: none;
}

.lightbox-backdrop {
    position: absolute;
    inset: 0;
    background: var(--lb-bg, rgba(0,0,0,0.92));
    cursor: zoom-out;
}

.lightbox-toolbar {
    position: absolute;
    top: 16px;
    left: 16px;
    right: 16px;
    display: flex;
    gap: 8px;
    z-index: 10;
    align-items: center;
}

.lb-filename {
    color: rgba(255,255,255,0.85);
    font-size: 13px;
    user-select: none;
    pointer-events: none;
    background: rgba(0,0,0,0.5);
    padding: 4px 12px;
    border-radius: 12px;
    backdrop-filter: blur(4px);
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.lb-nav-btn {
    background: rgba(0,0,0,0.5) !important;
    color: rgba(255,255,255,0.9) !important;
    backdrop-filter: blur(4px);
}

.lb-nav-btn:hover {
    background: rgba(255,255,255,0.2) !important;
}

.lightbox-bottom-bar {
    position: absolute;
    bottom: 16px;
    left: 16px;
    right: 16px;
    display: flex;
    gap: 8px;
    z-index: 10;
    align-items: center;
    justify-content: center;
}

.lb-counter {
    color: rgba(255,255,255,0.7);
    font-size: 12px;
    min-width: 40px;
    text-align: center;
    user-select: none;
    pointer-events: none;
    background: rgba(0,0,0,0.5);
    padding: 2px 8px;
    border-radius: 10px;
    backdrop-filter: blur(4px);
}

.lb-btn {
    width: 40px;
    height: 40px;
    border: none;
    border-radius: 8px;
    background: var(--lb-toolbar-bg, rgba(255,255,255,0.9));
    color: var(--text-primary);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.15s, transform 0.15s;
    backdrop-filter: blur(8px);
    touch-action: manipulation;
    flex-shrink: 0;
}

.lb-btn:hover {
    background: var(--accent-color);
    transform: scale(1.05);
}

.lb-btn svg {
    width: 20px;
    height: 20px;
}

.lb-btn.lb-close:hover {
    background: #ef4444;
}

.lightbox-content {
    position: relative;
    z-index: 5;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
    touch-action: none;
    overscroll-behavior: none;
    cursor: grab;
}

.lightbox-content.grabbing {
    cursor: grabbing;
}

.lightbox-content.slide-left {
    animation: slideLeft 0.2s ease-out;
}

.lightbox-content.slide-right {
    animation: slideRight 0.2s ease-out;
}

@keyframes slideLeft {
    from { opacity: 0; transform: translateX(30px); }
    to { opacity: 1; transform: translateX(0); }
}

@keyframes slideRight {
    from { opacity: 0; transform: translateX(-30px); }
    to { opacity: 1; transform: translateX(0); }
}

.lightbox-content img {
    max-width: 100%;
    max-height: 100%;
    transform-origin: center center;
    user-select: none;
    -webkit-user-drag: none;
    -webkit-user-select: none;
    pointer-events: auto;
}

.lightbox-content :deep(svg) {
    max-width: 100%;
    max-height: 100%;
    transform-origin: center center;
    user-select: none;
    background: var(--bg-primary);
    border-radius: 4px;
}
</style>

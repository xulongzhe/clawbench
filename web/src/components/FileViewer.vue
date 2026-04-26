<template>
  <div class="file-viewer">
    <!-- Common header -->
    <FileHeader
      v-if="file && !loading && !file.error"
      :file="file"
      :view-mode="markdownViewMode"
      :toc-open="tocOpen"
      :search-open="searchOpen"
      @delete="emit('delete', file.path)"
      @toggle-view="emit('toggleView')"
      @show-details="emit('showDetails')"
      @open-git-history="emit('openGitHistory')"
      @toggle-toc="emit('toggleToc')"
      @toggle-search="emit('toggleSearch')"
    />

    <div class="file-viewer-content" ref="contentRef">
      <!-- Loading -->
      <div v-if="loading" class="loading">
        <div class="spinner" />
      </div>

      <!-- Error -->
      <div v-else-if="file.error" class="error-message">
        <strong>Error loading file:</strong> {{ file.error }}
      </div>

      <!-- Image / PDF -->
      <ImagePreview
        v-else-if="file.isImage"
        :file="file"
      />

      <!-- Audio -->
      <AudioPreview
        v-else-if="file.isAudio"
        :file="file"
      />

      <!-- Video -->
      <VideoPreview
        v-else-if="file.isVideo"
        :file="file"
      />

      <!-- Too large -->
      <div v-else-if="file.tooLarge" class="raw-content-viewer">
        <div class="unsupported-file">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14,2 14,8 20,8"/>
          </svg>
          <div class="unsupported-title">{{ file.name }}</div>
          <div class="unsupported-desc">文件过大，无法在浏览器中预览 {{ file.size ? '(' + formatSize(file.size) + ')' : '' }}</div>
          <a :href="'/api/local-file/' + encodeURIComponent(file.path)" class="download-btn" download>
            <svg viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" width="14" height="14">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <polyline points="7,10 12,15 17,10"/>
              <line x1="12" y1="15" x2="12" y2="3"/>
            </svg>
            下载
          </a>
        </div>
      </div>

      <!-- Markdown file -->
      <MarkdownPreview
        v-else-if="isMarkdown"
        :file="file"
        :view-mode="markdownViewMode"
        @delete="emit('delete', file.path)"
        @show-details="emit('showDetails')"
        @open-git-history="emit('openGitHistory')"
      />

      <!-- Code / plain text -->
      <RawFileView
        v-else
        :file="file"
        @delete="emit('delete', file.path)"
        @show-details="emit('showDetails')"
        @open-git-history="emit('openGitHistory')"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import ImagePreview from './ImagePreview.vue'
import AudioPreview from './AudioPreview.vue'
import VideoPreview from './VideoPreview.vue'
import MarkdownPreview from './MarkdownPreview.vue'
import RawFileView from './RawFileView.vue'
import FileHeader from './FileHeader.vue'
import { getFileType, formatFileSize } from '@/utils/helpers.ts'

const props = defineProps({
    file: Object,
    tocOpen: Boolean,
    searchOpen: Boolean,
    markdownViewMode: String,
})
const emit = defineEmits(['delete', 'showDetails', 'openGitHistory', 'toggleToc', 'toggleSearch', 'toggleView'])

const fileType = computed(() => props.file ? getFileType(props.file.name) : null)
const isMarkdown = computed(() => fileType.value?.isMarkdown || false)
const loading = ref(false)
const contentRef = ref(null)

// Per-file scroll position cache
const scrollPositions = new Map()
let pendingRestore = null // { path, scrollTop }
let restoreTimer = null
let currentFilePath = null
let scrollHandler = null
let scrollEl = null // reference to the element we attached scroll listener to

function clearRestoreTimer() {
    if (restoreTimer) {
        clearInterval(restoreTimer)
        restoreTimer = null
    }
}

// Find the actual scroll container based on file type
function getScrollEl() {
    const el = contentRef.value
    if (!el) return null
    if (isMarkdown.value) {
        return el.querySelector('.markdown-body')
    }
    return el.querySelector('.raw-content-pre')
}

// Listen for scroll events on the actual scroll container and save position
function attachScrollListener() {
    detachScrollListener()
    const el = getScrollEl()
    if (!el || !currentFilePath) return
    scrollEl = el
    scrollHandler = () => {
        scrollPositions.set(currentFilePath, el.scrollTop)
    }
    el.addEventListener('scroll', scrollHandler, { passive: true })
}

function detachScrollListener() {
    if (scrollHandler && scrollEl) {
        scrollEl.removeEventListener('scroll', scrollHandler)
    }
    scrollHandler = null
    scrollEl = null
}

function tryRestoreOrAttach() {
    if (loading.value) return
    const el = getScrollEl()
    if (!el) return
    // Content must be scrollable (scrollHeight > clientHeight)
    if (el.scrollHeight <= el.clientHeight) return

    // Restore scroll if needed
    if (pendingRestore) {
        el.scrollTop = pendingRestore.scrollTop
        pendingRestore = null
        clearRestoreTimer()
    }
    // Always attach listener once content is ready
    attachScrollListener()
}

onBeforeUnmount(() => {
    detachScrollListener()
    clearRestoreTimer()
})

// Save/restore scroll position when switching files
watch(() => props.file, (f, oldF) => {
    // Stop listening on old scroll container
    detachScrollListener()

    clearRestoreTimer()
    if (!f) { currentFilePath = null; loading.value = true; return }
    currentFilePath = f.path
    if (f.isImage || f.isAudio || f.isVideo || f.tooLarge || f.error) {
        loading.value = false
    } else {
        loading.value = f.content == null
    }
    if (f?.path !== oldF?.path) {
        const savedScroll = scrollPositions.get(f.path)
        if (savedScroll != null) {
            pendingRestore = { path: f.path, scrollTop: savedScroll }
        }
        // Poll until content is rendered and scrollable
        restoreTimer = setInterval(tryRestoreOrAttach, 50)
        tryRestoreOrAttach()
    }
}, { immediate: true })

watch(() => props.file?.content, (content) => {
    if (!props.file) return
    if (props.file.isImage || props.file.isAudio || props.file.isVideo || props.file.tooLarge || props.file.error) return
    loading.value = content == null
    // Content loaded, try restore or attach listener
    if (content != null) {
        tryRestoreOrAttach()
    }
})

function formatSize(bytes) {
    return formatFileSize(bytes)
}
</script>

<style scoped>
.file-viewer {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
    position: relative;
}

.file-viewer-content {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
}

.unsupported-file {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px 24px;
    text-align: center;
    height: 100%;
}

.unsupported-file > svg {
    width: 48px;
    height: 48px;
    color: var(--text-muted);
    margin-bottom: 12px;
}

.unsupported-title {
    font-size: 16px;
    font-weight: 500;
    color: var(--text-primary);
    margin-bottom: 8px;
    word-break: break-all;
}

.unsupported-desc {
    font-size: 14px;
    color: var(--text-muted);
    margin-bottom: 20px;
}

.download-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 5px 12px;
    background: var(--accent-color);
    color: #fff;
    border-radius: 14px;
    text-decoration: none;
    font-size: 12px;
    font-weight: 500;
    transition: filter 0.15s;
    gap: 4px;
    line-height: 1;
}

.download-btn svg {
    flex-shrink: 0;
}

.download-btn:hover {
    filter: brightness(1.15);
}

.loading {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 40px;
}

.error-message {
    background: #fef2f2;
    border: 1px solid #fecaca;
    color: #991b1b;
    padding: 16px;
    border-radius: var(--radius-md);
    margin: 20px 0;
}
</style>

<style>
[data-theme="dark"] .error-message {
    background: #450a0a;
    border-color: #7f1d1d;
    color: #fca5a5;
}
</style>

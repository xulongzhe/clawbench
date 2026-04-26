<template>
  <div class="file-header-bar">
    <span class="lang-badge" :style="badgeStyle">{{ badgeLabel }}</span>
    <div class="file-name-wrap">
      <span class="file-path-hint" style="cursor:pointer" @click="$emit('showDetails')" :title="file.name">{{ file.name }}</span>
    </div>
    <div class="header-actions">
      <!-- TOC button (standalone, not in dropdown) -->
      <button class="file-header-btn" :class="{ active: tocOpen }" @click.stop="$emit('toggleToc')" title="目录">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
          <line x1="8" y1="6" x2="21" y2="6"/>
          <line x1="8" y1="12" x2="21" y2="12"/>
          <line x1="8" y1="18" x2="21" y2="18"/>
          <line x1="3" y1="6" x2="5" y2="6"/>
          <line x1="3" y1="12" x2="5" y2="12"/>
          <line x1="3" y1="18" x2="5" y2="18"/>
        </svg>
      </button>


      <!-- Search button (standalone, not in dropdown) -->
      <button class="file-header-btn" :class="{ active: searchOpen }" :disabled="!file.content" @click.stop="$emit('toggleSearch')" title="搜索">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
          <circle cx="11" cy="11" r="8"/>
          <line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
      </button>

      <!-- More actions dropdown -->
      <div class="dropdown-wrapper" ref="dropdownRef">
        <button class="file-header-btn" @click.stop="menuOpen = !menuOpen" title="更多">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
            <circle cx="12" cy="5" r="1"/>
            <circle cx="12" cy="12" r="1"/>
            <circle cx="12" cy="19" r="1"/>
          </svg>
        </button>
        <div v-if="menuOpen" class="dropdown-menu">
          <button v-if="isMarkdown" class="dropdown-item" @click="handleToggleView">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <polyline points="16 18 22 12 16 6"/>
              <polyline points="8 6 2 12 8 18"/>
            </svg>
            {{ viewMode === 'rendered' ? '源码' : '渲染' }}
          </button>
          <a class="dropdown-item" :href="'/api/local-file/' + encodeURIComponent(file.path)" :download="file.name" @click="menuOpen = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <polyline points="7,10 12,15 17,10"/>
              <line x1="12" y1="15" x2="12" y2="3"/>
            </svg>
            下载
          </a>
          <button class="dropdown-item" @click="handleDelete">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <polyline points="3,6 5,6 21,6"/>
              <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>
              <path d="M10 11v6M14 11v6"/>
              <path d="M9 6V4h6v2"/>
            </svg>
            删除
          </button>
          <button class="dropdown-item" @click="handleGitHistory">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <line x1="6" y1="3" x2="6" y2="15"/>
              <circle cx="18" cy="6" r="3"/>
              <circle cx="6" cy="18" r="3"/>
              <path d="M15 6a9 9 0 0 0-9 9V3"/>
            </svg>
            代码历史
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { getFileType } from '@/utils/helpers.ts'

const props = defineProps({
    file: Object,
    viewMode: String,
    tocOpen: Boolean,
    searchOpen: Boolean,
})
const emit = defineEmits(['delete', 'toggleView', 'showDetails', 'openGitHistory', 'toggleToc', 'toggleSearch'])

const menuOpen = ref(false)
const dropdownRef = ref(null)

const fileType = computed(() => props.file ? getFileType(props.file.name) : null)
const isMarkdown = computed(() => fileType.value?.isMarkdown || false)

const badgeLabel = computed(() => {
    if (!props.file) return ''
    return getFileType(props.file.name)?.label || 'TXT'
})

const badgeColor = computed(() => {
    if (!props.file) return '#8b8b8b'
    return getFileType(props.file.name)?.color || '#8b8b8b'
})

const badgeStyle = computed(() => ({
    background: badgeColor.value + '22',
    color: badgeColor.value,
    border: `1px solid ${badgeColor.value}44`,
}))

function handleToggleView() {
    menuOpen.value = false
    emit('toggleView')
}

function handleDelete() {
    menuOpen.value = false
    if (!confirm(`确定要删除"${props.file?.name}"吗？`)) return
    emit('delete', props.file?.path)
}

function handleGitHistory() {
    menuOpen.value = false
    emit('openGitHistory')
}

// Close dropdown on outside click
function handleClickOutside(e) {
    if (dropdownRef.value && !dropdownRef.value.contains(e.target)) {
        menuOpen.value = false
    }
}

onMounted(() => {
    document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
    document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.file-header-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-left: none;
    border-right: none;
    border-radius: 0;
    z-index: 10;
    border-bottom: none;
    font-size: 13px;
    position: sticky;
    top: 0;
    left: 0;
    min-width: 0;
}

.file-name-wrap {
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
}

.lang-badge {
    font-size: 11px;
    font-weight: 700;
    padding: 3px 8px;
    border-radius: 5px;
    flex-shrink: 0;
    font-family: monospace;
    letter-spacing: 0.5px;
}

.file-path-hint {
    flex: 0 0 auto;
    max-width: 100%;
    color: var(--text-muted);
    font-family: monospace;
    font-size: 12px;
    overflow-x: auto;
    white-space: nowrap;
    cursor: pointer;
    transition: color 0.15s;
    scrollbar-width: none;
}
.file-path-hint::-webkit-scrollbar {
    display: none;
}
.file-path-hint:hover {
    color: var(--accent-color);
}
.file-path-hint.copied {
    color: #22c55e;
}

.header-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: auto;
    flex-shrink: 0;
}

.file-header-btn {
    margin-left: auto;
    padding: 4px 12px;
    border: 1px solid var(--border-color);
    background: var(--bg-tertiary);
    border-radius: var(--radius-sm);
    font-size: 12px;
    cursor: pointer;
    color: var(--text-primary);
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 4px;
}
.file-header-btn:hover {
    background: var(--accent-color);
    color: #fff;
    border-color: var(--accent-color);
}
.file-header-btn svg {
    width: 13px;
    height: 13px;
}
.file-header-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
    pointer-events: none;
}
.file-header-btn:disabled:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
    border-color: var(--border-color);
}
.file-header-btn.active {
    background: var(--accent-color);
    color: #fff;
    border-color: var(--accent-color);
}

/* Dropdown */
.dropdown-wrapper {
    position: relative;
}

.dropdown-menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 4px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    z-index: 100;
    min-width: 140px;
    padding: 4px 0;
    overflow: hidden;
}

.dropdown-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    width: 100%;
    border: none;
    background: none;
    color: var(--text-primary);
    font-size: 13px;
    cursor: pointer;
    text-decoration: none;
    white-space: nowrap;
}
.dropdown-item:hover {
    background: var(--accent-color);
    color: #fff;
}
.dropdown-item svg {
    flex-shrink: 0;
}
</style>

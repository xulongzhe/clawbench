<template>
  <Teleport to="body">
  <header class="header">
    <img class="header-logo" src="/logo.png" alt="ClawBench">

    <div class="project-dropdown-wrapper" ref="dropdownRef">
      <button class="project-switch-btn" @click="toggleDropdown" title="切换项目">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="16" height="16">
          <rect x="3" y="3" width="7" height="7" rx="1.5"/>
          <rect x="14" y="3" width="7" height="7" rx="1.5"/>
          <rect x="3" y="14" width="7" height="7" rx="1.5"/>
          <rect x="14" y="14" width="7" height="7" rx="1.5"/>
        </svg>
        <span class="project-name">{{ projectName }}</span>
        <svg class="switch-chevron" :class="{ open: dropdownOpen }" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="12" height="12">
          <polyline points="6 9 12 15 18 9"/>
        </svg>
      </button>
      <Transition name="dropdown">
        <div v-if="dropdownOpen" class="project-dropdown">
          <div v-if="loadingRecent" class="dropdown-loading">加载中...</div>
          <template v-else>
            <div v-if="recentItems.length === 0" class="dropdown-empty">暂无最近项目</div>
            <div
              v-for="item in recentItems"
              :key="item.path"
              class="dropdown-item"
              :class="{ active: item.path === projectRoot }"
              @click="selectRecent(item)"
            >
              <svg class="item-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="14" height="14">
                <rect x="3" y="3" width="7" height="7" rx="1.5"/>
                <rect x="14" y="3" width="7" height="7" rx="1.5"/>
                <rect x="3" y="14" width="7" height="7" rx="1.5"/>
                <rect x="14" y="14" width="7" height="7" rx="1.5"/>
              </svg>
              <span class="item-label">{{ item.name }}</span>
              <span class="item-path" @mousedown.prevent="onPathMouseDown" @click.stop>{{ item.displayPath }}</span>
            </div>
            <div class="dropdown-divider"></div>
            <div class="dropdown-item other-item" @click="openBrowse">
              <svg class="item-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="14" height="14">
                <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
              </svg>
              <span class="item-label">浏览...</span>
            </div>
          </template>
        </div>
      </Transition>
    </div>

    <button class="theme-toggle" @click="$emit('toggleTheme')" aria-label="Toggle theme">
      <svg v-if="theme === 'dark'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
      </svg>
      <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="5"/>
        <line x1="12" y1="1" x2="12" y2="3"/>
        <line x1="12" y1="21" x2="12" y2="23"/>
        <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
        <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
        <line x1="1" y1="12" x2="3" y2="12"/>
        <line x1="21" y1="12" x2="23" y2="12"/>
        <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
        <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
      </svg>
    </button>
  </header>
  </Teleport>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, inject } from 'vue'
import { baseName } from '@/utils/helpers.ts'

const props = defineProps({
    projectRoot: String,
    theme: String,
})
const emit = defineEmits(['toggleTheme', 'openProjectDialog'])

const toast = inject('toast')

const projectName = computed(() => {
    if (!props.projectRoot) return '选择项目'
    return baseName(props.projectRoot) || props.projectRoot
})

// Dropdown state
const dropdownOpen = ref(false)
const dropdownRef = ref(null)
const loadingRecent = ref(false)
const recentItems = ref([])

let watchBase = ''

function toRelative(absPath) {
    if (!watchBase) return absPath
    const rel = absPath.slice(watchBase.length).replace(/^\//, '')
    return rel || '/'
}

function toggleDropdown() {
    if (dropdownOpen.value) {
        dropdownOpen.value = false
    } else {
        loadRecentProjects()
        dropdownOpen.value = true
    }
}

async function loadRecentProjects() {
    loadingRecent.value = true
    try {
        const wdResp = await fetch('/api/watch-dir')
        if (wdResp.ok) {
            const wd = await wdResp.json()
            watchBase = wd.watchDir || ''
        }
        const resp = await fetch('/api/recent-projects')
        const paths = await resp.json()
        recentItems.value = paths.map(p => {
            const rel = toRelative(p)
            const name = baseName(rel)
            return { name, path: p, displayPath: rel }
        })
    } catch (_) {
        recentItems.value = []
    } finally {
        loadingRecent.value = false
    }
}

async function selectRecent(item) {
    dropdownOpen.value = false
    if (item.path === props.projectRoot) return
    try {
        const resp = await fetch('/api/project', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: item.path })
        })
        if (resp.ok) {
            window.location.reload()
        } else {
            const text = await resp.text()
            let msg = text
            try { msg = JSON.parse(text).error || msg } catch (_) {}
            if (msg === 'Not a directory') {
                toast?.show('项目路径不存在或已被删除', { icon: '⚠️', type: 'error', duration: 3000 })
                // Remove stale entry from recent projects
                fetch('/api/recent-projects', {
                    method: 'DELETE',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: item.path })
                }).catch(() => {})
                // Remove from local list immediately
                recentItems.value = recentItems.value.filter(r => r.path !== item.path)
            } else {
                toast?.show('切换项目失败: ' + msg, { icon: '⚠️', type: 'error', duration: 3000 })
            }
        }
    } catch (err) {
        toast?.show('切换项目失败: 网络错误', { icon: '⚠️', type: 'error', duration: 3000 })
    }
}

function openBrowse() {
    dropdownOpen.value = false
    emit('openProjectDialog')
}

// Close dropdown on outside click
function onClickOutside(e) {
    if (dropdownRef.value && !dropdownRef.value.contains(e.target)) {
        dropdownOpen.value = false
    }
}

function onPathMouseDown(e) {
    const el = e.currentTarget
    if (el.scrollWidth <= el.clientWidth) return
    let startX = e.pageX
    let scrollLeft = el.scrollLeft
    let moved = false

    function onMouseMove(ev) {
        const dx = ev.pageX - startX
        if (Math.abs(dx) > 2) moved = true
        el.scrollLeft = scrollLeft - dx
    }
    function onMouseUp(ev) {
        if (moved) {
            ev.preventDefault()
            ev.stopPropagation()
        }
        document.removeEventListener('mousemove', onMouseMove)
        document.removeEventListener('mouseup', onMouseUp)
    }
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
}

onMounted(() => {
    document.addEventListener('click', onClickOutside)
})

onUnmounted(() => {
    document.removeEventListener('click', onClickOutside)
})
</script>

<style scoped>
.header-logo {
    width: 28px;
    height: 28px;
    border-radius: 6px;
    flex-shrink: 0;
}

.project-dropdown-wrapper {
    position: relative;
    flex-shrink: 0;
}

.project-switch-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 3px 6px 3px 8px;
    border: 1px solid var(--border-color);
    background: var(--bg-secondary);
    cursor: pointer;
    color: var(--text-primary);
    border-radius: 999px;
    font-size: 13px;
    font-weight: 500;
    max-width: 220px;
    transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
    line-height: 1;
}

.project-switch-btn:hover {
    background: var(--bg-primary);
    border-color: var(--accent-color);
    box-shadow: 0 0 0 1px var(--accent-color);
}

.project-switch-btn:active {
    transform: scale(0.97);
}

.project-switch-btn svg:first-child {
    color: var(--accent-color);
    flex-shrink: 0;
}

.switch-chevron {
    color: var(--text-muted);
    margin-left: -2px;
    transition: transform 0.2s;
}

.switch-chevron.open {
    transform: rotate(180deg);
}

.project-switch-btn:hover .switch-chevron {
    color: var(--accent-color);
}

.project-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

/* Dropdown */
.project-dropdown {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    min-width: 220px;
    max-width: 280px;
    background: var(--bg-primary);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    box-shadow: 0 4px 16px rgba(0,0,0,0.1);
    z-index: 3000;
    overflow: hidden;
    padding: 3px 0;
}

.dropdown-loading,
.dropdown-empty {
    text-align: center;
    padding: 10px 12px;
    color: var(--text-muted);
    font-size: 12px;
}

.dropdown-item {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 10px;
    cursor: pointer;
    transition: background 0.1s;
    font-size: 12px;
}

.dropdown-item:hover {
    background: var(--bg-tertiary);
}

.dropdown-item.active {
    background: var(--accent-color);
    color: #fff;
}

.dropdown-item.active .item-icon {
    color: #fff;
}

.dropdown-item.active .item-path {
    color: rgba(255,255,255,0.6);
}

.item-icon {
    flex-shrink: 0;
    color: var(--accent-color);
}

.dropdown-item.active .item-icon {
    color: #fff;
}

.item-label {
    flex-shrink: 0;
    font-weight: 500;
    white-space: nowrap;
}

.item-path {
    flex: 1;
    color: var(--text-muted);
    font-size: 11px;
    overflow-x: auto;
    overflow-y: hidden;
    white-space: nowrap;
    cursor: default;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.item-path::-webkit-scrollbar {
    display: none;
}

.other-item .item-icon {
    color: var(--text-secondary);
}

.dropdown-divider {
    height: 1px;
    background: var(--border-color);
    margin: 2px 0;
}

/* Dropdown transition */
.dropdown-enter-active,
.dropdown-leave-active {
    transition: opacity 0.15s, transform 0.15s;
}

.dropdown-enter-from,
.dropdown-leave-to {
    opacity: 0;
    transform: translateY(-4px);
}

.theme-toggle {
    padding: 6px;
    border: none;
    background: transparent;
    cursor: pointer;
    color: var(--text-primary);
    border-radius: var(--radius-sm);
    transition: background 0.15s;
    flex-shrink: 0;
    margin-left: auto;
}

.theme-toggle:hover {
    background: var(--bg-tertiary);
}

.theme-toggle svg {
    width: 20px;
    height: 20px;
}
</style>

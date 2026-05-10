<template>
  <Teleport to="body">
  <header class="header">
    <img class="header-logo" src="/logo.png" alt="ClawBench">

    <div class="project-dropdown-wrapper" ref="dropdownRef">
      <button class="project-switch-btn" @click="toggleDropdown" :title="t('appHeader.switchProject')">
        <Projector :size="16" />
        <span class="project-name">{{ projectName }}</span>
        <ChevronDown :size="12" class="switch-chevron" :class="{ open: dropdownOpen }" />
      </button>
      <Transition name="dropdown">
        <div v-if="dropdownOpen" class="project-dropdown">
          <div v-if="loadingRecent" class="dropdown-loading">{{ t('common.loading') }}</div>
          <template v-else>
            <div v-if="recentItems.length === 0" class="dropdown-empty">{{ t('appHeader.noRecentProjects') }}</div>
            <div
              v-for="item in recentItems"
              :key="item.path"
              class="dropdown-item"
              :class="{ active: item.path === projectRoot }"
              @click="selectRecent(item)"
            >
              <Projector :size="14" class="item-icon" />
              <span class="item-label">{{ item.name }}</span>
              <span class="item-path" @mousedown.prevent="onPathMouseDown" @click.stop>{{ item.displayPath }}</span>
            </div>
            <div class="dropdown-divider"></div>
            <div class="dropdown-item other-item" @click="openBrowse">
              <Search :size="14" class="item-icon" />
              <span class="item-label">{{ t('appHeader.browse') }}</span>
            </div>
          </template>
        </div>
      </Transition>
    </div>

    <button class="locale-toggle" @click="toggleLocale" :title="currentLocale === 'zh' ? t('locale.switchToEn') : t('locale.switchToZh')">
      {{ localeLabel }}
    </button>
    <button class="theme-toggle" @click="$emit('toggleTheme')" aria-label="Toggle theme">
      <Moon v-if="theme === 'dark'" :size="20" />
      <Sun v-else :size="20" />
    </button>
  </header>
  </Teleport>
</template>

<script setup>
import { Projector, ChevronDown, Search, Moon, Sun } from 'lucide-vue-next'
import { ref, computed, onMounted, onUnmounted, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { useLocale } from '@/composables/useLocale'
import { baseName } from '@/utils/path.ts'

const { t } = useI18n()
const { currentLocale, toggleLocale, localeLabel } = useLocale()

const props = defineProps({
    projectRoot: String,
    theme: String,
})
const emit = defineEmits(['toggleTheme', 'openProjectDialog'])

const toast = inject('toast')

const projectName = computed(() => {
    if (!props.projectRoot) return t('appHeader.selectProject')
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
            let msgKey = ''
            try {
                const parsed = JSON.parse(text)
                msg = parsed.error || msg
                msgKey = parsed.msgKey || ''
            } catch (_) {}
            if (msgKey === 'NotADirectory') {
                toast?.show(t('appHeader.projectPathNotFound'), { icon: '⚠️', type: 'error', duration: 3000 })
                // Remove stale entry from recent projects
                fetch('/api/recent-projects', {
                    method: 'DELETE',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: item.path })
                }).catch(() => {})
                // Remove from local list immediately
                recentItems.value = recentItems.value.filter(r => r.path !== item.path)
            } else {
                toast?.show(t('appHeader.switchProjectFailed', { error: msg }), { icon: '⚠️', type: 'error', duration: 3000 })
            }
        }
    } catch (err) {
        toast?.show(t('appHeader.switchProjectNetworkError'), { icon: '⚠️', type: 'error', duration: 3000 })
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
    border-radius: 50%;
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
    line-height: 1.4;
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

.locale-toggle {
    padding: 4px 8px;
    border: 1px solid var(--border-color);
    background: transparent;
    cursor: pointer;
    color: var(--text-secondary);
    border-radius: var(--radius-sm);
    transition: background 0.15s, border-color 0.15s;
    flex-shrink: 0;
    margin-left: auto;
    font-size: 12px;
    font-weight: 600;
    line-height: 1.4;
}

.locale-toggle:hover {
    background: var(--bg-tertiary);
    border-color: var(--accent-color);
    color: var(--accent-color);
}

.locale-toggle:active {
    transform: scale(0.95);
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
}

.theme-toggle:hover {
    background: var(--bg-tertiary);
}

.theme-toggle svg {
    width: 20px;
    height: 20px;
}
</style>

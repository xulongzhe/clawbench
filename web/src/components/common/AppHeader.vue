<template>
  <Teleport to="body">
  <header v-show="!props.hidden" class="header">
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
              <span class="item-path" @mousedown.prevent="onPathMouseDown" @click="onPathClick">{{ item.displayPath }}</span>
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

    <button ref="statusBtnRef" class="status-toggle" @click="toggleStatusMenu" :title="t('appHeader.connectionStatus')">
      <span class="status-dot" :class="statusDotClass"></span>
    </button>
    <PopupMenu v-model:show="statusMenuOpen" :target-element="statusBtnRef" :max-width="200" :max-height="160" :menu-items-count="3">
      <div class="status-menu-title">{{ t('appHeader.connectionStatus') }}</div>
      <div class="status-menu-item">
        <span class="status-indicator" :class="wsDotClass"></span>
        <span class="status-label">{{ t('appHeader.websocket') }}</span>
        <span class="status-value">{{ wsStatusLabel }}</span>
      </div>
      <div class="status-menu-divider"></div>
      <div class="status-menu-item">
        <span class="status-indicator" :class="pushDotClass"></span>
        <span class="status-label">{{ t('appHeader.jpush') }}</span>
        <span class="status-value">{{ pushStatusLabel }}</span>
      </div>
    </PopupMenu>

    <button ref="settingsBtnRef" class="settings-toggle" @click="toggleSettingsMenu" :title="t('appHeader.settings')">
      <Settings :size="20" />
    </button>
    <PopupMenu v-model:show="settingsMenuOpen" :target-element="settingsBtnRef" :max-width="200" :max-height="280" :menu-items-count="settingsItemCount">
      <div class="settings-menu-title">{{ t('appHeader.settings') }}</div>
      <!-- Reconfigure server — always available in app mode -->
      <template v-if="isAppMode">
        <button class="settings-menu-item reconfigure-item" @click="handleReconfigure">
          <Server :size="14" />
          <span>{{ t('appHeader.reconfigureServer') }}</span>
        </button>
        <div class="settings-menu-divider"></div>
      </template>
      <button class="settings-menu-item" :class="{ active: currentLocale === 'zh' }" @click="handleLocaleSwitch('zh')">
          <Check v-if="currentLocale === 'zh'" :size="14" />
          <span v-else class="settings-menu-check-spacer"></span>
          <span>中文</span>
        </button>
        <button class="settings-menu-item" :class="{ active: currentLocale === 'en' }" @click="handleLocaleSwitch('en')">
          <Check v-if="currentLocale === 'en'" :size="14" />
          <span v-else class="settings-menu-check-spacer"></span>
          <span>English</span>
        </button>
        <div class="settings-menu-divider"></div>
        <button class="settings-menu-item" :class="{ active: theme === 'dark' }" @click="handleThemeSwitch('dark')">
          <Check v-if="theme === 'dark'" :size="14" />
          <span v-else class="settings-menu-check-spacer"></span>
          <Moon :size="14" />
          <span>{{ t('appHeader.darkMode') }}</span>
        </button>
        <button class="settings-menu-item" :class="{ active: theme === 'light' }" @click="handleThemeSwitch('light')">
          <Check v-if="theme === 'light'" :size="14" />
          <span v-else class="settings-menu-check-spacer"></span>
          <Sun :size="14" />
        <span>{{ t('appHeader.lightMode') }}</span>
      </button>
    </PopupMenu>
  </header>
  </Teleport>
</template>

<script setup>
import { Projector, ChevronDown, Search, Moon, Sun, Settings, Check, Server } from 'lucide-vue-next'
import { ref, computed, onMounted, onUnmounted, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { useLocale } from '@/composables/useLocale'
import { useAppMode } from '@/composables/useAppMode'
import { useGlobalEvents } from '@/composables/useGlobalEvents'
import { baseName, toRelativePath } from '@/utils/path.ts'
import PopupMenu from '@/components/common/PopupMenu.vue'

const { t } = useI18n()
const { currentLocale, setLocale } = useLocale()
const { isAppMode } = useAppMode()
const { wsStatus, pushRegistered } = useGlobalEvents()

const props = defineProps({
    projectRoot: String,
    theme: String,
    hidden: Boolean,
})
const emit = defineEmits(['toggleTheme', 'openProjectDialog', 'reconfigureServer'])

const toast = inject('toast')

// Settings menu state
const settingsBtnRef = ref(null)
const settingsMenuOpen = ref(false)

// Connection status menu state
const statusBtnRef = ref(null)
const statusMenuOpen = ref(false)

function toggleStatusMenu() {
    statusMenuOpen.value = !statusMenuOpen.value
}

// Status dot class for the button indicator (worst status wins)
const statusDotClass = computed(() => {
    if (wsStatus.value === 'disconnected') return 'status-dot-disconnected'
    if (wsStatus.value === 'reconnecting') return 'status-dot-reconnecting'
    return 'status-dot-connected'
})

// WS status dot and label
const wsDotClass = computed(() => {
    if (wsStatus.value === 'connected') return 'status-indicator-connected'
    if (wsStatus.value === 'reconnecting') return 'status-indicator-reconnecting'
    return 'status-indicator-disconnected'
})

const wsStatusLabel = computed(() => {
    if (wsStatus.value === 'connected') return t('appHeader.wsConnected')
    if (wsStatus.value === 'reconnecting') return t('appHeader.wsReconnecting')
    return t('appHeader.wsDisconnected')
})

// Push status dot and label
const pushDotClass = computed(() => {
    return pushRegistered.value ? 'status-indicator-connected' : 'status-indicator-disabled'
})

const pushStatusLabel = computed(() => {
    return pushRegistered.value ? t('appHeader.pushRegistered') : t('appHeader.pushNotEnabled')
})

function toggleSettingsMenu() {
    settingsMenuOpen.value = !settingsMenuOpen.value
}

function handleLocaleSwitch(lang) {
    if (currentLocale.value !== lang) {
        setLocale(lang)
    }
    settingsMenuOpen.value = false
}

function handleThemeSwitch(mode) {
    if (props.theme !== mode) {
        emit('toggleTheme')
    }
    settingsMenuOpen.value = false
}

function handleReconfigure() {
    settingsMenuOpen.value = false
    emit('reconfigureServer')
}

// Calculate menu item count for PopupMenu positioning
const settingsItemCount = computed(() => {
    // 4 interactive items: zh + en + dark + light (divider height negligible)
    let count = 4
    if (isAppMode.value) {
        count += 2 // reconfigure item + divider
    }
    return count
})

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
    return toRelativePath(absPath, watchBase)
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

// Track whether the path element was dragged, so click can decide to bubble or not
let pathDragged = false

function onPathMouseDown(e) {
    const el = e.currentTarget
    pathDragged = false
    if (el.scrollWidth <= el.clientWidth) return
    let startX = e.pageX
    let scrollLeft = el.scrollLeft

    function onMouseMove(ev) {
        const dx = ev.pageX - startX
        if (Math.abs(dx) > 2) pathDragged = true
        el.scrollLeft = scrollLeft - dx
    }
    function onMouseUp() {
        document.removeEventListener('mousemove', onMouseMove)
        document.removeEventListener('mouseup', onMouseUp)
    }
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
}

function onPathClick(e) {
    if (pathDragged) {
        e.stopPropagation()
    }
    // If not dragged, let the click bubble up to the parent .dropdown-item's selectRecent
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

.settings-toggle {
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

/* Connection status button */
.status-toggle {
    padding: 6px;
    border: none;
    background: transparent;
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: background 0.15s;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
}

.status-toggle:hover {
    background: var(--bg-tertiary);
}

.status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    transition: background-color 0.3s;
}

.status-dot-connected {
    background: var(--color-green, #22c55e);
}

.status-dot-reconnecting {
    background: var(--color-yellow, #eab308);
    animation: status-pulse 1.2s ease-in-out infinite;
}

.status-dot-disconnected {
    background: var(--color-red, #ef4444);
}

@keyframes status-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
}

.settings-toggle:hover {
    background: var(--bg-tertiary);
}

.settings-toggle svg {
    width: 20px;
    height: 20px;
}
</style>

<!-- Unscoped styles for teleported settings menu content (PopupMenu uses Teleport to body, scoped styles won't reach it) -->
<style>
.settings-menu-title {
    padding: 4px 10px 1px;
    font-size: 10px;
    color: var(--text-muted, #999);
    font-weight: 500;
    letter-spacing: 0.3px;
}

.settings-menu-item {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    width: 100%;
    border: none;
    background: none;
    color: var(--text-primary);
    font-size: 12px;
    cursor: pointer;
    white-space: nowrap;
    text-align: left;
}

.settings-menu-item:hover {
    background: var(--accent-color, #0066cc);
    color: #fff;
}

.settings-menu-item.active {
    color: var(--accent-color, #0066cc);
    font-weight: 500;
}

.settings-menu-item.active:hover {
    color: #fff;
}

.settings-menu-item.reconfigure-item {
    color: var(--color-red, #dc2626);
}

.settings-menu-item.reconfigure-item:hover {
    background: color-mix(in srgb, var(--color-red, #dc2626) 10%, transparent);
    color: var(--color-red, #dc2626);
}

.settings-menu-item svg {
    flex-shrink: 0;
    width: 14px;
    height: 14px;
}

.settings-menu-check-spacer {
    width: 14px;
    flex-shrink: 0;
}

.settings-menu-divider {
    height: 1px;
    background: var(--border-color, #e5e5e5);
    margin: 3px 6px;
}

/* Connection status menu (teleported to body, needs unscoped styles) */
.status-menu-title {
    padding: 4px 10px 1px;
    font-size: 10px;
    color: var(--text-muted, #999);
    font-weight: 500;
    letter-spacing: 0.3px;
}

.status-menu-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 10px;
    font-size: 12px;
    white-space: nowrap;
}

.status-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
}

.status-indicator-connected {
    background: var(--color-green, #22c55e);
}

.status-indicator-reconnecting {
    background: var(--color-yellow, #eab308);
    animation: status-pulse 1.2s ease-in-out infinite;
}

.status-indicator-disconnected {
    background: var(--color-red, #ef4444);
}

.status-indicator-disabled {
    background: var(--text-muted, #999);
}

.status-label {
    color: var(--text-secondary, #666);
    font-weight: 500;
}

.status-value {
    color: var(--text-primary, #333);
    margin-left: auto;
}

.status-menu-divider {
    height: 1px;
    background: var(--border-color, #e5e5e5);
    margin: 3px 6px;
}
</style>

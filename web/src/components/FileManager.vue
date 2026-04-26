<template>
  <BottomSheet :open="open" title="文件管理器" @close="$emit('close')">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
      </svg>
      <span class="bs-header-title">文件管理器</span>
      <div v-if="projectPath" class="bs-header-description">
        <span class="bs-header-description-inner" :title="store.state.projectRoot">
          {{ projectPath }}
        </span>
      </div>
      <button class="bs-close" @click.stop="$emit('close')" title="关闭">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    </template>

    <!-- Dir nav -->
    <div id="dirNav" class="dir-nav">
      <div class="dir-toolbar">
        <button class="toolbar-btn" :disabled="!currentDir" @click="$emit('navigateDir', '')" title="项目根目录">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>
        </button>
        <button class="toolbar-btn" :disabled="!currentDir" @click="navigateUp" title="返回">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15,18 9,12 15,6"/></svg>
        </button>
        <button class="toolbar-btn" :class="{ active: sortField === 'name' }" @click="$emit('toggleSort', 'name')" :title="sortField === 'name' ? '按名称排序 (' + (sortDir === 'asc' ? '升序)' : '降序)') : '按名称排序'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M3 12h12M3 18h6"/></svg>
          <svg class="sort-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline v-if="sortField === 'name' && sortDir === 'desc'" points="6,15 12,9 18,15"/><polyline v-else points="6,9 12,15 18,9"/></svg>
        </button>
        <button class="toolbar-btn" :class="{ active: sortField === 'time' }" @click="$emit('toggleSort', 'time')" :title="sortField === 'time' ? '按时间排序 (' + (sortDir === 'asc' ? '升序)' : '降序)') : '按时间排序'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12,6 12,12 16,14"/></svg>
          <svg class="sort-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline v-if="sortField === 'time' && sortDir === 'desc'" points="6,15 12,9 18,15"/><polyline v-else points="6,9 12,15 18,9"/></svg>
        </button>
        <button class="toolbar-btn" :class="{ active: sortField === 'type' }" @click="$emit('toggleSort', 'type')" :title="sortField === 'type' ? '按后缀排序 (' + (sortDir === 'asc' ? '升序)' : '降序)') : '按后缀排序'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14,2 14,8 20,8"/></svg>
          <svg class="sort-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline v-if="sortField === 'type' && sortDir === 'desc'" points="6,15 12,9 18,15"/><polyline v-else points="6,9 12,15 18,9"/></svg>
        </button>
        <button class="toolbar-btn" :class="{ active: !showHidden }" @click="$emit('toggleHidden')" title="隐藏隐藏文件">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="1"/><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
        </button>
        <SearchInput v-model="searchQuery" placeholder="Filter files..." @dblclick="searchQuery = ''" />
      </div>
      <div class="dir-breadcrumb" id="dirBreadcrumb" v-html="dirBreadcrumbHtml" />
    </div>

    <!-- File list -->
    <div class="file-list" id="fileList"
      @click="handleFileClick"
      @contextmenu.prevent="showCtx($event, null)"
      @touchstart="onContainerTouchStart"
      @touchmove="onContainerTouchMove"
      @touchend="onContainerTouchEnd"
      @touchcancel="onContainerTouchEnd"
    >
      <div v-if="filteredEntries.length === 0" class="empty-state">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
        </svg>
        <p>{{ currentDir ? 'This directory is empty.' : 'No supported files found.' }}</p>
      </div>

      <template v-for="entry in filteredEntries" :key="entry.name">
        <!-- Directory -->
        <div v-if="entry.type === 'dir'"
          class="file-item dir-item"
          :data-action="'dir'"
          :data-path="(currentDir ? currentDir + '/' : '') + entry.name"
          @contextmenu.prevent="showCtx($event, entry)">
          <svg class="file-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
          </svg>
          <span class="file-name">{{ entry.name }}</span>
          <svg class="chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9,18 15,12 9,6"/></svg>
          <span class="file-meta">{{ formatDate(entry.modified) }}</span>
        </div>

        <!-- File -->
        <div v-else
          class="file-item"
          :class="{ active: currentFile?.path === (currentDir ? currentDir + '/' : '') + entry.name }"
          :data-action="'file'"
          :data-path="(currentDir ? currentDir + '/' : '') + entry.name"
          @contextmenu.prevent="showCtx($event, entry)">
          <svg v-if="isImage(entry)" class="file-icon" viewBox="0 0 24 24" fill="none" stroke="#a855f7" stroke-width="2">
            <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
            <circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21,15 16,10 5,21"/>
          </svg>
          <svg v-else-if="isAudio(entry)" class="file-icon" viewBox="0 0 24 24" fill="none" stroke="#22c55e" stroke-width="2">
            <path d="M9 18V5l12-2v13"/>
            <circle cx="6" cy="18" r="3"/>
            <circle cx="18" cy="16" r="3"/>
          </svg>
          <svg v-else class="file-icon" viewBox="0 0 24 24" fill="none" :stroke="getFileType(entry.name).color" stroke-width="2">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14,2 14,8 20,8"/>
          </svg>
          <span class="file-name">{{ entry.name }}</span>
          <span class="file-meta">{{ formatSize(entry.size) }} · {{ formatDate(entry.modified) }}</span>
        </div>
      </template>
    </div>

    <!-- Context menu -->
    <Teleport to="body">
      <div v-if="ctxMenu.visible" class="context-menu visible" :style="{ left: ctxMenu.x + 'px', top: ctxMenu.y + 'px' }" @click.stop>
        <div class="context-menu-item" @click.stop="doCopy">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          复制
        </div>
        <div class="context-menu-item" @click.stop="doCut">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="6" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><line x1="20" y1="4" x2="8.12" y2="15.88"/><line x1="14.47" y1="14.48" x2="20" y2="20"/><line x1="8.12" y1="8.12" x2="12" y2="12"/></svg>
          剪切
        </div>
        <div class="context-menu-item" :class="{ disabled: !clipboard.entry }" @click.stop="clipboard.entry && doPaste()">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><rect x="8" y="2" width="8" height="4" rx="1" ry="1"/></svg>
          粘贴
        </div>
        <div class="context-menu-divider" />
        <div class="context-menu-item" @click.stop="doNewFile">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14,2 14,8 20,8"/><line x1="12" y1="18" x2="12" y2="12"/><line x1="9" y1="15" x2="15" y2="15"/></svg>
          新建文件
        </div>
        <div class="context-menu-item" @click.stop="doNewFolder">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" y1="11" x2="12" y2="17"/><line x1="9" y1="14" x2="15" y2="14"/></svg>
          新建文件夹
        </div>
        <div class="context-menu-divider" v-if="ctxMenu.entry" />
        <div class="context-menu-item" v-if="ctxMenu.entry" @click.stop="doRename">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
          重命名
        </div>
        <div class="context-menu-item danger" v-if="ctxMenu.entry" @click.stop="doDelete">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3,6 5,6 21,6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
          删除
        </div>
        <template v-if="ctxMenu.entry && ctxMenu.entry.type === 'dir'">
          <div class="context-menu-divider" />
          <div class="context-menu-item" @click.stop="doOpenAsProject">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><polyline points="14,12 10,12 10,16 14,16"/></svg>
            打开为项目
          </div>
        </template>
      </div>
      <div v-if="ctxMenu.visible" class="ctx-overlay" @click="ctxMenu.visible = false" @touchstart="ctxMenu.visible = false" />
    </Teleport>
  </BottomSheet>
</template>

<script setup>
import { ref, computed, reactive, inject, nextTick, Teleport, watch } from 'vue'
import BottomSheet from './BottomSheet.vue'
import { getFileType, splitPath } from '@/utils/helpers.ts'
import { store } from '@/stores/app.ts'
import SearchInput from './SearchInput.vue'

const toast = inject('toast', null)

const props = defineProps({
    entries: Array,
    currentDir: String,
    currentFile: Object,
    open: Boolean,
    showHidden: Boolean,
    sortField: String,
    sortDir: String,
})

const emit = defineEmits(['close', 'navigateDir', 'selectFile', 'toggleSort', 'toggleHidden', 'rename', 'delete', 'refresh'])

const projectPath = computed(() => {
    const abs = store.state.projectRoot
    const wd = store.state.watchDir
    if (!abs) return ''
    if (!wd) return abs
    const rel = abs.slice(wd.length).replace(/^\//, '')
    return rel || abs
})

function copyProjectPath() {
    const path = projectPath.value
    if (!path) return
    const ta = document.createElement('textarea')
    ta.value = path
    ta.style.cssText = 'position:fixed;opacity:0'
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
    if (toast) toast.show('已复制', { icon: '📋', duration: 1500 })
}

const searchQuery = ref('')

// Clear search when directory changes
watch(() => props.currentDir, () => {
    searchQuery.value = ''
})

const ctxMenu = reactive({ visible: false, x: 0, y: 0, entry: null })

// Container long-press for empty area (mobile)
let containerPressTimer = null
let containerPressMoved = false
let containerPressPos = { x: 0, y: 0 }

function onContainerTouchStart(e) {
    // Only trigger if touch started on empty area (not on a file-item)
    if (e.target.closest('.file-item')) return
    containerPressMoved = false
    const touch = e.touches[0]
    containerPressPos = { x: touch.clientX, y: touch.clientY }
    containerPressTimer = setTimeout(() => {
        if (!containerPressMoved) {
            ctxMenu.x = touch.clientX
            ctxMenu.y = touch.clientY + 10
            ctxMenu.entry = null
            ctxMenu.visible = true
            nextTick(() => clampCtxMenu())
        }
        containerPressTimer = null
    }, 450)
}

function onContainerTouchMove() { containerPressMoved = true }

function onContainerTouchEnd() {
    if (containerPressTimer) {
        clearTimeout(containerPressTimer)
        containerPressTimer = null
    }
}
const clipboard = reactive({ entry: null, isCut: false })

function getDestDir(entry) {
    if (!entry) return props.currentDir
    if (entry.type === 'dir') return entry.path
    const idx = entry.path.lastIndexOf('/')
    return idx > 0 ? entry.path.slice(0, idx) : ''
}

async function doCopy() {
    if (!ctxMenu.entry) return
    clipboard.entry = ctxMenu.entry
    clipboard.isCut = false
    ctxMenu.visible = false
    if (toast) toast.show('已复制', { icon: '📋', duration: 1500 })
}

async function doCut() {
    if (!ctxMenu.entry) return
    clipboard.entry = ctxMenu.entry
    clipboard.isCut = true
    ctxMenu.visible = false
    if (toast) toast.show('已剪切', { icon: '✂️', duration: 1500 })
}

async function doPaste() {
    if (!clipboard.entry) return
    ctxMenu.visible = false
    const destDir = getDestDir(ctxMenu.entry)
    const destPath = (destDir ? destDir + '/' : '') + clipboard.entry.name
    try {
        const api = clipboard.isCut ? '/api/file/move' : '/api/file/copy'
        const resp = await fetch(api, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: clipboard.entry.path, dest: destPath }),
        })
        if (resp.ok) {
            // Only clear clipboard after cut (move), not after copy
            if (clipboard.isCut) {
                clipboard.entry = null
            }
            emit('refresh')
            if (toast) toast.show(clipboard.isCut ? '已移动' : '已复制', { icon: '✅', duration: 1500 })
        } else {
            const err = await resp.json()
            if (toast) toast.show('操作失败: ' + (err.error || ''), { icon: '❌', duration: 2000 })
        }
    } catch (err) {
        if (toast) toast.show('操作失败', { icon: '❌', duration: 2000 })
    }
}

async function doNewFile() {
    ctxMenu.visible = false
    const name = prompt('输入文件名：')
    if (!name || !name.trim()) return
    const dir = getDestDir(ctxMenu.entry)
    try {
        const resp = await fetch('/api/file/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: dir, name: name.trim() }),
        })
        if (resp.ok) {
            emit('refresh')
            if (toast) toast.show('文件已创建', { icon: '📄', duration: 1500 })
        } else {
            const err = await resp.json()
            if (toast) toast.show('创建失败: ' + (err.error || ''), { icon: '❌', duration: 2000 })
        }
    } catch (err) {
        if (toast) toast.show('创建失败', { icon: '❌', duration: 2000 })
    }
}

async function doNewFolder() {
    ctxMenu.visible = false
    const name = prompt('输入文件夹名：')
    if (!name || !name.trim()) return
    const dir = getDestDir(ctxMenu.entry)
    try {
        const resp = await fetch('/api/dir/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: dir, name: name.trim() }),
        })
        if (resp.ok) {
            emit('refresh')
            if (toast) toast.show('文件夹已创建', { icon: '📁', duration: 1500 })
        } else {
            const err = await resp.json()
            if (toast) toast.show('创建失败: ' + (err.error || ''), { icon: '❌', duration: 2000 })
        }
    } catch (err) {
        if (toast) toast.show('创建失败', { icon: '❌', duration: 2000 })
    }
}

const filteredEntries = computed(() => {
    let entries = [...props.entries]
    if (!props.showHidden) entries = entries.filter(e => !e.name.startsWith('.'))
    const q = searchQuery.value.toLowerCase()
    if (q) entries = entries.filter(e => e.name.toLowerCase().includes(q))
    if (props.sortField) {
        entries = entries.sort((a, b) => {
            if (props.sortField !== 'type') {
                if (a.type === 'dir' && b.type !== 'dir') return -1
                if (a.type !== 'dir' && b.type === 'dir') return 1
            }
            let cmp = 0
            if (props.sortField === 'name') cmp = a.name.localeCompare(b.name)
            else if (props.sortField === 'time') cmp = new Date(a.modified) - new Date(b.modified)
            else if (props.sortField === 'type') {
                const extA = a.name.includes('.') ? a.name.split('.').pop().toLowerCase() : ''
                const extB = b.name.includes('.') ? b.name.split('.').pop().toLowerCase() : ''
                cmp = extA < extB ? -1 : extA > extB ? 1 : 0
                if (cmp === 0) cmp = a.name < b.name ? -1 : a.name > b.name ? 1 : 0
            }
            return props.sortDir === 'asc' ? cmp : -cmp
        })
    }
    return entries
})

const dirBreadcrumbHtml = computed(() => {
    if (!props.currentDir || props.currentDir === '.') return ''
    const parts = splitPath(props.currentDir)
    let crumbPath = ''
    return parts.map((part, i) => {
        crumbPath = i === 0 ? part : parts.slice(0, i + 1).join('/')
        const isLast = i === parts.length - 1
        return `<span class="crumb ${isLast ? 'current' : ''}" ${!isLast ? `onclick="window.__navigateDir('${crumbPath.replace(/'/g, "\\'")}')"` : ''}>${part}</span>${!isLast ? '<span class="crumb-sep">›</span>' : ''}`
    }).join('')
})

function navigateUp() {
    if (!props.currentDir) return
    const parts = splitPath(props.currentDir)
    parts.pop()
    emit('navigateDir', parts.join('/'))
}

function handleFileClick(e) {
    const item = e.target.closest('.file-item')
    if (!item) return
    const action = item.dataset.action
    const path = item.dataset.path
    if (action === 'dir') {
        emit('navigateDir', path)
    } else {
        emit('selectFile', path)
        emit('close')
    }
}

function isImage(entry) {
    return entry.type === 'image' || getFileType(entry.name).isImage
}

function isAudio(entry) {
    return getFileType(entry.name).isAudio
}

function formatDate(modified) {
    if (!modified) return ''
    const d = new Date(modified)
    const isToday = d.toDateString() === new Date().toDateString()
    return isToday
        ? d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
        : `${(d.getMonth() + 1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')}`
}

function formatSize(size) {
    if (size == null) return ''
    if (size < 1024) return size + ' B'
    if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' K'
    return (size / (1024 * 1024)).toFixed(1) + ' M'
}

function showCtx(e, entry) {
    const path = (props.currentDir ? props.currentDir + '/' : '') + entry.name
    ctxMenu.x = e.clientX
    ctxMenu.y = e.clientY
    ctxMenu.entry = { ...entry, path }
    ctxMenu.visible = true
    nextTick(() => clampCtxMenu())
}

// Clamp menu position to stay within viewport on all sides
function clampCtxMenu() {
    const menu = document.querySelector('.context-menu.visible')
    if (!menu) return
    const w = menu.offsetWidth
    const h = menu.offsetHeight
    const vw = window.innerWidth
    const vh = window.innerHeight
    // Add small padding from edges
    const pad = 8
    ctxMenu.x = Math.max(pad, Math.min(ctxMenu.x, vw - w - pad))
    ctxMenu.y = Math.max(pad, Math.min(ctxMenu.y, vh - h - pad))
}

function doOpenAsProject() {
    if (!ctxMenu.entry || ctxMenu.entry.type !== 'dir') return
    ctxMenu.visible = false
    const absPath = store.state.projectRoot + '/' + ctxMenu.entry.path
    fetch('/api/project', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: absPath }),
    }).then(resp => {
        if (resp.ok) {
            window.location.reload()
        } else {
            resp.text().then(text => {
                let msg = text
                try { msg = JSON.parse(text).error || msg } catch (_) {}
                if (toast) toast.show('切换项目失败: ' + msg, { icon: '❌', duration: 2000 })
            })
        }
    }).catch(() => {
        if (toast) toast.show('切换项目失败', { icon: '❌', duration: 2000 })
    })
}

function doRename() {
    if (!ctxMenu.entry) return
    const newName = prompt('输入新名称：', ctxMenu.entry.name)
    if (!newName || newName === ctxMenu.entry.name) { ctxMenu.visible = false; return }
    emit('rename', { path: ctxMenu.entry.path, name: newName })
    ctxMenu.visible = false
}

function doDelete() {
    if (!ctxMenu.entry) return
    ctxMenu.visible = false
    emit('delete', ctxMenu.entry.path)
}

// Expose navigateDir globally for inline onclick handlers
if (typeof window !== 'undefined') {
    window.__navigateDir = (path) => emit('navigateDir', path)
}
</script>

<style scoped>
/* ── File manager specific ── */

.fm-header-row {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    min-width: 0;
}

.fm-project-path {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
    color: var(--text-muted, #999);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
}

.fm-copy-icon {
    flex-shrink: 0;
    cursor: pointer;
    color: var(--text-muted, #999);
    transition: color 0.15s;
}
.fm-copy-icon:hover {
    color: var(--accent-color, #4a90d9);
}

.dir-nav {
    padding: 6px 8px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-height: 32px;
    border-bottom: 1px solid var(--border-color, #e5e5e5);
    background: var(--bg-tertiary, #f5f5f5);
    flex-shrink: 0;
}

.dir-toolbar {
    display: flex;
    align-items: center;
    gap: 4px;
}

.dir-toolbar :deep(.search-pill) {
    flex: 1;
    min-width: 0;
    max-width: 180px;
    margin-left: auto;
}

/* ── File list area ── */
.file-list {
    flex: 1;
    overflow-y: auto;
    padding: 4px 6px;
}

/* Unified toolbar button */
.toolbar-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 3px;
    width: 28px;
    height: 28px;
    padding: 0;
    border: none;
    border-radius: 50%;
    background: var(--bg-tertiary, #f0f0f0);
    color: var(--text-secondary, #666);
    cursor: pointer;
    transition: all 0.15s;
    flex-shrink: 0;
}

.toolbar-btn:hover {
    background: var(--bg-secondary, #e0e0e0);
    color: var(--accent-color, #4a90d9);
}

.toolbar-btn:disabled {
    opacity: 0.35;
    cursor: not-allowed;
}

.toolbar-btn:disabled:hover {
    background: transparent;
    color: var(--text-secondary, #666);
}

.toolbar-btn.active {
    background: var(--accent-color, #4a90d9);
    color: #fff;
}

.toolbar-btn.active:hover {
    background: var(--accent-hover, #3a80c9);
    color: #fff;
}

.toolbar-btn svg {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
}

.toolbar-btn .sort-arrow {
    width: 8px;
    height: 8px;
    opacity: 0.4;
}

.toolbar-btn.active .sort-arrow {
    opacity: 1;
}

.dir-breadcrumb {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 4px;
    overflow-x: auto;
    font-size: 13px;
    color: var(--text-muted, #999);
    scrollbar-width: none;
}
.dir-breadcrumb::-webkit-scrollbar {
    display: none;
}

.dir-breadcrumb :deep(.crumb) {
    padding: 3px 6px;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
    transition: background 0.15s;
}

.dir-breadcrumb :deep(.crumb:hover) {
    background: var(--bg-tertiary, #f0f0f0);
    color: var(--accent-color, #4a90d9);
}

.dir-breadcrumb :deep(.crumb.current) {
    font-weight: 600;
    color: var(--text-primary, #1a1a1a);
    cursor: default;
}

.dir-breadcrumb :deep(.crumb.current:hover) {
    background: none;
    color: var(--text-primary, #1a1a1a);
}

.dir-breadcrumb :deep(.crumb-sep) {
    color: var(--text-muted, #999);
    font-size: 11px;
}

/* ── File Items ── */
.file-item {
    display: flex;
    align-items: center;
    padding: 6px 12px;
    border-radius: var(--radius-sm, 6px);
    cursor: pointer;
    transition: background 0.15s;
    gap: 10px;
    color: var(--text-secondary, #666);
    font-size: 14px;
    user-select: none;
    -webkit-user-select: none;
}

.file-item:hover {
    background: var(--bg-tertiary, #f0f0f0);
}

.file-item.active {
    background: var(--accent-color, #4a90d9);
    color: white;
}

.file-item.dir-item {
    color: var(--text-primary, #1a1a1a);
    font-weight: 500;
}

.file-item.dir-item .file-icon {
    color: var(--accent-color, #4a90d9);
}

.file-item.dir-item:hover {
    background: var(--bg-tertiary, #f0f0f0);
}

.file-item.dir-item .chevron {
    margin-left: auto;
    color: var(--text-muted, #999);
    transition: transform 0.2s;
}

.file-item.dir-item:hover .chevron {
    transform: translateX(2px);
    color: var(--accent-color, #4a90d9);
}

.file-icon {
    flex-shrink: 0;
    width: 18px;
    height: 18px;
}

.file-name {
    flex: 1;
    overflow-x: auto;
    white-space: nowrap;
    scrollbar-width: none;
}
.file-name::-webkit-scrollbar {
    display: none;
}

.file-meta {
    font-size: 11px;
    color: var(--text-muted, #999);
    flex-shrink: 0;
}

.file-item.active .file-meta {
    color: rgba(255,255,255,0.7);
}

/* Empty State */
.empty-state {
    text-align: center;
    padding: 40px 20px;
    color: var(--text-muted, #999);
}

.empty-state svg {
    width: 48px;
    height: 48px;
    margin-bottom: 12px;
    opacity: 0.5;
}

</style>

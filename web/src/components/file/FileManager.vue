<template>
  <BottomSheet :open="open" :title="t('file.manager')" @close="$emit('close')">
    <template #header>
      <Folder :size="16" class="bs-header-icon" />
      <span class="bs-header-title">{{ t('file.manager') }}</span>
    </template>

    <!-- Dir nav -->
    <div id="dirNav" class="dir-nav">
      <div class="dir-toolbar">
        <button class="toolbar-btn" :class="{ 'sort-active': sortField === 'name' }" @click="$emit('toggleSort', 'name')" :title="sortField === 'name' ? t('file.sortByName') + ' (' + (sortDir === 'asc' ? t('file.sortAsc') : t('file.sortDesc') + ' · ' + t('file.sortClickToClear')) + ')' : t('file.sortByName') + ' (' + t('file.sortDefault') + ')'">
          <ArrowDownAz :size="14" />
          <ChevronDown v-if="sortField === 'name' && sortDir === 'desc'" :size="8" :stroke-width="3" class="sort-arrow" />
          <ChevronUp v-else-if="sortField === 'name'" :size="8" :stroke-width="3" class="sort-arrow" />
          <ArrowUpDown v-else-if="!sortField" :size="8" :stroke-width="3" class="sort-arrow sort-arrow-default" />
        </button>
        <button class="toolbar-btn" :class="{ 'sort-active': sortField === 'time' }" @click="$emit('toggleSort', 'time')" :title="sortField === 'time' ? t('file.sortByTime') + ' (' + (sortDir === 'asc' ? t('file.sortAsc') : t('file.sortDesc') + ' · ' + t('file.sortClickToClear')) + ')' : t('file.sortByTime') + ' (' + t('file.sortDefault') + ')'">
          <Clock :size="14" />
          <ChevronDown v-if="sortField === 'time' && sortDir === 'desc'" :size="8" :stroke-width="3" class="sort-arrow" />
          <ChevronUp v-else-if="sortField === 'time'" :size="8" :stroke-width="3" class="sort-arrow" />
          <ArrowUpDown v-else-if="!sortField" :size="8" :stroke-width="3" class="sort-arrow sort-arrow-default" />
        </button>
        <button class="toolbar-btn" :class="{ 'sort-active': sortField === 'type' }" @click="$emit('toggleSort', 'type')" :title="sortField === 'type' ? t('file.sortByType') + ' (' + (sortDir === 'asc' ? t('file.sortAsc') : t('file.sortDesc') + ' · ' + t('file.sortClickToClear')) + ')' : t('file.sortByType') + ' (' + t('file.sortDefault') + ')'">
          <FileText :size="14" />
          <ChevronDown v-if="sortField === 'type' && sortDir === 'desc'" :size="8" :stroke-width="3" class="sort-arrow" />
          <ChevronUp v-else-if="sortField === 'type'" :size="8" :stroke-width="3" class="sort-arrow" />
          <ArrowUpDown v-else-if="!sortField" :size="8" :stroke-width="3" class="sort-arrow sort-arrow-default" />
        </button>
        <button class="toolbar-btn" @click="$emit('toggleHidden')" :title="showHidden ? t('file.hideHiddenFiles') : t('file.showHiddenFiles')">
          <EyeOff v-if="!showHidden" :size="14" />
          <Eye v-else :size="14" />
        </button>
        <button class="toolbar-btn" :disabled="!currentFile?.path" @click="syncToCurrentFile" :title="t('file.syncToCurrentDir')">
          <ArrowRightLeft :size="14" />
        </button>
        <button class="toolbar-btn" @click="$emit('refresh')" :title="t('nav.refresh')">
          <RotateCw :size="14" />
        </button>
        <!-- Expandable search -->
        <button v-if="!searchExpanded" class="toolbar-btn" @click="expandSearch" :title="t('search.defaultPlaceholder')">
          <Search :size="14" />
        </button>
        <div v-else class="search-expanded">
          <SearchInput ref="searchInputRef" v-model="searchQuery" :placeholder="t('search.defaultPlaceholder')" @blur="onSearchBlur" @dblclick="searchQuery = ''" />
        </div>
      </div>
      <DirBreadcrumb :path="currentDir" @navigate="$emit('navigateDir', $event)" />
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
      <div v-if="dirLoading" class="dir-loading-overlay">
        <Loader :size="24" class="dir-loading-spinner" />
        <span>{{ t('common.loading') }}</span>
      </div>
      <template v-else>
      <div v-if="filteredEntries.length === 0" class="empty-state">
        <Folder :size="48" />
        <p>{{ currentDir ? t('file.emptyDir') : t('file.noFiles') }}</p>
      </div>

      <template v-for="entry in visibleEntries" :key="entry.name">
        <!-- Directory -->
        <div v-if="entry.type === 'dir'"
          class="file-item dir-item"
          :data-action="'dir'"
          :data-path="(currentDir ? currentDir + '/' : '') + entry.name"
          @contextmenu.prevent="showCtx($event, entry)">
          <Folder class="file-icon" :size="16" />
          <span class="file-name">{{ entry.name }}</span>
          <ChevronRight :size="14" class="chevron" />
          <span class="file-meta">{{ formatDate(entry.modified) }}</span>
        </div>

        <!-- File -->
        <div v-else
          class="file-item"
          :class="{ active: currentFile?.path === (currentDir ? currentDir + '/' : '') + entry.name }"
          :data-action="'file'"
          :data-path="(currentDir ? currentDir + '/' : '') + entry.name"
          @contextmenu.prevent="showCtx($event, entry)">
          <FileImage v-if="isImage(entry)" class="file-icon" :size="16" color="#a855f7" />
          <FileMusic v-else-if="isAudio(entry)" class="file-icon" :size="16" color="#22c55e" />
          <FileText v-else class="file-icon" :size="16" :color="getFileType(entry.name).color" />
          <span class="file-name">{{ entry.name }}</span>
          <span class="file-meta">{{ formatSize(entry.size) }} · {{ formatDate(entry.modified) }}</span>
        </div>
      </template>
      <div v-if="hasMoreEntries" class="truncate-hint">
        {{ t('file.truncateHint', { max: MAX_VISIBLE_ENTRIES, total: filteredEntries.length }) }}
      </div>
      </template>
    </div>

    <!-- Context menu -->
    <Teleport to="body">
      <div v-if="ctxMenu.visible" class="context-menu visible" :style="{ left: ctxMenu.x + 'px', top: ctxMenu.y + 'px' }" @click.stop>
        <div class="context-menu-item" @click.stop="doCopy">
          <Copy :size="14" />
          {{ t('file.context.copy') }}
        </div>
        <div class="context-menu-item" @click.stop="doCut">
          <Scissors :size="14" />
          {{ t('file.context.cut') }}
        </div>
        <div class="context-menu-item" :class="{ disabled: !clipboard.entry }" @click.stop="clipboard.entry && doPaste()">
          <ClipboardPaste :size="14" />
          {{ t('file.context.paste') }}
        </div>
        <div class="context-menu-divider" />
        <div class="context-menu-item" @click.stop="doNewFile">
          <FilePlus :size="14" />
          {{ t('file.context.newFile') }}
        </div>
        <div class="context-menu-item" @click.stop="doNewFolder">
          <FolderPlus :size="14" />
          {{ t('file.context.newFolder') }}
        </div>
        <div class="context-menu-divider" v-if="ctxMenu.entry" />
        <div class="context-menu-item" v-if="ctxMenu.entry" @click.stop="doRename">
          <Pencil :size="14" />
          {{ t('common.rename') }}
        </div>
        <div class="context-menu-item" v-if="ctxMenu.entry && ctxMenu.entry.type !== 'dir'" @click.stop="doDownload">
          <Download :size="14" />
          {{ t('common.download') }}
        </div>
        <div class="context-menu-item danger" v-if="ctxMenu.entry" @click.stop="doDelete">
          <Trash2 :size="14" />
          {{ t('common.delete') }}
        </div>
        <template v-if="ctxMenu.entry && ctxMenu.entry.type === 'dir'">
          <div class="context-menu-divider" />
          <div class="context-menu-item" @click.stop="doOpenAsProject">
            <FolderOpen :size="14" />
            {{ t('file.context.openAsProject') }}
          </div>
          <div class="context-menu-item" @click.stop="doOpenTerminal">
            <TerminalIcon :size="14" />
            {{ t('file.context.openTerminal') }}
          </div>
        </template>
      </div>
      <div v-if="ctxMenu.visible" class="ctx-overlay" @click="ctxMenu.visible = false" @touchstart="ctxMenu.visible = false" />
    </Teleport>
  </BottomSheet>
</template>

<script setup>
import { ref, computed, reactive, inject, nextTick, Teleport, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Folder, ArrowDownAz, ArrowUpDown, ChevronDown, ChevronUp, Clock, FileText, Eye, EyeOff, ArrowRightLeft, Loader, FileImage, FileMusic, ChevronRight, Copy, Scissors, ClipboardPaste, FilePlus, FolderPlus, Pencil, Download, Trash2, FolderOpen, RotateCw, Search, Terminal as TerminalIcon } from 'lucide-vue-next'
import BottomSheet from '@/components/common/BottomSheet.vue'
import { getFileType } from '@/utils/fileType.ts'
import { dirName } from '@/utils/path.ts'
import { store } from '@/stores/app.ts'
import { useAppMode } from '@/composables/useAppMode.ts'
import { useDialog } from '@/composables/useDialog.ts'
import SearchInput from '@/components/common/SearchInput.vue'
import DirBreadcrumb from './DirBreadcrumb.vue'

const toast = inject('toast', null)
const { isAppMode } = useAppMode()
const { t } = useI18n()
const dialog = useDialog()

const props = defineProps({
    entries: Array,
    currentDir: String,
    currentFile: Object,
    open: Boolean,
    showHidden: Boolean,
    sortField: String,
    sortDir: String,
    dirLoading: Boolean,
})

const emit = defineEmits(['close', 'navigateDir', 'selectFile', 'toggleSort', 'toggleHidden', 'rename', 'delete', 'refresh', 'openTerminal'])


const searchQuery = ref('')
const searchExpanded = ref(false)
const searchInputRef = ref(null)

function expandSearch() {
    searchExpanded.value = true
    nextTick(() => {
        searchInputRef.value?.focus()
    })
}

function onSearchBlur() {
    // Delay to allow clear button click to register
    setTimeout(() => {
        if (!searchQuery.value) {
            searchExpanded.value = false
        }
    }, 150)
}

// Sync button: navigate to the directory of the currently opened file
const isInSync = computed(() => {
    if (!props.currentFile?.path) return false
    return dirName(props.currentFile.path) === props.currentDir
})

function syncToCurrentFile() {
    if (!props.currentFile?.path) return
    const targetDir = dirName(props.currentFile.path)
    if (targetDir === props.currentDir) {
        if (toast) toast.show(t('file.alreadyInDir'), { icon: '📍', type: 'success', duration: 1500 })
        return
    }
    emit('navigateDir', targetDir)
}

// Clear search when directory changes
watch(() => props.currentDir, () => {
    searchQuery.value = ''
    searchExpanded.value = false
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
    if (toast) toast.show(t('common.copied'), { icon: '📋', type: 'success', duration: 1500 })
}

async function doCut() {
    if (!ctxMenu.entry) return
    clipboard.entry = ctxMenu.entry
    clipboard.isCut = true
    ctxMenu.visible = false
    if (toast) toast.show(t('file.toast.cutDone'), { icon: '✂️', type: 'success', duration: 1500 })
}

async function doPaste() {
    if (!clipboard.entry) return
    ctxMenu.visible = false
    const destDir = getDestDir(ctxMenu.entry)
    const destPath = (destDir ? destDir + '/' : '') + clipboard.entry.name
    const api = clipboard.isCut ? '/api/file/move' : '/api/file/copy'
    try {
        let finalDest = destPath
        let resp = await fetch(api, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: clipboard.entry.path, dest: finalDest }),
        })
        // Name conflict: prompt user for a new name
        if (resp.status === 409) {
            const newName = await dialog.prompt(t('file.prompt.pasteNewName', { name: clipboard.entry.name }), { value: clipboard.entry.name })
            if (!newName || !newName.trim()) return
            finalDest = (destDir ? destDir + '/' : '') + newName.trim()
            resp = await fetch(api, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path: clipboard.entry.path, dest: finalDest }),
            })
        }
        if (resp.ok) {
            if (clipboard.isCut) {
                clipboard.entry = null
            }
            emit('refresh')
            if (toast) toast.show(clipboard.isCut ? t('file.toast.moved') : t('common.copied'), { icon: '✅', type: 'success', duration: 1500 })
        } else {
            const err = await resp.json()
            if (toast) toast.show(t('file.toast.operationFailedDetail', { error: err.error || '' }), { icon: '❌', type: 'error', duration: 2000 })
        }
    } catch (err) {
        if (toast) toast.show(t('common.operationFailed'), { icon: '❌', type: 'error', duration: 2000 })
    }
}

async function doNewFile() {
    ctxMenu.visible = false
    const name = await dialog.prompt(t('file.prompt.fileName'))
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
            if (toast) toast.show(t('file.toast.fileCreated'), { icon: '📄', type: 'success', duration: 1500 })
        } else {
            const err = await resp.json()
            if (toast) toast.show(t('file.toast.createFailedDetail', { error: err.error || '' }), { icon: '❌', type: 'error', duration: 2000 })
        }
    } catch (err) {
        if (toast) toast.show(t('file.toast.createFailed'), { icon: '❌', type: 'error', duration: 2000 })
    }
}

async function doNewFolder() {
    ctxMenu.visible = false
    const name = await dialog.prompt(t('file.prompt.folderName'))
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
            if (toast) toast.show(t('file.toast.folderCreated'), { icon: '📁', type: 'success', duration: 1500 })
        } else {
            const err = await resp.json()
            if (toast) toast.show(t('file.toast.createFailedDetail', { error: err.error || '' }), { icon: '❌', type: 'error', duration: 2000 })
        }
    } catch (err) {
        if (toast) toast.show(t('file.toast.createFailed'), { icon: '❌', type: 'error', duration: 2000 })
    }
}

const MAX_VISIBLE_ENTRIES = 1000

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

const hasMoreEntries = computed(() => filteredEntries.value.length > MAX_VISIBLE_ENTRIES)
const visibleEntries = computed(() => filteredEntries.value.slice(0, MAX_VISIBLE_ENTRIES))

function handleFileClick(e) {
    if (props.dirLoading) return
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
                if (toast) toast.show(t('file.toast.switchProjectFailed', { error: msg }), { icon: '❌', type: 'error', duration: 2000 })
            })
        }
    }).catch(() => {
        if (toast) toast.show(t('file.toast.switchProjectFailedShort'), { icon: '❌', type: 'error', duration: 2000 })
    })
}

function doOpenTerminal() {
    if (!ctxMenu.entry || ctxMenu.entry.type !== 'dir') return
    ctxMenu.visible = false
    // Navigate to the directory first so computeCwd() picks it up
    store.state.currentDir = ctxMenu.entry.path
    emit('openTerminal', ctxMenu.entry.path)
}

async function doRename() {
    if (!ctxMenu.entry) return
    const newName = await dialog.prompt(t('file.prompt.newName'), { value: ctxMenu.entry.name })
    if (!newName || newName === ctxMenu.entry.name) { ctxMenu.visible = false; return }
    emit('rename', { path: ctxMenu.entry.path, name: newName })
    ctxMenu.visible = false
}

function doDownload() {
    if (!ctxMenu.entry) return
    ctxMenu.visible = false
    const path = ctxMenu.entry.path
    const native = window.AndroidNative
    if (isAppMode.value && native && native.downloadFile) {
        native.downloadFile(path)
    } else {
        const a = document.createElement('a')
        a.href = '/api/local-file/' + encodeURIComponent(path)
        a.download = ctxMenu.entry.name
        a.click()
    }
}

function doDelete() {
    if (!ctxMenu.entry) return
    ctxMenu.visible = false
    emit('delete', ctxMenu.entry.path)
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
}

.search-expanded {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
}

.search-expanded :deep(.search-pill) {
    width: 100%;
    max-width: none;
}

.dir-nav :deep(.dir-breadcrumb) {
    padding: 0 6px;
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

.toolbar-btn svg {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
}

.toolbar-btn .sort-arrow {
    width: 8px;
    height: 8px;
    opacity: 1;
    color: var(--accent-color, #4a90d9);
}

.toolbar-btn .sort-arrow-default {
    color: var(--text-tertiary, #999);
}

/* ── File Items ── */

.file-item + .file-item {
    border-top: 1px solid var(--border-color, #e5e5e5);
}

.file-item {
    display: flex;
    align-items: center;
    padding: 4px 16px;
    border-radius: 0;
    min-height: 36px;
    cursor: pointer;
    transition: background 0.15s;
    gap: 8px;
    color: var(--text-secondary, #666);
    font-size: 13px;
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
    width: 16px;
    height: 16px;
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

/* Truncate hint */
.truncate-hint {
    text-align: center;
    padding: 10px 16px;
    font-size: 12px;
    color: var(--text-muted, #999);
    background: var(--bg-tertiary, #f5f5f5);
    border-top: 1px solid var(--border-color, #e5e5e5);
    flex-shrink: 0;
}

/* Loading overlay */
.dir-loading-overlay {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 40px 20px;
    color: var(--text-muted, #999);
    font-size: 13px;
}

.dir-loading-spinner {
    width: 24px;
    height: 24px;
    animation: dir-spin 1s linear infinite;
}

@keyframes dir-spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
}

</style>

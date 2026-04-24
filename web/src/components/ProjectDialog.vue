<template>
  <Teleport to="body">
    <div v-show="open" class="modal-overlay" @click.self="$emit('close')">
      <div class="modal-dialog">
        <div class="modal-header">
          <span class="modal-title">选择项目</span>
          <button class="modal-close" @click="$emit('close')" title="关闭">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <!-- Tabs row -->
          <div class="dialog-tabs-row">
            <div class="dialog-tabs">
              <button class="dialog-tab" :class="{ active: tab === 'recent' }" @click="tab = 'recent'">最近</button>
              <button class="dialog-tab" :class="{ active: tab === 'browse' }" @click="tab = 'browse'">浏览</button>
            </div>
          </div>

          <!-- Browse nav - v-show to prevent layout shift -->
          <div class="dialog-nav" v-show="tab === 'browse'">
            <div class="dialog-toolbar-row">
              <button class="toolbar-btn" :disabled="browsePath === '/'" @click="browseNavigate('/')" title="返回根目录">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
              </button>
              <button class="toolbar-btn" :disabled="browsePathParts.length === 0" @click="navigateUp" title="返回上级">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><polyline points="15,18 9,12 15,6"/></svg>
              </button>
              <button class="toolbar-btn" @click="doNewFolder" title="新建文件夹">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/><line x1="12" y1="11" x2="12" y2="17"/><line x1="9" y1="14" x2="15" y2="14"/></svg>
              </button>
              <button class="toolbar-btn" :class="{ active: !showHidden }" @click="showHidden = !showHidden" title="隐藏隐藏文件">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><circle cx="12" cy="12" r="1"/><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
              </button>
            </div>
            <div class="dialog-breadcrumb">
              <span @click="browseNavigate('/')">根目录</span>
              <span v-for="(_, i) in browsePathParts" :key="i" @click="browseNavigate('/' + browsePathParts.slice(0, i + 1).join('/'))">
                / {{ browsePathParts[i] }}
              </span>
            </div>
          </div>

          <!-- Content -->
          <div class="dialog-content">
            <div v-if="loading" class="dialog-loading">加载中...</div>
            <div v-else-if="displayItems.length === 0" class="dialog-empty">{{ tab === 'recent' ? '暂无最近项目' : (searchQuery ? '没有匹配的目录' : '空目录') }}</div>
            <div
              v-else
              v-for="item in displayItems"
              :key="item.path"
              class="dialog-item"
              :class="{ selected: selectedPath === item.path }"
              @click="selectItem(item)"
              @dblclick="tab === 'browse' && enterDir(item)"
            >
              <span class="item-icon">📁</span>
              <span class="item-name">{{ item.displayPath || item.name || item.path }}</span>
              <button v-if="tab === 'browse'" class="item-action-btn" @click.stop="toggleItemMenu(item, $event)" title="操作">
                <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16"><circle cx="12" cy="5" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="12" cy="19" r="2"/></svg>
              </button>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <input type="text" class="search-input" v-model="searchQuery" placeholder="搜索..." @dblclick="searchQuery = ''" />
          <button class="confirm-btn" :disabled="tab === 'recent' && !selectedPath" @click="confirm">
            <span>确定</span>
          </button>
        </div>

        <!-- Item action popup -->
        <div v-if="itemMenu.visible" class="item-menu-popup" :style="{ left: itemMenu.x + 'px', top: itemMenu.y + 'px' }" @click.stop>
          <div class="item-menu-popup-item" @click.stop="doRename">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
            重命名
          </div>
          <div class="item-menu-popup-item danger" @click.stop="doDelete">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3,6 5,6 21,6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
            删除
          </div>
        </div>
        <div v-if="itemMenu.visible" class="ctx-overlay" @click="itemMenu.visible = false" @touchstart="itemMenu.visible = false" />
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, inject, nextTick } from 'vue'

const props = defineProps({
  open: Boolean,
})
const emit = defineEmits(['close'])
const toast = inject('toast', null)

const tab = ref('recent')
const loading = ref(false)
const selectedPath = ref('')
const searchQuery = ref('')
const showHidden = ref(false)

// Recent projects
const recentItems = ref([])

// Browse state
const browsePath = ref('/')
const itemMenu = reactive({ visible: false, x: 0, y: 0, entry: null })
const browseItems = ref([])
let watchBase = ''
let currentBrowseAbs = ''

function toRelative(absPath) {
    if (!watchBase) return absPath
    const rel = absPath.slice(watchBase.length).replace(/^\//, '')
    return rel || '/'
}

// Load browse when tab switches to browse
watch(tab, (newTab) => {
    itemMenu.visible = false
    if (newTab === 'browse' && browseItems.value.length === 0) {
        loadBrowse()
    }
})

// Reload data when dialog opens
watch(() => props.open, (isOpen) => {
    if (isOpen) {
        selectedPath.value = ''
        searchQuery.value = ''
        itemMenu.visible = false

        if (tab.value === 'recent') {
            loadRecentProjects()
        } else {
            loadBrowse()
        }
    }
})

async function loadRecentProjects() {
    loading.value = true
    try {
        const wdResp = await fetch('/api/watch-dir')
        if (wdResp.ok) {
            const wd = await wdResp.json()
            watchBase = wd.watchDir || ''
        }
        const resp = await fetch('/api/recent-projects')
        recentItems.value = await resp.json()
    } catch (_) {
        recentItems.value = []
    } finally {
        loading.value = false
    }
}

const browsePathParts = computed(() => browsePath.value.split('/').filter(Boolean))

const displayItems = computed(() => {
    if (tab.value === 'recent') {
        const q = searchQuery.value.trim().toLowerCase()
        const items = q ? recentItems.value.filter(p => p.toLowerCase().includes(q)) : recentItems.value
        return items.map(p => {
            const rel = toRelative(p)
            const name = rel.split('/').pop() || rel
            return { name, path: p, displayPath: rel }
        })
    }
    const q = searchQuery.value.trim().toLowerCase()
    let dirs = browseItems.value.filter(d => !q || d.name.toLowerCase().includes(q))
    if (!showHidden.value) dirs = dirs.filter(d => !d.name.startsWith('.'))
    return dirs.map(d => {
        const name = d.name
        const path = browsePath.value === '/' ? name : browsePath.value + '/' + name
        return { name, path }
    })
})

function selectItem(item) {
    selectedPath.value = item.path
}

function enterDir(item) {
    browseNavigate(item.path)
    selectedPath.value = item.path
}

function browseNavigate(path) {
    browsePath.value = path
    selectedPath.value = path
    loadBrowse()
}

function navigateUp() {
    const parts = browsePathParts.value
    if (parts.length === 0) return
    if (parts.length === 1) {
        browseNavigate('/')
    } else {
        browseNavigate(parts.slice(0, -1).join('/'))
    }
}

async function loadBrowse() {
    loading.value = true
    searchQuery.value = ''
    try {
        const resp = await fetch('/api/projects?path=' + encodeURIComponent(browsePath.value === '/' ? '' : browsePath.value))
        const data = await resp.json()
        if (!watchBase) {
            watchBase = data.path || browsePath.value
        }
        currentBrowseAbs = data.path || browsePath.value
        browsePath.value = toRelative(currentBrowseAbs)
        browseItems.value = (data.items || []).filter(i => i.type === 'dir')
    } catch (_) {
        browseItems.value = []
    } finally {
        loading.value = false
    }
}

function toggleItemMenu(item, e) {
    if (itemMenu.visible && itemMenu.entry?.path === item.path) {
        itemMenu.visible = false
        return
    }
    const rect = e.currentTarget.getBoundingClientRect()
    itemMenu.x = rect.right - 140
    itemMenu.y = rect.bottom + 4
    itemMenu.entry = item
    itemMenu.visible = true
    nextTick(() => clampItemMenu())
}

function clampItemMenu() {
    const menu = document.querySelector('.item-menu-popup')
    if (!menu) return
    const w = menu.offsetWidth
    const h = menu.offsetHeight
    const vw = window.innerWidth
    const vh = window.innerHeight
    const pad = 8
    itemMenu.x = Math.max(pad, Math.min(itemMenu.x, vw - w - pad))
    itemMenu.y = Math.max(pad, Math.min(itemMenu.y, vh - h - pad))
}

async function doNewFolder() {
    if (tab.value !== 'browse') return
    itemMenu.visible = false
    const name = prompt('输入文件夹名：')
    if (!name || !name.trim()) return
    const dir = browsePath.value
    try {
        const resp = await fetch('/api/projects', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: dir, name: name.trim() })
        })
        if (resp.ok) await loadBrowse()
        else alert('创建失败')
    } catch (_) { alert('创建失败') }
}

async function doRename() {
    if (!itemMenu.entry) return
    itemMenu.visible = false
    const newName = prompt('输入新名称：', itemMenu.entry.name)
    if (!newName || newName === itemMenu.entry.name) return
    try {
        const resp = await fetch('/api/file/rename', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: itemMenu.entry.name, name: newName, basePath: currentBrowseAbs })
        })
        if (resp.ok) await loadBrowse()
        else {
            const err = await resp.json()
            alert('重命名失败: ' + (err.error || ''))
        }
    } catch (_) { alert('重命名失败') }
}

async function doDelete() {
    if (!itemMenu.entry) return
    itemMenu.visible = false
    if (!window.confirm('确认删除目录 "' + itemMenu.entry.name + '" 及其所有内容？')) return
    try {
        const resp = await fetch('/api/file/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: itemMenu.entry.name, basePath: currentBrowseAbs })
        })
        if (resp.ok) {
            selectedPath.value = ''
            await loadBrowse()
        }
        else {
            const err = await resp.json()
            alert('删除失败: ' + (err.error || ''))
        }
    } catch (_) { alert('删除失败') }
}

async function confirm() {
    let path = selectedPath.value
    // 如果没有选择项目，使用 watchdir 目录
    if (!path && watchBase) {
        path = watchBase
    }
    if (!path) return
    try {
        const resp = await fetch('/api/project', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path })
        })
        if (resp.ok) {
            window.location.reload()
        } else {
            const text = await resp.text()
            let msg = text
            try { msg = JSON.parse(text).error || msg } catch (_) {}
            alert('设置项目失败: ' + msg)
        }
    } catch (err) {
        alert('设置项目失败: ' + err.message)
    }
}

onMounted(() => {
    // Initial load is handled by watch(() => props.open)
})
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.85);
  z-index: 2500;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
}

.modal-dialog {
  background: var(--bg-secondary, #fff);
  border-radius: var(--radius-md, 10px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  width: 100%;
  max-width: 480px;
  height: calc(100dvh - 48px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  flex-shrink: 0;
}

.modal-title {
  font-weight: 600;
  font-size: 15px;
  color: var(--text-primary, #1a1a1a);
}

.modal-close {
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
}

.modal-close:hover {
  color: var(--text-primary, #1a1a1a);
  background: var(--bg-tertiary, #f0f0f0);
}

.modal-body {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.modal-footer {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-top: 1px solid var(--border-color, #e5e5e5);
  flex-shrink: 0;
}

/* Tabs row */
.dialog-tabs-row {
  display: flex;
  align-items: center;
  padding: 0 10px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  background: var(--bg-tertiary, #f5f5f5);
  flex-shrink: 0;
}

.dialog-tabs {
  display: flex;
  gap: 4px;
}

.dialog-tab {
  padding: 8px 16px;
  border: none;
  background: transparent;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  color: var(--text-muted, #999);
  border-bottom: 2px solid transparent;
  transition: color 0.2s, border-color 0.2s;
}

.dialog-tab:hover { color: var(--text-secondary, #666); }
.dialog-tab.active { color: var(--accent-color, #0066cc); border-bottom-color: var(--accent-color, #0066cc); }

/* Browse nav */
.dialog-nav {
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  background: var(--bg-tertiary, #f5f5f5);
}

.dialog-toolbar-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px 4px;
}

.dialog-breadcrumb {
  padding: 4px 10px 6px;
  font-size: 12px;
  color: var(--text-secondary, #666);
  overflow-x: auto;
  white-space: nowrap;
  scrollbar-width: none;
}
.dialog-breadcrumb::-webkit-scrollbar {
  display: none;
}
.dialog-breadcrumb span { cursor: pointer; }
.dialog-breadcrumb span:hover { color: var(--accent-color, #0066cc); }

/* Toolbar buttons */
.toolbar-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: var(--radius-sm, 6px);
  background: var(--bg-primary, #fff);
  color: var(--text-secondary, #666);
  cursor: pointer;
  transition: all 0.15s;
  flex-shrink: 0;
}
.toolbar-btn:hover {
  background: var(--bg-tertiary, #f0f0f0);
  border-color: var(--accent-color, #0066cc);
  color: var(--accent-color, #0066cc);
}
.toolbar-btn:disabled { opacity: 0.35; cursor: not-allowed; }
.toolbar-btn.active {
  background: var(--accent-color, #0066cc);
  border-color: var(--accent-color, #0066cc);
  color: #fff;
}

/* Content */
.dialog-content {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
  min-height: 200px;
}

.dialog-item {
  display: flex;
  align-items: center;
  padding: 8px 16px;
  cursor: pointer;
  gap: 8px;
  transition: background 0.1s;
}
.dialog-item:hover { background: var(--bg-tertiary, #f0f0f0); }
.dialog-item.selected { background: var(--accent-color, #0066cc); color: #fff; }
.dialog-item.selected .item-icon,
.dialog-item.selected .item-name { color: #fff; }

.item-icon { font-size: 18px; flex-shrink: 0; width: 24px; text-align: center; }
.item-name { flex: 1; font-size: 14px; color: var(--text-primary, #1a1a1a); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

/* Item action button */
.item-action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted, #999);
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.15s, color 0.15s;
}
.item-action-btn:hover {
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-primary, #1a1a1a);
}
.dialog-item.selected .item-action-btn {
  color: rgba(255,255,255,0.7);
}
.dialog-item.selected .item-action-btn:hover {
  background: rgba(255,255,255,0.15);
  color: #fff;
}

/* Item menu popup */
.item-menu-popup {
  position: fixed;
  z-index: 3000;
  background: var(--bg-secondary, #fff);
  border: 1px solid var(--border-color, #dee2e6);
  border-radius: var(--radius-md, 8px);
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
  padding: 4px 0;
  min-width: 120px;
}

.item-menu-popup-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  font-size: 13px;
  color: var(--text-primary, #212529);
  cursor: pointer;
  transition: background 0.1s;
}
.item-menu-popup-item:hover {
  background: var(--bg-tertiary, #f0f0f0);
}
.item-menu-popup-item.danger {
  color: #dc2626;
}

.ctx-overlay {
  position: fixed;
  inset: 0;
  z-index: 2900;
}

.dialog-empty, .dialog-loading {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted, #999);
  font-size: 14px;
}

.search-input {
  flex: 1;
  border: 1px solid var(--border-color, #dee2e6);
  border-radius: 6px;
  outline: none;
  padding: 6px 10px;
  font-size: 13px;
  background: var(--bg-primary, #fff);
  color: var(--text-primary, #212529);
}

.search-input:focus {
  border-color: var(--accent-color, #4a90d9);
}

.confirm-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  background: var(--accent-color, #0066cc);
  color: #fff;
  border: none;
  border-radius: var(--radius-sm, 6px);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
  flex-shrink: 0;
}
.confirm-btn:hover { background: #0055aa; }
.confirm-btn:disabled { opacity: 0.5; cursor: not-allowed; }
</style>

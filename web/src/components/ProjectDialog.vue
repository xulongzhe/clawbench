<template>
  <ModalDialog :open="open" title="浏览项目" max-width="480px" full-height :z-index="2500" @close="$emit('close')">
    <!-- Browse nav -->
    <div class="dialog-nav">
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
        <SearchInput v-model="searchQuery" placeholder="搜索..." />
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
      <div v-else-if="displayItems.length === 0" class="dialog-empty">{{ searchQuery ? '没有匹配的目录' : '空目录' }}</div>
      <div
        v-else
        v-for="item in displayItems"
        :key="item.path"
        class="dialog-item"
        :class="{ selected: selectedPath === item.path }"
        @click="selectItem(item)"
        @dblclick="enterDir(item)"
      >
        <svg class="item-icon-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="16" height="16">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
        </svg>
        <span class="item-name">{{ item.name }}</span>
        <button class="item-action-btn" @click.stop="doRename(item)" title="重命名">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
        </button>
        <button class="item-action-btn danger" @click.stop="doDelete(item)" title="删除">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
        </button>
      </div>
    </div>

    <template #footer>
      <button class="cancel-btn" @click="$emit('close')">取消</button>
      <button class="confirm-btn" @click="confirm">
        <span>确定</span>
      </button>
    </template>
  </ModalDialog>
</template>

<script setup>
import { ref, computed, watch, inject } from 'vue'
import ModalDialog from './ModalDialog.vue'
import SearchInput from './SearchInput.vue'
import { baseName, splitPath } from '@/utils/helpers.ts'

const props = defineProps({
  open: Boolean,
})
const emit = defineEmits(['close'])
const toast = inject('toast', null)

const loading = ref(false)
const selectedPath = ref('')
const searchQuery = ref('')
const showHidden = ref(false)

// Browse state
const browsePath = ref('/')
const browseItems = ref([])
let watchBase = ''
let currentBrowseAbs = ''

function toRelative(absPath) {
    if (!watchBase) return absPath
    const rel = absPath.slice(watchBase.length).replace(/^\//, '')
    return rel || '/'
}

// Reload data when dialog opens (only first time)
let initialized = false
watch(() => props.open, (isOpen) => {
    if (isOpen) {
        searchQuery.value = ''
        if (!initialized) {
            initialized = true
            loadBrowse()
        }
    }
})

const browsePathParts = computed(() => splitPath(browsePath.value).filter(Boolean))

const displayItems = computed(() => {
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
        if (toast) toast.show('无法加载目录，后端服务可能未启动', { icon: '⚠️', duration: 5000 })
    } finally {
        loading.value = false
    }
}

async function doNewFolder() {
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

async function doRename(item) {
    const newName = prompt('输入新名称：', item.name)
    if (!newName || newName === item.name) return
    try {
        const resp = await fetch('/api/file/rename', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: item.name, name: newName, basePath: currentBrowseAbs })
        })
        if (resp.ok) await loadBrowse()
        else {
            const err = await resp.json()
            alert('重命名失败: ' + (err.error || ''))
        }
    } catch (_) { alert('重命名失败') }
}

async function doDelete(item) {
    if (!window.confirm('确认删除目录 "' + item.name + '" 及其所有内容？')) return
    try {
        const resp = await fetch('/api/file/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: item.name, basePath: currentBrowseAbs })
        })
        if (resp.ok) {
            selectedPath.value = ''
            await loadBrowse()
        } else {
            const err = await resp.json()
            alert('删除失败: ' + (err.error || ''))
        }
    } catch (_) { alert('删除失败') }
}

async function confirm() {
    let path = selectedPath.value
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
</script>

<style scoped>
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
  color: var(--accent-color, #0066cc);
}
.toolbar-btn:disabled { opacity: 0.35; cursor: not-allowed; }
.toolbar-btn.active {
  background: var(--accent-color, #0066cc);
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
.dialog-item.selected .item-name { color: #fff; }

.item-icon-svg { flex-shrink: 0; color: var(--accent-color, #0066cc); }
.dialog-item.selected .item-icon-svg { color: #fff; }
.item-name { flex: 1; font-size: 14px; color: var(--text-primary, #1a1a1a); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

/* Item action buttons */
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
.item-action-btn.danger:hover {
  color: #dc2626;
}
.dialog-item.selected .item-action-btn {
  color: rgba(255,255,255,0.7);
}
.dialog-item.selected .item-action-btn:hover {
  background: rgba(255,255,255,0.15);
  color: #fff;
}

.dialog-empty, .dialog-loading {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted, #999);
  font-size: 14px;
}

.cancel-btn {
  padding: 7px 14px;
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-secondary, #666);
  border: 1px solid var(--border-color, #dee2e6);
  border-radius: var(--radius-sm, 6px);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
  flex-shrink: 0;
}
.cancel-btn:hover { background: var(--bg-secondary); }

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

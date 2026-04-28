<template>
  <BottomSheet :open="open" @close="$emit('close')">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
        <polyline points="14 2 14 8 20 8"/>
        <line x1="16" y1="13" x2="8" y2="13"/>
        <line x1="16" y1="17" x2="8" y2="17"/>
      </svg>
      <span class="bs-header-title">文件详情</span>
      <button class="bs-close" @click.stop="$emit('close')" title="关闭">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    </template>

    <div class="details-body">
      <div class="details-row" v-for="item in detailItems" :key="item.label"
        :class="{ 'details-row-copyable': item.copyable }">
        <span class="details-label">{{ item.label }}</span>
        <div class="details-value-wrap" @click="item.copyable && copyValue(item.value, $event)">
          <span class="details-value" :class="{ 'details-value-copyable': item.copyable }">{{ item.value }}</span>
          <button v-if="item.copyable" class="details-copy-btn" @click.stop="copyValue(item.value, $event)" title="复制">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
            </svg>
          </button>
        </div>
      </div>
    </div>

  </BottomSheet>
</template>

<script setup>
import { computed, inject } from 'vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import { store } from '@/stores/app.ts'
import { getFileType, formatFileSize } from '@/utils/helpers.ts'

const props = defineProps({
  file: Object,
  open: Boolean,
})
defineEmits(['close'])

const toast = inject('toast', null)

function copyValue(value, event) {
  const wrap = event.currentTarget.closest('.details-value-wrap')
  const btn = wrap.querySelector('.details-copy-btn')
  const txt = wrap.querySelector('.details-value')
  const doCopy = () => {
    if (btn) { btn.classList.add('copied'); setTimeout(() => btn.classList.remove('copied'), 800) }
    if (txt) { txt.classList.add('copied'); setTimeout(() => txt.classList.remove('copied'), 800) }
    if (toast) toast.show('已复制', { icon: '📋', type: 'success', duration: 1500 })
  }
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(value).then(doCopy).catch(() => fallbackCopy(value, doCopy))
  } else {
    fallbackCopy(value, doCopy)
  }
}

function fallbackCopy(value, cb) {
  const ta = document.createElement('textarea')
  ta.value = value
  ta.style.cssText = 'position:fixed;opacity:0;top:0;left:0'
  document.body.appendChild(ta)
  ta.select()
  document.execCommand('copy')
  document.body.removeChild(ta)
  cb()
}

const fileType = computed(() => props.file ? getFileType(props.file.name) : null)

const absPath = computed(() => {
    if (!props.file) return ''
    const root = store.state.projectRoot
    return root ? root + '/' + props.file.path : props.file.path
})

const modified = computed(() => {
  if (!props.file) return ''
  const dir = store.state.currentDir
  const name = props.file.name
  const fullPath = dir ? dir + '/' + name : name
  const entry = store.state.dirEntries?.find(e => {
    const ep = dir ? dir + '/' + e.name : e.name
    return ep === props.file.path
  })
  if (entry?.modified) {
    const d = new Date(entry.modified)
    return d.toLocaleString()
  }
  return ''
})

const lineCount = computed(() => {
  if (!props.file?.content) return ''
  return props.file.content.split('\n').length
})

const charCount = computed(() => {
  if (!props.file?.content) return ''
  return props.file.content.length.toLocaleString()
})

const detailItems = computed(() => {
  if (!props.file) return []
  const items = [
    { label: '文件名', value: props.file.name, copyable: true },
    { label: '路径', value: absPath.value, copyable: true },
    { label: '类型', value: fileType.value?.label || '未知' },
  ]
  if (props.file.size != null) {
    items.push({ label: '大小', value: formatFileSize(props.file.size) })
  }
  if (modified.value) {
    items.push({ label: '修改时间', value: modified.value })
  }
  if (props.file.content) {
    items.push({ label: '行数', value: lineCount.value })
    items.push({ label: '字符数', value: charCount.value })
  }
  items.push({ label: '编码', value: 'UTF-8' })
  return items
})
</script>

<style scoped>
.details-title {
  font-weight: 600;
  font-size: 14px;
  color: var(--text-primary, #1a1a1a);
}

.details-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.details-row {
  display: flex;
  align-items: center;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
}

.details-label {
  width: 80px;
  flex-shrink: 0;
  font-size: 13px;
  color: var(--text-muted, #999);
}

.details-value-wrap {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.details-value {
  flex: 1;
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  word-break: break-all;
}

.details-value-copyable {
  cursor: pointer;
}

.details-copy-btn {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-muted, #999);
  padding: 2px;
  border-radius: 3px;
  transition: color 0.15s, background 0.15s;
}
.details-copy-btn:hover {
  color: var(--accent-color, #4a90d9);
  background: var(--bg-tertiary, #f0f0f0);
}
.details-copy-btn.copied {
  color: #22c55e;
}

.details-row-copyable {
  user-select: none;
}
.details-row-copyable:hover {
  background: var(--bg-tertiary, #f5f5f5);
}
.details-value-copyable:hover {
  color: var(--accent-color, #4a90d9);
}
.details-value-copyable.copied {
  color: #22c55e;
}

</style>

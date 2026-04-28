<template>
  <pre class="raw-content-pre" ref="codeRef">
    <code v-html="codeHtml" />
  </pre>

  <!-- Context menu -->
  <Teleport to="body">
    <div v-if="showContextMenu" class="line-context-menu" :style="{ left: menuPos.x + 'px', top: menuPos.y + 'px' }">
      <div v-if="editable" class="line-context-item" @click="handleEditLine">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>编辑
      </div>
      <div class="line-context-item" @click="handleCopyLine">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>复制
      </div>
      <div v-if="editable" class="line-context-item" @click="handleInsertAbove">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="19" x2="12" y2="5"/><polyline points="5 12 12 5 19 12"/></svg>上方插入
      </div>
      <div v-if="editable" class="line-context-item" @click="handleInsertBelow">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><polyline points="19 12 12 19 5 12"/></svg>下方插入
      </div>
      <div v-if="editable" class="line-context-item danger" @click="handleDeleteLine">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>删除
      </div>
    </div>
    <div v-if="showContextMenu" class="line-context-backdrop" @click="showContextMenu = false" />
  </Teleport>

  <!-- Edit / Insert dialog (using BottomSheet) -->
  <BottomSheet
    :open="editingLine !== null"
    :title="editDrawerTitle"
    compact
    @close="closeEditDrawer"
  >
    <textarea v-model="editContent" class="line-edit-textarea" rows="3" />
    <template #footer>
      <div class="line-edit-actions">
        <button class="btn" @click="closeEditDrawer">取消</button>
        <button class="btn btn-confirm" @click="handleSaveEditOrInsert">保存</button>
      </div>
    </template>
  </BottomSheet>
</template>

<script setup>
import { ref, watch, nextTick, inject, computed } from 'vue'
import { hljs } from '@/utils/globals.ts'
import { escapeHtml } from '@/utils/helpers.ts'
import { useLongPressLineMenu } from '@/composables/useLongPressLineMenu.ts'
import BottomSheet from '@/components/common/BottomSheet.vue'

const props = defineProps({
    /** Raw file content */
    content: { type: String, default: '' },
    /** Language for syntax highlighting */
    language: { type: String, default: 'plaintext' },
    /** File path for edit API calls */
    filePath: { type: String, default: null },
    /** Enable edit/delete/insert operations */
    editable: { type: Boolean, default: true },
})

const emit = defineEmits(['content-change'])

const toast = inject('toast', null)

const codeHtml = ref('')
const codeRef = ref(null)

// Internal mutable content for editing
let internalContent = props.content

function getContent() { return internalContent }
function setContent(v) {
    internalContent = v
    emit('content-change', v)
}

const {
    showContextMenu, menuPos,
    editingLine, editContent, insertMode,
    handleEditLine, handleSaveEditOrInsert, handleDeleteLine, handleCopyLine,
    handleInsertAbove, handleInsertBelow,
    copiedLine, highlightedLine,
} = useLongPressLineMenu(
    codeRef,
    () => props.filePath,
    getContent,
    setContent,
    props.editable,
    () => { if (toast) toast.show('已复制', { icon: '📋', type: 'success', duration: 1500 }) },
)

// Computed title for edit drawer
const editDrawerTitle = computed(() => {
    if (insertMode.value === 'above') return '在上方插入行'
    if (insertMode.value === 'below') return '在下方插入行'
    return `编辑第 ${editingLine.value + 1} 行`
})

// Close edit drawer
function closeEditDrawer() {
    editingLine.value = null
    insertMode.value = null
}

// Highlight the line being edited and show insert marker
watch([editingLine, insertMode], async ([lineIdx, mode]) => {
    await nextTick()
    if (!codeRef.value) return
    codeRef.value.querySelectorAll('.code-line').forEach(el => el.classList.remove('line-editing', 'line-insert-marker', 'insert-above', 'insert-below'))
    if (lineIdx !== null) {
        if (mode === 'above') {
            const el = codeRef.value.querySelector(`.code-line[data-line="${lineIdx + 1}"]`)
            if (el) { el.classList.add('line-insert-marker', 'insert-above'); el.scrollIntoView({ behavior: 'smooth', block: 'center' }) }
        } else if (mode === 'below') {
            const el = codeRef.value.querySelector(`.code-line[data-line="${lineIdx + 1}"]`)
            if (el) { el.classList.add('line-insert-marker', 'insert-below'); el.scrollIntoView({ behavior: 'smooth', block: 'center' }) }
        } else {
            const el = codeRef.value.querySelector(`.code-line[data-line="${lineIdx + 1}"]`)
            if (el) { el.classList.add('line-editing'); el.scrollIntoView({ behavior: 'smooth', block: 'center' }) }
        }
    }
})

// Add copied feedback — flash entire line background
watch(copiedLine, (ln) => {
    if (!codeRef.value || !ln) return
    const el = codeRef.value.querySelector(`.code-line[data-line="${ln}"]`)
    if (!el) return
    el.classList.add('line-copied')
    setTimeout(() => el.classList.remove('line-copied'), 800)
})

// Add highlight feedback for long-press
watch(highlightedLine, async (ln) => {
    await nextTick()
    if (!codeRef.value) return
    codeRef.value.querySelectorAll('.code-line').forEach(el => el.classList.remove('line-highlighted'))
    if (ln) {
        const el = codeRef.value.querySelector(`.code-line[data-line="${ln}"]`)
        if (el) el.classList.add('line-highlighted')
    }
})

function renderCode(content, lang) {
    return content.split('\n').map((rawLine, i) => {
        let h
        try { h = hljs.highlight(rawLine, { language: lang, ignoreIllegals: true }).value } catch { h = escapeHtml(rawLine) }
        h = h.replace(/^<span class="line">/, '').replace(/<\/span>\s*$/, '')
        return `<div class="code-line" data-line="${i + 1}"><span class="line-num">${i + 1}</span><span class="code-text">${h}</span></div>`
    }).join('')
}

function doRender(content) {
    if (!content) return
    internalContent = content
    codeHtml.value = renderCode(content, props.language)
}

watch(() => props.content, doRender, { immediate: true })
</script>

<style scoped>
pre {
    -webkit-touch-callout: none;
    -webkit-user-select: none;
    user-select: none;
    min-height: 0;
}
pre :deep(code) {
    min-height: 0;
}

/* Line context menu */
.line-context-backdrop {
    position: fixed;
    inset: 0;
    z-index: 2499;
}

.line-context-menu {
    position: fixed;
    z-index: 2500;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
    padding: 4px 0;
    min-width: 0;
}

.line-context-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 14px;
    font-size: 13px;
    color: var(--text-primary);
    cursor: pointer;
    transition: background 0.1s;
}

.line-context-item svg {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
}

.line-context-item:hover, .line-context-item:active { background: var(--bg-tertiary); }

.line-context-item.danger {
    color: #ef4444;
}

.line-context-item.danger:hover {
    background: #fef2f2;
}

:global([data-theme="dark"]) .line-context-item.danger:hover {
    background: #2d1b1b;
}

/* Line edit textarea and actions */
.line-edit-textarea {
    padding: 10px 12px;
    border: none;
    background: var(--bg-primary);
    color: var(--text-primary);
    font-family: monospace;
    font-size: 13px;
    resize: none;
    min-height: 112px;
    max-height: 40vh;
    outline: none;
    display: block;
    width: 100%;
    box-sizing: border-box;
}

.line-edit-actions {
    display: flex;
    width: 100%;
    padding: 8px 12px;
    gap: 8px;
    box-sizing: border-box;
    /* Negate BottomSheet footer padding (8px 12px) to avoid doubling */
    margin: -8px -12px;
    /* Restore safe-area bottom padding that footer would have provided */
    padding-bottom: calc(8px + env(safe-area-inset-bottom, 0px));
}

.line-edit-actions .btn {
    flex: 1;
    padding: 10px 0;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    border: none;
    background: var(--bg-tertiary);
    color: var(--text-secondary);
    transition: opacity 0.15s;
}

.line-edit-actions .btn:active { opacity: 0.7; }

.line-edit-actions .btn-confirm {
    background: var(--accent-color);
    color: #fff;
}

.line-edit-actions .btn-confirm:active { opacity: 0.7; }

/* Raw content pre - code display area */
.raw-content-pre {
    margin: 0;
    flex: 1;
    min-height: 0;
    overflow: auto;
    background: var(--code-bg);
    border: none;
    font-size: 13px;
    line-height: 1.6;
    tab-size: 4;
}

.raw-content-pre :deep(code) {
    font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Segoe UI Mono', 'Roboto Mono', Consolas, 'Liberation Mono', monospace;
    background: transparent;
    padding: 0;
    font-size: inherit;
    white-space: pre;
    display: block;
    min-width: max-content;
}

.raw-content-pre :deep(code .code-line) {
    display: flex;
    align-items: start;
}

.raw-content-pre :deep(code .line-num) {
    position: sticky;
    left: 0;
    display: inline-block;
    min-width: 48px;
    padding-right: 12px;
    margin-right: 0;
    color: var(--text-muted);
    text-align: right;
    user-select: none;
    cursor: pointer;
    border-right: 1px solid var(--border-color);
    opacity: 0.5;
    transition: opacity 0.15s, color 0.15s;
    font-size: inherit;
    line-height: inherit;
    background: var(--code-bg);
}

.raw-content-pre :deep(code .code-text) {
    white-space: pre;
    padding-left: 12px;
}

.raw-content-pre :deep(code .line-num:hover) {
    opacity: 1;
    color: var(--accent-color);
}

.raw-content-pre :deep(code .line-num.copied) {
    color: #22c55e;
    opacity: 1;
}

.raw-content-pre :deep(code .code-line.line-editing) {
    background: var(--accent-color) !important;
    border-radius: 3px;
}

.raw-content-pre :deep(code .code-line.line-highlighted) {
    background: rgba(74, 144, 217, 0.15) !important;
}

.raw-content-pre :deep(code .code-line.line-copied) {
    background: rgba(34, 197, 94, 0.2) !important;
}

.raw-content-pre :deep(code .line-insert-marker) {
    border: none;
}

.raw-content-pre :deep(code .line-insert-marker.insert-above) {
    border-top: 3px solid var(--accent-color);
}

.raw-content-pre :deep(code .line-insert-marker.insert-below) {
    border-bottom: 3px solid var(--accent-color);
}
</style>

<style>
/* Line highlight flash animation - non-scoped for dynamic classList access */
@keyframes line-flash {
    0%, 100% { background: transparent; }
    10%, 30%  { background: rgba(255, 230, 0, 0.4); }
    20%, 40%  { background: transparent; }
    50%, 70%  { background: rgba(255, 230, 0, 0.3); }
    60%, 80%  { background: transparent; }
    90%       { background: rgba(255, 230, 0, 0.2); }
}
.line-flash {
    animation: line-flash 1.2s ease-out forwards;
}

/* Copy flash animation for block elements */
@keyframes copy-flash {
    0%, 100% { background: transparent; }
    50%      { background: rgba(255, 230, 0, 0.4); }
}
.copy-flash {
    animation: copy-flash 0.4s ease-out forwards;
    border-radius: 4px;
}
</style>

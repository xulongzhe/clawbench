<template>
  <ModalDialog :open="open" :title="currentPage === 'list' ? t('chat.quickSend.title') : (editingItem ? t('chat.quickSend.editItem') : t('chat.quickSend.addItem'))" @close="handleClose">
    <template #header>
      <span class="modal-header-icon" @click="goBackIfEdit">
        <ChevronLeftIcon v-if="currentPage === 'edit'" :size="16" />
        <SendIcon v-else :size="16" />
      </span>
      <span class="modal-title">{{ currentPage === 'list' ? t('chat.quickSend.title') : (editingItem ? t('chat.quickSend.editItem') : t('chat.quickSend.addItem')) }}</span>
    </template>

    <!-- Page: Item list -->
    <div v-if="currentPage === 'list'" class="qs-content">
      <div v-if="items.length > 0" class="qs-list">
        <draggable
          v-model="localItems"
          handle=".qs-drag-handle"
          item-key="id"
          @end="onDragEnd"
        >
          <template #item="{ element: item }">
            <div class="qs-item-wrapper">
              <div class="qs-row" :class="{ 'qs-hidden': item.hidden }">
                <span class="qs-drag-handle">≡</span>
                <span class="qs-label">
                  <EyeOffIcon v-if="item.hidden" :size="12" class="qs-badge-dim" />
                  {{ item.label }}
                </span>
                <span class="qs-cmd" :title="item.command">{{ item.command }}</span>
                <button class="qs-action" @click="editItem(item)" :title="t('chat.quickSend.editItem')">
                  <PencilIcon :size="14" />
                </button>
                <button class="qs-action danger" @click="toggleDeleteConfirm(item.id)" :title="t('chat.quickSend.deleteItem')">
                  <Trash2Icon :size="14" />
                </button>
              </div>
              <!-- Inline delete confirmation -->
              <div v-if="deleteConfirmId === item.id" class="qs-delete-confirm">
                <span>{{ t('chat.quickSend.deleteConfirm') }}</span>
                <button class="qs-confirm-btn delete" @click="doDelete(item.id)">{{ t('common.confirm') }}</button>
                <button class="qs-confirm-btn cancel" @click="deleteConfirmId = null">{{ t('common.cancel') }}</button>
              </div>
            </div>
          </template>
        </draggable>
      </div>

      <button class="qs-add" @click="addNewItem">
        <PlusIcon :size="16" />
        {{ t('chat.quickSend.addItem') }}
      </button>
    </div>

    <!-- Page: Edit form (drill-down) -->
    <div v-else class="qs-edit-content">
      <div class="form-group">
        <label class="form-label">{{ t('chat.quickSend.itemLabel') }} <span class="required">*</span></label>
        <input type="text" class="form-input" v-model="form.label" :placeholder="t('chat.quickSend.itemLabel')" />
      </div>
      <div class="form-group">
        <label class="form-label">{{ t('chat.quickSend.itemCommand') }} <span class="required">*</span></label>
        <input type="text" class="form-input" v-model="form.command" :placeholder="t('chat.quickSend.itemCommand')" />
      </div>
      <div v-if="formError" class="form-error">{{ formError }}</div>

      <label class="form-checkbox">
        <input type="checkbox" v-model="form.hidden" />
        <span>{{ t('chat.quickSend.itemHidden') }}</span>
      </label>
    </div>

    <template #footer>
      <template v-if="currentPage === 'list'">
        <button class="modal-btn" @click="$emit('close')">{{ t('common.close') }}</button>
      </template>
      <template v-else>
        <button class="modal-btn" @click="currentPage = 'list'">{{ t('common.cancel') }}</button>
        <button class="modal-btn primary" :disabled="saving" @click="saveItem">{{ saving ? '...' : t('common.save') }}</button>
      </template>
    </template>
  </ModalDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import draggable from 'vuedraggable'
import ModalDialog from '@/components/common/ModalDialog.vue'
import { Send as SendIcon, PencilIcon, Trash2Icon, PlusIcon, ChevronLeftIcon, EyeOffIcon } from 'lucide-vue-next'
import { useQuickSend, type QuickSendItem } from '@/composables/useQuickSend'
import { useToast } from '@/composables/useToast'

const props = defineProps({
  open: Boolean,
})

const emit = defineEmits(['close'])

const { t } = useI18n()
const toast = useToast()
const { items, reorderItems, addItem, updateItem, deleteItem } = useQuickSend()

const localItems = ref<QuickSendItem[]>([...items.value])
const currentPage = ref<'list' | 'edit'>('list')
const editingItem = ref<QuickSendItem | null>(null)
const form = ref({ label: '', command: '', hidden: false })
const formError = ref('')
const saving = ref(false)
const deleteConfirmId = ref<number | null>(null)

// Sync local list when items change
watch(items, (val) => {
  localItems.value = [...val]
}, { deep: true })

// Reset state when dialog opens/closes
watch(() => props.open, (isOpen) => {
  if (isOpen) {
    currentPage.value = 'list'
    deleteConfirmId.value = null
    formError.value = ''
    editingItem.value = null
  }
})

function handleClose() {
  currentPage.value = 'list'
  deleteConfirmId.value = null
  emit('close')
}

function goBackIfEdit() {
  if (currentPage.value === 'edit') {
    currentPage.value = 'list'
  }
}

function editItem(item: QuickSendItem) {
  editingItem.value = item
  form.value = { label: item.label, command: item.command, hidden: item.hidden }
  formError.value = ''
  currentPage.value = 'edit'
}

function addNewItem() {
  editingItem.value = null
  form.value = { label: '', command: '', hidden: false }
  formError.value = ''
  currentPage.value = 'edit'
}

async function saveItem() {
  const label = form.value.label.trim()
  const command = form.value.command.trim()
  if (!label || !command) {
    formError.value = t('chat.quickSend.itemRequired')
    return
  }
  formError.value = ''
  saving.value = true

  try {
    let ok: boolean
    if (editingItem.value) {
      ok = await updateItem(editingItem.value.id, { label, command, hidden: form.value.hidden })
    } else {
      ok = await addItem({ label, command, hidden: form.value.hidden })
    }

    if (ok) {
      toast.show(t('chat.quickSend.itemSaved'), { type: 'success' })
      currentPage.value = 'list'
    } else {
      formError.value = t('chat.quickSend.saveFailed')
    }
  } finally {
    saving.value = false
  }
}

function toggleDeleteConfirm(id: number) {
  deleteConfirmId.value = deleteConfirmId.value === id ? null : id
}

async function doDelete(id: number) {
  deleteConfirmId.value = null
  const ok = await deleteItem(id)
  if (ok) {
    toast.show(t('chat.quickSend.itemDeleted'), { type: 'success' })
  }
}

async function onDragEnd() {
  const ids = localItems.value.map(it => it.id)
  const ok = await reorderItems(ids)
  if (!ok) {
    toast.show(t('chat.quickSend.reorderFailed'), { type: 'error' })
    localItems.value = [...items.value] // Reset from source of truth
  }
}
</script>

<style>
.qs-content {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.qs-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}

.qs-item-wrapper {
  border-bottom: 1px solid var(--border-color, #e5e5e5);
}

.qs-item-wrapper:last-child {
  border-bottom: none;
}

.qs-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  font-size: 13px;
  color: var(--text-primary);
  transition: background 0.12s;
}

.qs-row.qs-hidden {
  opacity: 0.55;
}

.qs-row:hover {
  background: var(--bg-tertiary, #f5f5f5);
}

.qs-drag-handle {
  cursor: grab;
  color: var(--text-muted, #999);
  font-size: 16px;
  line-height: 1;
  user-select: none;
  padding: 0 2px;
}

.qs-drag-handle:active {
  cursor: grabbing;
}

.qs-label {
  flex-shrink: 0;
  font-weight: 500;
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 3px;
}

.qs-badge-dim {
  color: var(--text-muted, #999);
  flex-shrink: 0;
}

.qs-cmd {
  flex: 1;
  min-width: 0;
  color: var(--text-muted, #999);
  font-family: monospace;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.qs-action {
  background: none;
  border: none;
  color: var(--text-muted, #999);
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  border-radius: 4px;
  transition: background 0.12s, color 0.12s;
}

.qs-action:hover {
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-primary);
}

.qs-action.danger:hover {
  color: #e53e3e;
}

.qs-delete-confirm {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px 6px 28px;
  background: color-mix(in srgb, #e53e3e 8%, transparent);
  font-size: 12px;
  color: var(--text-secondary, #666);
}

.qs-confirm-btn {
  padding: 3px 10px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  background: var(--bg-primary, #fff);
  color: var(--text-primary);
}

.qs-confirm-btn.delete {
  background: #e53e3e;
  color: #fff;
  border-color: #e53e3e;
}

.qs-confirm-btn.cancel {
  color: var(--text-muted, #999);
}

.qs-add {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 10px;
  margin: 4px 10px 10px;
  border: 1px dashed var(--border-color, #ddd);
  border-radius: 8px;
  background: none;
  color: var(--accent-color, #0066cc);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.12s;
}

.qs-add:hover {
  background: color-mix(in srgb, var(--accent-color, #0066cc) 8%, transparent);
}

.qs-edit-content {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary, #666);
}

.form-label .required {
  color: #e53e3e;
}

.form-input {
  padding: 8px 10px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 6px;
  font-size: 13px;
  background: var(--bg-primary, #fff);
  color: var(--text-primary);
  outline: none;
  transition: border-color 0.15s;
}

.form-input:focus {
  border-color: var(--accent-color, #0066cc);
}

.form-error {
  font-size: 12px;
  color: #e53e3e;
}

.form-checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-primary);
  cursor: pointer;
}

.form-checkbox input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: var(--accent-color, #0066cc);
}

.modal-btn {
  padding: 6px 16px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 6px;
  background: var(--bg-primary, #fff);
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.12s;
}

.modal-btn:hover {
  background: var(--bg-tertiary, #f5f5f5);
}

.modal-btn.primary {
  background: var(--accent-color, #0066cc);
  color: #fff;
  border-color: var(--accent-color, #0066cc);
}

.modal-btn.primary:hover {
  opacity: 0.9;
}

.modal-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>

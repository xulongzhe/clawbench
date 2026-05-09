import { ref, computed } from 'vue'
import { apiGet, apiPost, apiPut, apiDelete } from '@/utils/api'

export interface QuickSendItem {
  id: number
  label: string
  command: string
  hidden: boolean
  sort_order: number
}

// Module-level singleton state
const items = ref<QuickSendItem[]>([])
const loaded = ref(false)
const showEditDialog = ref(false)

export function useQuickSend() {
  const visibleItems = computed(() => items.value.filter(i => !i.hidden))

  async function fetchItems(force = false) {
    if (loaded.value && !force) return
    try {
      const data = await apiGet<QuickSendItem[]>('/api/chat/quick-send')
      items.value = data || []
      loaded.value = true
    } catch {
      // Silently fail on initial load
    }
  }

  async function addItem(item: { label: string; command: string; hidden?: boolean }): Promise<boolean> {
    try {
      await apiPost('/api/chat/quick-send', item)
      await fetchItems(true)
      return true
    } catch {
      return false
    }
  }

  async function updateItem(id: number, item: { label: string; command: string; hidden?: boolean }): Promise<boolean> {
    try {
      await apiPut(`/api/chat/quick-send/${id}`, item)
      await fetchItems(true)
      return true
    } catch {
      return false
    }
  }

  async function deleteItem(id: number): Promise<boolean> {
    try {
      await apiDelete(`/api/chat/quick-send/${id}`)
      await fetchItems(true)
      return true
    } catch {
      return false
    }
  }

  async function reorderItems(ids: number[]): Promise<boolean> {
    const oldItems = [...items.value]
    // Optimistic reorder
    const reordered = ids.map((id, i) => {
      const item = items.value.find(it => it.id === id)
      return item ? { ...item, sort_order: i } : null
    }).filter(Boolean) as QuickSendItem[]
    items.value = reordered
    try {
      await apiPut('/api/chat/quick-send/reorder', { ids })
      return true
    } catch {
      items.value = oldItems // Rollback
      return false
    }
  }

  return {
    items,
    visibleItems,
    fetchItems,
    addItem,
    updateItem,
    deleteItem,
    reorderItems,
    showEditDialog,
    loaded,
  }
}

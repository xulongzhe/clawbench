import { describe, expect, it, vi, beforeEach } from 'vitest'

// ────────────────────────────────────────────────────────────
// useQuickSend composable relies on apiGet/apiPost/apiPut/apiDelete.
// We mock the API helpers and test the core logic:
// - visibleItems filters hidden items
// - fetchItems loads from API and caches
// - addItem/updateItem/deleteItem call correct endpoints + refetch
// - reorderItems does optimistic update with rollback on failure
// ────────────────────────────────────────────────────────────

// Mock API helpers
const mockApiGet = vi.fn()
const mockApiPost = vi.fn()
const mockApiPut = vi.fn()
const mockApiDelete = vi.fn()

vi.mock('@/utils/api', () => ({
  apiGet: (...args: any[]) => mockApiGet(...args),
  apiPost: (...args: any[]) => mockApiPost(...args),
  apiPut: (...args: any[]) => mockApiPut(...args),
  apiDelete: (...args: any[]) => mockApiDelete(...args),
}))

// Import after mocks
import { useQuickSend, type QuickSendItem } from '@/composables/useQuickSend'

// Helper to create test items
function makeItem(overrides: Partial<QuickSendItem> = {}): QuickSendItem {
  return {
    id: 1,
    label: '继续',
    command: '继续',
    hidden: false,
    sort_order: 0,
    ...overrides,
  }
}

beforeEach(() => {
  // Reset module-level singleton state between tests
  // by re-importing would be ideal, but module-level state persists.
  // We call fetchItems(true) to reset in each test instead.
  mockApiGet.mockReset()
  mockApiPost.mockReset()
  mockApiPut.mockReset()
  mockApiDelete.mockReset()
})

// ---------- visibleItems computed ----------

describe('visibleItems', () => {
  it('filters out hidden items', async () => {
    const items = [
      makeItem({ id: 1, label: '继续', hidden: false }),
      makeItem({ id: 2, label: '隐藏', hidden: true }),
      makeItem({ id: 3, label: '提交', hidden: false }),
    ]
    mockApiGet.mockResolvedValue(items)

    const { visibleItems, fetchItems } = useQuickSend()
    await fetchItems(true)

    expect(visibleItems.value).toHaveLength(2)
    expect(visibleItems.value[0].label).toBe('继续')
    expect(visibleItems.value[1].label).toBe('提交')
  })

  it('returns all items when none hidden', async () => {
    const items = [
      makeItem({ id: 1, hidden: false }),
      makeItem({ id: 2, hidden: false }),
    ]
    mockApiGet.mockResolvedValue(items)

    const { visibleItems, fetchItems } = useQuickSend()
    await fetchItems(true)

    expect(visibleItems.value).toHaveLength(2)
  })

  it('returns empty when all hidden', async () => {
    const items = [
      makeItem({ id: 1, hidden: true }),
      makeItem({ id: 2, hidden: true }),
    ]
    mockApiGet.mockResolvedValue(items)

    const { visibleItems, fetchItems } = useQuickSend()
    await fetchItems(true)

    expect(visibleItems.value).toHaveLength(0)
  })
})

// ---------- fetchItems ----------

describe('fetchItems', () => {
  it('calls GET /api/chat/quick-send', async () => {
    mockApiGet.mockResolvedValue([])

    const { fetchItems } = useQuickSend()
    await fetchItems(true)

    expect(mockApiGet).toHaveBeenCalledWith('/api/chat/quick-send')
  })

  it('uses cache on second call without force', async () => {
    mockApiGet.mockResolvedValue([makeItem()])

    const { fetchItems, items } = useQuickSend()
    await fetchItems(true) // First load
    expect(mockApiGet).toHaveBeenCalledTimes(1)

    await fetchItems(false) // Should use cache
    expect(mockApiGet).toHaveBeenCalledTimes(1) // No additional call
  })

  it('forces refetch when force=true', async () => {
    mockApiGet.mockResolvedValue([makeItem()])

    const { fetchItems } = useQuickSend()
    await fetchItems(true)
    await fetchItems(true) // Force again

    expect(mockApiGet).toHaveBeenCalledTimes(2)
  })

  it('silently fails on error', async () => {
    mockApiGet.mockRejectedValue(new Error('Network error'))

    const { fetchItems, items } = useQuickSend()
    // Should not throw
    await expect(fetchItems(true)).resolves.toBeUndefined()
  })

  it('handles null API response', async () => {
    mockApiGet.mockResolvedValue(null)

    const { fetchItems, items } = useQuickSend()
    await fetchItems(true)

    expect(items.value).toEqual([])
  })
})

// ---------- addItem ----------

describe('addItem', () => {
  it('calls POST /api/chat/quick-send and refetches', async () => {
    mockApiPost.mockResolvedValue({ id: 1 })
    mockApiGet.mockResolvedValue([makeItem({ id: 1, label: '继续' })])

    const { addItem, items } = useQuickSend()
    const result = await addItem({ label: '继续', command: '继续', hidden: false })

    expect(result).toBe(true)
    expect(mockApiPost).toHaveBeenCalledWith('/api/chat/quick-send', {
      label: '继续',
      command: '继续',
      hidden: false,
    })
    // Refetch called after add
    expect(mockApiGet).toHaveBeenCalledWith('/api/chat/quick-send')
  })

  it('returns false on API error', async () => {
    mockApiPost.mockRejectedValue(new Error('Server error'))

    const { addItem } = useQuickSend()
    const result = await addItem({ label: '继续', command: '继续' })

    expect(result).toBe(false)
  })
})

// ---------- updateItem ----------

describe('updateItem', () => {
  it('calls PUT /api/chat/quick-send/{id} and refetches', async () => {
    mockApiPut.mockResolvedValue({ success: true })
    mockApiGet.mockResolvedValue([makeItem({ id: 1, label: '▶️ 继续' })])

    const { updateItem } = useQuickSend()
    const result = await updateItem(1, { label: '▶️ 继续', command: '请继续', hidden: true })

    expect(result).toBe(true)
    expect(mockApiPut).toHaveBeenCalledWith('/api/chat/quick-send/1', {
      label: '▶️ 继续',
      command: '请继续',
      hidden: true,
    })
  })

  it('returns false on API error', async () => {
    mockApiPut.mockRejectedValue(new Error('Not found'))

    const { updateItem } = useQuickSend()
    const result = await updateItem(999, { label: 'x', command: 'y' })

    expect(result).toBe(false)
  })
})

// ---------- deleteItem ----------

describe('deleteItem', () => {
  it('calls DELETE /api/chat/quick-send/{id} and refetches', async () => {
    mockApiDelete.mockResolvedValue({ success: true })
    mockApiGet.mockResolvedValue([])

    const { deleteItem } = useQuickSend()
    const result = await deleteItem(1)

    expect(result).toBe(true)
    expect(mockApiDelete).toHaveBeenCalledWith('/api/chat/quick-send/1')
  })

  it('returns false on API error', async () => {
    mockApiDelete.mockRejectedValue(new Error('Server error'))

    const { deleteItem } = useQuickSend()
    const result = await deleteItem(1)

    expect(result).toBe(false)
  })
})

// ---------- reorderItems (optimistic update) ----------

describe('reorderItems', () => {
  it('optimistically reorders items before API call', async () => {
    const originalItems = [
      makeItem({ id: 1, label: 'A', sort_order: 0 }),
      makeItem({ id: 2, label: 'B', sort_order: 1 }),
      makeItem({ id: 3, label: 'C', sort_order: 2 }),
    ]
    mockApiGet.mockResolvedValue(originalItems)

    const { fetchItems, items, reorderItems } = useQuickSend()
    await fetchItems(true)

    // Start reorder but delay the API response
    let resolveReorder: (v: any) => void
    const reorderPromise = new Promise(resolve => { resolveReorder = resolve })
    mockApiPut.mockReturnValue(reorderPromise)

    const resultPromise = reorderItems([3, 2, 1])

    // Before API resolves, items should be optimistically reordered
    expect(items.value.map(i => i.id)).toEqual([3, 2, 1])
    expect(items.value[0].sort_order).toBe(0)
    expect(items.value[1].sort_order).toBe(1)
    expect(items.value[2].sort_order).toBe(2)

    // Resolve the API call
    resolveReorder!({ success: true })
    const result = await resultPromise
    expect(result).toBe(true)
  })

  it('rolls back on API failure', async () => {
    const originalItems = [
      makeItem({ id: 1, label: 'A', sort_order: 0 }),
      makeItem({ id: 2, label: 'B', sort_order: 1 }),
    ]
    mockApiGet.mockResolvedValue(originalItems)

    const { fetchItems, items, reorderItems } = useQuickSend()
    await fetchItems(true)

    // API fails
    mockApiPut.mockRejectedValue(new Error('Network error'))

    const result = await reorderItems([2, 1])

    expect(result).toBe(false)
    // Should roll back to original order
    expect(items.value.map(i => i.id)).toEqual([1, 2])
  })

  it('calls PUT /api/chat/quick-send/reorder with IDs', async () => {
    mockApiGet.mockResolvedValue([makeItem({ id: 1 }), makeItem({ id: 2 })])
    mockApiPut.mockResolvedValue({ success: true })

    const { fetchItems, reorderItems } = useQuickSend()
    await fetchItems(true)

    await reorderItems([2, 1])

    expect(mockApiPut).toHaveBeenCalledWith('/api/chat/quick-send/reorder', { ids: [2, 1] })
  })
})

// ---------- showEditDialog ----------

describe('showEditDialog', () => {
  it('is shared across composable instances', async () => {
    mockApiGet.mockResolvedValue([])

    const instance1 = useQuickSend()
    const instance2 = useQuickSend()

    instance1.showEditDialog.value = true
    expect(instance2.showEditDialog.value).toBe(true)

    instance2.showEditDialog.value = false
    expect(instance1.showEditDialog.value).toBe(false)
  })
})

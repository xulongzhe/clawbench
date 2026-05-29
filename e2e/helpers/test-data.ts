import { getServerURL } from './server'

/**
 * Test data constants and seeding helpers for E2E tests.
 *
 * The Go server auto-creates an empty database on startup.
 * Data is seeded via API calls in test fixtures or test bodies.
 */

/** Default quick-send items for tests */
export const DEFAULT_QUICK_SEND_ITEMS = [
  { label: '继续', command: '继续' },
  { label: 'Review', command: '请 review 这个文件' },
  { label: 'Commit', command: '请提交当前的改动' },
]

/**
 * Seed quick-send items via API.
 * The server must be running and the user must be authenticated.
 */
export async function seedQuickSendItems(
  baseURL: string,
  items: { label: string; command: string }[] = DEFAULT_QUICK_SEND_ITEMS,
): Promise<void> {
  for (const item of items) {
    const response = await fetch(`${baseURL}/api/chat/quick-send`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(item),
    })
    if (!response.ok) {
      throw new Error(`Failed to seed quick-send item "${item.label}": ${response.status} ${response.statusText}`)
    }
  }
}

/**
 * Get all quick-send items via API.
 */
export async function getQuickSendItems(baseURL: string): Promise<{ id: number; label: string; command: string }[]> {
  const response = await fetch(`${baseURL}/api/chat/quick-send`)
  if (!response.ok) {
    throw new Error(`Failed to get quick-send items: ${response.status}`)
  }
  return response.json()
}

/**
 * Delete all quick-send items via API.
 */
export async function clearQuickSendItems(baseURL: string): Promise<void> {
  const items = await getQuickSendItems(baseURL)
  for (const item of items) {
    await fetch(`${baseURL}/api/chat/quick-send/${item.id}`, { method: 'DELETE' })
  }
}

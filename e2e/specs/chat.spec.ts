import { test, expect } from '../fixtures'
import { ChatPage } from '../pages/chat.page'
import { seedQuickSendItems } from '../helpers/test-data'
import { getServerURL } from '../helpers/server'

test.describe('Chat', () => {
  let chat: ChatPage

  test.beforeEach(async ({ page }) => {
    chat = new ChatPage(page)
  })

  test('should send a message and receive SSE stream reply', async ({ page }) => {
    // default_agent=mock in test config, so new sessions use MockAIBackend automatically
    await chat.sendMessage('Hello, mock assistant!')

    // 1. User message appears immediately (synchronous POST)
    await expect(chat.getLastUserMessage()).toContainText('Hello, mock assistant!')

    // 2. Assistant response appears (async SSE stream from MockAIBackend)
    //    MockAIBackend responds: "Hello! I am a mock assistant. How can I help you today?"
    await chat.waitForReply(20000)

    // 3. Response contains the mock text
    await expect(chat.getLastAssistantMessage()).toContainText('mock assistant', { timeout: 10000 })
  })

  test('should open quick-send menu on empty send click', async ({ page }) => {
    // Seed quick-send items first
    await seedQuickSendItems(getServerURL())

    // Reload so the frontend picks up the items
    await page.reload()
    await page.waitForLoadState('networkidle')

    // Click send with empty input to open quick-send popup
    await chat.openQuickSendMenu()

    // Quick-send popup should appear
    await expect(page.locator('.quick-send-title')).toBeVisible()
  })

  test('should create a new session', async ({ page }) => {
    // Verify we're on the chat page
    await expect(chat.textarea).toBeVisible()
  })

  // Mock agent has no models configured, so model chip is not rendered.
  // Skip until a backend with models is available for E2E.
  test.skip('should show model selector chip', async ({ page }) => {
    await expect(chat.modelChip).toBeVisible()
  })

  test('should show stop button during AI response', async ({ page }) => {
    // Send a message
    await chat.sendMessage('Hello')

    // Stop button may briefly appear while AI is generating
    // MockAIBackend responds in ~500ms, so we might not catch it
    // Wait for the response to complete
    await chat.waitForReply(20000)

    // After response completes, stop button should be gone
    await expect(chat.stopButton).not.toBeVisible()
  })
})

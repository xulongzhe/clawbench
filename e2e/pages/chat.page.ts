import { type Locator, type Page, expect } from '@playwright/test'

/**
 * Page Object Model for the Chat panel.
 *
 * Key selectors (from ChatInputBar.vue, ChatMessageItem.vue, ChatMessageList.vue):
 * - .chat-textarea       → message input textarea
 * - .chat-send-btn        → send button (hidden during loading)
 * - .chat-stop-btn        → stop/cancel button (visible during loading)
 * - .quick-send-title     → quick-send popup title
 * - .model-chip           → model selector chip
 * - .thinking-effort-chip → thinking effort selector chip
 * - .chat-messages        → messages scroll container
 * - .chat-message.user    → user message
 * - .chat-message.assistant → AI assistant message
 * - .chat-action-btn      → session management buttons (sessions, new, delete, speech)
 */
export class ChatPage {
  readonly page: Page
  readonly textarea: Locator
  readonly sendButton: Locator
  readonly stopButton: Locator
  readonly messagesContainer: Locator
  readonly modelChip: Locator

  constructor(page: Page) {
    this.page = page
    this.textarea = page.locator('.chat-textarea')
    this.sendButton = page.locator('.chat-send-btn')
    this.stopButton = page.locator('.chat-stop-btn')
    this.messagesContainer = page.locator('.chat-messages')
    this.modelChip = page.locator('.model-chip')
  }

  /** Fill the textarea with text */
  async fillInput(text: string) {
    await this.textarea.fill(text)
  }

  /** Clear the textarea */
  async clearInput() {
    await this.textarea.clear()
  }

  /** Fill the textarea and click send */
  async sendMessage(text: string) {
    await this.textarea.fill(text)
    // Brief pause to let Vue's v-model react to the filled value.
    // Without this, Firefox/WebKit may fire the click before the
    // framework has processed the input event, sending an empty message.
    await this.page.waitForTimeout(100)
    await this.sendButton.click()
  }

  /** Click send with empty input to open quick-send popup */
  async openQuickSendMenu() {
    await this.sendButton.click()
  }

  /** Wait for an assistant message to fully load (SSE stream reply with content) */
  async waitForReply(timeout = 15000) {
    // Wait for assistant message element to appear AND have non-empty text content.
    // SSE streams content incrementally, so we must wait until text is actually rendered.
    const assistantMsg = this.page.locator('.chat-message.assistant').last()
    await expect(assistantMsg).toBeVisible({ timeout })
    // Wait for actual text content (the mock response contains "mock assistant")
    await expect(assistantMsg).not.toBeEmpty({ timeout })
  }

  /** Get the last user message element */
  getLastUserMessage(): Locator {
    return this.page.locator('.chat-message.user').last()
  }

  /** Get the last assistant message element */
  getLastAssistantMessage(): Locator {
    return this.page.locator('.chat-message.assistant').last()
  }

  /** Click the new session button (the "+" button in chat action row) */
  async createSession() {
    // The second .chat-action-btn is the "new session" button
    await this.page.locator('.chat-action-btn').nth(1).click()
  }

  /** Click the sessions list button (the first .chat-action-btn) */
  async openSessionList() {
    await this.page.locator('.chat-action-btn').first().click()
  }
}

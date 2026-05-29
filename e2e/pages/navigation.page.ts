import { type Locator, type Page, expect } from '@playwright/test'

/**
 * Page Object Model for tab/drawer navigation.
 *
 * Dock buttons inside .dock-center (ordered by position):
 *   [0] Chat           (inside .dock-btn-wrap)
 *   [1] File Viewer    (direct .dock-btn)
 *   [2] File Manager   (direct .dock-btn, switches to 'browse' tab)
 *   [3] Tasks           (inside .dock-btn-wrap)
 *
 * Titles come from i18n and vary by locale (Chat/会话, File Manager/文件管理器, etc.),
 * so we use positional selectors in .dock-center for locale independence.
 */
export class NavigationPage {
  readonly page: Page
  private readonly dockBtns: Locator

  constructor(page: Page) {
    this.page = page
    this.dockBtns = page.locator('.dock-center .dock-btn')
  }

  // --- Tab switching via position (locale-independent) ---

  /** Get the Chat dock button (1st .dock-btn in .dock-center) */
  private get chatBtn(): Locator {
    return this.dockBtns.nth(0)
  }

  /** Get the File Viewer dock button (2nd .dock-btn in .dock-center) */
  private get viewerBtn(): Locator {
    return this.dockBtns.nth(1)
  }

  /** Get the File Manager / Browse dock button (3rd .dock-btn in .dock-center) */
  private get browseBtn(): Locator {
    return this.dockBtns.nth(2)
  }

  /** Get the Tasks dock button (4th .dock-btn in .dock-center) */
  private get tasksBtn(): Locator {
    return this.dockBtns.nth(3)
  }

  /** Switch to Chat tab */
  async switchToChat() {
    await this.chatBtn.click()
  }

  /** Switch to File Viewer tab */
  async switchToViewer() {
    await this.viewerBtn.click()
  }

  /** Switch to File Manager (Browse) tab */
  async switchToFileManager() {
    await this.browseBtn.click()
  }

  /** Switch to Tasks tab */
  async switchToTasks() {
    await this.tasksBtn.click()
  }

  // --- Overflow menu ---

  /** Open the overflow menu (3-dot button) */
  async openOverflowMenu() {
    await this.page.locator('.dock-overflow-btn').click()
    await expect(this.page.locator('.dock-overflow-popup')).toBeVisible()
  }

  /** Switch to History tab (via overflow menu) */
  async switchToHistory() {
    await this.openOverflowMenu()
    await this.page.locator('.dock-overflow-item', { hasText: /History|历史/ }).click()
  }

  /** Switch to Terminal tab (via overflow menu) */
  async switchToTerminal() {
    await this.openOverflowMenu()
    await this.page.locator('.dock-overflow-item', { hasText: /Terminal|终端/ }).click()
  }

  /** Open Settings (via overflow menu) */
  async openSettings() {
    await this.openOverflowMenu()
    await this.page.locator('.dock-overflow-item', { hasText: /Settings|设置/ }).click()
  }

  // --- Assertions ---

  /** Assert that the chat tab is active */
  async expectChatActive() {
    await expect(this.chatBtn).toHaveClass(/active/)
  }
}

import { type Locator, type Page, expect } from '@playwright/test'

/**
 * Page Object Model for the File Manager panel.
 *
 * Key selectors (from FileManagerContent.vue, FileViewer.vue):
 * - .file-list    → file list container
 * - .file-item    → individual file or directory item
 * - .dir-item     → directory item (also has .file-item)
 * - .file-viewer  → file content viewer
 */
export class FileManagerPage {
  readonly page: Page
  readonly fileList: Locator
  readonly fileViewer: Locator

  constructor(page: Page) {
    this.page = page
    this.fileList = page.locator('.file-list')
    this.fileViewer = page.locator('.file-viewer')
  }

  /** Get a file item by name */
  getFileItem(name: string): Locator {
    return this.page.locator('.file-item', { hasText: name })
  }

  /** Get the first directory item */
  getFirstDirectory(): Locator {
    return this.page.locator('.file-item.dir-item').first()
  }

  /** Get the first regular file item */
  getFirstFile(): Locator {
    return this.page.locator('.file-item:not(.dir-item)').first()
  }

  /** Navigate into a directory by clicking it */
  async openDirectory(name: string) {
    await this.getFileItem(name).click()
  }

  /** Open a file in the viewer by double-clicking it */
  async openFile(name: string) {
    await this.getFileItem(name).dblclick()
  }

  /** Expect a file/directory with the given name to be visible */
  async expectFileVisible(name: string) {
    await expect(this.getFileItem(name)).toBeVisible()
  }

  /** Expect the file viewer to be visible */
  async expectFileViewerVisible() {
    await expect(this.fileViewer).toBeVisible()
  }
}

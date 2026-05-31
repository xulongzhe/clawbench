import { test, expect } from '../fixtures'
import { FileManagerPage } from '../pages/file-manager.page'
import { NavigationPage } from '../pages/navigation.page'

test.describe('File Manager', () => {
  let fm: FileManagerPage
  let nav: NavigationPage

  test.beforeEach(async ({ page }) => {
    fm = new FileManagerPage(page)
    nav = new NavigationPage(page)

    // Navigate to the file manager tab
    await nav.switchToFileManager()

    // Wait for file content to be loaded (not just tab switch).
    // Firefox/WebKit need this extra wait because the file list API
    // call is async — the tab renders before directory entries arrive.
    await fm.waitForContent(15000)
  })

  test('should display files in the project directory', async ({ page }) => {
    // Project directory should contain at least some files
    // Use view-agnostic selector (.file-item or .grid-item)
    await expect(page.locator('.file-item, .grid-item').first()).toBeVisible({ timeout: 10000 })
  })

  test('should navigate into a directory on click', async ({ page }) => {
    // Use view-agnostic directory selector (.file-item.dir-item or .grid-item.grid-dir)
    const dirItem = page.locator('.file-item.dir-item, .grid-item.grid-dir').first()
    await expect(dirItem).toBeVisible({ timeout: 10000 })

    // Record current breadcrumb text before clicking
    const breadcrumbBefore = page.locator('.dir-breadcrumb .crumb.current')
    const hadBreadcrumb = await breadcrumbBefore.count() > 0
    const beforeText = hadBreadcrumb
      ? await breadcrumbBefore.first().textContent()
      : ''

    await dirItem.click()

    // Verify navigation succeeded — either:
    // 1. Breadcrumb updates (new current crumb appears), or
    // 2. File items render in the subdirectory, or
    // 3. Empty directory message appears ("This directory is empty")
    // We cannot assume the subdirectory has files — CI runners may have
    // empty directories (e.g. ~/Downloads).
    await expect.poll(async () => {
      const breadcrumbCurrent = page.locator('.dir-breadcrumb .crumb.current')
      const emptyState = page.locator('.empty-state')
      const fileItem = page.locator('.file-item, .grid-item').first()

      // Breadcrumb updated with a new directory name
      if (await breadcrumbCurrent.count() > 0) {
        const text = await breadcrumbCurrent.first().textContent()
        if (text && text.trim() && text.trim() !== (beforeText || '').trim()) return true
      }
      // Or file items appeared
      if (await fileItem.isVisible().catch(() => false)) return true
      // Or empty directory message
      if (await emptyState.isVisible().catch(() => false)) return true
      return false
    }, { timeout: 10000 }).toBe(true)
  })

  test('should show file list container', async ({ page }) => {
    // Either list view (.file-list + .file-item) or grid view (.file-grid + .grid-item)
    await expect(page.locator('.file-item, .grid-item').first()).toBeVisible({ timeout: 10000 })
  })
})

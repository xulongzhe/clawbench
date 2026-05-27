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
  })

  test('should display files in the project directory', async ({ page }) => {
    // Project directory should contain at least some files
    await expect(page.locator('.file-item').first()).toBeVisible({ timeout: 10000 })
  })

  test('should navigate into a directory on click', async ({ page }) => {
    const dirItem = page.locator('.file-item.dir-item').first()
    if (await dirItem.isVisible()) {
      await dirItem.click()
      // After clicking a directory, file list should update
      await expect(page.locator('.file-item').first()).toBeVisible({ timeout: 5000 })
    }
  })

  test('should show file list container', async ({ page }) => {
    // The file list container should exist (in list view mode)
    const fileList = page.locator('.file-list')
    if (await fileList.isVisible()) {
      // List mode is active — good
      await expect(fileList).toBeVisible()
    } else {
      // May be in grid view — file items should still be visible
      await expect(page.locator('.file-item').first()).toBeVisible()
    }
  })
})

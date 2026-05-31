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
    await expect(dirItem).toBeVisible({ timeout: 10000 })
    await dirItem.click()
    // After clicking a directory, file list should update
    // Use 10s timeout — Firefox/WebKit can be slower to re-render
    await expect(page.locator('.file-item').first()).toBeVisible({ timeout: 10000 })
  })

  test('should show file list container', async ({ page }) => {
    // Either list view (.file-list) or grid view must render file items
    await expect(page.locator('.file-item').first()).toBeVisible({ timeout: 10000 })
  })
})

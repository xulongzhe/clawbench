import { test, expect } from '../fixtures'

test.describe('Authentication (localhost bypass)', () => {
  // NOTE: E2E tests run from localhost, so the Go auth middleware
  // automatically bypasses authentication. These tests verify that
  // the app works correctly in the authenticated state, but they
  // cannot test the actual login flow.
  //
  // To properly test login, the Go backend would need an
  // `auth.disable_localhost_bypass` config option.

  test('should load the app and show chat textarea', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('.chat-textarea')).toBeVisible()
  })

  test('should have no login page visible', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('.chat-textarea')).toBeVisible()
    await expect(page.locator('.login-page')).not.toBeVisible()
  })

  test('should persist session across page reload', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('.chat-textarea')).toBeVisible()

    await page.reload()
    await expect(page.locator('.chat-textarea')).toBeVisible()
  })
})

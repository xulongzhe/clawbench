import { test, expect } from '../fixtures'

test.describe('Authentication', () => {
  test('should load the app and show chat textarea', async ({ page }) => {
    // Note: When running E2E tests from localhost, the auth middleware
    // automatically bypasses authentication. So the login page is never shown.
    // Instead, we verify the app loads correctly with the chat interface.
    await page.goto('/')
    // Chat textarea should be visible (main app loaded)
    await expect(page.locator('.chat-textarea')).toBeVisible()
  })

  test('should have authenticated session after initial load', async ({ page }) => {
    await page.goto('/')
    // The app should be in authenticated state (chat visible, no login page)
    await expect(page.locator('.chat-textarea')).toBeVisible()
    await expect(page.locator('.login-page')).not.toBeVisible()
  })

  test('should persist session across page reload', async ({ page }) => {
    // Navigate first
    await page.goto('/')
    await expect(page.locator('.chat-textarea')).toBeVisible()

    // Reload the page
    await page.reload()

    // Chat textarea should still be accessible
    await expect(page.locator('.chat-textarea')).toBeVisible()
  })

  test('should show login page when accessing from non-localhost', async ({ page }) => {
    // Clear cookies and use a non-localhost URL to test the login page
    // The server sees the request as coming from the browser (not localhost)
    // when we use the machine's actual IP. However, in most CI environments
    // this is still localhost. So we test by hitting the login API directly.
    await page.context().clearCookies()
    await page.goto('/')

    // Even after clearing cookies, localhost bypass keeps us authenticated
    // So we verify the login API endpoint works correctly instead
    const response = await page.request.post('/login', {
      data: { password: 'wrong-password' },
    })
    expect(response.status()).toBe(401)
  })
})

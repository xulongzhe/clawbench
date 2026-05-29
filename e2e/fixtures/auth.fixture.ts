import { test as base, expect } from '@playwright/test'

const E2E_PASSWORD = process.env.E2E_PASSWORD || 'e2e-test-password'

/**
 * Auth fixture: automatically logs in before each test.
 *
 * Strategy:
 * 1. Navigate to the app root
 * 2. If login page is shown (401 or redirect to /login), fill password and submit
 * 3. If already authenticated (session cookie present), skip login
 *
 * The session cookie (clawbench_session) persists across navigations within
 * the same browser context, so subsequent tests reuse the same session.
 *
 * NOTE: In current E2E setup, the Go server's auth middleware automatically
 * bypasses authentication for localhost requests. Therefore `needsLogin` is
 * always false and the login code path (lines 29-42) is unreachable. This
 * code is kept for future use when localhost bypass may be disabled for
 * proper auth E2E testing. See auth.spec.ts for details.
 */
export const test = base.extend({
  page: async ({ page }, use) => {
    // Navigate to the app root
    const response = await page.goto('/')

    // Check if we need to log in
    // The server returns 401 for unauthenticated requests, or the SPA
    // shows the LoginView component when not authenticated
    const needsLogin =
      response?.status() === 401 ||
      page.url().includes('/login') ||
      await page.locator('.login-page').isVisible().catch(() => false)

    if (needsLogin) {
      // Make sure we're on the login page
      if (!page.url().includes('/login') && !await page.locator('.login-page').isVisible().catch(() => false)) {
        await page.goto('/')
        await page.waitForLoadState('networkidle')
      }

      // Fill password and submit
      await page.locator('.login-page input[type="password"]').fill(E2E_PASSWORD)
      await page.locator('.login-btn').click()

      // Wait for the main app to load (login page should disappear)
      await expect(page.locator('.login-page')).not.toBeVisible({ timeout: 10000 })
    }

    await use(page)
  },
})

export { expect }

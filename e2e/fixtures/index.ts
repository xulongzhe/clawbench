import { mergeTests } from '@playwright/test'
import { test as authTest } from './auth.fixture'
import { test as coverageTest } from './coverage.fixture'

/**
 * Merged test fixture: combines auth (auto-login) and coverage collection.
 *
 * Usage in test files:
 *   import { test, expect } from '../fixtures'
 *   test('my test', async ({ page }) => { ... })
 *
 * The auth fixture ensures the user is logged in before each test.
 * The coverage fixture auto-collects V8 coverage on Chromium tests.
 */
export const test = mergeTests(authTest, coverageTest)
export { expect } from '@playwright/test'

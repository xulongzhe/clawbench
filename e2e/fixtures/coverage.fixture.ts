import { test as base } from '@playwright/test'
import { writeFileSync, mkdirSync } from 'fs'
import { join } from 'path'

const COVERAGE_DIR = '.nyc_output'

/**
 * Coverage fixture: collects V8 JS coverage on Chromium-only test runs.
 *
 * For each test:
 * 1. Before navigation: start JS coverage collection via page.coverage API
 * 2. After test: stop coverage, convert V8 → Istanbul via v8-to-istanbul
 * 3. Write per-test Istanbul JSON to .nyc_output/ directory
 *
 * Only runs on the 'chromium-coverage' project (projectName === 'chromium-coverage').
 * Other Chromium-based projects, Firefox, and WebKit tests run without coverage collection.
 *
 * After all tests complete, run `npx nyc report` to generate the combined report.
 */
export const test = base.extend({
  coverageCollector: [async ({ page, projectName }, use, testInfo) => {
    // Only collect coverage on the chromium-coverage project
    if (projectName !== 'chromium-coverage') {
      await use(null)
      return
    }

    // Start JS coverage before the test runs
    await page.coverage.startJSCoverage({
      reportAnonymousScripts: true,
    })

    await use(null)

    // Stop coverage and convert to Istanbul format
    const jsCoverage = await page.coverage.stopJSCoverage()

    for (const entry of jsCoverage) {
      // Only convert app code (skip node_modules, vendors, third-party)
      if (!entry.url.includes('/src/') && !entry.url.includes('/assets/')) continue

      try {
        // Dynamic import for v8-to-istanbul (ESM-only package)
        const { default: V8ToIstanbul } = await import('v8-to-istanbul')

        const converter = new V8ToIstanbul(entry.url, 0, { source: entry.source })
        await converter.load()
        converter.applyCoverage(entry.functions)

        const istanbulData = converter.toIstanbul()
        const filename = `e2e-${testInfo.testId}-${entry.url.replace(/[^a-z0-9]/gi, '_')}.json`
        mkdirSync(COVERAGE_DIR, { recursive: true })
        writeFileSync(join(COVERAGE_DIR, filename), JSON.stringify(istanbulData))
      } catch (err) {
        // Coverage conversion failure should not fail the test
        console.warn(`Failed to convert coverage for ${entry.url}:`, err)
      }
    }
  }, { auto: true, scope: 'test' }],
})

export { expect } from '@playwright/test'

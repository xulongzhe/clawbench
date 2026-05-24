import { describe, expect, it } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

/**
 * ISS-021: Verify that the HTML iframe sandbox attribute does NOT include
 * "allow-same-origin" together with "allow-scripts", as that combination
 * defeats the sandbox protection.
 */
describe('FileViewer iframe sandbox (ISS-021)', () => {
  const componentPath = resolve(__dirname, '../file/FileViewer.vue')
  const source = readFileSync(componentPath, 'utf-8')

  it('should not have allow-same-origin in the iframe sandbox attribute', () => {
    // Find the sandbox attribute on the iframe element
    const sandboxMatch = source.match(/sandbox="([^"]*)"/)
    expect(sandboxMatch).not.toBeNull()

    const sandboxValue = sandboxMatch![1]
    const tokens = sandboxValue.split(/\s+/).filter(Boolean)

    // Must have allow-scripts for HTML preview to work
    expect(tokens).toContain('allow-scripts')

    // Must NOT have allow-same-origin — it defeats sandbox when combined with allow-scripts
    expect(tokens).not.toContain('allow-same-origin')
  })

  it('should still have allow-scripts for HTML preview functionality', () => {
    const sandboxMatch = source.match(/sandbox="([^"]*)"/)
    expect(sandboxMatch).not.toBeNull()

    const sandboxValue = sandboxMatch![1]
    expect(sandboxValue).toContain('allow-scripts')
  })
})

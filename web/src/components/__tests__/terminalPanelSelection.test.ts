import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const terminalComponentPaths = [
  '../terminal/TerminalPanelContent.vue',
]

const readTerminalComponent = (path: string) => readFileSync(resolve(__dirname, path), 'utf8')
const readToolbarStyleBlock = (source: string) => {
  const start = source.indexOf('.terminal-toolbar {')
  const end = source.indexOf('</style>', start)

  return source.slice(start, end)
}

describe('TerminalPanel xterm selection defaults', () => {
  it('does not force xterm selection to line mode', () => {
    const source = readTerminalComponent('../terminal/TerminalPanelContent.vue')

    expect(source).not.toContain("selectionStyle: 'line'")
  })

  it('hides toolbar buttons whose actions are covered by gestures', () => {
    const source = readTerminalComponent('../terminal/TerminalPanelContent.vue')
    const gestureMappedKeys = ['Esc', 'Tab', 'Page Up', 'Page Down', '↑', '↓', '←', '→']

    for (const title of gestureMappedKeys) {
      expect(source).toContain(`title="${title}"`)
    }
    expect(source.match(/v-if="!gestures\.enabled\.value"/g)?.length).toBeGreaterThanOrEqual(4)
    expect(source).toContain('v-show="!gestures.enabled.value" class="key-group"')
  })

  it('keeps terminal virtual keys in a borderless, transparent overlay system', () => {
    for (const path of terminalComponentPaths) {
      const source = readTerminalComponent(path)
      const toolbarStyle = readToolbarStyleBlock(source)

      // Borderless: no border on buttons
      expect(toolbarStyle).toContain('border: none')
      // Transparent default background
      expect(toolbarStyle).toContain('background: transparent')
      // Hover/active use semi-transparent overlays
      expect(toolbarStyle).toContain('--toolbar-key-hover')
      expect(toolbarStyle).toContain('--toolbar-key-active')
      // Scrollbar still present
      expect(toolbarStyle).toContain('--toolbar-scrollbar-track')
      expect(toolbarStyle).toContain('--toolbar-scrollbar-thumb')
      expect(toolbarStyle).toContain('--toolbar-scrollbar-thumb-hover')
      // No decorative masks or accent colors
      expect(toolbarStyle).not.toContain('mask-image')
      expect(toolbarStyle).toContain('height: 2px')
      expect(toolbarStyle).toContain('transition: background 140ms ease')
      expect(toolbarStyle).not.toContain('var(--color-green)')
      expect(toolbarStyle).not.toContain('var(--color-yellow)')
      expect(toolbarStyle).not.toContain('var(--color-purple)')
    }
  })
})

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

describe('TerminalPanel xterm selection defaults', () => {
  it('does not force xterm selection to line mode', () => {
    const source = readFileSync(resolve(__dirname, '../terminal/TerminalPanel.vue'), 'utf8')

    expect(source).not.toContain("selectionStyle: 'line'")
  })

  it('hides toolbar buttons whose actions are covered by gestures', () => {
    const source = readFileSync(resolve(__dirname, '../terminal/TerminalPanel.vue'), 'utf8')
    const gestureMappedKeys = ['Esc', 'Tab', 'Page Up', 'Page Down', '↑', '↓', '←', '→']

    for (const title of gestureMappedKeys) {
      expect(source).toContain(`title="${title}"`)
    }
    expect(source.match(/v-if="!gestures\.enabled\.value"/g)?.length).toBeGreaterThanOrEqual(8)
  })
})

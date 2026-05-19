import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SettingsIndex from '@/components/settings/SettingsIndex.vue'

// Stub lucide-vue-next icons
const globalStubs = {
  'lucide-chevron-right': true,
  'lucide-palette': true,
  'lucide-message-square': true,
  'lucide-bot': true,
  'lucide-folder-open': true,
  'lucide-file-text': true,
  'lucide-terminal': true,
  'lucide-volume2': true,
  'lucide-brain': true,
  'lucide-network': true,
  'lucide-shield': true,
  'lucide-bell': true,
  'lucide-smartphone': true,
  'lucide-server': true,
  'lucide-info': true,
}

function mountIndex() {
  return mount(SettingsIndex, {
    global: { stubs: globalStubs },
  })
}

describe('SettingsIndex', () => {
  it('renders all 14 category rows', () => {
    const wrapper = mountIndex()

    const rows = wrapper.findAll('.settings-index__row')
    expect(rows.length).toBe(14)
  })

  it('renders category labels', () => {
    const wrapper = mountIndex()

    const labels = wrapper.findAll('.settings-index__label').map(el => el.text())
    expect(labels).toContain('外观')
    expect(labels).toContain('聊天')
    expect(labels).toContain('Agent偏好')
    expect(labels).toContain('服务器')
    expect(labels).toContain('关于')
  })

  it('emits navigate with categoryId when row clicked', async () => {
    const wrapper = mountIndex()

    const rows = wrapper.findAll('.settings-index__row')
    await rows[0].trigger('click')

    expect(wrapper.emitted('navigate')).toBeTruthy()
    expect(wrapper.emitted('navigate')![0]).toEqual(['appearance'])
  })

  it('emits correct categoryId for each row', async () => {
    const wrapper = mountIndex()

    const expectedIds = [
      'appearance', 'chat', 'agents', 'fileManager', 'fileViewer',
      'terminal', 'tts', 'rag', 'proxy', 'ssh', 'push', 'android',
      'server', 'about',
    ]

    const rows = wrapper.findAll('.settings-index__row')
    for (let i = 0; i < expectedIds.length; i++) {
      await rows[i].trigger('click')
      expect(wrapper.emitted('navigate')![i]).toEqual([expectedIds[i]])
    }
  })
})

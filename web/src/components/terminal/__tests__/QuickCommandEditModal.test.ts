import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import { createI18n } from 'vue-i18n'
import QuickCommandEditModal from '@/components/terminal/QuickCommandEditModal.vue'

// ── Mocks ────────────────────────────────────────────────────
const mockAddCommand = vi.fn()
const mockUpdateCommand = vi.fn()
const mockToastShow = vi.fn()

// Module-level ref shared across instances
const mockCommandsRef = ref<any[]>([])

vi.mock('@/composables/useQuickCommands', () => ({
  useQuickCommands: () => ({
    commands: mockCommandsRef,
    addCommand: mockAddCommand,
    updateCommand: mockUpdateCommand,
  }),
}))

vi.mock('@/composables/useToast', () => ({
  useToast: () => ({ show: mockToastShow }),
}))

// ── i18n ─────────────────────────────────────────────────────
const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: {
    zh: {
      terminal: {
        addCommand: '添加命令',
        editCommand: '编辑命令',
        commandLabel: '标签',
        commandText: '命令',
        commandHidden: '隐藏',
        commandAutoExecute: '自动执行',
        commandRequired: '必填',
        commandSaved: '已保存',
        saveFailed: '保存失败',
        autoExecuteWarning: '已有自动执行命令',
      },
      common: { cancel: '取消', save: '保存' },
    },
  },
})

beforeEach(() => {
  mockAddCommand.mockReset()
  mockUpdateCommand.mockReset()
  mockToastShow.mockReset()
  mockCommandsRef.value = []
})

function mountModal(props = {}) {
  return mount(QuickCommandEditModal, {
    props: {
      open: true,
      editingCommand: null,
      ...props,
    },
    global: {
      stubs: { teleport: true },
      plugins: [i18n],
    },
  })
}

// ── Tests ─────────────────────────────────────────────────────

describe('QuickCommandEditModal', () => {
  describe('rendering', () => {
    it('renders label input and command textarea when open', () => {
      const wrapper = mountModal()

      expect(wrapper.find('input.form-input').exists()).toBe(true)
      expect(wrapper.find('textarea.form-input.form-textarea').exists()).toBe(true)
    })

    it('renders textarea with rows=4 for command field', () => {
      const wrapper = mountModal()

      const textarea = wrapper.find('textarea.form-textarea')
      expect(textarea.exists()).toBe(true)
      expect(textarea.attributes('rows')).toBe('4')
    })

    it('renders hidden and auto_execute checkboxes', () => {
      const wrapper = mountModal()

      const checkboxes = wrapper.findAll('input[type="checkbox"]')
      expect(checkboxes).toHaveLength(2)
    })

    it('does not show auto-execute warning when no existing auto-exec command', () => {
      const wrapper = mountModal()

      expect(wrapper.find('.form-hint').exists()).toBe(false)
    })
  })

  describe('form behavior', () => {
    it('pre-fills form when editingCommand is provided', async () => {
      // Mount closed, then open — watch triggers on open change
      const wrapper = mount(QuickCommandEditModal, {
        props: { open: false, editingCommand: null },
        global: { stubs: { teleport: true }, plugins: [i18n] },
      })
      await wrapper.setProps({
        open: true,
        editingCommand: {
          id: 1, label: 'ls', command: 'ls -la', hidden: true, auto_execute: false, sort_order: 0,
        },
      })
      await nextTick()

      expect(wrapper.find('input.form-input').element.value).toBe('ls')
      expect(wrapper.find('textarea.form-textarea').element.value).toBe('ls -la')
    })

    it('resets form when dialog opens with no editingCommand', async () => {
      const wrapper = mountModal()
      await nextTick()

      expect(wrapper.find('input.form-input').element.value).toBe('')
      expect(wrapper.find('textarea.form-textarea').element.value).toBe('')
    })

    it('shows validation error when saving with empty fields', async () => {
      const wrapper = mountModal()

      await wrapper.find('.modal-btn.primary').trigger('click')
      await nextTick()

      expect(wrapper.find('.form-error').exists()).toBe(true)
    })

    it('calls addCommand when saving new command', async () => {
      mockAddCommand.mockResolvedValue(true)
      const wrapper = mountModal()

      await wrapper.find('input.form-input').setValue('grep')
      await wrapper.find('textarea.form-textarea').setValue('grep -r "test" .')
      await wrapper.find('.modal-btn.primary').trigger('click')
      await nextTick()

      expect(mockAddCommand).toHaveBeenCalledWith(
        expect.objectContaining({ label: 'grep', command: 'grep -r "test" .' }),
      )
    })

    it('calls updateCommand when saving edited command', async () => {
      mockUpdateCommand.mockResolvedValue(true)
      // Mount closed, then open with editingCommand
      const wrapper = mount(QuickCommandEditModal, {
        props: { open: false, editingCommand: null },
        global: { stubs: { teleport: true }, plugins: [i18n] },
      })
      await wrapper.setProps({
        open: true,
        editingCommand: {
          id: 3, label: 'ls', command: 'ls', hidden: false, auto_execute: false, sort_order: 0,
        },
      })
      await nextTick()

      await wrapper.find('textarea.form-textarea').setValue('ls -la')
      await wrapper.find('.modal-btn.primary').trigger('click')
      await nextTick()

      expect(mockUpdateCommand).toHaveBeenCalledWith(3, expect.objectContaining({ command: 'ls -la' }))
    })

    it('emits saved on successful save', async () => {
      mockAddCommand.mockResolvedValue(true)
      const wrapper = mountModal()

      await wrapper.find('input.form-input').setValue('ls')
      await wrapper.find('textarea.form-textarea').setValue('ls')
      await wrapper.find('.modal-btn.primary').trigger('click')
      await nextTick()

      expect(wrapper.emitted('saved')).toBeTruthy()
    })

    it('shows error when save API fails', async () => {
      mockAddCommand.mockResolvedValue(false)
      const wrapper = mountModal()

      await wrapper.find('input.form-input').setValue('ls')
      await wrapper.find('textarea.form-textarea').setValue('ls')
      await wrapper.find('.modal-btn.primary').trigger('click')
      await nextTick()

      expect(wrapper.find('.form-error').exists()).toBe(true)
      expect(wrapper.emitted('saved')).toBeFalsy()
    })

    it('emits close when cancel button is clicked', async () => {
      const wrapper = mountModal()

      await wrapper.find('.modal-btn:not(.primary)').trigger('click')

      expect(wrapper.emitted('close')).toBeTruthy()
    })

    it('shows auto-execute warning when another command has auto_execute', async () => {
      mockCommandsRef.value = [
        { id: 1, label: 'cd', command: 'cd ~', hidden: false, auto_execute: true, sort_order: 0 },
      ]
      const wrapper = mountModal()

      // Check the auto_execute checkbox (second checkbox)
      const checkboxes = wrapper.findAll('input[type="checkbox"]')
      await checkboxes[1].setValue(true)
      await nextTick()

      expect(wrapper.find('.form-hint').exists()).toBe(true)
    })
  })
})

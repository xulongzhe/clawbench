import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import SettingsItem from '@/components/settings/SettingsItem.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: {
    zh: {
      common: { ok: '确定' },
      settings: { needsRestart: '需重启' },
    },
  },
})

function mountItem(props: Record<string, any> = {}) {
  return mount(SettingsItem, {
    props: { label: 'Test Item', type: 'switch', ...props },
    global: { plugins: [i18n] },
  })
}

describe('SettingsItem', () => {
  it('renders switch type with checkbox', () => {
    const wrapper = mountItem({ type: 'switch', modelValue: true })

    const checkbox = wrapper.find('input[type="checkbox"]')
    expect(checkbox.exists()).toBe(true)
    expect((checkbox.element as HTMLInputElement).checked).toBe(true)
  })

  it('renders select type with current value displayed', () => {
    const wrapper = mountItem({
      type: 'select',
      modelValue: 'dark',
      options: [
        { label: 'Light', value: 'light' },
        { label: 'Dark', value: 'dark' },
      ],
    })

    expect(wrapper.find('.settings-item__value').text()).toBe('Dark')
    expect(wrapper.find('.settings-item__arrow').exists()).toBe(true)
  })

  it('renders number type with value displayed', () => {
    const wrapper = mountItem({ type: 'number', modelValue: 42 })

    expect(wrapper.find('.settings-item__value').text()).toBe('42')
    expect(wrapper.find('.settings-item__arrow').exists()).toBe(true)
  })

  it('renders needsRestart badge when true', () => {
    const wrapper = mountItem({ type: 'switch', needsRestart: true })

    expect(wrapper.find('.settings-item__badge').exists()).toBe(true)
    expect(wrapper.find('.settings-item__badge').text()).toBe('需重启')
  })

  it('does not render needsRestart badge when false/undefined', () => {
    const wrapper = mountItem({ type: 'switch' })

    expect(wrapper.find('.settings-item__badge').exists()).toBe(false)

    const wrapper2 = mountItem({ type: 'switch', needsRestart: false })
    expect(wrapper2.find('.settings-item__badge').exists()).toBe(false)
  })

  it('emits update:modelValue when switch toggled', async () => {
    const wrapper = mountItem({ type: 'switch', modelValue: false })

    await wrapper.find('input[type="checkbox"]').setValue(true)

    expect(wrapper.emitted('update:modelValue')).toBeTruthy()
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([true])
  })

  it('emits click when action type clicked', async () => {
    const wrapper = mountItem({ type: 'action' })

    await wrapper.find('.settings-item').trigger('click')

    expect(wrapper.emitted('click')).toBeTruthy()
    expect(wrapper.emitted('click')!.length).toBe(1)
  })

  // Inline editor tests
  it('opens select editor on click and emits value on option select', async () => {
    const wrapper = mountItem({
      type: 'select',
      modelValue: 'light',
      options: [
        { label: 'Light', value: 'light' },
        { label: 'Dark', value: 'dark' },
      ],
    })

    // No editor initially
    expect(wrapper.find('.settings-item__editor').exists()).toBe(false)

    // Click row to open editor
    await wrapper.find('.settings-item').trigger('click')
    expect(wrapper.find('.settings-item__editor').exists()).toBe(true)
    expect(wrapper.findAll('.settings-item__option').length).toBe(2)

    // Click an option
    await wrapper.findAll('.settings-item__option')[1].trigger('click')

    expect(wrapper.emitted('update:modelValue')).toBeTruthy()
    expect(wrapper.emitted('update:modelValue')![0]).toEqual(['dark'])
    // Editor should close after selecting
    expect(wrapper.find('.settings-item__editor').exists()).toBe(false)
  })

  it('opens number editor on click and emits value on confirm', async () => {
    const wrapper = mountItem({ type: 'number', modelValue: 42 })

    // Click row to open editor
    await wrapper.find('.settings-item').trigger('click')
    expect(wrapper.find('.settings-item__editor').exists()).toBe(true)
    expect(wrapper.find('input[type="number"]').exists()).toBe(true)

    // Change value and confirm
    await wrapper.find('input[type="number"]').setValue(80)
    await wrapper.find('.settings-item__editor-confirm').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toBeTruthy()
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([80])
    // Editor should close after confirming
    expect(wrapper.find('.settings-item__editor').exists()).toBe(false)
  })

  it('opens text editor on click and emits value on confirm', async () => {
    const wrapper = mountItem({ type: 'text', modelValue: 'hello' })

    // Click row to open editor
    await wrapper.find('.settings-item').trigger('click')
    expect(wrapper.find('.settings-item__editor').exists()).toBe(true)
    expect(wrapper.find('input[type="text"]').exists()).toBe(true)

    // Change value and confirm
    await wrapper.find('input[type="text"]').setValue('world')
    await wrapper.find('.settings-item__editor-confirm').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toBeTruthy()
    expect(wrapper.emitted('update:modelValue')![0]).toEqual(['world'])
    // Editor should close after confirming
    expect(wrapper.find('.settings-item__editor').exists()).toBe(false)
  })

  it('does not open editor for switch type', async () => {
    const wrapper = mountItem({ type: 'switch', modelValue: false })

    await wrapper.find('.settings-item').trigger('click')

    expect(wrapper.find('.settings-item__editor').exists()).toBe(false)
  })

  it('does not open editor for slider type', async () => {
    const wrapper = mountItem({ type: 'slider', modelValue: 50, min: 0, max: 100 })

    await wrapper.find('.settings-item').trigger('click')

    expect(wrapper.find('.settings-item__editor').exists()).toBe(false)
  })

  it('toggles editor open/closed on repeated clicks', async () => {
    const wrapper = mountItem({
      type: 'select',
      modelValue: 'light',
      options: [
        { label: 'Light', value: 'light' },
        { label: 'Dark', value: 'dark' },
      ],
    })

    // Open
    await wrapper.find('.settings-item').trigger('click')
    expect(wrapper.find('.settings-item__editor').exists()).toBe(true)

    // Close (toggle)
    await wrapper.find('.settings-item').trigger('click')
    expect(wrapper.find('.settings-item__editor').exists()).toBe(false)
  })
})

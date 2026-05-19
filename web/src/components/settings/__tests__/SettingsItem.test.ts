import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SettingsItem from '@/components/settings/SettingsItem.vue'

function mountItem(props: Record<string, any> = {}) {
  return mount(SettingsItem, {
    props: { label: 'Test Item', type: 'switch', ...props },
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
})

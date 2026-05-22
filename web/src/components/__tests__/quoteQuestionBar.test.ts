import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import { truncateQuoteText, canSendInput } from '@/utils/quoteQuestionUtils'
import QuoteQuestionBar from '@/components/common/QuoteQuestionBar.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: {
    en: {
      quoteBar: {
        chat: 'Chat',
        clear: 'Clear',
        placeholder: 'Ask...',
        send: 'Send',
        newSession: 'New Session',
        aiChat: 'AI Chat',
      },
    },
  },
})

describe('truncateQuoteText (pure function)', () => {
  it('returns text unchanged when under limit', () => {
    expect(truncateQuoteText('Hello world')).toBe('Hello world')
  })

  it('returns text unchanged at exact limit', () => {
    const text = 'a'.repeat(150)
    expect(truncateQuoteText(text)).toBe(text)
  })

  it('truncates and appends ellipsis when over limit', () => {
    const text = 'a'.repeat(200)
    const result = truncateQuoteText(text)
    expect(result).toBe('a'.repeat(150) + '…')
    expect(result.length).toBe(151)
  })

  it('handles empty string', () => {
    expect(truncateQuoteText('')).toBe('')
  })

  it('preserves unicode characters before truncation', () => {
    const text = '你好世界'.repeat(40)
    const result = truncateQuoteText(text)
    expect(result.endsWith('…')).toBe(true)
    expect(result.length).toBe(151)
  })

  it('handles text with newlines', () => {
    const text = 'line1\nline2\nline3\n' + 'a'.repeat(150)
    const result = truncateQuoteText(text)
    expect(result.endsWith('…')).toBe(true)
  })

  it('handles single character over limit', () => {
    const text = 'a'.repeat(151)
    expect(truncateQuoteText(text)).toBe('a'.repeat(150) + '…')
  })

  it('handles text at limit + 1', () => {
    const text = 'a'.repeat(151)
    const result = truncateQuoteText(text)
    expect(result.length).toBe(151)
    expect(result.endsWith('…')).toBe(true)
  })

  it('custom maxLen parameter', () => {
    const text = 'a'.repeat(60)
    expect(truncateQuoteText(text, 50)).toBe('a'.repeat(50) + '…')
    expect(truncateQuoteText(text, 100)).toBe(text)
  })
})

describe('canSendInput (pure function)', () => {
  it('returns false for empty string', () => {
    expect(canSendInput('')).toBe(false)
  })

  it('returns false for whitespace-only string', () => {
    expect(canSendInput('   ')).toBe(false)
  })

  it('returns true for non-empty trimmed string', () => {
    expect(canSendInput('hello')).toBe(true)
  })

  it('returns true for string with leading/trailing whitespace', () => {
    expect(canSendInput('  hello  ')).toBe(true)
  })

  it('returns true for single character', () => {
    expect(canSendInput('a')).toBe(true)
  })

  it('returns false for newline-only string', () => {
    expect(canSendInput('\n')).toBe(false)
  })

  it('returns true for string with content and newlines', () => {
    expect(canSendInput('\nhello\n')).toBe(true)
  })

  it('returns false for tab-only string', () => {
    expect(canSendInput('\t')).toBe(false)
  })

  it('returns false for mixed whitespace string', () => {
    expect(canSendInput(' \n\t ')).toBe(false)
  })
})

describe('QuoteQuestionBar component', () => {
  function mountBar(props = {}) {
    return mount(QuoteQuestionBar, {
      props: {
        visible: true,
        quoteData: { text: 'Hello world' },
        ...props,
      },
      global: {
        plugins: [i18n],
        stubs: {
          HeaderMarquee: true,
          'lucide-vue-next': true,
        },
      },
    })
  }

  it('renders collapsed bar when visible with quoteData', () => {
    const wrapper = mountBar()
    expect(wrapper.find('.quote-question-bar').exists()).toBe(true)
    expect(wrapper.find('.quote-bar-row').exists()).toBe(true)
  })

  it('does not render when visible is false', () => {
    const wrapper = mountBar({ visible: false })
    expect(wrapper.find('.quote-question-bar').exists()).toBe(false)
  })

  it('does not render when quoteData is null', () => {
    const wrapper = mountBar({ quoteData: null })
    expect(wrapper.find('.quote-question-bar').exists()).toBe(false)
  })

  it('displays truncated quote text in collapsed mode', () => {
    const longText = 'a'.repeat(200)
    const wrapper = mountBar({ quoteData: { text: longText } })
    const textEl = wrapper.find('.qq-quoted-text--single')
    expect(textEl.text()).toBe('a'.repeat(150) + '…')
  })

  it('emits pin and expands when collapsed bar is clicked', async () => {
    const wrapper = mountBar()
    await wrapper.find('.quote-bar-row').trigger('click')
    await nextTick()
    expect(wrapper.emitted('pin')).toBeTruthy()
    expect(wrapper.find('.quote-bar-expanded').exists()).toBe(true)
  })

  it('shows session label in expanded mode', async () => {
    const wrapper = mountBar({ sessionLabel: 'GPT-4', sessionTitle: 'Chat Session' })
    await wrapper.find('.quote-bar-row').trigger('click')
    await nextTick()
    expect(wrapper.find('.qq-session-label').text()).toBe('GPT-4')
  })

  it('send button is disabled when input is empty', async () => {
    const wrapper = mountBar()
    await wrapper.find('.quote-bar-row').trigger('click')
    await nextTick()
    expect(wrapper.find('.qq-send-btn').classes()).toContain('disabled')
  })

  it('send button is enabled when input has text', async () => {
    const wrapper = mountBar()
    await wrapper.find('.quote-bar-row').trigger('click')
    await nextTick()
    const textarea = wrapper.find('.qq-textarea')
    await textarea.setValue('test message')
    await nextTick()
    expect(wrapper.find('.qq-send-btn').classes()).not.toContain('disabled')
  })

  it('emits send with input text when send is clicked', async () => {
    const wrapper = mountBar()
    await wrapper.find('.quote-bar-row').trigger('click')
    await nextTick()
    await wrapper.find('.qq-textarea').setValue('my question')
    await nextTick()
    await wrapper.find('.qq-send-btn').trigger('click')
    expect(wrapper.emitted('send')).toBeTruthy()
    expect(wrapper.emitted('send')![0]).toEqual(['my question'])
  })

  it('clears input and collapses after send', async () => {
    const wrapper = mountBar()
    await wrapper.find('.quote-bar-row').trigger('click')
    await nextTick()
    await wrapper.find('.qq-textarea').setValue('test')
    await nextTick()
    await wrapper.find('.qq-send-btn').trigger('click')
    await nextTick()
    // After send, should collapse back
    expect(wrapper.find('.quote-bar-row').exists()).toBe(true)
    expect(wrapper.find('.quote-bar-expanded').exists()).toBe(false)
  })

  it('resets expanded and input when visible becomes false', async () => {
    const wrapper = mountBar()
    await wrapper.find('.quote-bar-row').trigger('click')
    await nextTick()
    expect(wrapper.find('.quote-bar-expanded').exists()).toBe(true)

    await wrapper.setProps({ visible: false })
    await nextTick()
    // When hidden, the whole bar is not rendered
    expect(wrapper.find('.quote-question-bar').exists()).toBe(false)

    // Re-show — should be collapsed again
    await wrapper.setProps({ visible: true })
    await nextTick()
    expect(wrapper.find('.quote-bar-row').exists()).toBe(true)
  })

  it('emits close when clicking outside the bar', async () => {
    const wrapper = mountBar()
    // The component registers a pointerdown listener on document.
    // Simulate clicking on a real DOM element outside the bar.
    const outsideEl = document.createElement('div')
    document.body.appendChild(outsideEl)
    const event = new PointerEvent('pointerdown', { bubbles: true })
    // Dispatch from the outside element so e.target is a proper Element
    outsideEl.dispatchEvent(event)
    await nextTick()
    expect(wrapper.emitted('close')).toBeTruthy()
    document.body.removeChild(outsideEl)
  })

  it('clears input text when clear button is clicked', async () => {
    const wrapper = mountBar()
    await wrapper.find('.quote-bar-row').trigger('click')
    await nextTick()
    await wrapper.find('.qq-textarea').setValue('test')
    await nextTick()
    await wrapper.find('.qq-clear-btn').trigger('click')
    await nextTick()
    expect(wrapper.find('.qq-send-btn').classes()).toContain('disabled')
  })

  it('emits open-sessions when session selector is clicked', async () => {
    const wrapper = mountBar()
    await wrapper.find('.quote-bar-row').trigger('click')
    await nextTick()
    await wrapper.find('.qq-session').trigger('click')
    expect(wrapper.emitted('open-sessions')).toBeTruthy()
  })
})

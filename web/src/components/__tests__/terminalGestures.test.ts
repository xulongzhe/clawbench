import { afterEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { shouldPreventTerminalContextMenu, useTerminalGestures } from '@/composables/useTerminalGestures'

function makeTouch(clientX: number, clientY: number): Touch {
  return { clientX, clientY } as Touch
}

function makeTouchEvent(
  type: string,
  touches: Touch[],
  changedTouches: Touch[] = touches
): TouchEvent & { preventDefault: ReturnType<typeof vi.fn> } {
  const event = new Event(type, { bubbles: true, cancelable: true }) as TouchEvent & { preventDefault: ReturnType<typeof vi.fn> }
  Object.defineProperty(event, 'touches', { value: touches })
  Object.defineProperty(event, 'changedTouches', { value: changedTouches })
  event.preventDefault = vi.fn()
  return event
}

function dispatchTouch(
  el: HTMLElement,
  type: string,
  touches: Touch[],
  changedTouches: Touch[] = touches
) {
  const event = makeTouchEvent(type, touches, changedTouches)
  el.dispatchEvent(event)
  return event
}

afterEach(() => {
  vi.clearAllTimers()
  vi.useRealTimers()
  document.body.innerHTML = ''
})

function setupGestures() {
  const el = document.createElement('div')
  document.body.appendChild(el)

  const sent: string[] = []
  const hints: string[] = []
  const zoomDeltas: number[] = []
  const gestures = useTerminalGestures(ref(el), {
    sendArrowUp: () => sent.push('up'),
    sendArrowDown: () => sent.push('down'),
    sendArrowLeft: () => sent.push('left'),
    sendArrowRight: () => sent.push('right'),
    sendPageUp: () => sent.push('pageup'),
    sendPageDown: () => sent.push('pagedown'),
    sendEscape: () => sent.push('escape'),
    sendTab: () => sent.push('tab'),
    onPinchZoom: (delta) => zoomDeltas.push(delta),
    onGestureHint: (symbol) => hints.push(symbol),
  })
  gestures.attach()

  return { el, sent, hints, zoomDeltas, gestures }
}

describe('useTerminalGestures', () => {
  it('prevents the native double-tap selection side effect when sending Tab', () => {
    const { el, sent, hints } = setupGestures()

    dispatchTouch(el, 'touchstart', [makeTouch(40, 40)])
    dispatchTouch(el, 'touchend', [], [makeTouch(40, 40)])
    dispatchTouch(el, 'touchstart', [makeTouch(42, 42)])
    const secondTapEnd = dispatchTouch(el, 'touchend', [], [makeTouch(42, 42)])

    expect(sent).toEqual(['tab'])
    expect(hints).toEqual(['⇥'])
    expect(secondTapEnd.preventDefault).toHaveBeenCalled()
  })

  it('does not prevent default touch handling for a short stationary tap', () => {
    const { el, sent, zoomDeltas } = setupGestures()

    dispatchTouch(el, 'touchstart', [makeTouch(60, 60)])
    const touchEnd = dispatchTouch(el, 'touchend', [], [makeTouch(60, 60)])

    expect(sent).toEqual([])
    expect(zoomDeltas).toEqual([])
    expect(touchEnd.preventDefault).not.toHaveBeenCalled()
  })

  it('maps a stationary long press to Escape in gesture mode', () => {
    vi.useFakeTimers()
    const { el, sent, hints } = setupGestures()

    const touchStart = dispatchTouch(el, 'touchstart', [makeTouch(60, 60)])
    vi.advanceTimersByTime(550)
    const touchEnd = dispatchTouch(el, 'touchend', [], [makeTouch(60, 60)])

    expect(sent).toEqual(['escape'])
    expect(hints).toEqual(['Esc'])
    expect(touchStart.preventDefault).not.toHaveBeenCalled()
    expect(touchEnd.preventDefault).toHaveBeenCalled()
  })

  it('does not send arrows if the finger moves after long press already sent Escape', () => {
    vi.useFakeTimers()
    const { el, sent } = setupGestures()

    dispatchTouch(el, 'touchstart', [makeTouch(60, 60)])
    vi.advanceTimersByTime(550)
    const touchMove = dispatchTouch(el, 'touchmove', [makeTouch(110, 60)])
    dispatchTouch(el, 'touchend', [], [makeTouch(110, 60)])

    expect(sent).toEqual(['escape'])
    expect(touchMove.preventDefault).toHaveBeenCalled()
  })

  it('cancels pending long press when the browser cancels the touch sequence', () => {
    vi.useFakeTimers()
    const { el, sent } = setupGestures()

    dispatchTouch(el, 'touchstart', [makeTouch(60, 60)])
    dispatchTouch(el, 'touchcancel', [], [makeTouch(60, 60)])
    vi.advanceTimersByTime(550)

    expect(sent).toEqual([])
  })

  it('prevents native selection/scroll only after a swipe gesture is recognized', () => {
    const { el, sent } = setupGestures()

    dispatchTouch(el, 'touchstart', [makeTouch(100, 100)])
    const smallMove = dispatchTouch(el, 'touchmove', [makeTouch(108, 102)])
    const swipeMove = dispatchTouch(el, 'touchmove', [makeTouch(150, 102)])

    expect(sent).toEqual(['right'])
    expect(smallMove.preventDefault).not.toHaveBeenCalled()
    expect(swipeMove.preventDefault).toHaveBeenCalled()
  })

  it('prevents native pinch handling while applying terminal zoom', () => {
    const { el, zoomDeltas } = setupGestures()

    const pinchStart = dispatchTouch(el, 'touchstart', [makeTouch(0, 0), makeTouch(20, 0)])
    const pinchMove = dispatchTouch(el, 'touchmove', [makeTouch(0, 0), makeTouch(40, 0)])

    expect(zoomDeltas).toEqual([2])
    expect(pinchStart.preventDefault).toHaveBeenCalled()
    expect(pinchMove.preventDefault).toHaveBeenCalled()
  })

  it('maps two-finger upward and downward swipes to Page Up and Page Down', () => {
    const { el, sent, hints, zoomDeltas } = setupGestures()

    const upStart = dispatchTouch(el, 'touchstart', [makeTouch(50, 100), makeTouch(80, 100)])
    const upMove = dispatchTouch(el, 'touchmove', [makeTouch(50, 50), makeTouch(80, 50)])
    dispatchTouch(el, 'touchend', [], [makeTouch(50, 50), makeTouch(80, 50)])

    const downStart = dispatchTouch(el, 'touchstart', [makeTouch(50, 50), makeTouch(80, 50)])
    const downMove = dispatchTouch(el, 'touchmove', [makeTouch(50, 100), makeTouch(80, 100)])

    expect(sent).toEqual(['pageup', 'pagedown'])
    expect(hints).toEqual(['⇞', '⇟'])
    expect(zoomDeltas).toEqual([])
    expect(upStart.preventDefault).toHaveBeenCalled()
    expect(upMove.preventDefault).toHaveBeenCalled()
    expect(downStart.preventDefault).toHaveBeenCalled()
    expect(downMove.preventDefault).toHaveBeenCalled()
  })

  it('keeps a drifting pinch gesture in zoom mode instead of sending Page Up or Page Down', () => {
    const { el, sent, zoomDeltas } = setupGestures()

    dispatchTouch(el, 'touchstart', [makeTouch(50, 100), makeTouch(80, 100)])
    dispatchTouch(el, 'touchmove', [makeTouch(40, 65), makeTouch(100, 65)])
    dispatchTouch(el, 'touchmove', [makeTouch(40, 25), makeTouch(100, 25)])

    expect(sent).toEqual([])
    expect(zoomDeltas).toEqual([3])
  })

  it('does not treat one mostly stationary finger as a two-finger page swipe', () => {
    const { el, sent } = setupGestures()

    dispatchTouch(el, 'touchstart', [makeTouch(50, 100), makeTouch(80, 100)])
    dispatchTouch(el, 'touchmove', [makeTouch(50, 100), makeTouch(80, 30)])

    expect(sent).toEqual([])
  })

  it('restores fully native touch handling when gestures are disabled', () => {
    const { el, gestures } = setupGestures()

    gestures.toggle()

    expect(gestures.enabled.value).toBe(false)
    expect(el.style.touchAction).toBe('auto')
  })

  it('does not disable native touch selection when gestures are toggled back on', () => {
    const { el, gestures } = setupGestures()

    gestures.toggle()
    expect(gestures.enabled.value).toBe(false)
    gestures.toggle()

    expect(gestures.enabled.value).toBe(true)
    expect(el.style.touchAction).not.toBe('none')
  })
})

describe('shouldPreventTerminalContextMenu', () => {
  it('allows the native long-press copy menu when gestures are disabled', () => {
    expect(shouldPreventTerminalContextMenu(false)).toBe(false)
  })

  it('suppresses the native context menu while gestures are enabled', () => {
    expect(shouldPreventTerminalContextMenu(true)).toBe(true)
  })
})

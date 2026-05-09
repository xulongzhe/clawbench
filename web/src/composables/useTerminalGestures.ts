import { ref, type Ref } from 'vue'

export interface GestureCallbacks {
  sendArrowUp: () => void
  sendArrowDown: () => void
  sendArrowLeft: () => void
  sendArrowRight: () => void
  sendPageUp: () => void
  sendPageDown: () => void
  sendEscape: () => void
  sendTab: () => void
  onPinchZoom?: (delta: number) => void
  onGestureHint?: (symbol: string) => void
}

type Direction = 'up' | 'down' | 'left' | 'right'
type TwoFingerMode = 'pending' | 'pinch' | 'swipe'

export function shouldPreventTerminalContextMenu(gesturesEnabled: boolean): boolean {
  return gesturesEnabled
}

/**
 * Termius-style touch gestures for the terminal area.
 * - Swipe left/right → arrow left/right
 * - Swipe up/down → arrow up/down
 * - Hold direction → auto-repeat arrow keys
 * - Long-press → Esc
 * - Double-tap → Tab
 * - Pinch (two-finger) → zoom font size
 *
 * When gestures are disabled, all touch listeners are detached so that
 * xterm.js native touch selection (long-press to select) works normally.
 *
 * Gestures are bound only to the xterm container element,
 * not the entire BottomSheet, to avoid conflicting with drawer drag.
 */
export function useTerminalGestures(
  elementRef: Ref<HTMLElement | null>,
  callbacks: GestureCallbacks
) {
  const SWIPE_THRESHOLD = 30 // minimum px for a swipe
  const TWO_FINGER_SWIPE_THRESHOLD = 30 // minimum px for PgUp/PgDn
  const PINCH_THRESHOLD = 10 // minimum px change before triggering zoom
  const REPEAT_INITIAL_DELAY = 500 // ms before auto-repeat starts
  const REPEAT_INTERVAL = 150 // ms between repeated arrow keys
  const LONG_PRESS_MS = 500 // ms before long-press sends Esc
  const DOUBLE_TAP_MS = 300 // max ms between two taps for double-tap
  const TAP_THRESHOLD = 10 // max px movement to still count as a tap

  // Gesture enable/disable state
  const enabled = ref(true)
  let listenersAttached = false

  let touchStartX = 0
  let touchStartY = 0
  let isActive = false

  // Direction tracking for hold-to-repeat
  let currentDirection: Direction | null = null
  let repeatTimer: ReturnType<typeof setTimeout> | null = null
  let repeatInterval: ReturnType<typeof setInterval> | null = null

  // Stationary long-press sends Esc
  let longPressTimer: ReturnType<typeof setTimeout> | null = null
  let longPressTriggered = false

  // Pinch zoom and two-finger swipe state
  let initialPinchDistance = 0
  let lastPinchDistance = 0
  let accumulatedPinchDelta = 0
  let twoFingerStartCenterX = 0
  let twoFingerStartCenterY = 0
  let twoFingerStart1X = 0
  let twoFingerStart1Y = 0
  let twoFingerStart2X = 0
  let twoFingerStart2Y = 0
  let twoFingerMode: TwoFingerMode = 'pending'

  // Double-tap Tab state
  let lastTapTime = 0
  let lastTapX = 0
  let lastTapY = 0

  function getTouchDistance(t1: Touch, t2: Touch): number {
    const dx = t1.clientX - t2.clientX
    const dy = t1.clientY - t2.clientY
    return Math.sqrt(dx * dx + dy * dy)
  }

  function getTouchCenter(t1: Touch, t2: Touch): { x: number; y: number } {
    return {
      x: (t1.clientX + t2.clientX) / 2,
      y: (t1.clientY + t2.clientY) / 2,
    }
  }

  const DIRECTION_SYMBOLS: Record<Direction, string> = {
    up: '↑',
    down: '↓',
    left: '←',
    right: '→',
  }

  function sendArrow(dir: Direction) {
    switch (dir) {
      case 'up': callbacks.sendArrowUp(); break
      case 'down': callbacks.sendArrowDown(); break
      case 'left': callbacks.sendArrowLeft(); break
      case 'right': callbacks.sendArrowRight(); break
    }
    callbacks.onGestureHint?.(DIRECTION_SYMBOLS[dir])
  }

  function startRepeat(dir: Direction) {
    stopRepeat()
    repeatTimer = setTimeout(() => {
      repeatInterval = setInterval(() => {
        sendArrow(dir)
      }, REPEAT_INTERVAL)
    }, REPEAT_INITIAL_DELAY)
  }

  function stopRepeat() {
    if (repeatTimer) {
      clearTimeout(repeatTimer)
      repeatTimer = null
    }
    if (repeatInterval) {
      clearInterval(repeatInterval)
      repeatInterval = null
    }
  }

  function startLongPress(e: TouchEvent) {
    clearLongPress()
    longPressTriggered = false
    longPressTimer = setTimeout(() => {
      longPressTriggered = true
      callbacks.sendEscape()
      callbacks.onGestureHint?.('Esc')
    }, LONG_PRESS_MS)
  }

  function clearLongPress() {
    if (longPressTimer) {
      clearTimeout(longPressTimer)
      longPressTimer = null
    }
  }

  function resetGestureState() {
    stopRepeat()
    clearLongPress()
    longPressTriggered = false
    isActive = false
    currentDirection = null
    initialPinchDistance = 0
    lastPinchDistance = 0
    accumulatedPinchDelta = 0
    twoFingerMode = 'pending'
  }

  function detectDirection(dx: number, dy: number): Direction | null {
    const absDx = Math.abs(dx)
    const absDy = Math.abs(dy)
    if (absDx < SWIPE_THRESHOLD && absDy < SWIPE_THRESHOLD) return null
    if (absDx > absDy) {
      return dx > 0 ? 'right' : 'left'
    } else {
      return dy > 0 ? 'down' : 'up'
    }
  }

  function preventNativeTouch(e: TouchEvent) {
    if (e.cancelable) {
      e.preventDefault()
    }
  }

  function onTouchStart(e: TouchEvent) {
    if (e.touches.length === 2) {
      clearLongPress()
      // Pinch gesture start. Prevent browser pinch/selection only once a
      // terminal gesture is clear; stationary single-finger long press is left
      // untouched so xterm/browser text selection can still start normally.
      preventNativeTouch(e)
      initialPinchDistance = getTouchDistance(e.touches[0], e.touches[1])
      lastPinchDistance = initialPinchDistance
      accumulatedPinchDelta = 0
      const center = getTouchCenter(e.touches[0], e.touches[1])
      twoFingerStartCenterX = center.x
      twoFingerStartCenterY = center.y
      twoFingerStart1X = e.touches[0].clientX
      twoFingerStart1Y = e.touches[0].clientY
      twoFingerStart2X = e.touches[1].clientX
      twoFingerStart2Y = e.touches[1].clientY
      twoFingerMode = 'pending'
      isActive = false // cancel any single-finger gesture
      stopRepeat()
      currentDirection = null
      return
    }

    if (e.touches.length !== 1) return

    const touch = e.touches[0]
    touchStartX = touch.clientX
    touchStartY = touch.clientY
    isActive = true
    currentDirection = null
    startLongPress(e)
  }

  function onTouchMove(e: TouchEvent) {
    // Pinch zoom
    if (e.touches.length === 2 && initialPinchDistance > 0) {
      preventNativeTouch(e)
      const center = getTouchCenter(e.touches[0], e.touches[1])
      const centerDx = center.x - twoFingerStartCenterX
      const centerDy = center.y - twoFingerStartCenterY
      const currentDistance = getTouchDistance(e.touches[0], e.touches[1])
      const distanceDeltaFromStart = currentDistance - initialPinchDistance
      const distanceDelta = currentDistance - lastPinchDistance
      const touch1Dx = e.touches[0].clientX - twoFingerStart1X
      const touch1Dy = e.touches[0].clientY - twoFingerStart1Y
      const touch2Dx = e.touches[1].clientX - twoFingerStart2X
      const touch2Dy = e.touches[1].clientY - twoFingerStart2Y
      const sameVerticalDirection = Math.sign(touch1Dy) === Math.sign(touch2Dy)
      const bothMovedVertically = Math.abs(touch1Dy) >= TWO_FINGER_SWIPE_THRESHOLD && Math.abs(touch2Dy) >= TWO_FINGER_SWIPE_THRESHOLD
      const mostlyVertical = Math.abs(centerDy) > Math.abs(centerDx) && Math.abs(touch1Dy) > Math.abs(touch1Dx) && Math.abs(touch2Dy) > Math.abs(touch2Dx)

      if (twoFingerMode === 'pending') {
        if (Math.abs(distanceDeltaFromStart) >= PINCH_THRESHOLD) {
          twoFingerMode = 'pinch'
        } else if (Math.abs(centerDy) >= TWO_FINGER_SWIPE_THRESHOLD && sameVerticalDirection && bothMovedVertically && mostlyVertical) {
          twoFingerMode = 'swipe'
          if (centerDy < 0) {
            callbacks.sendPageUp()
            callbacks.onGestureHint?.('⇞')
          } else {
            callbacks.sendPageDown()
            callbacks.onGestureHint?.('⇟')
          }
          return
        }
      }

      if (twoFingerMode === 'pinch') {
        accumulatedPinchDelta += distanceDelta
        lastPinchDistance = currentDistance

        if (Math.abs(accumulatedPinchDelta) >= PINCH_THRESHOLD) {
          const steps = Math.trunc(accumulatedPinchDelta / PINCH_THRESHOLD)
          callbacks.onPinchZoom?.(steps)
          accumulatedPinchDelta -= steps * PINCH_THRESHOLD
        }
      }
      return
    }

    if (!isActive || e.touches.length !== 1) return

    if (longPressTriggered) {
      preventNativeTouch(e)
      return
    }

    // Direction detection for hold-to-repeat
    const touch = e.touches[0]
    const dx = touch.clientX - touchStartX
    const dy = touch.clientY - touchStartY
    const dir = detectDirection(dx, dy)

    if (Math.abs(dx) > TAP_THRESHOLD || Math.abs(dy) > TAP_THRESHOLD) {
      clearLongPress()
    }

    if (dir || currentDirection) {
      // Once the movement is clearly a terminal gesture, suppress native
      // selection/scroll for the remainder of the gesture. Before the
      // threshold is crossed, do not prevent default so long-press selection
      // can start normally.
      preventNativeTouch(e)
    }

    if (dir && dir !== currentDirection) {
      // Direction changed or first detection — send once and start repeat
      currentDirection = dir
      sendArrow(dir)
      startRepeat(dir)
    }
  }

  function onTouchCancel() {
    resetGestureState()
  }

  function onTouchEnd(e: TouchEvent) {
    // Reset pinch state when one or both fingers lift
    if (e.touches.length < 2) {
      initialPinchDistance = 0
      lastPinchDistance = 0
      twoFingerMode = 'pending'
    }

    // Stop any hold-to-repeat
    stopRepeat()
    clearLongPress()

    if (!isActive) return

    if (longPressTriggered) {
      longPressTriggered = false
      isActive = false
      currentDirection = null
      preventNativeTouch(e)
      return
    }

    const wasDirection = currentDirection
    currentDirection = null
    isActive = false

    // If direction was already handled in touchmove (hold-to-repeat),
    // skip the legacy swipe-on-touchend logic
    if (wasDirection) {
      preventNativeTouch(e)
      return
    }

    const touch = e.changedTouches[0]
    const dx = touch.clientX - touchStartX
    const dy = touch.clientY - touchStartY
    const dir = detectDirection(dx, dy)
    if (dir) {
      // It's a swipe — send the arrow key
      preventNativeTouch(e)
      sendArrow(dir)
    } else if (Math.abs(dx) <= TAP_THRESHOLD && Math.abs(dy) <= TAP_THRESHOLD) {
      // It's a tap (no significant movement) — check for double-tap
      const now = Date.now()
      const tapDx = touch.clientX - lastTapX
      const tapDy = touch.clientY - lastTapY
      const isDoubleTap = (now - lastTapTime) < DOUBLE_TAP_MS
        && Math.abs(tapDx) < TAP_THRESHOLD * 2
        && Math.abs(tapDy) < TAP_THRESHOLD * 2
      if (isDoubleTap) {
        preventNativeTouch(e)
        callbacks.sendTab()
        callbacks.onGestureHint?.('⇥')
        lastTapTime = 0 // reset to avoid triple-tap
      } else {
        lastTapTime = now
        lastTapX = touch.clientX
        lastTapY = touch.clientY
      }
    }
  }

  function attachListeners() {
    if (listenersAttached) return
    const el = elementRef.value
    if (!el) return

    el.addEventListener('touchstart', onTouchStart, { passive: false })
    el.addEventListener('touchmove', onTouchMove, { passive: false })
    el.addEventListener('touchend', onTouchEnd, { passive: false })
    el.addEventListener('touchcancel', onTouchCancel, { passive: false })
    listenersAttached = true
  }

  function detachListeners() {
    if (!listenersAttached) return
    const el = elementRef.value
    if (!el) return

    stopRepeat()
    clearLongPress()
    el.removeEventListener('touchstart', onTouchStart)
    el.removeEventListener('touchmove', onTouchMove)
    el.removeEventListener('touchend', onTouchEnd)
    el.removeEventListener('touchcancel', onTouchCancel)
    listenersAttached = false
  }

  // Apply gesture state: attach when enabled, detach when disabled.
  // Keep touch-action permissive enough for long-press/native selection; the
  // handlers call preventDefault only after recognizing a terminal gesture.
  function applyState() {
    const el = elementRef.value
    if (enabled.value) {
      attachListeners()
      if (el) el.style.touchAction = 'manipulation'
    } else {
      detachListeners()
      // Restore fully native touch handling so long-press can open the
      // platform selection/copy UI instead of only allowing vertical panning.
      if (el) el.style.touchAction = 'auto'
    }
  }

  function toggle() {
    enabled.value = !enabled.value
    if (!enabled.value) {
      resetGestureState()
      lastTapTime = 0
    }
    applyState()
  }

  // Called by TerminalPanel on mount
  function attach() {
    applyState()
  }

  // Called by TerminalPanel on unmount
  function detach() {
    detachListeners()
    const el = elementRef.value
    if (el) el.style.touchAction = ''
  }

  return {
    attach,
    detach,
    enabled,
    toggle,
  }
}

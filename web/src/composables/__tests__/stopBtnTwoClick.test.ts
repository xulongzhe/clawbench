import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'

// ────────────────────────────────────────────────────────────
// Stop button two-click confirmation logic (extracted from ChatInputBar.vue)
// The actual state lives inside the component. We replicate the
// state machine logic here for isolated testing.
// ────────────────────────────────────────────────────────────

function createStopButtonMachine() {
  let primed = false
  let timer: ReturnType<typeof setTimeout> | null = null

  function handleClick(): { primed: boolean; cancelled: boolean } {
    if (!primed) {
      // First click: enter confirmation state
      primed = true
      if (timer) clearTimeout(timer)
      timer = setTimeout(() => { primed = false }, 1500)
      return { primed: true, cancelled: false }
    } else {
      // Second click: confirmed — execute stop
      primed = false
      if (timer) { clearTimeout(timer); timer = null }
      return { primed: false, cancelled: true }
    }
  }

  function reset() {
    primed = false
    if (timer) { clearTimeout(timer); timer = null }
  }

  function getPrimed() { return primed }

  return { handleClick, reset, getPrimed }
}

describe('stop-button-two-click', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  // ── First click → primed state ──
  it('first click enters primed state', () => {
    const machine = createStopButtonMachine()
    const result = machine.handleClick()
    expect(result.primed).toBe(true)
    expect(result.cancelled).toBe(false)
    expect(machine.getPrimed()).toBe(true)
  })

  // ── Second click → cancel ──
  it('second click cancels', () => {
    const machine = createStopButtonMachine()
    machine.handleClick()
    const result = machine.handleClick()
    expect(result.primed).toBe(false)
    expect(result.cancelled).toBe(true)
    expect(machine.getPrimed()).toBe(false)
  })

  // ── Timeout resets primed state ──
  it('primed state resets after 1.5s timeout', () => {
    const machine = createStopButtonMachine()
    machine.handleClick()
    expect(machine.getPrimed()).toBe(true)

    vi.advanceTimersByTime(1500)
    expect(machine.getPrimed()).toBe(false)
  })

  // ── Primed state persists before timeout ──
  it('primed state persists before timeout', () => {
    const machine = createStopButtonMachine()
    machine.handleClick()

    vi.advanceTimersByTime(1400)
    expect(machine.getPrimed()).toBe(true)
  })

  // ── Click after timeout starts new cycle ──
  it('click after timeout starts new primed cycle', () => {
    const machine = createStopButtonMachine()
    machine.handleClick()
    vi.advanceTimersByTime(1500)
    expect(machine.getPrimed()).toBe(false)

    // New click should start primed again (not cancel)
    const result = machine.handleClick()
    expect(result.primed).toBe(true)
    expect(result.cancelled).toBe(false)
  })

  // ── Manual reset ──
  it('reset clears primed state', () => {
    const machine = createStopButtonMachine()
    machine.handleClick()
    expect(machine.getPrimed()).toBe(true)

    machine.reset()
    expect(machine.getPrimed()).toBe(false)
  })

  // ── Reset then click starts fresh ──
  it('reset then click starts fresh cycle', () => {
    const machine = createStopButtonMachine()
    machine.handleClick()
    machine.reset()

    const result = machine.handleClick()
    expect(result.primed).toBe(true)
    expect(result.cancelled).toBe(false)
  })

  // ── Rapid triple click: primed → cancel → primed ──
  it('rapid triple click: primed → cancel → primed again', () => {
    const machine = createStopButtonMachine()
    const r1 = machine.handleClick() // prime
    const r2 = machine.handleClick() // cancel
    const r3 = machine.handleClick() // prime again (new cycle)

    expect(r1).toEqual({ primed: true, cancelled: false })
    expect(r2).toEqual({ primed: false, cancelled: true })
    expect(r3).toEqual({ primed: true, cancelled: false })
  })

  // ── Loading ends resets primed state ──
  it('loading=false resets primed state (simulated)', () => {
    const machine = createStopButtonMachine()
    machine.handleClick()
    expect(machine.getPrimed()).toBe(true)

    // Simulate: loading ends → reset
    machine.reset()
    expect(machine.getPrimed()).toBe(false)
  })
})

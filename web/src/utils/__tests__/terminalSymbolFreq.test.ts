import { describe, expect, it } from 'vitest'
import {
  decayFactor,
  currentScore,
  sortSymbolsByFreq,
  incrementSymbolFreq,
  loadSymbolFreqs,
  saveSymbolFreqs,
  ALL_SYMBOLS,
  DECAY_LAMBDA,
  SYMBOL_FREQ_KEY,
  type SymbolFreqs,
} from '@/utils/terminalSymbolFreq'

describe('decayFactor', () => {
  it('returns 1 for 0 hours', () => {
    expect(decayFactor(0)).toBe(1)
  })

  it('returns values between 0 and 1 for positive hours', () => {
    const val = decayFactor(1)
    expect(val).toBeGreaterThan(0)
    expect(val).toBeLessThan(1)
  })

  it('approaches 0 for very large hours', () => {
    expect(decayFactor(1000)).toBeCloseTo(0, 10)
  })

  it('follows exponential decay formula: e^(-λh)', () => {
    const hours = 4.6 // approximately half-life
    const val = decayFactor(hours)
    // e^(-0.15 * 4.6) ≈ 0.5
    expect(val).toBeCloseTo(0.5, 1)
  })

  it('returns > 1 for negative hours (mathematically valid)', () => {
    expect(decayFactor(-1)).toBeGreaterThan(1)
  })

  it('decays to ~0.86 after 1 hour', () => {
    const val = decayFactor(1)
    expect(val).toBeCloseTo(Math.exp(-0.15), 5)
  })
})

describe('currentScore', () => {
  const now = Date.now()

  it('returns 0 for undefined entry', () => {
    expect(currentScore(undefined, now)).toBe(0)
  })

  it('returns score with no decay for just-timestamped entry', () => {
    expect(currentScore({ s: 5, t: now }, now)).toBeCloseTo(5, 5)
  })

  it('applies decay for older entries', () => {
    const oneHourAgo = now - 3_600_000
    const val = currentScore({ s: 10, t: oneHourAgo }, now)
    expect(val).toBeCloseTo(10 * Math.exp(-0.15), 3)
  })

  it('returns near-zero for very old entries', () => {
    const veryOld = now - 3_600_000 * 1000
    expect(currentScore({ s: 100, t: veryOld }, now)).toBeCloseTo(0, 5)
  })

  it('handles zero score', () => {
    expect(currentScore({ s: 0, t: now }, now)).toBe(0)
  })
})

describe('sortSymbolsByFreq', () => {
  const now = Date.now()

  it('returns all symbols when freqs is empty', () => {
    const sorted = sortSymbolsByFreq({}, now)
    expect(sorted).toHaveLength(ALL_SYMBOLS.length)
    // Should contain all symbols
    for (const sym of ALL_SYMBOLS) {
      expect(sorted).toContain(sym)
    }
  })

  it('places frequently used symbols first', () => {
    const freqs: SymbolFreqs = {
      '|': { s: 100, t: now },
      '.': { s: 1, t: now },
    }
    const sorted = sortSymbolsByFreq(freqs, now)
    expect(sorted[0]).toBe('|')
  })

  it('considers recency in sorting', () => {
    const tenHoursAgo = now - 3_600_000 * 10
    const freqs: SymbolFreqs = {
      '.': { s: 10, t: tenHoursAgo },  // older → heavily decayed: 10*e^(-1.5)≈2.23
      '|': { s: 5, t: now },            // recent → no decay: 5
    }
    const sorted = sortSymbolsByFreq(freqs, now)
    // Recent '|' with score 5 > old '.' with decayed score ~2.23
    expect(sorted[0]).toBe('|')
  })

  it('sorts equal-score entries by default alphabetical', () => {
    const freqs: SymbolFreqs = {}
    const sorted = sortSymbolsByFreq(freqs, now)
    // All have score 0, so order is determined by sort stability
    expect(sorted).toHaveLength(ALL_SYMBOLS.length)
  })
})

describe('incrementSymbolFreq', () => {
  const now = Date.now()

  it('creates new entry for unseen symbol', () => {
    const freqs: SymbolFreqs = {}
    const result = incrementSymbolFreq(freqs, '.', now)
    expect(result['.']).toEqual({ s: 1, t: now })
  })

  it('increments existing entry with decay', () => {
    const oneHourAgo = now - 3_600_000
    const freqs: SymbolFreqs = {
      '.': { s: 5, t: oneHourAgo },
    }
    const result = incrementSymbolFreq(freqs, '.', now)
    // new score = 5 * e^(-0.15) + 1 ≈ 5.30
    expect(result['.'].s).toBeCloseTo(5 * Math.exp(-0.15) + 1, 3)
    expect(result['.'].t).toBe(now)
  })

  it('does not mutate the original freqs object', () => {
    const freqs: SymbolFreqs = { '.': { s: 5, t: now - 1000 } }
    const original = { ...freqs, '.': { ...freqs['.'] } }
    incrementSymbolFreq(freqs, '.', now)
    expect(freqs).toEqual(original)
  })

  it('increments just-touched entry with minimal decay', () => {
    const freqs: SymbolFreqs = { '.': { s: 5, t: now } }
    const result = incrementSymbolFreq(freqs, '.', now)
    // 0 hours elapsed, so decay is 1 → new score = 5 * 1 + 1 = 6
    expect(result['.'].s).toBe(6)
  })

  it('handles multiple different symbols', () => {
    const freqs: SymbolFreqs = { '.': { s: 2, t: now } }
    const result1 = incrementSymbolFreq(freqs, '.', now)
    const result2 = incrementSymbolFreq(result1, '|', now)
    expect(result2['.'].s).toBe(3) // 2+1
    expect(result2['|']).toEqual({ s: 1, t: now })
  })
})

describe('loadSymbolFreqs', () => {
  it('returns empty object when no saved data', () => {
    expect(loadSymbolFreqs(() => null)).toEqual({})
  })

  it('parses valid JSON', () => {
    const data = JSON.stringify({ '.': { s: 5, t: 1000 } })
    expect(loadSymbolFreqs(() => data)).toEqual({ '.': { s: 5, t: 1000 } })
  })

  it('returns empty object for invalid JSON', () => {
    expect(loadSymbolFreqs(() => 'not json')).toEqual({})
  })
})

describe('saveSymbolFreqs', () => {
  it('stores JSON to localStorage', () => {
    let storedKey = ''
    let storedValue = ''
    const setItem = (k: string, v: string) => { storedKey = k; storedValue = v }
    const freqs: SymbolFreqs = { '.': { s: 5, t: 1000 } }
    saveSymbolFreqs(freqs, setItem)
    expect(storedKey).toBe(SYMBOL_FREQ_KEY)
    expect(JSON.parse(storedValue)).toEqual(freqs)
  })
})

describe('constants', () => {
  it('ALL_SYMBOLS contains expected symbols', () => {
    expect(ALL_SYMBOLS).toContain('.')
    expect(ALL_SYMBOLS).toContain('/')
    expect(ALL_SYMBOLS).toContain('-')
    expect(ALL_SYMBOLS).toContain('$')
    expect(ALL_SYMBOLS).toContain('|')
    expect(ALL_SYMBOLS).toContain('_')
    expect(ALL_SYMBOLS).toContain('~')
    expect(ALL_SYMBOLS).toContain('#')
  })

  it('ALL_SYMBOLS has 19 entries', () => {
    expect(ALL_SYMBOLS).toHaveLength(19)
  })

  it('ALL_SYMBOLS has no duplicates', () => {
    expect(new Set(ALL_SYMBOLS).size).toBe(ALL_SYMBOLS.length)
  })

  it('DECAY_LAMBDA is 0.15', () => {
    expect(DECAY_LAMBDA).toBe(0.15)
  })

  it('SYMBOL_FREQ_KEY has expected value', () => {
    expect(SYMBOL_FREQ_KEY).toBe('clawbench-terminal-symbol-freq')
  })
})

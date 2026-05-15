/**
 * Terminal symbol bar frequency sorting with exponential decay.
 * Pure functions extracted from TerminalPanelContent.vue for testability.
 */

export const ALL_SYMBOLS = ['.', '/', '-', '$', '"', "'", '&', ';', '|', '=', '>', '_', '~', '*', ':', '<', '`', '!', '#']
export const SYMBOL_FREQ_KEY = 'clawbench-terminal-symbol-freq'
export const DECAY_LAMBDA = 0.15 // decay rate per hour — half-life ≈ 4.6h

export interface SymbolScore { s: number; t: number }
export type SymbolFreqs = Record<string, SymbolScore>

/**
 * Exponential decay factor for a given elapsed hours.
 */
export function decayFactor(hoursElapsed: number): number {
  return Math.exp(-DECAY_LAMBDA * hoursElapsed)
}

/**
 * Compute current decayed score for a symbol.
 */
export function currentScore(entry: SymbolScore | undefined, now: number): number {
  if (!entry) return 0
  const hours = (now - entry.t) / 3_600_000
  return entry.s * decayFactor(hours)
}

/**
 * Sort symbols by decayed score (descending).
 */
export function sortSymbolsByFreq(freqs: SymbolFreqs, now: number): string[] {
  return [...ALL_SYMBOLS].sort((a, b) => currentScore(freqs[b], now) - currentScore(freqs[a], now))
}

/**
 * Update symbol frequency after a click: apply decay to old score, add 1, update timestamp.
 * Returns the updated freqs object.
 */
export function incrementSymbolFreq(
  freqs: SymbolFreqs,
  sym: string,
  now: number,
): SymbolFreqs {
  const updated: SymbolFreqs = {}
  for (const key of Object.keys(freqs)) {
    updated[key] = { ...freqs[key] }
  }
  const entry = updated[sym]
  if (entry) {
    const hours = (now - entry.t) / 3_600_000
    entry.s = entry.s * decayFactor(hours) + 1
    entry.t = now
  } else {
    updated[sym] = { s: 1, t: now }
  }
  return updated
}

/**
 * Load symbol frequencies from localStorage.
 */
export function loadSymbolFreqs(getItem: (key: string) => string | null = localStorage.getItem.bind(localStorage)): SymbolFreqs {
  try {
    const raw = getItem(SYMBOL_FREQ_KEY)
    return raw ? JSON.parse(raw) : {}
  } catch { return {} }
}

/**
 * Save symbol frequencies to localStorage.
 */
export function saveSymbolFreqs(
  freqs: SymbolFreqs,
  setItem: (key: string, value: string) => void = localStorage.setItem.bind(localStorage),
): void {
  setItem(SYMBOL_FREQ_KEY, JSON.stringify(freqs))
}

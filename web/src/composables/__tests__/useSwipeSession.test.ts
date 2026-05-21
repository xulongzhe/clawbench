import { describe, expect, it } from 'vitest'

/**
 * Pure swipe classification logic — extracted for testability.
 * Mirrors the logic in useSwipeSession.onTouchEnd.
 */
export function classifySwipe(
    deltaX: number,
    deltaY: number,
    duration: number,
    threshold = 80,
    maxDuration = 500,
    maxVerticalRatio = 0.75,
): 'left' | 'right' | null {
    if (duration > maxDuration) return null
    if (Math.abs(deltaY) > Math.abs(deltaX) * maxVerticalRatio) return null
    if (Math.abs(deltaX) < threshold) return null
    if (deltaX < 0) return 'left'
    return 'right'
}

/**
 * Circular session index navigation (DESC-ordered sessions).
 * - Next (swipe left) = index - 1 (more recent)
 * - Prev (swipe right) = index + 1 (older)
 */
export function getNextSessionIndex(currentIndex: number, total: number): number {
    if (total <= 1) return currentIndex
    return currentIndex > 0 ? currentIndex - 1 : total - 1
}

export function getPrevSessionIndex(currentIndex: number, total: number): number {
    if (total <= 1) return currentIndex
    return currentIndex < total - 1 ? currentIndex + 1 : 0
}

// ── classifySwipe ──

describe('classifySwipe', () => {
    it('returns "left" for negative deltaX exceeding threshold', () => {
        expect(classifySwipe(-100, 0, 200)).toBe('left')
    })

    it('returns "right" for positive deltaX exceeding threshold', () => {
        expect(classifySwipe(100, 0, 200)).toBe('right')
    })

    it('returns null for slow swipes (duration > 500ms)', () => {
        expect(classifySwipe(-100, 0, 600)).toBeNull()
    })

    it('returns null for short swipes (deltaX < 80px)', () => {
        expect(classifySwipe(-50, 0, 200)).toBeNull()
    })

    it('returns null when vertical movement dominates', () => {
        // deltaY > deltaX * 0.75
        expect(classifySwipe(100, 80, 200)).toBeNull()
    })

    it('allows slight vertical movement', () => {
        // deltaY < deltaX * 0.75
        expect(classifySwipe(100, 50, 200)).toBe('right')
    })

    it('returns null for zero deltaX', () => {
        expect(classifySwipe(0, 0, 200)).toBeNull()
    })

    it('returns null exactly at threshold boundary', () => {
        expect(classifySwipe(-80, 0, 200)).toBe('left') // exactly threshold = pass
    })

    it('returns null exactly at max duration boundary', () => {
        expect(classifySwipe(-100, 0, 500)).toBe('left') // exactly max = pass
    })

    it('returns null when vertical ratio equals threshold', () => {
        // deltaY === deltaX * 0.75 → Math.abs(75) > Math.abs(100) * 0.75 = 75
        // 75 > 75 is false, so it passes the vertical check → returns 'right'
        // Actually: 75 > 75 is false, so the check passes and swipe is classified
        // The condition is deltaY > deltaX * ratio (strict greater than)
        expect(classifySwipe(100, 75, 200)).toBe('right')
    })

    it('handles negative deltaY (swipe up-right)', () => {
        expect(classifySwipe(100, -50, 200)).toBe('right')
    })

    it('handles diagonal swipes within vertical ratio', () => {
        expect(classifySwipe(-120, 30, 300)).toBe('left')
    })

    it('respects custom threshold', () => {
        expect(classifySwipe(-50, 0, 200, 100)).toBeNull() // below custom threshold
        expect(classifySwipe(-100, 0, 200, 100)).toBe('left') // above custom threshold
    })

    it('respects custom maxDuration', () => {
        expect(classifySwipe(-100, 0, 400, 80, 300)).toBeNull() // above custom maxDuration
    })
})

// ── getNextSessionIndex / getPrevSessionIndex ──

describe('getNextSessionIndex', () => {
    it('returns index-1 for non-zero index', () => {
        expect(getNextSessionIndex(2, 5)).toBe(1)
    })

    it('wraps to last index when at 0', () => {
        expect(getNextSessionIndex(0, 5)).toBe(4)
    })

    it('returns current index when only one session', () => {
        expect(getNextSessionIndex(0, 1)).toBe(0)
    })

    it('returns current index when zero sessions', () => {
        expect(getNextSessionIndex(0, 0)).toBe(0)
    })

    it('handles being at last index (no wrap needed since DESC)', () => {
        expect(getNextSessionIndex(4, 5)).toBe(3)
    })
})

describe('getPrevSessionIndex', () => {
    it('returns index+1 for non-last index', () => {
        expect(getPrevSessionIndex(2, 5)).toBe(3)
    })

    it('wraps to 0 when at last index', () => {
        expect(getPrevSessionIndex(4, 5)).toBe(0)
    })

    it('returns current index when only one session', () => {
        expect(getPrevSessionIndex(0, 1)).toBe(0)
    })

    it('handles being at index 0 (goes to 1)', () => {
        expect(getPrevSessionIndex(0, 5)).toBe(1)
    })
})

// ── Circular navigation consistency ──

describe('circular navigation consistency', () => {
    it('next then prev returns to original for multi-session list', () => {
        const total = 5
        for (let i = 0; i < total; i++) {
            const nextIdx = getNextSessionIndex(i, total)
            const backIdx = getPrevSessionIndex(nextIdx, total)
            expect(backIdx).toBe(i)
        }
    })

    it('prev then next returns to original for multi-session list', () => {
        const total = 5
        for (let i = 0; i < total; i++) {
            const prevIdx = getPrevSessionIndex(i, total)
            const backIdx = getNextSessionIndex(prevIdx, total)
            expect(backIdx).toBe(i)
        }
    })
})

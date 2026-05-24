import { describe, expect, it, vi, beforeEach } from 'vitest'
import { registerBackHandler, handleBackNavigation, canNavigateBack } from '../useBackHandler'

describe('useBackHandler', () => {
    beforeEach(() => {
        // Clear all handlers between tests by using the module's internal state
        // Since handlers is module-scoped, we need to handle this via the API
    })

    it('returns false when no handlers are registered', () => {
        expect(handleBackNavigation()).toBe(false)
        expect(canNavigateBack()).toBe(false)
    })

    it('calls the first handler that can go back', () => {
        const goBack1 = vi.fn()
        const goBack2 = vi.fn()

        const unregister1 = registerBackHandler({
            id: 'test1',
            canGoBack: () => false,
            goBack: goBack1,
        })
        const unregister2 = registerBackHandler({
            id: 'test2',
            canGoBack: () => true,
            goBack: goBack2,
        })

        const result = handleBackNavigation()
        expect(result).toBe(true)
        expect(goBack1).not.toHaveBeenCalled()
        expect(goBack2).toHaveBeenCalledTimes(1)

        unregister1()
        unregister2()
    })

    it('returns false when no handler can go back', () => {
        const goBack = vi.fn()
        const unregister = registerBackHandler({
            id: 'test',
            canGoBack: () => false,
            goBack,
        })

        expect(handleBackNavigation()).toBe(false)
        expect(goBack).not.toHaveBeenCalled()

        unregister()
    })

    it('unregisters a handler correctly', () => {
        const unregister = registerBackHandler({
            id: 'test',
            canGoBack: () => true,
            goBack: vi.fn(),
        })

        expect(canNavigateBack()).toBe(true)
        unregister()
        expect(canNavigateBack()).toBe(false)
    })

    it('gives priority to the last registered handler', () => {
        const goBack1 = vi.fn()
        const goBack2 = vi.fn()

        const unregister1 = registerBackHandler({
            id: 'test1',
            canGoBack: () => true,
            goBack: goBack1,
        })
        const unregister2 = registerBackHandler({
            id: 'test2',
            canGoBack: () => true,
            goBack: goBack2,
        })

        handleBackNavigation()
        expect(goBack1).not.toHaveBeenCalled()
        expect(goBack2).toHaveBeenCalledTimes(1)

        unregister1()
        unregister2()
    })

    it('canNavigateBack returns true if any handler can go back', () => {
        const unregister1 = registerBackHandler({
            id: 'test1',
            canGoBack: () => false,
            goBack: vi.fn(),
        })
        const unregister2 = registerBackHandler({
            id: 'test2',
            canGoBack: () => true,
            goBack: vi.fn(),
        })

        expect(canNavigateBack()).toBe(true)

        unregister1()
        unregister2()
    })
})

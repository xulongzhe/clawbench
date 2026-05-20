import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { useSettingsNavigation } from '@/composables/useSettingsNavigation'

// Mock dependencies
const mockLoadConfig = vi.fn()
const mockRestartServer = vi.fn()
const mockToastShow = vi.fn()

vi.mock('@/composables/useSettingsConfig', () => ({
  useSettingsConfig: () => ({
    loadConfig: mockLoadConfig,
    restartServer: mockRestartServer,
  }),
}))

vi.mock('@/composables/useToast', () => ({
  useToast: () => ({
    show: mockToastShow,
  }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

// Mock vue's onUnmounted to be a no-op in tests
vi.mock('vue', async () => {
  const actual = await vi.importActual('vue')
  return {
    ...actual,
    onUnmounted: vi.fn(),
  }
})

describe('useSettingsNavigation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('pollUntilServerUp timeout', () => {
    it('shows error toast when polling times out', async () => {
      mockRestartServer.mockResolvedValue(undefined)

      // Mock fetch to always fail (server never comes back)
      vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Connection refused'))

      const { handleRestart, restartingOverlay, restarting } = useSettingsNavigation()

      await handleRestart()

      // Fast-forward through all poll attempts (60 attempts × 2s = 120s)
      for (let i = 0; i < 60; i++) {
        await vi.advanceTimersByTimeAsync(2000)
      }

      // Should have shown error toast
      expect(mockToastShow).toHaveBeenCalledWith(
        'settings.restartTimeout',
        expect.objectContaining({ type: 'error' }),
      )

      // Overlay and restarting should be cleared
      expect(restartingOverlay.value).toBe(false)
      expect(restarting.value).toBe(false)
    })

    it('clears overlay and reloading when server comes back', async () => {
      mockRestartServer.mockResolvedValue(undefined)

      // Mock fetch to succeed on the 3rd attempt
      let fetchCount = 0
      vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
        fetchCount++
        if (fetchCount >= 3) {
          return { ok: true } as Response
        }
        throw new Error('Connection refused')
      })

      // Mock window.location.reload
      const reloadMock = vi.fn()
      Object.defineProperty(window, 'location', {
        value: { reload: reloadMock },
        writable: true,
      })

      const { handleRestart, restartingOverlay } = useSettingsNavigation()

      await handleRestart()

      // Advance through polls
      for (let i = 0; i < 5; i++) {
        await vi.advanceTimersByTimeAsync(2000)
      }

      // Should have reloaded, not shown error toast
      expect(mockToastShow).not.toHaveBeenCalled()
      expect(reloadMock).toHaveBeenCalled()
    })
  })
})

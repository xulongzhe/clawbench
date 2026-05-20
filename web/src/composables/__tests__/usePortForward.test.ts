import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ref } from 'vue'

// Mock API utilities
const mockApiGet = vi.fn()
const mockApiPost = vi.fn()
const mockApiDelete = vi.fn()

vi.mock('@/utils/api', () => ({
  apiGet: (...args: any[]) => mockApiGet(...args),
  apiPost: (...args: any[]) => mockApiPost(...args),
  apiDelete: (...args: any[]) => mockApiDelete(...args),
}))

vi.mock('@/composables/useAppMode', () => ({
  useAppMode: () => ({ isAppMode: ref(false) }),
}))

vi.mock('@/composables/useLocale', () => ({
  gt: (key: string) => key,
}))

describe('usePortForward', () => {
  beforeEach(() => {
    vi.resetModules()
    mockApiGet.mockReset()
    mockApiPost.mockReset()
    mockApiDelete.mockReset()
  })

  describe('loadSSHInfo', () => {
    it('fetches SSH info and stores in sshInfo ref', async () => {
      const sshInfoResponse = {
        enabled: true,
        host: 'example.com',
        port: 20001,
        username: 'clawbench',
        fingerprint: 'SHA256:abc',
        command: 'ssh -L ...',
        connectionStats: { connected: true, clientCount: 1, activeChannels: 1 },
      }
      mockApiGet.mockResolvedValue(sshInfoResponse)

      const { usePortForward } = await import('@/composables/usePortForward')
      const { loadSSHInfo, sshInfo } = usePortForward()

      await loadSSHInfo()

      expect(mockApiGet).toHaveBeenCalledWith('/api/ssh/info')
      expect(sshInfo.value).toEqual(sshInfoResponse)
    })

    it('sets sshInfo to null on API failure', async () => {
      mockApiGet.mockRejectedValue(new Error('Network error'))

      const { usePortForward } = await import('@/composables/usePortForward')
      const { loadSSHInfo, sshInfo } = usePortForward()

      await loadSSHInfo()

      expect(sshInfo.value).toBeNull()
    })
  })

  describe('loadPorts', () => {
    it('fetches ports and stores in ports ref', async () => {
      const portsResponse = {
        ports: [
          { port: 3000, name: 'App', protocol: 'http', autoDetect: false, active: true },
        ],
      }
      mockApiGet.mockResolvedValue(portsResponse)

      const { usePortForward } = await import('@/composables/usePortForward')
      const { loadPorts, ports } = usePortForward()

      await loadPorts()

      expect(mockApiGet).toHaveBeenCalledWith('/api/proxy/ports')
      expect(ports.value).toHaveLength(1)
      expect(ports.value[0].port).toBe(3000)
    })
  })

  describe('checkTunnelHealth', () => {
    it('returns early when SSH is disabled', async () => {
      mockApiGet.mockImplementation((url: string) => {
        if (url === '/api/proxy/ports') return { ports: [] }
        if (url === '/api/ssh/info') return { enabled: false, host: '', port: 0, username: '', fingerprint: '', command: '', connectionStats: null }
        return {}
      })

      const { usePortForward } = await import('@/composables/usePortForward')
      const { checkTunnelHealth, tunnelStatus, tunnelChecking } = usePortForward()

      await checkTunnelHealth()

      // When SSH is disabled, tunnel status stays 'unknown' and checking is false
      expect(tunnelChecking.value).toBe(false)
    })
  })

  describe('openPort', () => {
    it('calls window.open in web mode', async () => {
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

      const { usePortForward } = await import('@/composables/usePortForward')
      const { openPort } = usePortForward()

      openPort(3000, 'http')

      expect(openSpy).toHaveBeenCalledWith('http://localhost:3000', '_blank')

      openSpy.mockRestore()
    })
  })

  describe('module-level state sharing', () => {
    it('shares sshInfo ref across multiple usePortForward calls', async () => {
      const sshInfoResponse = { enabled: true, host: 'test', port: 20001, username: 'u', fingerprint: 'f', command: 'c', connectionStats: null }
      mockApiGet.mockResolvedValue(sshInfoResponse)

      const { usePortForward } = await import('@/composables/usePortForward')
      const instance1 = usePortForward()
      const instance2 = usePortForward()

      await instance1.loadSSHInfo()

      // Both instances should see the same sshInfo (module-level singleton)
      expect(instance1.sshInfo.value).toEqual(sshInfoResponse)
      expect(instance2.sshInfo.value).toEqual(sshInfoResponse)
    })
  })
})

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

const mockIsAppMode = ref(false)
vi.mock('@/composables/useAppMode', () => ({
    useAppMode: () => ({ isAppMode: mockIsAppMode }),
}))

vi.mock('@/composables/useLocale', () => ({
    gt: (key: string) => key,
}))

vi.mock('@/utils/portForwardUtils', () => ({
    tunnelStatusFromPorts: () => 'ok',
    buildPortUrl: (port: number, protocol?: string, host?: string) => `${protocol || 'http'}://${host || 'localhost'}:${port}`,
}))

describe('usePortForward', () => {
    beforeEach(() => {
        vi.resetModules()
        mockApiGet.mockReset()
        mockApiPost.mockReset()
        mockApiDelete.mockReset()
        mockIsAppMode.value = false
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

        it('sets loading state when not silent', async () => {
            mockApiGet.mockResolvedValue({ ports: [] })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { loadPorts, loading } = usePortForward()

            const promise = loadPorts()
            // Loading should be true during the API call
            expect(loading.value).toBe(true)

            await promise
            expect(loading.value).toBe(false)
        })

        it('does not set loading state when silent', async () => {
            mockApiGet.mockResolvedValue({ ports: [] })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { loadPorts, loading } = usePortForward()

            await loadPorts(true)
            expect(loading.value).toBe(false)
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
            const { checkTunnelHealth, tunnelChecking } = usePortForward()

            await checkTunnelHealth()

            expect(tunnelChecking.value).toBe(false)
        })

        it('sets disconnected when SSH is enabled but not connected', async () => {
            mockApiGet.mockImplementation((url: string) => {
                if (url === '/api/proxy/ports') return { ports: [] }
                if (url === '/api/ssh/info') return {
                    enabled: true, host: 'test', port: 22, username: 'u', fingerprint: 'f', command: 'c',
                    connectionStats: { connected: false, clientCount: 0, activeChannels: 0 },
                }
                return {}
            })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { checkTunnelHealth, tunnelStatus, tunnelChecking } = usePortForward()

            await checkTunnelHealth()

            expect(tunnelChecking.value).toBe(false)
            expect(tunnelStatus.value).toBe('disconnected')
        })

        it('sets ok when SSH is connected with active ports', async () => {
            mockApiGet.mockImplementation((url: string) => {
                if (url === '/api/proxy/ports') return { ports: [{ port: 3000, name: 'App', protocol: 'http', autoDetect: false, active: true }] }
                if (url === '/api/ssh/info') return {
                    enabled: true, host: 'test', port: 22, username: 'u', fingerprint: 'f', command: 'c',
                    connectionStats: { connected: true, clientCount: 1, activeChannels: 1 },
                }
                return {}
            })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { checkTunnelHealth, tunnelStatus, tunnelChecking } = usePortForward()

            await checkTunnelHealth()

            expect(tunnelChecking.value).toBe(false)
            expect(tunnelStatus.value).toBe('ok')
        })

        it('sets ok when SSH reports disconnected but ports are active', async () => {
            mockApiGet.mockImplementation((url: string) => {
                if (url === '/api/proxy/ports') return { ports: [{ port: 3000, name: 'App', protocol: 'http', autoDetect: false, active: true }] }
                if (url === '/api/ssh/info') return {
                    enabled: true, host: 'test', port: 22, username: 'u', fingerprint: 'f', command: 'c',
                    connectionStats: { connected: false, clientCount: 0, activeChannels: 0 },
                }
                return {}
            })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { checkTunnelHealth, tunnelStatus } = usePortForward()

            await checkTunnelHealth()

            expect(tunnelStatus.value).toBe('ok')
        })

        it('resets tunnel state before checking', async () => {
            mockApiGet.mockImplementation((url: string) => {
                if (url === '/api/proxy/ports') return { ports: [] }
                if (url === '/api/ssh/info') return { enabled: false, host: '', port: 0, username: '', fingerprint: '', command: '', connectionStats: null }
                return {}
            })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { checkTunnelHealth, tunnelStatus, tunnelError } = usePortForward()

            // Pre-set some state
            tunnelStatus.value = 'ok' as any
            tunnelError.value = 'old error'

            await checkTunnelHealth()

            // Should be reset
            expect(tunnelError.value).toBe('')
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

        it('calls Android native sandbox browser in app mode', async () => {
            mockIsAppMode.value = true
            const mockOpenInSandbox = vi.fn()
            ;(window as any).AndroidNative = { openInSandbox: mockOpenInSandbox }

            const { usePortForward } = await import('@/composables/usePortForward')
            const { openPort } = usePortForward()

            openPort(3000, 'http')

            expect(mockOpenInSandbox).toHaveBeenCalledWith(3000, 'http', '')

            delete (window as any).AndroidNative
        })

        it('falls back to openInBrowser when sandbox not available', async () => {
            mockIsAppMode.value = true
            const mockOpenInBrowser = vi.fn()
            ;(window as any).AndroidNative = { openInBrowser: mockOpenInBrowser }

            const { usePortForward } = await import('@/composables/usePortForward')
            const { openPort } = usePortForward()

            openPort(3000, 'https')

            expect(mockOpenInBrowser).toHaveBeenCalledWith(3000, 'https', '')

            delete (window as any).AndroidNative
        })
    })

    describe('registerPort', () => {
        it('posts port to API and refreshes', async () => {
            mockApiPost.mockResolvedValue({})
            mockApiGet.mockResolvedValue({ ports: [] })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { registerPort } = usePortForward()

            await registerPort(3000, 'App', 'http')

            expect(mockApiPost).toHaveBeenCalledWith('/api/proxy/ports', {
                port: 3000, host: '', name: 'App', protocol: 'http',
            })
        })
    })

    describe('unregisterPort', () => {
        it('deletes port from API and refreshes', async () => {
            mockApiDelete.mockResolvedValue({})
            mockApiGet.mockResolvedValue({ ports: [] })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { unregisterPort } = usePortForward()

            await unregisterPort(3000)

            expect(mockApiDelete).toHaveBeenCalledWith('/api/proxy/ports?port=3000')
        })
    })

    describe('detectPorts', () => {
        it('fetches detected ports from API', async () => {
            const detected = [
                { port: 8080, protocol: 'http', processName: 'node', processArgs: '' },
            ]
            mockApiGet.mockResolvedValue({ ports: detected })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { detectPorts, detectedPorts } = usePortForward()

            await detectPorts()

            expect(mockApiGet).toHaveBeenCalledWith('/api/proxy/detect')
            expect(detectedPorts.value).toHaveLength(1)
        })
    })

    describe('ensurePortRegistered', () => {
        it('returns immediately if port already exists', async () => {
            mockApiGet.mockResolvedValue({
                ports: [{ port: 3000, name: 'App', protocol: 'http', autoDetect: false, active: true }],
            })

            const { usePortForward } = await import('@/composables/usePortForward')
            const { ensurePortRegistered, loadPorts } = usePortForward()

            // First load ports so the port exists
            await loadPorts()

            // Should not call registerPort
            await ensurePortRegistered(3000, 'http')

            expect(mockApiPost).not.toHaveBeenCalled()
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

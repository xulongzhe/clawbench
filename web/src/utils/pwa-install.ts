/**
 * PWA 安装管理器
 * 用于处理 PWA 安装提示和检测
 */

export interface BeforeInstallPromptEvent extends Event {
  readonly platforms: string[]
  readonly userChoice: Promise<{
    outcome: 'accepted' | 'dismissed'
    platform: string
  }>
  prompt(): Promise<void>
}

export class PWAInstallManager {
  private deferredPrompt: BeforeInstallPromptEvent | null = null
  private isInstalled = false

  constructor() {
    this.checkIfInstalled()
    this.setupListeners()
  }

  private checkIfInstalled() {
    // Check if app is already installed
    if (window.matchMedia('(display-mode: standalone)').matches) {
      this.isInstalled = true
      console.log('[PWA] App is already installed')
    }

    // Check for iOS standalone mode
    if ((navigator as any).standalone === true) {
      this.isInstalled = true
      console.log('[PWA] App is running in iOS standalone mode')
    }
  }

  private setupListeners() {
    window.addEventListener('beforeinstallprompt', (e) => {
      e.preventDefault()
      this.deferredPrompt = e as BeforeInstallPromptEvent
      console.log('[PWA] Install prompt captured')
      this.showInstallButton()
    })

    window.addEventListener('appinstalled', () => {
      console.log('[PWA] App installed successfully')
      this.isInstalled = true
      this.deferredPrompt = null
      this.hideInstallButton()
    })
  }

  private showInstallButton() {
    // Dispatch custom event that Vue components can listen to
    window.dispatchEvent(
      new CustomEvent('pwa-install-available', {
        detail: { available: true },
      })
    )
  }

  private hideInstallButton() {
    window.dispatchEvent(
      new CustomEvent('pwa-install-available', {
        detail: { available: false },
      })
    )
  }

  /**
   * 检查是否可以安装
   */
  canInstall(): boolean {
    return this.deferredPrompt !== null && !this.isInstalled
  }

  /**
   * 检查是否已安装
   */
  isAppInstalled(): boolean {
    return this.isInstalled
  }

  /**
   * 触发安装提示
   */
  async promptInstall(): Promise<boolean> {
    if (!this.deferredPrompt) {
      console.log('[PWA] No install prompt available')
      return false
    }

    try {
      await this.deferredPrompt.prompt()
      const { outcome } = await this.deferredPrompt.userChoice

      if (outcome === 'accepted') {
        console.log('[PWA] User accepted install')
        return true
      } else {
        console.log('[PWA] User dismissed install')
        return false
      }
    } catch (error) {
      console.error('[PWA] Install prompt error:', error)
      return false
    }
  }

  /**
   * 获取 iOS 安装指南
   */
  getIOSInstallInstructions(): string[] {
    return [
      '1. 点击 Safari 底部的"分享"按钮',
      '2. 向下滑动并点击"添加到主屏幕"',
      '3. 点击右上角的"添加"按钮',
      '4. 在主屏幕上找到 ClawBench 图标',
    ]
  }
}

// 全局单例
export const pwaInstallManager = new PWAInstallManager()

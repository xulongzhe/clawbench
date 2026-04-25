import { ref } from 'vue'

// Browser notification permission state
const permission = ref<NotificationPermission>('default')

// Track active notifications for cleanup
const activeNotifications = new Set<Notification>()

/**
 * Request notification permission
 */
export async function requestNotificationPermission(): Promise<NotificationPermission> {
  if (!('Notification' in window)) {
    console.warn('This browser does not support desktop notifications')
    return 'denied'
  }

  if (Notification.permission === 'granted') {
    permission.value = 'granted'
    return 'granted'
  }

  if (Notification.permission !== 'denied') {
    const result = await Notification.requestPermission()
    permission.value = result
    return result
  }

  permission.value = 'denied'
  return 'denied'
}

/**
 * Show browser notification
 */
export function showBrowserNotification(
  title: string,
  options?: {
    body?: string
    icon?: string
    badge?: string
    tag?: string
    onClick?: () => void
  }
): void {
  // Don't show notifications when page is visible and focused
  if (document.visibilityState === 'visible' && document.hasFocus()) {
    return
  }

  // Check permission
  if (!('Notification' in window)) {
    console.warn('Notifications not supported')
    return
  }

  if (Notification.permission !== 'granted') {
    console.warn('Notification permission not granted')
    return
  }

  // Create notification with unique tag to avoid replacement
  const notification = new Notification(title, {
    body: options?.body || '',
    icon: options?.icon || '/favicon.ico',
    badge: options?.badge || '/favicon.ico',
    tag: options?.tag || `clawbench-${Date.now()}`,
    requireInteraction: false,
    silent: false,
  })

  // Track for cleanup
  activeNotifications.add(notification)
  notification.onclose = () => {
    activeNotifications.delete(notification)
  }

  // Handle click
  if (options?.onClick) {
    notification.onclick = () => {
      window.focus()
      options.onClick()
      notification.close()
    }
  }
}

/**
 * Close all active notifications
 */
export function closeAllNotifications(): void {
  for (const n of activeNotifications) {
    n.close()
  }
  activeNotifications.clear()
}

/**
 * useNotification()
 *
 * Composable for browser notifications
 */
export function useNotification() {
  return {
    permission,
    requestPermission: requestNotificationPermission,
    show: showBrowserNotification,
    closeAll: closeAllNotifications,
  }
}

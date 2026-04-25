import { ref } from 'vue'

// Browser notification permission state
const permission = ref<NotificationPermission>('default')

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
  // Check permission
  if (!('Notification' in window)) {
    console.warn('Notifications not supported')
    return
  }

  if (Notification.permission !== 'granted') {
    console.warn('Notification permission not granted')
    return
  }

  // Create notification
  const notification = new Notification(title, {
    body: options?.body || '',
    icon: options?.icon || '/favicon.ico',
    badge: options?.badge || '/favicon.ico',
    tag: options?.tag || 'ai-reply',
    requireInteraction: false,
    silent: false,
  })

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
 * useNotification()
 *
 * Composable for browser notifications
 */
export function useNotification() {
  return {
    permission,
    requestPermission: requestNotificationPermission,
    show: showBrowserNotification,
  }
}

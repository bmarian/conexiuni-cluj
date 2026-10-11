import {useSettingsStore} from '@/stores/settings.ts'

export type NotifyPermission = 'granted' | 'denied' | 'unsupported'

export const notificationsSupported = () => typeof window !== 'undefined' && 'Notification' in window

// Only one tab alerts.
let alerting = typeof navigator === 'undefined' || !navigator.locks
if (!alerting) {
  void navigator.locks.request('conexiuni-alerts', () => {
    alerting = true
    return new Promise<void>(() => {})
  })
}

const watched = () => typeof document !== 'undefined' && document.visibilityState === 'visible' && document.hasFocus()

export async function askNotifications(): Promise<NotifyPermission> {
  if (!notificationsSupported()) return 'unsupported'
  if (Notification.permission === 'granted') return 'granted'
  if (Notification.permission === 'denied') return 'denied'
  try {
    return (await Notification.requestPermission()) === 'granted' ? 'granted' : 'denied'
  } catch {
    return 'denied'
  }
}

// No PWA worker in dev.
export async function notificationWorker(): Promise<ServiceWorkerRegistration | null> {
  if (!('serviceWorker' in navigator)) return null
  try {
    if (import.meta.env.DEV && !(await navigator.serviceWorker.getRegistration())) {
      await navigator.serviceWorker.register('/push-sw.js')
    }
    const reg = await navigator.serviceWorker.getRegistration()
    return reg ? await navigator.serviceWorker.ready : null
  } catch {
    return null
  }
}

export interface Notice {
  title: string
  body?: string
  tag?: string
  url?: string
  buzz?: boolean
  silent?: boolean
}

export async function notify(notice: Notice): Promise<boolean> {
  if (!alerting || !useSettingsStore().busAlerts) return false
  if (notice.buzz) navigator.vibrate?.([200, 100, 200])
  if (!notificationsSupported() || Notification.permission !== 'granted') return false
  const silent = notice.silent || watched()
  const options: NotificationOptions & {renotify?: boolean} = {
    body: notice.body,
    tag: notice.tag,
    renotify: Boolean(notice.tag) && !silent,
    silent,
    icon: '/pwa-192x192.png',
    data: {url: notice.url ?? '/'},
  }
  const reg = await notificationWorker()
  try {
    if (reg) {
      await reg.showNotification(notice.title, options)
      return true
    }
    const shown = new Notification(notice.title, options)
    shown.onclick = () => {
      window.focus()
      if (notice.url) window.location.assign(notice.url)
      shown.close()
    }
    return true
  } catch {
    return false
  }
}

export async function clearNotice(tag: string) {
  const reg = await notificationWorker()
  const shown = await reg?.getNotifications({tag}).catch(() => []) ?? []
  shown.forEach((n) => n.close())
}

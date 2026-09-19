import {ref, watch} from 'vue'
import {defineStore} from 'pinia'
import {apiPost, apiRequest} from '@/utils/api'
import {useRouteUpdatesStore} from '@/stores/routeUpdates'
import {useSettingsStore} from '@/stores/settings'

const ENABLED_KEY = 'settings.pushNotifications'
const SYNC_DELAY_MS = 1000

export type PushResult = 'enabled' | 'unsupported' | 'denied' | 'brave' | 'failed'

function base64UrlToBytes(value: string): Uint8Array<ArrayBuffer> {
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=')
  return Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
}

function sameKey(a: ArrayBuffer | null, b: Uint8Array): boolean {
  if (!a || a.byteLength !== b.length) return false
  return new Uint8Array(a).every((v, i) => v === b[i])
}

export const usePushStore = defineStore('push', () => {
  const settings = useSettingsStore()
  const routeUpdates = useRouteUpdatesStore()

  const supported = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window
  const enabled = ref(supported && localStorage.getItem(ENABLED_KEY) === 'true')
  const busy = ref(false)
  let syncTimer: ReturnType<typeof setTimeout> | null = null

  function setEnabled(value: boolean) {
    enabled.value = value
    localStorage.setItem(ENABLED_KEY, value ? 'true' : 'false')
  }

  // vite-plugin-pwa registers no worker in dev, so the push handler stands in for it there.
  async function registration(): Promise<ServiceWorkerRegistration | null> {
    if (import.meta.env.DEV) await navigator.serviceWorker.register('/push-sw.js')
    else if (!(await navigator.serviceWorker.getRegistration())) return null
    return navigator.serviceWorker.ready
  }

  async function subscription(reg: ServiceWorkerRegistration): Promise<PushSubscription> {
    const {public_key} = await apiRequest<{ public_key: string }>('push/key')
    const key = base64UrlToBytes(public_key)
    const existing = await reg.pushManager.getSubscription()
    if (existing && sameKey(existing.options.applicationServerKey, key)) return existing
    await existing?.unsubscribe()
    return reg.pushManager.subscribe({userVisibleOnly: true, applicationServerKey: key})
  }

  async function register(sub: PushSubscription) {
    await apiPost('push/subscribe', {
      ...sub.toJSON(),
      routes: routeUpdates.followed,
      locale: settings.locale,
    })
  }

  async function enable(): Promise<PushResult> {
    if (!supported) return 'unsupported'
    busy.value = true
    try {
      if (await Notification.requestPermission() !== 'granted') return 'denied'
      const reg = await registration()
      if (!reg) return 'failed'
      await register(await subscription(reg))
      setEnabled(true)
      return 'enabled'
    } catch (err) {
      console.warn('Push subscribe failed:', err)
      // Brave keeps Google's push service off until the user opts in.
      if ('brave' in navigator && err instanceof DOMException && err.name === 'AbortError') return 'brave'
      return 'failed'
    } finally {
      busy.value = false
    }
  }

  async function disable() {
    setEnabled(false)
    if (!supported) return
    try {
      const sub = await (await registration())?.pushManager.getSubscription()
      if (!sub) return
      const {endpoint} = sub
      await sub.unsubscribe()
      await apiPost('push/unsubscribe', {endpoint})
    } catch (err) {
      console.warn('Push unsubscribe failed:', err)
    }
  }

  async function sync() {
    if (!enabled.value) return
    if (!settings.showTimetableChanges || Notification.permission !== 'granted') return disable()
    try {
      const reg = await registration()
      if (reg) await register(await subscription(reg))
    } catch (err) {
      console.warn('Push sync failed:', err)
    }
  }

  function scheduleSync() {
    if (!enabled.value) return
    if (syncTimer) clearTimeout(syncTimer)
    syncTimer = setTimeout(() => void sync(), SYNC_DELAY_MS)
  }

  watch(() => [...routeUpdates.followed], scheduleSync)
  watch(() => settings.locale, scheduleSync)
  watch(() => settings.showTimetableChanges, scheduleSync)

  return {supported, enabled, busy, enable, disable, sync}
})

import {apiRequest} from '@/utils/api.ts'

export const pushSupported = () =>
  typeof window !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window

function base64UrlToBytes(value: string): Uint8Array<ArrayBuffer> {
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=')
  return Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
}

function sameKey(a: ArrayBuffer | null, b: Uint8Array): boolean {
  if (!a || a.byteLength !== b.length) return false
  return new Uint8Array(a).every((v, i) => v === b[i])
}

// No PWA worker in dev.
export async function pushRegistration(): Promise<ServiceWorkerRegistration | null> {
  if (import.meta.env.DEV) await navigator.serviceWorker.register('/push-sw.js')
  else if (!(await navigator.serviceWorker.getRegistration())) return null
  return navigator.serviceWorker.ready
}

export async function pushSubscription(reg: ServiceWorkerRegistration): Promise<PushSubscription> {
  const {public_key} = await apiRequest<{ public_key: string }>('push/key')
  const key = base64UrlToBytes(public_key)
  const existing = await reg.pushManager.getSubscription()
  if (existing && sameKey(existing.options.applicationServerKey, key)) return existing
  await existing?.unsubscribe()
  return reg.pushManager.subscribe({userVisibleOnly: true, applicationServerKey: key})
}

// Read by push-sw.js: an alert already on its way for a cancelled reminder is dropped there.
const CANCELLED_CACHE = 'cancelled-reminders'

/** Takes back a cancelled reminder's alerts; [untilMs] is its next bus, after which no push for it can arrive. */
export async function retractReminder(id: string, untilMs: number) {
  const tag = `reminder-${id}`
  try {
    const cache = await caches.open(CANCELLED_CACHE)
    for (const request of await cache.keys()) {
      if (Number(await (await cache.match(request))?.text()) < Date.now()) await cache.delete(request)
    }
    await cache.put(`/cancelled/${tag}`, new Response(String(untilMs)))
  } catch { /* noop */ }
  try {
    const reg = await navigator.serviceWorker?.getRegistration()
    for (const shown of (await reg?.getNotifications({tag})) ?? []) shown.close()
  } catch { /* noop */ }
}

export async function currentSubscription(): Promise<PushSubscription | null> {
  if (!pushSupported()) return null
  try {
    const reg = await pushRegistration()
    return (await reg?.pushManager.getSubscription()) ?? null
  } catch {
    return null
  }
}

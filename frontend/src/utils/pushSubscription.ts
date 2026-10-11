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

export async function currentSubscription(): Promise<PushSubscription | null> {
  if (!pushSupported()) return null
  try {
    const reg = await pushRegistration()
    return (await reg?.pushManager.getSubscription()) ?? null
  } catch {
    return null
  }
}

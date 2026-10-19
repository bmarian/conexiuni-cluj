import {
  createTransferKey,
  createTransferLink,
  deleteTransferLink,
  isSealed,
  openTransfer,
  readTransferLink,
  sendToTransferLink,
} from '@/utils/transferLink.ts'
import {LEGACY_HOST, SITE_ORIGIN} from '@/utils/site.ts'
import {apiPath} from '@/utils/city.ts'

const HASH_PARAM = 'moved'
const IMPORTED_KEY = 'moved.at'
const NOTICE_KEY = 'moved.notice'
// Rebuilt by the app, or only valid on the host that wrote them.
const LOCAL_ONLY = /^(cache:|network:|admin:)/

type Moved = { storage: Record<string, string>; pushEndpoint: string | null; standalone: boolean }

export type MoveNotice = {
  standalone: boolean
  routeAlerts: boolean
  reminders: boolean
  pushEndpoint: string | null
}

const isMoved = (value: unknown): value is Moved => {
  const moved = value as Moved
  return typeof moved === 'object' && moved !== null
    && typeof moved.storage === 'object' && moved.storage !== null
    && Object.values(moved.storage).every((v) => typeof v === 'string')
}

function takeToken(): string | null {
  if (typeof location === 'undefined' || !location.hash) return null
  const hash = new URLSearchParams(location.hash.slice(1))
  const token = hash.get(HASH_PARAM)
  if (!token) return null
  hash.delete(HASH_PARAM)
  const rest = hash.toString()
  history.replaceState(history.state, '', location.pathname + location.search + (rest ? `#${rest}` : ''))
  return token
}

// Runs on import, before the router copies the URL into its first history entry.
const token = takeToken()

function savedStorage(): Record<string, string> | null {
  try {
    const saved: Record<string, string> = {}
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (key && !LOCAL_ONLY.test(key)) saved[key] = localStorage.getItem(key) ?? ''
    }
    return Object.keys(saved).length ? saved : null
  } catch {
    return null
  }
}

async function pushEndpoint(): Promise<string | null> {
  try {
    const reg = await navigator.serviceWorker?.getRegistration()
    return (await reg?.pushManager.getSubscription())?.endpoint ?? null
  } catch {
    return null
  }
}

function sendMoveEvent(key: 'sent' | 'empty' | 'received') {
  const body = JSON.stringify({metric: 'host_move', key})
  navigator.sendBeacon?.(apiPath('stats/event'), new Blob([body], {type: 'application/json'}))
}

// True when the page is on its way to the new host. Anything that fails keeps the app here.
export async function leaveLegacyHost(): Promise<boolean> {
  if (location.host !== LEGACY_HOST) return false
  const hash = new URLSearchParams(location.hash.slice(1))
  const storage = savedStorage()
  if (storage) {
    const key = createTransferKey()
    if (!key) return false
    try {
      const link = await createTransferLink()
      const moved: Moved = {
        storage,
        pushEndpoint: await pushEndpoint(),
        standalone: matchMedia('(display-mode: standalone)').matches,
      }
      if (await sendToTransferLink(link.code, moved, key) !== 'sent') return false
      hash.set(HASH_PARAM, `${link.code}.${link.secret}.${key}`)
    } catch {
      return false
    }
  }
  sendMoveEvent(storage ? 'sent' : 'empty')
  const rest = hash.toString()
  location.replace(SITE_ORIGIN + location.pathname + location.search + (rest ? `#${rest}` : ''))
  return true
}

// Only the first move is applied, so a later visit to an old link can't overwrite newer data.
export async function arriveFromLegacyHost(): Promise<void> {
  const [code, secret, key] = token?.split('.') ?? []
  if (!code || !secret || !key) return
  const link = {code, secret, expiresIn: 0}
  try {
    if (localStorage.getItem(IMPORTED_KEY)) return
    const read = await readTransferLink(link)
    // Anyone who guesses the code can put plain data on it.
    if (read.state !== 'received' || !isSealed(read.data)) return
    const moved = await openTransfer(read.data, key)
    if (!isMoved(moved)) return
    for (const [name, value] of Object.entries(moved.storage)) localStorage.setItem(name, value)
    const notice: MoveNotice = {
      standalone: moved.standalone === true,
      routeAlerts: moved.storage['settings.pushNotifications'] === 'true',
      reminders: moved.storage['reminders.synced'] === 'true',
      pushEndpoint: typeof moved.pushEndpoint === 'string' ? moved.pushEndpoint : null,
    }
    localStorage.setItem(NOTICE_KEY, JSON.stringify(notice))
    localStorage.setItem(IMPORTED_KEY, String(Date.now()))
    sendMoveEvent('received')
  } catch {
    /* noop */
  } finally {
    deleteTransferLink(link)
  }
}

export function moveNotice(): MoveNotice | null {
  try {
    return JSON.parse(localStorage.getItem(NOTICE_KEY) ?? 'null')
  } catch {
    return null
  }
}

export function clearMoveNotice() {
  try {
    localStorage.removeItem(NOTICE_KEY)
  } catch {
    /* noop */
  }
}

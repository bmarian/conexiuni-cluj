import {beforeAll, beforeEach, describe, expect, it, vi} from 'vitest'

// CacheStorage and Cache, as far as the page and push-sw.js use them.
const stores = new Map<string, Map<string, string>>()
const key = (request: string | Request) => new URL(typeof request === 'string' ? request : request.url, 'https://ceautobuziau.ro').pathname
const cacheNamed = (name: string) => {
  if (!stores.has(name)) stores.set(name, new Map())
  const entries = stores.get(name)!
  return {
    keys: async () => [...entries.keys()].map((path) => new Request(`https://ceautobuziau.ro${path}`)),
    match: async (request: string | Request) => (entries.has(key(request)) ? new Response(entries.get(key(request))) : undefined),
    put: async (request: string | Request, response: Response) => { entries.set(key(request), await response.text()) },
    delete: async (request: string | Request) => entries.delete(key(request)),
  }
}

const shown = vi.fn(async () => {})
const closed = vi.fn()
const handlers: Record<string, (event: unknown) => void> = {}

function push(tag: string) {
  let done: Promise<unknown> = Promise.resolve()
  handlers.push!({data: {json: () => ({title: 'Leave now', tag})}, waitUntil: (p: Promise<unknown>) => { done = p }})
  return done
}

describe('a cancelled reminder', () => {
  beforeAll(async () => {
    vi.stubGlobal('caches', {
      open: async (name: string) => cacheNamed(name),
      match: async (request: string, options: {cacheName: string}) => cacheNamed(options.cacheName).match(request),
    })
    vi.stubGlobal('navigator', {serviceWorker: {getRegistration: async () => ({
      getNotifications: async ({tag}: {tag: string}) => (tag === 'reminder-gone' ? [{close: closed}] : []),
    })}})
    vi.stubGlobal('self', {
      addEventListener: (type: string, fn: (event: unknown) => void) => { handlers[type] = fn },
      registration: {showNotification: shown},
    })
    // @ts-expect-error plain service worker script
    await import('../../../public/push-sw.js')
  })
  beforeEach(() => {
    shown.mockClear()
    closed.mockClear()
  })

  it('closes its alert on screen and drops one still on its way', async () => {
    const {retractReminder} = await import('@/utils/pushSubscription.ts')
    await retractReminder('gone', Date.now() + 60_000)
    expect(closed).toHaveBeenCalledOnce()
    await push('reminder-gone')
    expect(shown).not.toHaveBeenCalled()
    await push('reminder-kept')
    expect(shown).toHaveBeenCalledOnce()
  })

  it('forgets the cancellation once its bus has gone', async () => {
    const {retractReminder} = await import('@/utils/pushSubscription.ts')
    await retractReminder('old', Date.now() - 1)
    await push('reminder-old')
    expect(shown).toHaveBeenCalledOnce()
    await retractReminder('newer', Date.now() + 60_000)
    expect([...stores.get('cancelled-reminders')!.keys()]).not.toContain('/cancelled/reminder-old')
  })
})

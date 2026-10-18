import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {createPinia, setActivePinia} from 'pinia'
import {nextTick} from 'vue'

class MemoryStorage {
  private items = new Map<string, string>()
  get length() { return this.items.size }
  key(i: number) { return [...this.items.keys()][i] ?? null }
  getItem(key: string) { return this.items.get(key) ?? null }
  setItem(key: string, value: string) { this.items.set(key, String(value)) }
  removeItem(key: string) { this.items.delete(key) }
  clear() { this.items.clear() }
}

const sub = {endpoint: 'https://fcm.googleapis.com/fcm/send/x', toJSON: () => ({endpoint: 'https://fcm.googleapis.com/fcm/send/x'}), unsubscribe: vi.fn(async () => true)}
const posts: {url: string, city?: string, routes?: string[]}[] = []

vi.mock('@/utils/api', () => ({
  apiPost: vi.fn(async (url: string, body: {routes?: string[]}, city?: string) => { posts.push({url, city, routes: body.routes}) }),
  apiRequest: vi.fn(async () => []),
  readCachedList: () => [],
  writeCachedList: () => {},
}))
vi.mock('@/utils/pushSubscription', () => ({
  pushSupported: () => true,
  pushRegistration: async () => ({pushManager: {getSubscription: async () => sub}}),
  pushSubscription: async () => sub,
}))
vi.mock('@/stores/settings', () => ({useSettingsStore: () => ({locale: 'ro', showToast: () => {}})}))

describe('push across cities', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', new MemoryStorage())
    vi.stubGlobal('Notification', {permission: 'granted', requestPermission: async () => 'granted'})
    vi.stubGlobal('fetch', async () => ({ok: false}))
    localStorage.setItem('settings.pushNotifications', 'true')
    localStorage.setItem('cluj:follows:routes', JSON.stringify(['24']))
    posts.length = 0
    sub.unsubscribe.mockClear()
    vi.useFakeTimers()
    vi.resetModules()
    setActivePinia(createPinia())
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  async function stores() {
    const city = await import('@/utils/city.ts')
    const {usePushStore} = await import('@/stores/push.ts')
    const {useRouteUpdatesStore} = await import('@/stores/routeUpdates.ts')
    useRouteUpdatesStore()
    return {city, push: usePushStore()}
  }

  it('keeps Cluj alerts when another city is opened', async () => {
    const {city, push} = await stores()
    city.setCity('iasi')
    await nextTick()
    await vi.advanceTimersByTimeAsync(2000)
    expect(posts).toEqual([])
    await push.sync('iasi')
    expect(posts).toEqual([{url: 'push/unsubscribe', city: 'iasi', routes: undefined}])
    expect(sub.unsubscribe).not.toHaveBeenCalled()
    expect(push.enabled).toBe(true)
  })

  it('turns alerts off in every city and drops the browser subscription', async () => {
    const {city, push} = await stores()
    city.setCity('iasi')
    await push.disable()
    expect(posts.map((p) => `${p.url}@${p.city}`).sort()).toEqual(['push/unsubscribe@cluj', 'push/unsubscribe@iasi'])
    expect(sub.unsubscribe).toHaveBeenCalledOnce()
  })

  it('keeps the subscription while a city has reminders', async () => {
    localStorage.setItem('cluj:reminders.v1', JSON.stringify([{id: 'a', slots: {}}]))
    const {city, push} = await stores()
    city.setCity('iasi')
    await push.disable()
    expect(posts).toContainEqual({url: 'push/subscribe', city: 'cluj', routes: []})
    expect(sub.unsubscribe).not.toHaveBeenCalled()
  })
})

import {beforeEach, describe, expect, it, vi} from 'vitest'
import {createPinia, setActivePinia} from 'pinia'
import {ref} from 'vue'

class MemoryStorage {
  private items = new Map<string, string>()
  get length() { return this.items.size }
  key(i: number) { return [...this.items.keys()][i] ?? null }
  getItem(key: string) { return this.items.get(key) ?? null }
  setItem(key: string, value: string) { this.items.set(key, String(value)) }
  removeItem(key: string) { this.items.delete(key) }
  clear() { this.items.clear() }
}

vi.mock('vue-i18n', async (importOriginal) => ({...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({locale: ref('ro')})}))
vi.mock('@/utils/api', () => ({apiPost: async () => {}, apiRequest: async () => [], readCachedList: () => [], writeCachedList: () => {}}))
vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => new Proxy({locale: 'ro'} as Record<string, unknown>, {
    get: (target, key: string) => (key in target ? target[key] : () => false),
  }),
}))

const saved = (key: string) => JSON.parse(localStorage.getItem(key) ?? 'null')

describe('settings transfer across cities', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', new MemoryStorage())
    vi.stubGlobal('fetch', async () => ({ok: false}))
    vi.resetModules()
    setActivePinia(createPinia())
  })

  async function transfer() {
    const city = await import('@/utils/city.ts')
    const {useSettingsTransfer, describeExport} = await import('@/composables/useSettingsTransfer.ts')
    return {city, describeExport, ...useSettingsTransfer()}
  }

  it('sends every city and puts each back in its own city', async () => {
    localStorage.setItem('cluj:favorites:stops', '[391]')
    localStorage.setItem('cluj:follows:routes', '["24"]')
    localStorage.setItem('iasi:favorites:stops', '[12]')
    const sender = await transfer()
    const data = JSON.parse(JSON.stringify(sender.buildExport()))
    expect(data.favorites.stops).toEqual([391])
    expect(Object.keys(data.cities).sort()).toEqual(['cluj', 'iasi'])
    expect(sender.describeExport(data)).toMatchObject({stops: 2, followed: 1})

    localStorage.clear()
    localStorage.setItem('bucuresti:favorites:stops', '[7]')
    vi.resetModules()
    setActivePinia(createPinia())
    const receiver = await transfer()
    receiver.city.setCity('bucuresti')
    receiver.applyExport(data)
    expect(saved('cluj:favorites:stops')).toEqual([391])
    expect(saved('cluj:follows:routes')).toEqual(['24'])
    expect(saved('iasi:favorites:stops')).toEqual([12])
    expect(saved('bucuresti:favorites:stops')).toEqual([])
  })

  it('reads an export from before cities as Cluj only', async () => {
    localStorage.setItem('iasi:favorites:stops', '[12]')
    const receiver = await transfer()
    receiver.city.setCity('iasi')
    receiver.applyExport({version: 1, settings: {}, favorites: {stops: [391]}, followedLines: ['24']})
    expect(saved('cluj:favorites:stops')).toEqual([391])
    expect(saved('cluj:follows:routes')).toEqual(['24'])
    expect(saved('iasi:favorites:stops')).toEqual([12])
  })

  it('ignores cities that are not slugs', async () => {
    const receiver = await transfer()
    receiver.applyExport({version: 1, settings: {}, favorites: {}, cities: {'../admin': {favorites: {stops: [1]}}}})
    expect([...Array(localStorage.length).keys()].map((i) => localStorage.key(i)).filter((k) => k?.includes('admin'))).toEqual([])
  })
})

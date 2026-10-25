import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {effectScope, nextTick, ref} from 'vue'
import type {Place} from '@/utils/places.ts'

const calls: {query: string; signal?: AbortSignal; resolve: (places: Place[]) => void}[] = []

vi.mock('@/utils/places.ts', () => ({
  searchPlaces: (query: string, _limit: number, signal?: AbortSignal) => new Promise<Place[]>((resolve) => calls.push({query, signal, resolve})),
}))

const {usePlaceSearch} = await import('@/composables/usePlaceSearch.ts')

const place = (label: string) => ({label, lat: '46.77', lon: '23.6'}) as Place

function setup(online = true) {
  const isOnline = ref(online)
  const scope = effectScope()
  const search = scope.run(() => usePlaceSearch(isOnline))!
  return {isOnline, scope, search}
}

describe('usePlaceSearch', () => {
  beforeEach(() => {
    calls.length = 0
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('searches once typing pauses', async () => {
    const {search, scope} = setup()
    search.openSearchField('destination')
    for (const q of ['Pia', 'Piat', 'Piata']) {
      search.searchQuery.value = q
      await nextTick()
      vi.advanceTimersByTime(100)
    }
    expect(calls).toHaveLength(0)
    vi.advanceTimersByTime(50)
    expect(calls.map((c) => c.query)).toEqual(['Piata'])
    expect(search.isSearching.value).toBe(true)
    calls[0]!.resolve([place('Piata Unirii')])
    await vi.runAllTimersAsync()
    expect(search.searchResults.value.map((p) => p.label)).toEqual(['Piata Unirii'])
    expect(search.isSearching.value).toBe(false)
    scope.stop()
  })

  it('clears the results for queries under three letters', async () => {
    const {search, scope} = setup()
    search.searchResults.value = [place('old')]
    search.searchQuery.value = 'Pi'
    await nextTick()
    vi.advanceTimersByTime(150)
    expect(calls).toHaveLength(0)
    expect(search.searchResults.value).toEqual([])
    scope.stop()
  })

  it('does not search destinations offline', async () => {
    const {search, isOnline, scope} = setup()
    search.openSearchField('destination')
    isOnline.value = false
    search.searchQuery.value = 'Piata'
    await nextTick()
    vi.advanceTimersByTime(150)
    expect(calls).toHaveLength(0)
    expect(search.activeSearchField.value).toBe('destination')
    scope.stop()
  })

  it('only opens the origin field offline', () => {
    const {search, scope} = setup(false)
    search.openSearchField('destination')
    expect(search.activeSearchField.value).toBeNull()
    search.openSearchField('origin')
    expect(search.activeSearchField.value).toBe('origin')
    scope.stop()
  })

  it('drops the answer to a query that was typed over', async () => {
    const {search, scope} = setup()
    search.searchQuery.value = 'Piata'
    const first = search.searchNow()
    search.searchQuery.value = 'Gara'
    const second = search.searchNow()
    expect(calls.map((c) => c.query)).toEqual(['Piata', 'Gara'])
    expect(calls[0]!.signal?.aborted).toBe(true)
    calls[1]!.resolve([place('Gara')])
    calls[0]!.resolve([place('Piata Unirii')])
    await Promise.all([first, second])
    expect(search.searchResults.value.map((p) => p.label)).toEqual(['Gara'])
    expect(search.isSearching.value).toBe(false)
    scope.stop()
  })

  it('searches right away on demand and closes cleanly', async () => {
    const {search, scope} = setup()
    search.openSearchField('origin')
    search.searchQuery.value = 'Piata'
    await nextTick()
    const pending = search.searchNow()
    vi.advanceTimersByTime(150)
    expect(calls).toHaveLength(1)
    calls[0]!.resolve([place('Piata Unirii')])
    await pending
    search.closeSearchField()
    expect(search.activeSearchField.value).toBeNull()
    expect(search.searchQuery.value).toBe('')
    expect(search.searchResults.value).toEqual([])
    scope.stop()
  })
})

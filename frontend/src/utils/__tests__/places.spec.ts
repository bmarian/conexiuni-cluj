import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {MIN_GAP_MS, placeFromFeature, resetPlaces, reversePlace, searchPlaces} from '@/utils/places.ts'

const feature = (props: Record<string, unknown>, lon = 23.6, lat = 46.77) =>
  ({geometry: {coordinates: [lon, lat]}, properties: props})

function respond(features: unknown) {
  return {ok: true, json: async () => ({features})} as Response
}

describe('placeFromFeature', () => {
  it('labels a named place on a street with its number and district', () => {
    expect(placeFromFeature(feature({name: 'Iulius Mall', street: 'Strada Alexandru Vaida Voevod', housenumber: '53B', district: 'Gheorgheni', osm_type: 'W', osm_id: 42}), 0))
      .toEqual({id: 'W:42', lat: '46.77', lon: '23.6', label: 'Iulius Mall, Strada Alexandru Vaida Voevod, 53B, Gheorgheni'})
  })

  it('does not repeat a street that is also the name', () => {
    expect(placeFromFeature(feature({name: 'strada horea', street: 'Strada Horea', housenumber: '3'}), 0)!.label).toBe('Strada Horea, 3')
  })

  it('falls back to the name, then to coordinates', () => {
    expect(placeFromFeature(feature({name: 'Cluj Arena'}), 0)!.label).toBe('Cluj Arena')
    expect(placeFromFeature(feature({}), 2)).toEqual({id: '46.77:23.6:2', lat: '46.77', lon: '23.6', label: '46.7700, 23.6000'})
  })

  it('skips features without usable coordinates', () => {
    expect(placeFromFeature({properties: {name: 'x'}}, 0)).toBeNull()
    expect(placeFromFeature({geometry: {coordinates: [23.6]}}, 0)).toBeNull()
    expect(placeFromFeature({geometry: {coordinates: [Number.NaN, 46]}}, 0)).toBeNull()
  })
})

describe('Photon requests', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.useFakeTimers()
    resetPlaces()
    fetchMock = vi.fn(async () => respond([feature({name: 'Iulius Mall'})]))
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('asks Photon for the Cluj area, ranked around the centre, never around the user', async () => {
    await searchPlaces('iulius', 'ro', 3)
    const url = new URL(fetchMock.mock.calls[0]![0] as string)
    expect(url.origin + url.pathname).toBe('https://photon.komoot.io/api/')
    expect(url.searchParams.get('q')).toBe('iulius')
    expect(url.searchParams.get('bbox')).toBe('22.75,46.38,24.27,47.50')
    expect(url.searchParams.get('lat')).toBe('46.77')
    expect(url.searchParams.get('lon')).toBe('23.59')
    expect(url.searchParams.get('limit')).toBe('3')
    expect(url.searchParams.get('lang')).toBe('default')
  })

  it('asks for English names only in English', async () => {
    await searchPlaces('iulius', 'en', 3)
    expect(new URL(fetchMock.mock.calls[0]![0] as string).searchParams.get('lang')).toBe('en')
  })

  it('keeps requests at least a second apart', async () => {
    const first = searchPlaces('iul', 'ro', 3)
    const second = searchPlaces('iuli', 'ro', 3)
    await vi.advanceTimersByTimeAsync(0)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(MIN_GAP_MS - 1)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    await Promise.all([first, second])
  })

  it('never sends a query that was replaced while it waited', async () => {
    const first = searchPlaces('iu', 'ro', 3)
    const stale = new AbortController()
    const second = searchPlaces('iul', 'ro', 3, stale.signal)
    const third = searchPlaces('iulius', 'ro', 3)
    stale.abort()
    await vi.advanceTimersByTimeAsync(5 * MIN_GAP_MS)
    await Promise.all([first, second, third])
    const queries = fetchMock.mock.calls.map((call) => new URL(call[0] as string).searchParams.get('q'))
    expect(queries).toEqual(['iu', 'iulius'])
    expect(await second).toEqual([])
  })

  it('answers a repeated query from its cache', async () => {
    await searchPlaces('iulius', 'ro', 3)
    await vi.advanceTimersByTimeAsync(MIN_GAP_MS)
    const again = await searchPlaces('iulius', 'ro', 3)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(again[0]!.label).toBe('Iulius Mall')
  })

  it('returns nothing when Photon fails or is unreachable, and keeps working after', async () => {
    fetchMock.mockResolvedValueOnce({ok: false, json: async () => ({})} as Response)
    expect(await searchPlaces('a1', 'ro', 3)).toEqual([])
    fetchMock.mockRejectedValueOnce(new TypeError('offline'))
    const p = searchPlaces('a2', 'ro', 3)
    await vi.advanceTimersByTimeAsync(MIN_GAP_MS)
    expect(await p).toEqual([])
    const ok = searchPlaces('a3', 'ro', 3)
    await vi.advanceTimersByTimeAsync(MIN_GAP_MS)
    expect((await ok)[0]!.label).toBe('Iulius Mall')
  })

  it('names a dropped pin through reverse lookup', async () => {
    fetchMock.mockResolvedValueOnce(respond(feature({street: 'Strada Horea', housenumber: '3'})))
    const place = await reversePlace(46.77, 23.59, 'ro')
    const url = new URL(fetchMock.mock.calls[0]![0] as string)
    expect(url.pathname).toBe('/reverse')
    expect(place!.label).toBe('Strada Horea, 3')
  })
})

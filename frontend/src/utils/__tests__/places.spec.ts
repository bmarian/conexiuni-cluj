import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {placeDetail, placeFromApi, resetPlaces, reversePlace, searchPlaces} from '@/utils/places.ts'

const apiPlace = (label: string, lat = 46.77, lon = 23.6, id = '7') => ({id, lat, lon, label})

function respond(places: unknown) {
  return {ok: true, json: async () => places} as Response
}

describe('placeFromApi', () => {
  it('keeps the label and coordinates as the app shows them', () => {
    expect(placeFromApi(apiPlace('Iulius Mall, Strada Alexandru Vaida Voevod, 53B, Gheorgheni'))).toEqual({
      id: '7',
      lat: '46.77',
      lon: '23.6',
      label: 'Iulius Mall, Strada Alexandru Vaida Voevod, 53B, Gheorgheni',
      name: 'Iulius Mall, Strada Alexandru Vaida Voevod, 53B, Gheorgheni',
    })
  })

  it('leaves where a place is to the rest of its label', () => {
    const place = placeFromApi({lat: 46.77, lon: 23.6, label: 'Iulius Mall, Strada Vaida Voevod, 53B, Micro 1', name: 'Iulius Mall'})!
    expect(place.name).toBe('Iulius Mall')
    expect(placeDetail(place)).toBe('Strada Vaida Voevod, 53B, Micro 1')
  })

  it('shows the whole label as the name when an older server sends none', () => {
    const place = placeFromApi(apiPlace('Iulius Mall, Micro 1'))!
    expect(place.name).toBe('Iulius Mall, Micro 1')
    expect(placeDetail(place)).toBe('')
  })

  it('falls back to coordinates for a label, and to them for an id', () => {
    expect(placeFromApi({lat: 46.77, lon: 23.6})).toEqual({id: '46.77:23.6', lat: '46.77', lon: '23.6', label: '46.7700, 23.6000', name: '46.7700, 23.6000'})
  })

  it('skips places without usable coordinates', () => {
    expect(placeFromApi({label: 'x'})).toBeNull()
    expect(placeFromApi({lat: 46.77, lon: '23.6'})).toBeNull()
    expect(placeFromApi({lat: Number.NaN, lon: 23.6})).toBeNull()
  })
})

describe('place requests', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    resetPlaces()
    fetchMock = vi.fn(async () => respond([apiPlace('Iulius Mall, Între Lacuri')]))
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('asks the backend for as many places as wanted', async () => {
    await searchPlaces('iulius', 3)
    const url = new URL(fetchMock.mock.calls[0]![0] as string, 'http://app')
    expect(url.pathname).toBe('/api/cluj/places')
    expect(url.searchParams.get('q')).toBe('iulius')
    expect(url.searchParams.get('limit')).toBe('3')
  })

  it('answers a repeated query from its cache', async () => {
    await searchPlaces('iulius', 3)
    const again = await searchPlaces('iulius', 3)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(again[0]!.label).toBe('Iulius Mall, Între Lacuri')
  })

  it('returns nothing when the backend fails or is unreachable, and keeps working after', async () => {
    fetchMock.mockResolvedValueOnce({ok: false, json: async () => ({})} as Response)
    expect(await searchPlaces('a1', 3)).toEqual([])
    fetchMock.mockRejectedValueOnce(new TypeError('offline'))
    expect(await searchPlaces('a2', 3)).toEqual([])
    expect((await searchPlaces('a3', 3))[0]!.label).toBe('Iulius Mall, Între Lacuri')
  })

  it('names a dropped pin through reverse lookup', async () => {
    fetchMock.mockResolvedValueOnce(respond([apiPlace('Strada Horea, 3, Centru')]))
    const place = await reversePlace(46.77, 23.59)
    const url = new URL(fetchMock.mock.calls[0]![0] as string, 'http://app')
    expect(url.pathname).toBe('/api/cluj/places')
    expect(url.searchParams.get('lat')).toBe('46.77')
    expect(url.searchParams.get('lon')).toBe('23.59')
    expect(place!.label).toBe('Strada Horea, 3, Centru')
  })
})

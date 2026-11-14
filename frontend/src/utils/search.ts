import {apiPath} from '@/utils/city.ts'
import {placeFromApi, type Place} from '@/utils/places.ts'
import {recentCache} from '@/utils/recentCache.ts'

// Same shape as backend/search/answer.go.
export type SearchBest = 'line' | 'stop' | 'place' | 'bus' | 'other' | 'city' | 'trip' | 'far'

export interface SearchStop {
  id: number
  rank: number
}

export interface SearchPlace extends Place {
  town: boolean
  far: boolean
}

export interface SearchBus {
  label: string
  route_id: number
  trip_id: string
  lat: number
  lon: number
}

export interface SearchOtherLine {
  city: string
  city_name: string
  route_id: number
  name: string
  long_name: string
  color: string
  slug?: string
}

export interface SearchEnd {
  name: string
  lat: number
  lon: number
  stop_id?: number
}

export interface SearchFar {
  name: string
  town?: boolean
  route_id?: number
  stop_id: number
  meters: number
}

export interface SearchNotice {
  kind: 'city_missing' | 'bus_off' | 'tickets'
  name?: string
  operator?: string
  url?: string
}

export interface SearchAnswer {
  best?: SearchBest
  show?: 'timetable' | 'map'
  lines: number[]
  stops: SearchStop[]
  places: SearchPlace[]
  buses: SearchBus[]
  other: SearchOtherLine[]
  cities: {slug: string; name: string}[]
  trip?: {from: SearchEnd | null; to: SearchEnd}
  far?: SearchFar
  notice?: SearchNotice
}

const CACHED = 64

const cache = recentCache<SearchAnswer>(CACHED)

function answerFromApi(raw: SearchAnswer): SearchAnswer {
  const places = (raw.places ?? []).flatMap((p) => {
    const place = placeFromApi(p)
    return place ? [{...place, town: !!p.town, far: !!p.far}] : []
  })
  return {
    ...raw,
    lines: raw.lines ?? [],
    stops: raw.stops ?? [],
    places,
    buses: raw.buses ?? [],
    other: raw.other ?? [],
    cities: raw.cities ?? [],
  }
}

export async function searchEverything(query: string, signal?: AbortSignal): Promise<SearchAnswer | null> {
  const url = `${apiPath('search')}?${new URLSearchParams({q: query})}`
  const hit = cache.get(url)
  if (hit) return hit
  try {
    const resp = await fetch(url, {signal})
    if (!resp.ok) return null
    const answer = answerFromApi((await resp.json()) as SearchAnswer)
    cache.set(url, answer)
    return answer
  } catch {
    return null
  }
}

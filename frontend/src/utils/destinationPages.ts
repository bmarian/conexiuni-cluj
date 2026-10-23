import {apiRequest} from '@/utils/api.ts'
import {fold} from '@/utils/text.ts'
import {RESULTS_PER_KIND, stopMatchRank} from '@/utils/stopSearch.ts'
import type {NetworkIndex} from '@/utils/network.ts'

// Answers of backend/destinations.
export type DestinationSummary = {slug: string, name: string, district: boolean, stops: number, lines: number}

export type Ride = {
  route_id: number
  direction: number
  board: number
  alight: number
  minutes: [number, number]
  every_min?: number
  first: string
  last: string
  day_type: string
}

export type Destination = {
  slug: string
  name: string
  district: boolean
  lat: number
  lon: number
  stop_ids: number[]
  route_ids: number[]
  from: {slug: string, name: string, rides: Ride[]}[]
}

export const fetchDestinations = () => apiRequest<DestinationSummary[]>('destinations')

export const fetchDestination = (slug: string) => apiRequest<Destination>(`destinations/${encodeURIComponent(slug)}`)

const byName = new Intl.Collator(undefined, {numeric: true})

export function namedLike(slug: string, areas: DestinationSummary[], index: NetworkIndex) {
  const q = fold(slug.replace(/-/g, ' ').trim())
  const ranked = <T>(items: T[], name: (item: T) => string) => items
    .map((item) => ({item, rank: stopMatchRank(fold(name(item)), q)}))
    .filter((m) => m.rank >= 0)
    .sort((a, b) => a.rank - b.rank || byName.compare(name(a.item), name(b.item)))
    .map((m) => m.item)
  return {
    areas: ranked(areas, (a) => a.name).slice(0, RESULTS_PER_KIND),
    stops: index.oneStopPerStation(ranked([...index.stopById.values()], (s) => s.stop_name)).slice(0, RESULTS_PER_KIND),
  }
}

export const rideMinutes =([fastest, slowest]: [number, number]) => fastest === slowest ? `${fastest} min` : `${fastest}–${slowest} min`

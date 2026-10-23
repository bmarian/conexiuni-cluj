import {apiRequest} from '@/utils/api.ts'

// The areas of a city and the direct rides there from the other areas (backend/destinations).
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

export const rideMinutes = ([fastest, slowest]: [number, number]) => fastest === slowest ? `${fastest} min` : `${fastest}–${slowest} min`

import {decodePolyline} from '@/utils/geo.ts'
import type {Journey, JourneyLeg, JourneyStop} from '@/utils/journey.ts'
import type {PlanTimeMode} from '@/stores/planner.ts'

// What /api/plan returns: finished options, timed on the server. Times are unix seconds.
export interface PlanRide {
  route_id: number
  trip_id: string
  depart: number
  arrive: number
  live?: boolean
  vehicle_id?: number
}

export interface PlanRun {
  depart: number
  arrive: number
  rides: PlanRide[]
}

export interface PlanLeg {
  kind: 'walk' | 'ride'
  from: number
  to: number
  passed?: number[]
  meters?: number
  seconds: number
  path: string
  lines?: number[]
}

export interface PlanOption {
  key: string
  transfers: number
  walk_m: number
  legs: PlanLeg[]
  runs: PlanRun[]
}

export interface PlanStop {
  name: string
  lat: number
  lon: number
}

export interface PlanLine {
  short_name: string
  long_name: string
  color: string
  type: number
}

export interface PlanResponse {
  options: PlanOption[]
  stops: Record<string, PlanStop>
  lines: Record<string, PlanLine>
  // Seconds until the timings change (a new vehicle poll, a first bus leaving); 0 when they don't.
  refresh_in: number
}

export function planQuery(from: {lat: number, lon: number}, to: {lat: number, lon: number}, mode: PlanTimeMode, time: string): string {
  const params = new URLSearchParams({
    from_lat: from.lat.toFixed(4),
    from_lng: from.lon.toFixed(4),
    to_lat: to.lat.toFixed(4),
    to_lng: to.lon.toFixed(4),
  })
  if (mode === 'last') {
    params.set('last', 'true')
  } else if (mode !== 'now' && time) {
    params.set('time', time)
    params.set('arrive_by', mode === 'arrive' ? 'true' : 'false')
  }
  return `plan?${params.toString()}`
}

export const rideLegs = (option: PlanOption) => option.legs.filter((l) => l.kind === 'ride')

// The trip each of a leg's lines runs on, from the timed runs.
export function legTrips(option: PlanOption, rideIndex: number): Map<number, string> {
  const trips = new Map<number, string>()
  for (const run of option.runs) {
    const ride = run.rides[rideIndex]
    if (ride && !trips.has(ride.route_id)) trips.set(ride.route_id, ride.trip_id)
  }
  return trips
}

export function journeyFromOption(option: PlanOption, plan: PlanResponse, destination: JourneyStop, planPath: string): Journey | null {
  const stopAt = (id: number): JourneyStop | null => {
    const s = plan.stops[String(id)]
    return s ? {id, name: s.name, lat: s.lat, lon: s.lon} : null
  }
  const legs: JourneyLeg[] = []
  let walkSeconds = 0
  for (const leg of option.legs) {
    if (leg.kind === 'walk') {
      walkSeconds += leg.seconds
      continue
    }
    const stops = [leg.from, ...(leg.passed ?? []), leg.to].map(stopAt).filter((s): s is JourneyStop => s !== null)
    if (stops.length < 2) return null
    const trips = legTrips(option, legs.length)
    const lines = leg.lines ?? []
    legs.push({
      routeIds: lines,
      tripIds: lines.map((id) => trips.get(id) ?? ''),
      lines: lines.map((id) => plan.lines[String(id)]?.short_name ?? String(id)),
      colors: lines.map((id) => plan.lines[String(id)]?.color ?? '#64748b'),
      stops,
      walkMinutes: walkSeconds / 60,
      rideMinutes: leg.seconds / 60,
    })
    walkSeconds = 0
  }
  if (!legs.length) return null
  return {
    id: crypto.randomUUID(),
    destination,
    legs,
    walkEndMinutes: walkSeconds / 60,
    walks: option.legs.filter((l) => l.kind === 'walk').map((l) => decodePolyline(l.path)),
    planPath,
    runs: option.runs.map((r) => ({arrive: r.arrive, departs: r.rides.map((ride) => ride.depart)})),
    startedAt: Date.now(),
  }
}

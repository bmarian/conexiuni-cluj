import type {StopTime} from '@/types/tranzy.ts'
import {
  canDriveLiveEta,
  distanceOnShape,
  type IndexedVehicle,
  isOnRoute,
  type ShapeIndex,
  stopShapePositions,
  vehicleEtaSeconds,
} from '@/composables/useVehicleTracking.ts'

/**
 * Minutes from `nowMinutes` until `absMinutes`, both in minutes past midnight.
 * Signed: negative means it has already happened. Differences beyond half a day are
 * read as crossing midnight rather than as a run most of a day away.
 */
export const minutesUntil = (absMinutes: number, nowMinutes: number): number => {
  const diff = absMinutes - nowMinutes
  if (diff < -720) return diff + 1440
  if (diff > 720) return diff - 1440
  return diff
}

// With nothing tracked, a run this late is still listed rather than dropped.
export const SCHEDULE_LATE_GRACE_MIN = 3

export type ScheduleQuery = {
  /** Departures from the terminus, in minutes past midnight. */
  departureMinutes: number[]
  /** Minutes this trip takes from the terminus to the stop being asked about. */
  offsetMinutes: number
  /** Now, in minutes past midnight. */
  nowMinutes: number
  /** Ignore anything further out than this. Defaults to no limit. */
  horizonMinutes?: number
}

/** Upcoming timetable arrivals at one stop, soonest first. May start slightly negative. */
export const scheduledArrivals = (query: ScheduleQuery): number[] => {
  const horizon = query.horizonMinutes ?? Number.POSITIVE_INFINITY
  return query.departureMinutes
    .map((departure) => minutesUntil(departure + query.offsetMinutes, query.nowMinutes))
    .filter((minutes) => minutes >= -SCHEDULE_LATE_GRACE_MIN && minutes < horizon)
    .sort((a, b) => a - b)
}

export type Arrival = { minutes: number; isLive: boolean }

// How far off its timetable a bus can run and still be taken for that run.
const RUN_MATCH_WINDOW_MIN = 20
// A bus already on the road cannot belong to a run that leaves later than this.
const EARLY_DEPARTURE_MIN = 2

export type TripArrivalsQuery = {
  /** Departures from the terminus, in minutes past midnight. */
  departureMinutes: number[]
  /** Stops of this one trip, any order. */
  tripStops: StopTime[]
  vehicles: IndexedVehicle[]
  index: ShapeIndex
  referenceTime: Date
  horizonMinutes?: number
  limit?: number
}

type Placed = {
  vehicle: IndexedVehicle
  atTerminus: boolean
  live: boolean
  // When this bus left (or can leave) the terminus, relative to now, judged from where it is.
  departedRel: number
}

/**
 * Arrivals at every stop of a trip, in stop_sequence order.
 *
 * Every tracked bus is first matched to the timetable run it is driving, once for the
 * whole trip, so a run is either live or scheduled and never both, and a run whose
 * bus is already past a stop is not listed there.
 */
export function arrivalsAlongTrip(query: TripArrivalsQuery): Arrival[][] {
  const stops = [...query.tripStops].sort((a, b) => a.stop_sequence - b.stop_sequence)
  if (!stops.length) return []
  const limit = query.limit ?? 3
  const horizon = query.horizonMinutes ?? Number.POSITIVE_INFINITY
  const {index, referenceTime} = query
  const nowMs = referenceTime.getTime()
  const clockMinutes = referenceTime.getHours() * 60 + referenceTime.getMinutes()
  const exactMinutes = clockMinutes + referenceTime.getSeconds() / 60

  const cumulativeSec: number[] = []
  let total = 0
  for (const st of stops) {
    total += st.offset_arrival_time
    cumulativeSec.push(total)
  }
  const offsetMinutes = cumulativeSec.map((sec) => Math.ceil(sec / 60))
  const positions = index.shape.length ? stopShapePositions(stops, index.shape) : []
  const tripMinutes = total / 60

  const scheduledProgressSec = (shapeIdx: number): number => {
    let i = 0
    while (i + 1 < positions.length && positions[i + 1]! <= shapeIdx) i++
    if (i + 1 >= positions.length) return cumulativeSec[i] ?? 0
    const span = distanceOnShape(index, positions[i]!, positions[i + 1]!)
    const done = shapeIdx > positions[i]! ? distanceOnShape(index, positions[i]!, shapeIdx) : 0
    const share = span > 0 ? Math.min(1, done / span) : 0
    return cumulativeSec[i]! + share * (cumulativeSec[i + 1]! - cumulativeSec[i]!)
  }

  const placed: Placed[] = positions.length
    ? query.vehicles.filter(isOnRoute).map((vehicle) => {
      if (vehicle.atStartTerminus) return {vehicle, atTerminus: true, live: false, departedRel: 0}
      const fixAgeMin = Math.max(0, (nowMs - new Date(vehicle.timestamp).getTime()) / 60_000)
      return {
        vehicle,
        atTerminus: false,
        live: canDriveLiveEta(vehicle, nowMs),
        departedRel: -fixAgeMin - scheduledProgressSec(vehicle.shapeIdx) / 60,
      }
    })
    : []
  // Furthest along first: buses on one line do not overtake, so this is also run order.
  placed.sort((a, b) => Number(a.atTerminus) - Number(b.atTerminus) || b.vehicle.shapeIdx - a.vehicle.shapeIdx)

  const runs = query.departureMinutes
    .map((departure) => ({
      rel: minutesUntil(departure, exactMinutes),
      clockRel: minutesUntil(departure, clockMinutes),
    }))
    .filter((run) => run.rel >= -(tripMinutes + RUN_MATCH_WINDOW_MIN) && run.clockRel < horizon)
    .sort((a, b) => a.rel - b.rel)

  const matchCost = (p: Placed, runRel: number): number => {
    if (!p.atTerminus && runRel > EARLY_DEPARTURE_MIN) return Infinity
    const cost = Math.abs(p.departedRel - runRel)
    return cost <= RUN_MATCH_WINDOW_MIN ? cost : Infinity
  }
  const matched = matchRunsInOrder(placed, runs.map((run) => run.rel), matchCost, RUN_MATCH_WINDOW_MIN)

  // Too far off any run to place, but still a real bus coming.
  const unplaced = placed.filter((p) => p.live && !matched.includes(p))
  const tracked = query.vehicles.length > 0

  return stops.map((stop, k) => {
    const stopShapeIdx = positions[k] ?? -1
    const hasPassed = (p: Placed) => !p.atTerminus && p.vehicle.shapeIdx > stopShapeIdx
    const liveMinutes = (p: Placed) => Math.max(0, Math.round(vehicleEtaSeconds(p.vehicle, stopShapeIdx, index, {
      tripStops: stops,
      targetStopId: stop.stop_id,
      referenceTime,
    }) / 60))
    // A run the timetable says left before a bus that is already past this stop is past it too.
    let lastRunPastStop = -1
    matched.forEach((p, j) => {
      if (p && hasPassed(p)) lastRunPastStop = j
    })
    // Ahead of the leading tracked bus: with tracking up, that is a bus that has finished.
    const firstRunOnRoad = matched.findIndex((p) => p !== null && !p.atTerminus)

    const arrivals: Arrival[] = []
    const add = (minutes: number, isLive: boolean) => {
      if (minutes < horizon) arrivals.push({minutes, isLive})
    }
    runs.forEach((run, j) => {
      const p = matched[j]
      if (p && hasPassed(p)) return
      if (p?.live) {
        add(liveMinutes(p), true)
      } else if (p?.atTerminus) {
        add(Math.max(run.clockRel, 0) + offsetMinutes[k]!, false)
      } else if (p) {
        // Last fix too old to drive an estimate: hold the bus to the pace it had then.
        const minutes = Math.round(p.departedRel + cumulativeSec[k]! / 60)
        if (minutes >= -SCHEDULE_LATE_GRACE_MIN) add(Math.max(0, minutes), false)
      } else {
        if (j < lastRunPastStop || j < firstRunOnRoad) return
        const minutes = run.clockRel + offsetMinutes[k]!
        if (minutes >= (tracked ? 0 : -SCHEDULE_LATE_GRACE_MIN)) add(Math.max(0, minutes), false)
      }
    })
    for (const p of unplaced) {
      if (!hasPassed(p)) add(liveMinutes(p), true)
    }

    return arrivals
      .sort((a, b) => a.minutes - b.minutes || Number(b.isLive) - Number(a.isLive))
      .slice(0, limit)
  })
}

/**
 * Order-preserving assignment of vehicles (furthest along first) to runs (earliest
 * first) at the lowest total cost. A vehicle may stay unmatched at `unmatchedCost`.
 */
function matchRunsInOrder<T>(
  vehicles: T[],
  runs: number[],
  cost: (vehicle: T, run: number) => number,
  unmatchedCost: number,
): (T | null)[] {
  const V = vehicles.length
  const R = runs.length
  const width = R + 1
  const best = new Float64Array((V + 1) * width)
  const step = new Uint8Array((V + 1) * width)
  const SKIP_RUN = 1, SKIP_VEHICLE = 2, MATCH = 3

  for (let i = 1; i <= V; i++) {
    best[i * width] = best[(i - 1) * width]! + unmatchedCost
    step[i * width] = SKIP_VEHICLE
    for (let j = 1; j <= R; j++) {
      let value = best[i * width + j - 1]!
      let choice = SKIP_RUN
      const skipVehicle = best[(i - 1) * width + j]! + unmatchedCost
      if (skipVehicle < value) {
        value = skipVehicle
        choice = SKIP_VEHICLE
      }
      const match = best[(i - 1) * width + j - 1]! + cost(vehicles[i - 1]!, runs[j - 1]!)
      if (match <= value) {
        value = match
        choice = MATCH
      }
      best[i * width + j] = value
      step[i * width + j] = choice
    }
  }

  const byRun: (T | null)[] = Array.from({length: R}, () => null)
  let i = V
  let j = R
  while (i > 0) {
    const choice = j === 0 ? SKIP_VEHICLE : step[i * width + j]
    if (choice === MATCH) {
      byRun[j - 1] = vehicles[i - 1]!
      i--
      j--
    } else if (choice === SKIP_VEHICLE) {
      i--
    } else {
      j--
    }
  }
  return byRun
}

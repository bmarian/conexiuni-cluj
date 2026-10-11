import type {Shape, StopInfo, Vehicle} from '@/types/tranzy.ts'
import {OUTGOING_SUFFIX} from '@/types/tranzy.ts'
import {getMinutesFromDate, getTimetableForDay, isTimetableAvailableOnDay, timeStringToMinutes} from '@/utils/time.ts'
import {getRouteIdFromTripId, getShapeStopTimes, getTimeOffsetToStop} from '@/utils/trips.ts'
import {mergeArrivals, scheduledArrivals} from '@/utils/arrivals.ts'
import {buildShapeIndex, buildStopShapeIdxByStopId, etaForStop, getIndexedVehicles} from '@/composables/useVehicleTracking.ts'

const HORIZON_MIN = 480

export interface TripDeparture {
  tripId: string
  routeId: number
  line: string
  color: string
  longName: string
  minutes: number
  live: boolean
  later: number[]
  /** Negative when the bus is late. */
  scheduled: number[]
  liveMinutes: number | null
}

export async function tripDepartures(
  stop: StopInfo,
  tripIds: string[],
  shapes: Map<string, Shape[]>,
  vehicles: Map<string, Vehicle[]>,
  now: Date,
): Promise<TripDeparture[]> {
  const out: TripDeparture[] = []
  for (const tripId of tripIds) {
    const routeId = getRouteIdFromTripId(tripId)
    const shapeInfo = stop.shapes_info.find((s) => s.route_id === routeId)
    if (routeId === null || !shapeInfo || !isTimetableAvailableOnDay(now, shapeInfo.timetable)) continue
    const outgoing = tripId.endsWith(OUTGOING_SUFFIX)
    const stopTimes = getShapeStopTimes(shapeInfo)
    const departures = getTimetableForDay(shapeInfo.timetable, now).entries
      .map((entry) => timeStringToMinutes(outgoing ? entry.departure_in : entry.departure_out))
      .filter((m): m is number => m !== null)
    const scheduled = scheduledArrivals({
      departureMinutes: departures,
      offsetMinutes: getTimeOffsetToStop(stopTimes, tripId, stop.stop_id),
      nowMinutes: getMinutesFromDate(now),
      horizonMinutes: HORIZON_MIN,
    })

    let liveMinutes: number | null = null
    let tracked = false
    const points = shapes.get(tripId) ?? []
    if (points.length) {
      const index = buildShapeIndex(points)
      const indexed = await getIndexedVehicles(tripId, shapeInfo.route_short_name, shapeInfo.route_color, index, now, vehicles.get(tripId) ?? [])
      tracked = indexed.length > 0
      const tripStops = stopTimes.filter((st) => st.trip_id === tripId)
      const stopIdx = buildStopShapeIdxByStopId(tripStops, points).get(stop.stop_id) ?? -1
      const eta = etaForStop(stopIdx, indexed, index, {tripStops, targetStopId: stop.stop_id, referenceTime: now})
      liveMinutes = eta ? eta.etaMinutes : null
    }
    const merged = mergeArrivals(liveMinutes, scheduled, {tracked, limit: 3})
    if (!merged.length) continue
    out.push({
      tripId,
      routeId,
      line: shapeInfo.route_short_name,
      color: shapeInfo.route_color,
      longName: outgoing ? shapeInfo.timetable.route_long_name : shapeInfo.timetable.route_long_name.split(' - ').reverse().join(' - '),
      minutes: merged[0]!.minutes,
      live: merged[0]!.isLive,
      later: merged.slice(1).map((m) => m.minutes),
      scheduled,
      liveMinutes,
    })
  }
  return out.sort((a, b) => a.minutes - b.minutes)
}

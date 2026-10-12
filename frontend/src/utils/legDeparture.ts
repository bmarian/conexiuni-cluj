import type {LiveRow, StopInfo} from '@/types/tranzy.ts'
import {OUTGOING_SUFFIX} from '@/types/tranzy.ts'
import {getMinutesFromDate, getTimetableForDay, isTimetableAvailableOnDay, timeStringToMinutes} from '@/utils/time.ts'
import {getRouteIdFromTripId, getShapeStopTimes, getTimeOffsetToStop} from '@/utils/trips.ts'
import {nextArrivals, scheduledArrivals} from '@/utils/arrivals.ts'
import {entriesAt, liveRowKey} from '@/composables/useLive.ts'

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

// The next departures of some trips from a stop: the server's prediction rows when the
// live feed has them, the timetable otherwise.
export function tripDepartures(
  stop: StopInfo,
  tripIds: string[],
  rows: Map<string, LiveRow>,
  asOfMs: number | null,
  now: Date,
): TripDeparture[] {
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
      offsetSeconds: getTimeOffsetToStop(stopTimes, tripId, stop.stop_id),
      nowMinutes: getMinutesFromDate(now),
      horizonMinutes: HORIZON_MIN,
      graceMinutes: stop.late_grace_min ?? 1,
    })

    const row = asOfMs === null ? undefined : rows.get(liveRowKey(tripId, stop.stop_id))
    const entries = row ? entriesAt(row, now.getTime()) : null
    const merged = entries
      ? entries.map((e) => ({minutes: e.minutes, isLive: e.live}))
      : nextArrivals(scheduled, 3)
    if (!merged.length) continue
    const firstLive = entries?.find((e) => e.live)
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
      liveMinutes: firstLive ? firstLive.minutes : null,
    })
  }
  return out.sort((a, b) => a.minutes - b.minutes)
}

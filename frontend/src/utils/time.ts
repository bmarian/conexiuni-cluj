import type {DaySchedule, Timetable} from '@/types/ctp.ts'
import {
  OUTGOING_SUFFIX,
  type StopInfo,
  type TimeEntry,
  type VehiclesInStop
} from '@/types/tranzy.ts'
import {getRouteIdFromTripId, getShapeStopTimes, getTimeOffsetToStop, getTripIdForRouteAtStop} from "@/utils/trips.ts";

export const timeStringToMinutes = (timeString: string): number | null => {
  if (!timeString || !timeString.includes(':')) {
    return null;
  }

  const parts = timeString.trim().split(':');

  const hours = parseInt(parts[0]!, 10);
  const minutes = parseInt(parts[1]!, 10);

  if (isNaN(hours) || isNaN(minutes)) {
    return null;
  }

  return (hours * 60) + minutes;
}

export const getMinutesFromDate = (dateObject: Date): number => {
  const hours = dateObject.getHours();
  const minutes = dateObject.getMinutes();

  return (hours * 60) + minutes;
}

export const formatMinutesFromNow = (minutes: number, referenceDate: Date, nowLabel: string): string => {
  if (minutes === 0) return nowLabel;
  if (minutes < 60) return `${minutes}m`;
  const future = new Date(referenceDate.getTime() + minutes * 60_000);
  return `${future.getHours().toString().padStart(2, '0')}:${future.getMinutes().toString().padStart(2, '0')}`;
}

export type TimetableDayKey = 'weekdays' | 'saturday' | 'sunday'

export const getTimetableDayKey = (referenceDate: Date): TimetableDayKey => {
  const day = referenceDate.getDay()
  if (day === 0) return 'sunday'
  if (day === 6) return 'saturday'
  return 'weekdays'
}

export const getTimetableForDay = (timetable: Timetable, referenceDate: Date): DaySchedule => {
  return timetable[getTimetableDayKey(referenceDate)]
}

export const hasTimetableEntries = (timetable?: Timetable | null): boolean => {
  return !!(
    timetable?.weekdays?.entries?.length
    || timetable?.saturday?.entries?.length
    || timetable?.sunday?.entries?.length
  )
}

export const isTimetableAvailableOnDay = (referenceDate: Date, timetable: Timetable): boolean => {
  return !!getTimetableForDay(timetable, referenceDate)?.entries?.length
}

export const reverseRouteLongName = (routeLongName: string): string => {
  return routeLongName.split(' - ').reverse().join(' - ')
}

export const getAvailableBusesForStop = (
  stopInfo: StopInfo,
  referenceDate: Date,
  options: { maxMinutes?: number; limit?: number; tripId?: string } = {}
): VehiclesInStop[] => {
  const referenceMinutes = getMinutesFromDate(referenceDate)
  const {outgoing_trip_ids, incoming_trip_ids, shapes_info, stop_id} = stopInfo

  const results: VehiclesInStop[] = []
  for (const shape of shapes_info) {
    const {route_short_name, route_type, route_color, route_id, timetable} = shape
    if (!isTimetableAvailableOnDay(referenceDate, timetable)) continue

    // If a specific tripId is provided, we only care about shapes that match its route
    if (options.tripId && getRouteIdFromTripId(options.tripId) !== route_id) continue

    const tripId = options.tripId || getTripIdForRouteAtStop(outgoing_trip_ids ?? [], incoming_trip_ids ?? [], route_id)
    if (!tripId) continue

    const stopTimes = getShapeStopTimes(shape)
    const timeOffsetToStop = getTimeOffsetToStop(stopTimes, tripId, stop_id)
    const isOutgoing = tripId.endsWith(OUTGOING_SUFFIX)
    const daySchedule = getTimetableForDay(timetable, referenceDate)

    let upcomingEntries: TimeEntry[] = daySchedule.entries
      .map(entry => {
        const depStr = isOutgoing ? entry.departure_in : entry.departure_out
        const depMinutes = timeStringToMinutes(depStr)
        if (depMinutes === null) return null

        const arrivalAtStopMinutes = depMinutes + timeOffsetToStop
        const minutesDiff = ((arrivalAtStopMinutes - referenceMinutes) + 1440) % 1440

        if (options.maxMinutes !== undefined && minutesDiff >= options.maxMinutes) return null

        return {
          minutes: minutesDiff,
          is_live: false
        }
      })
      .filter((e): e is TimeEntry => e !== null)
      .sort((a, b) => a.minutes - b.minutes)

    if (options.limit !== undefined) {
      upcomingEntries = upcomingEntries.slice(0, options.limit)
    }

    if (upcomingEntries.length === 0) continue

    results.push({
      minutes_left: upcomingEntries[0]!.minutes,
      next_times: upcomingEntries,
      route_short_name,
      route_type,
      route_color,
      trip_id: tripId,
      route_id,
      route_long_name: isOutgoing ? timetable.route_long_name : reverseRouteLongName(timetable.route_long_name),
      static_time_approximation: true,
    })
  }

  return results.sort((a, b) => a.minutes_left - b.minutes_left)
}

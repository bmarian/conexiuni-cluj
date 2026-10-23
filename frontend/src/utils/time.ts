import {cityClock, cityDate, cityMinutes, cityTime} from '@/utils/cityClock.ts'
import {shallowRef} from 'vue'
import type {DaySchedule, Timetable} from '@/types/ctp.ts'
import {
  OUTGOING_SUFFIX,
  type StopInfo,
  type TimeEntry,
  type VehiclesInStop
} from '@/types/tranzy.ts'
import {getRouteIdFromTripId, getShapeStopTimes, getTimeOffsetToStop, getTripIdForRouteAtStop} from "@/utils/trips.ts";
import {scheduledArrivals} from "@/utils/arrivals.ts";

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

export const getMinutesFromDate = (dateObject: Date): number => cityMinutes(dateObject)

export const formatMinutesFromNow = (rawMinutes: number, referenceDate: Date, nowLabel: string): string => {
  const minutes = Math.round(rawMinutes);
  if (minutes === 0) return nowLabel;
  if (minutes > 0 && minutes < 60) return `${minutes}m`;
  return cityClock(referenceDate.getTime() + minutes * 60_000);
}

export type TimetableDayKey = 'weekdays' | 'saturday' | 'sunday'

export type ScheduleDay = 'weekday' | 'saturday' | 'sunday'

export type DayNote = {reason: 'holiday' | 'set' | 'observed'; holiday?: string}

export type SpecialDay = {schedule: ScheduleDay; note: DayNote | null}

const scheduleDays = shallowRef<Record<string, ScheduleDay>>({})
const dayNotes = shallowRef<Record<string, DayNote>>({})

export const setScheduleDays = (days: Record<string, ScheduleDay> | undefined, notes?: Record<string, DayNote>) => {
  scheduleDays.value = days ?? {}
  dayNotes.value = notes ?? {}
}

export const scheduleDayOn = (date: string): ScheduleDay | undefined => scheduleDays.value[date]

export const plainTimetableDayKey = (referenceDate: Date): TimetableDayKey => {
  const day = cityTime(referenceDate).weekday
  if (day === 0) return 'sunday'
  if (day === 6) return 'saturday'
  return 'weekdays'
}

export const getTimetableDayKey = (referenceDate: Date): TimetableDayKey => {
  const special = scheduleDays.value[cityDate(referenceDate)]
  if (special) return special === 'weekday' ? 'weekdays' : special
  return plainTimetableDayKey(referenceDate)
}

export const specialDayOn = (referenceDate: Date): SpecialDay | null => {
  const date = cityDate(referenceDate)
  const schedule = scheduleDays.value[date]
  return schedule ? {schedule, note: dayNotes.value[date] ?? null} : null
}

export const getTimetableForDay = (timetable: Timetable, referenceDate: Date): DaySchedule => {
  return timetable[getTimetableDayKey(referenceDate)]
}

// Today's departures of one trip from its first stop, in minutes past midnight.
export const tripDepartureMinutes = (timetable: Timetable | null | undefined, tripId: string, referenceDate: Date): number[] => {
  if (!timetable) return []
  const outgoing = tripId.endsWith(OUTGOING_SUFFIX)
  return (getTimetableForDay(timetable, referenceDate)?.entries ?? [])
    .map((entry) => timeStringToMinutes(outgoing ? entry.departure_in : entry.departure_out))
    .filter((m): m is number => m !== null)
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

export const directedRouteName = (routeLongName: string, direction: string): string =>
  (direction === '1' ? reverseRouteLongName(routeLongName) : routeLongName).replaceAll(' - ', ' → ')

export function routeDestination(name: string): string {
  const i = name.lastIndexOf(' - ')
  return i >= 0 ? name.slice(i + 3) : name
}

export function routeOrigin(name: string): string {
  const i = name.lastIndexOf(' - ')
  return i >= 0 ? name.slice(0, i) : ''
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

    if (options.tripId && getRouteIdFromTripId(options.tripId) !== route_id) continue

    const tripId = options.tripId || getTripIdForRouteAtStop(outgoing_trip_ids ?? [], incoming_trip_ids ?? [], route_id)
    if (!tripId) continue

    const stopTimes = getShapeStopTimes(shape)
    const timeOffsetToStop = getTimeOffsetToStop(stopTimes, tripId, stop_id)
    const isOutgoing = tripId.endsWith(OUTGOING_SUFFIX)
    const daySchedule = getTimetableForDay(timetable, referenceDate)

    const departureMinutes = daySchedule.entries
      .map(entry => timeStringToMinutes(isOutgoing ? entry.departure_in : entry.departure_out))
      .filter((m): m is number => m !== null)

    // Signed, so a run a few minutes late is still listed instead of wrapping to tomorrow.
    let upcomingEntries: TimeEntry[] = scheduledArrivals({
      departureMinutes,
      offsetSeconds: timeOffsetToStop,
      nowMinutes: referenceMinutes,
      horizonMinutes: options.maxMinutes,
      graceMinutes: shape.late_grace_min ?? 1,
    }).map(minutes => ({minutes: Math.max(Math.round(minutes), 0), is_live: false}))

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

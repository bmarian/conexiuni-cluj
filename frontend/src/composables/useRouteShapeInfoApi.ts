import {OUTGOING_SUFFIX, type Route, type ShapeInfo, type StopTime} from '@/types/tranzy.ts'
import type {DaySchedule, Timetable} from '@/types/ctp.ts'
import {apiRequest, LOW_ACCURACY_SHELF_LIFE} from '@/utils/request_cache.ts'

const pending = new Map<number, Promise<ShapeInfo>>()

const emptyDay = (): DaySchedule => ({service_name: '', service_start: '', entries: []})

const emptyTimetable = (route: Route): Timetable => ({
  route_short_name: route.route_short_name,
  route_long_name: route.route_long_name,
  in_stop_name: '',
  out_stop_name: '',
  weekdays: emptyDay(),
  saturday: emptyDay(),
  sunday: emptyDay(),
})

/** Best-effort stop-name lookup from stop_times when CTP metadata is missing. */
function deriveTerminalName(stopTimes: StopTime[], position: 'first' | 'last'): string {
  const outbound = stopTimes
    .filter((st) => st.trip_id.endsWith(OUTGOING_SUFFIX))
    .sort((a, b) => a.stop_sequence - b.stop_sequence)
  if (!outbound.length) return ''
  const target = position === 'first' ? outbound[0] : outbound[outbound.length - 1]
  return target?.stop_headsign?.trim() ?? ''
}

export function useRouteShapeInfoApi() {
  async function fetchShapeInfo(route: Route): Promise<ShapeInfo> {
    if (pending.has(route.route_id)) {
      return pending.get(route.route_id)!
    }

    const promise = (async () => {
      const encoded = encodeURIComponent(route.route_short_name)
      // Tolerate either fetch failing — backend returns 500 for routes without
      // a ctpcj.ro CSV (e.g. M24L, M32, 52B). RouteView degrades gracefully
      // when timetable entries / stop_times are empty.
      const [timetableResult, stopTimesResult] = await Promise.allSettled([
        apiRequest(`timetable?route_short_name=${encoded}`, LOW_ACCURACY_SHELF_LIFE) as Promise<Timetable>,
        apiRequest(`stop_times?route_short_name=${encoded}`, LOW_ACCURACY_SHELF_LIFE) as Promise<StopTime[]>,
      ])

      const timetable: Timetable =
        timetableResult.status === 'fulfilled' && timetableResult.value
          ? timetableResult.value
          : emptyTimetable(route)

      const stopTimes: StopTime[] =
        stopTimesResult.status === 'fulfilled' && Array.isArray(stopTimesResult.value)
          ? stopTimesResult.value
          : []

      if (timetableResult.status === 'rejected') {
        console.warn(`No timetable for route ${route.route_short_name}:`, timetableResult.reason)
      }
      if (stopTimesResult.status === 'rejected') {
        console.warn(`No stop_times for route ${route.route_short_name}:`, stopTimesResult.reason)
      }

      // Backend returns the timetable shell even when CTP has no CSV — its meta
      // strings come back empty. Backfill from the canonical Route + stop_times
      // so RouteView's header / direction toggle / first+last stop labels still
      // render something meaningful.
      if (!timetable.route_short_name) timetable.route_short_name = route.route_short_name
      if (!timetable.route_long_name) timetable.route_long_name = route.route_long_name
      if (!timetable.in_stop_name) timetable.in_stop_name = deriveTerminalName(stopTimes, 'first')
      if (!timetable.out_stop_name) timetable.out_stop_name = deriveTerminalName(stopTimes, 'last')

      return {
        route_short_name: route.route_short_name,
        route_id: route.route_id,
        route_type: route.route_type,
        route_color: route.route_color,
        stop_times: stopTimes,
        timetable,
      } satisfies ShapeInfo
    })()

    pending.set(route.route_id, promise)
    try {
      return await promise
    } finally {
      pending.delete(route.route_id)
    }
  }

  return {fetchShapeInfo}
}

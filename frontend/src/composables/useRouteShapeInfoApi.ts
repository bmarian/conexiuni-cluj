import type {Route, ShapeInfo, StopTime} from '@/types/tranzy.ts'
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

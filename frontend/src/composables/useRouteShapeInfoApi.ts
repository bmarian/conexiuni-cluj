import {type HourlyStopOffsets, OUTGOING_SUFFIX, type Route, type ShapeInfo, type StopTime} from '@/types/tranzy.ts'
import type {DaySchedule, Timetable} from '@/types/ctp.ts'
import {buildHourlyOffsets, dayTypeAndHour, network, networkLoaded, refreshNetwork, routeStopTimes} from '@/utils/network.ts'
import {currentCity} from '@/utils/city.ts'
import {watch} from 'vue'

const pending = new Map<number, Promise<ShapeInfo>>()
watch(currentCity, () => pending.clear(), {flush: 'sync'})

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

function deriveTerminalName(stopTimes: StopTime[], position: 'first' | 'last'): string {
  const outbound = stopTimes
    .filter((st) => st.trip_id.endsWith(OUTGOING_SUFFIX))
    .sort((a, b) => a.stop_sequence - b.stop_sequence)
  if (!outbound.length) return ''
  const target = position === 'first' ? outbound[0] : outbound[outbound.length - 1]
  return target?.stop_headsign?.trim() ?? ''
}

async function loadedNetwork() {
  await networkLoaded()
  if (!network.value) await refreshNetwork(true)
  return network.value
}

export async function fetchHourlyStopOffsets(
  routeShortName: string,
  dayType: 'weekday' | 'saturday' | 'sunday',
): Promise<Record<string, HourlyStopOffsets>> {
  const index = await loadedNetwork()
  const local = index?.routeByShortName.get(routeShortName)
  return index && local ? buildHourlyOffsets(index, local, dayType) : {}
}

export function useRouteShapeInfoApi() {
  async function fetchShapeInfo(route: Route): Promise<ShapeInfo> {
    if (pending.has(route.route_id)) {
      return pending.get(route.route_id)!
    }

    const promise = (async () => {
      const index = await loadedNetwork()
      const local = index?.routeById.get(route.route_id)
      const {dayType, hour} = dayTypeAndHour()
      const timetable: Timetable = local?.timetable ? structuredClone(local.timetable) : emptyTimetable(route)
      const stopTimes: StopTime[] = index && local ? routeStopTimes(index, local, dayType, hour) : []
      if (!local?.timetable) console.warn(`No timetable for route ${route.route_short_name}`)

      if (!timetable.route_short_name) timetable.route_short_name = route.route_short_name
      if (!timetable.route_long_name) timetable.route_long_name = route.route_long_name
      if (!timetable.in_stop_name) timetable.in_stop_name = deriveTerminalName(stopTimes, 'first')
      if (!timetable.out_stop_name) timetable.out_stop_name = deriveTerminalName(stopTimes, 'last')

      return {
        route_short_name: route.route_short_name,
        route_long_name: route.route_long_name,
        route_id: route.route_id,
        route_type: route.route_type,
        route_color: route.route_color,
        stop_times: stopTimes,
        timetable,
        late_grace_min: index?.lateGraceMin ?? 1,
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

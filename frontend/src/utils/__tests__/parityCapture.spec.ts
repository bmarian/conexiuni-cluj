// Temporary: writes the Android parity expectations from the website's own logic.
// PARITY_FIXTURE=<parity.json(.gz)> PARITY_OUT=<expected.json> TZ=Europe/Bucharest npx vitest run parityCapture
import {it} from 'vitest'
import fs from 'node:fs'
import zlib from 'node:zlib'
import type {Route, Shape, StopInfo, StopTime, Vehicle, VehiclesInStop} from '@/types/tranzy.ts'
import type {Timetable} from '@/types/ctp.ts'
import {getAvailableBusesForStop, getMinutesFromDate, getTimetableForDay, timeStringToMinutes, tripDepartureMinutes} from '@/utils/time.ts'
import {markRepeatedNow, mergeArrivals, scheduledArrivals} from '@/utils/arrivals.ts'
import {getShapeStopTimes} from '@/utils/trips.ts'
import {
  buildShapeIndex,
  buildStopShapeIdxByStopId,
  type FeederTrip,
  getIndexedVehicles,
  type IndexedVehicle,
  liveEtasForStop,
  otherDirectionTripId,
  type ShapeIndex,
  type ThroughRunOptions,
  turnaroundOf,
} from '@/composables/useVehicleTracking.ts'
import {OUTGOING_SUFFIX, INCOMING_SUFFIX} from '@/types/tranzy.ts'

type Fixture = {
  nowMs: number
  stops: Record<string, StopInfo>
  routes: Record<string, {route: Route; timetable?: Timetable | null; stop_times?: StopTime[]}>
  shapes: Record<string, Shape[]>
  vehicles: Vehicle[]
}

const readJson = (path: string) => {
  const raw = fs.readFileSync(path)
  return JSON.parse((path.endsWith('.gz') ? zlib.gunzipSync(raw) : raw).toString('utf8'))
}

const deriveTerminalName = (stopTimes: StopTime[], position: 'first' | 'last'): string => {
  const outbound = stopTimes.filter((st) => st.trip_id.endsWith(OUTGOING_SUFFIX)).sort((a, b) => a.stop_sequence - b.stop_sequence)
  if (!outbound.length) return ''
  const target = position === 'first' ? outbound[0] : outbound[outbound.length - 1]
  return target?.stop_headsign?.trim() ?? ''
}

// Mirror of useRouteShapeInfoApi.fetchShapeInfo's assembly.
const assembleShapeInfo = (route: Route, timetable: Timetable | null | undefined, stopTimes: StopTime[]) => {
  const tt: Timetable = timetable ? structuredClone(timetable) : {
    route_short_name: route.route_short_name, route_long_name: route.route_long_name, in_stop_name: '', out_stop_name: '',
    weekdays: {service_name: '', service_start: '', entries: []}, saturday: {service_name: '', service_start: '', entries: []}, sunday: {service_name: '', service_start: '', entries: []},
  }
  if (!tt.route_short_name) tt.route_short_name = route.route_short_name
  if (!tt.route_long_name) tt.route_long_name = route.route_long_name
  if (!tt.in_stop_name) tt.in_stop_name = deriveTerminalName(stopTimes, 'first')
  if (!tt.out_stop_name) tt.out_stop_name = deriveTerminalName(stopTimes, 'last')
  return {
    route_short_name: route.route_short_name, route_long_name: route.route_long_name, route_id: route.route_id,
    route_type: route.route_type, route_color: route.route_color, stop_times: stopTimes, timetable: tt,
  }
}

// Mirror of useStopDepartures' departure list.
async function computeStopDepartures(info: StopInfo, shapes: Record<string, Shape[]>, byTrip: Map<string, Vehicle[]>, now: Date) {
  const coming = getAvailableBusesForStop(info, now, {maxMinutes: 480, limit: 4})
  const results: VehiclesInStop[] = []
  const push = (shape: VehiclesInStop, live: number[], tracked: boolean) => {
    const scheduled = (shape.next_times ?? []).map((e) => e.minutes)
    const merged = mergeArrivals(live, scheduled, {tracked, limit: 3})
    if (!merged.length) return
    results.push({...shape, minutes_left: merged[0]!.minutes, next_times: merged.map((a) => ({minutes: a.minutes, is_live: a.isLive})), static_time_approximation: !live.length})
  }
  const byRoute = new Map(info.shapes_info.map((s) => [s.route_id, s]))
  for (const shape of coming) {
    const trip = shapes[shape.trip_id] ?? []
    if (!trip.length) { push(shape, [], false); continue }
    const index = buildShapeIndex(trip)
    const vehicles = await getIndexedVehicles(shape.trip_id, shape.route_short_name, shape.route_color, index, now, byTrip.get(shape.trip_id) ?? [])
    const tracked = vehicles.length > 0
    const routeShapeInfo = byRoute.get(shape.route_id)
    if (!routeShapeInfo) { push(shape, [], tracked); continue }
    const tripStops = getShapeStopTimes(routeShapeInfo).filter((st) => st.trip_id === shape.trip_id)
    const stopShapeIdx = buildStopShapeIdxByStopId(tripStops, trip).get(info.stop_id) ?? -1
    if (stopShapeIdx < 0) { push(shape, [], tracked); continue }
    let throughRun: ThroughRunOptions | undefined
    const turnaround = turnaroundOf(tripStops)
    if (turnaround?.through) {
      const feederTripId = otherDirectionTripId(shape.trip_id)
      const feederShape = shapes[feederTripId]
      throughRun = {
        turnaround,
        departureMinutes: tripDepartureMinutes(routeShapeInfo.timetable, shape.trip_id, now),
        feederDepartureMinutes: tripDepartureMinutes(routeShapeInfo.timetable, feederTripId, now),
        nowMinutes: getMinutesFromDate(now),
      }
      if (feederShape?.length) {
        const feederIndex = buildShapeIndex(feederShape)
        throughRun.feeder = {
          vehicles: await getIndexedVehicles(feederTripId, shape.route_short_name, shape.route_color, feederIndex, now, byTrip.get(feederTripId) ?? []),
          index: feederIndex,
          tripStops: getShapeStopTimes(routeShapeInfo).filter((st) => st.trip_id === feederTripId),
        }
      }
    }
    const etas = liveEtasForStop(stopShapeIdx, vehicles, index, {tripStops, targetStopId: info.stop_id, referenceTime: now, throughRun})
    push(shape, etas.map((e) => e.etaMinutes), tracked)
  }
  return results.sort((a, b) => a.minutes_left - b.minutes_left)
}

type DirectionShape = {shape: Shape[]; shapeIndex: ShapeIndex; stopShapeIdxByStopId: Map<number, number>}

// Mirror of useRouteArrivals' stop list.
function routeStopArrivals(
  info: ReturnType<typeof assembleShapeInfo>, stops: (StopTime & {timeOffsetFromStart: number})[], outgoing: boolean,
  dirShape: DirectionShape | null, vehicles: IndexedVehicle[], now: Date, feeder: FeederTrip | undefined, feederTripId: string,
) {
  const sched = getTimetableForDay(info.timetable, now)
  const departures = (sched?.entries ?? []).map((e) => timeStringToMinutes(outgoing ? e.departure_in : e.departure_out)).filter((v): v is number => v !== null)
  const nowMinutes = getMinutesFromDate(now)
  const tracked = vehicles.length > 0
  const turnaround = turnaroundOf(stops)
  const throughRun: ThroughRunOptions | undefined = turnaround?.through ? {
    turnaround, departureMinutes: departures, feederDepartureMinutes: tripDepartureMinutes(info.timetable, feederTripId, now), nowMinutes, feeder,
  } : undefined
  const rows = stops.map((stop) => {
    const stopIdx = dirShape?.stopShapeIdxByStopId.get(stop.stop_id)
    const live = !dirShape || stopIdx === undefined || stopIdx < 0 ? [] :
      liveEtasForStop(stopIdx, vehicles, dirShape.shapeIndex, {tripStops: stops, targetStopId: stop.stop_id, referenceTime: now, throughRun}).map((e) => e.etaMinutes)
    return mergeArrivals(live, scheduledArrivals({departureMinutes: departures, offsetMinutes: stop.timeOffsetFromStart, nowMinutes, horizonMinutes: 480}), {tracked})
  })
  return markRepeatedNow(rows)
}

it('captures parity expectations', async () => {
  const fixturePath = process.env.PARITY_FIXTURE
  const out = process.env.PARITY_OUT
  if (!fixturePath || !out) return
  const fixture = readJson(fixturePath) as Fixture
  const scenarios: {label: string; nowMs: number; live: boolean}[] = [
    ...[0, 1, 3].map((m) => ({label: `live+${m}`, nowMs: fixture.nowMs + m * 60_000, live: true})),
    ...Array.from({length: 24}, (_, h) => ({label: `timetable+${h}h`, nowMs: fixture.nowMs + 17 * 60_000 + h * 3_600_000, live: false})),
    ...Array.from({length: 6}, (_, d) => ({label: `timetable+${d + 1}d`, nowMs: fixture.nowMs + (d + 1) * 86_400_000, live: false})),
  ]
  const expected = []
  for (const scenario of scenarios) {
    const now = new Date(scenario.nowMs)
    const byTrip = new Map<string, Vehicle[]>()
    if (scenario.live) for (const v of fixture.vehicles) byTrip.set(v.trip_id, [...(byTrip.get(v.trip_id) ?? []), v])
    const stops: Record<string, unknown> = {}
    for (const [stopId, info] of Object.entries(fixture.stops)) {
      stops[stopId] = (await computeStopDepartures(info, fixture.shapes, byTrip, now)).map((d) => ({
        route: d.route_short_name, trip: d.trip_id, name: d.route_long_name, left: d.minutes_left, static: d.static_time_approximation,
        times: d.next_times.map((t) => [t.minutes, t.is_live]),
      }))
    }
    const routes: Record<string, unknown> = {}
    for (const [routeId, r] of Object.entries(fixture.routes)) {
      const info = assembleShapeInfo(r.route, r.timetable, r.stop_times ?? [])
      const dirs: Record<string, unknown> = {in: info.timetable.in_stop_name, out: info.timetable.out_stop_name}
      const prepared = new Map<string, {stops: (StopTime & {timeOffsetFromStart: number})[]; shape: DirectionShape | null; indexed: IndexedVehicle[]}>()
      for (const dir of ['0', '1']) {
        const tripId = `${info.route_id}${dir === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`
        let cumulative = 0
        const stops = info.stop_times.filter((st) => st.trip_id === tripId).sort((a, b) => a.stop_sequence - b.stop_sequence).map((st) => {
          cumulative += st.offset_arrival_time
          return {...st, timeOffsetFromStart: Math.ceil(cumulative / 60)}
        })
        const points = fixture.shapes[tripId] ?? []
        const shape: DirectionShape | null = points.length ? {shape: points, shapeIndex: buildShapeIndex(points), stopShapeIdxByStopId: buildStopShapeIdxByStopId(info.stop_times.filter((st) => st.trip_id === tripId), points)} : null
        const indexed = shape ? await getIndexedVehicles(tripId, info.route_short_name, info.route_color, shape.shapeIndex, now, byTrip.get(tripId) ?? []) : []
        prepared.set(dir, {stops, shape, indexed})
      }
      for (const dir of ['0', '1']) {
        const tripId = `${info.route_id}${dir === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`
        const other = prepared.get(dir === '0' ? '1' : '0')!
        const feederTripId = otherDirectionTripId(tripId)
        const feeder: FeederTrip | undefined = other.shape ? {vehicles: other.indexed, index: other.shape.shapeIndex, tripStops: info.stop_times.filter((st) => st.trip_id === feederTripId)} : undefined
        const {stops, shape, indexed} = prepared.get(dir)!
        const rows = routeStopArrivals(info, stops, dir === '0', shape, indexed, now, feeder, feederTripId)
        dirs[dir] = {
          vehicles: indexed.length,
          stops: stops.map((s) => [s.stop_id, s.timeOffsetFromStart]),
          rows: rows.map((row) => row.map((a) => [a.minutes, a.isLive, a.soon])),
        }
      }
      routes[routeId] = dirs
    }
    expected.push({label: scenario.label, nowMs: scenario.nowMs, localMinutes: getMinutesFromDate(now), live: scenario.live, stops, routes})
  }
  fs.writeFileSync(out, JSON.stringify(expected))
})

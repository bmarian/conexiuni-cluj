import type {ShapeInfo, Stop, StopInfo, StopTime, TimeEntry} from '@/types/tranzy.ts'
import {apiRequest} from '@/utils/request_cache.ts'

export const WALKING_SPEED_M_PER_MIN = 80

export type RouteLeg = {
  routes: ShapeInfo[]
  startStop: StopInfo | Stop
  destStop: StopInfo | Stop
  tripIds: string[]
}

export type PlannedRoute = {
  legs: RouteLeg[]
  isDirect: boolean
  totalDistance: number
  walkStartMeters: number
  walkEndMeters: number
  transitDurationSec: number
}

export const getRouteIdFromTripId = (tripId: string): number | null => {
  const [routeIdPart] = String(tripId).split('_')
  const routeId = Number(routeIdPart)
  return Number.isFinite(routeId) ? routeId : null
}

export const getTripIdForRouteAtStop = (outgoingTripIds: string[], incomingTripIds: string[], routeId: number | string): string | undefined => {
  const wantedRouteId = Number(routeId)
  if (!Number.isFinite(wantedRouteId)) return undefined
  return [...(outgoingTripIds || []), ...(incomingTripIds || [])]
    .find((id) => getRouteIdFromTripId(id) === wantedRouteId)
}

export const getTimeOffsetToStop = (stopTimes: StopTime[], tripId: string, stopId: number): number => {
  let timeOffset = 0
  for (const st of stopTimes) {
    if (st.trip_id !== tripId) continue
    if (st.stop_id === stopId) break
    timeOffset += Math.ceil(st.offset_arrival_time / 60)
  }
  return timeOffset
}

// Minutes the bus takes to ride from `fromStopId` to `toStopId` on the given
// trip. The GTFS feed stores offset_arrival_time as a per-stop delta (seconds
// since the previous stop), so we sum deltas after fromStop, including the
// arrival delta into toStop. Used by the connection check so arrivalAtTransfer
// reflects when the user *actually arrives* at the transfer (boarding stop +
// ride), not when the bus left its terminus.
export const getRideMinutesBetweenStops = (
  stopTimes: StopTime[],
  tripId: string,
  fromStopId: number,
  toStopId: number,
): number => {
  let started = false
  let totalSec = 0
  for (const st of stopTimes) {
    if (st.trip_id !== tripId) continue
    if (!started) {
      if (st.stop_id === fromStopId) started = true
      continue
    }
    totalSec += st.offset_arrival_time
    if (st.stop_id === toStopId) return Math.ceil(totalSec / 60)
  }
  return 0
}

export const getShapeStopTimes = (shapeInfo: ShapeInfo | null | undefined): StopTime[] => {
  return shapeInfo?.stop_times ?? shapeInfo?.stop_time ?? []
}

// Stable identifier for a planned route — same shape across re-renders so
// downstream watches can gate on identity without relying on object refs.
export const routeSignature = (route: PlannedRoute): string => {
  return (route.isDirect ? 'D' : `C${route.legs.length}`) + ':' + route.legs.map(l => {
    const tid = l.tripIds[0] ?? ''
    const dir = tid.endsWith('_0') ? '0' : tid.endsWith('_1') ? '1' : 'x'
    return `${l.routes[0]?.route_id ?? 'x'}/${dir}@${l.startStop.stop_id}>${l.destStop.stop_id}`
  }).join('|')
}

export const estimateMinutesToDestination = (
  route: PlannedRoute,
  nextTimes: TimeEntry[]
): number => {
  if (!nextTimes.length) return Infinity
  const walkStartMin = route.walkStartMeters / WALKING_SPEED_M_PER_MIN
  const walkEndMin = route.walkEndMeters / WALKING_SPEED_M_PER_MIN
  const transitMin = route.transitDurationSec / 60
  const transferPenaltyMin = (route.legs.length - 1) * 5
  // Skip arrivals the user can't physically reach in time — wait for the next catchable one.
  const catchable = nextTimes.find(t => t.minutes >= walkStartMin) ?? nextTimes[nextTimes.length - 1]!
  return catchable.minutes + transitMin + transferPenaltyMin + walkEndMin
}

// Existing call sites in the view treat startStop/destStop as StopInfo and rely
// on shapes_info / outgoing_trip_ids. The backend response only carries basic Stop,
// so we synthesize the minimum the rest of the frontend code expects.
const stopInfoForLeg = (stop: Stop, shape: ShapeInfo, tripId: string): StopInfo => ({
  ...stop,
  shapes_info: [shape],
  outgoing_trip_ids: tripId.endsWith('_0') ? [tripId] : [],
  incoming_trip_ids: tripId.endsWith('_1') ? [tripId] : [],
})

type PlanLegResp = {
  route_id: number
  trip_id: string
  start_stop_id: number
  dest_stop_id: number
  ride_seconds: number
  intermediate_stop_ids: number[]
}

type PlanRouteResp = {
  legs: PlanLegResp[]
  is_direct: boolean
  walk_start_meters: number
  walk_end_meters: number
  walk_transfer_meters: number
  transit_duration_sec: number
  total_distance: number
}

type PlanResp = {
  plans: PlanRouteResp[]
  stops: Record<string, Stop>
  shapes: Record<string, ShapeInfo>
}

export const findRoutes = async (
  userLat: number,
  userLon: number,
  destLat: number,
  destLon: number
): Promise<PlannedRoute[]> => {
  const resp = await apiRequest(
    `plan_routes?from_lat=${userLat}&from_lng=${userLon}&to_lat=${destLat}&to_lng=${destLon}`
  ) as PlanResp

  if (!resp?.plans?.length) return []

  const plannedRoutes: PlannedRoute[] = []
  for (const p of resp.plans) {
    const legs: RouteLeg[] = []
    let valid = true
    for (const l of p.legs) {
      const start = resp.stops[String(l.start_stop_id)]
      const dest = resp.stops[String(l.dest_stop_id)]
      const shape = resp.shapes[String(l.route_id)]
      if (!start || !dest || !shape) {
        valid = false
        break
      }
      legs.push({
        routes: [shape],
        startStop: stopInfoForLeg(start, shape, l.trip_id),
        destStop: stopInfoForLeg(dest, shape, l.trip_id),
        tripIds: [l.trip_id],
      })
    }
    if (!valid) continue
    plannedRoutes.push({
      legs,
      isDirect: p.is_direct,
      totalDistance: p.total_distance,
      walkStartMeters: p.walk_start_meters,
      walkEndMeters: p.walk_end_meters,
      transitDurationSec: p.transit_duration_sec,
    })
  }

  return groupPlannedRoutes(plannedRoutes)
}

const groupPlannedRoutes = (routes: PlannedRoute[]): PlannedRoute[] => {
  const groups = new Map<string, PlannedRoute>()

  for (const r of routes) {
    const key = (r.isDirect ? 'D' : 'C') + r.legs.map(leg => {
      const tripId = leg.tripIds[0]
      const start = leg.startStop.stop_id
      const dest = leg.destStop.stop_id
      return `|${start}>${dest}@${tripId?.endsWith('_0') ? '0' : '1'}`
    }).join('')

    if (groups.has(key)) {
      const existing = groups.get(key)!
      for (let i = 0; i < r.legs.length; i++) {
        const rLeg = r.legs[i]
        const eLeg = existing.legs[i]
        if (!rLeg || !eLeg) continue
        const eStart = eLeg.startStop as StopInfo
        const eDest = eLeg.destStop as StopInfo
        for (let j = 0; j < rLeg.routes.length; j++) {
          const route = rLeg.routes[j]
          const tripId = rLeg.tripIds[j]
          if (!route || !tripId) continue
          if (eLeg.routes.some(rt => rt.route_id === route.route_id)) continue
          eLeg.routes.push(route)
          eLeg.tripIds.push(tripId)
          if (eStart.shapes_info && !eStart.shapes_info.some(s => s.route_id === route.route_id)) {
            eStart.shapes_info.push(route)
          }
          if (eDest.shapes_info && !eDest.shapes_info.some(s => s.route_id === route.route_id)) {
            eDest.shapes_info.push(route)
          }
          if (tripId.endsWith('_0')) {
            if (!eStart.outgoing_trip_ids.includes(tripId)) eStart.outgoing_trip_ids.push(tripId)
            if (!eDest.outgoing_trip_ids.includes(tripId)) eDest.outgoing_trip_ids.push(tripId)
          } else if (tripId.endsWith('_1')) {
            if (!eStart.incoming_trip_ids.includes(tripId)) eStart.incoming_trip_ids.push(tripId)
            if (!eDest.incoming_trip_ids.includes(tripId)) eDest.incoming_trip_ids.push(tripId)
          }
        }
      }
      existing.totalDistance = Math.min(existing.totalDistance, r.totalDistance)
    } else {
      groups.set(key, JSON.parse(JSON.stringify(r)))
    }
  }

  return Array.from(groups.values())
}

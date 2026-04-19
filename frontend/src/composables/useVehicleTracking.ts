import {apiRequest, HIGH_ACCURACY_SHELF_LIFE} from '@/utils/request_cache.ts'
import {calculateBearing, haversineMeters} from '@/utils/geo.ts'
import type {Shape, Vehicle} from '@/types/tranzy.ts'

export const CLOSE_TO_STOP_THRESHOLD = 200
export const VEHICLE_GRACE_PERIOD = 10
export const MIN_SPEED_KMH = 12
const STALE_POSITION_METERS = 20
const STALE_POSITION_MS = 3 * 60_000

const lastMovement = new Map<number, {lat: number; lon: number; movedAt: number}>()

function isStuckAtTerminus(vehicle: Vehicle, nearTerminus: boolean): boolean {
  const now = Date.now()
  const prev = lastMovement.get(vehicle.id)
  if (!prev) {
    lastMovement.set(vehicle.id, {lat: vehicle.latitude, lon: vehicle.longitude, movedAt: now})
    return false
  }
  const moved = haversineMeters(prev.lat, prev.lon, vehicle.latitude, vehicle.longitude)
  if (moved >= STALE_POSITION_METERS) {
    lastMovement.set(vehicle.id, {lat: vehicle.latitude, lon: vehicle.longitude, movedAt: now})
    return false
  }
  return nearTerminus && now - prev.movedAt >= STALE_POSITION_MS
}

export type TrackedVehicle = Vehicle & { route_short_name: string; route_color: string; heading: number }

export type ShapeIndex = {
  shape: Shape[]
  cumulativeDist: number[]
}

export type IndexedVehicle = TrackedVehicle & {
  shapeIdx: number
}

export function buildShapeIndex(shape: Shape[]): ShapeIndex {
  const cumulativeDist = new Array<number>(shape.length)
  if (!shape.length) return {shape, cumulativeDist}
  cumulativeDist[0] = 0
  for (let i = 1; i < shape.length; i++) {
    const a = shape[i - 1]!
    const b = shape[i]!
    const segment = haversineMeters(a.shape_pt_lat, a.shape_pt_lon, b.shape_pt_lat, b.shape_pt_lon)
    cumulativeDist[i] = cumulativeDist[i - 1]! + segment
  }
  return {shape, cumulativeDist}
}

export function findClosestShapeIdx(lat: number, lon: number, shape: Shape[]): number {
  let best = -1
  let bestDist = Infinity
  for (let i = 0; i < shape.length; i++) {
    const p = shape[i]!
    const d = haversineMeters(p.shape_pt_lat, p.shape_pt_lon, lat, lon)
    if (d < bestDist) { bestDist = d; best = i }
  }
  return best
}

export async function fetchVehiclesForTrips(tripIds: string[]): Promise<Map<string, Vehicle[]>> {
  const grouped = new Map<string, Vehicle[]>()
  if (!tripIds.length) return grouped

  const key = [...new Set(tripIds)].sort().join(',')
  const raw = (await apiRequest(`vehicles?trip_ids=${key}`, HIGH_ACCURACY_SHELF_LIFE) as Vehicle[]) ?? []

  for (const id of tripIds) grouped.set(id, [])
  for (const v of raw) {
    const bucket = grouped.get(v.trip_id)
    if (bucket) bucket.push(v)
  }
  return grouped
}

export async function getIndexedVehicles(
  tripId: string,
  routeShortName: string,
  routeColor: string,
  index: ShapeIndex,
  userTime?: Date | null,
  prefetched?: Vehicle[],
): Promise<IndexedVehicle[]> {
  const {shape} = index
  if (!shape.length) return []

  const raw = prefetched ?? ((await apiRequest(`vehicles?trip_id=${tripId}`, HIGH_ACCURACY_SHELF_LIFE) as Vehicle[]) ?? [])
  const firstStop = shape[0]!
  const lastStop = shape[shape.length - 1]!
  const now = userTime?.getTime() ?? Date.now()
  const result: IndexedVehicle[] = []

  for (const vehicle of raw) {
    if (!vehicle.latitude || !vehicle.longitude || vehicle.latitude < 0 || vehicle.longitude < 0) continue

    const nearFirst = haversineMeters(vehicle.latitude, vehicle.longitude, firstStop.shape_pt_lat, firstStop.shape_pt_lon) <= CLOSE_TO_STOP_THRESHOLD
    const nearLast  = haversineMeters(vehicle.latitude, vehicle.longitude, lastStop.shape_pt_lat,  lastStop.shape_pt_lon)  <= CLOSE_TO_STOP_THRESHOLD
    if ((nearFirst || nearLast) && vehicle.speed < MIN_SPEED_KMH + 1) continue
    if (isStuckAtTerminus(vehicle, nearFirst || nearLast)) continue

    const ts = new Date(vehicle.timestamp).getTime()
    if (isNaN(ts) || now - ts > VEHICLE_GRACE_PERIOD * 60_000) continue

    const shapeIdx = findClosestShapeIdx(vehicle.latitude, vehicle.longitude, shape)
    let heading = 0
    if (shapeIdx >= 0) {
      const target = shape[Math.min(shapeIdx + 3, shape.length - 1)]!
      heading = calculateBearing(vehicle.latitude, vehicle.longitude, target.shape_pt_lat, target.shape_pt_lon)
    }

    result.push({...vehicle, route_short_name: routeShortName, route_color: routeColor, heading, shapeIdx})
  }

  return result
}

export function etaForStop(
  stopShapeIdx: number,
  vehicles: IndexedVehicle[],
  index: ShapeIndex,
): {vehicle: IndexedVehicle; etaMinutes: number} | null {
  if (stopShapeIdx < 0) return null

  let bestVehicle: IndexedVehicle | null = null
  let bestIdx = -1
  for (const v of vehicles) {
    if (v.shapeIdx < 0 || v.shapeIdx > stopShapeIdx) continue
    if (v.shapeIdx > bestIdx) { bestIdx = v.shapeIdx; bestVehicle = v }
  }
  if (!bestVehicle) return null

  const distMeters = index.cumulativeDist[stopShapeIdx]! - index.cumulativeDist[bestVehicle.shapeIdx]!
  const speed = Math.max(bestVehicle.speed, MIN_SPEED_KMH)
  const etaMinutes = Math.ceil(((distMeters / 1000) / speed) * 60)
  return {vehicle: bestVehicle, etaMinutes}
}

export function getClosestNodeToPoint(
  {lat, lon}: {lat: number; lon: number},
  trip: Shape[],
): Shape | undefined {
  let closestDistance = Infinity
  let closest: Shape | undefined
  for (const point of trip) {
    const d = haversineMeters(point.shape_pt_lat, point.shape_pt_lon, lat, lon)
    if (d < closestDistance) {
      closestDistance = d
      closest = point
    }
  }
  return closest
}

export async function getVehiclesOnRoute(
  tripId: string,
  routeShortName: string,
  routeColor: string,
  trip: Shape[],
  userTime?: Date | null,
  prefetched?: Vehicle[],
): Promise<TrackedVehicle[]> {
  if (!trip?.length) return []

  const raw = prefetched ?? ((await apiRequest(`vehicles?trip_id=${tripId}`, HIGH_ACCURACY_SHELF_LIFE) as Vehicle[]) ?? [])
  const firstStop = trip[0]!
  const lastStop = trip[trip.length - 1]!
  const now = userTime?.getTime() ?? Date.now()

  const result: TrackedVehicle[] = []

  for (const vehicle of raw) {
    if (!vehicle.latitude || !vehicle.longitude || vehicle.latitude < 0 || vehicle.longitude < 0) continue

    const nearFirst = haversineMeters(vehicle.latitude, vehicle.longitude, firstStop.shape_pt_lat, firstStop.shape_pt_lon) <= CLOSE_TO_STOP_THRESHOLD
    const nearLast  = haversineMeters(vehicle.latitude, vehicle.longitude, lastStop.shape_pt_lat,  lastStop.shape_pt_lon)  <= CLOSE_TO_STOP_THRESHOLD
    if ((nearFirst || nearLast) && vehicle.speed < MIN_SPEED_KMH + 1) continue
    if (isStuckAtTerminus(vehicle, nearFirst || nearLast)) continue

    const ts = new Date(vehicle.timestamp).getTime()
    if (isNaN(ts) || now - ts > VEHICLE_GRACE_PERIOD * 60_000) continue

    let heading = 0
    const closestNode = getClosestNodeToPoint({lat: vehicle.latitude, lon: vehicle.longitude}, trip)
    if (closestNode) {
      const idx = trip.findIndex(t => t.shape_pt_sequence === closestNode.shape_pt_sequence)
      const target = trip[Math.min(idx + 3, trip.length - 1)]!
      heading = calculateBearing(vehicle.latitude, vehicle.longitude, target.shape_pt_lat, target.shape_pt_lon)
    }

    result.push({...vehicle, route_short_name: routeShortName, route_color: routeColor, heading})
  }

  return result
}

export function getClosestVehicleBeforeStop(
  vehicles: TrackedVehicle[],
  closestNodeToStop: Shape,
  trip: Shape[],
): {closestVehicle: TrackedVehicle | undefined; closestNode: Shape | undefined} {
  let closestDistance = Infinity
  let bestVehicle: TrackedVehicle | undefined
  let bestNode: Shape | undefined

  for (const vehicle of vehicles) {
    const currentNode = getClosestNodeToPoint({lat: vehicle.latitude, lon: vehicle.longitude}, trip)
    if (!currentNode || currentNode.shape_pt_sequence > closestNodeToStop.shape_pt_sequence) continue
    const d = haversineMeters(
      currentNode.shape_pt_lat, currentNode.shape_pt_lon,
      closestNodeToStop.shape_pt_lat, closestNodeToStop.shape_pt_lon,
    )
    if (d < closestDistance) { closestDistance = d; bestVehicle = vehicle; bestNode = currentNode }
  }

  return {closestVehicle: bestVehicle, closestNode: bestNode}
}

export function computeETA(stopShape: Shape, busShape: Shape, vehicle: TrackedVehicle, trip: Shape[]): number {
  const busIdx  = trip.findIndex(t => t.shape_pt_sequence === busShape.shape_pt_sequence)
  const stopIdx = trip.findIndex(t => t.shape_pt_sequence === stopShape.shape_pt_sequence)
  if (busIdx === -1 || stopIdx === -1) return -1
  if (busIdx > stopIdx) return -2

  let totalDistance = 0
  if (busIdx === stopIdx) {
    totalDistance = haversineMeters(vehicle.latitude, vehicle.longitude, stopShape.shape_pt_lat, stopShape.shape_pt_lon)
  } else {
    for (let i = busIdx; i < stopIdx; i++) {
      const cur = trip[i], next = trip[i + 1]
      if (cur && next) totalDistance += haversineMeters(cur.shape_pt_lat, cur.shape_pt_lon, next.shape_pt_lat, next.shape_pt_lon)
    }
  }

  return Math.ceil(((totalDistance / 1000) / Math.max(vehicle.speed, 12)) * 60)
}

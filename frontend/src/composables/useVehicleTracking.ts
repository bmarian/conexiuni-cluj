import {apiRequest} from '@/utils/api.ts'
import {calculateBearing, haversineMeters} from '@/utils/geo.ts'
import type {Shape, StopTime, Vehicle} from '@/types/tranzy.ts'

export const CLOSE_TO_STOP_THRESHOLD = 200
export const VEHICLE_GRACE_PERIOD = 10
export const MIN_SPEED_KMH = 7 // MinSpeedFloor
const STALE_POSITION_METERS = 20
const STALE_POSITION_MS = 3 * 60_000
const HEADING_LOOKAHEAD = 3

export type TrackedVehicle = Vehicle & {
  route_short_name: string;
  route_color: string;
  heading: number
}
export type IndexedVehicle = TrackedVehicle & { shapeIdx: number }
export type ShapeIndex = { shape: Shape[]; cumulativeDist: number[] }

const lastMovement = new Map<number, { lat: number; lon: number; movedAt: number }>()

function hasInvalidCoords(v: Vehicle): boolean {
  return !v.latitude || !v.longitude || v.latitude < 0 || v.longitude < 0
}

function isStale(v: Vehicle, now: number): boolean {
  const ts = new Date(v.timestamp).getTime()
  return isNaN(ts) || now - ts > VEHICLE_GRACE_PERIOD * 60_000
}

function isStuckAtTerminus(v: Vehicle, nearTerminus: boolean, now: number): boolean {
  const prev = lastMovement.get(v.id)
  if (!prev) {
    lastMovement.set(v.id, {lat: v.latitude, lon: v.longitude, movedAt: now})
    return false
  }
  const moved = haversineMeters(prev.lat, prev.lon, v.latitude, v.longitude)
  if (moved >= STALE_POSITION_METERS) {
    lastMovement.set(v.id, {lat: v.latitude, lon: v.longitude, movedAt: now})
    return false
  }
  return nearTerminus && now - prev.movedAt >= STALE_POSITION_MS
}

function isVisibleAtTerminus(v: Vehicle, firstStop: Shape, lastStop: Shape, now: number): boolean {
  const nearFirst = haversineMeters(v.latitude, v.longitude, firstStop.shape_pt_lat, firstStop.shape_pt_lon) <= CLOSE_TO_STOP_THRESHOLD
  const nearLast = haversineMeters(v.latitude, v.longitude, lastStop.shape_pt_lat, lastStop.shape_pt_lon) <= CLOSE_TO_STOP_THRESHOLD
  const nearTerminus = nearFirst || nearLast
  if (nearTerminus && v.speed < MIN_SPEED_KMH + 1) return false
  return !isStuckAtTerminus(v, nearTerminus, now)
}

function computeHeading(lat: number, lon: number, shape: Shape[], shapeIdx: number): number {
  if (shapeIdx < 0 || !shape.length) return 0
  const targetIdx = Math.min(shapeIdx + HEADING_LOOKAHEAD, shape.length - 1)
  const target = shape[targetIdx]!

  // If we're at the very end or too close to the target point to get a stable bearing,
  // use the direction of the segment leading to the current point.
  if (targetIdx === shapeIdx || haversineMeters(lat, lon, target.shape_pt_lat, target.shape_pt_lon) < 2) {
    if (shapeIdx > 0) {
      const prev = shape[shapeIdx - 1]!
      const curr = shape[shapeIdx]!
      return calculateBearing(prev.shape_pt_lat, prev.shape_pt_lon, curr.shape_pt_lat, curr.shape_pt_lon)
    }
    if (shape.length > 1) {
      // At index 0, use the first segment direction
      return calculateBearing(shape[0]!.shape_pt_lat, shape[0]!.shape_pt_lon, shape[1]!.shape_pt_lat, shape[1]!.shape_pt_lon)
    }
  }

  return calculateBearing(lat, lon, target.shape_pt_lat, target.shape_pt_lon)
}

function forEachVisibleVehicle(
  raw: Vehicle[],
  shape: Shape[],
  now: number,
  onVehicle: (vehicle: Vehicle, shapeIdx: number, heading: number) => void,
): void {
  const firstStop = shape[0]!
  const lastStop = shape[shape.length - 1]!

  for (const v of raw) {
    if (hasInvalidCoords(v)) continue
    if (isStale(v, now)) continue
    if (!isVisibleAtTerminus(v, firstStop, lastStop, now)) continue

    const shapeIdx = findClosestShapeIdx(v.latitude, v.longitude, shape)
    const heading = computeHeading(v.latitude, v.longitude, shape, shapeIdx)
    onVehicle(v, shapeIdx, heading)
  }
}

function distanceOnShape(index: ShapeIndex, fromShapeIdx: number, toShapeIdx: number): number {
  if (fromShapeIdx < 0 || toShapeIdx < 0 || fromShapeIdx > toShapeIdx) return 0
  return index.cumulativeDist[toShapeIdx]! - index.cumulativeDist[fromShapeIdx]!
}

function estimateEtaMinutes(distanceMeters: number, speedKmh: number): number {
  const speed = Math.max(speedKmh, MIN_SPEED_KMH)
  return Math.ceil(((distanceMeters / 1000) / speed) * 60)
}

export function buildShapeIndex(shape: Shape[]): ShapeIndex {
  const cumulativeDist = Array.from<number>({ length: shape.length })
  if (!shape.length) return {shape, cumulativeDist}
  cumulativeDist[0] = 0
  for (let i = 1; i < shape.length; i++) {
    const a = shape[i - 1]!
    const b = shape[i]!
    cumulativeDist[i] = cumulativeDist[i - 1]! + haversineMeters(a.shape_pt_lat, a.shape_pt_lon, b.shape_pt_lat, b.shape_pt_lon)
  }
  return {shape, cumulativeDist}
}

export function findClosestShapeIdx(lat: number, lon: number, shape: Shape[]): number {
  let best = -1
  let bestDist = Infinity
  for (let i = 0; i < shape.length; i++) {
    const p = shape[i]!
    const d = haversineMeters(p.shape_pt_lat, p.shape_pt_lon, lat, lon)
    if (d < bestDist) {
      bestDist = d;
      best = i
    }
  }
  return best
}

export function buildStopShapeIdxByStopId(tripStops: StopTime[], shape: Shape[]): Map<number, number> {
  const stopShapeIdxByStopId = new Map<number, number>()
  for (const st of tripStops) {
    if (!st.stop_lat || !st.stop_lon) continue
    stopShapeIdxByStopId.set(st.stop_id, findClosestShapeIdx(st.stop_lat, st.stop_lon, shape))
  }
  return stopShapeIdxByStopId
}

async function fetchRawVehicles(tripId: string, prefetched?: Vehicle[]): Promise<Vehicle[]> {
  if (prefetched) return prefetched
  return (await apiRequest(`vehicles?trip_id=${tripId}`) as Vehicle[]) ?? []
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

  const raw = await fetchRawVehicles(tripId, prefetched)
  const now = userTime?.getTime() ?? Date.now()
  const result: IndexedVehicle[] = []

  forEachVisibleVehicle(raw, shape, now, (v, shapeIdx, heading) => {
    result.push({
      ...v,
      route_short_name: routeShortName,
      route_color: routeColor,
      heading,
      shapeIdx
    })
  })

  return result
}

export async function getVehiclesOnRoute(
  tripId: string,
  routeShortName: string,
  routeColor: string,
  trip: Shape[],
  userTime?: Date | null,
  prefetched?: Vehicle[],
): Promise<TrackedVehicle[]> {
  if (!trip.length) return []

  const raw = await fetchRawVehicles(tripId, prefetched)
  const now = userTime?.getTime() ?? Date.now()
  const result: TrackedVehicle[] = []

  forEachVisibleVehicle(raw, trip, now, (v, _, heading) => {
    result.push({...v, route_short_name: routeShortName, route_color: routeColor, heading})
  })

  return result
}

export function etaForStop(
  stopShapeIdx: number,
  vehicles: IndexedVehicle[],
  index: ShapeIndex,
): { vehicle: IndexedVehicle; etaMinutes: number } | null {
  if (stopShapeIdx < 0) return null

  const candidates = vehicles
    .filter(v => v.shapeIdx >= 0 && v.shapeIdx <= stopShapeIdx)
    .sort((a, b) => b.shapeIdx - a.shapeIdx)

  for (const v of candidates) {
    const distMeters = distanceOnShape(index, v.shapeIdx, stopShapeIdx)
    const etaMinutes = estimateEtaMinutes(distMeters, v.speed)
    if (etaMinutes > 0) return {vehicle: v, etaMinutes}
  }

  return null
}

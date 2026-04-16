import {apiRequest, HIGH_ACCURACY_SHELF_LIFE} from '@/utils/request_cache.ts'
import {calculateBearing, haversineMeters} from '@/utils/geo.ts'
import type {Shape, Vehicle} from '@/types/tranzy.ts'


export const CLOSE_TO_STOP_THRESHOLD = 200  // metres
export const VEHICLE_GRACE_PERIOD = 10       // minutes
export const MIN_SPEED_KMH = 12              // floor for ETA divisor (km/h)

export type TrackedVehicle = Vehicle & { route_short_name: string; heading: number }

/**
 * Precomputed index over a shape polyline for O(1) lookups during rendering.
 * Build once per direction; reuse for every stop / vehicle / ETA computation.
 */
export type ShapeIndex = {
  shape: Shape[]
  /** Cumulative haversine distance (m) from shape[0] to shape[i]. */
  cumulativeDist: number[]
}

export type IndexedVehicle = TrackedVehicle & {
  /** Index into ShapeIndex.shape; -1 if no shape match. */
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

/** Linear scan returning the index of the shape point closest to lat/lon. -1 if shape is empty. */
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

/**
 * Same logic as getVehiclesOnRoute but uses a precomputed ShapeIndex and stamps
 * each vehicle with its `shapeIdx` so the render path doesn't have to scan the
 * polyline for every (stop × vehicle) combination.
 */
export async function getIndexedVehicles(
  tripId: string,
  routeShortName: string,
  index: ShapeIndex,
  userTime?: Date | null,
): Promise<IndexedVehicle[]> {
  const {shape} = index
  if (!shape.length) return []

  const raw = (await apiRequest(`vehicles?trip_id=${tripId}`, HIGH_ACCURACY_SHELF_LIFE) as Vehicle[]) ?? []
  const firstStop = shape[0]!
  const lastStop = shape[shape.length - 1]!
  const now = userTime?.getTime() ?? Date.now()
  const result: IndexedVehicle[] = []

  for (const vehicle of raw) {
    if (!vehicle.latitude || !vehicle.longitude || vehicle.latitude < 0 || vehicle.longitude < 0) continue

    const nearFirst = haversineMeters(vehicle.latitude, vehicle.longitude, firstStop.shape_pt_lat, firstStop.shape_pt_lon) <= CLOSE_TO_STOP_THRESHOLD
    const nearLast  = haversineMeters(vehicle.latitude, vehicle.longitude, lastStop.shape_pt_lat,  lastStop.shape_pt_lon)  <= CLOSE_TO_STOP_THRESHOLD
    if ((nearFirst || nearLast) && vehicle.speed < 1) continue

    const ts = new Date(vehicle.timestamp).getTime()
    if (isNaN(ts) || now - ts > VEHICLE_GRACE_PERIOD * 60_000) continue

    const shapeIdx = findClosestShapeIdx(vehicle.latitude, vehicle.longitude, shape)
    let heading = 0
    if (shapeIdx >= 0) {
      const target = shape[Math.min(shapeIdx + 3, shape.length - 1)]!
      heading = calculateBearing(vehicle.latitude, vehicle.longitude, target.shape_pt_lat, target.shape_pt_lon)
    }

    result.push({...vehicle, route_short_name: routeShortName, heading, shapeIdx})
  }

  return result
}

/**
 * O(vehicles) ETA: find the vehicle that is BEFORE the given stop along the
 * polyline and closest to it (highest shapeIdx <= stopIdx), then derive ETA
 * from the precomputed cumulative distance array. Returns null if no valid
 * vehicle is found.
 */
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

/** Returns the shape-point on `trip` that is closest to the given lat/lon. */
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

/**
 * Fetches live vehicles for `tripId`, filters out:
 *  - vehicles parked at terminals (near first/last stop AND speed < 1)
 *  - vehicles with stale timestamps (> VEHICLE_GRACE_PERIOD minutes old)
 *  - vehicles with missing/invalid coordinates
 *
 * Enriches each vehicle with `route_short_name` and a computed `heading`
 * derived from the shape geometry.
 */
export async function getVehiclesOnRoute(
  tripId: string,
  routeShortName: string,
  trip: Shape[],
  userTime?: Date | null,
): Promise<TrackedVehicle[]> {
  if (!trip?.length) return []

  const raw = (await apiRequest(`vehicles?trip_id=${tripId}`, HIGH_ACCURACY_SHELF_LIFE) as Vehicle[]) ?? []
  const firstStop = trip[0]!
  const lastStop = trip[trip.length - 1]!
  const now = userTime?.getTime() ?? Date.now()

  const result: TrackedVehicle[] = []

  for (const vehicle of raw) {
    // Skip vehicles with missing/invalid coordinates
    if (!vehicle.latitude || !vehicle.longitude || vehicle.latitude < 0 || vehicle.longitude < 0) continue

    // Skip stationary vehicles sitting at terminals
    const nearFirst = haversineMeters(vehicle.latitude, vehicle.longitude, firstStop.shape_pt_lat, firstStop.shape_pt_lon) <= CLOSE_TO_STOP_THRESHOLD
    const nearLast  = haversineMeters(vehicle.latitude, vehicle.longitude, lastStop.shape_pt_lat,  lastStop.shape_pt_lon)  <= CLOSE_TO_STOP_THRESHOLD
    if ((nearFirst || nearLast) && vehicle.speed < 1) continue

    // Skip stale data
    const ts = new Date(vehicle.timestamp).getTime()
    if (isNaN(ts) || now - ts > VEHICLE_GRACE_PERIOD * 60_000) continue

    // Compute heading from shape geometry
    let heading = 0
    const closestNode = getClosestNodeToPoint({lat: vehicle.latitude, lon: vehicle.longitude}, trip)
    if (closestNode) {
      const idx = trip.findIndex(t => t.shape_pt_sequence === closestNode.shape_pt_sequence)
      const target = trip[Math.min(idx + 3, trip.length - 1)]!
      heading = calculateBearing(vehicle.latitude, vehicle.longitude, target.shape_pt_lat, target.shape_pt_lon)
    }

    result.push({...vehicle, route_short_name: routeShortName, heading})
  }

  return result
}

/**
 * Among `vehicles`, finds the one that is closest to `closestNodeToStop`
 * along the shape while still being BEFORE the stop (lower shape_pt_sequence).
 */
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

/**
 * Computes minutes for a vehicle to travel from its current position to a stop,
 * using the shape polyline for distance and the vehicle's speed.
 * Returns -1 if shapes can't be matched, -2 if the vehicle has already passed the stop.
 */
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

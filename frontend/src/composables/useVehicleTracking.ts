import {apiRequest} from '@/utils/api.ts'
import {calculateBearing, haversineMeters} from '@/utils/geo.ts'
import type {Shape, StopTime, Vehicle} from '@/types/tranzy.ts'

export const CLOSE_TO_STOP_THRESHOLD = 200
export const VEHICLE_GRACE_PERIOD = 10
export const MIN_SPEED_KMH = 7 // MinSpeedFloor
const STALE_POSITION_METERS = 20
const STALE_POSITION_MS = 3 * 60_000
const HEADING_LOOKAHEAD = 3
const LIVE_ETA_MAX_POSITION_AGE_MS = 3 * 60_000
// userTime ticks every 10s, so a just-fetched position often reads as future-dated.
const LIVE_ETA_MAX_CLOCK_SKEW_MS = 60_000
const PROFILE_SEGMENT_WEIGHT = 0.65
const FALLBACK_SEGMENT_CONFIDENCE = 0.25

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

function isFreshForLiveEta(v: Vehicle, now: number): boolean {
  const ts = new Date(v.timestamp).getTime()
  if (isNaN(ts)) return false
  const age = now - ts
  return age >= -LIVE_ETA_MAX_CLOCK_SKEW_MS && age <= LIVE_ETA_MAX_POSITION_AGE_MS
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

function estimateEtaSeconds(distanceMeters: number, speedKmh: number): number {
  const speed = Math.max(speedKmh, MIN_SPEED_KMH)
  return ((distanceMeters / 1000) / speed) * 3600
}

function estimateEtaMinutes(distanceMeters: number, speedKmh: number): number {
  return Math.ceil(estimateEtaSeconds(distanceMeters, speedKmh) / 60)
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

function sortedTripStops(tripStops: StopTime[]): StopTime[] {
  return [...tripStops].sort((a, b) => a.stop_sequence - b.stop_sequence)
}

function stopShapePositions(tripStops: StopTime[], shape: Shape[]): number[] {
  let last = 0
  return tripStops.map((st) => {
    const idx = st.stop_lat && st.stop_lon ? findClosestShapeIdx(st.stop_lat, st.stop_lon, shape) : last
    last = Math.max(last, idx)
    return last
  })
}

function blendedRemainingSegmentSeconds(segmentSec: number, segmentMeters: number, remainingMeters: number, speedKmh: number, confidence: number): number {
  const liveSec = estimateEtaSeconds(remainingMeters, speedKmh)
  if (segmentSec <= 0 || segmentMeters <= 0) return liveSec
  const ratio = Math.min(1, Math.max(0, remainingMeters / segmentMeters))
  const profileSec = segmentSec * ratio
  const profileWeight = PROFILE_SEGMENT_WEIGHT * Math.min(1, Math.max(0, confidence))
  return liveSec * (1 - profileWeight) + profileSec * profileWeight
}

type EtaOptions = {
  tripStops?: StopTime[]
  targetStopId?: number
  referenceTime?: Date | null
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
  options: EtaOptions = {},
): { vehicle: IndexedVehicle; etaMinutes: number } | null {
  if (stopShapeIdx < 0) return null

  const now = options.referenceTime?.getTime() ?? Date.now()
  const candidates = vehicles
    .filter(v => v.shapeIdx >= 0 && v.shapeIdx <= stopShapeIdx && isFreshForLiveEta(v, now))
    .sort((a, b) => b.shapeIdx - a.shapeIdx)

  for (const v of candidates) {
    const etaMinutes = profileAwareEtaMinutes(v, stopShapeIdx, index, options)
      ?? estimateEtaMinutes(distanceOnShape(index, v.shapeIdx, stopShapeIdx), v.speed)
    if (etaMinutes > 0) return {vehicle: v, etaMinutes}
  }

  return null
}

function profileAwareEtaMinutes(
  vehicle: IndexedVehicle,
  stopShapeIdx: number,
  index: ShapeIndex,
  options: EtaOptions,
): number | null {
  const stops = options.tripStops?.length ? sortedTripStops(options.tripStops) : []
  if (!stops.length || options.targetStopId === undefined) return null

  const targetPos = stops.findIndex(st => st.stop_id === options.targetStopId)
  if (targetPos < 0) return null

  const positions = stopShapePositions(stops, index.shape)
  if (targetPos === 0) {
    return estimateEtaMinutes(distanceOnShape(index, vehicle.shapeIdx, stopShapeIdx), vehicle.speed)
  }

  let prevPos = -1
  for (let i = 0; i < targetPos; i++) {
    if (positions[i]! <= vehicle.shapeIdx) prevPos = i
    else break
  }
  if (prevPos < 0) return null

  const nextPos = prevPos + 1
  if (nextPos > targetPos) return null

  const currentStartIdx = positions[prevPos]!
  const currentEndIdx = positions[nextPos]!
  const remainingMeters = distanceOnShape(index, Math.max(vehicle.shapeIdx, currentStartIdx), currentEndIdx)
  const segmentMeters = distanceOnShape(index, currentStartIdx, currentEndIdx)
  let seconds = blendedRemainingSegmentSeconds(
    stops[nextPos]!.offset_arrival_time,
    segmentMeters,
    remainingMeters,
    vehicle.speed,
    stops[nextPos]!.offset_confidence || FALLBACK_SEGMENT_CONFIDENCE,
  )

  for (let pos = nextPos + 1; pos <= targetPos; pos++) {
    seconds += stops[pos]!.offset_arrival_time
  }

  return Math.ceil(seconds / 60)
}

import {apiRequest} from '@/utils/api.ts'
import {calculateBearing, haversineMeters} from '@/utils/geo.ts'
import type {Shape, StopTime, Vehicle} from '@/types/tranzy.ts'

export const TERMINUS_RADIUS_METERS = 200
export const VEHICLE_GRACE_PERIOD = 10
export const MIN_SPEED_KMH = 7 // MinSpeedFloor
// A bus standing this long is out of service, not on a layover. Terminus layovers
// run 5-15 min, so this only catches vehicles parked with a stale trip assigned.
const LAYOVER_HIDE_MS = 30 * 60_000
const HEADING_LOOKAHEAD = 3
const LIVE_ETA_MAX_POSITION_AGE_MS = 4 * 60_000
// userTime ticks every 10s, so a just-fetched position often reads as future-dated.
const LIVE_ETA_MAX_CLOCK_SKEW_MS = 60_000
// Tranzy drops vehicles from a poll now and then. Keep counting the last estimate
// down instead of snapping the row back to its timetable value and back again.
const LIVE_ETA_HOLD_MS = 3 * 60_000
const ETA_SMOOTHING = 0.4
// Past this the new estimate is a different reality (traffic, a re-route), not
// noise - take it as is rather than easing towards it.
const ETA_SNAP_MS = 5 * 60_000
const ETA_CACHE_MAX_ENTRIES = 400
const PROFILE_SEGMENT_WEIGHT = 0.65
const FALLBACK_SEGMENT_CONFIDENCE = 0.25

export type TrackedVehicle = Vehicle & {
  route_short_name: string;
  route_color: string;
  heading: number
}
export type IndexedVehicle = TrackedVehicle & {
  shapeIdx: number
  // Sitting at (or still rolling into) the departure terminus: drawn on the map,
  // but never the source of a live ETA - see etaForStop.
  atStartTerminus: boolean
  dwellMs: number
}
export type ShapeIndex = { shape: Shape[]; cumulativeDist: number[] }
export type StopEta = { vehicle: IndexedVehicle | null; etaMinutes: number }

function hasInvalidCoords(v: Vehicle): boolean {
  return !v.latitude || !v.longitude || v.latitude < 0 || v.longitude < 0
}

function vehicleTimestamp(v: Vehicle, fallback: number): number {
  const ts = new Date(v.timestamp).getTime()
  return isNaN(ts) ? fallback : ts
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

// How long the vehicle has been standing, per the backend's movement anchor.
// Absent (older backend, first fix) reads as moving, so nothing is hidden by default.
function dwellMs(v: Vehicle, now: number): number {
  if (!v.stationary_since) return 0
  const since = new Date(v.stationary_since).getTime()
  if (isNaN(since)) return 0
  return Math.max(0, now - since)
}

function isTracked(v: Vehicle, now: number): boolean {
  if (hasInvalidCoords(v)) return false
  if (isStale(v, now)) return false
  return dwellMs(v, now) < LAYOVER_HIDE_MS
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

function distanceOnShape(index: ShapeIndex, fromShapeIdx: number, toShapeIdx: number): number {
  if (fromShapeIdx < 0 || toShapeIdx < 0 || fromShapeIdx > toShapeIdx) return 0
  return index.cumulativeDist[toShapeIdx]! - index.cumulativeDist[fromShapeIdx]!
}

function estimateEtaSeconds(distanceMeters: number, speedKmh: number): number {
  const speed = Math.max(speedKmh, MIN_SPEED_KMH)
  return ((distanceMeters / 1000) / speed) * 3600
}

export function buildShapeIndex(shape: Shape[]): ShapeIndex {
  const cumulativeDist = Array.from<number>({length: shape.length})
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
  tripId?: string
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
  const start = shape[0]!
  const result: IndexedVehicle[] = []

  for (const v of raw) {
    if (!isTracked(v, now)) continue
    const shapeIdx = findClosestShapeIdx(v.latitude, v.longitude, shape)
    result.push({
      ...v,
      route_short_name: routeShortName,
      route_color: routeColor,
      heading: computeHeading(v.latitude, v.longitude, shape, shapeIdx),
      shapeIdx,
      atStartTerminus: haversineMeters(v.latitude, v.longitude, start.shape_pt_lat, start.shape_pt_lon) <= TERMINUS_RADIUS_METERS,
      dwellMs: dwellMs(v, now),
    })
  }

  return result
}

// Live ETAs are kept as an absolute arrival instant rather than a minute count, so
// the number ticks down between polls instead of freezing and then jumping, and so
// one missing poll does not drop the row back to its timetable value.
type LiveEta = { vehicleId: number; dataTs: number; arrivalAt: number }

const liveEtaCache = new Map<string, LiveEta>()

function etaCacheKey(options: EtaOptions): string | null {
  if (!options.tripId || options.targetStopId === undefined) return null
  return `${options.tripId}:${options.targetStopId}`
}

function pruneLiveEtaCache(now: number): void {
  if (liveEtaCache.size <= ETA_CACHE_MAX_ENTRIES) return
  for (const [key, entry] of liveEtaCache) {
    if (now - entry.dataTs > LIVE_ETA_HOLD_MS) liveEtaCache.delete(key)
  }
}

// Idempotent per data frame: the views call etaForStop on every render, so the same
// (vehicle, timestamp) pair has to keep producing the same arrival instant.
function commitLiveEta(key: string | null, vehicleId: number, dataTs: number, rawArrivalAt: number): number {
  if (!key) return rawArrivalAt
  const prev = liveEtaCache.get(key)
  if (prev && prev.vehicleId === vehicleId && prev.dataTs === dataTs) return prev.arrivalAt

  let arrivalAt = rawArrivalAt
  if (prev && prev.vehicleId === vehicleId && Math.abs(rawArrivalAt - prev.arrivalAt) <= ETA_SNAP_MS) {
    arrivalAt = prev.arrivalAt + ETA_SMOOTHING * (rawArrivalAt - prev.arrivalAt)
  }
  liveEtaCache.set(key, {vehicleId, dataTs, arrivalAt})
  pruneLiveEtaCache(dataTs)
  return arrivalAt
}

function heldLiveEta(key: string | null, now: number, vehicles: IndexedVehicle[], stopShapeIdx: number): number | null {
  if (!key) return null
  const entry = liveEtaCache.get(key)
  if (!entry) return null

  // The vehicle is still being reported and has driven past the stop: the run is
  // done, so the estimate is spent even if it had not counted down to zero yet.
  const owner = vehicles.find((v) => v.id === entry.vehicleId)
  const expired = now - entry.dataTs > LIVE_ETA_HOLD_MS
    || (owner !== undefined && owner.shapeIdx > stopShapeIdx)
    || entry.arrivalAt < now - 60_000
  if (expired) {
    liveEtaCache.delete(key)
    return null
  }
  return Math.max(0, Math.round((entry.arrivalAt - now) / 60_000))
}

export function etaForStop(
  stopShapeIdx: number,
  vehicles: IndexedVehicle[],
  index: ShapeIndex,
  options: EtaOptions = {},
): StopEta | null {
  if (stopShapeIdx < 0) return null

  const now = options.referenceTime?.getTime() ?? Date.now()
  const key = etaCacheKey(options)

  // A vehicle still at the departure terminus says nothing about when it leaves -
  // that is the timetable's job. Tracking it would read as "0m" for the whole
  // layover and then flip back to the schedule the moment it pulled away.
  const candidate = vehicles
    .filter(v => v.shapeIdx >= 0 && v.shapeIdx <= stopShapeIdx && !v.atStartTerminus && isFreshForLiveEta(v, now))
    .sort((a, b) => b.shapeIdx - a.shapeIdx)[0]

  if (candidate) {
    const seconds = etaSecondsForStop(candidate, stopShapeIdx, index, options)
    const dataTs = vehicleTimestamp(candidate, now)
    const arrivalAt = commitLiveEta(key, candidate.id, dataTs, dataTs + seconds * 1000)
    return {vehicle: candidate, etaMinutes: Math.max(0, Math.round((arrivalAt - now) / 60_000))}
  }

  const held = heldLiveEta(key, now, vehicles, stopShapeIdx)
  return held === null ? null : {vehicle: null, etaMinutes: held}
}

function etaSecondsForStop(
  vehicle: IndexedVehicle,
  stopShapeIdx: number,
  index: ShapeIndex,
  options: EtaOptions,
): number {
  return profileAwareEtaSeconds(vehicle, stopShapeIdx, index, options)
    ?? estimateEtaSeconds(distanceOnShape(index, vehicle.shapeIdx, stopShapeIdx), vehicle.speed)
}

function profileAwareEtaSeconds(
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
    return estimateEtaSeconds(distanceOnShape(index, vehicle.shapeIdx, stopShapeIdx), vehicle.speed)
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

  return seconds
}

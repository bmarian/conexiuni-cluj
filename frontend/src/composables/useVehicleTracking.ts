import { apiRequest } from "@/utils/api.ts";
import { calculateBearing, haversineMeters } from "@/utils/geo.ts";
import type { Shape, StopTime, Vehicle } from "@/types/tranzy.ts";

export const TERMINUS_RADIUS_METERS = 200;
// Tranzy tags a vehicle with the trip it is *about* to run, so a bus deadheading
// back to the terminus carries the outbound trip while it is streets away. Snapped
// to the nearest shape point it lands on index 0 and then drives the live ETA for
// every stop on the route. Stops on these shapes sit within ~25 m of the line, so
// anything past this is not on the route.
const MAX_OFF_ROUTE_METERS = 150;
export const VEHICLE_GRACE_PERIOD = 10;
export const MIN_SPEED_KMH = 7; // MinSpeedFloor
// A bus standing this long is out of service, not on a layover. Terminus layovers
// run 5-15 min, so this only catches vehicles parked with a stale trip assigned.
const LAYOVER_HIDE_MS = 30 * 60_000;
const HEADING_LOOKAHEAD = 3;
const LIVE_ETA_MAX_POSITION_AGE_MS = 4 * 60_000;
// userTime ticks every 10s, so a just-fetched position often reads as future-dated.
const LIVE_ETA_MAX_CLOCK_SKEW_MS = 60_000;
// No bus covers ground faster than this, so no estimate may claim it did. This is
// the backstop that keeps a stop from reading "now" while the bus is streets away.
const MAX_PLAUSIBLE_SPEED_MPS = 50 / 3.6;
// A position fix is already this old by the time it is drawn, so the bus has covered
// ground the estimate would otherwise still charge for. Capped, because past a minute
// or so the silence is more likely a gap in the feed than a bus making progress.
const MAX_FIX_AGE_CREDIT_SEC = 90;

export type TrackedVehicle = Vehicle & {
  route_short_name: string;
  route_color: string;
  heading: number;
};
export type IndexedVehicle = TrackedVehicle & {
  shapeIdx: number;
  // Distance from the shape at shapeIdx. Snapping is unconditional, so this is the
  // only thing separating a bus on the route from one that merely carries its trip.
  offRouteMeters: number;
  // Sitting at (or still rolling into) the departure terminus: drawn on the map,
  // but never the source of a live ETA - see etaForStop.
  atStartTerminus: boolean;
  dwellMs: number;
};
export type ShapeIndex = { shape: Shape[]; cumulativeDist: number[] };
export type StopEta = { vehicle: IndexedVehicle | null; etaMinutes: number; etaSeconds: number };

function hasInvalidCoords(v: Vehicle): boolean {
  return !v.latitude || !v.longitude || v.latitude < 0 || v.longitude < 0;
}

function isStale(v: Vehicle, now: number): boolean {
  const ts = new Date(v.timestamp).getTime();
  return isNaN(ts) || now - ts > VEHICLE_GRACE_PERIOD * 60_000;
}

function isFreshForLiveEta(v: Vehicle, now: number): boolean {
  const ts = new Date(v.timestamp).getTime();
  if (isNaN(ts)) return false;
  const age = now - ts;
  return age >= -LIVE_ETA_MAX_CLOCK_SKEW_MS && age <= LIVE_ETA_MAX_POSITION_AGE_MS;
}

// How long the vehicle has been standing, per the backend's movement anchor.
// Absent (older backend, first fix) reads as moving, so nothing is hidden by default.
function dwellMs(v: Vehicle, now: number): number {
  if (!v.stationary_since) return 0;
  const since = new Date(v.stationary_since).getTime();
  if (isNaN(since)) return 0;
  return Math.max(0, now - since);
}

function isTracked(v: Vehicle, now: number): boolean {
  if (hasInvalidCoords(v)) return false;
  if (isStale(v, now)) return false;
  return dwellMs(v, now) < LAYOVER_HIDE_MS;
}

function computeHeading(lat: number, lon: number, shape: Shape[], shapeIdx: number): number {
  if (shapeIdx < 0 || !shape.length) return 0;
  const targetIdx = Math.min(shapeIdx + HEADING_LOOKAHEAD, shape.length - 1);
  const target = shape[targetIdx]!;

  // If we're at the very end or too close to the target point to get a stable bearing,
  // use the direction of the segment leading to the current point.
  if (
    targetIdx === shapeIdx ||
    haversineMeters(lat, lon, target.shape_pt_lat, target.shape_pt_lon) < 2
  ) {
    if (shapeIdx > 0) {
      const prev = shape[shapeIdx - 1]!;
      const curr = shape[shapeIdx]!;
      return calculateBearing(
        prev.shape_pt_lat,
        prev.shape_pt_lon,
        curr.shape_pt_lat,
        curr.shape_pt_lon,
      );
    }
    if (shape.length > 1) {
      // At index 0, use the first segment direction
      return calculateBearing(
        shape[0]!.shape_pt_lat,
        shape[0]!.shape_pt_lon,
        shape[1]!.shape_pt_lat,
        shape[1]!.shape_pt_lon,
      );
    }
  }

  return calculateBearing(lat, lon, target.shape_pt_lat, target.shape_pt_lon);
}

export function distanceOnShape(
  index: ShapeIndex,
  fromShapeIdx: number,
  toShapeIdx: number,
): number {
  if (fromShapeIdx < 0 || toShapeIdx < 0 || fromShapeIdx > toShapeIdx) return 0;
  return index.cumulativeDist[toShapeIdx]! - index.cumulativeDist[fromShapeIdx]!;
}

function estimateEtaSeconds(distanceMeters: number, speedKmh: number): number {
  const speed = Math.max(speedKmh, MIN_SPEED_KMH);
  return (distanceMeters / 1000 / speed) * 3600;
}

export function buildShapeIndex(shape: Shape[]): ShapeIndex {
  const cumulativeDist = Array.from<number>({ length: shape.length });
  if (!shape.length) return { shape, cumulativeDist };
  cumulativeDist[0] = 0;
  for (let i = 1; i < shape.length; i++) {
    const a = shape[i - 1]!;
    const b = shape[i]!;
    cumulativeDist[i] =
      cumulativeDist[i - 1]! +
      haversineMeters(a.shape_pt_lat, a.shape_pt_lon, b.shape_pt_lat, b.shape_pt_lon);
  }
  return { shape, cumulativeDist };
}

export function closestShapePoint(
  lat: number,
  lon: number,
  shape: Shape[],
): { idx: number; distanceMeters: number } {
  let idx = -1;
  let distanceMeters = Infinity;
  for (let i = 0; i < shape.length; i++) {
    const p = shape[i]!;
    const d = haversineMeters(p.shape_pt_lat, p.shape_pt_lon, lat, lon);
    if (d < distanceMeters) {
      distanceMeters = d;
      idx = i;
    }
  }
  return { idx, distanceMeters };
}

export function findClosestShapeIdx(lat: number, lon: number, shape: Shape[]): number {
  return closestShapePoint(lat, lon, shape).idx;
}

export function buildStopShapeIdxByStopId(
  tripStops: StopTime[],
  shape: Shape[],
): Map<number, number> {
  const stopShapeIdxByStopId = new Map<number, number>();
  for (const st of tripStops) {
    if (!st.stop_lat || !st.stop_lon) continue;
    stopShapeIdxByStopId.set(st.stop_id, findClosestShapeIdx(st.stop_lat, st.stop_lon, shape));
  }
  return stopShapeIdxByStopId;
}

function sortedTripStops(tripStops: StopTime[]): StopTime[] {
  return [...tripStops].sort((a, b) => a.stop_sequence - b.stop_sequence);
}

export function stopShapePositions(tripStops: StopTime[], shape: Shape[]): number[] {
  let last = 0;
  return tripStops.map((st) => {
    const idx =
      st.stop_lat && st.stop_lon ? findClosestShapeIdx(st.stop_lat, st.stop_lon, shape) : last;
    last = Math.max(last, idx);
    return last;
  });
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

type EtaOptions = {
  tripStops?: StopTime[];
  targetStopId?: number;
  referenceTime?: Date | null;
};

async function fetchRawVehicles(tripId: string, prefetched?: Vehicle[]): Promise<Vehicle[]> {
  if (prefetched) return prefetched;
  return ((await apiRequest(`vehicles?trip_id=${tripId}`)) as Vehicle[]) ?? [];
}

export async function getIndexedVehicles(
  tripId: string,
  routeShortName: string,
  routeColor: string,
  index: ShapeIndex,
  userTime?: Date | null,
  prefetched?: Vehicle[],
): Promise<IndexedVehicle[]> {
  const { shape } = index;
  if (!shape.length) return [];

  const raw = await fetchRawVehicles(tripId, prefetched);
  const now = userTime?.getTime() ?? Date.now();
  const start = shape[0]!;
  const result: IndexedVehicle[] = [];

  for (const v of raw) {
    if (!isTracked(v, now)) continue;
    const { idx: shapeIdx, distanceMeters } = closestShapePoint(v.latitude, v.longitude, shape);
    result.push({
      ...v,
      route_short_name: routeShortName,
      route_color: routeColor,
      heading: computeHeading(v.latitude, v.longitude, shape, shapeIdx),
      shapeIdx,
      offRouteMeters: distanceMeters,
      atStartTerminus:
        haversineMeters(v.latitude, v.longitude, start.shape_pt_lat, start.shape_pt_lon) <=
        TERMINUS_RADIUS_METERS,
      dwellMs: dwellMs(v, now),
    });
  }

  return result;
}

export function etaForStop(
  stopShapeIdx: number,
  vehicles: IndexedVehicle[],
  index: ShapeIndex,
  options: EtaOptions = {},
): StopEta | null {
  if (stopShapeIdx < 0) return null;

  const now = options.referenceTime?.getTime() ?? Date.now();

  // Two ways a vehicle carries this trip without being on it yet: parked at the
  // departure terminus, or still driving in from somewhere else entirely. Neither
  // says anything about when it leaves - that is the timetable's job - and both
  // snap to shape index 0, which would make them the candidate for every stop.
  const candidate = vehicles
    .filter((v) => v.shapeIdx <= stopShapeIdx && canDriveLiveEta(v, now))
    .sort((a, b) => b.shapeIdx - a.shapeIdx)[0];
  if (!candidate) return null;

  const seconds = vehicleEtaSeconds(candidate, stopShapeIdx, index, options);
  return {
    vehicle: candidate,
    etaMinutes: Math.max(0, Math.round(seconds / 60)),
    etaSeconds: seconds,
  };
}

export function isOnRoute(v: IndexedVehicle): boolean {
  return v.shapeIdx >= 0 && v.offRouteMeters <= MAX_OFF_ROUTE_METERS;
}

export function canDriveLiveEta(v: IndexedVehicle, now: number): boolean {
  return isOnRoute(v) && !v.atStartTerminus && isFreshForLiveEta(v, now);
}

// Deliberately *not* aged against the wall clock. Between position fixes we have
// no evidence the bus moved, and letting the estimate tick down anyway drained it
// to "now" while the bus was still hundreds of metres out. The number holds until
// the vehicle reports again, then it is recomputed from where it actually is.
export function vehicleEtaSeconds(
  vehicle: IndexedVehicle,
  stopShapeIdx: number,
  index: ShapeIndex,
  options: EtaOptions = {},
): number {
  const estimated = etaSecondsForStop(vehicle, stopShapeIdx, index, options);
  const remainingMeters = distanceOnShape(index, vehicle.shapeIdx, stopShapeIdx);
  return Math.max(estimated, remainingMeters / MAX_PLAUSIBLE_SPEED_MPS);
}

function etaSecondsForStop(
  vehicle: IndexedVehicle,
  stopShapeIdx: number,
  index: ShapeIndex,
  options: EtaOptions,
): number {
  return (
    profileAwareEtaSeconds(vehicle, stopShapeIdx, index, options) ??
    estimateEtaSeconds(distanceOnShape(index, vehicle.shapeIdx, stopShapeIdx), vehicle.speed)
  );
}

// Blends what this route historically takes against what this bus is doing right now.
//
// Live speed used to be blended in and made things worse: against 23k predictions
// replayed from production it ran 45s pessimistic on average, and over 2 min
// pessimistic whenever a bus reported standing at a light. A momentary speed says
// nothing about the rest of the run, while the learned segment times already carry the
// lights and the traffic for this hour of this day.
function profileAwareEtaSeconds(
  vehicle: IndexedVehicle,
  stopShapeIdx: number,
  index: ShapeIndex,
  options: EtaOptions,
): number | null {
  const stops = options.tripStops?.length ? sortedTripStops(options.tripStops) : [];
  if (!stops.length || options.targetStopId === undefined) return null;

  const targetPos = stops.findIndex((st) => st.stop_id === options.targetStopId);
  if (targetPos <= 0) return null;

  const positions = stopShapePositions(stops, index.shape);

  let prevPos = -1;
  for (let i = 0; i < targetPos; i++) {
    if (positions[i]! <= vehicle.shapeIdx) prevPos = i;
    else break;
  }
  if (prevPos < 0) return null;

  const nextPos = prevPos + 1;
  if (nextPos > targetPos) return null;

  const segmentStartIdx = positions[prevPos]!;
  const segmentEndIdx = positions[nextPos]!;
  const segmentMeters = distanceOnShape(index, segmentStartIdx, segmentEndIdx);
  const remainingInSegment = distanceOnShape(
    index,
    Math.max(vehicle.shapeIdx, segmentStartIdx),
    segmentEndIdx,
  );
  // Share of the current segment still to run: 1 on entering it, 0 on reaching its
  // far stop, which is what lets the profile total hand over without a step.
  const share = segmentMeters > 0 ? clamp(remainingInSegment / segmentMeters, 0, 1) : 0;

  let profileSeconds = 0;
  for (let pos = nextPos; pos <= targetPos; pos++) {
    profileSeconds += stops[pos]!.offset_arrival_time * (pos === nextPos ? share : 1);
  }
  if (profileSeconds <= 0) return null;

  const now = options.referenceTime?.getTime() ?? Date.now();
  const fixAgeSec = clamp(
    (now - new Date(vehicle.timestamp).getTime()) / 1000,
    0,
    MAX_FIX_AGE_CREDIT_SEC,
  );
  return Math.max(0, profileSeconds - fixAgeSec);
}

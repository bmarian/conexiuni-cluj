import { apiRequest } from "@/utils/api.ts";
import { calculateBearing, haversineMeters } from "@/utils/geo.ts";
import { minutesUntil } from "@/utils/arrivals.ts";
import type { Shape, StopTime, Turnaround, Vehicle } from "@/types/tranzy.ts";

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
// A bus arriving early at a through terminus waits for its slot; past this it is read
// as running late for an earlier one instead.
const MAX_EARLY_WAIT_MIN = 8;
// A run is paired with a timetable departure only when it sits clearly on one.
const MAX_RUN_MATCH_MIN = 8;
const RUN_MATCH_HEADWAY_SHARE = 0.4;
const PAIRED_SLOT_MARGIN_MIN = 3;
// Past this a paired slot is a timetable the bus is not keeping, not a wait.
const MAX_PAIRED_WAIT_MIN = 10;
const OVERSTAY_MS = 60_000;
const NEXT_SLOT_MAX_MIN = 20;
const STANDING_MAX_MS = 25 * 60_000;
// Further out than this, a bus still on the other direction predicts no better than the
// timetable it would replace, and it hides the runs between.
const FEEDER_HORIZON_MS = 12 * 60_000;

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
  // Within the radius of the departure terminus: drawn on the map, but not the source
  // of a live ETA until it is past the trip's first stop - see etaForStop.
  atStartTerminus: boolean;
  dwellMs: number;
  // When Tranzy moved it onto this trip, which at a terminus is when it arrived there.
  tripChangedAtMs: number | null;
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

function parseMs(text: string | undefined): number | null {
  if (!text) return null;
  const ms = new Date(text).getTime();
  return isNaN(ms) ? null : ms;
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
      tripChangedAtMs: parseMs(v.trip_changed_at),
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
  const firstStopIdx = firstStopShapeIdx(index, options.tripStops);
  const candidate = vehicles
    .filter((v) => v.shapeIdx <= stopShapeIdx && canDriveLiveEta(v, now, firstStopIdx))
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

// Shape index of the first stop after the departure terminus. Infinity without stops,
// which keeps every bus inside the terminus radius on the timetable.
function firstStopShapeIdx(index: ShapeIndex, tripStops?: StopTime[]): number {
  const first = tripStops?.length ? sortedTripStops(tripStops)[1] : undefined;
  if (!first?.stop_lat || !first.stop_lon) return Infinity;
  return findClosestShapeIdx(first.stop_lat, first.stop_lon, index.shape);
}

// The terminus radius alone cannot tell a bus pulling out from one standing in the
// bay, and a bus that left a few minutes late was held to the next departure until it
// cleared the radius. Only a bus that has left is past the trip's first stop: parked
// buses were seen up to 240 m along the shape, always short of it.
export function canDriveLiveEta(v: IndexedVehicle, now: number, firstStopIdx = Infinity): boolean {
  const waitingAtTerminus = v.atStartTerminus && v.shapeIdx <= firstStopIdx;
  return isOnRoute(v) && !waitingAtTerminus && isFreshForLiveEta(v, now);
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

// ---- Through running: the bus that ends the other direction continues as this trip.

export type FeederTrip = { vehicles: IndexedVehicle[]; index: ShapeIndex; tripStops: StopTime[] };

export type ThroughRunOptions = {
  turnaround: Turnaround;
  // This trip's departures today, in minutes past midnight, and the clock the same way.
  departureMinutes: number[];
  nowMinutes: number;
  // The feeder direction's departures today, so a bus can be tied to the run it is on.
  feederDepartureMinutes?: number[];
  feeder?: FeederTrip;
};

export type LiveEtaOptions = EtaOptions & { throughRun?: ThroughRunOptions };

export function turnaroundOf(tripStops: StopTime[]): Turnaround | null {
  const first = sortedTripStops(tripStops)[0];
  if (!first?.turnaround_sec) return null;
  return { through: !!first.through_running, sec: first.turnaround_sec, leaves: !!first.turnaround_leaves };
}

export function otherDirectionTripId(tripId: string): string {
  return tripId.endsWith("_1") ? tripId.slice(0, -2) + "_0" : tripId.slice(0, -2) + "_1";
}

// The first stop's own offset is the schedule delay, which a bus already here has paid.
function rideSecondsFromStart(sortedStops: StopTime[], targetPos: number): number {
  let total = 0;
  for (let pos = 1; pos <= targetPos; pos++) total += sortedStops[pos]!.offset_arrival_time;
  return total;
}

function cameFromOtherDirection(v: IndexedVehicle): boolean {
  const prev = v.prev_trip_id;
  return !!prev && prev !== v.trip_id && prev.split("_")[0] === v.trip_id.split("_")[0];
}

function standingAtStart(v: IndexedVehicle, now: number, firstStopIdx: number): boolean {
  return (
    v.atStartTerminus &&
    v.shapeIdx <= firstStopIdx &&
    isOnRoute(v) &&
    isFreshForLiveEta(v, now) &&
    v.tripChangedAtMs !== null &&
    now - v.tripChangedAtMs <= STANDING_MAX_MS &&
    cameFromOtherDirection(v)
  );
}

// The timetable departure a run belongs to, unless it sits too far between two.
export function matchRunDeparture(departureMinutes: number[], impliedMin: number): number | null {
  let best: number | null = null;
  let bestDiff = Infinity;
  const sorted = [...departureMinutes].sort((a, b) => a - b);
  sorted.forEach((d, i) => {
    const diff = Math.abs(minutesUntil(d, impliedMin));
    if (diff < bestDiff) {
      bestDiff = diff;
      best = i;
    }
  });
  if (best === null) return null;
  let gap = Infinity;
  if (best > 0) gap = sorted[best]! - sorted[best - 1]!;
  if (best + 1 < sorted.length) gap = Math.min(gap, sorted[best + 1]! - sorted[best]!);
  return bestDiff <= Math.min(MAX_RUN_MATCH_MIN, RUN_MATCH_HEADWAY_SHARE * gap) ? sorted[best]! : null;
}

// The first slot after the feeder run's scheduled arrival: in the past for a late bus.
function pairedSlotMinutes(options: ThroughRunOptions, arrivalMin: number, feederRideMin: number): number | null {
  if (!options.feederDepartureMinutes?.length) return null;
  const matched = matchRunDeparture(options.feederDepartureMinutes, arrivalMin - feederRideMin);
  if (matched === null) return null;
  const scheduledArrival = matched + feederRideMin;
  let slot: number | null = null;
  for (const d of options.departureMinutes) {
    const diff = minutesUntil(d, scheduledArrival);
    if (diff >= -PAIRED_SLOT_MARGIN_MIN && (slot === null || diff < slot)) slot = diff;
  }
  return slot === null ? null : scheduledArrival + slot;
}

// When a bus arriving at `arrivalMs` leaves: after the stand, or at its slot when that
// comes later. A standing bus that has outstayed that takes the next slot.
export function projectDeparture(
  arrivalMs: number,
  options: ThroughRunOptions,
  nowMs: number,
  standing: boolean,
  feederRideMin: number | null = null,
): number | null {
  const ready = arrivalMs + options.turnaround.sec * 1000;
  const readyMin = options.nowMinutes + (ready - nowMs) / 60_000;
  const arrivalMin = options.nowMinutes + (arrivalMs - nowMs) / 60_000;
  let departure = ready;
  if (!options.turnaround.leaves) {
    const paired = feederRideMin === null ? null : pairedSlotMinutes(options, arrivalMin, feederRideMin);
    const pairedWait = paired === null ? null : minutesUntil(paired, readyMin);
    if (pairedWait !== null && pairedWait <= MAX_PAIRED_WAIT_MIN) {
      if (pairedWait > 0) departure = ready + pairedWait * 60_000;
    } else {
      let nearest: number | null = null;
      for (const d of options.departureMinutes) {
        const diff = minutesUntil(d, readyMin);
        if (nearest === null || Math.abs(diff) < Math.abs(nearest)) nearest = diff;
      }
      if (nearest !== null && nearest >= 0 && nearest <= MAX_EARLY_WAIT_MIN) departure = ready + nearest * 60_000;
    }
  }
  if (standing && nowMs > departure + OVERSTAY_MS) {
    let next: number | null = null;
    for (const d of options.departureMinutes) {
      const diff = minutesUntil(d, options.nowMinutes);
      if (diff >= 0 && diff <= NEXT_SLOT_MAX_MIN && (next === null || diff < next)) next = diff;
    }
    if (next === null) return null;
    departure = nowMs + next * 60_000;
  }
  return departure;
}

function toEta(vehicle: IndexedVehicle, seconds: number): StopEta {
  return { vehicle, etaMinutes: Math.max(0, Math.round(seconds / 60)), etaSeconds: seconds };
}

// Every live estimate for one stop, soonest first, including the buses still to arrive
// from the other direction when the trip runs through.
export function liveEtasForStop(
  stopShapeIdx: number,
  vehicles: IndexedVehicle[],
  index: ShapeIndex,
  options: LiveEtaOptions = {},
): StopEta[] {
  if (stopShapeIdx < 0) return [];
  const now = options.referenceTime?.getTime() ?? Date.now();
  const firstStopIdx = firstStopShapeIdx(index, options.tripStops);
  const etas: StopEta[] = [];
  for (const v of vehicles) {
    if (v.shapeIdx <= stopShapeIdx && canDriveLiveEta(v, now, firstStopIdx)) {
      etas.push(toEta(v, vehicleEtaSeconds(v, stopShapeIdx, index, options)));
    }
  }

  const through = options.throughRun;
  if (through?.turnaround.through && options.tripStops?.length && options.targetStopId !== undefined) {
    const stops = sortedTripStops(options.tripStops);
    const targetPos = stops.findIndex((st) => st.stop_id === options.targetStopId);
    if (targetPos >= 0) {
      const ride = rideSecondsFromStart(stops, targetPos);
      const feeder = through.feeder;
      const feederStops = feeder ? sortedTripStops(feeder.tripStops) : [];
      const feederRideMin = feederStops.length ? rideSecondsFromStart(feederStops, feederStops.length - 1) / 60 : null;
      for (const v of vehicles) {
        if (!standingAtStart(v, now, firstStopIdx)) continue;
        const departure = projectDeparture(v.tripChangedAtMs!, through, now, true, feederRideMin);
        if (departure !== null) etas.push(toEta(v, Math.max(0, (departure - now) / 1000) + ride));
      }
      const last = feederStops[feederStops.length - 1];
      if (feeder && last?.stop_lat && last.stop_lon) {
        const lastIdx = findClosestShapeIdx(last.stop_lat, last.stop_lon, feeder.index.shape);
        const feederFirstIdx = firstStopShapeIdx(feeder.index, feeder.tripStops);
        for (const v of feeder.vehicles) {
          if (v.shapeIdx > lastIdx || !canDriveLiveEta(v, now, feederFirstIdx)) continue;
          const toEnd = vehicleEtaSeconds(v, lastIdx, feeder.index, {
            tripStops: feeder.tripStops,
            targetStopId: last.stop_id,
            referenceTime: options.referenceTime,
          });
          const departure = projectDeparture(now + toEnd * 1000, through, now, false, feederRideMin);
          if (departure !== null && departure - now <= FEEDER_HORIZON_MS) etas.push(toEta(v, Math.max(0, (departure - now) / 1000) + ride));
        }
      }
    }
  }
  return etas.sort((a, b) => a.etaSeconds - b.etaSeconds);
}

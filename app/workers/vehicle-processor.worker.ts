/**
 * Web Worker for computing vehicle-to-stop proximity.
 * Types are inlined to avoid TS path alias issues in worker context.
 */

interface VehicleData {
  id: string;
  label: string;
  latitude?: number;
  longitude?: number;
  speed?: number;
  vehicle_type: number;
  direction_id?: number;
}

interface StopData {
  stop_id: number;
  stop_name: string;
  stop_lat: number;
  stop_lon: number;
}

export interface WorkerInput {
  vehicles: VehicleData[];
  outboundStops: StopData[];
  inboundStops: StopData[];
}

export interface WorkerOutput {
  stopVehicleMap: {
    outbound: Record<number, VehicleData[]>;
    inbound: Record<number, VehicleData[]>;
  };
  directionVehicles: {
    outbound: VehicleData[];
    inbound: VehicleData[];
  };
}

/** Max distance (meters) to associate a vehicle with a stop on the linear map. */
const PROXIMITY_THRESHOLD = 300;
const DEPOT_DISTANCE_THRESHOLD = 200; // meters — vehicles within this distance of the first stop with speed 0 are likely at the depot

function isAtDepot(v: VehicleData, stops: StopData[]): boolean {
  if (stops.length === 0 || v.latitude == null || v.longitude == null) return false;
  if (v.speed && v.speed > 0) return false;
  return haversine(v.latitude, v.longitude, stops[0].stop_lat, stops[0].stop_lon) < DEPOT_DISTANCE_THRESHOLD;
}

function haversine(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const R = 6_371_000;
  const toRad = Math.PI / 180;
  const dLat = (lat2 - lat1) * toRad;
  const dLon = (lon2 - lon1) * toRad;
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(lat1 * toRad) * Math.cos(lat2 * toRad) * Math.sin(dLon / 2) ** 2;
  return R * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}

function nearestStop(
  lat: number,
  lon: number,
  stops: StopData[],
): {stop: StopData; dist: number} | null {
  if (stops.length === 0) return null;

  let best = stops[0];
  let bestDist = Infinity;

  for (const s of stops) {
    const d = haversine(lat, lon, s.stop_lat, s.stop_lon);
    if (d < bestDist) {
      bestDist = d;
      best = s;
    }
  }
  return {stop: best, dist: bestDist};
}

function process(input: WorkerInput): WorkerOutput {
  const outboundMap: Record<number, VehicleData[]> = {};
  const inboundMap: Record<number, VehicleData[]> = {};
  const outboundVehicles: VehicleData[] = [];
  const inboundVehicles: VehicleData[] = [];

  for (const v of input.vehicles) {
    if (v.latitude == null || v.longitude == null) continue;

    if (v.direction_id === 0) {
      if (isAtDepot(v, input.outboundStops)) continue;
      outboundVehicles.push(v);
      const result = nearestStop(v.latitude, v.longitude, input.outboundStops);
      if (result && result.dist < PROXIMITY_THRESHOLD) {
        (outboundMap[result.stop.stop_id] ??= []).push(v);
      }
    } else if (v.direction_id === 1) {
      if (isAtDepot(v, input.inboundStops)) continue;
      inboundVehicles.push(v);
      const result = nearestStop(v.latitude, v.longitude, input.inboundStops);
      if (result && result.dist < PROXIMITY_THRESHOLD) {
        (inboundMap[result.stop.stop_id] ??= []).push(v);
      }
    } else {
      // Unknown direction — filter if at depot in either direction
      if (isAtDepot(v, input.outboundStops) || isAtDepot(v, input.inboundStops)) continue;
      outboundVehicles.push(v);
      inboundVehicles.push(v);
      const outResult = nearestStop(v.latitude, v.longitude, input.outboundStops);
      const inResult = nearestStop(v.latitude, v.longitude, input.inboundStops);
      const best =
        outResult && inResult
          ? outResult.dist <= inResult.dist
            ? {map: outboundMap, r: outResult}
            : {map: inboundMap, r: inResult}
          : outResult
            ? {map: outboundMap, r: outResult}
            : inResult
              ? {map: inboundMap, r: inResult}
              : null;
      if (best && best.r.dist < PROXIMITY_THRESHOLD) {
        (best.map[best.r.stop.stop_id] ??= []).push(v);
      }
    }
  }

  return {
    stopVehicleMap: {outbound: outboundMap, inbound: inboundMap},
    directionVehicles: {outbound: outboundVehicles, inbound: inboundVehicles},
  };
}

// Cache within the worker to avoid recomputing identical inputs
let lastInputHash = "";
let lastOutput: WorkerOutput | null = null;
let lastTimestamp = 0;
const WORKER_CACHE_TTL = 60_000;

function hashInput(input: WorkerInput): string {
  const parts: string[] = [];
  for (const v of input.vehicles) {
    parts.push(`${v.id}:${v.latitude}:${v.longitude}:${v.direction_id}`);
  }
  return parts.join("|");
}

self.onmessage = (e: MessageEvent<WorkerInput>) => {
  const hash = hashInput(e.data);
  const now = Date.now();

  if (hash === lastInputHash && lastOutput && now - lastTimestamp < WORKER_CACHE_TTL) {
    self.postMessage(lastOutput);
    return;
  }

  const result = process(e.data);
  lastInputHash = hash;
  lastOutput = result;
  lastTimestamp = now;
  self.postMessage(result);
};

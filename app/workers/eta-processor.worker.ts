/**
 * Web Worker for computing estimated arrival times of vehicles at a stop.
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
  stop_sequence: number;
}

interface RouteData {
  route_id: number;
  route_short_name: string;
  route_color: string;
  route_type: number;
  stops: {outbound: StopData[]; inbound: StopData[]};
  vehicles: VehicleData[];
}

export interface ETAWorkerInput {
  stopName: string;
  routes: RouteData[];
}

export interface VehicleETA {
  vehicle_id: string;
  vehicle_label: string;
  eta_minutes: number;
  direction: "outbound" | "inbound";
  stops_away: number;
  destination: string; // last stop name in this direction
}

export interface RouteArrival {
  route_id: number;
  route_short_name: string;
  route_color: string;
  route_type: number;
  vehicles: VehicleETA[];
}

export interface ETAWorkerOutput {
  arrivals: RouteArrival[];
}

// Default speeds in km/h when vehicle speed is 0 or unavailable
const DEFAULT_SPEED_BUS = 20;
const DEFAULT_SPEED_TRAM = 18;
const MAX_ETA_MINUTES = 30;
const MAX_VEHICLES_PER_DIRECTION = 3;
const DEPOT_DISTANCE_THRESHOLD = 200; // meters — vehicles within this distance of the first stop with speed 0 are likely at the depot

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

function nearestStopIndex(lat: number, lon: number, stops: StopData[]): number {
  let bestIdx = 0;
  let bestDist = Infinity;
  for (let i = 0; i < stops.length; i++) {
    const d = haversine(lat, lon, stops[i].stop_lat, stops[i].stop_lon);
    if (d < bestDist) {
      bestDist = d;
      bestIdx = i;
    }
  }
  return bestIdx;
}

function sumDistanceBetweenStops(stops: StopData[], fromIdx: number, toIdx: number): number {
  let total = 0;
  for (let i = fromIdx; i < toIdx; i++) {
    total += haversine(
      stops[i].stop_lat, stops[i].stop_lon,
      stops[i + 1].stop_lat, stops[i + 1].stop_lon,
    );
  }
  return total;
}

function getDefaultSpeed(routeType: number): number {
  return routeType === 0 ? DEFAULT_SPEED_TRAM : DEFAULT_SPEED_BUS;
}

function computeETAsForDirection(
  vehicles: VehicleData[],
  stops: StopData[],
  targetStopName: string,
  routeType: number,
  direction: "outbound" | "inbound",
): VehicleETA[] {
  if (stops.length === 0) return [];

  const targetIdx = stops.findIndex((s) => s.stop_name === targetStopName);
  if (targetIdx === -1) return [];

  const destination = stops[stops.length - 1].stop_name;
  const etas: VehicleETA[] = [];

  for (const v of vehicles) {
    if (v.latitude == null || v.longitude == null) continue;

    // Filter out vehicles likely parked at the depot: near the first stop with speed 0
    if ((!v.speed || v.speed === 0) && stops.length > 0) {
      const distToFirst = haversine(v.latitude, v.longitude, stops[0].stop_lat, stops[0].stop_lon);
      if (distToFirst < DEPOT_DISTANCE_THRESHOLD) continue;
    }

    const vehicleIdx = nearestStopIndex(v.latitude, v.longitude, stops);
    if (vehicleIdx >= targetIdx) continue;

    const distance = sumDistanceBetweenStops(stops, vehicleIdx, targetIdx);
    const speedKmh = v.speed && v.speed > 0 ? v.speed : getDefaultSpeed(routeType);
    const etaSeconds = (distance / 1000) / speedKmh * 3600;
    const etaMinutes = Math.round(etaSeconds / 60);

    if (etaMinutes > MAX_ETA_MINUTES) continue;

    etas.push({
      vehicle_id: v.id,
      vehicle_label: v.label,
      eta_minutes: Math.max(1, etaMinutes),
      direction,
      stops_away: targetIdx - vehicleIdx,
      destination,
    });
  }

  etas.sort((a, b) => a.eta_minutes - b.eta_minutes);
  return etas.slice(0, MAX_VEHICLES_PER_DIRECTION);
}

function process(input: ETAWorkerInput): ETAWorkerOutput {
  const arrivals: RouteArrival[] = [];

  for (const route of input.routes) {
    const outboundVehicles = route.vehicles.filter((v) => v.direction_id === 0);
    const inboundVehicles = route.vehicles.filter((v) => v.direction_id === 1);
    const unknownVehicles = route.vehicles.filter((v) => v.direction_id == null);

    const outETAs = computeETAsForDirection(
      [...outboundVehicles, ...unknownVehicles],
      route.stops.outbound,
      input.stopName,
      route.route_type,
      "outbound",
    );

    const inETAs = computeETAsForDirection(
      [...inboundVehicles, ...unknownVehicles],
      route.stops.inbound,
      input.stopName,
      route.route_type,
      "inbound",
    );

    // Deduplicate: if a vehicle appears in both directions (unknown direction),
    // keep only the one with the shorter ETA
    const seen = new Set<string>();
    const deduped: VehicleETA[] = [];
    const all = [...outETAs, ...inETAs].sort((a, b) => a.eta_minutes - b.eta_minutes);
    for (const eta of all) {
      if (!seen.has(eta.vehicle_id)) {
        seen.add(eta.vehicle_id);
        deduped.push(eta);
      }
    }

    if (deduped.length > 0) {
      arrivals.push({
        route_id: route.route_id,
        route_short_name: route.route_short_name,
        route_color: route.route_color,
        route_type: route.route_type,
        vehicles: deduped,
      });
    }
  }

  return {arrivals};
}

// Cache within the worker to avoid recomputing identical inputs
let lastInputHash = "";
let lastOutput: ETAWorkerOutput | null = null;
let lastTimestamp = 0;
const WORKER_CACHE_TTL = 60_000;

function hashInput(input: ETAWorkerInput): string {
  const parts: string[] = [input.stopName];
  for (const r of input.routes) {
    for (const v of r.vehicles) {
      parts.push(`${v.id}:${v.latitude}:${v.longitude}:${v.speed}`);
    }
  }
  return parts.join("|");
}

self.onmessage = (e: MessageEvent<ETAWorkerInput>) => {
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

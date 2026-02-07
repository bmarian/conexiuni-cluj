"use client";

import type {RouteStopInfo} from "@/lib/cluj-api";
import type {RouteType} from "@/types/tranzy";
import Link from "next/link";
import Image from "next/image";
import {useMemo} from "react";
import {useRouteVehicles} from "@/app/hooks/useRouteVehicles";
import {useUserLocation} from "@/app/hooks/useUserLocation";
import {getVehicleIconPath} from "@/app/utils/route-utils";

interface VehicleAtStop {
  id: string;
  label: string;
  speed?: number;
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

const DEFAULT_SPEED_KMH = 20;
const MAX_NEARBY_DISTANCE = 800; // metres — max distance to consider a stop "closest"

interface StopETA {
  stopId: number;
  etaMinutes: number;
}

/** Find the closest stop to the user and compute ETAs of vehicles approaching it. */
function computeClosestStopETAs(
  userLat: number,
  userLon: number,
  stops: RouteStopInfo[],
  vehiclesByStop: Record<number, VehicleAtStop[]>,
  allVehicles: {latitude?: number; longitude?: number; speed?: number}[],
): StopETA | null {
  if (stops.length === 0) return null;

  // Find the stop closest to the user
  let bestIdx = 0;
  let bestDist = Infinity;
  for (let i = 0; i < stops.length; i++) {
    const d = haversine(userLat, userLon, stops[i].stop_lat, stops[i].stop_lon);
    if (d < bestDist) {
      bestDist = d;
      bestIdx = i;
    }
  }

  if (bestDist > MAX_NEARBY_DISTANCE) return null;

  const targetStop = stops[bestIdx];

  // Find the nearest vehicle that is BEFORE this stop and compute ETA
  let bestEta = Infinity;

  for (const v of allVehicles) {
    if (v.latitude == null || v.longitude == null) continue;

    // Find which stop the vehicle is nearest to
    let vIdx = 0;
    let vBestDist = Infinity;
    for (let i = 0; i < stops.length; i++) {
      const d = haversine(v.latitude, v.longitude, stops[i].stop_lat, stops[i].stop_lon);
      if (d < vBestDist) {
        vBestDist = d;
        vIdx = i;
      }
    }

    // Only vehicles before the target stop
    if (vIdx >= bestIdx) continue;

    // Sum route distance from vehicle stop to target stop
    let dist = 0;
    for (let i = vIdx; i < bestIdx; i++) {
      dist += haversine(stops[i].stop_lat, stops[i].stop_lon, stops[i + 1].stop_lat, stops[i + 1].stop_lon);
    }

    const speedKmh = v.speed && v.speed > 0 ? v.speed : DEFAULT_SPEED_KMH;
    const etaMin = Math.round(((dist / 1000) / speedKmh) * 60);

    if (etaMin < bestEta && etaMin <= 30) {
      bestEta = etaMin;
    }
  }

  return {
    stopId: targetStop.stop_id,
    etaMinutes: bestEta === Infinity ? -1 : Math.max(1, bestEta),
  };
}

function StopLine({stops, color, label, vehiclesByStop, routeType, closestStop}: {
  stops: RouteStopInfo[];
  color: string;
  label: string;
  vehiclesByStop: Record<number, VehicleAtStop[]>;
  routeType?: RouteType;
  closestStop: StopETA | null;
}) {
  return (
    <div>
      <p className="mb-2 px-1 text-xs font-medium text-zinc-500 dark:text-zinc-400">{label}</p>
      <div className="flex flex-col">
        {stops.map((stop, i) => {
          const isFirst = i === 0;
          const isLast = i === stops.length - 1;
          const isTerminal = isFirst || isLast;
          const vehicles = vehiclesByStop[stop.stop_id] ?? [];
          const isClosest = closestStop?.stopId === stop.stop_id;
          const etaMinutes = isClosest ? closestStop.etaMinutes : -1;

          return (
            <div key={`${stop.stop_id}-${i}`} className={`flex items-stretch ${isClosest ? "rounded-md bg-purple-50 dark:bg-purple-950/30" : ""}`}>
              {/* Left column: line + dot */}
              <div className="relative flex w-8 shrink-0 flex-col items-center">
                {!isFirst && (
                  <div className="w-0.5 grow" style={{backgroundColor: color}} />
                )}
                {isFirst && <div className="grow" />}

                {vehicles.length > 0 ? (
                  <div className="relative z-10 shrink-0" title={vehicles.map(v => v.label).join(", ")}>
                    <Image
                      src={getVehicleIconPath(routeType)}
                      alt="Vehicul"
                      width={20}
                      height={20}
                      className="drop-shadow"
                    />
                  </div>
                ) : isClosest ? (
                  <div
                    className="relative z-10 h-3.5 w-3.5 shrink-0 rounded-full border-2 ring-2 ring-purple-400/50"
                    style={{
                      borderColor: "#a855f7",
                      backgroundColor: "#a855f7",
                    }}
                  />
                ) : (
                  <div
                    className={`relative z-10 shrink-0 rounded-full border-2 ${
                      isTerminal ? "h-3.5 w-3.5" : "h-2.5 w-2.5"
                    }`}
                    style={{
                      borderColor: color,
                      backgroundColor: isTerminal ? color : "var(--background)",
                    }}
                  />
                )}

                {!isLast && (
                  <div className="w-0.5 grow" style={{backgroundColor: color}} />
                )}
                {isLast && <div className="grow" />}
              </div>

              {/* Right column: stop name + vehicle count + ETA */}
              <Link
                href={`/statii/${encodeURIComponent(stop.stop_name)}`}
                className={`flex min-h-8 flex-1 items-center gap-1.5 py-1 pl-2 transition-colors hover:text-purple-600 dark:hover:text-purple-400 ${
                  isClosest
                    ? "text-sm font-semibold text-purple-700 dark:text-purple-300"
                    : isTerminal
                      ? "text-sm font-semibold text-zinc-900 dark:text-white"
                      : "text-sm text-zinc-600 dark:text-zinc-400"
                }`}
              >
                <span className="flex items-center gap-1">
                  {isClosest && (
                    <svg className="h-3 w-3 shrink-0 text-purple-500" viewBox="0 0 24 24" fill="currentColor">
                      <path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
                    </svg>
                  )}
                  {stop.stop_name}
                </span>
                {vehicles.length > 1 && (
                  <span className="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-zinc-200 px-1 text-[10px] font-bold text-zinc-600 dark:bg-zinc-700 dark:text-zinc-300">
                    ×{vehicles.length}
                  </span>
                )}
                {isClosest && etaMinutes > 0 && (
                  <span className="ml-auto shrink-0 rounded-full bg-purple-100 px-2 py-0.5 text-[11px] font-bold text-purple-700 dark:bg-purple-900/50 dark:text-purple-300">
                    ~{etaMinutes} min
                  </span>
                )}
              </Link>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default function LinearRouteMap({outbound, inbound, color, routeId, routeType}: {
  outbound: RouteStopInfo[];
  inbound: RouteStopInfo[];
  color: string;
  routeShortName?: string;
  routeId?: number;
  routeType?: RouteType;
}) {
  const vehicleData = useRouteVehicles(routeId, outbound, inbound);
  const userLocation = useUserLocation();

  const outVehicles = vehicleData?.stopVehicleMap.outbound ?? {};
  const inVehicles = vehicleData?.stopVehicleMap.inbound ?? {};

  // Compute closest stop + ETA for each direction
  const outClosest = useMemo(() => {
    if (!userLocation || !vehicleData) return null;
    return computeClosestStopETAs(
      userLocation.latitude, userLocation.longitude,
      outbound, outVehicles,
      vehicleData.directionVehicles.outbound,
    );
  }, [userLocation, vehicleData, outbound, outVehicles]);

  const inClosest = useMemo(() => {
    if (!userLocation || !vehicleData) return null;
    return computeClosestStopETAs(
      userLocation.latitude, userLocation.longitude,
      inbound, inVehicles,
      vehicleData.directionVehicles.inbound,
    );
  }, [userLocation, vehicleData, inbound, inVehicles]);

  if (outbound.length === 0 && inbound.length === 0) return null;

  const firstDir = outbound.length > 0 ? outbound : null;
  const secondDir = inbound.length > 0 ? inbound : null;

  const outLabel = firstDir
    ? `${firstDir[0].stop_name} → ${firstDir[firstDir.length - 1].stop_name}`
    : "";
  const inLabel = secondDir
    ? `${secondDir[0].stop_name} → ${secondDir[secondDir.length - 1].stop_name}`
    : "";

  return (
    <div className="animate-fade-slide-up mt-6">
      <h2 className="mb-3 text-sm font-semibold text-zinc-900 dark:text-white">
        Hartă liniară
      </h2>

      <div className="grid gap-4 md:grid-cols-2">
        {firstDir && (
          <div className="rounded-lg border border-zinc-200 bg-white px-3 py-3 dark:border-zinc-700 dark:bg-zinc-900">
            <StopLine stops={firstDir} color={color} label={outLabel} vehiclesByStop={outVehicles} routeType={routeType} closestStop={outClosest} />
          </div>
        )}

        {secondDir && (
          <div className="rounded-lg border border-zinc-200 bg-white px-3 py-3 dark:border-zinc-700 dark:bg-zinc-900">
            <StopLine stops={secondDir} color={color} label={inLabel} vehiclesByStop={inVehicles} routeType={routeType} closestStop={inClosest} />
          </div>
        )}
      </div>
    </div>
  );
}

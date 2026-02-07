"use client";

import type {RouteStopInfo} from "@/lib/cluj-api";
import type {RouteType} from "@/types/tranzy";
import Link from "next/link";
import Image from "next/image";
import {useRouteVehicles} from "@/app/hooks/useRouteVehicles";
import {getVehicleIconPath} from "@/app/utils/route-utils";

interface VehicleAtStop {
  id: string;
  label: string;
}

function StopLine({stops, color, label, vehiclesByStop, routeType}: {
  stops: RouteStopInfo[];
  color: string;
  label: string;
  vehiclesByStop: Record<number, VehicleAtStop[]>;
  routeType?: RouteType;
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

          return (
            <div key={`${stop.stop_id}-${i}`} className="flex items-stretch">
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

              {/* Right column: stop name + vehicle count */}
              <Link
                href={`/statii/${encodeURIComponent(stop.stop_name)}`}
                className={`flex min-h-8 items-center gap-1.5 py-1 pl-2 transition-colors hover:text-purple-600 dark:hover:text-purple-400 ${
                  isTerminal
                    ? "text-sm font-semibold text-zinc-900 dark:text-white"
                    : "text-sm text-zinc-600 dark:text-zinc-400"
                }`}
              >
                {stop.stop_name}
                {vehicles.length > 1 && (
                  <span className="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-zinc-200 px-1 text-[10px] font-bold text-zinc-600 dark:bg-zinc-700 dark:text-zinc-300">
                    ×{vehicles.length}
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

  if (outbound.length === 0 && inbound.length === 0) return null;

  const firstDir = outbound.length > 0 ? outbound : null;
  const secondDir = inbound.length > 0 ? inbound : null;

  const outLabel = firstDir
    ? `${firstDir[0].stop_name} → ${firstDir[firstDir.length - 1].stop_name}`
    : "";
  const inLabel = secondDir
    ? `${secondDir[0].stop_name} → ${secondDir[secondDir.length - 1].stop_name}`
    : "";

  const outVehicles = vehicleData?.stopVehicleMap.outbound ?? {};
  const inVehicles = vehicleData?.stopVehicleMap.inbound ?? {};

  return (
    <div className="animate-fade-slide-up mt-6">
      <h2 className="mb-3 text-sm font-semibold text-zinc-900 dark:text-white">
        Hartă liniară
      </h2>

      <div className="grid gap-4 md:grid-cols-2">
        {firstDir && (
          <div className="rounded-lg border border-zinc-200 bg-white px-3 py-3 dark:border-zinc-700 dark:bg-zinc-900">
            <StopLine stops={firstDir} color={color} label={outLabel} vehiclesByStop={outVehicles} routeType={routeType} />
          </div>
        )}

        {secondDir && (
          <div className="rounded-lg border border-zinc-200 bg-white px-3 py-3 dark:border-zinc-700 dark:bg-zinc-900">
            <StopLine stops={secondDir} color={color} label={inLabel} vehiclesByStop={inVehicles} routeType={routeType} />
          </div>
        )}
      </div>
    </div>
  );
}

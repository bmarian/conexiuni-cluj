"use client";

import Link from "next/link";
import {useStopArrivals} from "@/app/hooks/useStopArrivals";
import {routeTypeLabel} from "@/app/utils/route-utils";
import type {VehicleETA} from "@/app/workers/eta-processor.worker";

function ETABadge({vehicle}: {vehicle: VehicleETA}) {
  const isImminent = vehicle.eta_minutes <= 2;

  return (
    <div className="flex items-center gap-1.5">
      <span
        className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold ${
          isImminent
            ? "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300"
            : "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
        }`}
      >
        {isImminent ? "" : "~"}{vehicle.eta_minutes} min
      </span>
      <span className="text-[11px] text-zinc-400 dark:text-zinc-500">
        {vehicle.stops_away} {vehicle.stops_away === 1 ? "stație" : "stații"}
      </span>
    </div>
  );
}

export default function StopArrivals({stopName}: {stopName: string}) {
  const {data, loading} = useStopArrivals(stopName);

  if (loading) {
    return (
      <div className="animate-fade-slide-up mt-6">
        <h2 className="mb-3 text-sm font-semibold text-zinc-900 dark:text-white">
          Sosiri estimate
        </h2>
        <div className="flex items-center gap-2 rounded-lg border border-zinc-200 bg-white px-4 py-6 dark:border-zinc-700 dark:bg-zinc-900">
          <svg className="h-4 w-4 animate-spin text-zinc-400" viewBox="0 0 24 24" fill="none">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
          <span className="text-sm text-zinc-400">Se calculează sosirile...</span>
        </div>
      </div>
    );
  }

  if (!data || data.arrivals.length === 0) {
    return (
      <div className="animate-fade-slide-up mt-6">
        <h2 className="mb-3 text-sm font-semibold text-zinc-900 dark:text-white">
          Sosiri estimate
        </h2>
        <div className="rounded-lg border border-zinc-200 bg-white px-4 py-6 text-center dark:border-zinc-700 dark:bg-zinc-900">
          <p className="text-sm text-zinc-400 dark:text-zinc-500">
            Nu sunt vehicule active în apropiere
          </p>
        </div>
      </div>
    );
  }

  // Group vehicles by direction within each route, sort by soonest arrival
  const groupedArrivals = data.arrivals
    .map((route) => {
      const outbound = route.vehicles.filter((v) => v.direction === "outbound");
      const inbound = route.vehicles.filter((v) => v.direction === "inbound");
      const soonest = Math.min(...route.vehicles.map((v) => v.eta_minutes));
      return {...route, outbound, inbound, soonest};
    })
    .sort((a, b) => a.soonest - b.soonest);

  return (
    <div className="animate-fade-slide-up mt-6">
      <h2 className="mb-3 text-sm font-semibold text-zinc-900 dark:text-white">
        Sosiri estimate
      </h2>

      <div className="space-y-2">
        {groupedArrivals.map((route, i) => (
          <Link
            key={route.route_id}
            href={`/linii/${encodeURIComponent(route.route_short_name)}`}
            className="animate-row flex items-start gap-3 rounded-lg border border-zinc-100 bg-white p-3 transition-colors hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-600"
            style={{animationDelay: `${Math.min(i * 30, 300)}ms`}}
          >
            <div
              className="flex h-10 min-w-10 shrink-0 items-center justify-center rounded-md px-2 text-xs font-bold text-white"
              style={{backgroundColor: route.route_color || "#7c3aed"}}
            >
              {route.route_short_name}
            </div>

            <div className="min-w-0 flex-1">
              <div className="text-xs font-medium text-zinc-500 dark:text-zinc-400">
                {routeTypeLabel(route.route_type)}
              </div>

              <div className="mt-1.5 space-y-2">
                {route.outbound.length > 0 && (
                  <div>
                    <div className="mb-1 text-[11px] text-zinc-400 dark:text-zinc-500">
                      spre {route.outbound[0].destination}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {route.outbound.map((v) => (
                        <ETABadge key={v.vehicle_id} vehicle={v} />
                      ))}
                    </div>
                  </div>
                )}

                {route.inbound.length > 0 && (
                  <div>
                    <div className="mb-1 text-[11px] text-zinc-400 dark:text-zinc-500">
                      spre {route.inbound[0].destination}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {route.inbound.map((v) => (
                        <ETABadge key={v.vehicle_id} vehicle={v} />
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}

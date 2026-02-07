"use client";

import Link from "next/link";
import {useStopArrivals} from "@/app/hooks/useStopArrivals";

function routeTypeLabel(type: number): string {
  switch (type) {
    case 0: return "Tramvai";
    case 3: return "Autobuz";
    case 11: return "Troleibuz";
    default: return "Linie";
  }
}

function DirectionArrow({direction}: {direction: "outbound" | "inbound"}) {
  return (
    <span className="text-[10px] text-zinc-400 dark:text-zinc-500">
      {direction === "outbound" ? "→" : "←"}
    </span>
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

  // Group vehicles by direction within each route
  const groupedArrivals = data.arrivals.map((route) => {
    const outbound = route.vehicles.filter((v) => v.direction === "outbound");
    const inbound = route.vehicles.filter((v) => v.direction === "inbound");
    return {...route, outbound, inbound};
  });

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

              <div className="mt-1 space-y-1">
                {route.outbound.length > 0 && (
                  <div className="flex flex-wrap items-center gap-1.5">
                    <DirectionArrow direction="outbound" />
                    {route.outbound.map((v, j) => (
                      <span
                        key={j}
                        className="inline-flex items-center rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400"
                      >
                        ~{v.eta_minutes} min
                      </span>
                    ))}
                  </div>
                )}

                {route.inbound.length > 0 && (
                  <div className="flex flex-wrap items-center gap-1.5">
                    <DirectionArrow direction="inbound" />
                    {route.inbound.map((v, j) => (
                      <span
                        key={j}
                        className="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                      >
                        ~{v.eta_minutes} min
                      </span>
                    ))}
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

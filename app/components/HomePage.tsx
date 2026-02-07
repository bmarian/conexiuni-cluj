"use client";

import {useEffect, useState} from "react";
import Link from "next/link";
import type {Stop} from "@/types/tranzy";
import {RouteType} from "@/types/tranzy";
import {getRecentLines, getRecentStops, RecentLine, RecentStop} from "@/lib/recent-history";

function routeTypeLabel(type: RouteType): string {
  switch (type) {
    case 0: return "Tramvai";
    case 3: return "Autobuz";
    case 11: return "Troleibuz";
    default: return "Linie";
  }
}

function haversineDistance(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const R = 6371e3; // Earth radius in meters
  const toRad = (deg: number) => (deg * Math.PI) / 180;
  const dLat = toRad(lat2 - lat1);
  const dLon = toRad(lon2 - lon1);
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) * Math.sin(dLon / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return R * c;
}

function formatDistance(meters: number): string {
  if (meters < 1000) return `${Math.round(meters)} m`;
  return `${(meters / 1000).toFixed(1)} km`;
}

interface NearbyStop {
  stop_name: string;
  distance: number;
}

function SectionTitle({children}: { children: React.ReactNode }) {
  return (
    <h2 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-white">
      {children}
    </h2>
  );
}

function NearbyStationsSection({stops}: { stops: Stop[] }) {
  const [nearby, setNearby] = useState<NearbyStop[] | null>(null);

  useEffect(() => {
    if (!("geolocation" in navigator)) return;

    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const {latitude, longitude} = pos.coords;

        // Calculate distances and deduplicate by name (keep closest)
        const byName = new Map<string, number>();
        for (const stop of stops) {
          const dist = haversineDistance(latitude, longitude, stop.stop_lat, stop.stop_lon);
          const existing = byName.get(stop.stop_name);
          if (existing === undefined || dist < existing) {
            byName.set(stop.stop_name, dist);
          }
        }

        const sorted = Array.from(byName.entries())
          .map(([stop_name, distance]) => ({stop_name, distance}))
          .sort((a, b) => a.distance - b.distance)
          .slice(0, 5);

        setNearby(sorted);
      },
      () => {
        // Permission denied or error — don't show section
      },
      {enableHighAccuracy: false, timeout: 10000, maximumAge: 60000},
    );
  }, [stops]);

  if (!nearby || nearby.length === 0) return null;

  return (
    <section className="animate-fade-slide-up">
      <SectionTitle>📍 Stații aproape de tine</SectionTitle>
      <div className="flex flex-col gap-2">
        {nearby.map((stop, i) => (
          <Link
            key={stop.stop_name}
            href={`/statii/${encodeURIComponent(stop.stop_name)}?from=home`}
            className="animate-row flex items-center justify-between rounded-lg border border-zinc-100 bg-white px-4 py-3 transition-colors hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-600"
            style={{animationDelay: `${i * 50}ms`}}
          >
            <span className="font-medium text-zinc-900 dark:text-white">{stop.stop_name}</span>
            <span className="shrink-0 rounded-full bg-purple-100 px-2.5 py-0.5 text-xs font-medium text-purple-700 dark:bg-purple-950/50 dark:text-purple-300">
              {formatDistance(stop.distance)}
            </span>
          </Link>
        ))}
      </div>
    </section>
  );
}

function RecentLinesSection() {
  const [lines, setLines] = useState<RecentLine[]>([]);

  useEffect(() => {
    setLines(getRecentLines());
  }, []);

  if (lines.length === 0) return null;

  return (
    <section className="animate-fade-slide-up" style={{animationDelay: "0.1s"}}>
      <SectionTitle>🚌 Linii vizualizate recent</SectionTitle>
      <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-none">
        {lines.map((line, i) => {
          const color = line.route_color || "#7c3aed";
          return (
            <Link
              key={line.route_short_name}
              href={`/linii/${encodeURIComponent(line.route_short_name)}?from=home`}
              className="animate-row flex shrink-0 items-start gap-3 rounded-lg border border-zinc-100 bg-white p-3 transition-colors hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-600"
              style={{animationDelay: `${i * 50}ms`, minWidth: "180px"}}
            >
              <div
                className="flex h-10 min-w-10 shrink-0 items-center justify-center rounded-md px-2 text-xs font-bold text-white"
                style={{backgroundColor: color}}
              >
                {line.route_short_name}
              </div>
              <div className="min-w-0">
                <div className="text-xs font-medium text-zinc-500 dark:text-zinc-400">
                  {routeTypeLabel(line.route_type)}
                </div>
                <div className="mt-0.5 line-clamp-2 text-xs text-zinc-700 dark:text-zinc-300">
                  {line.route_long_name}
                </div>
              </div>
            </Link>
          );
        })}
      </div>
    </section>
  );
}

function RecentStopsSection() {
  const [stops, setStops] = useState<RecentStop[]>([]);

  useEffect(() => {
    setStops(getRecentStops());
  }, []);

  if (stops.length === 0) return null;

  return (
    <section className="animate-fade-slide-up" style={{animationDelay: "0.2s"}}>
      <SectionTitle>🏁 Stații vizualizate recent</SectionTitle>
      <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-none">
        {stops.map((stop, i) => (
          <Link
            key={stop.stop_name}
            href={`/statii/${encodeURIComponent(stop.stop_name)}?from=home`}
            className="animate-row shrink-0 rounded-lg border border-zinc-100 bg-white px-4 py-3 text-sm font-medium text-zinc-900 transition-colors hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:text-white dark:hover:border-zinc-600"
            style={{animationDelay: `${i * 50}ms`}}
          >
            {stop.stop_name}
          </Link>
        ))}
      </div>
    </section>
  );
}

export default function HomePage({stops}: { stops: Stop[] }) {
  return (
    <div className="flex flex-col gap-8">
      <NearbyStationsSection stops={stops} />
      <RecentLinesSection />
      <RecentStopsSection />
    </div>
  );
}

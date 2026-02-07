"use client";

import {useEffect, useRef, useState} from "react";
import Link from "next/link";
import dynamic from "next/dynamic";
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
  stop_lat: number;
  stop_lon: number;
  distance: number;
}

interface NearbyData {
  stops: NearbyStop[];
  userLat: number;
  userLon: number;
}

function SectionTitle({children}: { children: React.ReactNode }) {
  return (
    <h2 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-white">
      {children}
    </h2>
  );
}

function NearbyStopsMapInner({nearbyData}: { nearbyData: NearbyData }) {
  const mapRef = useRef<L.Map | null>(null);
  const [L, setL] = useState<typeof import("leaflet") | null>(null);
  const mapContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    import("leaflet").then((leaflet) => {
      setL(leaflet.default ? leaflet.default : leaflet);
    });
  }, []);

  useEffect(() => {
    if (document.querySelector('link[href*="leaflet.css"]')) return;
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = "https://unpkg.com/leaflet@1.9.4/dist/leaflet.css";
    document.head.appendChild(link);
  }, []);

  useEffect(() => {
    if (!L || !mapContainerRef.current) return;

    if (!mapRef.current) {
      mapRef.current = L.map(mapContainerRef.current, {
        zoomControl: true,
        attributionControl: true,
      });

      L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a>',
        maxZoom: 19,
      }).addTo(mapRef.current);
    }

    const map = mapRef.current;

    // Clear existing layers (except tile layer)
    map.eachLayer((layer) => {
      if (!(layer instanceof L.TileLayer)) {
        map.removeLayer(layer);
      }
    });

    // User location marker — pulsing blue dot
    const userIcon = L.divIcon({
      className: "",
      html: `<div style="
        width: 16px; height: 16px;
        background: #3b82f6;
        border: 3px solid white;
        border-radius: 50%;
        box-shadow: 0 0 0 3px rgba(59,130,246,0.3), 0 2px 4px rgba(0,0,0,0.3);
      "></div>`,
      iconSize: [16, 16],
      iconAnchor: [8, 8],
    });

    L.marker([nearbyData.userLat, nearbyData.userLon], {icon: userIcon})
      .bindTooltip("Tu ești aici", {direction: "top", offset: [0, -10]})
      .addTo(map);

    // Stop markers
    const bounds = L.latLngBounds([[nearbyData.userLat, nearbyData.userLon]]);

    nearbyData.stops.forEach((stop) => {
      const circle = L.circleMarker([stop.stop_lat, stop.stop_lon], {
        radius: 6,
        color: "#7c3aed",
        fillColor: "#7c3aed",
        fillOpacity: 1,
        weight: 2,
      }).addTo(map);

      circle.bindTooltip(stop.stop_name, {direction: "top", offset: [0, -8]});
      bounds.extend([stop.stop_lat, stop.stop_lon]);
    });

    map.fitBounds(bounds, {padding: [40, 40]});
  }, [L, nearbyData]);

  useEffect(() => {
    return () => {
      if (mapRef.current) {
        mapRef.current.remove();
        mapRef.current = null;
      }
    };
  }, []);

  return (
    <div className="overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-700">
      <div ref={mapContainerRef} style={{height: "250px", width: "100%"}} />
    </div>
  );
}

const NearbyStopsMap = dynamic(() => Promise.resolve(NearbyStopsMapInner), {
  ssr: false,
});

function NearbyStationsSection({stops}: { stops: Stop[] }) {
  const [nearbyData, setNearbyData] = useState<NearbyData | null>(null);

  useEffect(() => {
    if (!("geolocation" in navigator)) return;

    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const {latitude, longitude} = pos.coords;

        // Calculate distances and deduplicate by name (keep closest with coords)
        const byName = new Map<string, { distance: number; stop_lat: number; stop_lon: number }>();
        for (const stop of stops) {
          const dist = haversineDistance(latitude, longitude, stop.stop_lat, stop.stop_lon);
          const existing = byName.get(stop.stop_name);
          if (existing === undefined || dist < existing.distance) {
            byName.set(stop.stop_name, {distance: dist, stop_lat: stop.stop_lat, stop_lon: stop.stop_lon});
          }
        }

        const sorted = Array.from(byName.entries())
          .map(([stop_name, data]) => ({stop_name, ...data}))
          .sort((a, b) => a.distance - b.distance)
          .slice(0, 5);

        setNearbyData({stops: sorted, userLat: latitude, userLon: longitude});
      },
      () => {
        // Permission denied or error — don't show section
      },
      {enableHighAccuracy: false, timeout: 10000, maximumAge: 60000},
    );
  }, [stops]);

  if (!nearbyData || nearbyData.stops.length === 0) return null;

  return (
    <section className="animate-fade-slide-up">
      <SectionTitle>📍 Stații aproape de tine</SectionTitle>
      <div className="mt-3 mb-5 flex flex-col gap-2">
        {nearbyData.stops.map((stop, i) => (
          <Link
            key={stop.stop_name}
            href={`/statii/${encodeURIComponent(stop.stop_name)}`}
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
      <NearbyStopsMap nearbyData={nearbyData} />
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
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {lines.map((line, i) => {
          const color = line.route_color || "#7c3aed";
          return (
            <Link
              key={line.route_short_name}
              href={`/linii/${encodeURIComponent(line.route_short_name)}`}
              className="animate-row flex items-start gap-3 rounded-lg border border-zinc-100 bg-white p-3 transition-colors hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-600"
              style={{animationDelay: `${i * 50}ms`}}
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
      <div className="flex flex-wrap gap-2">
        {stops.map((stop, i) => (
          <Link
            key={stop.stop_name}
            href={`/statii/${encodeURIComponent(stop.stop_name)}`}
            className="animate-row rounded-lg border border-zinc-100 bg-white px-3 py-2 text-sm font-medium text-zinc-900 transition-colors hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:text-white dark:hover:border-zinc-600"
            style={{animationDelay: `${i * 50}ms`}}
          >
            {stop.stop_name}
          </Link>
        ))}
      </div>
    </section>
  );
}

function WelcomeSection() {
  return (
    <section className="animate-fade-slide-up text-center py-12">
      <h1 className="text-3xl font-bold text-zinc-900 dark:text-white">
        Bine ai venit!
      </h1>
      <p className="mt-3 text-zinc-500 dark:text-zinc-400">
        Transportul public din Cluj-Napoca, la un click distanță.
      </p>
      <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:justify-center">
        <Link
          href="/linii"
          className="inline-flex items-center justify-center gap-2 rounded-lg bg-purple-600 px-6 py-3 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-purple-700"
        >
          🚌 Vezi toate liniile
        </Link>
        <Link
          href="/statii"
          className="inline-flex items-center justify-center gap-2 rounded-lg border border-zinc-200 bg-white px-6 py-3 text-sm font-semibold text-zinc-900 shadow-sm transition-colors hover:border-zinc-300 dark:border-zinc-700 dark:bg-zinc-900 dark:text-white dark:hover:border-zinc-600"
        >
          📍 Caută o stație
        </Link>
      </div>
    </section>
  );
}

export default function HomePage({stops}: { stops: Stop[] }) {
  const [contentState, setContentState] = useState<"loading" | "empty" | "has-content">("loading");

  useEffect(() => {
    const lines = getRecentLines();
    const recentStops = getRecentStops();
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setContentState(lines.length > 0 || recentStops.length > 0 ? "has-content" : "empty");
  }, []);

  return (
    <div className="flex flex-col gap-8">
      <NearbyStationsSection stops={stops} />
      <RecentLinesSection />
      <RecentStopsSection />
      {contentState === "empty" && <WelcomeSection />}
    </div>
  );
}

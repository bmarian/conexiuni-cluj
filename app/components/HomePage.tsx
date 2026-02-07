"use client";

import {useEffect, useRef, useState} from "react";
import Link from "next/link";
import type {Stop} from "@/types/tranzy";
import {RouteType} from "@/types/tranzy";
import {getRecentLines, getRecentStops, RecentLine, RecentStop} from "@/lib/recent-history";
import {getLeaflet, loadLeaflet} from "@/lib/leaflet-loader";

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

function NearbyStopsMap({nearbyData, hoveredStop}: { nearbyData: NearbyData; hoveredStop: string | null }) {
  const mapRef = useRef<L.Map | null>(null);
  const [L, setL] = useState<typeof import("leaflet") | null>(getLeaflet);
  const mapContainerRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const markersRef = useRef<Map<string, L.CircleMarker>>(new Map());
  const [isVisible, setIsVisible] = useState(false);

  // Observe visibility
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setIsVisible(true);
          observer.disconnect();
        }
      },
      {rootMargin: "100px"},
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  // Get Leaflet from singleton when visible (instant if already loaded)
  useEffect(() => {
    if (!isVisible || L) return;
    loadLeaflet().then(setL);
  }, [isVisible, L]);

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
    markersRef.current.clear();

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
      markersRef.current.set(stop.stop_name, circle);
    });

    map.fitBounds(bounds, {padding: [40, 40]});
  }, [L, nearbyData]);

  // Highlight hovered stop
  useEffect(() => {
    markersRef.current.forEach((marker, name) => {
      if (name === hoveredStop) {
        marker.setStyle({radius: 10, fillColor: "#a855f7", color: "#a855f7", weight: 3});
        marker.openTooltip();
      } else {
        marker.setStyle({radius: 6, fillColor: "#7c3aed", color: "#7c3aed", weight: 2});
        marker.closeTooltip();
      }
    });
  }, [hoveredStop]);

  useEffect(() => {
    return () => {
      if (mapRef.current) {
        mapRef.current.remove();
        mapRef.current = null;
      }
    };
  }, []);

  return (
    <div ref={sentinelRef} className="overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-700" style={{minHeight: "250px"}}>
      <div ref={mapContainerRef} style={{height: "100%", minHeight: "250px", width: "100%"}} />
    </div>
  );
}

function NearbyStationsSection({stops}: { stops: Stop[] }) {
  const [nearbyData, setNearbyData] = useState<NearbyData | null>(null);
  const [hoveredStop, setHoveredStop] = useState<string | null>(null);

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
      <div className="grid gap-4 md:grid-cols-2">
        <div className="flex flex-col gap-2">
          {nearbyData.stops.map((stop, i) => (
            <div
              key={stop.stop_name}
              className="animate-row flex items-center gap-2"
              style={{animationDelay: `${i * 50}ms`}}
              onMouseEnter={() => setHoveredStop(stop.stop_name)}
              onMouseLeave={() => setHoveredStop(null)}
            >
              <Link
                href={`/statii/${encodeURIComponent(stop.stop_name)}`}
                className={`flex flex-1 items-center justify-between rounded-lg border bg-white px-4 py-3 transition-all dark:bg-zinc-900 ${
                  hoveredStop === stop.stop_name
                    ? "border-purple-300 shadow-sm dark:border-purple-700"
                    : "border-zinc-100 hover:border-zinc-300 dark:border-zinc-800 dark:hover:border-zinc-600"
                }`}
              >
                <span className="font-medium text-zinc-900 dark:text-white">{stop.stop_name}</span>
                <span className="shrink-0 rounded-full bg-purple-100 px-2.5 py-0.5 text-xs font-medium text-purple-700 dark:bg-purple-950/50 dark:text-purple-300">
                  {formatDistance(stop.distance)}
                </span>
              </Link>
              <a
                href={`https://www.google.com/maps/dir/?api=1&destination=${stop.stop_lat},${stop.stop_lon}&travelmode=walking`}
                target="_blank"
                rel="noopener noreferrer"
                className="flex h-full shrink-0 items-center justify-center rounded-lg border border-zinc-100 bg-white px-3 py-3 text-zinc-400 transition-colors hover:border-blue-300 hover:text-blue-600 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-blue-700 dark:hover:text-blue-400"
                title="Navighează cu Google Maps"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <polygon points="3 11 22 2 13 21 11 13 3 11" />
                </svg>
              </a>
            </div>
          ))}
        </div>
        <NearbyStopsMap nearbyData={nearbyData} hoveredStop={hoveredStop} />
      </div>
    </section>
  );
}

function RecentLinesSection() {
  const [lines, setLines] = useState<RecentLine[]>([]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
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
    // eslint-disable-next-line react-hooks/set-state-in-effect
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

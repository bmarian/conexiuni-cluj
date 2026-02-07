"use client";

import {useEffect, useMemo, useRef, useState} from "react";
import type {Shape} from "@/types/tranzy";
import type {RouteStopInfo} from "@/lib/cluj-api";

// Dynamically import Leaflet to avoid SSR issues
import dynamic from "next/dynamic";

interface RouteMapInnerProps {
  outboundShape: Shape[];
  inboundShape: Shape[];
  outboundStops: RouteStopInfo[];
  inboundStops: RouteStopInfo[];
  color: string;
}

function RouteMapInner({outboundShape, inboundShape, outboundStops, inboundStops, color}: RouteMapInnerProps) {
  const mapRef = useRef<L.Map | null>(null);
  const [direction, setDirection] = useState<"outbound" | "inbound">("outbound");
  const [L, setL] = useState<typeof import("leaflet") | null>(null);
  const mapContainerRef = useRef<HTMLDivElement>(null);

  const activeShape = direction === "outbound" ? outboundShape : inboundShape;
  const activeStops = direction === "outbound" ? outboundStops : inboundStops;
  const hasInbound = inboundShape.length > 0;

  const positions = useMemo(
    () => activeShape.map((s) => [s.shape_pt_lat, s.shape_pt_lon] as [number, number]),
    [activeShape],
  );

  // Load Leaflet on mount
  useEffect(() => {
    import("leaflet").then((leaflet) => {
      setL(leaflet.default ? leaflet.default : leaflet);
    });
  }, []);

  // Import Leaflet CSS via link tag
  useEffect(() => {
    if (document.querySelector('link[href*="leaflet.css"]')) return;
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = "https://unpkg.com/leaflet@1.9.4/dist/leaflet.css";
    document.head.appendChild(link);
  }, []);

  // Create / update map
  useEffect(() => {
    if (!L || !mapContainerRef.current || positions.length === 0) return;

    // Create map if not exists
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

    // Draw route polyline
    const polyline = L.polyline(positions, {
      color: color,
      weight: 4,
      opacity: 0.85,
    }).addTo(map);

    // Direction arrow at the start of the route
    if (positions.length >= 2) {
      const [startLat, startLon] = positions[0];
      const [nextLat, nextLon] = positions[Math.min(3, positions.length - 1)];

      // Calculate angle
      const angle = Math.atan2(nextLon - startLon, nextLat - startLat) * (180 / Math.PI);

      const arrowIcon = L.divIcon({
        className: "",
        html: `<svg width="28" height="28" viewBox="0 0 28 28" style="transform: rotate(${90 - angle}deg);">
          <polygon points="4,10 24,14 4,18" fill="${color}" stroke="white" stroke-width="1.5"/>
        </svg>`,
        iconSize: [28, 28],
        iconAnchor: [14, 14],
      });

      L.marker([startLat, startLon], {icon: arrowIcon}).addTo(map);
    }

    // Add periodic arrows along the route for direction clarity
    const arrowInterval = Math.max(Math.floor(positions.length / 6), 10);
    for (let i = arrowInterval; i < positions.length - arrowInterval; i += arrowInterval) {
      const [lat, lon] = positions[i];
      const [nextLat, nextLon] = positions[Math.min(i + 3, positions.length - 1)];
      const angle = Math.atan2(nextLon - lon, nextLat - lat) * (180 / Math.PI);

      const smallArrow = L.divIcon({
        className: "",
        html: `<svg width="16" height="16" viewBox="0 0 16 16" style="transform: rotate(${90 - angle}deg);">
          <polygon points="2,5 14,8 2,11" fill="${color}" opacity="0.7"/>
        </svg>`,
        iconSize: [16, 16],
        iconAnchor: [8, 8],
      });

      L.marker([lat, lon], {icon: smallArrow, interactive: false}).addTo(map);
    }

    // Add stop markers
    activeStops.forEach((stop, i) => {
      const isTerminal = i === 0 || i === activeStops.length - 1;
      const radius = isTerminal ? 7 : 4;

      const circle = L.circleMarker([stop.stop_lat, stop.stop_lon], {
        radius,
        color: color,
        fillColor: isTerminal ? color : "#ffffff",
        fillOpacity: 1,
        weight: 2,
      }).addTo(map);

      circle.bindTooltip(stop.stop_name, {
        direction: "top",
        offset: [0, -8],
      });
    });

    // Fit bounds
    map.fitBounds(polyline.getBounds(), {padding: [30, 30]});

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [L, positions, activeStops, color]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (mapRef.current) {
        mapRef.current.remove();
        mapRef.current = null;
      }
    };
  }, []);

  if (positions.length === 0) return null;

  return (
    <div className="animate-fade-slide-up mt-6">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-zinc-900 dark:text-white">
          Hartă traseu
        </h2>
        {hasInbound && (
          <div className="flex gap-1 rounded-lg bg-zinc-100 p-0.5 dark:bg-zinc-800">
            <button
              onClick={() => setDirection("outbound")}
              className={`rounded-md px-3 py-1 text-xs font-medium transition-all ${
                direction === "outbound"
                  ? "bg-white text-zinc-900 shadow-sm dark:bg-zinc-700 dark:text-white"
                  : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400"
              }`}
            >
              Tur
            </button>
            <button
              onClick={() => setDirection("inbound")}
              className={`rounded-md px-3 py-1 text-xs font-medium transition-all ${
                direction === "inbound"
                  ? "bg-white text-zinc-900 shadow-sm dark:bg-zinc-700 dark:text-white"
                  : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400"
              }`}
            >
              Retur
            </button>
          </div>
        )}
      </div>

      <div className="overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-700">
        <div ref={mapContainerRef} style={{height: "400px", width: "100%"}} />
      </div>
    </div>
  );
}

// Dynamic import wrapper to prevent SSR
const RouteMap = dynamic(() => Promise.resolve(RouteMapInner), {
  ssr: false,
  loading: () => (
    <div className="mt-6">
      <h2 className="mb-3 text-sm font-semibold text-zinc-900 dark:text-white">
        Hartă traseu
      </h2>
      <div className="flex h-[400px] items-center justify-center rounded-lg border border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900">
        <p className="text-sm text-zinc-400">Se încarcă harta...</p>
      </div>
    </div>
  ),
});

export default function RouteMapWrapper(props: RouteMapInnerProps) {
  if (props.outboundShape.length === 0 && props.inboundShape.length === 0) return null;
  return <RouteMap {...props} />;
}

"use client";

import {useEffect, useMemo, useRef, useState} from "react";
import {useRouter} from "next/navigation";
import type {Shape} from "@/types/tranzy";
import type {RouteType} from "@/types/tranzy";
import type {RouteStopInfo} from "@/lib/cluj-api";
import {getLeaflet, loadLeaflet} from "@/lib/leaflet-loader";
import {useRouteVehicles} from "@/app/hooks/useRouteVehicles";
import {getVehicleIconPath} from "@/app/utils/route-utils";
import {useUserLocation} from "@/app/hooks/useUserLocation";
import {addCenterOnUserControl} from "@/app/components/CenterOnUserButton";

interface RouteMapInnerProps {
  outboundShape: Shape[];
  inboundShape: Shape[];
  outboundStops: RouteStopInfo[];
  inboundStops: RouteStopInfo[];
  color: string;
  routeId?: number;
  routeType?: RouteType;
}

function RouteMapInner({outboundShape, inboundShape, outboundStops, inboundStops, color, routeId, routeType}: RouteMapInnerProps) {
  const mapRef = useRef<L.Map | null>(null);
  const [direction, setDirection] = useState<"outbound" | "inbound">("outbound");
  const [L, setL] = useState<typeof import("leaflet") | null>(getLeaflet);
  const mapContainerRef = useRef<HTMLDivElement>(null);
  const router = useRouter();
  const vehicleMarkersRef = useRef<L.Marker[]>([]);

  const vehicleData = useRouteVehicles(routeId, outboundStops, inboundStops);
  const userLocation = useUserLocation();
  const userMarkerRef = useRef<L.Marker | null>(null);
  const centerControlRef = useRef<L.Control | null>(null);

  const activeShape = direction === "outbound" ? outboundShape : inboundShape;
  const activeStops = direction === "outbound" ? outboundStops : inboundStops;
  const hasInbound = inboundShape.length > 0;

  const positions = useMemo(
    () => activeShape.map((s) => [s.shape_pt_lat, s.shape_pt_lon] as [number, number]),
    [activeShape],
  );

  // Get Leaflet from singleton (instant if already preloaded)
  useEffect(() => {
    if (L) return;
    loadLeaflet().then(setL);
  }, [L]);

  // Create / update map
  useEffect(() => {
    if (!L || !mapContainerRef.current || positions.length === 0) return;

    if (!mapRef.current) {
      mapRef.current = L.map(mapContainerRef.current, {
        zoomControl: true,
        attributionControl: false,
      });

      L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
        maxZoom: 19,
      }).addTo(mapRef.current);
    }

    const map = mapRef.current;

    map.eachLayer((layer) => {
      if (!(layer instanceof L.TileLayer)) {
        map.removeLayer(layer);
      }
    });

    // Clear vehicle markers ref
    vehicleMarkersRef.current = [];

    const polyline = L.polyline(positions, {
      color: color,
      weight: 4,
      opacity: 0.85,
    }).addTo(map);

    if (positions.length >= 2) {
      const [startLat, startLon] = positions[0];
      const [, nextLon] = positions[Math.min(5, positions.length - 1)];
      const goingRight = nextLon >= startLon;
      const arrowChar = goingRight ? "→" : "←";

      const arrowIcon = L.divIcon({
        className: "",
        html: `<div style="
          background: ${color};
          color: white;
          font-weight: bolder;
          width: 32px;
          height: 32px;
          border-radius: 50%;
          display: flex;
          align-items: center;
          justify-content: center;
          border: 2px solid white;
          box-shadow: 0 2px 6px rgba(0,0,0,0.3);
          line-height: 1;
        ">${arrowChar}</div>`,
        iconSize: [32, 32],
        iconAnchor: [16, 16],
      });

      L.marker([startLat, startLon], {icon: arrowIcon}).addTo(map);
    }

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
      circle.on("click", () => {
        router.push(`/statii/${encodeURIComponent(stop.stop_name)}`);
      });
      circle.on("mouseover", () => {
        map.getContainer().style.cursor = "pointer";
      });
      circle.on("mouseout", () => {
        map.getContainer().style.cursor = "";
      });
    });

    map.fitBounds(polyline.getBounds(), {padding: [30, 30]});
  }, [L, positions, activeStops, color]);

  // Overlay vehicle markers (separate effect so they update without re-rendering the whole map)
  useEffect(() => {
    if (!L || !mapRef.current || !vehicleData) return;
    const map = mapRef.current;

    // Remove previous vehicle markers
    for (const m of vehicleMarkersRef.current) {
      map.removeLayer(m);
    }
    vehicleMarkersRef.current = [];

    const vehicles = direction === "outbound"
      ? vehicleData.directionVehicles.outbound
      : vehicleData.directionVehicles.inbound;

    const iconPath = getVehicleIconPath(routeType);

    for (const v of vehicles) {
      if (v.latitude == null || v.longitude == null) continue;

      const icon = L.divIcon({
        className: "",
        html: `<div style="position:relative;display:flex;flex-direction:column;align-items:center;">
          <img src="${iconPath}" width="28" height="28" style="filter:drop-shadow(0 2px 4px rgba(0,0,0,0.4));" />
          <div style="
            background:white;
            padding:2px 5px;
            border-radius:4px;
            font-size:11px;
            font-weight:bold;
            white-space:nowrap;
            box-shadow:0 1px 4px rgba(0,0,0,0.3);
            margin-top:1px;
            color:#1a1a1a;
            line-height:1;
          ">${v.label}</div>
        </div>`,
        iconSize: [40, 44],
        iconAnchor: [20, 22],
      });

      const marker = L.marker([v.latitude, v.longitude], {icon, zIndexOffset: 1000}).addTo(map);
      const speed = v.speed != null ? `${Math.round(v.speed)} km/h` : "";
      marker.bindTooltip(`${v.label}${speed ? ` · ${speed}` : ""}`, {
        direction: "top",
        offset: [0, -20],
      });
      vehicleMarkersRef.current.push(marker);
    }
  }, [L, vehicleData, direction, routeType]);

  // User location marker + center control
  useEffect(() => {
    if (!L || !mapRef.current || !userLocation) return;
    const map = mapRef.current;

    if (userMarkerRef.current) {
      map.removeLayer(userMarkerRef.current);
    }
    if (centerControlRef.current) {
      map.removeControl(centerControlRef.current);
    }

    const userIcon = L.divIcon({
      className: "",
      html: `<div style="
        width: 14px; height: 14px;
        background: #3b82f6;
        border: 2.5px solid white;
        border-radius: 50%;
        box-shadow: 0 0 0 3px rgba(59,130,246,0.3), 0 2px 4px rgba(0,0,0,0.3);
      "></div>`,
      iconSize: [14, 14],
      iconAnchor: [7, 7],
    });

    userMarkerRef.current = L.marker([userLocation.latitude, userLocation.longitude], {icon: userIcon, zIndexOffset: 900})
      .bindTooltip("Tu ești aici", {direction: "top", offset: [0, -10]})
      .addTo(map);

    centerControlRef.current = addCenterOnUserControl(L, map, userLocation.latitude, userLocation.longitude);
  }, [L, userLocation]);

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

export default function RouteMapWrapper(props: RouteMapInnerProps) {
  if (props.outboundShape.length === 0 && props.inboundShape.length === 0) return null;
  return <RouteMapInner {...props} />;
}

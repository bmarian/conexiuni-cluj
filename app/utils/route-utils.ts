import type {RouteType} from "@/types/tranzy";

export function routeTypeLabel(type: RouteType | number): string {
  switch (type) {
    case 0: return "Tramvai";
    case 3: return "Autobuz";
    case 11: return "Troleibuz";
    default: return "Linie";
  }
}

export function getVehicleIconPath(routeType?: RouteType | number): string {
  if (routeType === 0) return "/tram-icon.svg";
  if (routeType === 11) return "/trolleybus-icon.svg";
  return "/bus-icon.svg";
}

export function haversine(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const R = 6_371_000; // Earth radius in meters
  const toRad = Math.PI / 180;
  const dLat = (lat2 - lat1) * toRad;
  const dLon = (lon2 - lon1) * toRad;
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(lat1 * toRad) * Math.cos(lat2 * toRad) * Math.sin(dLon / 2) ** 2;
  return R * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}

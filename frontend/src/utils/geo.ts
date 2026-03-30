import type {StopTime, UserLocation} from "@/types/tranzy.ts";

const earthRadiusMeters = 6_371_000.0;

export const haversineMeters = (lat1: number, lon1: number, lat2: number, lon2: number): number => {
  const dLat = (lat2 - lat1) * (Math.PI / 180);
  const dLon = (lon2 - lon1) * (Math.PI / 180);
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(lat1 * (Math.PI / 180)) * Math.cos(lat2 * (Math.PI / 180)) * Math.sin(dLon / 2) * Math.sin(dLon / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return earthRadiusMeters * c;
};

export const closestStop = (userPosition: UserLocation, stops: StopTime[]): StopTime | null => {
  if (!Array.isArray(stops) || stops.length === 0) return null;

  let clStop = stops[0]!;
  let closestDistance = haversineMeters(userPosition.latitude, userPosition.longitude, clStop.stop_lat, clStop.stop_lon);

  for (let i = 1; i < stops.length; i++) {
    const currentStop = stops[i]!;
    const distance = haversineMeters(userPosition.latitude, userPosition.longitude, currentStop.stop_lat, currentStop.stop_lon);
    if (distance < closestDistance) {
      closestDistance = distance;
      clStop = currentStop;
    }
  }

  return clStop;
}

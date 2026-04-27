import type {StopTime, UserLocation} from "@/types/tranzy.ts"

const earthRadiusMeters = 6_371_000.0

export const formatMeters = (m: number): string =>
  m < 1000 ? `${Math.round(m)} m` : `${(m / 1000).toFixed(1)} km`

export const sortByDistance = <T>(
  items: T[],
  userLat: number,
  userLon: number,
  getLat: (item: T) => number,
  getLon: (item: T) => number,
): T[] =>
  items
    .map(item => ({item, dist: haversineMeters(userLat, userLon, getLat(item), getLon(item))}))
    .sort((a, b) => a.dist - b.dist)
    .map(({item}) => item)

export const haversineMeters = (lat1: number, lon1: number, lat2: number, lon2: number): number => {
  const dLat = (lat2 - lat1) * (Math.PI / 180)
  const dLon = (lon2 - lon1) * (Math.PI / 180)
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(lat1 * (Math.PI / 180)) * Math.cos(lat2 * (Math.PI / 180)) * Math.sin(dLon / 2) * Math.sin(dLon / 2)
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))
  return earthRadiusMeters * c
}

export const calculateBearing = (lat1: number, lon1: number, lat2: number, lon2: number) => {
  const toRad = (deg: number) => (deg * Math.PI) / 180;
  const toDeg = (rad: number) => (rad * 180) / Math.PI;

  const phi1 = toRad(lat1);
  const phi2 = toRad(lat2);
  const deltaLambda = toRad(lon2 - lon1);

  const y = Math.sin(deltaLambda) * Math.cos(phi2);
  const x = Math.cos(phi1) * Math.sin(phi2) - Math.sin(phi1) * Math.cos(phi2) * Math.cos(deltaLambda);

  const theta = Math.atan2(y, x);
  return (toDeg(theta) + 360) % 360;
};

export const closestStop = ({latitude, longitude}: {latitude: number, longitude: number}, stops: StopTime[]): StopTime | null => {
  if (!Array.isArray(stops) || stops.length === 0) return null

  let clStop = stops[0]!
  let closestDistance = haversineMeters(latitude, longitude, clStop.stop_lat, clStop.stop_lon)

  for (let i = 1; i < stops.length; i++) {
    const currentStop = stops[i]!
    const distance = haversineMeters(latitude, longitude, currentStop.stop_lat, currentStop.stop_lon)
    if (distance < closestDistance) {
      closestDistance = distance
      clStop = currentStop
    }
  }

  return clStop
}

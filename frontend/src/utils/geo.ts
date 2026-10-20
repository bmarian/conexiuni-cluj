import type {Stop, StopTime} from "@/types/tranzy.ts"

const earthRadiusMeters = 6_371_000.0

export const formatMeters = (m: number): string =>
  m < 1000 ? `${Math.round(m)} m` : `${(m / 1000).toFixed(1)} km`

export const sortByDistance = <T>(
  items: T[],
  userLat: number,
  userLon: number,
  getLat: (item: T) => number,
  getLon: (item: T) => number,
  maxDistanceMeters?: number,
): T[] => {
  let mapped = items.map(item => ({item, dist: haversineMeters(userLat, userLon, getLat(item), getLon(item))}))
  if (maxDistanceMeters !== undefined) {
    mapped = mapped.filter(d => d.dist <= maxDistanceMeters)
  }
  return mapped.sort((a, b) => a.dist - b.dist).map(({item}) => item)
}

export const haversineMeters = (lat1: number, lon1: number, lat2: number, lon2: number): number => {
  const dLat = (lat2 - lat1) * (Math.PI / 180)
  const dLon = (lon2 - lon1) * (Math.PI / 180)
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(lat1 * (Math.PI / 180)) * Math.cos(lat2 * (Math.PI / 180)) * Math.sin(dLon / 2) * Math.sin(dLon / 2)
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))
  return earthRadiusMeters * c
}

export const findClosestShapeIdx = (lat: number, lon: number, shape: {shape_pt_lat: number; shape_pt_lon: number}[]): number => {
  let idx = -1
  let best = Infinity
  for (let i = 0; i < shape.length; i++) {
    const d = haversineMeters(shape[i]!.shape_pt_lat, shape[i]!.shape_pt_lon, lat, lon)
    if (d < best) {
      best = d
      idx = i
    }
  }
  return idx
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

// Decodes a Google-encoded polyline into [lat, lng] pairs
export const decodePolyline = (encoded: string): [number, number][] => {
  const coords: [number, number][] = []
  let index = 0, lat = 0, lng = 0
  while (index < encoded.length) {
    let shift = 0, result = 0, b: number
    do { b = encoded.charCodeAt(index++) - 63; result |= (b & 0x1f) << shift; shift += 5 } while (b >= 0x20)
    lat += (result & 1) ? ~(result >> 1) : result >> 1
    shift = 0; result = 0
    do { b = encoded.charCodeAt(index++) - 63; result |= (b & 0x1f) << shift; shift += 5 } while (b >= 0x20)
    lng += (result & 1) ? ~(result >> 1) : result >> 1
    coords.push([lat / 1e5, lng / 1e5])
  }
  return coords
}

export const closestStop = (latitude: number, longitude: number, stops: StopTime[] | Stop[]): StopTime | Stop | null => {
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

type LngLat = [number, number]

const cross = (o: LngLat, a: LngLat, b: LngLat) => (a[0] - o[0]) * (b[1] - o[1]) - (a[1] - o[1]) * (b[0] - o[0])

const halfHull = (points: LngLat[]) => {
  const out: LngLat[] = []
  for (const p of points) {
    while (out.length >= 2 && cross(out.at(-2)!, out.at(-1)!, p) <= 0) out.pop()
    out.push(p)
  }
  return out.slice(0, -1)
}

export const convexHull = (points: LngLat[]): LngLat[] => {
  const sorted = [...points].sort((a, b) => a[0] - b[0] || a[1] - b[1])
  if (sorted.length < 3) return sorted
  return [...halfHull(sorted), ...halfHull(sorted.reverse())]
}

// The hull widened all round by `radius` degrees of longitude, drawn round on a Mercator map, as a closed ring.
export const roundedHull = (points: LngLat[], radius: number, steps = 16): LngLat[] => {
  const around = convexHull(points).flatMap(([lng, lat]) => {
    const k = Math.cos(lat * Math.PI / 180)
    return Array.from({length: steps}, (_, i): LngLat => {
      const a = 2 * Math.PI * i / steps
      return [lng + radius * Math.cos(a), lat + radius * k * Math.sin(a)]
    })
  })
  const ring = convexHull(around)
  return [...ring, ring[0]!]
}

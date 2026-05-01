import {apiRequest} from '@/utils/request_cache.ts'
import {decodePolyline} from '@/utils/geo.ts'
import type {DirectionsResponse} from '@/types/tranzy.ts'

type LatLng = [number, number]

const cache = new Map<string, LatLng[]>()
const failed = new Set<string>()

const roundCoord = (n: number) => n.toFixed(5)

const cacheKey = (a: LatLng, b: LatLng) =>
  `${roundCoord(a[0])},${roundCoord(a[1])}>${roundCoord(b[0])},${roundCoord(b[1])}`

// straightLine returns a 2-point polyline between two coordinates. Used as a
// fallback when the routing service is unreachable (quota exhausted, offline).
const straightLine = (from: LatLng, to: LatLng): LatLng[] => [from, to]

export const fetchWalkingPolyline = async (from: LatLng, to: LatLng): Promise<LatLng[]> => {
  const key = cacheKey(from, to)
  const cached = cache.get(key)
  if (cached) return cached
  if (failed.has(key)) return straightLine(from, to)

  try {
    const res = await apiRequest(
      `directions?from_lat=${from[0]}&from_lng=${from[1]}&to_lat=${to[0]}&to_lng=${to[1]}`
    ) as DirectionsResponse
    const geom = res?.routes?.[0]?.geometry
    if (!geom) throw new Error('no geometry in directions response')
    const decoded = decodePolyline(geom)
    cache.set(key, decoded)
    return decoded
  } catch (e) {
    console.warn('walking directions unavailable, falling back to straight line:', e)
    failed.add(key)
    const line = straightLine(from, to)
    cache.set(key, line)
    return line
  }
}

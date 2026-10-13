// Places come from the backend's own index of the area's addresses, named places and streets.

export interface Place {
  id: string
  lat: string
  lon: string
  label: string
}

interface ApiPlace {
  id?: unknown
  lat?: unknown
  lon?: unknown
  label?: unknown
}

const CACHED = 64

const cache = new Map<string, Place[]>()

function remember(url: string, places: Place[]) {
  cache.delete(url)
  cache.set(url, places)
  while (cache.size > CACHED) cache.delete(cache.keys().next().value!)
}

export function placeFromApi(raw: ApiPlace): Place | null {
  const lat = typeof raw.lat === 'number' ? raw.lat : Number.NaN
  const lon = typeof raw.lon === 'number' ? raw.lon : Number.NaN
  if (!Number.isFinite(lat) || !Number.isFinite(lon)) return null
  const label = typeof raw.label === 'string' ? raw.label.trim() : ''
  return {
    id: typeof raw.id === 'string' && raw.id ? raw.id : `${lat}:${lon}`,
    lat: String(lat),
    lon: String(lon),
    label: label || `${lat.toFixed(4)}, ${lon.toFixed(4)}`,
  }
}

async function request(url: string, signal?: AbortSignal): Promise<Place[]> {
  const hit = cache.get(url)
  if (hit) {
    remember(url, hit)
    return hit
  }
  const resp = await fetch(url, {signal})
  if (!resp.ok) return []
  const data = (await resp.json()) as unknown
  const places = (Array.isArray(data) ? data : [])
    .map((raw) => placeFromApi(raw as ApiPlace))
    .filter((place): place is Place => place !== null)
  remember(url, places)
  return places
}

export async function searchPlaces(query: string, limit: number, signal?: AbortSignal): Promise<Place[]> {
  const params = new URLSearchParams({q: query, limit: String(limit)})
  try {
    return await request(`/api/places?${params}`, signal)
  } catch {
    return []
  }
}

export async function reversePlace(lat: number, lon: number): Promise<Place | null> {
  const params = new URLSearchParams({lat: String(lat), lon: String(lon)})
  try {
    return (await request(`/api/places?${params}`))[0] ?? null
  } catch {
    return null
  }
}

export function resetPlaces() {
  cache.clear()
}

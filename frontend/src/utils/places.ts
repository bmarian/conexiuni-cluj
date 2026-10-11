// Photon is shared and free: requests are spaced out and cached.

export interface Place {
  id: string
  lat: string
  lon: string
  label: string
}

interface PhotonProperties {
  osm_id?: number | string
  osm_type?: string
  name?: string
  street?: string
  housenumber?: string
  district?: string
}

interface PhotonFeature {
  geometry?: { coordinates?: number[] }
  properties?: PhotonProperties
}

interface PhotonResponse {
  features?: PhotonFeature[] | PhotonFeature
}

const BASE_URL = 'https://photon.komoot.io'
// Ranked by the city centre, so the user's position isn't sent.
const BBOX = '22.75,46.38,24.27,47.50'
const CENTER_LAT = '46.77'
const CENTER_LON = '23.59'
export const MIN_GAP_MS = 1000
const CACHED = 64

const cache = new Map<string, PhotonResponse>()
let gate: Promise<unknown> = Promise.resolve()
let lastRequestAt = 0

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms))

function remember(url: string, data: PhotonResponse) {
  cache.delete(url)
  cache.set(url, data)
  while (cache.size > CACHED) cache.delete(cache.keys().next().value!)
}

function cached(url: string): PhotonResponse | undefined {
  const data = cache.get(url)
  if (data) remember(url, data)
  return data
}

function request(url: string, signal?: AbortSignal): Promise<PhotonResponse | null> {
  const hit = cached(url)
  if (hit) return Promise.resolve(hit)
  const run = gate.then(async () => {
    const again = cached(url)
    if (again) return again
    if (signal?.aborted) return null
    const wait = lastRequestAt + MIN_GAP_MS - Date.now()
    if (wait > 0) await sleep(wait)
    if (signal?.aborted) return null
    lastRequestAt = Date.now()
    try {
      const resp = await fetch(url, {signal})
      if (!resp.ok) return null
      const data = await resp.json() as PhotonResponse
      remember(url, data)
      return data
    } catch {
      return null
    }
  })
  gate = run.catch(() => null)
  return run
}

function features(data: PhotonResponse | null): PhotonFeature[] {
  const list = data?.features
  if (Array.isArray(list)) return list
  return list ? [list] : []
}

function compact(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' ? String(value).trim() : ''
}

function sameText(a: string, b: string): boolean {
  return a.localeCompare(b, undefined, {sensitivity: 'base'}) === 0
}

function formatStreetLabel(p: PhotonProperties | undefined, fallback: string): string {
  const name = compact(p?.name)
  const street = compact(p?.street)
  const houseNumber = compact(p?.housenumber)
  const district = compact(p?.district)
  let base: string
  if (street && houseNumber && name && !sameText(name, street)) base = `${name}, ${street}, ${houseNumber}`
  else if (street && houseNumber) base = `${street}, ${houseNumber}`
  else if (street) base = street
  else if (name) base = name
  else base = fallback
  return district ? `${base}, ${district}` : base
}

export function placeFromFeature(feature: PhotonFeature, index: number): Place | null {
  const coords = feature.geometry?.coordinates
  if (!coords || coords.length < 2) return null
  const [lon, lat] = coords
  if (typeof lat !== 'number' || typeof lon !== 'number' || !Number.isFinite(lat) || !Number.isFinite(lon)) return null
  const latStr = String(lat)
  const lonStr = String(lon)
  const p = feature.properties
  const label = formatStreetLabel(p, `${lat.toFixed(4)}, ${lon.toFixed(4)}`)
  const idKey = `${compact(p?.osm_type)}:${compact(p?.osm_id)}`
  return {id: idKey !== ':' ? idKey : `${latStr}:${lonStr}:${index}`, lat: latStr, lon: lonStr, label}
}

// Photon has no "ro"; its default names are Romanian.
const language = (locale: string) => (locale.startsWith('en') ? 'en' : 'default')

export async function searchPlaces(query: string, locale: string, limit: number, signal?: AbortSignal): Promise<Place[]> {
  const params = new URLSearchParams({
    q: query,
    bbox: BBOX,
    lat: CENTER_LAT,
    lon: CENTER_LON,
    limit: String(limit),
    lang: language(locale),
  })
  const data = await request(`${BASE_URL}/api/?${params}`, signal)
  return features(data)
    .map((feature, index) => placeFromFeature(feature, index))
    .filter((place): place is Place => place !== null)
}

export async function reversePlace(lat: number, lon: number, locale: string): Promise<Place | null> {
  const params = new URLSearchParams({lat: String(lat), lon: String(lon), lang: language(locale)})
  const first = features(await request(`${BASE_URL}/reverse?${params}`))[0]
  return first ? placeFromFeature(first, 0) : null
}

export function resetPlaces() {
  cache.clear()
  gate = Promise.resolve()
  lastRequestAt = 0
}

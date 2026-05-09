export interface NominatimPlace {
  id: string
  lat: string
  lon: string
  label: string
}

interface GeocodingData {
  osm_id?: number | string
  osm_type?: string
  label?: string
  name?: string
  street?: string
  housenumber?: string
  district?: string
}

interface GeocodeJsonFeature {
  geometry?: {
    coordinates?: [number, number]
  }
  properties?: {
    geocoding?: GeocodingData
  }
}

interface GeocodeJsonResponse {
  features?: GeocodeJsonFeature[] | GeocodeJsonFeature
}

const NOMINATIM_BASE_URL = 'https://nominatim.openstreetmap.org'

function toFeatureList(features: GeocodeJsonResponse['features']): GeocodeJsonFeature[] {
  if (Array.isArray(features)) return features
  return features ? [features] : []
}

function compact(value: string | undefined): string {
  return (value ?? '').trim()
}

function sameText(a: string, b: string): boolean {
  return a.localeCompare(b, undefined, {sensitivity: 'base'}) === 0
}

function formatStreetLabel(geocoding: GeocodingData | undefined, fallback: string): string {
  const name = compact(geocoding?.name)
  const street = compact(geocoding?.street)
  const houseNumber = compact(geocoding?.housenumber)
  const district = compact(geocoding?.district)
  let base = ''
  if (street && houseNumber && name && !sameText(name, street)) {
    base = `${name}, ${street}, ${houseNumber}`
  } else if (street && houseNumber) {
    base = `${street}, ${houseNumber}`
  } else if (street) {
    base = street
  } else if (name) {
    base = name
  } else {
    base = fallback
  }
  if (!district) return base
  return `${base}, ${district}`
}

function makeFallbackLabel(lat: string, lon: string): string {
  return `${Number(lat).toFixed(4)}, ${Number(lon).toFixed(4)}`
}

function mapFeatureToPlace(feature: GeocodeJsonFeature, index: number): NominatimPlace | null {
  const coords = feature.geometry?.coordinates
  if (!coords || coords.length < 2) return null
  const [lon, lat] = coords
  const latStr = String(lat)
  const lonStr = String(lon)
  const geocoding = feature.properties?.geocoding
  const fallback = compact(geocoding?.label) || makeFallbackLabel(latStr, lonStr)
  const label = formatStreetLabel(geocoding, fallback)
  const idKey = `${compact(String(geocoding?.osm_type ?? ''))}:${compact(String(geocoding?.osm_id ?? ''))}`
  const id = idKey !== ':' ? idKey : `${latStr}:${lonStr}:${index}`
  return {
    id,
    lat: latStr,
    lon: lonStr,
    label,
  }
}

export async function searchNominatimPlaces(
  query: string,
  locale: string,
  limit: number,
): Promise<NominatimPlace[]> {
  const params = new URLSearchParams({
    q: query,
    format: 'geocodejson',
    addressdetails: '1',
    countrycodes: 'ro',
    viewbox: '22.75,47.50,24.27,46.38',
    bounded: '1',
    limit: String(limit),
    'accept-language': locale,
  })
  const resp = await fetch(`${NOMINATIM_BASE_URL}/search?${params}`)
  if (!resp.ok) return []
  const data: GeocodeJsonResponse = await resp.json()
  return toFeatureList(data.features)
    .map((feature, index) => mapFeatureToPlace(feature, index))
    .filter((place): place is NominatimPlace => place !== null)
}

export async function reverseNominatimPlace(
  lat: number,
  lon: number,
  locale: string,
): Promise<NominatimPlace | null> {
  const params = new URLSearchParams({
    lat: lat.toString(),
    lon: lon.toString(),
    format: 'geocodejson',
    addressdetails: '1',
    'accept-language': locale,
  })
  const resp = await fetch(`${NOMINATIM_BASE_URL}/reverse?${params}`)
  if (!resp.ok) return null
  const data: GeocodeJsonResponse = await resp.json()
  const firstFeature = toFeatureList(data.features)[0]
  if (!firstFeature) return null
  return mapFeatureToPlace(firstFeature, 0)
}

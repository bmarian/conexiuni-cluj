import {ref} from 'vue'

// The city is fixed for the page's lifetime: every cache below is keyed by it, so switching reloads.

export type City = {
  slug: string
  name: string
  operator: string
  realtime: boolean
  lat: number
  lon: number
}

const DEFAULT_CITY = 'cluj'
// The city the person picked, or absent when the nearest one is chosen on each start.
const CHOICE_KEY = 'settings.city'
const LAST_KEY = 'settings.cityLast'
const LIST_KEY = 'cities'
// Long enough for a cold fix indoors; the last city stands when it runs out.
const LOCATE_TIMEOUT_MS = 8000

function stored(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function store(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key)
    else localStorage.setItem(key, value)
  } catch { /* noop */ }
}

function readList(): City[] {
  try {
    const list = JSON.parse(stored(LIST_KEY) ?? 'null')
    return Array.isArray(list) ? list : []
  } catch {
    return []
  }
}

export const cities = ref<City[]>(readList())

export const citiesLoaded: Promise<City[]> = typeof window === 'undefined' ? Promise.resolve([]) : (async () => {
  try {
    const response = await fetch('/api/cities')
    if (response.ok) {
      const list = await response.json() as City[]
      if (Array.isArray(list) && list.length) {
        cities.value = list
        store(LIST_KEY, JSON.stringify(list))
      }
    }
  } catch { /* offline: the stored list stands */ }
  return cities.value
})()

function cityFromPath(): string | null {
  const first = window.location.pathname.split('/')[1] ?? ''
  return /^[a-z][a-z0-9-]*$/.test(first) && first !== 'android' && first !== 'admin' ? first : null
}

export const cityChoice = ref<string | null>(stored(CHOICE_KEY))

export const currentCity: string = typeof window === 'undefined' ? DEFAULT_CITY
  : (cityFromPath() ?? cityChoice.value ?? stored(LAST_KEY) ?? DEFAULT_CITY)

export function rememberCity(slug: string) {
  store(LAST_KEY, slug)
}

export function knownCity(slug: string): boolean {
  return !cities.value.length || cities.value.some((city) => city.slug === slug)
}

export function cityInfo(): City | undefined {
  return cities.value.find((city) => city.slug === currentCity)
}

export const cityPath = (path: string) => `/${currentCity}${path.startsWith('/') ? path : '/' + path}`

export const apiPath = (path: string) => `/api/${currentCity}/${path.replace(/^\//, '')}`

export const cacheKey = (key: string) => `${currentCity}:${key}`

function distanceKm(aLat: number, aLon: number, bLat: number, bLon: number): number {
  const toRad = (d: number) => d * Math.PI / 180
  const dLat = toRad(bLat - aLat)
  const dLon = toRad(bLon - aLon)
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(toRad(aLat)) * Math.cos(toRad(bLat)) * Math.sin(dLon / 2) ** 2
  return 2 * 6371 * Math.asin(Math.sqrt(h))
}

function locate(): Promise<GeolocationPosition | null> {
  return new Promise((resolve) => {
    if (!navigator.geolocation) return resolve(null)
    navigator.geolocation.getCurrentPosition(resolve, () => resolve(null), {timeout: LOCATE_TIMEOUT_MS, maximumAge: Infinity})
  })
}

// The chosen city, else the nearest to where the phone is, else the last one used.
export async function resolveCity(): Promise<string> {
  const list = await citiesLoaded
  if (cityChoice.value && knownCity(cityChoice.value)) return cityChoice.value
  const fallback = stored(LAST_KEY) ?? list[0]?.slug ?? DEFAULT_CITY
  if (list.length < 2) return fallback
  const position = await locate()
  if (!position) return fallback
  const {latitude, longitude} = position.coords
  let nearest = list[0]!
  for (const city of list) {
    if (distanceKm(latitude, longitude, city.lat, city.lon) < distanceKm(latitude, longitude, nearest.lat, nearest.lon)) nearest = city
  }
  return nearest.slug
}

// A full load: every store starts from that city's caches. null means back to choosing by location.
export async function switchCity(slug: string | null) {
  cityChoice.value = slug
  store(CHOICE_KEY, slug)
  const target = slug ?? await resolveCity()
  rememberCity(target)
  window.location.href = `/${target}/`
}

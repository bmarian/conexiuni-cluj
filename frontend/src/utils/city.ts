import {ref} from 'vue'

export type City = {
  slug: string
  name: string
  operator: string
  realtime: boolean
  lat: number
  lon: number
  credits?: {name: string; url: string}[]
}

export const DEFAULT_CITY = 'cluj'
const CHOICE_KEY = 'settings.city'
const LAST_KEY = 'settings.cityLast'
const LIST_KEY = 'cities'
// Long enough for a cold fix indoors.
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

// Saves from before cities are Cluj's; copied, not moved, so an older build still reads them.
const PRE_CITY_KEYS = ['favorites:routes', 'favorites:stops', 'favorites:plans', 'recents:plans', 'follows:routes',
  'follows.v1', 'reminders.v1', 'reminders.synced', 'route-changes:seen', 'map.lastView', 'settings.showGreenFriday']

export function adoptPreCitySaves() {
  for (const key of PRE_CITY_KEYS) {
    const value = stored(key)
    if (value !== null && stored(`${DEFAULT_CITY}:${key}`) === null) store(`${DEFAULT_CITY}:${key}`, value)
  }
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

// Every per-city store and cache reloads when it changes.
export const currentCity = ref(typeof window === 'undefined' ? DEFAULT_CITY
  : (cityFromPath() ?? cityChoice.value ?? stored(LAST_KEY) ?? DEFAULT_CITY))

export function setCity(slug: string) {
  store(LAST_KEY, slug)
  currentCity.value = slug
}

export function knownCity(slug: string): boolean {
  return !cities.value.length || cities.value.some((city) => city.slug === slug)
}

export function cityInfo(): City | undefined {
  return cities.value.find((city) => city.slug === currentCity.value)
}

export function cityParams(): {city: string; operator: string} {
  const info = cityInfo()
  return {city: info?.name ?? currentCity.value, operator: info?.operator ?? ''}
}

export function cityCenter(): {lat: number; lon: number} {
  const info = cityInfo()
  return info ? {lat: info.lat, lon: info.lon} : {lat: 46.7712, lon: 23.6236}
}

export const cityPath = (path: string) => `/${currentCity.value}${path.startsWith('/') ? path : '/' + path}`

export const apiPath = (path: string) => `/api/${currentCity.value}/${path.replace(/^\//, '')}`

export const cacheKey = (key: string, city = currentCity.value) => `${city}:${key}`

function distanceKm(aLat: number, aLon: number, bLat: number, bLon: number): number {
  const toRad = (d: number) => d * Math.PI / 180
  const dLat = toRad(bLat - aLat)
  const dLon = toRad(bLon - aLon)
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(toRad(aLat)) * Math.cos(toRad(bLat)) * Math.sin(dLon / 2) ** 2
  return 2 * 6371 * Math.asin(Math.sqrt(h))
}

// Read from an existing grant; nothing here asks for it.
export const locationGranted = ref(false)

async function checkLocationGranted(): Promise<boolean> {
  try {
    const status = await navigator.permissions.query({name: 'geolocation'})
    locationGranted.value = status.state === 'granted'
    status.onchange = () => { locationGranted.value = status.state === 'granted' }
  } catch {
    locationGranted.value = false
  }
  return locationGranted.value
}

if (typeof window !== 'undefined') void checkLocationGranted()

function locate(): Promise<GeolocationPosition | null> {
  return new Promise((resolve) => {
    if (!navigator.geolocation) return resolve(null)
    navigator.geolocation.getCurrentPosition(resolve, () => resolve(null), {timeout: LOCATE_TIMEOUT_MS, maximumAge: Infinity})
  })
}

export async function resolveCity(): Promise<string> {
  const list = await citiesLoaded
  if (cityChoice.value && knownCity(cityChoice.value)) return cityChoice.value
  const fallback = stored(LAST_KEY) ?? list[0]?.slug ?? DEFAULT_CITY
  if (list.length < 2 || !await checkLocationGranted()) return fallback
  const position = await locate()
  if (!position) return fallback
  const {latitude, longitude} = position.coords
  let nearest = list[0]!
  for (const city of list) {
    if (distanceKm(latitude, longitude, city.lat, city.lon) < distanceKm(latitude, longitude, nearest.lat, nearest.lon)) nearest = city
  }
  return nearest.slug
}

// The city to show; null goes back to choosing by location.
export async function chooseCity(slug: string | null): Promise<string> {
  cityChoice.value = slug
  store(CHOICE_KEY, slug)
  return slug ?? await resolveCity()
}

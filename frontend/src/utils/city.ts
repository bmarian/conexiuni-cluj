import {ref} from 'vue'

export type Bounds = {south: number; west: number; north: number; east: number}

export type City = {
  slug: string
  name: string
  operator: string
  operatorUrl?: string
  realtime: boolean
  news?: boolean
  lat: number
  lon: number
  bounds?: Bounds
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
  } catch {
    /* noop */
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

export async function reloadCities(): Promise<City[]> {
  try {
    const response = await fetch('/api/cities')
    if (response.ok) {
      const list = (await response.json()) as City[]
      if (Array.isArray(list) && list.length) {
        cities.value = list
        store(LIST_KEY, JSON.stringify(list))
      }
    }
  } catch {
    /* offline: the stored list stands */
  }
  return cities.value
}

export const citiesLoaded: Promise<City[]> = typeof window === 'undefined' ? Promise.resolve([]) : reloadCities()

export const citiesReady: Promise<City[]> = cities.value.length ? Promise.resolve(cities.value) : citiesLoaded

function cityFromPath(): string | null {
  const first = window.location.pathname.split('/')[1] ?? ''
  return /^[a-z][a-z0-9-]*$/.test(first) && !['admin', 'plan'].includes(first) && knownCity(first) ? first : null
}

export const cityChoice = ref<string | null>(stored(CHOICE_KEY))

export const currentCity = ref(typeof window === 'undefined' ? DEFAULT_CITY : (cityFromPath() ?? cityChoice.value ?? stored(LAST_KEY) ?? DEFAULT_CITY))

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

export function cityBounds(slug = currentCity.value): Bounds | undefined {
  return cities.value.find((city) => city.slug === slug)?.bounds
}

export function inCity(lat: number, lon: number, slug = currentCity.value): boolean {
  const b = cityBounds(slug)
  return !b || (lat >= b.south && lat <= b.north && lon >= b.west && lon <= b.east)
}

export const cityPath = (path: string) => `/${currentCity.value}${path.startsWith('/') ? path : '/' + path}`

export const apiPath = (path: string, city = currentCity.value) => `/api/${city}/${path.replace(/^\//, '')}`

export const cacheKey = (key: string, city = currentCity.value) => `${city}:${key}`

export function citiesSaving(key: string): string[] {
  const out: string[] = []
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const name = localStorage.key(i)
      const slug = name?.endsWith(`:${key}`) ? name.slice(0, -key.length - 1) : ''
      if (!/^[a-z][a-z0-9-]*$/.test(slug)) continue
      const list = JSON.parse(localStorage.getItem(name!) ?? 'null')
      if (Array.isArray(list) && list.length) out.push(slug)
    }
  } catch {
    /* noop */
  }
  return out
}

function distanceKm(aLat: number, aLon: number, bLat: number, bLon: number): number {
  const toRad = (d: number) => (d * Math.PI) / 180
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
    status.onchange = () => {
      locationGranted.value = status.state === 'granted'
    }
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

export function openingCity(): string {
  if (cityChoice.value && knownCity(cityChoice.value)) return cityChoice.value
  return [stored(LAST_KEY), DEFAULT_CITY].find((slug) => slug && knownCity(slug)) ?? cities.value[0]?.slug ?? DEFAULT_CITY
}

export async function locatedCity(): Promise<string | null> {
  const list = await citiesReady
  if ((cityChoice.value && knownCity(cityChoice.value)) || list.length < 2 || !(await checkLocationGranted())) return null
  const position = await locate()
  if (!position) return null
  const {latitude, longitude} = position.coords
  let nearest = list[0]!
  for (const city of list) {
    if (distanceKm(latitude, longitude, city.lat, city.lon) < distanceKm(latitude, longitude, nearest.lat, nearest.lon)) nearest = city
  }
  return nearest.slug
}

export async function chooseCity(slug: string | null): Promise<string> {
  cityChoice.value = slug
  store(CHOICE_KEY, slug)
  return slug ?? (await locatedCity()) ?? openingCity()
}

import {defineStore} from 'pinia'
import {computed, ref, watch} from 'vue'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'
import type {RouteDirection} from '@/types/tranzy.ts'
import {cacheKey, currentCity} from '@/utils/city.ts'
import {network} from '@/utils/network.ts'
import {matching, RESULTS_PER_KIND} from '@/utils/stopSearch.ts'
import {fold} from '@/utils/text.ts'
import {CITY_CAP, RECENT_CAP, roundCoordinate} from '@/utils/sync/records.ts'
import {useSettingsStore} from '@/stores/settings'
import {translate} from '@/i18n'

export const FAVORITE_KEYS = ['favorites:routes', 'favorites:stops', 'favorites:plans', 'recents:plans']
const routesKey = (city?: string) => cacheKey('favorites:routes', city)
const stopsKey = (city?: string) => cacheKey('favorites:stops', city)
const plansKey = (city?: string) => cacheKey('favorites:plans', city)
const recentPlansKey = (city?: string) => cacheKey('recents:plans', city)

export interface FavoritePlan {
  name: string
  label?: string
  lat: number
  lon: number
  originName?: string
  originLat?: number
  originLon?: number
}

export type Recent = FavoritePlan | {stopId: number} | {routeId: number}

export function recentKey(r: Recent): string {
  if ('stopId' in r) return `stop-${r.stopId}`
  if ('routeId' in r) return `route-${r.routeId}`
  return `${r.lat}_${r.lon}_${r.originLat ?? ''}_${r.originLon ?? ''}`
}

export interface FavoriteRoute {
  routeId: number
  direction: RouteDirection
}

function sameRoute(a: FavoriteRoute, b: FavoriteRoute): boolean {
  return a.routeId === b.routeId && a.direction === b.direction
}

const rounded = (x: number | undefined) => (x === undefined ? undefined : roundCoordinate(x))

// Saved places keep six decimals, as sync carries them, so a place compares equal on every device.
function samePlace(p: FavoritePlan, lat: number, lon: number, originLat?: number, originLon?: number): boolean {
  return p.lat === rounded(lat) && p.lon === rounded(lon) && rounded(p.originLat) === rounded(originLat) && rounded(p.originLon) === rounded(originLon)
}

function atCap(list: unknown[]): boolean {
  if (list.length < CITY_CAP) return false
  useSettingsStore().showToast(translate('limitReached', {n: CITY_CAP}))
  return true
}

export function parseFavoriteRoutes(raw: unknown): FavoriteRoute[] {
  if (!Array.isArray(raw)) return []
  const parsed: FavoriteRoute[] = []
  for (const item of raw) {
    let fav: FavoriteRoute | null = null
    // Favorites saved before directions existed are bare route ids.
    if (typeof item === 'number') fav = {routeId: item, direction: '0'}
    else if (typeof item?.routeId === 'number' && (item.direction === '0' || item.direction === '1')) {
      fav = {routeId: item.routeId, direction: item.direction}
    }
    if (fav && !parsed.some((p) => sameRoute(p, fav))) parsed.push(fav)
  }
  return parsed
}

export function parseStopIds(raw: unknown): number[] {
  return Array.isArray(raw) ? raw.filter((x): x is number => typeof x === 'number') : []
}

export function parsePlans(raw: unknown): FavoritePlan[] {
  if (!Array.isArray(raw)) return []
  return raw.filter((p): p is FavoritePlan => typeof p?.name === 'string' && Number.isFinite(p.lat) && Number.isFinite(p.lon))
}

export function parseRecents(raw: unknown): Recent[] {
  if (!Array.isArray(raw)) return []
  return raw.filter((r): r is Recent => Number.isInteger(r?.stopId) || Number.isInteger(r?.routeId) || parsePlans([r]).length > 0)
}

export function matchingPlans(plans: FavoritePlan[], query: string): FavoritePlan[] {
  const q = fold(query.trim())
  if (!q) return []
  return matching(plans, (p) => fold(`${p.label ?? ''} ${p.name}`), q)
    .slice(0, RESULTS_PER_KIND)
    .map((m) => m.item)
}

function readSaved(key: string): unknown {
  try {
    return JSON.parse(localStorage.getItem(key) ?? 'null')
  } catch {
    return null
  }
}

export function savedFavorites(city: string) {
  return {
    routes: parseFavoriteRoutes(readSaved(routesKey(city))),
    stops: parseStopIds(readSaved(stopsKey(city))),
    plans: parsePlans(readSaved(plansKey(city))),
    recentPlans: parseRecents(readSaved(recentPlansKey(city))),
  }
}

export const useFavoritesStore = defineStore('favorites', () => {
  const favoriteRoutes = ref<FavoriteRoute[]>([])
  const favoriteStopIds = ref<number[]>([])
  const favoritePlans = ref<FavoritePlan[]>([])
  const recents = ref<Recent[]>([])
  const isHydrated = ref(false)
  const favoriteLineNames = computed(() => new Set(favoriteRoutes.value.map((r) => network.value?.routeById.get(r.routeId)?.route_short_name ?? '')))

  async function hydrate() {
    try {
      const routes = JSON.parse(localStorage.getItem(routesKey()) ?? 'null')
      const stops = JSON.parse(localStorage.getItem(stopsKey()) ?? 'null')
      const plans = JSON.parse(localStorage.getItem(plansKey()) ?? 'null')
      const savedRecents = JSON.parse(localStorage.getItem(recentPlansKey()) ?? 'null')
      favoriteRoutes.value = parseFavoriteRoutes(routes)
      favoriteStopIds.value = Array.isArray(stops) ? stops : []
      favoritePlans.value = Array.isArray(plans) ? plans : []
      recents.value = parseRecents(savedRecents)
    } catch (err) {
      console.warn('Failed to hydrate favorites:', err)
    } finally {
      isHydrated.value = true
    }
  }

  function persistRoutes() {
    localStorage.setItem(routesKey(), JSON.stringify(favoriteRoutes.value))
  }

  function persistStops() {
    localStorage.setItem(stopsKey(), JSON.stringify(favoriteStopIds.value))
  }

  function persistPlans() {
    localStorage.setItem(plansKey(), JSON.stringify(favoritePlans.value))
  }

  function persistRecents() {
    localStorage.setItem(recentPlansKey(), JSON.stringify(recents.value))
  }

  function isRouteFavorite(routeId: number, direction?: RouteDirection): boolean {
    return favoriteRoutes.value.some((r) => r.routeId === routeId && (direction === undefined || r.direction === direction))
  }

  function isStopFavorite(id: number): boolean {
    return favoriteStopIds.value.includes(id)
  }

  function favoritePlatform(stopId: number): number | undefined {
    const platforms = network.value?.platforms(stopId).map((p) => p.stop_id)
    return (platforms?.length ? platforms : [stopId]).find(isStopFavorite)
  }

  // Devices may have saved different platforms of one station: every one of them goes.
  function toggleStationFavorite(stopId: number) {
    const platforms = network.value?.platforms(stopId).map((p) => p.stop_id) ?? []
    const saved = [stopId, ...platforms].filter(isStopFavorite)
    if (!saved.length) return toggleStopFavorite(stopId)
    reorderStopIds(favoriteStopIds.value.filter((id) => !saved.includes(id)))
  }

  function isPlanFavorite(lat: number, lon: number, originLat?: number, originLon?: number): boolean {
    return favoritePlans.value.some((p) => samePlace(p, lat, lon, originLat, originLon))
  }

  function toggleRouteFavorite(routeId: number, direction: RouteDirection) {
    const fav = {routeId, direction}
    const idx = favoriteRoutes.value.findIndex((r) => sameRoute(r, fav))
    if (idx === -1 && atCap(favoriteRoutes.value)) return
    if (idx === -1) favoriteRoutes.value.push(fav)
    else favoriteRoutes.value.splice(idx, 1)
    persistRoutes()
  }

  function toggleStopFavorite(id: number) {
    const idx = favoriteStopIds.value.indexOf(id)
    if (idx === -1 && atCap(favoriteStopIds.value)) return
    if (idx === -1) favoriteStopIds.value.push(id)
    else favoriteStopIds.value.splice(idx, 1)
    persistStops()
  }

  function togglePlanFavorite(plan: FavoritePlan) {
    const idx = favoritePlans.value.findIndex((p) => samePlace(p, plan.lat, plan.lon, plan.originLat, plan.originLon))
    if (idx === -1 && atCap(favoritePlans.value)) return
    if (idx === -1)
      favoritePlans.value.push({
        ...plan,
        lat: roundCoordinate(plan.lat),
        lon: roundCoordinate(plan.lon),
        originLat: rounded(plan.originLat),
        originLon: rounded(plan.originLon),
      })
    else favoritePlans.value.splice(idx, 1)
    persistPlans()
  }

  function renamePlanFavorite(lat: number, lon: number, originLat: number | undefined, originLon: number | undefined, label: string | undefined) {
    const plan = favoritePlans.value.find((p) => samePlace(p, lat, lon, originLat, originLon))
    if (plan) {
      plan.label = label || undefined
      persistPlans()
    }
  }

  function reorderRoutes(newRoutes: FavoriteRoute[]) {
    favoriteRoutes.value = newRoutes
    persistRoutes()
  }

  function reorderStopIds(newIds: number[]) {
    favoriteStopIds.value = newIds
    persistStops()
  }

  function reorderPlans(newPlans: FavoritePlan[]) {
    favoritePlans.value = newPlans
    persistPlans()
  }

  function addRecent(recent: Recent) {
    const key = recentKey(recent)
    recents.value = [recent, ...recents.value.filter((r) => recentKey(r) !== key)].slice(0, RECENT_CAP)
    persistRecents()
  }

  function removeRecent(recent: Recent) {
    const key = recentKey(recent)
    recents.value = recents.value.filter((r) => recentKey(r) !== key)
    persistRecents()
  }

  function importRecents(list: Recent[], city = currentCity.value) {
    localStorage.setItem(recentPlansKey(city), JSON.stringify(list))
    if (city === currentCity.value) recents.value = list
  }

  function importAll(data: {routes?: unknown; stops?: unknown; plans?: unknown; recentPlans?: unknown}, city = currentCity.value) {
    const routes = parseFavoriteRoutes(data.routes)
    const stops = parseStopIds(data.stops)
    const plans = parsePlans(data.plans)
    const imported = parseRecents(data.recentPlans)
    localStorage.setItem(routesKey(city), JSON.stringify(routes))
    localStorage.setItem(stopsKey(city), JSON.stringify(stops))
    localStorage.setItem(plansKey(city), JSON.stringify(plans))
    localStorage.setItem(recentPlansKey(city), JSON.stringify(imported))
    if (city !== currentCity.value) return
    favoriteRoutes.value = routes
    favoriteStopIds.value = stops
    favoritePlans.value = plans
    recents.value = imported
  }

  async function preloadFavorites() {
    const jobs: Promise<unknown>[] = []

    if (favoriteRoutes.value.length) {
      jobs.push(preloadFavoriteRoutes())
    }
    if (favoriteStopIds.value.length) {
      const {fetchStopData} = useStopInfoApi()
      jobs.push(Promise.allSettled(favoriteStopIds.value.map((id) => fetchStopData(String(id)))))
    }

    await Promise.all(jobs)
  }

  async function preloadFavoriteRoutes() {
    const {routes, fetchRoutes} = useRoutesApi()
    const {fetchShapeInfo} = useRouteShapeInfoApi()
    try {
      await fetchRoutes()
    } catch (err) {
      console.warn('Could not fetch routes list for favorite preload:', err)
      return
    }
    const wanted = new Set(favoriteRoutes.value.map((r) => r.routeId))
    const targets = routes.value.filter((r) => wanted.has(r.route_id))
    await Promise.allSettled(targets.map((r) => fetchShapeInfo(r)))
  }

  watch(currentCity, () => void hydrate().then(preloadFavorites), {flush: 'sync'})

  return {
    favoriteRoutes,
    favoriteLineNames,
    favoriteStopIds,
    favoritePlans,
    recents,
    isHydrated,
    hydrate,
    isRouteFavorite,
    isStopFavorite,
    favoritePlatform,
    isPlanFavorite,
    toggleRouteFavorite,
    toggleStopFavorite,
    toggleStationFavorite,
    togglePlanFavorite,
    renamePlanFavorite,
    reorderRoutes,
    reorderStopIds,
    reorderPlans,
    addRecent,
    removeRecent,
    importRecents,
    importAll,
    preloadFavorites,
  }
})

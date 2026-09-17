import {defineStore} from 'pinia'
import {ref} from 'vue'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'
import type {RouteDirection} from '@/types/tranzy.ts'

const ROUTES_KEY = 'favorites:routes'
const STOPS_KEY = 'favorites:stops'
const PLANS_KEY = 'favorites:plans'
const RECENT_PLANS_KEY = 'recents:plans'

const MAX_RECENT_PLANS = 10

export interface FavoritePlan {
  name: string
  label?: string
  lat: number
  lon: number
  originName?: string
  originLat?: number
  originLon?: number
}

export interface FavoriteRoute {
  routeId: number
  direction: RouteDirection
}

function sameRoute(a: FavoriteRoute, b: FavoriteRoute): boolean {
  return a.routeId === b.routeId && a.direction === b.direction
}

function parseFavoriteRoutes(raw: unknown): FavoriteRoute[] {
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

export const useFavoritesStore = defineStore('favorites', () => {
  const favoriteRoutes = ref<FavoriteRoute[]>([])
  const favoriteStopIds = ref<number[]>([])
  const favoritePlans = ref<FavoritePlan[]>([])
  const recentPlans = ref<FavoritePlan[]>([])
  const isHydrated = ref(false)

  async function hydrate() {
    try {
      const routes = JSON.parse(localStorage.getItem(ROUTES_KEY) ?? 'null')
      const stops = JSON.parse(localStorage.getItem(STOPS_KEY) ?? 'null')
      const plans = JSON.parse(localStorage.getItem(PLANS_KEY) ?? 'null')
      const recents = JSON.parse(localStorage.getItem(RECENT_PLANS_KEY) ?? 'null')
      favoriteRoutes.value = parseFavoriteRoutes(routes)
      favoriteStopIds.value = Array.isArray(stops) ? stops : []
      favoritePlans.value = Array.isArray(plans) ? plans : []
      recentPlans.value = Array.isArray(recents) ? recents : []
    } catch (err) {
      console.warn('Failed to hydrate favorites:', err)
    } finally {
      isHydrated.value = true
    }
  }

  function persistRoutes() {
    localStorage.setItem(ROUTES_KEY, JSON.stringify(favoriteRoutes.value))
  }

  function persistStops() {
    localStorage.setItem(STOPS_KEY, JSON.stringify(favoriteStopIds.value))
  }

  function persistPlans() {
    localStorage.setItem(PLANS_KEY, JSON.stringify(favoritePlans.value))
  }

  function persistRecentPlans() {
    localStorage.setItem(RECENT_PLANS_KEY, JSON.stringify(recentPlans.value))
  }

  function isRouteFavorite(routeId: number, direction?: RouteDirection): boolean {
    return favoriteRoutes.value.some((r) =>
      r.routeId === routeId && (direction === undefined || r.direction === direction)
    )
  }

  function isStopFavorite(id: number): boolean {
    return favoriteStopIds.value.includes(id)
  }

  function isPlanFavorite(lat: number, lon: number, originLat?: number, originLon?: number): boolean {
    return favoritePlans.value.some(p =>
      p.lat === lat &&
      p.lon === lon &&
      p.originLat === originLat &&
      p.originLon === originLon
    )
  }

  function toggleRouteFavorite(routeId: number, direction: RouteDirection) {
    const fav = {routeId, direction}
    const idx = favoriteRoutes.value.findIndex((r) => sameRoute(r, fav))
    if (idx === -1) favoriteRoutes.value.push(fav)
    else favoriteRoutes.value.splice(idx, 1)
    persistRoutes()
  }

  function toggleStopFavorite(id: number) {
    const idx = favoriteStopIds.value.indexOf(id)
    if (idx === -1) favoriteStopIds.value.push(id)
    else favoriteStopIds.value.splice(idx, 1)
    persistStops()
  }

  function togglePlanFavorite(plan: FavoritePlan) {
    const idx = favoritePlans.value.findIndex(p =>
      p.lat === plan.lat &&
      p.lon === plan.lon &&
      p.originLat === plan.originLat &&
      p.originLon === plan.originLon
    )
    if (idx === -1) favoritePlans.value.push(plan)
    else favoritePlans.value.splice(idx, 1)
    persistPlans()
  }

  function renamePlanFavorite(lat: number, lon: number, originLat: number | undefined, originLon: number | undefined, label: string | undefined) {
    const plan = favoritePlans.value.find(p =>
      p.lat === lat && p.lon === lon && p.originLat === originLat && p.originLon === originLon
    )
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

  function addRecentPlan(plan: FavoritePlan) {
    const idx = recentPlans.value.findIndex(p =>
      p.lat === plan.lat &&
      p.lon === plan.lon &&
      p.originLat === plan.originLat &&
      p.originLon === plan.originLon
    )
    if (idx !== -1) recentPlans.value.splice(idx, 1)
    recentPlans.value.unshift(plan)
    if (recentPlans.value.length > MAX_RECENT_PLANS) {
      recentPlans.value.length = MAX_RECENT_PLANS
    }
    persistRecentPlans()
  }

  function removeRecentPlan(plan: FavoritePlan) {
    const idx = recentPlans.value.findIndex(p =>
      p.lat === plan.lat &&
      p.lon === plan.lon &&
      p.originLat === plan.originLat &&
      p.originLon === plan.originLon
    )
    if (idx === -1) return
    recentPlans.value.splice(idx, 1)
    persistRecentPlans()
  }

  function importAll(data: { routes?: unknown; stops?: unknown; plans?: unknown; recentPlans?: unknown }) {
    favoriteRoutes.value = parseFavoriteRoutes(data.routes)
    favoriteStopIds.value = Array.isArray(data.stops) ? data.stops.filter((x): x is number => typeof x === 'number') : []
    favoritePlans.value = Array.isArray(data.plans) ? data.plans as FavoritePlan[] : []
    recentPlans.value = Array.isArray(data.recentPlans) ? data.recentPlans as FavoritePlan[] : []
    persistRoutes()
    persistStops()
    persistPlans()
    persistRecentPlans()
  }

  async function preloadFavorites() {
    const jobs: Promise<unknown>[] = []

    if (favoriteRoutes.value.length) {
      jobs.push(preloadFavoriteRoutes())
    }
    if (favoriteStopIds.value.length) {
      const {fetchStopData} = useStopInfoApi()
      jobs.push(
        Promise.allSettled(favoriteStopIds.value.map((id) => fetchStopData(String(id)))),
      )
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

  return {
    favoriteRoutes,
    favoriteStopIds,
    favoritePlans,
    recentPlans,
    isHydrated,
    hydrate,
    isRouteFavorite,
    isStopFavorite,
    isPlanFavorite,
    toggleRouteFavorite,
    toggleStopFavorite,
    togglePlanFavorite,
    renamePlanFavorite,
    reorderRoutes,
    reorderStopIds,
    reorderPlans,
    addRecentPlan,
    removeRecentPlan,
    importAll,
    preloadFavorites,
  }
})

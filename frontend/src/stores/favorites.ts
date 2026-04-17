import {defineStore} from 'pinia'
import {ref} from 'vue'
import {get, set} from 'idb-keyval'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'

const ROUTES_KEY = 'favorites:routes'
const STOPS_KEY = 'favorites:stops'

export const useFavoritesStore = defineStore('favorites', () => {
  const favoriteRouteIds = ref<number[]>([])
  const favoriteStopIds = ref<number[]>([])
  const isHydrated = ref(false)

  async function hydrate() {
    try {
      const [routes, stops] = await Promise.all([
        get(ROUTES_KEY) as Promise<number[] | undefined>,
        get(STOPS_KEY) as Promise<number[] | undefined>,
      ])
      favoriteRouteIds.value = Array.isArray(routes) ? routes : []
      favoriteStopIds.value = Array.isArray(stops) ? stops : []
    } catch (err) {
      console.warn('Failed to hydrate favorites:', err)
    } finally {
      isHydrated.value = true
    }
  }

  async function persistRoutes() {
    try {
      await set(ROUTES_KEY, [...favoriteRouteIds.value])
    } catch (err) {
      console.warn('Failed to persist favorite routes:', err)
    }
  }

  async function persistStops() {
    try {
      await set(STOPS_KEY, [...favoriteStopIds.value])
    } catch (err) {
      console.warn('Failed to persist favorite stops:', err)
    }
  }

  function isRouteFavorite(id: number): boolean {
    return favoriteRouteIds.value.includes(id)
  }

  function isStopFavorite(id: number): boolean {
    return favoriteStopIds.value.includes(id)
  }

  function toggleRouteFavorite(id: number) {
    const idx = favoriteRouteIds.value.indexOf(id)
    if (idx === -1) favoriteRouteIds.value.push(id)
    else favoriteRouteIds.value.splice(idx, 1)
    void persistRoutes()
  }

  function toggleStopFavorite(id: number) {
    const idx = favoriteStopIds.value.indexOf(id)
    if (idx === -1) favoriteStopIds.value.push(id)
    else favoriteStopIds.value.splice(idx, 1)
    void persistStops()
  }

  function reorderRouteIds(newIds: number[]) {
    favoriteRouteIds.value = newIds
    void persistRoutes()
  }

  function reorderStopIds(newIds: number[]) {
    favoriteStopIds.value = newIds
    void persistStops()
  }

  /**
   * Warm the apiRequest IndexedDB cache for everything the user has starred,
   * so opening a favorite is instant. Fire-and-forget; per-request failures
   * are swallowed (e.g. routes whose CTP CSV is missing). Safe to call
   * concurrently with user navigation — the underlying composables dedupe
   * in-flight requests by route_id / stop_id.
   */
  async function preloadFavorites() {
    const jobs: Promise<unknown>[] = []

    if (favoriteRouteIds.value.length) {
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
    const wanted = new Set(favoriteRouteIds.value)
    const targets = routes.value.filter((r) => wanted.has(r.route_id))
    await Promise.allSettled(targets.map((r) => fetchShapeInfo(r)))
  }

  return {
    favoriteRouteIds,
    favoriteStopIds,
    isHydrated,
    hydrate,
    isRouteFavorite,
    isStopFavorite,
    toggleRouteFavorite,
    toggleStopFavorite,
    reorderRouteIds,
    reorderStopIds,
    preloadFavorites,
  }
})

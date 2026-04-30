import {defineStore} from 'pinia'
import {ref} from 'vue'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'

const ROUTES_KEY = 'favorites:routes'
const STOPS_KEY = 'favorites:stops'
const PLANS_KEY = 'favorites:plans'
const IDB_MIGRATION_KEY = 'favorites:idb-migrated'

export interface FavoritePlan {
  name: string
  lat: number
  lon: number
}

function idbGet(store: IDBObjectStore, key: string): Promise<unknown> {
  return new Promise((resolve) => {
    const req = store.get(key)
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => resolve(undefined)
  })
}

async function migrateFromIdb(): Promise<void> {
  if (localStorage.getItem(IDB_MIGRATION_KEY)) return

  try {
    const db = await new Promise<IDBDatabase | null>((resolve) => {
      const req = indexedDB.open('keyval-store')
      let isNew = false
      req.onupgradeneeded = () => {
        isNew = true
      }
      req.onsuccess = () => {
        if (isNew) {
          req.result.close()
          resolve(null)
        } else {
          resolve(req.result)
        }
      }
      req.onerror = () => resolve(null)
    })

    if (db && db.objectStoreNames.contains('keyval')) {
      const tx = db.transaction('keyval', 'readonly')
      const store = tx.objectStore('keyval')
      const [routes, stops] = await Promise.all([idbGet(store, ROUTES_KEY), idbGet(store, STOPS_KEY)])
      db.close()

      if (Array.isArray(routes) && routes.length > 0) {
        localStorage.setItem(ROUTES_KEY, JSON.stringify(routes))
      }
      if (Array.isArray(stops) && stops.length > 0) {
        localStorage.setItem(STOPS_KEY, JSON.stringify(stops))
      }
    }
  } catch (err) {
    console.warn('Failed to migrate favorites from IndexedDB:', err)
    return
  }

  localStorage.setItem(IDB_MIGRATION_KEY, '1')
}

export const useFavoritesStore = defineStore('favorites', () => {
  const favoriteRouteIds = ref<number[]>([])
  const favoriteStopIds = ref<number[]>([])
  const favoritePlans = ref<FavoritePlan[]>([])
  const isHydrated = ref(false)

  async function hydrate() {
    await migrateFromIdb()
    try {
      const routes = JSON.parse(localStorage.getItem(ROUTES_KEY) ?? 'null')
      const stops = JSON.parse(localStorage.getItem(STOPS_KEY) ?? 'null')
      const plans = JSON.parse(localStorage.getItem(PLANS_KEY) ?? 'null')
      favoriteRouteIds.value = Array.isArray(routes) ? routes : []
      favoriteStopIds.value = Array.isArray(stops) ? stops : []
      favoritePlans.value = Array.isArray(plans) ? plans : []
    } catch (err) {
      console.warn('Failed to hydrate favorites:', err)
    } finally {
      isHydrated.value = true
    }
  }

  function persistRoutes() {
    localStorage.setItem(ROUTES_KEY, JSON.stringify(favoriteRouteIds.value))
  }

  function persistStops() {
    localStorage.setItem(STOPS_KEY, JSON.stringify(favoriteStopIds.value))
  }

  function persistPlans() {
    localStorage.setItem(PLANS_KEY, JSON.stringify(favoritePlans.value))
  }

  function isRouteFavorite(id: number): boolean {
    return favoriteRouteIds.value.includes(id)
  }

  function isStopFavorite(id: number): boolean {
    return favoriteStopIds.value.includes(id)
  }

  function isPlanFavorite(lat: number, lon: number): boolean {
    return favoritePlans.value.some(p => p.lat === lat && p.lon === lon)
  }

  function toggleRouteFavorite(id: number) {
    const idx = favoriteRouteIds.value.indexOf(id)
    if (idx === -1) favoriteRouteIds.value.push(id)
    else favoriteRouteIds.value.splice(idx, 1)
    persistRoutes()
  }

  function toggleStopFavorite(id: number) {
    const idx = favoriteStopIds.value.indexOf(id)
    if (idx === -1) favoriteStopIds.value.push(id)
    else favoriteStopIds.value.splice(idx, 1)
    persistStops()
  }

  function togglePlanFavorite(plan: FavoritePlan) {
    const idx = favoritePlans.value.findIndex(p => p.lat === plan.lat && p.lon === plan.lon)
    if (idx === -1) favoritePlans.value.push(plan)
    else favoritePlans.value.splice(idx, 1)
    persistPlans()
  }

  function reorderRouteIds(newIds: number[]) {
    favoriteRouteIds.value = newIds
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
    favoritePlans,
    isHydrated,
    hydrate,
    isRouteFavorite,
    isStopFavorite,
    isPlanFavorite,
    toggleRouteFavorite,
    toggleStopFavorite,
    togglePlanFavorite,
    reorderRouteIds,
    reorderStopIds,
    reorderPlans,
    preloadFavorites,
  }
})

import {defineStore} from 'pinia'
import {ref} from 'vue'
import {get, set} from 'idb-keyval'

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

  return {
    favoriteRouteIds,
    favoriteStopIds,
    isHydrated,
    hydrate,
    isRouteFavorite,
    isStopFavorite,
    toggleRouteFavorite,
    toggleStopFavorite,
  }
})

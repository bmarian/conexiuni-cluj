import {ref, watch} from 'vue'
import type {Route} from '@/types/tranzy.ts'
import {readCachedList, writeCachedList} from '@/utils/api.ts'
import {currentCity} from '@/utils/city.ts'
import {network, networkLoaded, refreshNetwork} from '@/utils/network.ts'

const CACHE_KEY = 'cache:routes'

// Starts from the last session's list so favorites render before the network answers.
const routes = ref<Route[]>(readCachedList(CACHE_KEY))

watch(
  currentCity,
  () => {
    routes.value = readCachedList(CACHE_KEY)
  },
  {flush: 'sync'},
)

watch(network, (index) => {
  if (!index?.routes.length) return
  routes.value = index.routes
  writeCachedList(CACHE_KEY, routes.value)
})

export function useRoutesApi() {
  const isLoading = ref(false)
  const error = ref<unknown>(null)

  async function fetchRoutes() {
    isLoading.value = true
    try {
      await networkLoaded()
      if (!network.value) await refreshNetwork(true)
      if (!network.value && !routes.value.length) throw new Error('network bundle unavailable')
    } catch (e) {
      error.value = e
      console.error('Failed to fetch routes:', e)
    } finally {
      isLoading.value = false
    }
  }

  return {routes, isLoading, error, fetchRoutes}
}

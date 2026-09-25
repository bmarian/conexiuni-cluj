import {ref} from 'vue'
import type {Route} from '@/types/tranzy.ts'
import {apiRequest, readCachedList, writeCachedList} from '@/utils/api.ts'

const CACHE_KEY = 'cache:routes'

// Starts from the last session's list so favorites render before the network answers.
const routes = ref<Route[]>(readCachedList(CACHE_KEY))
let pending: Promise<void> | null = null

export function useRoutesApi() {
  const isLoading = ref(false)
  const error = ref<unknown>(null)

  async function fetchRoutes() {
    isLoading.value = true
    try {
      pending ??= (apiRequest('routes') as Promise<Route[]>).then((data) => {
        if (!Array.isArray(data) || !data.length) return
        routes.value = data
        writeCachedList(CACHE_KEY, data)
      })
      await pending
    } catch (e) {
      error.value = e
      console.error('Failed to fetch routes:', e)
      pending = null
    } finally {
      isLoading.value = false
    }
  }

  return {routes, isLoading, error, fetchRoutes}
}

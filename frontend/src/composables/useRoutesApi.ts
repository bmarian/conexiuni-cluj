import {ref} from 'vue'
import type {Route} from '@/types/tranzy.ts'
import {apiRequest, LOW_ACCURACY_SHELF_LIFE} from '@/utils/request_cache.ts'

let pending: Promise<Route[]> | null = null

export function useRoutesApi() {
  const routes = ref<Route[]>([])
  const isLoading = ref(false)
  const error = ref<unknown>(null)

  async function fetchRoutes() {
    isLoading.value = true
    try {
      if (!pending) {
        pending = apiRequest('routes', LOW_ACCURACY_SHELF_LIFE) as Promise<Route[]>
      }
      const data = await pending
      routes.value = Array.isArray(data) ? data : []
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

import {ref} from 'vue'
import type {Stop} from '@/types/tranzy.ts'
import {apiRequest} from '@/utils/request_cache.ts'

let pending: Promise<Stop[]> | null = null

export function useStopsApi() {
  const stops = ref<Stop[]>([])
  const isLoading = ref(false)
  const error = ref<unknown>(null)

  async function fetchStops() {
    isLoading.value = true
    try {
      if (!pending) {
        pending = apiRequest('stops') as Promise<Stop[]>
      }
      const data = await pending
      stops.value = Array.isArray(data) ? data : []
    } catch (e) {
      error.value = e
      console.error('Failed to fetch stops:', e)
      pending = null
    } finally {
      isLoading.value = false
    }
  }

  return {stops, isLoading, error, fetchStops}
}

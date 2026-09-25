import {ref} from 'vue'
import type {Stop} from '@/types/tranzy.ts'
import {apiRequest, readCachedList, writeCachedList} from '@/utils/api.ts'

const CACHE_KEY = 'cache:stops'

// Starts from the last session's list so favorites render before the network answers.
const stops = ref<Stop[]>(readCachedList(CACHE_KEY))
let pending: Promise<void> | null = null

export function useStopsApi() {
  const isLoading = ref(false)
  const error = ref<unknown>(null)

  async function fetchStops() {
    isLoading.value = true
    try {
      pending ??= (apiRequest('stops') as Promise<Stop[]>).then((data) => {
        if (!Array.isArray(data) || !data.length) return
        stops.value = data
        writeCachedList(CACHE_KEY, data)
      })
      await pending
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

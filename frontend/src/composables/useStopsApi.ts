import {ref, watch} from 'vue'
import type {Stop} from '@/types/tranzy.ts'
import {apiRequest, readCachedList, writeCachedList} from '@/utils/api.ts'
import {network, networkLoaded} from '@/utils/network.ts'

const CACHE_KEY = 'cache:stops'

// Starts from the last session's list so favorites render before the network answers.
const stops = ref<Stop[]>(readCachedList(CACHE_KEY))
let pending: Promise<void> | null = null

watch(network, (index) => {
  if (!index?.bundle.stops.length) return
  stops.value = index.bundle.stops
  writeCachedList(CACHE_KEY, stops.value)
})

export function useStopsApi() {
  const isLoading = ref(false)
  const error = ref<unknown>(null)

  async function fetchStops() {
    isLoading.value = true
    try {
      await networkLoaded
      if (network.value) return
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

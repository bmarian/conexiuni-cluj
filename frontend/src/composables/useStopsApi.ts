import {ref, watch} from 'vue'
import type {Stop} from '@/types/tranzy.ts'
import {readCachedList, writeCachedList} from '@/utils/api.ts'
import {currentCity} from '@/utils/city.ts'
import {network, networkLoaded, refreshNetwork} from '@/utils/network.ts'

const CACHE_KEY = 'cache:stops'

// Starts from the last session's list so favorites render before the network answers.
const stops = ref<Stop[]>(readCachedList(CACHE_KEY))

watch(
  currentCity,
  () => {
    stops.value = readCachedList(CACHE_KEY)
  },
  {flush: 'sync'},
)

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
      await networkLoaded()
      if (!network.value) await refreshNetwork(true)
      if (!network.value && !stops.value.length) throw new Error('network bundle unavailable')
    } catch (e) {
      error.value = e
      console.error('Failed to fetch stops:', e)
    } finally {
      isLoading.value = false
    }
  }

  return {stops, isLoading, error, fetchStops}
}

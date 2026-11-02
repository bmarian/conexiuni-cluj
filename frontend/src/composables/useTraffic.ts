import {computed, onScopeDispose, ref, watch} from 'vue'
import {storeToRefs} from 'pinia'
import {useMapStore} from '@/stores/map.ts'
import {useSettingsStore} from '@/stores/settings'
import {apiRequest} from '@/utils/api.ts'
import {cityInfo, currentCity} from '@/utils/city.ts'
import type {TrafficNow} from '@/utils/traffic.ts'

const MINUTE_MS = 60_000

// The traffic on the lines drawn on the map, while the setting is on and the city measures it.
export function useTraffic() {
  const {traffic} = storeToRefs(useSettingsStore())
  const {shapesToDisplay} = storeToRefs(useMapStore())
  const now = ref<TrafficNow | null>(null)
  const tripIds = computed(() => [...new Set(shapesToDisplay.value.map(([shape]) => shape.trip_id))].sort().join(','))
  const wanted = computed(() => (traffic.value && !!cityInfo()?.traffic ? tripIds.value : ''))
  let ticker: ReturnType<typeof setInterval> | null = null

  // Every minute: the stretches the last buses drove change with each bus.
  async function load() {
    const city = currentCity.value
    const ids = wanted.value
    try {
      const fetched = await apiRequest<TrafficNow>(`traffic?trip_ids=${ids}`)
      if (city === currentCity.value && ids === wanted.value) now.value = fetched
    } catch {
      now.value = null
    }
  }

  watch(
    wanted,
    (ids) => {
      if (ticker) clearInterval(ticker)
      ticker = null
      now.value = null
      if (!ids) return
      void load()
      ticker = setInterval(() => void load(), MINUTE_MS)
    },
    {immediate: true},
  )

  onScopeDispose(() => {
    if (ticker) clearInterval(ticker)
  })

  return {now}
}

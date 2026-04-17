import {onUnmounted, ref, watch, type Ref} from 'vue'
import type {Vehicle} from '@/types/tranzy.ts'

export function useVehicleStream(tripIds: Ref<string[]>) {
  const vehiclesByTrip = ref<Map<string, Vehicle[]>>(new Map())
  let es: EventSource | null = null
  let activeKey = ''

  const buildKey = (ids: string[]) => [...new Set(ids)].sort().join(',')

  function stop() {
    if (es) {
      es.close()
      es = null
    }
    activeKey = ''
  }

  function start(ids: string[]) {
    stop()
    if (!ids.length) {
      vehiclesByTrip.value = new Map()
      return
    }
    const key = buildKey(ids)
    activeKey = key
    const url = `/api/vehicles/stream?trip_ids=${encodeURIComponent(key)}`
    es = new EventSource(url)
    es.onmessage = (ev) => {
      try {
        const raw = JSON.parse(ev.data) as Vehicle[]
        const grouped = new Map<string, Vehicle[]>()
        for (const id of ids) grouped.set(id, [])
        for (const v of raw) {
          const bucket = grouped.get(v.trip_id)
          if (bucket) bucket.push(v)
        }
        vehiclesByTrip.value = grouped
      } catch (e) {
        console.warn('vehicle stream parse error:', e)
      }
    }
    es.onerror = () => {
    }
  }

  watch(
    tripIds,
    (ids) => {
      const key = buildKey(ids ?? [])
      if (key === activeKey) return
      // Keep stream closed while tab is hidden.
      if (typeof document !== 'undefined' && document.hidden) return
      start(ids ?? [])
    },
    {immediate: true, deep: true},
  )

  function onVisibility() {
    if (document.hidden) {
      stop()
    } else if (tripIds.value?.length) {
      start(tripIds.value)
    }
  }
  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', onVisibility)
  }

  onUnmounted(() => {
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', onVisibility)
    }
    stop()
  })

  return {vehiclesByTrip}
}

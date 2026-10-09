import {onUnmounted, ref, type Ref, watch} from 'vue'
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
    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      activeKey = buildKey(ids)
      return
    }
    const key = buildKey(ids)
    activeKey = key
    const url = `/api/vehicles/stream?v=2&trip_ids=${encodeURIComponent(key)}`
    es = new EventSource(url)
    const byId = new Map<number, Vehicle>()
    const publish = () => {
      const grouped = new Map<string, Vehicle[]>()
      for (const id of ids) grouped.set(id, [])
      for (const v of byId.values()) grouped.get(v.trip_id)?.push(v)
      vehiclesByTrip.value = grouped
    }
    es.addEventListener('snapshot', (ev) => {
      try {
        byId.clear()
        for (const v of (JSON.parse((ev as MessageEvent).data) as {vehicles: Vehicle[]}).vehicles) byId.set(v.id, v)
        publish()
      } catch (e) {
        console.warn('vehicle stream parse error:', e)
      }
    })
    es.addEventListener('delta', (ev) => {
      try {
        const delta = JSON.parse((ev as MessageEvent).data) as {upsert: Partial<Vehicle>[], remove: number[]}
        for (const v of delta.upsert) {
          if (v.id === undefined) continue
          byId.set(v.id, {...byId.get(v.id), ...v} as Vehicle)
        }
        for (const id of delta.remove) byId.delete(id)
        publish()
      } catch (e) {
        console.warn('vehicle stream parse error:', e)
      }
    })
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

  function onOnline() {
    if (tripIds.value?.length && !es && (typeof document === 'undefined' || !document.hidden)) {
      start(tripIds.value)
    }
  }

  function onOffline() {
    stop()
  }

  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', onVisibility)
  }
  if (typeof window !== 'undefined') {
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
  }

  onUnmounted(() => {
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', onVisibility)
    }
    if (typeof window !== 'undefined') {
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOffline)
    }
    stop()
  })

  return {vehiclesByTrip}
}

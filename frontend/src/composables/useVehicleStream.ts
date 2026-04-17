import {onUnmounted, ref, watch, type Ref} from 'vue'
import type {Vehicle} from '@/types/tranzy.ts'

/**
 * Live vehicle feed over SSE. Replaces per-tab polling: while the view is
 * mounted and visible, we keep one EventSource open; the backend hub runs
 * a single shared poll loop and pushes us filtered batches.
 *
 * - `tripIds` is reactive: when it changes (e.g. direction switch), we
 *   reconnect with the new filter. Sorted+deduped so same-set changes are
 *   no-ops.
 * - While the tab is hidden we close the connection entirely. When it comes
 *   back into focus we reopen. This is the "don't keep polling for a tab
 *   the user isn't looking at" behaviour the backend counts on.
 * - The browser's EventSource auto-reconnects on network blips; we don't
 *   manage retries ourselves.
 */
export function useVehicleStream(tripIds: Ref<string[]>) {
  const vehiclesByTrip = ref<Map<string, Vehicle[]>>(new Map())
  let es: EventSource | null = null
  // Stable join of current filter so we can no-op when watchers fire with
  // an identically-shaped array.
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
        // Group into the map layout the ETA code expects. Pre-seed every
        // trip so callers can always do `map.get(id) ?? []` without
        // undefined checks.
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
      // EventSource retries on its own — nothing to do. Logging on every
      // hiccup would be noisy during normal reconnects.
    }
  }

  watch(
    tripIds,
    (ids) => {
      const key = buildKey(ids ?? [])
      if (key === activeKey) return
      // When the tab is hidden we stay closed even if ids change; the
      // visibility handler will (re)open with the latest ids on return.
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

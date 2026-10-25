import {onUnmounted, ref, type Ref, watch} from 'vue'
import type {LiveEntry, LiveRow, LiveVehicle} from '@/types/tranzy.ts'
import {apiPath} from '@/utils/city.ts'

export const liveRowKey = (tripId: string, stopId: number) => `${tripId}@${stopId}`

type Snapshot = {as_of: number; rows: LiveRow[]; vehicles: LiveVehicle[]}
type Delta = {as_of: number; rows: LiveRow[]; vehicles: {upsert: LiveVehicle[]; remove: number[]}}

export type ShownEntry = LiveEntry & {minutes: number}

// The minutes a screen shows for each entry at nowMs: the only rounding in the chain.
export function entriesAt(row: LiveRow | undefined, nowMs: number): ShownEntry[] {
  if (!row) return []
  return row.entries.map((e) => ({...e, minutes: Math.max(0, Math.round((e.at * 1000 - nowMs) / 60_000))}))
}

// Kept current by the /live stream; offline, views fall back to the timetable.
export function useLive(stopIds: Ref<number[]>, tripIds: Ref<string[]>) {
  const rows = ref<Map<string, LiveRow>>(new Map())
  const vehiclesByTrip = ref<Map<string, LiveVehicle[]>>(new Map())
  const asOfMs = ref<number | null>(null)
  const connected = ref(false)
  let es: EventSource | null = null
  let activeKey = ''

  const buildKey = (stops: number[], trips: string[]) => `${[...new Set(stops)].sort((a, b) => a - b).join(',')}|${[...new Set(trips)].sort().join(',')}`

  function stop() {
    if (es) {
      es.close()
      es = null
    }
    connected.value = false
    activeKey = ''
  }

  function start(stops: number[], trips: string[]) {
    stop()
    if (!stops.length && !trips.length) {
      rows.value = new Map()
      vehiclesByTrip.value = new Map()
      asOfMs.value = null
      return
    }
    const key = buildKey(stops, trips)
    activeKey = key
    if (typeof navigator !== 'undefined' && !navigator.onLine) return
    const params = new URLSearchParams()
    if (stops.length) params.set('stop_ids', [...new Set(stops)].join(','))
    if (trips.length) params.set('trip_ids', [...new Set(trips)].join(','))
    es = new EventSource(`${apiPath('live')}?${params}`)
    const byId = new Map<number, LiveVehicle>()
    const publishVehicles = () => {
      const grouped = new Map<string, LiveVehicle[]>()
      for (const id of trips) grouped.set(id, [])
      for (const v of byId.values()) {
        const list = grouped.get(v.trip_id)
        if (list) list.push(v)
        else grouped.set(v.trip_id, [v])
      }
      vehiclesByTrip.value = grouped
    }
    es.addEventListener('snapshot', (ev) => {
      try {
        const snapshot = JSON.parse((ev as MessageEvent).data) as Snapshot
        rows.value = new Map(snapshot.rows.map((r) => [liveRowKey(r.trip_id, r.stop_id), r]))
        byId.clear()
        for (const v of snapshot.vehicles) byId.set(v.id, v)
        publishVehicles()
        asOfMs.value = snapshot.as_of * 1000
        connected.value = true
      } catch (e) {
        console.warn('live stream parse error:', e)
      }
    })
    es.addEventListener('delta', (ev) => {
      try {
        const delta = JSON.parse((ev as MessageEvent).data) as Delta
        const next = new Map(rows.value)
        for (const r of delta.rows) next.set(liveRowKey(r.trip_id, r.stop_id), r)
        rows.value = next
        for (const v of delta.vehicles.upsert) byId.set(v.id, v)
        for (const id of delta.vehicles.remove) byId.delete(id)
        publishVehicles()
        asOfMs.value = delta.as_of * 1000
        connected.value = true
      } catch (e) {
        console.warn('live stream parse error:', e)
      }
    })
    es.onerror = () => {
      connected.value = false
    }
  }

  watch(
    [stopIds, tripIds],
    ([stops, trips]) => {
      const key = buildKey(stops, trips)
      if (key !== activeKey) start(stops, trips)
    },
    {immediate: true, deep: true},
  )

  const onOnline = () => {
    const key = activeKey
    activeKey = ''
    if (key) start(stopIds.value, tripIds.value)
  }
  if (typeof window !== 'undefined') window.addEventListener('online', onOnline)

  onUnmounted(() => {
    stop()
    if (typeof window !== 'undefined') window.removeEventListener('online', onOnline)
  })

  return {rows, vehiclesByTrip, asOfMs, connected}
}

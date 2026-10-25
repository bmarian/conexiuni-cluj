import {watch, type Ref} from 'vue'
import type {VehiclesInStop} from '@/types/tranzy.ts'

export function useDueArrivals(departures: Ref<VehiclesInStop[]>, onDue: (shape: VehiclesInStop) => void) {
  let minutes = new Map<string, number>()
  watch(departures, (list) => {
    const next = new Map<string, number>()
    for (const shape of list) {
      const first = shape.next_times[0]
      if (!first?.is_live) continue
      const key = `${shape.trip_id}|${first.vehicle_id ?? ''}`
      next.set(key, first.minutes)
      if (first.minutes === 0 && (minutes.get(key) ?? 0) > 0) onDue(shape)
    }
    minutes = next
  })
}

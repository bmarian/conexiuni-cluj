import {computed, onUnmounted, ref, watchEffect, type Ref} from 'vue'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {useMapStore} from '@/stores/map.ts'
import {displayShapeOf, useGroupShapes} from '@/composables/useGroupShapes.ts'
import {liveRowKey, useLive} from '@/composables/useLive.ts'
import {SCHEDULE_HORIZON_MIN, withArrivals} from '@/composables/useStopDepartures.ts'
import type {StopInfo, VehiclesInStop} from '@/types/tranzy.ts'
import {buildStopInfo, network, type NetworkIndex} from '@/utils/network.ts'
import {cityTime} from '@/utils/cityClock.ts'
import {getAvailableBusesForStop} from '@/utils/time.ts'
import type {DisplayVehicle} from '@/utils/mapIcons.ts'
import {boardingStop, type GroupLine, tripOf} from '@/utils/lineGroups.ts'
import {pastStretch} from '@/utils/lineStretch.ts'

export function useLineGroup(lines: Ref<GroupLine[]>) {
  const {userLocation, userTime} = storeToRefs(useUserStore())
  const mapStore = useMapStore()
  const hidden = ref(new Set<number>())
  const shown = computed(() => lines.value.filter((l) => !hidden.value.has(l.routeId)))

  const toggleHidden = (routeId: number) => {
    const next = new Set(hidden.value)
    if (!next.delete(routeId)) next.add(routeId)
    hidden.value = next
  }

  // A fix that moves no boarding stop keeps the same list, or the stream restarts on each one.
  const boards = computed<(number | undefined)[]>((prev) => {
    const index = network.value
    const next = index ? lines.value.map((line) => boardingStop(index, line, userLocation.value)) : []
    return prev && prev.join() === next.join() ? prev : next
  })

  // Where the shown lines' times are from.
  watchEffect(() => {
    const stops = lines.value.flatMap((line, i) => {
      const stopId = boards.value[i]
      return stopId !== undefined && !hidden.value.has(line.routeId) ? [String(stopId)] : []
    })
    mapStore.setHighlightedStops([...new Set(stops)].map((stopId) => ({stopId, color: 'green'})))
  })

  const streamStops = computed(() => [...new Set(boards.value.filter((id) => id !== undefined))])
  // Every line streams, so hiding one doesn't restart the stream.
  const tripIds = computed(() => lines.value.map(tripOf))
  const {rows, vehiclesByTrip, asOfMs} = useLive(streamStops, tripIds)

  // Timetables change with the hour and with a new bundle.
  let infoHour = ''
  let infoIndex: NetworkIndex | null = null
  const infos = new Map<number, StopInfo | null>()
  function stopInfo(index: NetworkIndex, stopId: number, at: Date): StopInfo | null {
    const {date, hour} = cityTime(at)
    if (infoHour !== `${date} ${hour}` || infoIndex !== index) {
      infoHour = `${date} ${hour}`
      infoIndex = index
      infos.clear()
    }
    if (!infos.has(stopId)) infos.set(stopId, buildStopInfo(index, stopId, at))
    return infos.get(stopId)!
  }

  // Each line's next buses at its boarding stop; null once none is left today.
  const departures = computed<(VehiclesInStop | null)[]>(() => {
    const index = network.value
    const at = userTime.value || new Date()
    return lines.value.map((line, i) => {
      const stopId = boards.value[i]
      const info = index && stopId !== undefined ? stopInfo(index, stopId, at) : null
      const tripId = tripOf(line)
      const shape = info ? getAvailableBusesForStop(info, at, {maxMinutes: SCHEDULE_HORIZON_MIN, tripId})[0] : undefined
      if (!shape || stopId === undefined) return null
      const row = asOfMs.value === null ? undefined : rows.value.get(liveRowKey(tripId, stopId))
      return withArrivals({...shape, stop_id: stopId}, row, at.getTime())
    })
  })

  const {spans} = useGroupShapes(shown)

  watchEffect(() => {
    const vehicles: DisplayVehicle[] = []
    for (const line of shown.value) {
      const meta = displayShapeOf(network.value, line)
      if (!meta) continue
      const span = spans.value.get(line.routeId)
      for (const v of vehiclesByTrip.value.get(meta.trip_id) ?? []) {
        if (span && pastStretch(span, v)) continue
        vehicles.push({...v, route_short_name: meta.route_short_name, route_color: meta.route_color, faded: v.at_stand})
      }
    }
    mapStore.setVehiclesToDisplay(vehicles)
  })

  onUnmounted(() => {
    mapStore.setHighlightedStops([])
    mapStore.setVehiclesToDisplay([])
  })

  return {hidden, toggleHidden, boards, departures, asOfMs}
}

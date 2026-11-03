import {computed, onUnmounted, watch, watchEffect, type Ref} from 'vue'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {type DisplayShape, type HighlightedStop, useMapStore} from '@/stores/map.ts'
import {liveRowKey, useLive} from '@/composables/useLive.ts'
import {DEPARTURES_SHOWN, SCHEDULE_HORIZON_MIN} from '@/composables/useStopDepartures.ts'
import type {StopInfo} from '@/types/tranzy.ts'
import {buildStopInfo, network, type NetworkIndex} from '@/utils/network.ts'
import {cityTime} from '@/utils/cityClock.ts'
import {getAvailableBusesForStop} from '@/utils/time.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {WALKING_SPEED_M_PER_MIN} from '@/utils/trips.ts'
import type {DisplayVehicle} from '@/utils/mapIcons.ts'
import {type Departure, type GroupLine, type Leg, type LegLine, type LegTimes, legsNear, stopsOf, timeLegs, tripOf} from '@/utils/lineGroups.ts'

const legStops = (leg: Leg) => [...leg.lines, ...(leg.alight ?? [])].map((l) => l.stopId)

export function useLineGroup(lines: Ref<GroupLine[]>) {
  const {userTime, userLocation} = storeToRefs(useUserStore())
  const mapStore = useMapStore()
  const now = () => (userTime.value ?? new Date()).getTime()

  const legs = computed<Leg[]>(() => (network.value ? legsNear(network.value, lines.value, userLocation.value) : []))

  const streamStops = computed(() => [...new Set(legs.value.flatMap(legStops))])
  const tripIds = computed(() => lines.value.map(tripOf))
  const {rows, vehiclesByTrip, asOfMs} = useLive(streamStops, tripIds)

  // Timetables change with the hour.
  let infoHour = ''
  const infos = new Map<number, StopInfo | null>()
  function stopInfo(index: NetworkIndex, stopId: number, at: Date): StopInfo | null {
    const {date, hour} = cityTime(at)
    if (infoHour !== `${date} ${hour}`) {
      infoHour = `${date} ${hour}`
      infos.clear()
    }
    if (!infos.has(stopId)) infos.set(stopId, buildStopInfo(index, stopId, at))
    return infos.get(stopId)!
  }

  function busesAt(line: LegLine): Departure[] {
    const index = network.value
    if (!index) return []
    const tripId = tripOf(line)
    const row = asOfMs.value === null ? undefined : rows.value.get(liveRowKey(tripId, line.stopId))
    if (row) return row.entries.map((e) => ({routeId: line.routeId, at: e.at * 1000, live: e.live, vehicleId: e.vehicle_id}))
    const at = new Date(now())
    const info = stopInfo(index, line.stopId, at)
    const here = info ? getAvailableBusesForStop(info, at, {maxMinutes: SCHEDULE_HORIZON_MIN, tripId}) : []
    return (here[0]?.next_times ?? []).map((time) => ({routeId: line.routeId, at: at.getTime() + time.minutes * 60_000, live: false}))
  }

  function walkMs(from: number, to: number): number {
    const a = network.value?.stopById.get(from)
    const b = network.value?.stopById.get(to)
    if (from === to || !a || !b) return 0
    return (haversineMeters(a.stop_lat, a.stop_lon, b.stop_lat, b.stop_lon) / WALKING_SPEED_M_PER_MIN) * 60_000
  }

  const times = computed<LegTimes[]>(() => {
    void userTime.value
    return timeLegs(legs.value, busesAt, busesAt, walkMs, DEPARTURES_SHOWN)
  })

  const displayShapes = computed<DisplayShape[]>(() =>
    lines.value.flatMap((line) => {
      const route = network.value?.routeById.get(line.routeId)
      return route
        ? [
            {
              trip_id: tripOf(line),
              route_short_name: route.route_short_name,
              route_long_name: route.route_long_name,
              route_color: route.route_color,
              route_type: route.route_type,
            },
          ]
        : []
    }),
  )

  let drawnKey = ''
  watch(
    displayShapes,
    async (shapes) => {
      const key = shapes.map((s) => s.trip_id).join(',')
      if (key === drawnKey) return
      drawnKey = key
      const loaded = await mapStore.requestShapes(shapes)
      if (key !== drawnKey) return
      mapStore.setLoadedShapes(loaded.filter(([, points]) => points.length))
      mapStore.zoomOut = true
    },
    {immediate: true},
  )

  watchEffect(() => {
    const vehicles: DisplayVehicle[] = []
    for (const shape of displayShapes.value) {
      for (const v of vehiclesByTrip.value.get(shape.trip_id) ?? []) {
        vehicles.push({...v, route_short_name: shape.route_short_name, route_color: shape.route_color, faded: v.at_stand})
      }
    }
    mapStore.setVehiclesToDisplay(vehicles)
  })

  watchEffect(() => {
    const index = network.value
    const marked = new Set(legs.value.flatMap(legStops).map(String))
    const highlights = new Map<string, HighlightedStop>()
    for (const line of lines.value) {
      for (const id of index ? stopsOf(index, line) : []) {
        const stopId = String(id)
        if (!highlights.has(stopId)) highlights.set(stopId, {stopId, color: marked.has(stopId) ? 'green' : 'gray'})
      }
    }
    mapStore.setHighlightedStops([...highlights.values()])
  })

  onUnmounted(() => {
    mapStore.setHighlightedStops([])
    mapStore.setShapesToDisplay([])
    mapStore.setVehiclesToDisplay([])
  })

  return {legs, times, asOfMs}
}

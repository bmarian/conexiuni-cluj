import {computed, onMounted, onUnmounted, ref, watch, watchEffect, type Ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import {useRouter} from 'vue-router'
import {useRouteStore} from '@/stores/route.ts'
import {useUserStore} from '@/stores/user.ts'
import {useMapStore} from '@/stores/map.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type Shape, type StopTime} from '@/types/tranzy.ts'
import {formatMinutesFromNow, getMinutesFromDate, getTimetableForDay, timeStringToMinutes} from '@/utils/time.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {getShapeStopTimes} from '@/utils/trips.ts'
import {markRepeatedNow, mergeArrivals, scheduledArrivals} from '@/utils/arrivals.ts'
import {
  buildShapeIndex,
  buildStopShapeIdxByStopId,
  etaForStop,
  getIndexedVehicles,
  type IndexedVehicle,
  type ShapeIndex,
} from '@/composables/useVehicleTracking.ts'
import {useVehicleStream} from '@/composables/useVehicleStream.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'

type DirectionShape = {
  shape: Shape[]
  shapeIndex: ShapeIndex
  stopShapeIdxByStopId: Map<number, number>
}

export type IndexedStop = StopTime & { timeOffsetFromStart: number }

export interface StopTimeDisplay {
  label: string;
  isLive: boolean
  minutes: number
}

const SCHEDULE_HORIZON_MIN = 480

export function useRouteArrivals(routeId: Ref<string>, direction: Ref<string>) {
  const {t} = useI18n()
  const routeStore = useRouteStore()
  const userStore = useUserStore()
  const mapStore = useMapStore()
  const favoritesStore = useFavoritesStore()
  const router = useRouter()
  const {userTime, userLocation} = storeToRefs(userStore)
  const {zoomOut} = storeToRefs(mapStore)
  const {favoriteStopIds} = storeToRefs(favoritesStore)

  const shapeInfo = computed(() => routeStore.selectedShapeInfo)
  const fromStopId = computed(() => routeStore.fromStopId)
  const fromStopName = computed(() => routeStore.fromStopName)

  // 'auto' (from homepage/search) opens the direction whose nearest stop is closest
  // to the user. Resolved synchronously so the first paint already shows it.
  function resolveDirection(): '0' | '1' {
    if (direction.value === '1') return '1'
    if (direction.value !== 'auto') return '0'
    const loc = userLocation.value
    const info = routeStore.selectedShapeInfo
    if (!loc || !info || info.route_id !== Number(routeId.value)) return '0'
    const outId = `${routeId.value}${OUTGOING_SUFFIX}`
    const inId = `${routeId.value}${INCOMING_SUFFIX}`
    let bestDir: '0' | '1' = '0'
    let bestDist = Infinity
    for (const st of getShapeStopTimes(info)) {
      const dir = st.trip_id === outId ? '0' : st.trip_id === inId ? '1' : null
      if (!dir || !st.stop_lat || !st.stop_lon) continue
      const d = haversineMeters(loc.latitude, loc.longitude, st.stop_lat, st.stop_lon)
      if (d < bestDist) {
        bestDist = d
        bestDir = dir
      }
    }
    return bestDir
  }

  const currentDirection = ref<'0' | '1'>(resolveDirection())
  const isOutgoing = computed(() => currentDirection.value === '0')
  const currentTripId = computed(() =>
    `${routeId.value}${currentDirection.value === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`
  )

  const timetable = computed(() => shapeInfo.value?.timetable)
  const routeDisplayName = computed(() => {
    const tt = timetable.value
    if (!tt) return shapeInfo.value?.route_short_name || ''
    return isOutgoing.value ? tt.route_long_name : `${tt.out_stop_name} - ${tt.in_stop_name}`
  })

  const direction0Shape = ref<DirectionShape | null>(null)
  const direction1Shape = ref<DirectionShape | null>(null)
  const direction0Vehicles = ref<IndexedVehicle[]>([])
  const direction1Vehicles = ref<IndexedVehicle[]>([])

  const currentDirectionShape = computed(() =>
    currentDirection.value === '0' ? direction0Shape.value : direction1Shape.value
  )
  const currentDirectionVehicles = computed(() =>
    currentDirection.value === '0' ? direction0Vehicles.value : direction1Vehicles.value
  )

  const rawStops = computed((): StopTime[] => getShapeStopTimes(shapeInfo.value))

  const stopsForDirection = computed((): IndexedStop[] => {
    const filtered = rawStops.value
      .filter((st) => st.trip_id === currentTripId.value)
      .sort((a, b) => a.stop_sequence - b.stop_sequence)
    // offset_arrival_time is the ride from the previous stop into this one, so it has
    // to be added before the offset is read - otherwise every stop shows the previous
    // stop's arrival time and the last segment never counts at all.
    let cumulativeSec = 0
    return filtered.map((st) => {
      cumulativeSec += st.offset_arrival_time
      return {...st, timeOffsetFromStart: Math.ceil(cumulativeSec / 60)}
    })
  })

  const directionTerminals = computed(() => {
    const byTrip = (tripId: string): { first: string; last: string } => {
      const tripStops = rawStops.value
        .filter((st) => st.trip_id === tripId)
        .sort((a, b) => a.stop_sequence - b.stop_sequence)
      const first = tripStops[0]?.stop_headsign?.trim() ?? ''
      const last = tripStops[tripStops.length - 1]?.stop_headsign?.trim() ?? ''
      return {first, last}
    }
    return {
      outgoing: byTrip(`${routeId.value}${OUTGOING_SUFFIX}`),
      incoming: byTrip(`${routeId.value}${INCOMING_SUFFIX}`),
    }
  })

  const hasOutgoing = computed(() =>
    rawStops.value.some((st) => st.trip_id === `${routeId.value}${OUTGOING_SUFFIX}`)
  )
  const hasIncoming = computed(() =>
    rawStops.value.some((st) => st.trip_id === `${routeId.value}${INCOMING_SUFFIX}`)
  )

  const nearestStopIdx = computed(() => {
    const loc = userLocation.value
    if (!loc || !stopsForDirection.value.length) return -1
    let best = -1, bestDist = Infinity
    stopsForDirection.value.forEach((stop, idx) => {
      if (!stop.stop_lat || !stop.stop_lon) return
      const d = haversineMeters(loc.latitude, loc.longitude, stop.stop_lat, stop.stop_lon)
      if (d < bestDist) {
        bestDist = d;
        best = idx
      }
    })
    return best
  })

  const currentMinutes = computed(() => getMinutesFromDate(userTime.value || new Date()))

  function formatMinutes(minutes: number): string {
    return formatMinutesFromNow(minutes, userTime.value || new Date(), t('now'))
  }

  const departureTimes = computed((): number[] => {
    const tt = timetable.value
    if (!tt) return []
    const sched = getTimetableForDay(tt, userTime.value || new Date())
    if (!sched?.entries?.length) return []
    return sched.entries
      .map((e) => timeStringToMinutes(isOutgoing.value ? e.departure_in : e.departure_out))
      .filter((v): v is number => v !== null)
  })

  // Offset first, then sort, so runs already under way still count for later stops.
  function nextArrivalsAtStop(offsetFromStart: number): number[] {
    return scheduledArrivals({
      departureMinutes: departureTimes.value,
      offsetMinutes: offsetFromStart,
      nowMinutes: currentMinutes.value,
      horizonMinutes: SCHEDULE_HORIZON_MIN,
    })
  }

  // Whether live tracking has anything to say about this direction at all. It is the
  // difference between "no bus is behind this stop" and "we are not watching", and only
  // the first of those justifies dropping a timetable slot that reads as due now.
  const directionIsTracked = computed(() => currentDirectionVehicles.value.length > 0)

  function liveMinutesForStop(stop: IndexedStop): number | null {
    const dirShape = currentDirectionShape.value
    if (!dirShape) return null
    const stopIdx = dirShape.stopShapeIdxByStopId.get(stop.stop_id)
    if (stopIdx === undefined || stopIdx < 0) return null
    const eta = etaForStop(stopIdx, currentDirectionVehicles.value, dirShape.shapeIndex, {
      tripStops: stopsForDirection.value,
      targetStopId: stop.stop_id,
      referenceTime: userTime.value,
    })
    return eta ? eta.etaMinutes : null
  }

  // The live estimate used to be dropped into slot 0 and the timetable kept the rest,
  // which left the same bus counted twice and made a column mean a different thing on
  // every row. Merging on time instead keeps the columns comparable down the list.
  const stopTimesByStop = computed((): StopTimeDisplay[][] => {
    const rows = stopsForDirection.value.map((stop) =>
      mergeArrivals(liveMinutesForStop(stop), nextArrivalsAtStop(stop.timeOffsetFromStart), {tracked: directionIsTracked.value}))
    return markRepeatedNow(rows).map((row) => row.map((arrival) => ({
      label: arrival.soon ? t('soon') : formatMinutes(arrival.minutes),
      isLive: arrival.isLive,
      minutes: arrival.minutes,
    })))
  })

  // The header lists departures from the terminus, where there is no stop to have been
  // passed, so it stays on the timetable alone.
  function getHeaderTimes(): string[] {
    const offset = stopsForDirection.value[0]?.timeOffsetFromStart ?? 0
    return mergeArrivals(null, nextArrivalsAtStop(offset)).map((arrival) => formatMinutes(arrival.minutes))
  }

  function buildDisplayShape() {
    return {
      trip_id: currentTripId.value,
      route_short_name: shapeInfo.value!.route_short_name,
      route_long_name: routeDisplayName.value,
      route_color: shapeInfo.value!.route_color,
      route_type: shapeInfo.value!.route_type,
    }
  }

  function updateMap() {
    if (!shapeInfo.value) return
    const dirShape = currentDirectionShape.value
    const meta = buildDisplayShape()
    if (dirShape) {
      mapStore.setLoadedShapes([[meta, dirShape.shape]])
    } else {
      void mapStore.setShapesToDisplay([meta])
    }
    mapStore.setVehiclesToDisplay(currentDirectionVehicles.value)
    zoomOut.value = true
  }

  watchEffect(() => {
    const highlights: Array<{ stopId: string; color: 'green' | 'purple' | 'red' | 'gray' }> = []
    stopsForDirection.value.forEach((stop, idx) => {
      const stopId = String(stop.stop_id)
      if (stopId === fromStopId.value) highlights.push({stopId, color: 'green'})
      else if (idx === nearestStopIdx.value) highlights.push({stopId, color: 'purple'})
      else if (favoriteStopIds.value.includes(stop.stop_id)) highlights.push({stopId, color: 'red'})
      else highlights.push({stopId, color: 'gray'})
    })
    mapStore.setHighlightedStops(highlights)
  })

  const streamTripIds = computed<string[]>(() => {
    const ids: string[] = []
    if (direction0Shape.value) ids.push(`${routeId.value}${OUTGOING_SUFFIX}`)
    if (direction1Shape.value) ids.push(`${routeId.value}${INCOMING_SUFFIX}`)
    return ids
  })
  const {vehiclesByTrip} = useVehicleStream(streamTripIds)

  async function loadDirectionShape(dir: '0' | '1'): Promise<DirectionShape | null> {
    if (!shapeInfo.value) return null
    const tripId = `${routeId.value}${dir === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`
    try {
      const shapeData = await mapStore.requestShapes([{
        trip_id: tripId,
        route_short_name: shapeInfo.value.route_short_name,
        route_long_name: routeDisplayName.value,
        route_color: shapeInfo.value.route_color,
        route_type: shapeInfo.value.route_type,
      }])
      const shape = shapeData[0]?.[1] ?? []
      if (!shape.length) return null
      const shapeIndex = buildShapeIndex(shape)
      const tripStops = rawStops.value.filter((st) => st.trip_id === tripId)
      const stopShapeIdxByStopId = buildStopShapeIdxByStopId(tripStops, shape)
      return {shape, shapeIndex, stopShapeIdxByStopId}
    } catch (e) {
      console.warn(`Failed to load direction ${dir} shape:`, e)
      return null
    }
  }

  async function loadAllDirections() {
    const [d0, d1] = await Promise.all([loadDirectionShape('0'), loadDirectionShape('1')])
    direction0Shape.value = d0
    direction1Shape.value = d1
  }

  async function refreshVehiclesFromStream() {
    if (!shapeInfo.value) return
    const name = shapeInfo.value.route_short_name
    const color = shapeInfo.value.route_color
    const byTrip = vehiclesByTrip.value

    const refreshDirection = async (
      direction: '0' | '1',
      directionShape: DirectionShape | null,
      setVehicles: (vehicles: IndexedVehicle[]) => void,
      directionLabel: 'outgoing' | 'incoming',
    ) => {
      if (!directionShape) return
      const tid = `${routeId.value}${direction === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`
      try {
        setVehicles(await getIndexedVehicles(tid, name, color, directionShape.shapeIndex, userTime.value, byTrip.get(tid) ?? []))
      } catch (e) {
        console.warn(`Failed to index ${directionLabel} vehicles:`, e)
      }
    }

    await Promise.all([
      refreshDirection('0', direction0Shape.value, (vehicles) => {
        direction0Vehicles.value = vehicles
      }, 'outgoing'),
      refreshDirection('1', direction1Shape.value, (vehicles) => {
        direction1Vehicles.value = vehicles
      }, 'incoming'),
    ])

    mapStore.setVehiclesToDisplay(currentDirectionVehicles.value)
  }

  watch(vehiclesByTrip, () => {
    void refreshVehiclesFromStream()
  }, {deep: true})
  watch(currentDirection, () => {
    updateMap()
  })

  const isInitialLoading = ref(!shapeInfo.value || shapeInfo.value.route_id !== Number(routeId.value))

  async function loadShapeInfoFromApi(): Promise<boolean> {
    const {routes, fetchRoutes} = useRoutesApi()
    const {fetchShapeInfo} = useRouteShapeInfoApi()
    await fetchRoutes()
    const route = routes.value.find((r) => r.route_id === Number(routeId.value))
    if (!route) return false
    try {
      const loaded = await fetchShapeInfo(route)
      routeStore.setSelectedRoute(loaded, currentTripId.value, '', '')
      return true
    } catch (e) {
      console.error('Failed to load route shape info:', e)
      return false
    }
  }

  onMounted(async () => {
    mapStore.directionArrowAtStart = true
    const storeRouteId = shapeInfo.value?.route_id
    if (!shapeInfo.value || storeRouteId !== Number(routeId.value)) {
      isInitialLoading.value = true
      const ok = await loadShapeInfoFromApi()
      // Deep-linked 'auto': stops load async, so resolve once they arrive (still on
      // the skeleton, so no flicker).
      if (ok && direction.value === 'auto') currentDirection.value = resolveDirection()
      isInitialLoading.value = false
      if (!ok) return
    }
    // Swap the transient 'auto' in the URL for the resolved direction.
    if (direction.value !== currentDirection.value) {
      void router.replace({name: 'route', params: {routeId: routeId.value, direction: currentDirection.value}})
    }
    updateMap()
    void loadAllDirections().then(() => {
      updateMap()
    })
  })

  onUnmounted(() => {
    mapStore.directionArrowAtStart = false
    mapStore.setHighlightedStops([])
    mapStore.setShapesToDisplay([])
    mapStore.setVehiclesToDisplay([])
  })

  return {
    shapeInfo,
    fromStopId,
    fromStopName,
    currentDirection,
    isOutgoing,
    currentTripId,
    timetable,
    routeDisplayName,
    stopsForDirection,
    directionTerminals,
    hasOutgoing,
    hasIncoming,
    nearestStopIdx,
    currentMinutes,
    formatMinutes,
    stopTimesByStop,
    getHeaderTimes,
    isInitialLoading,
  }
}

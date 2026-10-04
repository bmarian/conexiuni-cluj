import {computed, onMounted, onUnmounted, ref, watch, type Ref} from 'vue'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {type DisplayShape, useMapStore} from '@/stores/map.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'
import {useVehicleStream} from '@/composables/useVehicleStream.ts'
import {
  buildShapeIndex,
  buildStopShapeIdxByStopId,
  etaForStop,
  getIndexedVehicles,
  type IndexedVehicle
} from '@/composables/useVehicleTracking.ts'
import type {Shape, ShapeInfo, Vehicle, VehiclesInStop} from '@/types/tranzy.ts'
import {getAvailableBusesForStop, hasTimetableEntries} from '@/utils/time.ts'
import {mergeArrivals} from '@/utils/arrivals.ts'
import {getDirectionFromTripId, getRouteIdFromTripId, getShapeStopTimes} from '@/utils/trips.ts'

// Pills per card. One more than this is asked of the timetable, because merging in a
// live estimate consumes the slot that same run occupies - without the spare the card
// would come up a time short whenever tracking has something to say.
const DEPARTURES_SHOWN = 3

export function useStopDepartures(stopId: Ref<string>) {
  const userStore = useUserStore()
  const mapStore = useMapStore()
  const favoritesStore = useFavoritesStore()
  const stopIdNum = computed(() => Number(stopId.value))

  const {userTime} = storeToRefs(userStore)
  const {zoomOut} = storeToRefs(mapStore)
  const {stopInfo, fetchStopData} = useStopInfoApi()
  const stopName = computed(() => stopInfo.value?.stop_name)

  const isLoading = ref(false)
  const loadError = ref(false)
  const isComputingDepartures = ref(true)
  const shapesComingToTheStopBasedOnVehiclePositions = ref<VehiclesInStop[]>([])
  const initialZoomAppliedStopId = ref<string | null>(null)
  const cachedShapeKey = ref('')
  const cachedShapesWithTrip = ref<Array<[DisplayShape, Shape[]]>>([])
  const displayedFavoriteShapeKey = ref<string | null>(null)

  const applyInitialZoomOutForCurrentStop = (shouldZoomOut: boolean) => {
    const loadedStopId = stopInfo.value?.stop_id?.toString()
    if (!loadedStopId || loadedStopId !== stopId.value) return
    if (initialZoomAppliedStopId.value === stopId.value) return
    if (!shouldZoomOut) return

    zoomOut.value = true
    initialZoomAppliedStopId.value = stopId.value
  }

  const setFavoriteRouteShapes = (entries: Array<[DisplayShape, Shape[]]>) => {
    const key = entries.map(([displayShape]) => displayShape.trip_id).sort().join(',')
    if (key === displayedFavoriteShapeKey.value) return
    displayedFavoriteShapeKey.value = key
    mapStore.setLoadedShapes(entries)
  }

  const streamTripIds = computed<string[]>(() => {
    const info = stopInfo.value
    if (!info) return []
    const shapes = info.shapes_info.filter((s: ShapeInfo) => hasTimetableEntries(s.timetable))
    const routeIds = new Set(shapes.map((s: ShapeInfo) => s.route_id))
    return [...(info.outgoing_trip_ids || []), ...(info.incoming_trip_ids || [])]
      .filter((tid) => {
        const tripRouteId = getRouteIdFromTripId(tid)
        return tripRouteId !== null && routeIds.has(tripRouteId)
      })
  })
  const {vehiclesByTrip} = useVehicleStream(streamTripIds)

  const busesWithAvailableTimetables = computed(() => {
    return stopInfo.value?.shapes_info.filter((shape: ShapeInfo) => hasTimetableEntries(shape.timetable))
  })

  const shapeInfoByRouteId = computed(() => {
    const info = stopInfo.value
    if (!info) return new Map<number, ShapeInfo>()
    return new Map<number, ShapeInfo>(info.shapes_info.map((shapeInfo: ShapeInfo) => [shapeInfo.route_id, shapeInfo]))
  })

  function isDepartureFavorite(shape: VehiclesInStop): boolean {
    return favoritesStore.isRouteFavorite(shape.route_id, getDirectionFromTripId(shape.trip_id))
  }

  const shapesComingToTheStopBasedOnTimetable = computed(() => {
    if (!stopInfo.value) return []
    return getAvailableBusesForStop(
      stopInfo.value,
      userTime.value || new Date(),
      {maxMinutes: 480, limit: DEPARTURES_SHOWN + 1}
    )
  })

  // The live estimate used to be spliced over next_times[0] with the rest of the
  // timetable left in place, so the tracked bus was also still listed as a scheduled run
  // a few minutes off. Reconciling on time drops that duplicate; and when tracking is
  // watching this trip but has no bus behind the stop, a slot the timetable still reads
  // as due now is a run that has already gone past.
  function withArrivals(shape: VehiclesInStop, liveMinutes: number | null, tracked: boolean): VehiclesInStop | null {
    const scheduled = (shape.next_times ?? []).map((entry) => entry.minutes)
    const merged = mergeArrivals(liveMinutes, scheduled, {tracked, limit: DEPARTURES_SHOWN})
    if (!merged.length) return null
    return {
      ...shape,
      minutes_left: merged[0]!.minutes,
      next_times: merged.map((arrival) => ({minutes: arrival.minutes, is_live: arrival.isLive})),
      static_time_approximation: liveMinutes === null,
    }
  }

  watch(stopId, async (newValue) => {
    isLoading.value = true
    loadError.value = false
    isComputingDepartures.value = true
    shapesComingToTheStopBasedOnVehiclePositions.value = []
    cachedShapeKey.value = ''
    cachedShapesWithTrip.value = []
    displayedFavoriteShapeKey.value = null
    await fetchStopData(newValue)
    if (!stopInfo.value) loadError.value = true
    isLoading.value = false
  }, {immediate: true})

  watch([shapesComingToTheStopBasedOnTimetable, vehiclesByTrip], async ([shapesComingNext]) => {
    if (!Array.isArray(shapesComingNext) || shapesComingNext.length === 0) {
      shapesComingToTheStopBasedOnVehiclePositions.value = []
      mapStore.setVehiclesToDisplay([])
      setFavoriteRouteShapes([])
      isComputingDepartures.value = false
      return
    }

    const displayShapes = getShapesDisplay(busesWithAvailableTimetables.value)
    const shapeKey = displayShapes.map(d => d.trip_id).sort().join(',')
    if (shapeKey !== cachedShapeKey.value) {
      cachedShapesWithTrip.value = await mapStore.requestShapes(displayShapes)
      cachedShapeKey.value = shapeKey
    }
    const displayShapesWithTrip = cachedShapesWithTrip.value
    if (!Array.isArray(displayShapesWithTrip) || displayShapesWithTrip.length === 0) {
      shapesComingToTheStopBasedOnVehiclePositions.value = shapesComingNext
        .map((shape) => withArrivals(shape, null, false))
        .filter((shape): shape is VehiclesInStop => shape !== null)
      mapStore.setVehiclesToDisplay([])
      setFavoriteRouteShapes([])
      isComputingDepartures.value = false
      return
    }

    const vehiclesByTripMap = vehiclesByTrip.value
    const favoriteVehicles: IndexedVehicle[] = []
    const favoriteTripIds = new Set<string>()

    const results: VehiclesInStop[] = []
    const push = (shape: VehiclesInStop, liveMinutes: number | null, tracked: boolean) => {
      const withTimes = withArrivals(shape, liveMinutes, tracked)
      if (withTimes) results.push(withTimes)
    }

    for (const shape of shapesComingNext) {
      const trip = displayShapesWithTrip.find(([s]) => s.trip_id === shape.trip_id)?.[1] as Shape[]
      if (!Array.isArray(trip) || !trip.length) {
        push(shape, null, false)
        continue
      }

      const shapeIndex = buildShapeIndex(trip)
      const vehiclesOnRoute = await getIndexedVehicles(
        shape.trip_id,
        shape.route_short_name,
        shape.route_color,
        shapeIndex,
        userTime.value,
        vehiclesByTripMap.get(shape.trip_id) ?? [],
      )

      if (isDepartureFavorite(shape)) {
        favoriteVehicles.push(...vehiclesOnRoute)
        if (vehiclesOnRoute.length) favoriteTripIds.add(shape.trip_id)
      }

      // No early-out on an empty vehicle list: etaForStop can still serve the last
      // estimate while Tranzy skips a poll, which is what stopped the row flipping
      // between its live and timetable values every few seconds.
      const tracked = vehiclesOnRoute.length > 0
      const routeShapeInfo = shapeInfoByRouteId.value.get(shape.route_id)
      if (!routeShapeInfo) {
        push(shape, null, tracked)
        continue
      }

      const tripStops = getShapeStopTimes(routeShapeInfo).filter((stopTime) => stopTime.trip_id === shape.trip_id)
      const stopShapeIdx = buildStopShapeIdxByStopId(tripStops, trip).get(stopIdNum.value) ?? -1
      if (stopShapeIdx < 0) {
        push(shape, null, tracked)
        continue
      }

      const eta = etaForStop(stopShapeIdx, vehiclesOnRoute, shapeIndex, {
        tripStops,
        targetStopId: stopIdNum.value,
        referenceTime: userTime.value,
      })
      push(shape, eta ? eta.etaMinutes : null, tracked)
    }
    shapesComingToTheStopBasedOnVehiclePositions.value = results.sort((a, b) => a.minutes_left - b.minutes_left)
    const highlightedShapes = displayShapesWithTrip.filter(([displayShape, shapePoints]) =>
      favoriteTripIds.has(displayShape.trip_id) && Array.isArray(shapePoints) && shapePoints.length > 0,
    )
    setFavoriteRouteShapes(highlightedShapes)
    applyInitialZoomOutForCurrentStop(highlightedShapes.length > 0)
    mapStore.setVehiclesToDisplay(favoriteVehicles as unknown as Vehicle[])
    isComputingDepartures.value = false
  })

  onMounted(() => {
    mapStore.setHighlightedStops([])
  })

  onUnmounted(() => {
    mapStore.setVehiclesToDisplay([])
    mapStore.setShapesToDisplay([])
  })

  const getShapesDisplay = (availableShapes: ShapeInfo[] | undefined): DisplayShape[] => {
    if (!availableShapes?.length) return []
    const routeIdsWithAvailableTimetables = new Set(availableShapes.map((shape: ShapeInfo) => shape.route_id))
    const {outgoing_trip_ids, incoming_trip_ids} = stopInfo.value!
    return [...(outgoing_trip_ids || []), ...(incoming_trip_ids || [])]
      .filter((trip_id) => {
        const tripRouteId = getRouteIdFromTripId(trip_id)
        return tripRouteId !== null && routeIdsWithAvailableTimetables.has(tripRouteId)
      })
      .reduce((acc: DisplayShape[], trip_id: string) => {
        const routeId = getRouteIdFromTripId(trip_id)
        if (routeId === null) return acc
        const shape = availableShapes.find((shape: ShapeInfo) => shape.route_id === routeId)
        if (shape) acc.push({
          trip_id,
          route_short_name: shape.route_short_name,
          route_long_name: shape.timetable?.route_long_name || '',
          route_color: shape.route_color,
          route_type: shape.route_type
        })
        return acc
      }, [])
  }

  return {
    stopInfo,
    stopName,
    isLoading,
    loadError,
    isComputingDepartures,
    departures: shapesComingToTheStopBasedOnVehiclePositions,
    busesWithAvailableTimetables,
    isDepartureFavorite,
  }
}

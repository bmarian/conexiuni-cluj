import {computed, onMounted, onUnmounted, ref, watch, type Ref} from 'vue'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {type DisplayShape, useMapStore} from '@/stores/map.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'
import {entriesAt, liveRowKey, useLive} from '@/composables/useLive.ts'
import type {Shape, ShapeInfo, VehiclesInStop} from '@/types/tranzy.ts'
import {getAvailableBusesForStop, hasTimetableEntries} from '@/utils/time.ts'
import {nextArrivals} from '@/utils/arrivals.ts'
import {getRouteIdFromTripId} from '@/utils/trips.ts'
import type {DisplayVehicle} from '@/utils/mapIcons.ts'

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

  const busesWithAvailableTimetables = computed(() => {
    return stopInfo.value?.shapes_info.filter((shape: ShapeInfo) => hasTimetableEntries(shape.timetable))
  })

  const shapeInfoByRouteId = computed(() => {
    const info = stopInfo.value
    if (!info) return new Map<number, ShapeInfo>()
    return new Map<number, ShapeInfo>(info.shapes_info.map((shapeInfo: ShapeInfo) => [shapeInfo.route_id, shapeInfo]))
  })

  // Trips at this stop whose route has a timetable, in the order the stop lists them.
  const tripsHere = computed<string[]>(() => {
    const info = stopInfo.value
    if (!info) return []
    const routeIds = new Set((busesWithAvailableTimetables.value ?? []).map((s: ShapeInfo) => s.route_id))
    return [...(info.outgoing_trip_ids || []), ...(info.incoming_trip_ids || [])]
      .filter((tid) => {
        const tripRouteId = getRouteIdFromTripId(tid)
        return tripRouteId !== null && routeIds.has(tripRouteId)
      })
  })

  function isDepartureFavorite(shape: VehiclesInStop): boolean {
    return favoritesStore.isRouteFavorite(shape.route_id)
  }

  // Favourite routes' buses are drawn on the map, so their trips stream too.
  const favoriteTripIds = computed<string[]>(() => tripsHere.value.filter((tid) => {
    const routeId = getRouteIdFromTripId(tid)
    return routeId !== null && favoritesStore.isRouteFavorite(routeId)
  }))
  const streamStopIds = computed<number[]>(() => (stopInfo.value ? [stopInfo.value.stop_id] : []))
  const {rows, vehiclesByTrip, asOfMs} = useLive(streamStopIds, favoriteTripIds)

  const shapesComingToTheStopBasedOnTimetable = computed(() => {
    if (!stopInfo.value) return []
    return getAvailableBusesForStop(
      stopInfo.value,
      userTime.value || new Date(),
      {maxMinutes: 480, limit: DEPARTURES_SHOWN + 1}
    )
  })

  // The server's row replaces the timetable list; without one the timetable stands.
  function withArrivals(shape: VehiclesInStop): VehiclesInStop | null {
    const row = asOfMs.value === null ? undefined : rows.value.get(liveRowKey(shape.trip_id, stopIdNum.value))
    if (row) {
      const entries = entriesAt(row, (userTime.value || new Date()).getTime())
      if (!entries.length) return null
      return {
        ...shape,
        minutes_left: entries[0]!.minutes,
        next_times: entries.map((e) => ({minutes: e.minutes, is_live: e.live})),
        static_time_approximation: !entries.some((e) => e.live),
      }
    }
    const scheduled = (shape.next_times ?? []).map((entry) => entry.minutes)
    const merged = nextArrivals(scheduled, DEPARTURES_SHOWN)
    if (!merged.length) return null
    return {
      ...shape,
      minutes_left: merged[0]!.minutes,
      next_times: merged.map((arrival) => ({minutes: arrival.minutes, is_live: false})),
      static_time_approximation: true,
    }
  }

  const departures = computed((): VehiclesInStop[] =>
    shapesComingToTheStopBasedOnTimetable.value
      .map((shape) => withArrivals(shape))
      .filter((shape): shape is VehiclesInStop => shape !== null)
      .sort((a, b) => a.minutes_left - b.minutes_left)
  )

  const isComputingDepartures = computed(() => !stopInfo.value)

  async function load(id: string) {
    isLoading.value = true
    loadError.value = false
    cachedShapeKey.value = ''
    cachedShapesWithTrip.value = []
    displayedFavoriteShapeKey.value = null
    await fetchStopData(id)
    if (!stopInfo.value) loadError.value = true
    isLoading.value = false
  }

  watch(stopId, (id) => void load(id), {immediate: true})

  // Favourite routes: their shapes and buses on the map.
  watch([favoriteTripIds, vehiclesByTrip, shapeInfoByRouteId], async ([tripIds, byTrip, byRoute]) => {
    const displayShapes = tripIds.reduce((acc: DisplayShape[], trip_id) => {
      const routeId = getRouteIdFromTripId(trip_id)
      const shape = routeId === null ? undefined : byRoute.get(routeId)
      if (shape) acc.push({
        trip_id,
        route_short_name: shape.route_short_name,
        route_long_name: shape.timetable?.route_long_name || '',
        route_color: shape.route_color,
        route_type: shape.route_type,
      })
      return acc
    }, [])
    const shapeKey = displayShapes.map((d) => d.trip_id).sort().join(',')
    if (shapeKey !== cachedShapeKey.value) {
      cachedShapesWithTrip.value = shapeKey ? await mapStore.requestShapes(displayShapes) : []
      cachedShapeKey.value = shapeKey
    }
    const vehicles: DisplayVehicle[] = []
    const liveTripIds = new Set<string>()
    for (const d of displayShapes) {
      const list = byTrip.get(d.trip_id) ?? []
      if (list.length) liveTripIds.add(d.trip_id)
      for (const v of list) vehicles.push({...v, route_short_name: d.route_short_name, route_color: d.route_color})
    }
    const highlighted = cachedShapesWithTrip.value.filter(([displayShape, points]) =>
      liveTripIds.has(displayShape.trip_id) && Array.isArray(points) && points.length > 0,
    )
    setFavoriteRouteShapes(highlighted)
    applyInitialZoomOutForCurrentStop(highlighted.length > 0)
    mapStore.setVehiclesToDisplay(vehicles)
  }, {immediate: true})

  onMounted(() => {
    mapStore.setHighlightedStops([])
  })

  onUnmounted(() => {
    mapStore.setVehiclesToDisplay([])
    mapStore.setShapesToDisplay([])
  })

  return {
    stopInfo,
    stopName,
    isLoading,
    loadError,
    isComputingDepartures,
    departures,
    busesWithAvailableTimetables,
    isDepartureFavorite,
    reload: () => load(stopId.value),
  }
}

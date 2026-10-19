import {computed, onMounted, onUnmounted, ref, watch, type Ref} from 'vue'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {type DisplayShape, useMapStore} from '@/stores/map.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'
import {entriesAt, liveRowKey, useLive} from '@/composables/useLive.ts'
import type {Shape, ShapeInfo, StopInfo, VehiclesInStop} from '@/types/tranzy.ts'
import {getAvailableBusesForStop, hasTimetableEntries} from '@/utils/time.ts'
import {nextArrivals} from '@/utils/arrivals.ts'
import {getRouteIdFromTripId} from '@/utils/trips.ts'
import type {DisplayVehicle} from '@/utils/mapIcons.ts'
import {buildStopInfo, network} from '@/utils/network.ts'

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

  const stopInfos = computed<StopInfo[]>(() => {
    const info: StopInfo | undefined = stopInfo.value
    const index = network.value
    if (!info) return []
    if (!index) return [info]
    const others = index.platforms(info.stop_id).slice(1)
      .map((s) => buildStopInfo(index, s.stop_id))
      .filter((other): other is StopInfo => other !== null)
    return [info, ...others]
  })

  const shapeInfoByRouteId = computed(() => {
    const byRoute = new Map<number, ShapeInfo>()
    for (const info of stopInfos.value) {
      for (const shapeInfo of info.shapes_info) if (!byRoute.has(shapeInfo.route_id)) byRoute.set(shapeInfo.route_id, shapeInfo)
    }
    return byRoute
  })

  const favoritePlatform = computed(() =>
    (stopInfos.value.length ? stopInfos.value.map((info) => info.stop_id) : [stopIdNum.value]).find((id) => favoritesStore.isStopFavorite(id)))
  const isFavorite = computed(() => favoritePlatform.value !== undefined)
  const toggleFavorite = () => favoritesStore.toggleStopFavorite(favoritePlatform.value ?? stopIdNum.value)

  const busesWithAvailableTimetables = computed(() =>
    [...shapeInfoByRouteId.value.values()].filter((shape: ShapeInfo) => hasTimetableEntries(shape.timetable)))

  function platformOf(routeId: number): StopInfo | undefined {
    return stopInfos.value.find((info) => info.shapes_info.some((s: ShapeInfo) => s.route_id === routeId))
  }

  // Trips at the station whose route has a timetable, in the order the stops list them.
  const tripsHere = computed<string[]>(() => {
    const routeIds = new Set(busesWithAvailableTimetables.value.map((s: ShapeInfo) => s.route_id))
    return [...new Set(stopInfos.value.flatMap((info) => [...(info.outgoing_trip_ids || []), ...(info.incoming_trip_ids || [])]))]
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
  const streamStopIds = computed<number[]>(() => stopInfos.value.map((info) => info.stop_id))
  const {rows, vehiclesByTrip, asOfMs} = useLive(streamStopIds, favoriteTripIds)

  // A trip calling at two platforms is listed at the one it reaches first.
  const shapesComingToTheStopBasedOnTimetable = computed(() => {
    const byTrip = new Map<string, VehiclesInStop>()
    for (const info of stopInfos.value) {
      const here = getAvailableBusesForStop(info, userTime.value || new Date(), {maxMinutes: 480, limit: DEPARTURES_SHOWN + 1})
      for (const shape of here) {
        const known = byTrip.get(shape.trip_id)
        if (!known || shape.minutes_left < known.minutes_left) byTrip.set(shape.trip_id, {...shape, stop_id: info.stop_id})
      }
    }
    return [...byTrip.values()]
  })

  // The server's row replaces the timetable list; without one the timetable stands.
  function withArrivals(shape: VehiclesInStop): VehiclesInStop | null {
    const row = asOfMs.value === null ? undefined : rows.value.get(liveRowKey(shape.trip_id, shape.stop_id ?? stopIdNum.value))
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
    stopInfos,
    platformOf,
    isFavorite,
    toggleFavorite,
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

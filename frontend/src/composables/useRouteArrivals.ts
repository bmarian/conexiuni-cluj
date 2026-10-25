import {computed, onMounted, onUnmounted, ref, watch, watchEffect, type Ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import {useRouter} from 'vue-router'
import {useRouteStore} from '@/stores/route.ts'
import {useUserStore} from '@/stores/user.ts'
import {type HighlightedStop, useMapStore} from '@/stores/map.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type Shape, type StopTime} from '@/types/tranzy.ts'
import {formatMinutesFromNow, getMinutesFromDate, getTimetableForDay, timeStringToMinutes} from '@/utils/time.ts'
import {haversineMeters, routeHeading} from '@/utils/geo.ts'
import {getShapeStopTimes} from '@/utils/trips.ts'
import {markRepeatedNow, nextArrivals, scheduledArrivals} from '@/utils/arrivals.ts'
import {entriesAt, liveRowKey, type ShownEntry, useLive} from '@/composables/useLive.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import type {DisplayVehicle} from '@/utils/mapIcons.ts'

export type IndexedStop = StopTime & {secondsFromStart: number}

export interface StopTimeDisplay {
  label: string
  isLive: boolean
  minutes: number
}

const SCHEDULE_HORIZON_MIN = 480
const ENTRIES_SHOWN = 3

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

  // 'auto' opens the direction whose nearest stop is closest to the user, before the first paint.
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
  const currentTripId = computed(() => `${routeId.value}${currentDirection.value === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`)

  const timetable = computed(() => shapeInfo.value?.timetable)
  const routeDisplayName = computed(() => {
    const tt = timetable.value
    if (!tt) return shapeInfo.value?.route_short_name || ''
    return isOutgoing.value ? tt.route_long_name : `${tt.out_stop_name} - ${tt.in_stop_name}`
  })

  const direction0Shape = ref<Shape[] | null>(null)
  const direction1Shape = ref<Shape[] | null>(null)
  const currentDirectionShape = computed(() => (currentDirection.value === '0' ? direction0Shape.value : direction1Shape.value))

  const rawStops = computed((): StopTime[] => getShapeStopTimes(shapeInfo.value))

  const stopsForDirection = computed((): IndexedStop[] => {
    const filtered = rawStops.value.filter((st) => st.trip_id === currentTripId.value).sort((a, b) => a.stop_sequence - b.stop_sequence)
    // offset_arrival_time is the ride into each stop, so add it before reading.
    let cumulativeSec = 0
    return filtered.map((st) => {
      cumulativeSec += st.offset_arrival_time
      return {...st, secondsFromStart: cumulativeSec}
    })
  })

  const directionTerminals = computed(() => {
    const byTrip = (tripId: string): {first: string; last: string} => {
      const tripStops = rawStops.value.filter((st) => st.trip_id === tripId).sort((a, b) => a.stop_sequence - b.stop_sequence)
      const first = tripStops[0]?.stop_headsign?.trim() ?? ''
      const last = tripStops[tripStops.length - 1]?.stop_headsign?.trim() ?? ''
      return {first, last}
    }
    return {
      outgoing: byTrip(`${routeId.value}${OUTGOING_SUFFIX}`),
      incoming: byTrip(`${routeId.value}${INCOMING_SUFFIX}`),
    }
  })

  const directionHeadings = computed(() => ({
    '0': routeHeading(direction0Shape.value ?? []),
    '1': routeHeading(direction1Shape.value ?? []),
  }))

  const hasOutgoing = computed(() => rawStops.value.some((st) => st.trip_id === `${routeId.value}${OUTGOING_SUFFIX}`))
  const hasIncoming = computed(() => rawStops.value.some((st) => st.trip_id === `${routeId.value}${INCOMING_SUFFIX}`))

  const nearestStopIdx = computed(() => {
    const loc = userLocation.value
    if (!loc || !stopsForDirection.value.length) return -1
    let best = -1,
      bestDist = Infinity
    stopsForDirection.value.forEach((stop, idx) => {
      if (!stop.stop_lat || !stop.stop_lon) return
      const d = haversineMeters(loc.latitude, loc.longitude, stop.stop_lat, stop.stop_lon)
      if (d < bestDist) {
        bestDist = d
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
    return sched.entries.map((e) => timeStringToMinutes(isOutgoing.value ? e.departure_in : e.departure_out)).filter((v): v is number => v !== null)
  })

  // Offset first, then sort, so runs already under way still count for later stops.
  function nextArrivalsAtStop(secondsFromStart: number): number[] {
    return scheduledArrivals({
      departureMinutes: departureTimes.value,
      offsetSeconds: secondsFromStart,
      nowMinutes: currentMinutes.value,
      horizonMinutes: SCHEDULE_HORIZON_MIN,
      graceMinutes: shapeInfo.value?.late_grace_min ?? 1,
    })
  }

  // Both directions stream: the server chains the other direction's buses onto this one.
  const streamTripIds = computed<string[]>(() =>
    shapeInfo.value?.route_id === Number(routeId.value) ? [`${routeId.value}${OUTGOING_SUFFIX}`, `${routeId.value}${INCOMING_SUFFIX}`] : [],
  )
  const {rows, vehiclesByTrip, asOfMs} = useLive(ref<number[]>([]), streamTripIds)
  const liveNowMs = computed(() => (userTime.value || new Date()).getTime())

  function liveEntriesFor(stop: IndexedStop): ShownEntry[] | null {
    if (asOfMs.value === null) return null
    const row = rows.value.get(liveRowKey(currentTripId.value, stop.stop_id))
    return row ? entriesAt(row, liveNowMs.value) : null
  }

  const toDisplay = (minutes: number, isLive: boolean, soon: boolean | undefined): StopTimeDisplay => ({
    label: soon ? t('soon') : formatMinutes(minutes),
    isLive,
    minutes,
  })

  // The server's rows when the live feed has them, the timetable alone otherwise.
  const stopTimesByStop = computed((): StopTimeDisplay[][] => {
    const stops = stopsForDirection.value
    const live = stops.map((stop) => liveEntriesFor(stop))
    if (live.some((entries) => entries !== null)) {
      return stops.map((stop, idx) => {
        const entries = live[idx]
        if (entries) return entries.map((e) => toDisplay(e.minutes, e.live, e.soon))
        return nextArrivals(nextArrivalsAtStop(stop.secondsFromStart), ENTRIES_SHOWN).map((a) => toDisplay(a.minutes, false, false))
      })
    }
    const offline = stops.map((stop) => nextArrivals(nextArrivalsAtStop(stop.secondsFromStart), ENTRIES_SHOWN))
    return markRepeatedNow(offline).map((row) => row.map((a) => toDisplay(a.minutes, false, a.soon)))
  })

  function getHeaderTimes(): string[] {
    const first = stopsForDirection.value[0]
    const entries = first ? liveEntriesFor(first) : null
    if (entries) return entries.map((e) => formatMinutes(e.minutes))
    return nextArrivals(nextArrivalsAtStop(first?.secondsFromStart ?? 0), ENTRIES_SHOWN).map((arrival) => formatMinutes(arrival.minutes))
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

  // The other direction's buses this direction's rows count on coming back, faded.
  const displayVehicles = computed((): DisplayVehicle[] => {
    const info = shapeInfo.value
    if (!info) return []
    const tripId = currentTripId.value
    const otherTripId = `${routeId.value}${currentDirection.value === '0' ? INCOMING_SUFFIX : OUTGOING_SUFFIX}`
    const counted = new Set<number>()
    for (const row of rows.value.values()) {
      if (row.trip_id !== tripId) continue
      for (const e of row.entries) if (e.vehicle_id) counted.add(e.vehicle_id)
    }
    const roundTrip = (vehiclesByTrip.value.get(otherTripId) ?? []).filter((v) => counted.has(v.id))
    const style = {route_short_name: info.route_short_name, route_color: info.route_color}
    return [...roundTrip.map((v) => ({...v, ...style, faded: true})), ...(vehiclesByTrip.value.get(tripId) ?? []).map((v) => ({...v, ...style}))]
  })

  function updateMap() {
    if (!shapeInfo.value) return
    const dirShape = currentDirectionShape.value
    const meta = buildDisplayShape()
    if (dirShape) {
      mapStore.setLoadedShapes([[meta, dirShape]])
    } else {
      void mapStore.setShapesToDisplay([meta])
    }
    mapStore.setVehiclesToDisplay(displayVehicles.value)
    zoomOut.value = true
  }

  watchEffect(() => {
    const highlights: HighlightedStop[] = []
    stopsForDirection.value.forEach((stop, idx) => {
      const stopId = String(stop.stop_id)
      if (stopId === fromStopId.value) highlights.push({stopId, color: 'green'})
      else if (idx === nearestStopIdx.value) highlights.push({stopId, color: 'purple'})
      else if (favoriteStopIds.value.includes(stop.stop_id)) highlights.push({stopId, color: 'favorite'})
      else highlights.push({stopId, color: 'gray'})
    })
    mapStore.setHighlightedStops(highlights)
  })

  async function loadDirectionShape(dir: '0' | '1'): Promise<Shape[] | null> {
    if (!shapeInfo.value) return null
    const tripId = `${routeId.value}${dir === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`
    try {
      const shapeData = await mapStore.requestShapes([
        {
          trip_id: tripId,
          route_short_name: shapeInfo.value.route_short_name,
          route_long_name: routeDisplayName.value,
          route_color: shapeInfo.value.route_color,
          route_type: shapeInfo.value.route_type,
        },
      ])
      const shape = shapeData[0]?.[1] ?? []
      return shape.length ? shape : null
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

  watch(displayVehicles, (vehicles) => {
    mapStore.setVehiclesToDisplay(vehicles)
  })
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
      // Deep-linked 'auto': resolve once the stops load, still on the skeleton.
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
    currentDirectionShape,
    directionTerminals,
    directionHeadings,
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

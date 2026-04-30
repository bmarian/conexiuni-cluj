<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useRoute} from 'vue-router'
import HeaderNavigation from "@/components/HeaderNavigation.vue"
import {useI18n} from 'vue-i18n'
import {useMapStore, type HighlightedStop} from '@/stores/map.ts'
import {useUserStore} from '@/stores/user.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import IconHeartFilled from "@/components/icons/IconHeartFilled.vue"
import IconHeartOutline from "@/components/icons/IconHeartOutline.vue"
import {decodePolyline, sortByDistance} from "@/utils/geo.ts";
import {apiRequest} from "@/utils/request_cache.ts";
import type {DirectionsResponse, Shape, Stop, StopInfo, TimeEntry} from "@/types/tranzy.ts";
import {storeToRefs} from "pinia";
import ClosestStopsList from "@/components/ClosestStopsList.vue";
import ViewErrorState from "@/components/ViewErrorState.vue";

import {findDirectRoutes, type DirectRoute} from "@/utils/trips.ts";
import {useVehicleStream} from "@/composables/useVehicleStream.ts";
import {
  buildShapeIndex,
  getIndexedVehicles,
  findClosestShapeIdx,
  buildStopShapeIdxByStopId,
  etaForStop,
  type IndexedVehicle,
  type ShapeIndex
} from "@/composables/useVehicleTracking.ts";
import {
  formatMinutesFromNow,
  getAvailableBusesForStop
} from "@/utils/time.ts";
import {getShapeStopTimes} from "@/utils/trips.ts";

const {t} = useI18n()
const route = useRoute()
const mapStore = useMapStore()
const userStore = useUserStore()
const settings = useSettingsStore()
const favoritesStore = useFavoritesStore()
const {userLocation, hasLocationPermission, userTime} = storeToRefs(userStore)

const destName = computed(() => (route.query.name as string | undefined) ?? '')
const destLat = computed(() => parseFloat((route.query.lat as string) ?? 'NaN'))
const destLon = computed(() => parseFloat((route.query.lon as string) ?? 'NaN'))

const isFavorite = computed(() => favoritesStore.isPlanFavorite(destLat.value, destLon.value))

function toggleFavorite() {
  if (isNaN(destLat.value) || isNaN(destLon.value)) return
  favoritesStore.togglePlanFavorite({
    name: destName.value || t('planTitleGeneric'),
    lat: destLat.value,
    lon: destLon.value
  })
}

const allStops = ref<Stop[]>([])
const directRoutes = ref<DirectRoute[]>([])
const selectedRouteIndex = ref(0)
const isCalculating = ref(false)
const hasFlownToFallback = ref(false)
const selectedRoute = computed(() => routesWithTimes.value[selectedRouteIndex.value])

const directRoutesShapes = ref(new Map<string, Shape[]>())
const routesWithTimes = ref<(DirectRoute & { nextTimes: TimeEntry[], isLive: boolean })[]>([])

watch(directRoutes, async (newRoutes) => {
  if (!newRoutes.length) {
    directRoutesShapes.value.clear()
    return
  }

  const toRequest = newRoutes
    .filter(dr => !directRoutesShapes.value.has(dr.tripId))
    .map(dr => ({
      trip_id: dr.tripId,
      route_short_name: dr.route.route_short_name,
      route_long_name: dr.route.route_long_name,
      route_color: dr.route.route_color,
      route_type: dr.route.route_type,
    }))

  if (toRequest.length === 0) return

  try {
    const shapeData = await mapStore.requestShapes(toRequest)
    shapeData.forEach(([info, points]) => {
      if (points?.length) {
        directRoutesShapes.value.set(info.trip_id, points)
      }
    })
  } catch (e) {
    console.warn('Failed to fetch shapes for direct routes:', e)
  }
}, {immediate: true})

const streamTripIds = computed(() => directRoutes.value.map(dr => dr.tripId))
const {vehiclesByTrip} = useVehicleStream(streamTripIds)

watch([directRoutes, vehiclesByTrip, directRoutesShapes, userTime], async ([routes, byTrip, shapesMap, uTime]) => {
  const now = uTime || new Date()
  const results: (DirectRoute & { nextTimes: TimeEntry[], isLive: boolean })[] = []

  for (const dr of routes) {
    const buses = getAvailableBusesForStop(dr.startStop, now, {limit: 3, tripId: dr.tripId})
    const myBus = buses.find(b => b.route_id === dr.route.route_id)

    // DEBUGGER: remove the filtering to see buses all the time
    let times = myBus ? myBus.next_times.filter(t => t.minutes <= 60) : []
    let isLive = false

    const points = shapesMap.get(dr.tripId)
    const vehicles = byTrip.get(dr.tripId) ?? []

    if (points && vehicles.length > 0) {
      const shapeIndex = buildShapeIndex(points)
      const indexedVehicles = await getIndexedVehicles(
        dr.tripId,
        dr.route.route_short_name,
        dr.route.route_color,
        shapeIndex,
        now,
        vehicles
      )

      const allStopTimes = getShapeStopTimes(dr.route)
      const tripStopTimes = allStopTimes.filter(st => st.trip_id === dr.tripId)
      const stopShapeIdxByStopId = buildStopShapeIdxByStopId(tripStopTimes, points)
      const startStopShapeIdx = stopShapeIdxByStopId.get(dr.startStop.stop_id) ?? -1

      if (startStopShapeIdx >= 0) {
        const eta = etaForStop(startStopShapeIdx, indexedVehicles, shapeIndex)
        if (eta && eta.etaMinutes <= 60) {
          isLive = true
          times = [
            {minutes: eta.etaMinutes, is_live: true},
            ...times.filter(t => t.minutes !== eta.etaMinutes).slice(0, 2)
          ]
        } else if (eta && eta.etaMinutes > 60) {
          times = []
        }
      }
    }

    if (times.length > 0) {
      results.push({
        ...dr,
        nextTimes: times,
        isLive
      })
    }
  }

  routesWithTimes.value = results
}, {immediate: true})

const currentShapeIndex = ref<ShapeIndex | null>(null)
const currentVehicles = ref<IndexedVehicle[]>([])

watch(selectedRoute, async (newRoute) => {
  if (!newRoute) {
    currentShapeIndex.value = null
    void mapStore.setShapesToDisplay([])
    return
  }

  try {
    const shapeData = await mapStore.requestShapes([{
      trip_id: newRoute.tripId,
      route_short_name: newRoute.route.route_short_name,
      route_long_name: newRoute.route.route_long_name,
      route_color: newRoute.route.route_color,
      route_type: newRoute.route.route_type,
    }])
    const pts = shapeData[0]?.[1] ?? []
    if (pts.length) {
      const startIdx = findClosestShapeIdx(newRoute.startStop.stop_lat, newRoute.startStop.stop_lon, pts)
      const destIdx = findClosestShapeIdx(newRoute.destStop.stop_lat, newRoute.destStop.stop_lon, pts)

      const [realStart, realEnd] = startIdx < destIdx ? [startIdx, destIdx] : [destIdx, startIdx]
      const croppedPts = pts.slice(realStart, realEnd + 1)

      currentShapeIndex.value = buildShapeIndex(croppedPts)
      if (shapeData[0]) {
        mapStore.setLoadedShapes([[shapeData[0][0], croppedPts]])
      }
    }
  } catch (e) {
    console.error('Failed to load shape:', e)
  }
}, {immediate: true})

watch([selectedRoute, userLocation], async ([newRoute, ul]) => {
  if (!newRoute || !ul || isNaN(destLat.value) || isNaN(destLon.value)) {
    mapStore.clearWalkingPolylines()
    return
  }

  try {
    const [dirToStart, dirToDest] = await Promise.all([
      apiRequest(`directions?from_lat=${ul.latitude}&from_lng=${ul.longitude}&to_lat=${newRoute.startStop.stop_lat}&to_lng=${newRoute.startStop.stop_lon}`) as Promise<DirectionsResponse>,
      apiRequest(`directions?from_lat=${newRoute.destStop.stop_lat}&from_lng=${newRoute.destStop.stop_lon}&to_lat=${destLat.value}&to_lng=${destLon.value}`) as Promise<DirectionsResponse>,
    ])

    const polylines: [number, number][][] = []
    const geomToStart = dirToStart?.routes?.[0]?.geometry
    if (geomToStart) polylines.push(decodePolyline(geomToStart))
    const geomToDest = dirToDest?.routes?.[0]?.geometry
    if (geomToDest) polylines.push(decodePolyline(geomToDest))

    mapStore.setWalkingPolylines(polylines)
  } catch (e) {
    console.error('Failed to fetch walking directions:', e)
  }
}, {immediate: true})

watch([vehiclesByTrip, selectedRoute, currentShapeIndex], async ([byTrip, route, shapeIndex]) => {
  if (!route || !shapeIndex) {
    currentVehicles.value = []
    mapStore.setVehiclesToDisplay([])
    return
  }

  const tid = route.tripId
  const v = byTrip.get(tid) ?? []

  try {
    currentVehicles.value = await getIndexedVehicles(
      tid,
      route.route.route_short_name,
      route.route.route_color,
      shapeIndex,
      userTime.value,
      v
    )
    mapStore.setVehiclesToDisplay(currentVehicles.value)
  } catch (e) {
    console.warn('Failed to index vehicles:', e)
  }
}, {deep: true})


const intermediateStops = computed(() => {
  if (!selectedRoute.value) return []
  const {tripId, startStop, destStop, route} = selectedRoute.value
  const allStopTimes = getShapeStopTimes(route)

  const tripStopTimes = allStopTimes
    .filter(st => st.trip_id === tripId)
    .sort((a, b) => a.stop_sequence - b.stop_sequence)

  const startSeq = tripStopTimes.find(st => st.stop_id === startStop.stop_id)?.stop_sequence ?? -1
  const destSeq = tripStopTimes.find(st => st.stop_id === destStop.stop_id)?.stop_sequence ?? -1

  if (startSeq === -1 || destSeq === -1) return []

  return tripStopTimes
    .filter(st => st.stop_sequence > startSeq && st.stop_sequence < destSeq)
    .map(st => ({
      stop_id: st.stop_id,
      stop_name: st.stop_headsign || `Stop ${st.stop_id}`,
    }))
})

watch([selectedRoute, intermediateStops], ([newRoute, iStops]) => {
  if (!newRoute) {
    mapStore.setHighlightedStops([])
    return
  }

  const highlighted: HighlightedStop[] = [
    {stopId: String(newRoute.startStop.stop_id), color: 'green'},
    {stopId: String(newRoute.destStop.stop_id), color: 'red'},
  ]

  iStops.forEach(s => {
    highlighted.push({stopId: String(s.stop_id), color: 'gray'})
  })

  mapStore.setHighlightedStops(highlighted)
}, {immediate: true})

const hasValidDest = computed(() => destName.value.length > 0)
const hasValidCoords = computed(() => !isNaN(destLat.value) && !isNaN(destLon.value))

const originLabel = computed(() => {
  if (userStore.userLocation) return t('planOriginCurrentLocation')
  return t('planOriginUnknown')
})

watch(selectedRouteIndex, () => {
  mapStore.fitWalkingPolylines = true
})

onMounted(async () => {
  mapStore.setHighlightedStops([])
  mapStore.setVehiclesToDisplay([])
  void mapStore.setShapesToDisplay([])

  if (hasValidCoords.value) {
    mapStore.setPinnedLocation(destLat.value, destLon.value, destName.value)
    allStops.value = await apiRequest('stops') as Stop[]

    if (!hasLocationPermission.value) {
      mapStore.setFlyToLocation(destLat.value, destLon.value)
    }
  }
})

onUnmounted(() => {
  mapStore.clearPinnedLocation()
  mapStore.setVehiclesToDisplay([])
  mapStore.setShapesToDisplay([])
  mapStore.setHighlightedStops([])
  mapStore.clearWalkingPolylines()
})

watch(routesWithTimes, (newRoutes) => {
  if (selectedRouteIndex.value >= newRoutes.length) {
    selectedRouteIndex.value = 0
  }
})

watch([isCalculating, routesWithTimes], ([calculating, routes]) => {
  if (calculating || routes.length > 0) {
    hasFlownToFallback.value = false
    return
  }

  if (routes.length === 0 && !isNaN(destLat.value) && !isNaN(destLon.value) && !hasFlownToFallback.value) {
    mapStore.setFlyToLocation(destLat.value, destLon.value)
    hasFlownToFallback.value = true
  }
})

const stopRouteCalculationWatcher = watch([destLat, destLon, userLocation, allStops], async ([lat, lon, ul, stops]) => {
  if (Number.isNaN(lat) || Number.isNaN(lon) || !ul || !hasLocationPermission.value || !Array.isArray(stops) || !stops.length) return

  isCalculating.value = true
  directRoutes.value = []
  try {
    const top4ClosestStopsToUser = await Promise.all(sortByDistance(
      stops, ul.latitude, ul.longitude, s => s.stop_lat, s => s.stop_lon, 700
    ).slice(0, 4).map(s => apiRequest(`stop_info?stop_id=${s.stop_id}`) as Promise<StopInfo>))

    const top4ClosestStopsToDestination = await Promise.all(sortByDistance(
      stops, lat, lon, s => s.stop_lat, s => s.stop_lon, 700
    ).slice(0, 4).map(s => apiRequest(`stop_info?stop_id=${s.stop_id}`) as Promise<StopInfo>))

    const routes = findDirectRoutes(top4ClosestStopsToUser, top4ClosestStopsToDestination)
    if (!routes.length) {
      directRoutes.value = []
      return
    }

    const now = userTime.value || new Date()
    routes.sort((a, b) => {
      const busesA = getAvailableBusesForStop(a.startStop, now, {limit: 1, tripId: a.tripId})
      const myBusA = busesA.find(bu => bu.route_id === a.route.route_id)
      const timeA = myBusA?.next_times?.[0] ? myBusA.next_times[0].minutes : Infinity

      const busesB = getAvailableBusesForStop(b.startStop, now, {limit: 1, tripId: b.tripId})
      const myBusB = busesB.find(bu => bu.route_id === b.route.route_id)
      const timeB = myBusB?.next_times?.[0] ? myBusB.next_times[0].minutes : Infinity

      return timeA - timeB
    })

    directRoutes.value = routes
    mapStore.fitWalkingPolylines = true

    stopRouteCalculationWatcher()
  } finally {
    isCalculating.value = false
  }
}, {immediate: true})

</script>

<template>
  <div v-if="!hasValidCoords"
       class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col">
    <ViewErrorState/>
  </div>
  <div v-else
       class="plan-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-6">
    <div class="flex items-center -mb-2">
      <HeaderNavigation/>
    </div>
    <header class="flex items-center gap-3">
      <div
        class="w-12 h-12 shrink-0 rounded-2xl bg-linear-to-br from-sky-400 to-sky-600 flex items-center justify-center shadow-lg shadow-sky-500/20">
        <span v-if="settings.traditionalActive" class="emoji-icon-xl" aria-hidden="true">🗺️</span>
        <svg v-else class="w-6 h-6 text-white" viewBox="0 0 24 24" fill="none"
             stroke="currentColor"
             stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round"
                d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <span
          class="text-[10px] font-semibold text-sky-600 dark:text-sky-400 tracking-wide uppercase">{{
            t('planTitle')
          }}</span>
        <h1
          class="text-xl font-black tracking-tight text-slate-900 dark:text-white leading-tight truncate"
          :title="hasValidDest ? destName : t('planTitleGeneric')">
          {{ hasValidDest ? destName : t('planTitleGeneric') }}
        </h1>
      </div>
      <button
        v-if="hasValidCoords"
        type="button"
        class="fav-btn mt-1 shrink-0"
        :class="{ 'is-fav': isFavorite }"
        :title="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-label="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-pressed="isFavorite"
        @click="toggleFavorite"
      >
        <IconHeartFilled v-if="isFavorite" class="w-5 h-5"/>
        <IconHeartOutline v-else class="w-5 h-5"/>
      </button>
    </header>

    <div v-if="hasLocationPermission">
      <section v-if="isCalculating || routesWithTimes.length > 0" class="route-legs-card">
        <div class="leg-row">
          <div class="leg-icon-col">
            <div class="leg-dot leg-dot-origin">
              <span v-if="settings.traditionalActive" class="text-sm">📍</span>
              <svg v-else class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
                <circle cx="12" cy="12" r="6"/>
              </svg>
            </div>
            <div class="leg-line"></div>
          </div>
          <div class="leg-label-col">
            <span class="leg-type-badge">{{ t('planFrom') }}</span>
            <span class="leg-name">{{ originLabel }}</span>
          </div>
        </div>

        <template v-if="selectedRoute">
          <div class="leg-row">
            <div class="leg-icon-col">
              <div class="leg-dot boarding-dot" :style="{ borderColor: selectedRoute.route.route_color }">
                <div class="dot-inner" :style="{ backgroundColor: selectedRoute.route.route_color }"></div>
              </div>
              <div class="leg-line" :style="{ backgroundColor: selectedRoute.route.route_color, backgroundImage: 'none' }"></div>
            </div>
            <div class="leg-label-col">
              <span class="leg-type-badge" :style="{ color: selectedRoute.route.route_color }">{{ t('planBoarding') }}</span>
              <span class="leg-name">{{ selectedRoute.startStop.stop_name }}</span>
            </div>
          </div>

          <div v-for="stop in intermediateStops" :key="stop.stop_id" class="leg-row intermediate-leg">
            <div class="leg-icon-col">
              <div class="leg-dot intermediate-dot">
                <div class="w-1 h-1 rounded-full bg-slate-300 dark:bg-slate-600"></div>
              </div>
              <div class="leg-line" :style="{ backgroundColor: selectedRoute.route.route_color, backgroundImage: 'none' }"></div>
            </div>
            <div class="leg-label-col">
              <span class="leg-name intermediate-name">{{ stop.stop_name }}</span>
            </div>
          </div>

          <div class="leg-row">
            <div class="leg-icon-col">
              <div class="leg-dot boarding-dot" :style="{ borderColor: selectedRoute.route.route_color }">
                <div class="dot-inner" :style="{ backgroundColor: selectedRoute.route.route_color }"></div>
              </div>
              <div class="leg-line"></div>
            </div>
            <div class="leg-label-col">
              <span class="leg-type-badge" :style="{ color: selectedRoute.route.route_color }">{{ t('planAlighting') }}</span>
              <span class="leg-name">{{ selectedRoute.destStop.stop_name }}</span>
            </div>
          </div>
        </template>

        <div class="leg-row">
          <div class="leg-icon-col">
            <div class="leg-dot leg-dot-dest">
              <span v-if="settings.traditionalActive" class="text-sm">🏁</span>
              <svg v-else class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
              </svg>
            </div>
          </div>
          <div class="leg-label-col">
            <span class="leg-type-badge leg-type-badge-dest">{{ t('planTo') }}</span>
            <span class="leg-name"
                  :title="hasValidDest ? destName : '—'">{{ hasValidDest ? destName : '—' }}</span>
          </div>
        </div>
      </section>
      <section class="flex flex-col gap-3 pb-8">
        <h2 class="section-label">
          <span class="w-2 h-2 rounded-full bg-sky-500 shrink-0"></span>
          {{ t('planRoutesLabel') }}
        </h2>

        <div v-if="isCalculating" class="plan-loading">
          <div class="bus-loader-container">
            <span v-if="settings.traditionalActive" class="emoji-icon-xl animate-bus-run" aria-hidden="true">🚌</span>
            <span v-else-if="settings.easterEggActive" class="emoji-icon-xl animate-bus-run" aria-hidden="true">🍔</span>
            <svg v-else class="w-12 h-12 text-sky-500 animate-bus-run" viewBox="0 0 24 24"
                 fill="none" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round"
                    d="M8.25 18.75a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 01-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 00-3.213-9.193 2.056 2.056 0 00-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 00-10.026 0 1.106 1.106 0 00-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12"/>
            </svg>
          </div>
          <p class="loading-text animate-pulse">{{ t('planCalculating') }}</p>
        </div>

        <div v-else-if="routesWithTimes.length > 0" class="flex flex-col gap-2.5">
          <div
            v-for="(direct, index) in routesWithTimes"
            :key="index"
            class="departure-card group"
            :class="{ 'is-selected': selectedRouteIndex === index }"
            @click="selectedRouteIndex = index"
          >
            <div
              class="w-1 self-stretch rounded-full shrink-0"
              :class="selectedRouteIndex === index ? 'bg-sky-500' : 'bg-transparent'"
            ></div>

            <div
              class="flex items-center justify-center shrink-0 w-11 h-9 rounded-xl font-black text-sm text-white shadow-sm"
              :style="{ backgroundColor: direct.route.route_color }"
            >{{ direct.route.route_short_name }}
            </div>

            <div class="flex-1 min-w-0 flex flex-col justify-center">
              <div class="flex items-center gap-1.5 mb-0.5">
                <span v-if="direct.isLive" class="live-badge">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
                  {{ t('live') }}
                </span>
                <span class="card-dest">→ {{ direct.destStop.stop_name }}</span>
              </div>
              <span class="card-origin">{{ direct.startStop.stop_name }}</span>
            </div>

            <div class="flex items-center gap-1 shrink-0">
              <span
                v-for="(entry, i) in direct.nextTimes"
                :key="i"
                :class="[
                  'time-pill',
                  entry.is_live ? 'time-pill-live' : 'time-pill-sched',
                  i > 0 ? 'time-pill-extra' : ''
                ]"
              >{{ entry.is_live ? '' : '~\u202f' }}{{ formatMinutesFromNow(entry.minutes, userTime || new Date(), t('now')) }}</span>
            </div>

            <svg
              class="w-4 h-4 text-slate-400 dark:text-slate-500 shrink-0 group-hover:text-slate-600 dark:group-hover:text-slate-300 transition-colors"
              fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </div>
        </div>

        <div v-else class="flex flex-col gap-4">
          <div class="plan-placeholder">
            <template v-if="directRoutes.length > 0">
              <svg class="w-10 h-10 text-slate-300 dark:text-slate-700 mb-3" viewBox="0 0 24 24"
                   fill="none" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round"
                      d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/>
              </svg>
              <p class="text-sm font-medium text-slate-400 dark:text-slate-500 text-center">
                {{ t('planNoBusesNextHour') }}</p>
            </template>
            <template v-else>
              <svg class="w-10 h-10 text-slate-300 dark:text-slate-700 mb-3" viewBox="0 0 24 24"
                   fill="none" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round"
                      d="M8.25 18.75a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 01-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 00-3.213-9.193 2.056 2.056 0 00-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 00-10.026 0 1.106 1.106 0 00-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12"/>
              </svg>
              <p class="text-sm font-medium text-slate-400 dark:text-slate-500 text-center">
                {{ t('planComingSoon') }}</p>
              <p class="text-xs text-slate-300 dark:text-slate-600 text-center mt-1">
                {{ t('planComingSoonDesc') }}</p>
            </template>
          </div>

          <ClosestStopsList :stops="allStops" :center-lat="destLat" :center-lon="destLon"/>
        </div>
      </section>
    </div>

    <div v-else>
      <section class="flex flex-col gap-3 pb-8">
        <ClosestStopsList :stops="allStops" :center-lat="destLat" :center-lon="destLon"/>
      </section>
    </div>
  </div>
</template>

<style scoped>
.plan-view-container {
  padding: 1.25rem 1.5rem 0;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

.section-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  margin-bottom: 0.25rem;
  margin-top: 1.25rem;
}

.route-legs-card {
  display: flex;
  flex-direction: column;
  background: #f8fafc;
  border: 1.5px solid #e2e8f0;
  border-radius: 1rem;
  padding: 0.875rem 1rem;
  gap: 0;
}

.leg-row {
  display: flex;
  align-items: stretch;
  gap: 0.875rem;
  min-height: 2.5rem;
}

.leg-icon-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 1.5rem;
  flex-shrink: 0;
}

.leg-dot {
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 0.125rem;
}

.boarding-dot {
  background: white;
  border-width: 2px;
}

.dot-inner {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
}

.dark .boarding-dot {
  background: #0f172a;
}

.dark .leg-dot-origin {
  box-shadow: 0 0 0 3px rgba(14, 165, 233, 0.2);
}

.dark .leg-dot-dest {
  box-shadow: 0 0 0 3px rgba(2, 132, 199, 0.2);
}

/* Hungry Theme Overrides */
html[data-hungry] .leg-dot-origin {
  background: #f59e0b;
  box-shadow: 0 0 0 3px #fef3c7;
}

html[data-hungry] .leg-dot-dest {
  background: #b45309;
  box-shadow: 0 0 0 3px #fde68a;
}

html[data-hungry] .leg-type-badge {
  color: #d97706;
}

/* Traditional Theme Overrides */
html[data-traditional] .leg-dot-origin,
html[data-traditional] .leg-dot-dest {
  background: transparent !important;
  box-shadow: none !important;
}

html[data-traditional] .boarding-dot {
  border-radius: 0;
}

html[data-traditional] .intermediate-dot {
  border-radius: 0;
}

html[data-traditional] .intermediate-dot div {
  border-radius: 0;
}

html[data-traditional] .dot-inner {
  border-radius: 0;
}

.leg-dot-origin {
  background: #0ea5e9;
  box-shadow: 0 0 0 3px #e0f2fe;
}

.leg-dot-dest {
  background: #0284c7;
  box-shadow: 0 0 0 3px #bae6fd;
}

.leg-line {
  flex: 1;
  width: 2px;
  background: repeating-linear-gradient(to bottom, #7dd3fc 0, #7dd3fc 4px, transparent 4px, transparent 8px);
  margin: 0.25rem 0;
  min-height: 0.75rem;
}

.leg-label-col {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0.125rem;
  flex: 1;
  min-width: 0;
  padding-bottom: 0.625rem;
}

.leg-row:last-child .leg-label-col {
  padding-bottom: 0;
}

.leg-type-badge {
  font-size: 0.625rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #0ea5e9;
}

.leg-type-badge-dest {
  color: #0284c7;
}

.leg-name {
  font-size: 0.9375rem;
  font-weight: 600;
  color: #0f172a;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.intermediate-leg {
  min-height: 1.5rem;
}

.intermediate-dot {
  width: 0.75rem;
  height: 0.75rem;
  background: white;
  border: 1px solid #e2e8f0;
}

.intermediate-name {
  font-size: 0.75rem;
  font-weight: 500;
  color: #64748b;
}

.dark .intermediate-name {
  color: #94a3b8;
}

.dark .intermediate-dot {
  background: #1e293b;
  border-color: #334155;
}

.plan-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2.5rem 1rem;
  border: 1.5px dashed #e2e8f0;
  border-radius: 1rem;
  background: #fafafa;
}

.departure-card {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.75rem 0.625rem 0.75rem 0.5rem;
  border-radius: 1rem;
  border: 1px solid #f1f5f9;
  background: #f8fafc;
  container-type: inline-size;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
}

.departure-card:hover {
  background: white;
  border-color: #e2e8f0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.live-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.625rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: #059669;
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  padding: 0.125rem 0.375rem;
  border-radius: 9999px;
}

.departure-card.is-selected {
  border-color: #0ea5e9;
  background: #f0f9ff;
  box-shadow: 0 4px 12px rgba(14, 165, 233, 0.1);
}

.dark .departure-card {
  background: #1e293b;
  border-color: #334155;
}

.dark .departure-card:hover {
  background: #1e293b;
  border-color: #475569;
}

.dark .departure-card.is-selected {
  background: #0f172a;
  border-color: #38bdf8;
}

.card-dest {
  font-size: 0.8125rem;
  font-weight: 700;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.dark .card-dest {
  color: #f1f5f9;
}

.card-origin {
  font-size: 0.6875rem;
  font-weight: 500;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.time-pill {
  font-size: 0.7rem;
  font-weight: 700;
  padding: 0.2rem 0.45rem;
  border-radius: 0.5rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.time-pill-live {
  background: #10b981;
  color: white;
}

.time-pill-sched {
  background: #f1f5f9;
  color: #475569;
}

.dark .time-pill-sched {
  background: #334155;
  color: #cbd5e1;
}

/* Hide 2nd + 3rd pill when card is narrow */
@container (max-width: 300px) {
  .time-pill-extra {
    display: none;
  }
}

.fav-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 9999px;
  color: #94a3b8;
  background: transparent;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, transform 0.15s;
}

.fav-btn:hover {
  background: #fef2f2;
  color: #f43f5e;
}

.fav-btn:active {
  transform: scale(0.92);
}

.fav-btn.is-fav {
  color: #f43f5e;
}

.fav-btn.is-fav:hover {
  background: #fee2e2;
}

.plan-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem 1rem;
  gap: 1rem;
  background: #f8fafc;
  border-radius: 1rem;
  border: 1.5px solid #e2e8f0;
}

.dark .plan-loading {
  background: #1e293b;
  border-color: #334155;
}

.bus-loader-container {
  overflow: hidden;
  width: 100px;
  display: flex;
  justify-content: center;
}

.animate-bus-run {
  animation: bus-run 1.2s infinite linear;
}

.loading-text {
  font-size: 0.875rem;
  font-weight: 600;
  color: #64748b;
}

.dark .loading-text {
  color: #94a3b8;
}

/* Hungry Theme */
html[data-hungry] .plan-loading {
  background: #fffbeb;
  border-color: #fde68a;
}

html[data-hungry] .loading-text {
  color: #d97706;
}

/* Traditional Theme */
html[data-traditional] .plan-loading {
  background: #ECE9D8;
  border: 2px solid #919B9C;
  border-radius: 0;
  box-shadow: inset -1px -1px 1px #ffffff, inset 1px 1px 1px #000000;
}

html[data-traditional] .loading-text {
  font-family: 'Tahoma', 'Trebuchet MS', sans-serif;
  color: #000000;
  animation: none !important;
}

.dark .plan-placeholder {
  background: #1e293b;
  border-color: #334155;
}

html[data-hungry] .plan-placeholder {
  background: #fffbeb;
  border-color: #fde68a;
  border-style: solid;
}

html[data-traditional] .plan-placeholder {
  background: #ECE9D8;
  border: 2px solid #919B9C;
  border-radius: 0;
  box-shadow: inset -1px -1px 1px #ffffff, inset 1px 1px 1px #000000;
  border-style: solid;
}
</style>

<style>
@keyframes bus-run {
  0% { transform: translateX(-50px); opacity: 0; }
  20% { opacity: 1; }
  80% { opacity: 1; }
  100% { transform: translateX(50px); opacity: 0; }
}
</style>

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
import {decodePolyline, haversineMeters} from "@/utils/geo.ts";
import {apiRequest} from "@/utils/request_cache.ts";
import type {DirectionsResponse, Shape, Stop, StopInfo, TimeEntry} from "@/types/tranzy.ts";
import {storeToRefs} from "pinia";
import ClosestStopsList from "@/components/ClosestStopsList.vue";
import ViewErrorState from "@/components/ViewErrorState.vue";

import {estimateMinutesToDestination, findRoutes, getTimeOffsetToStop, type PlannedRoute} from "@/utils/trips.ts";
import {useVehicleStream} from "@/composables/useVehicleStream.ts";
import type {DisplayShape} from "@/stores/map.ts";
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

const MAX_MINUTES = 60
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
const plannedRoutes = ref<PlannedRoute[]>([])
const selectedRouteIndex = ref(0)
const isCalculating = ref(false)
const hasFlownToFallback = ref(false)
const selectedRoute = computed(() => routesWithTimes.value[selectedRouteIndex.value])

const plannedRoutesShapes = ref(new Map<string, Shape[]>())
const routesWithTimes = ref<(PlannedRoute & { nextTimes: TimeEntry[], isLive: boolean })[]>([])
const directRoutes = computed(() => plannedRoutes.value.filter(r => r.isDirect))

watch(plannedRoutes, async (newRoutes) => {
  if (!newRoutes.length) {
    plannedRoutesShapes.value.clear()
    return
  }

  const tripIds = new Set<string>()
  newRoutes.forEach(pr => pr.legs.forEach(l => l.tripIds.forEach(tid => tripIds.add(tid))))

  const toRequest: DisplayShape[] = []
  newRoutes.forEach(pr => {
    pr.legs.forEach(l => {
      l.tripIds.forEach((tid, idx) => {
        if (!plannedRoutesShapes.value.has(tid)) {
          const route = l.routes[idx]
          if (route) {
            toRequest.push({
              trip_id: tid,
              route_short_name: route.route_short_name,
              route_long_name: route.route_long_name,
              route_color: route.route_color,
              route_type: route.route_type,
            })
          }
        }
      })
    })
  })

  if (toRequest.length === 0) return

  try {
    const shapeData = await mapStore.requestShapes(toRequest)
    shapeData.forEach(([info, points]) => {
      if (points?.length) {
        plannedRoutesShapes.value.set(info.trip_id, points)
      }
    })
  } catch (e) {
    console.warn('Failed to fetch shapes for planned routes:', e)
  }
}, {immediate: true})

const streamTripIds = computed(() => {
  const ids = new Set<string>()
  plannedRoutes.value.forEach(pr => pr.legs.forEach(l => l.tripIds.forEach(tid => ids.add(tid))))
  return [...ids]
})
const {vehiclesByTrip} = useVehicleStream(streamTripIds)

watch([plannedRoutes, vehiclesByTrip, plannedRoutesShapes, userTime], async ([routes, byTrip, shapesMap, uTime]) => {
  const now = uTime || new Date()
  const results: (PlannedRoute & { nextTimes: TimeEntry[], isLive: boolean })[] = []

  for (const pr of routes) {
    if (pr.isDirect) {
      const dr = pr.legs[0]!
      const times: TimeEntry[] = []
      let isLive = false

      for (let i = 0; i < dr.routes.length; i++) {
        const route = dr.routes[i]
        const tripId = dr.tripIds[i]
        if (!route || !tripId) continue

        const buses = getAvailableBusesForStop(dr.startStop as StopInfo, now, {limit: 2, tripId: tripId})
        const myBus = buses.find(b => b.route_id === route.route_id)

        if (myBus) {
          let busTimes = myBus.next_times.filter(t => t.minutes <= MAX_MINUTES)
          const points = shapesMap.get(tripId)
          const vehicles = byTrip.get(tripId) ?? []

          if (points && vehicles.length > 0) {
            const shapeIndex = buildShapeIndex(points)
            const indexedVehicles = await getIndexedVehicles(
              tripId,
              route.route_short_name,
              route.route_color,
              shapeIndex,
              now,
              vehicles
            )

            const allStopTimes = getShapeStopTimes(route)
            const tripStopTimes = allStopTimes.filter(st => st.trip_id === tripId)
            const stopShapeIdxByStopId = buildStopShapeIdxByStopId(tripStopTimes, points)
            const startStopShapeIdx = stopShapeIdxByStopId.get(dr.startStop.stop_id) ?? -1

            if (startStopShapeIdx >= 0) {
              const eta = etaForStop(startStopShapeIdx, indexedVehicles, shapeIndex)
              if (eta && eta.etaMinutes <= MAX_MINUTES) {
                isLive = true
                busTimes = [
                  {minutes: eta.etaMinutes, is_live: true},
                  ...busTimes.filter(t => t.minutes !== eta.etaMinutes).slice(0, 1)
                ]
              } else if (eta && eta.etaMinutes > MAX_MINUTES) {
                busTimes = []
              }
            }
          }
          times.push(...busTimes)
        }
      }

      if (times.length > 0) {
        results.push({
          ...pr,
          nextTimes: times.sort((a, b) => a.minutes - b.minutes).slice(0, 3),
          isLive
        })
      }
    } else {
      // Connecting route (2 legs)
      const leg1 = pr.legs[0]!
      const leg2 = pr.legs[1]!

      const validTimes: TimeEntry[] = []

      // For connecting routes, we use the first route option for Leg 1 as primary
      const route1 = leg1.routes[0]
      const tripId1 = leg1.tripIds[0]
      if (!route1 || !tripId1) continue

      const buses1 = getAvailableBusesForStop(leg1.startStop as StopInfo, now, {limit: 5, tripId: tripId1})
      const myBus1 = buses1.find(b => b.route_id === route1.route_id)
      if (!myBus1) continue

      const stopTimes1 = getShapeStopTimes(route1)
      const offsetToTransfer = getTimeOffsetToStop(stopTimes1, tripId1, leg1.destStop.stop_id)

      for (const t1 of myBus1.next_times) {
        if (t1.minutes > MAX_MINUTES) continue

        const arrivalAtTransfer = new Date(now.getTime() + (t1.minutes + offsetToTransfer) * 60_000)

        // Check any of the routes for Leg 2
        let hasConnection = false
        for (let j = 0; j < leg2.routes.length; j++) {
          const route2 = leg2.routes[j]
          const tripId2 = leg2.tripIds[j]
          const buses2 = getAvailableBusesForStop({
            stop_id: leg2.startStop.stop_id,
            shapes_info: [route2]
          } as unknown as StopInfo, arrivalAtTransfer, {limit: 1, tripId: tripId2, maxMinutes: 30})
          if (buses2.length > 0) {
            hasConnection = true
            break
          }
        }

        if (hasConnection) {
          validTimes.push(t1)
        }
      }

      if (validTimes.length > 0) {
        results.push({
          ...pr,
          nextTimes: validTimes.slice(0, 3),
          isLive: false
        })
      }
    }
  }

  routesWithTimes.value = results.sort((a, b) => {
    const arrivalA = estimateMinutesToDestination(a, a.nextTimes)
    const arrivalB = estimateMinutesToDestination(b, b.nextTimes)
    if (Math.abs(arrivalA - arrivalB) > 1) return arrivalA - arrivalB
    return (a.legs.length - b.legs.length) || (a.walkEndMeters - b.walkEndMeters)
  })
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
    const toRequest = newRoute.legs.map(l => {
      const r = l.routes[0]
      const tid = l.tripIds[0]
      if (!r || !tid) throw new Error("Invalid leg data")
      return {
        trip_id: tid,
        route_short_name: r.route_short_name,
        route_long_name: r.route_long_name,
        route_color: r.route_color,
        route_type: r.route_type,
      }
    })

    const shapeData = await mapStore.requestShapes(toRequest)
    const loadedShapes: Array<[DisplayShape, Shape[]]> = []

    shapeData.forEach(([info, pts], idx) => {
      const leg = newRoute.legs[idx]!
      if (pts.length) {
        const startIdx = findClosestShapeIdx(leg.startStop.stop_lat, leg.startStop.stop_lon, pts)
        const destIdx = findClosestShapeIdx(leg.destStop.stop_lat, leg.destStop.stop_lon, pts)

        const [realStart, realEnd] = startIdx < destIdx ? [startIdx, destIdx] : [destIdx, startIdx]
        const croppedPts = pts.slice(realStart, realEnd + 1)

        if (idx === 0) {
          currentShapeIndex.value = buildShapeIndex(croppedPts)
        }
        loadedShapes.push([info, croppedPts])
      }
    })

    mapStore.setLoadedShapes(loadedShapes)
  } catch (e) {
    console.error('Failed to load shapes:', e)
  }
}, {immediate: true})

watch([selectedRoute, userLocation], async ([newRoute, ul]) => {
  if (!newRoute || !ul || isNaN(destLat.value) || isNaN(destLon.value)) {
    mapStore.clearWalkingPolylines()
    return
  }

  try {
    const firstLeg = newRoute.legs[0]!
    const lastLeg = newRoute.legs[newRoute.legs.length - 1]!

    const promises = [
      apiRequest(`directions?from_lat=${ul.latitude}&from_lng=${ul.longitude}&to_lat=${firstLeg.startStop.stop_lat}&to_lng=${firstLeg.startStop.stop_lon}`) as Promise<DirectionsResponse>,
      apiRequest(`directions?from_lat=${lastLeg.destStop.stop_lat}&from_lng=${lastLeg.destStop.stop_lon}&to_lat=${destLat.value}&to_lng=${destLon.value}`) as Promise<DirectionsResponse>,
    ]

    // If there's a transfer with a walk
    if (newRoute.legs.length > 1) {
      for (let i = 0; i < newRoute.legs.length - 1; i++) {
        const legA = newRoute.legs[i]!
        const legB = newRoute.legs[i+1]!
        if (legA.destStop.stop_id !== legB.startStop.stop_id) {
          promises.push(apiRequest(`directions?from_lat=${legA.destStop.stop_lat}&from_lng=${legA.destStop.stop_lon}&to_lat=${legB.startStop.stop_lat}&to_lng=${legB.startStop.stop_lon}`) as Promise<DirectionsResponse>)
        }
      }
    }

    const responses = await Promise.all(promises)

    const polylines: [number, number][][] = []
    responses.forEach(res => {
      const geom = res?.routes?.[0]?.geometry
      if (geom) polylines.push(decodePolyline(geom))
    })

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

  const firstLeg = route.legs[0]!
  const tid = firstLeg.tripIds[0]
  if (!tid) return
  const v = byTrip.get(tid) ?? []

  try {
    const firstRoute = firstLeg.routes[0]
    if (!firstRoute) return
    currentVehicles.value = await getIndexedVehicles(
      tid,
      firstRoute.route_short_name,
      firstRoute.route_color,
      shapeIndex,
      userTime.value,
      v
    )
    mapStore.setVehiclesToDisplay(currentVehicles.value)
  } catch (e) {
    console.warn('Failed to index vehicles:', e)
  }
}, {deep: true})


const getTransferWalkMeters = (legIdx: number): number => {
  const route = selectedRoute.value
  if (!route) return 0
  const a = route.legs[legIdx]
  const b = route.legs[legIdx + 1]
  if (!a || !b) return 0
  if (a.destStop.stop_id === b.startStop.stop_id) return 0
  return haversineMeters(a.destStop.stop_lat, a.destStop.stop_lon, b.startStop.stop_lat, b.startStop.stop_lon)
}

const routeLegsData = computed(() => {
  if (!selectedRoute.value) return []

  return selectedRoute.value.legs.map((leg) => {
    const tripId = leg.tripIds[0]
    const route = leg.routes[0]
    const {startStop, destStop} = leg
    const allStopTimes = getShapeStopTimes(route)

    const tripStopTimes = allStopTimes
      .filter(st => st.trip_id === tripId)
      .sort((a, b) => a.stop_sequence - b.stop_sequence)

    const startSeq = tripStopTimes.find(st => st.stop_id === startStop.stop_id)?.stop_sequence ?? -1
    const destSeq = tripStopTimes.find(st => st.stop_id === destStop.stop_id)?.stop_sequence ?? -1

    const intermediates = (startSeq !== -1 && destSeq !== -1)
      ? tripStopTimes.filter(st => st.stop_sequence > startSeq && st.stop_sequence < destSeq)
          .map(st => ({
            stop_id: st.stop_id,
            stop_name: st.stop_headsign || `Stop ${st.stop_id}`,
          }))
      : []

    return {
      leg,
      intermediates
    }
  })
})

watch([selectedRoute, routeLegsData], ([newRoute, legsData]) => {
  if (!newRoute) {
    mapStore.setHighlightedStops([])
    return
  }

  const highlighted: HighlightedStop[] = []

  legsData.forEach((ld, idx) => {
    highlighted.push({
      stopId: String(ld.leg.startStop.stop_id),
      color: idx === 0 ? 'green' : 'purple'
    })

    ld.intermediates.forEach(s => {
      highlighted.push({stopId: String(s.stop_id), color: 'gray'})
    })

    if (idx === legsData.length - 1) {
      highlighted.push({stopId: String(ld.leg.destStop.stop_id), color: 'red'})
    } else if (ld.leg.destStop.stop_id !== legsData[idx + 1]?.leg.startStop.stop_id) {
      // Alighting stop differs from next boarding stop — mark it as a "get off here" point
      // so the walking transfer reads on the map.
      highlighted.push({stopId: String(ld.leg.destStop.stop_id), color: 'amber'})
    }
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
  plannedRoutes.value = []
  try {
    const routes = await findRoutes(ul.latitude, ul.longitude, lat, lon)
    if (!routes.length) {
      plannedRoutes.value = []
      return
    }

    plannedRoutes.value = routes
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
          <div v-for="(ld, legIdx) in routeLegsData" :key="legIdx">
            <div class="leg-row">
              <div class="leg-icon-col">
                <div class="leg-dot boarding-dot" :style="{ borderColor: ld.leg.routes[0]?.route_color }">
                  <div class="dot-inner" :style="{ backgroundColor: ld.leg.routes[0]?.route_color }"></div>
                </div>
                <div class="leg-line" :style="{ backgroundColor: ld.leg.routes[0]?.route_color, backgroundImage: 'none' }"></div>
              </div>
              <div class="leg-label-col">
                <span class="leg-type-badge" :style="{ color: ld.leg.routes[0]?.route_color }">
                  {{ t('planBoarding') }}
                  <template v-for="(r, rIdx) in ld.leg.routes" :key="rIdx">
                    {{ r.route_short_name }}{{ rIdx < ld.leg.routes.length - 1 ? ' / ' : '' }}
                  </template>
                </span>
                <span class="leg-name">{{ ld.leg.startStop.stop_name }}</span>
              </div>
            </div>

            <div v-for="stop in ld.intermediates" :key="stop.stop_id" class="leg-row intermediate-leg">
              <div class="leg-icon-col">
                <div class="leg-dot intermediate-dot">
                  <div class="w-1 h-1 rounded-full bg-slate-300 dark:bg-slate-600"></div>
                </div>
                <div class="leg-line" :style="{ backgroundColor: ld.leg.routes[0]?.route_color, backgroundImage: 'none' }"></div>
              </div>
              <div class="leg-label-col">
                <span class="leg-name intermediate-name">{{ stop.stop_name }}</span>
              </div>
            </div>

            <div class="leg-row" :class="{ 'leg-row-alight-transfer': legIdx < routeLegsData.length - 1 }">
              <div class="leg-icon-col">
                <div class="leg-dot boarding-dot" :style="{ borderColor: ld.leg.routes[0]?.route_color }">
                  <div class="dot-inner" :style="{ backgroundColor: ld.leg.routes[0]?.route_color }"></div>
                </div>
                <div v-if="legIdx === routeLegsData.length - 1" class="leg-line leg-line-dashed"></div>
              </div>
              <div class="leg-label-col">
                <span class="leg-type-badge" :style="{ color: ld.leg.routes[0]?.route_color }">{{ t('planAlighting') }}</span>
                <span class="leg-name">{{ ld.leg.destStop.stop_name }}</span>
              </div>
            </div>

            <div v-if="legIdx < routeLegsData.length - 1" class="transfer-block">
              <div class="transfer-block-rail">
                <span class="transfer-block-icon" aria-hidden="true">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="13" cy="4" r="2"/>
                    <path d="M5 22l4-9 4 4 4 5"/>
                    <path d="M9 13l-2-4 4-2 3 4 3 1"/>
                  </svg>
                </span>
              </div>
              <div class="transfer-block-content">
                <span class="transfer-block-label">{{ t('planTransfer') }}</span>
                <span class="transfer-block-detail">
                  <template v-if="getTransferWalkMeters(legIdx) > 25">
                    {{ Math.round(getTransferWalkMeters(legIdx)) }}&nbsp;m {{ t('planTransferWalk') }}
                  </template>
                  <template v-else>
                    {{ t('planTransferSameStop') }}
                  </template>
                </span>
              </div>
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
            v-for="(route, index) in routesWithTimes"
            :key="index"
            class="departure-card group"
            :class="{ 'is-selected': selectedRouteIndex === index }"
            @click="selectedRouteIndex = index"
          >
            <div class="card-rail" :class="{ 'is-active': selectedRouteIndex === index }"></div>

            <div class="card-body">
              <div class="card-row-primary">
                <div class="bus-chain">
                  <template v-for="(leg, lIdx) in route.legs" :key="lIdx">
                    <span
                      class="bus-chip"
                      :style="{ backgroundColor: leg.routes[0]?.route_color }"
                      :title="leg.routes.map(r => r.route_short_name).join(' / ')"
                    ><template v-for="(r, rIdx) in leg.routes" :key="rIdx"><span class="bus-chip-name">{{ r.route_short_name }}</span><span v-if="rIdx < leg.routes.length - 1" class="bus-chip-sep">/</span></template></span>
                    <svg v-if="lIdx < route.legs.length - 1" class="bus-chain-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M13 6l6 6-6 6"/>
                    </svg>
                  </template>
                </div>

                <span v-if="route.nextTimes[0]"
                  class="card-primary-time"
                  :class="route.nextTimes[0].is_live ? 'card-primary-time-live' : 'card-primary-time-sched'">
                  <span v-if="route.nextTimes[0].is_live" class="live-dot"></span><span v-if="!route.nextTimes[0].is_live">~ </span>{{ formatMinutesFromNow(route.nextTimes[0].minutes, userTime || new Date(), t('now')) }}
                </span>

                <svg class="card-chevron" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
                </svg>
              </div>

              <div class="card-row-meta">
                <span class="card-arrow">→</span>
                <span class="card-dest" :title="route.legs[route.legs.length - 1]?.destStop?.stop_name">{{ route.legs[route.legs.length - 1]?.destStop?.stop_name }}</span>
              </div>

              <div class="card-row-stats">
                <span v-if="!route.isDirect" class="stat-chip stat-chip-transfer">
                  <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M7 4v12m-3-3l3 3 3-3m7 9V4m-3 3l3-3 3 3"/>
                  </svg>
                  {{ route.legs.length - 1 }}&nbsp;{{ route.legs.length - 1 === 1 ? t('planChange') : t('planChanges') }}
                </span>
                <span v-if="route.isLive" class="stat-chip stat-chip-live">
                  <span class="live-dot"></span>{{ t('live') }}
                </span>
                <span v-if="route.nextTimes.length > 1" class="stat-chip-next">
                  <span class="stat-chip-label">{{ t('planThen') }}</span>
                  <template v-for="(entry, i) in route.nextTimes.slice(1, 3)" :key="i"><span class="stat-chip-time">{{ entry.is_live ? '' : '~\u202f' }}{{ formatMinutesFromNow(entry.minutes, userTime || new Date(), t('now')) }}</span></template>
                </span>
              </div>
            </div>
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
                {{ t('planNoResults') }}</p>
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

.leg-line-dashed {
  background: repeating-linear-gradient(to bottom, #cbd5e1 0, #cbd5e1 4px, transparent 4px, transparent 8px) !important;
}

.leg-row-alight-transfer .leg-label-col {
  padding-bottom: 0;
}

.transfer-block {
  display: flex;
  align-items: stretch;
  gap: 0.875rem;
  margin: 0.5rem 0 0.6rem;
}

.transfer-block-rail {
  width: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  position: relative;
}

.transfer-block-rail::before,
.transfer-block-rail::after {
  content: "";
  position: absolute;
  left: 50%;
  width: 2px;
  margin-left: -1px;
  background: repeating-linear-gradient(to bottom, #cbd5e1 0, #cbd5e1 3px, transparent 3px, transparent 6px);
}

.transfer-block-rail::before {
  top: -0.5rem;
  height: calc(50% - 0.65rem);
}

.transfer-block-rail::after {
  bottom: -0.6rem;
  height: calc(50% - 0.65rem);
}

.dark .transfer-block-rail::before,
.dark .transfer-block-rail::after {
  background: repeating-linear-gradient(to bottom, #475569 0, #475569 3px, transparent 3px, transparent 6px);
}

.transfer-block-icon {
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 9999px;
  background: #fef3c7;
  color: #b45309;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  z-index: 1;
}

.transfer-block-icon svg {
  width: 0.85rem;
  height: 0.85rem;
}

.dark .transfer-block-icon {
  background: rgba(245, 158, 11, 0.18);
  color: #fbbf24;
}

.transfer-block-content {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex: 1;
  min-width: 0;
  padding: 0.35rem 0.65rem;
  border-radius: 0.55rem;
  background: #fffbeb;
  border: 1px dashed #fde68a;
}

.dark .transfer-block-content {
  background: rgba(245, 158, 11, 0.08);
  border-color: rgba(251, 191, 36, 0.3);
}

.transfer-block-label {
  font-size: 0.65rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: #b45309;
}

.dark .transfer-block-label {
  color: #fbbf24;
}

.transfer-block-detail {
  font-size: 0.75rem;
  font-weight: 600;
  color: #92400e;
}

.dark .transfer-block-detail {
  color: #fcd34d;
}

html[data-traditional] .transfer-block-icon,
html[data-traditional] .transfer-block-content {
  border-radius: 0;
}

.departure-card {
  display: flex;
  align-items: stretch;
  gap: 0.625rem;
  padding: 0.875rem 0.875rem 0.875rem 0;
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

.departure-card.is-selected {
  border-color: #0ea5e9;
  background: #f0f9ff;
  box-shadow: 0 4px 12px rgba(14, 165, 233, 0.12);
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

.card-rail {
  width: 4px;
  flex-shrink: 0;
  border-radius: 0 4px 4px 0;
  background: transparent;
  transition: background 0.15s;
}

.card-rail.is-active {
  background: #0ea5e9;
}

.dark .card-rail.is-active {
  background: #38bdf8;
}

.card-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  padding-left: 0.5rem;
}

.card-row-primary {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  min-width: 0;
}

.bus-chain {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  flex: 1;
  min-width: 0;
  flex-wrap: wrap;
}

.bus-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.25rem 0.55rem;
  min-width: 2.25rem;
  height: 1.75rem;
  border-radius: 0.5rem;
  font-weight: 800;
  font-size: 0.78rem;
  letter-spacing: 0.01em;
  color: white;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
  white-space: nowrap;
}

.bus-chip-name {
  line-height: 1;
}

.bus-chip-sep {
  opacity: 0.65;
  margin: 0 0.18rem;
  font-weight: 600;
}

.bus-chain-arrow {
  width: 0.95rem;
  height: 0.95rem;
  color: #94a3b8;
  flex-shrink: 0;
}

.dark .bus-chain-arrow {
  color: #64748b;
}

.card-primary-time {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.95rem;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  margin-left: auto;
}

.card-primary-time-sched {
  color: #334155;
}

.dark .card-primary-time-sched {
  color: #e2e8f0;
}

.card-primary-time-live {
  color: #059669;
}

.dark .card-primary-time-live {
  color: #34d399;
}

.live-dot {
  display: inline-block;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
  background: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.18);
  animation: live-pulse 1.4s ease-in-out infinite;
}

@keyframes live-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.45; }
}

.card-chevron {
  width: 1rem;
  height: 1rem;
  color: #cbd5e1;
  flex-shrink: 0;
  transition: color 0.15s, transform 0.15s;
}

.departure-card:hover .card-chevron {
  color: #64748b;
  transform: translateX(2px);
}

.departure-card.is-selected .card-chevron {
  color: #0ea5e9;
}

.dark .card-chevron {
  color: #475569;
}

.dark .departure-card:hover .card-chevron {
  color: #94a3b8;
}

.dark .departure-card.is-selected .card-chevron {
  color: #38bdf8;
}

.card-row-meta {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  min-width: 0;
}

.card-arrow {
  color: #94a3b8;
  font-weight: 600;
  font-size: 0.85rem;
  flex-shrink: 0;
}

.card-dest {
  font-size: 0.85rem;
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

.card-row-stats {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.4rem;
  font-size: 0.7rem;
}

.stat-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.15rem 0.45rem;
  border-radius: 9999px;
  background: #f1f5f9;
  color: #475569;
  font-weight: 600;
  white-space: nowrap;
}

.dark .stat-chip {
  background: #334155;
  color: #cbd5e1;
}

.stat-chip-transfer {
  color: #475569;
  background: #e2e8f0;
}

.dark .stat-chip-transfer {
  color: #cbd5e1;
  background: #334155;
}

.stat-chip-live {
  background: #ecfdf5;
  color: #059669;
}

.dark .stat-chip-live {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
}

.stat-chip-next {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: #94a3b8;
  font-weight: 500;
}

.stat-chip-label {
  text-transform: lowercase;
  font-size: 0.7rem;
}

.stat-chip-time {
  font-weight: 700;
  color: #64748b;
  font-variant-numeric: tabular-nums;
}

.dark .stat-chip-time {
  color: #cbd5e1;
}

.dark .stat-chip-next {
  color: #64748b;
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

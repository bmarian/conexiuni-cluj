<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts"
import {storeToRefs} from "pinia"
import {computed, onMounted, onUnmounted, ref, watch} from "vue"
import {useHead} from '@unhead/vue'
import {useI18n} from "vue-i18n"
import {useStopInfoApi} from "@/composables/useStopInfoApi.ts"
import StopIcon from "@/assets/stop.svg"
import IconHeartFilled from "@/components/icons/IconHeartFilled.vue"
import ViewErrorState from "@/components/ViewErrorState.vue"
import IconHeartOutline from "@/components/icons/IconHeartOutline.vue"
import {
  OUTGOING_SUFFIX,
  type Shape,
  type ShapeInfo,
  type Vehicle,
  type VehiclesInStop
} from "@/types/tranzy.ts";
import {
  formatMinutesFromNow,
  hasTimetableEntries,
  getAvailableBusesForStop
} from "@/utils/time.ts";
import {type DisplayShape, useMapStore} from "@/stores/map.ts";
import {
  buildShapeIndex,
  buildStopShapeIdxByStopId,
  etaForStop,
  getIndexedVehicles,
  type IndexedVehicle
} from "@/composables/useVehicleTracking.ts";
import {useVehicleStream} from "@/composables/useVehicleStream.ts";
import {useRouteStore} from "@/stores/route.ts";
import {useFavoritesStore} from "@/stores/favorites.ts";
import {useRouter} from "vue-router";
import HeaderNavigation from "@/components/HeaderNavigation.vue"
import {
  getRouteIdFromTripId,
  getShapeStopTimes,
  getTripIdForRouteAtStop
} from "@/utils/trips.ts";
import {useSettingsStore} from "@/stores/settings.ts"
import ShareButton from "@/components/ShareButton.vue";

const props = defineProps<{ stopId: string }>()

const {t} = useI18n()
const userStore = useUserStore()
const settings = useSettingsStore()
const mapStore = useMapStore()
const routeStore = useRouteStore()
const favoritesStore = useFavoritesStore()
const router = useRouter()
const stopIdNum = computed(() => Number(props.stopId))
const isFavorite = computed(() => favoritesStore.isStopFavorite(stopIdNum.value))
const {userTime} = storeToRefs(userStore)
const {zoomOut} = storeToRefs(mapStore)
const {stopInfo, fetchStopData} = useStopInfoApi()
const stopName = computed(() => stopInfo.value?.stop_name)

useHead(() => ({
  title: stopName.value ? t('headStopTitle', {stopName: stopName.value}) : 'Conexiuni Cluj',
  meta: [
    {name: 'description', content: stopName.value ? t('headStopDesc', {stopName: stopName.value}) : ''},
    {property: 'og:title', content: stopName.value ? t('headStopTitle', {stopName: stopName.value}) : 'Conexiuni Cluj'},
    {property: 'og:description', content: stopName.value ? t('headStopDesc', {stopName: stopName.value}) : ''},
    {property: 'og:url', content: `https://bus.bmarian.online/stop/${props.stopId}`},
  ],
  link: [{rel: 'canonical', href: `https://bus.bmarian.online/stop/${props.stopId}`}],
}))
const isLoading = ref(false)
const loadError = ref(false)
const isComputingDepartures = ref(true)
const shapesComingToTheStopBasedOnVehiclePositions = ref<VehiclesInStop[]>([])
const initialZoomAppliedStopId = ref<string | null>(null)

const applyInitialZoomOutForCurrentStop = (shouldZoomOut: boolean) => {
  const loadedStopId = stopInfo.value?.stop_id?.toString()
  if (!loadedStopId || loadedStopId !== props.stopId) return
  if (initialZoomAppliedStopId.value === props.stopId) return
  if (!shouldZoomOut) return

  zoomOut.value = true
  initialZoomAppliedStopId.value = props.stopId
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

function formatMinutes(minutes: number): string {
  return formatMinutesFromNow(minutes, userTime.value || new Date(), t('now'))
}

const busesWithAvailableTimetables = computed(() => {
  return stopInfo.value?.shapes_info.filter((shape: ShapeInfo) => hasTimetableEntries(shape.timetable))
})

const busesWithAvailableTimetablesSorted = computed(() => {
  return [...(busesWithAvailableTimetables.value || [])].sort((a: ShapeInfo, b: ShapeInfo) => {
    const aFav = favoritesStore.isRouteFavorite(a.route_id) ? 0 : 1
    const bFav = favoritesStore.isRouteFavorite(b.route_id) ? 0 : 1
    if (aFav !== bFav) return aFav - bFav
    return a.route_short_name.localeCompare(b.route_short_name, undefined, {numeric: true})
  })
})

const shapeInfoByRouteId = computed(() => {
  const info = stopInfo.value
  if (!info) return new Map<number, ShapeInfo>()
  return new Map<number, ShapeInfo>(info.shapes_info.map((shapeInfo: ShapeInfo) => [shapeInfo.route_id, shapeInfo]))
})

const departuresSorted = computed(() => {
  return [...shapesComingToTheStopBasedOnVehiclePositions.value].sort((a, b) => {
    const aFav = favoritesStore.isRouteFavorite(a.route_id) ? 0 : 1
    const bFav = favoritesStore.isRouteFavorite(b.route_id) ? 0 : 1
    if (aFav !== bFav) return aFav - bFav
    return a.minutes_left - b.minutes_left
  })
})

const shapesComingToTheStopBasedOnTimetable = computed(() => {
  if (!stopInfo.value) return []
  return getAvailableBusesForStop(
    stopInfo.value,
    userTime.value || new Date(),
    {maxMinutes: 480, limit: 3}
  )
})

watch(() => props.stopId, async (newValue) => {
  isLoading.value = true
  loadError.value = false
  isComputingDepartures.value = true
  shapesComingToTheStopBasedOnVehiclePositions.value = []
  await fetchStopData(newValue)
  if (!stopInfo.value) loadError.value = true
  isLoading.value = false
}, {immediate: true})

watch([shapesComingToTheStopBasedOnTimetable, vehiclesByTrip], async ([shapesComingNext]) => {
  if (!Array.isArray(shapesComingNext) || shapesComingNext.length === 0) {
    shapesComingToTheStopBasedOnVehiclePositions.value = []
    mapStore.setVehiclesToDisplay([])
    mapStore.setLoadedShapes([])
    isComputingDepartures.value = false
    return
  }

  const displayShapes = getShapesDisplay(busesWithAvailableTimetables.value)
  const displayShapesWithTrip = await mapStore.requestShapes(displayShapes)
  if (!Array.isArray(displayShapesWithTrip) || displayShapesWithTrip.length === 0) {
    shapesComingToTheStopBasedOnVehiclePositions.value = shapesComingNext
    mapStore.setVehiclesToDisplay([])
    mapStore.setLoadedShapes([])
    isComputingDepartures.value = false
    return
  }

  const vehiclesByTripMap = vehiclesByTrip.value
  const favoriteVehicles: IndexedVehicle[] = []
  const favoriteTripIds = new Set<string>()

  const results: VehiclesInStop[] = []
  for (const shape of shapesComingNext) {
    const trip = displayShapesWithTrip.find(([s]) => s.trip_id === shape.trip_id)?.[1] as Shape[]
    if (!Array.isArray(trip) || !trip.length) {
      results.push(shape)
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

    if (favoritesStore.isRouteFavorite(shape.route_id)) {
      favoriteVehicles.push(...vehiclesOnRoute)
      if (vehiclesOnRoute.length) favoriteTripIds.add(shape.trip_id)
    }

    if (!vehiclesOnRoute.length) {
      results.push(shape)
      continue
    }

    const routeShapeInfo = shapeInfoByRouteId.value.get(shape.route_id)
    if (!routeShapeInfo) {
      results.push(shape)
      continue
    }

    const tripStops = getShapeStopTimes(routeShapeInfo).filter((stopTime) => stopTime.trip_id === shape.trip_id)
    const stopShapeIdx = buildStopShapeIdxByStopId(tripStops, trip).get(stopIdNum.value) ?? -1
    if (stopShapeIdx < 0) {
      results.push(shape)
      continue
    }

    const eta = etaForStop(stopShapeIdx, vehiclesOnRoute, shapeIndex)
    if (!eta) {
      results.push(shape)
      continue
    }

    results.push({
      ...shape,
      minutes_left: eta.etaMinutes,
      next_times: [
        {minutes: eta.etaMinutes, is_live: true},
        ...(shape.next_times || []).slice(1),
      ],
      static_time_approximation: false,
    })
  }
  shapesComingToTheStopBasedOnVehiclePositions.value = results.sort((a, b) => a.minutes_left - b.minutes_left)
  const highlightedShapes = displayShapesWithTrip.filter(([displayShape, shapePoints]) =>
    favoriteTripIds.has(displayShape.trip_id) && Array.isArray(shapePoints) && shapePoints.length > 0,
  )
  mapStore.setLoadedShapes(
    highlightedShapes,
  )
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

function routeDestination(name: string): string {
  const i = name.lastIndexOf(' - ')
  return i >= 0 ? name.slice(i + 3) : name
}

function routeOrigin(name: string): string {
  const i = name.lastIndexOf(' - ')
  return i >= 0 ? name.slice(0, i) : ''
}

const navigateToRoute = (shape: VehiclesInStop) => {
  const si = stopInfo.value?.shapes_info?.find((s: ShapeInfo) => s.route_id === shape.route_id)
  if (!si) return
  routeStore.setSelectedRoute(si, shape.trip_id, props.stopId, stopName.value || '')
  router.push({
    name: 'route',
    params: {
      routeId: shape.route_id,
      direction: shape.trip_id.endsWith(OUTGOING_SUFFIX) ? '0' : '1'
    }
  })
}

const navigateToAllRoute = (shape: ShapeInfo) => {
  const tripId = getTripIdForRouteAtStop(stopInfo.value?.outgoing_trip_ids || [], stopInfo.value?.incoming_trip_ids || [], shape.route_id) || `${shape.route_id}${OUTGOING_SUFFIX}`
  routeStore.setSelectedRoute(shape, tripId, props.stopId, stopName.value || '')
  router.push({
    name: 'route',
    params: {routeId: shape.route_id, direction: tripId.endsWith(OUTGOING_SUFFIX) ? '0' : '1'}
  })
}

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
</script>

<template>
  <div v-if="isLoading"
       class="stop-view-container bg-white dark:bg-[#0f172a] animate-pulse flex flex-col gap-8">
    <header class="flex items-center gap-4">
      <div class="w-11 h-11 rounded-full bg-slate-200 dark:bg-slate-800 shrink-0"></div>
      <div class="h-7 w-44 bg-slate-200 dark:bg-slate-800 rounded-lg"></div>
    </header>
    <section class="flex flex-col gap-3">
      <div class="h-3 w-32 bg-slate-200 dark:bg-slate-800 rounded mb-2"></div>
      <div v-for="i in 4" :key="i"
           class="flex items-center gap-3 rounded-2xl p-3 border border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/30">
        <div class="w-11 h-9 rounded-xl bg-slate-200 dark:bg-slate-700 shrink-0"></div>
        <div class="flex-1 flex flex-col gap-1.5">
          <div class="h-2.5 w-16 bg-slate-200 dark:bg-slate-700 rounded"></div>
          <div class="h-4 w-36 bg-slate-200 dark:bg-slate-700 rounded"></div>
        </div>
        <div class="flex gap-1.5">
          <div v-for="j in 3" :key="j"
               class="w-9 h-6 rounded-lg bg-slate-200 dark:bg-slate-700"></div>
        </div>
      </div>
    </section>
    <section class="flex flex-col gap-2">
      <div class="h-3 w-40 bg-slate-200 dark:bg-slate-800 rounded mb-2"></div>
      <div v-for="i in 5" :key="i" class="flex items-center gap-3 py-2">
        <div class="w-10 h-7 rounded-md bg-slate-200 dark:bg-slate-800 shrink-0"></div>
        <div class="h-3.5 w-40 bg-slate-200 dark:bg-slate-800 rounded"></div>
      </div>
    </section>
  </div>

  <div v-else-if="loadError"
       class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col">
    <ViewErrorState/>
  </div>

  <div v-else
       class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-8">

    <div class="flex items-center -mb-4">
      <HeaderNavigation />
    </div>

    <header class="flex items-start gap-4">
      <div
        class="w-14 h-14 shrink-0 rounded-2xl bg-gradient-to-br from-emerald-400 to-teal-600 flex items-center justify-center shadow-lg shadow-emerald-500/20 mt-0.5">
        <span v-if="settings.traditionalActive" class="emoji-icon-xl" aria-hidden="true">🚏</span>
        <StopIcon v-else class="w-7 h-7 text-white"/>
      </div>
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2 mb-0.5">
          <span
            class="text-[10px] font-semibold text-emerald-600 dark:text-emerald-400 tracking-wide">{{
              t('busStop')
            }}</span>
          <span v-if="stopInfo?.stop_code"
                class="text-[10px] font-semibold text-slate-400 dark:text-slate-500 font-mono tracking-wide">#{{
              stopInfo.stop_code
            }}</span>
        </div>
        <h1 class="text-2xl font-black tracking-tight text-slate-900 dark:text-white leading-tight">
          {{ stopName }}
        </h1>
      </div>
      <ShareButton class="mt-1"/>
      <button
        type="button"
        class="fav-btn mt-1 shrink-0"
        :class="{ 'is-fav': isFavorite }"
        :title="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-label="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-pressed="isFavorite"
        @click="favoritesStore.toggleStopFavorite(stopIdNum)"
      >
        <IconHeartFilled v-if="isFavorite" class="w-5 h-5"/>
        <IconHeartOutline v-else class="w-5 h-5"/>
      </button>
    </header>

    <section>
      <h2 class="section-label">
        <span
          class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse shadow-[0_0_6px_rgba(16,185,129,0.6)] shrink-0"></span>
        {{ t('nextDepartures') }}
      </h2>

      <div v-if="isComputingDepartures" class="flex flex-col gap-2.5 animate-pulse">
        <div v-for="i in 3" :key="i"
             class="flex items-center gap-2.5 rounded-2xl p-3 border border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/30">
          <div class="w-1 self-stretch rounded-full bg-slate-200 dark:bg-slate-700 shrink-0"></div>
          <div class="w-11 h-9 rounded-xl bg-slate-200 dark:bg-slate-700 shrink-0"></div>
          <div class="flex-1 flex flex-col gap-1.5">
            <div class="h-2.5 w-12 bg-slate-200 dark:bg-slate-700 rounded"></div>
            <div class="h-4 w-32 bg-slate-200 dark:bg-slate-700 rounded"></div>
          </div>
          <div class="flex gap-1">
            <div v-for="j in 3" :key="j"
                 class="w-10 h-6 rounded-lg bg-slate-200 dark:bg-slate-700"></div>
          </div>
          <div class="w-4 h-4 rounded bg-slate-100 dark:bg-slate-800 shrink-0"></div>
        </div>
      </div>
      <template v-else>
        <p v-if="!shapesComingToTheStopBasedOnVehiclePositions.length"
           class="text-sm text-slate-400 dark:text-slate-500 py-2">
          {{ t('noSchedule') }}
        </p>
        <div class="flex flex-col gap-2.5">
          <div
            v-for="shape in departuresSorted"
            :key="shape.route_short_name"
            @click="navigateToRoute(shape)"
            class="departure-card group"
            :class="{ 'departure-card-fav': favoritesStore.isRouteFavorite(shape.route_id) }"
          >
            <div
              :class="['w-1 self-stretch rounded-full shrink-0', !shape.static_time_approximation ? 'bg-emerald-500' : 'bg-transparent']"></div>

            <div
              class="flex items-center justify-center shrink-0 w-11 h-9 rounded-xl font-black text-sm text-white shadow-sm"
              :style="{ backgroundColor: shape.route_color }"
            >{{ shape.route_short_name }}
            </div>

            <div class="flex-1 min-w-0 flex flex-col justify-center">
              <div class="flex items-center gap-1.5 mb-0.5">
                <span v-if="!shape.static_time_approximation" class="live-badge">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
                  {{ t('live') }}
                </span>
                <span class="card-dest">→ {{ routeDestination(shape.route_long_name) }}</span>
              </div>
              <span class="card-origin">{{ routeOrigin(shape.route_long_name) }}</span>
            </div>

            <div class="flex items-center gap-1 shrink-0">
              <span
                v-for="(t, i) in shape.next_times"
                :key="i"
                :class="[
                  'time-pill',
                  i === 0 && !shape.static_time_approximation
                    ? 'time-pill-live'
                    : 'time-pill-sched',
                  i > 0 ? 'time-pill-extra' : ''
                ]"
              >{{
                  i === 0 && shape.static_time_approximation ? '~\u202f' : ''
                }}{{ formatMinutes(t.minutes) }}</span>
            </div>

            <svg
              class="w-4 h-4 text-slate-400 dark:text-slate-500 shrink-0 group-hover:text-slate-600 dark:group-hover:text-slate-300 transition-colors"
              fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </div>
        </div>
      </template>
    </section>

    <section class="pb-6">
      <h2 class="section-label">
        <span v-if="settings.traditionalActive" class="emoji-icon" aria-hidden="true">🗺️</span>
        <svg v-else class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500 shrink-0" fill="none"
             viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round"
                d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
        </svg>
        {{ t('allRoutesAtStop') }}
      </h2>

      <div class="flex flex-col divide-y divide-slate-100 dark:divide-slate-800/60">
        <div
          v-for="shape in busesWithAvailableTimetablesSorted"
          :key="shape.route_short_name"
          @click="navigateToAllRoute(shape)"
          class="all-route-row group"
          :class="{ 'all-route-row-fav': favoritesStore.isRouteFavorite(shape.route_id) }"
        >
          <div
            class="flex items-center justify-center shrink-0 w-10 h-7 rounded-md text-xs font-black text-white shadow-sm opacity-90 group-hover:opacity-100 transition-opacity"
            :style="{ backgroundColor: shape.route_color }"
          >{{ shape.route_short_name }}
          </div>

          <span
            class="flex-1 text-sm font-medium text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
            {{ shape.timetable.route_long_name }}
          </span>

          <svg
            class="w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
          </svg>
        </div>
      </div>
    </section>

  </div>
</template>

<style scoped>
.stop-view-container {
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
  margin-bottom: 0.875rem;
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

/* Hide 2nd + 3rd pill when card is narrow */
@container (max-width: 300px) {
  .time-pill-extra {
    display: none;
  }
}

.all-route-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 0.25rem;
  cursor: pointer;
  transition: background 0.15s;
  border-radius: 0.5rem;
  margin: 0 -0.25rem;
}

.all-route-row:hover {
  background: #f8fafc;
}

.departure-card-fav {
  background: #fff1f2;
  border-color: #fecdd3;
}

.departure-card-fav:hover {
  background: #ffe4e6;
  border-color: #fda4af;
}

.all-route-row-fav {
  background: #fff1f2;
}

.all-route-row-fav:hover {
  background: #ffe4e6 !important;
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

.card-origin {
  font-size: 0.6875rem;
  font-weight: 500;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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

</style>

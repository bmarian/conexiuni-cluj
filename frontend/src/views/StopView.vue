<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts"
import {storeToRefs} from "pinia"
import {computed, onMounted, onUnmounted, ref, watch} from "vue"
import {useI18n} from "vue-i18n"
import {useStopInfoApi} from "@/composables/useStopInfoApi.ts"
import StopIcon from "@/assets/stop.svg"
import IconBack from "@/components/icons/IconBack.vue"
import IconNotFoundFace from "@/components/icons/IconNotFoundFace.vue"
import IconHeartFilled from "@/components/icons/IconHeartFilled.vue"
import IconHeartOutline from "@/components/icons/IconHeartOutline.vue"
import type {Timetable} from "@/types/ctp.ts";
import {
  INCOMING_SUFFIX,
  OUTGOING_SUFFIX,
  type Shape,
  type ShapeInfo,
  type TimeEntry,
  type Vehicle,
  type VehiclesInStop
} from "@/types/tranzy.ts";
import {getMinutesFromDate, timeStringToMinutes} from "@/utils/time.ts";
import {type DisplayShape, useMapStore} from "@/stores/map.ts";
import {haversineMeters} from "@/utils/geo.ts";
import {getClosestNodeToPoint, getVehiclesOnRoute, getClosestVehicleBeforeStop, computeETA, type TrackedVehicle} from "@/composables/useVehicleTracking.ts";
import {useVehicleStream} from "@/composables/useVehicleStream.ts";
import {useRouteStore} from "@/stores/route.ts";
import {useFavoritesStore} from "@/stores/favorites.ts";
import {onBeforeRouteLeave, useRouter} from "vue-router";

const props = defineProps<{ stopId: string }>()

const {t} = useI18n()
const userStore = useUserStore()
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
  const shapes = info.shapes_info.filter((s: ShapeInfo) => !!(s.timetable?.weekdays?.entries?.length || s.timetable?.saturday?.entries?.length || s.timetable?.sunday?.entries?.length))
  const routeIds = new Set(shapes.map((s: ShapeInfo) => s.route_id))
  return [...(info.outgoing_trip_ids || []), ...(info.incoming_trip_ids || [])]
    .filter((tid) => routeIds.has(Number(tid.replace(OUTGOING_SUFFIX, '').replace(INCOMING_SUFFIX, ''))))
})
const {vehiclesByTrip} = useVehicleStream(streamTripIds)

function formatMinutes(minutes: number): string {
  if (minutes === 0) return t('now')
  if (minutes < 60) return `${minutes}m`
  const now = userTime.value || new Date()
  const future = new Date(now.getTime() + minutes * 60_000)
  return `${future.getHours().toString().padStart(2, '0')}:${future.getMinutes().toString().padStart(2, '0')}`
}

const hasTimetable = (timetable: Timetable): boolean => {
  return !!(timetable?.weekdays?.entries?.length || timetable?.saturday?.entries?.length || timetable?.sunday?.entries?.length)
}

const isAvailableToday = (today: number, timetable: Timetable): boolean => {
  return !!(today === 0 ? timetable?.sunday?.entries?.length : today === 6 ? timetable?.saturday?.entries?.length : timetable?.weekdays?.entries?.length)
}

const busesWithAvailableTimetables = computed(() => {
  return stopInfo.value?.shapes_info.filter((shape: ShapeInfo) => hasTimetable(shape.timetable))
})

const busesWithAvailableTimetablesSorted = computed(() => {
  return [...(busesWithAvailableTimetables.value || [])].sort((a: ShapeInfo, b: ShapeInfo) => {
    const aFav = favoritesStore.isRouteFavorite(a.route_id) ? 0 : 1
    const bFav = favoritesStore.isRouteFavorite(b.route_id) ? 0 : 1
    if (aFav !== bFav) return aFav - bFav
    return a.route_short_name.localeCompare(b.route_short_name, undefined, {numeric: true})
  })
})

const departuresSorted = computed(() => {
  return [...shapesComingToTheStopBasedOnVehiclePositions.value].sort((a, b) => {
    const aFav = favoritesStore.isRouteFavorite(a.route_id) ? 0 : 1
    const bFav = favoritesStore.isRouteFavorite(b.route_id) ? 0 : 1
    if (aFav !== bFav) return aFav - bFav
    return a.minutes_left - b.minutes_left
  })
})

const getTimeOffsetToStop = (stopTime: any[], tripId: string): number => {
  const propStopId = Number(props.stopId)
  let timeOffset = 0
  for (let j = 0; j < stopTime.length; j++) {
    const {trip_id, stop_id, offset_arrival_time} = stopTime[j]!
    if (trip_id !== tripId) continue
    if (stop_id === propStopId) break
    timeOffset += Math.ceil(offset_arrival_time / 60)
  }
  return timeOffset
}

const getTripId = (outgoingTripIds: string[], incomingTripIds: string[], routeId: string) => {
  return [...(outgoingTripIds || []), ...(incomingTripIds || [])].find((id) => id.startsWith(routeId))
}

const reverseRouteLongName = (routeLongName: string) => {
  return routeLongName.split(' - ').reverse().join(' - ')
}

const getShapesDisplay = (availableShapes: unknown): DisplayShape[] => {
  if (!Array.isArray(availableShapes) || availableShapes.length === 0) return []
  const routeIdsWithAvailableTimetables = availableShapes.map((shape: ShapeInfo) => shape.route_id)
  const {outgoing_trip_ids, incoming_trip_ids} = stopInfo.value
  return [...(outgoing_trip_ids || []), ...(incoming_trip_ids || [])]
    .filter((trip_id) => routeIdsWithAvailableTimetables.some((route_id: number) => trip_id.startsWith(route_id.toString())))
    .reduce((acc: DisplayShape[], trip_id: string) => {
      const routeId = Number(trip_id.replace(OUTGOING_SUFFIX, '').replace(INCOMING_SUFFIX, ''))
      const shape = availableShapes.find((shape: ShapeInfo) => shape.route_id === routeId)
      if (shape) acc.push({trip_id, route_short_name: shape.route_short_name, route_long_name: shape.timetable?.route_long_name || '', route_color: shape.route_color, route_type: shape.route_type})
      return acc
    }, [])
}

const shapesComingToTheStopBasedOnTimetable = computed(() => {
  if (!stopInfo.value) return []
  const today = userTime.value?.getDay() ?? new Date().getDay()
  const currentTimeInMinutes = getMinutesFromDate(userTime.value || new Date())
  const {outgoing_trip_ids, incoming_trip_ids, shapes_info} = stopInfo.value

  const results: VehiclesInStop[] = []
  for (const shapeInfo of shapes_info) {
    const {route_short_name, route_type, route_color, route_id, timetable} = shapeInfo
    const stop_time = (shapeInfo as any).stop_time ?? (shapeInfo as any).stop_times ?? []
    if (!hasTimetable(timetable) || !isAvailableToday(today, timetable)) continue

    const tripId = getTripId(outgoing_trip_ids, incoming_trip_ids, route_id)
    if (!tripId) continue

    const timeOffsetToStop = getTimeOffsetToStop(stop_time, tripId)
    const isOutgoing = tripId.endsWith(OUTGOING_SUFFIX)
    const todayTimetable = today === 0 ? timetable.sunday : today === 6 ? timetable.saturday : timetable.weekdays

    const upcomingMinutes: number[] = todayTimetable.entries
      .map((entry: { departure_in: string; departure_out: string }) => {
        const t = timeStringToMinutes(isOutgoing ? entry.departure_in : entry.departure_out)
        return t === null ? null : t + timeOffsetToStop
      })
      .filter((t: number | null): t is number => t !== null)
      .map((t: number) => ((t - currentTimeInMinutes) + 1440) % 1440)
      .filter((m: number) => m < 480)
      .sort((a: number, b: number) => a - b)
      .slice(0, 3)

    if (!upcomingMinutes.length) continue

    results.push({
      minutes_left: upcomingMinutes[0]!,
      next_times: upcomingMinutes.map((m) => ({minutes: m, is_live: false} as TimeEntry)),
      route_short_name,
      route_type,
      route_color,
      trip_id: tripId,
      route_id,
      route_long_name: isOutgoing ? timetable.route_long_name : reverseRouteLongName(timetable.route_long_name),
      static_time_approximation: true,
    })
  }
  return results.sort((a, b) => a.minutes_left - b.minutes_left)
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
  const favoriteVehicles: TrackedVehicle[] = []
  const favoriteTripIds = new Set<string>()

  const results: VehiclesInStop[] = []
  for (const shape of shapesComingNext) {
    const trip = displayShapesWithTrip.find(([s]) => s.trip_id === shape.trip_id)?.[1] as Shape[]
    const vehiclesOnRoute = await getVehiclesOnRoute(shape.trip_id, shape.route_short_name, shape.route_color, trip, userTime.value, vehiclesByTripMap.get(shape.trip_id) ?? [])

    if (favoritesStore.isRouteFavorite(shape.route_id)) {
      favoriteVehicles.push(...vehiclesOnRoute)
      if (vehiclesOnRoute.length) favoriteTripIds.add(shape.trip_id)
    }

    if (!vehiclesOnRoute.length) {
      results.push(shape)
      continue
    }

    const closestNodeToStop = getClosestNodeToPoint({lat: stopInfo.value!.stop_lat, lon: stopInfo.value!.stop_lon}, trip)
    if (!closestNodeToStop) {
      results.push(shape)
      continue
    }

    const {closestVehicle, closestNode} = getClosestVehicleBeforeStop(vehiclesOnRoute, closestNodeToStop, trip)
    if (!closestVehicle || !closestNode) {
      results.push(shape)
      continue
    }

    const eta = computeETA(closestNodeToStop, closestNode, closestVehicle, trip)
    if (eta === -1) { results.push(shape); continue }
    if (eta === -2) continue

    results.push({
      ...shape,
      minutes_left: eta,
      next_times: [
        {minutes: eta, is_live: true},
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

const goBack = () => {
  router.replace({name: 'home'})
}

const navigateToRoute = (shape: VehiclesInStop) => {
  const si = stopInfo.value?.shapes_info?.find((s: ShapeInfo) => s.route_id === shape.route_id)
  if (!si) return
  routeStore.setSelectedRoute(si, shape.trip_id, props.stopId, stopName.value || '')
  router.push({name: 'route', params: {routeId: shape.route_id, direction: shape.trip_id.endsWith(OUTGOING_SUFFIX) ? '0' : '1'}})
}

const navigateToAllRoute = (shape: ShapeInfo) => {
  const tripId = getTripId(stopInfo.value?.outgoing_trip_ids || [], stopInfo.value?.incoming_trip_ids || [], shape.route_id.toString()) || `${shape.route_id}${OUTGOING_SUFFIX}`
  routeStore.setSelectedRoute(shape, tripId, props.stopId, stopName.value || '')
  router.push({name: 'route', params: {routeId: shape.route_id, direction: tripId.endsWith(OUTGOING_SUFFIX) ? '0' : '1'}})
}
</script>

<template>
  <div v-if="isLoading" class="stop-view-container bg-white dark:bg-[#0f172a] animate-pulse flex flex-col gap-8">
    <header class="flex items-center gap-4">
      <div class="w-11 h-11 rounded-full bg-slate-200 dark:bg-slate-800 shrink-0"></div>
      <div class="h-7 w-44 bg-slate-200 dark:bg-slate-800 rounded-lg"></div>
    </header>
    <section class="flex flex-col gap-3">
      <div class="h-3 w-32 bg-slate-200 dark:bg-slate-800 rounded mb-2"></div>
      <div v-for="i in 4" :key="i" class="flex items-center gap-3 rounded-2xl p-3 border border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/30">
        <div class="w-11 h-9 rounded-xl bg-slate-200 dark:bg-slate-700 shrink-0"></div>
        <div class="flex-1 flex flex-col gap-1.5">
          <div class="h-2.5 w-16 bg-slate-200 dark:bg-slate-700 rounded"></div>
          <div class="h-4 w-36 bg-slate-200 dark:bg-slate-700 rounded"></div>
        </div>
        <div class="flex gap-1.5">
          <div v-for="j in 3" :key="j" class="w-9 h-6 rounded-lg bg-slate-200 dark:bg-slate-700"></div>
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

  <div v-else-if="loadError" class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col items-center justify-center gap-5">
    <div class="w-14 h-14 rounded-2xl bg-slate-100 dark:bg-slate-800 flex items-center justify-center">
      <IconNotFoundFace class="w-7 h-7 text-slate-400 dark:text-slate-500"/>
    </div>
    <div class="text-center">
      <h1 class="text-lg font-black text-slate-800 dark:text-white mb-1">{{ t('notFound') }}</h1>
      <p class="text-sm text-slate-500 dark:text-slate-400 max-w-xs leading-relaxed">{{ t('notFoundDesc') }}</p>
    </div>
    <button
      @click="goBack"
      class="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 text-sm font-bold hover:opacity-90 transition-opacity"
    >
      <IconBack class="w-4 h-4"/>
      {{ t('back') }}
    </button>
  </div>

  <div v-else class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-8">

    <div class="flex items-center -mb-4">
      <button
        @click="goBack"
        class="flex items-center gap-1.5 px-2 py-1.5 rounded-xl text-sm font-semibold text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200 transition-colors duration-150"
      >
        <IconBack class="w-4 h-4"/>
        {{ t('back') }}
      </button>
    </div>

    <header class="flex items-start gap-4">
      <div class="w-14 h-14 shrink-0 rounded-2xl bg-gradient-to-br from-emerald-400 to-teal-600 flex items-center justify-center shadow-lg shadow-emerald-500/20 mt-0.5">
        <StopIcon class="w-7 h-7 text-white"/>
      </div>
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2 mb-0.5">
          <span class="text-[10px] font-bold text-emerald-600 dark:text-emerald-400 uppercase tracking-[0.18em]">{{ t('busStop') }}</span>
          <span v-if="stopInfo?.stop_code" class="text-[10px] font-semibold text-slate-400 dark:text-slate-500 font-mono tracking-wide">#{{ stopInfo.stop_code }}</span>
        </div>
        <h1 class="text-2xl font-black tracking-tight text-slate-900 dark:text-white leading-tight">
          {{ stopName }}
        </h1>
      </div>
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
        <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse shadow-[0_0_6px_rgba(16,185,129,0.6)] shrink-0"></span>
        {{ t('nextDepartures') }}
      </h2>

      <div v-if="isComputingDepartures" class="flex flex-col gap-2.5 animate-pulse">
        <div v-for="i in 3" :key="i" class="flex items-center gap-2.5 rounded-2xl p-3 border border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/30">
          <div class="w-1 self-stretch rounded-full bg-slate-200 dark:bg-slate-700 shrink-0"></div>
          <div class="w-11 h-9 rounded-xl bg-slate-200 dark:bg-slate-700 shrink-0"></div>
          <div class="flex-1 flex flex-col gap-1.5">
            <div class="h-2.5 w-12 bg-slate-200 dark:bg-slate-700 rounded"></div>
            <div class="h-4 w-32 bg-slate-200 dark:bg-slate-700 rounded"></div>
          </div>
          <div class="flex gap-1">
            <div v-for="j in 3" :key="j" class="w-10 h-6 rounded-lg bg-slate-200 dark:bg-slate-700"></div>
          </div>
          <div class="w-4 h-4 rounded bg-slate-100 dark:bg-slate-800 shrink-0"></div>
        </div>
      </div>
      <template v-else>
        <p v-if="!shapesComingToTheStopBasedOnVehiclePositions.length" class="text-sm text-slate-400 dark:text-slate-500 py-2">
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
            <div :class="['w-1 self-stretch rounded-full shrink-0', !shape.static_time_approximation ? 'bg-emerald-500' : 'bg-transparent']"></div>

            <div
              class="flex items-center justify-center shrink-0 w-11 h-9 rounded-xl font-black text-sm text-white shadow-sm"
              :style="{ backgroundColor: shape.route_color }"
            >{{ shape.route_short_name }}</div>

            <div class="flex-1 min-w-0 flex flex-col justify-center">
              <div class="flex items-center gap-1.5 mb-0.5">
                <span v-if="!shape.static_time_approximation" class="live-badge">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
                  {{ t('live') }}
                </span>
                <span v-else class="text-[10px] font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-wider">
                  {{ t('scheduledApprox') }}
                </span>
              </div>
              <span class="text-sm font-semibold text-slate-800 dark:text-slate-200 truncate leading-tight">
                {{ shape.route_long_name }}
              </span>
            </div>

            <div class="flex items-center gap-1 shrink-0">
              <span
                v-for="(t, i) in shape.next_times"
                :key="i"
                :class="[
                  'time-pill',
                  i === 0 && !shape.static_time_approximation
                    ? 'time-pill-live'
                    : 'time-pill-sched'
                ]"
              >{{ i === 0 && shape.static_time_approximation ? '~\u202f' : '' }}{{ formatMinutes(t.minutes) }}</span>
            </div>

            <svg class="w-4 h-4 text-slate-400 dark:text-slate-500 shrink-0 group-hover:text-slate-600 dark:group-hover:text-slate-300 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </div>
        </div>
      </template>
    </section>

    <section class="pb-6">
      <h2 class="section-label">
        <svg class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
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
          >{{ shape.route_short_name }}</div>

          <span class="flex-1 text-sm font-medium text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
            {{ shape.timetable.route_long_name }}
          </span>

          <svg class="w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
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
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.15em;
  color: #64748b;
  margin-bottom: 0.875rem;
}

@media (prefers-color-scheme: dark) {
  .section-label { color: #94a3b8; }
}

.departure-card {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.75rem 0.625rem 0.75rem 0.5rem;
  border-radius: 1rem;
  border: 1px solid #f1f5f9;
  background: #f8fafc;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
}

.departure-card:hover {
  background: white;
  border-color: #e2e8f0;
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}

@media (prefers-color-scheme: dark) {
  .departure-card {
    border-color: rgb(51 65 85 / 0.5);
    background: rgb(30 41 59 / 0.6);
  }
  .departure-card:hover {
    background: rgb(30 41 59 / 0.9);
    border-color: rgb(51 65 85 / 0.8);
  }
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

@media (prefers-color-scheme: dark) {
  .live-badge {
    color: #34d399;
    background: rgb(16 185 129 / 0.1);
    border-color: rgb(16 185 129 / 0.3);
  }
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

@media (prefers-color-scheme: dark) {
  .time-pill-sched {
    background: rgb(51 65 85 / 0.7);
    color: #cbd5e1;
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

.all-route-row:hover { background: #f8fafc; }

@media (prefers-color-scheme: dark) {
  .all-route-row:hover { background: rgb(30 41 59 / 0.5); }
}

.departure-card-fav {
  background: #fff1f2;
  border-color: #fecdd3;
}
.departure-card-fav:hover {
  background: #ffe4e6;
  border-color: #fda4af;
}

.all-route-row-fav { background: #fff1f2; }
.all-route-row-fav:hover { background: #ffe4e6 !important; }

@media (prefers-color-scheme: dark) {
  .departure-card-fav {
    background: rgb(244 63 94 / 0.08);
    border-color: rgb(244 63 94 / 0.25);
  }
  .departure-card-fav:hover {
    background: rgb(244 63 94 / 0.14);
    border-color: rgb(244 63 94 / 0.35);
  }
  .all-route-row-fav { background: rgb(244 63 94 / 0.06); }
  .all-route-row-fav:hover { background: rgb(244 63 94 / 0.1) !important; }
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
.fav-btn:active { transform: scale(0.92); }
.fav-btn.is-fav { color: #f43f5e; }
.fav-btn.is-fav:hover { background: #fee2e2; }

@media (prefers-color-scheme: dark) {
  .fav-btn { color: #64748b; }
  .fav-btn:hover {
    background: rgb(244 63 94 / 0.12);
    color: #fb7185;
  }
  .fav-btn.is-fav { color: #fb7185; }
  .fav-btn.is-fav:hover { background: rgb(244 63 94 / 0.2); }
}
</style>

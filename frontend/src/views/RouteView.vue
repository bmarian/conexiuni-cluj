<script setup lang="ts">
import {computed, nextTick, onMounted, onUnmounted, ref, watch, watchEffect} from 'vue'
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import {useRouteStore} from '@/stores/route.ts'
import {useUserStore} from '@/stores/user.ts'
import {useMapStore} from '@/stores/map.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {OUTGOING_SUFFIX, INCOMING_SUFFIX, type Shape, type StopTime} from '@/types/tranzy.ts'
import {getMinutesFromDate, timeStringToMinutes} from '@/utils/time.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {
  buildShapeIndex,
  etaForStop,
  fetchVehiclesForTrips,
  findClosestShapeIdx,
  getIndexedVehicles,
  type IndexedVehicle,
  type ShapeIndex,
} from '@/composables/useVehicleTracking.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'

const props = defineProps<{ routeId: string; direction: string }>()

const router = useRouter()
const {t} = useI18n()
const routeStore = useRouteStore()
const userStore = useUserStore()
const mapStore = useMapStore()
const favoritesStore = useFavoritesStore()
const {userTime, userLocation} = storeToRefs(userStore)
const {zoomOut, centerOnUser} = storeToRefs(mapStore)

const routeIdNum = computed(() => Number(props.routeId))
const isFavorite = computed(() => favoritesStore.isRouteFavorite(routeIdNum.value))

const shapeInfo = computed(() => routeStore.selectedShapeInfo)
const fromStopId = computed(() => routeStore.fromStopId)
const fromStopName = computed(() => routeStore.fromStopName)

const currentDirection = ref<'0' | '1'>(props.direction === '1' ? '1' : '0')
const isOutgoing = computed(() => currentDirection.value === '0')

const currentTripId = computed(() =>
  `${props.routeId}${currentDirection.value === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`
)

const timetable = computed(() => shapeInfo.value?.timetable)

const routeDisplayName = computed(() => {
  const t = timetable.value
  if (!t) return shapeInfo.value?.route_short_name || ''
  return isOutgoing.value ? t.route_long_name : `${t.out_stop_name} - ${t.in_stop_name}`
})

function formatGtfsColor(c?: string) {
  if (!c) return '#3b82f6'
  return c.startsWith('#') ? c : `#${c}`
}

// ─── Per-direction state (preloaded for both `_0` and `_1`) ─────────────────
type DirectionShape = {
  shape: Shape[]
  shapeIndex: ShapeIndex
  /** stop_id → index into shape; missing stops have no live ETA. */
  stopShapeIdxByStopId: Map<number, number>
}

const direction0Shape = ref<DirectionShape | null>(null)
const direction1Shape = ref<DirectionShape | null>(null)
const direction0Vehicles = ref<IndexedVehicle[]>([])
const direction1Vehicles = ref<IndexedVehicle[]>([])

const currentDirectionShape = computed(() =>
  currentDirection.value === '0' ? direction0Shape.value : direction1Shape.value,
)
const currentDirectionVehicles = computed(() =>
  currentDirection.value === '0' ? direction0Vehicles.value : direction1Vehicles.value,
)

type IndexedStop = StopTime & {timeOffsetFromStart: number}

// ─── Stops for current direction ────────────────────────────────────────────
// Derived synchronously from shapeInfo so the timeline renders immediately,
// before the shape polyline finishes loading. Live ETAs join later via the
// per-direction `stopShapeIdxByStopId` map.
const stopsForDirection = computed((): IndexedStop[] => {
  if (!shapeInfo.value) return []
  const rawStops: StopTime[] = (shapeInfo.value as any).stop_time ?? (shapeInfo.value as any).stop_times ?? []
  const filtered = rawStops
    .filter((st) => st.trip_id === currentTripId.value)
    .sort((a, b) => a.stop_sequence - b.stop_sequence)

  let cumulative = 0
  return filtered.map((st) => {
    const offset = cumulative
    cumulative += Math.ceil(st.offset_arrival_time / 60)
    return {...st, timeOffsetFromStart: offset}
  })
})

const nearestStopIdx = computed(() => {
  const loc = userLocation.value
  if (!loc || !stopsForDirection.value.length) return -1
  let best = -1
  let bestDist = Infinity
  stopsForDirection.value.forEach((stop, idx) => {
    if (!stop.stop_lat || !stop.stop_lon) return
    const d = haversineMeters(loc.latitude, loc.longitude, stop.stop_lat, stop.stop_lon)
    if (d < bestDist) { bestDist = d; best = idx }
  })
  return best
})

const hasOutgoing = computed(() => {
  const raw: StopTime[] = (shapeInfo.value as any)?.stop_time ?? (shapeInfo.value as any)?.stop_times ?? []
  return raw.some((st) => st.trip_id === `${props.routeId}${OUTGOING_SUFFIX}`)
})

const hasIncoming = computed(() => {
  const raw: StopTime[] = (shapeInfo.value as any)?.stop_time ?? (shapeInfo.value as any)?.stop_times ?? []
  return raw.some((st) => st.trip_id === `${props.routeId}${INCOMING_SUFFIX}`)
})

// ─── Time helpers ────────────────────────────────────────────────────────────
const currentMinutes = computed(() => getMinutesFromDate(userTime.value || new Date()))

/** Format minutes-from-now: "now" / "Xm" / "HH:MM" */
function formatMinutes(minutes: number): string {
  if (minutes === 0) return t('now')
  if (minutes < 60) return `${minutes}m`
  const base = userTime.value || new Date()
  const future = new Date(base.getTime() + minutes * 60_000)
  return `${future.getHours().toString().padStart(2, '0')}:${future.getMinutes().toString().padStart(2, '0')}`
}

function minutesLeft(absMinutes: number): number {
  return ((absMinutes - currentMinutes.value) + 1440) % 1440
}

// ─── Next 3 base departure times (abs minutes) from first stop ───────────────
const baseDepartureTimes = computed((): number[] => {
  const t = timetable.value
  if (!t) return []
  const today = (userTime.value || new Date()).getDay()
  const todaySchedule = today === 0 ? t.sunday : today === 6 ? t.saturday : t.weekdays
  if (!todaySchedule?.entries?.length) return []

  return todaySchedule.entries
    .map((entry) => timeStringToMinutes(isOutgoing.value ? entry.departure_in : entry.departure_out))
    .filter((v): v is number => v !== null)
    .map((absMin) => ({absMin, delta: ((absMin - currentMinutes.value) + 1440) % 1440}))
    .filter((v) => v.delta < 480)
    .sort((a, b) => a.delta - b.delta)
    .slice(0, 3)
    .map((v) => v.absMin)
})

// ─── Vehicle-aware arrival times per stop ────────────────────────────────────
interface StopTimeDisplay { label: string; isLive: boolean }

function getStopTimesDisplay(stop: IndexedStop): StopTimeDisplay[] {
  const timetableTimes = baseDepartureTimes.value.map(base => base + stop.timeOffsetFromStart)
  if (!timetableTimes.length) return []

  // Live ETA: O(vehicles) — uses the per-direction precomputed shape index.
  let liveMinutes: number | null = null
  const dirShape = currentDirectionShape.value
  const vehicles = currentDirectionVehicles.value
  if (dirShape && vehicles.length) {
    const stopIdx = dirShape.stopShapeIdxByStopId.get(stop.stop_id)
    if (stopIdx !== undefined && stopIdx >= 0) {
      const eta = etaForStop(stopIdx, vehicles, dirShape.shapeIndex)
      if (eta && eta.etaMinutes > 0) liveMinutes = eta.etaMinutes
    }
  }

  return timetableTimes.map((absMin, i) => {
    if (i === 0 && liveMinutes !== null) {
      return {label: formatMinutes(liveMinutes), isLive: true}
    }
    return {label: formatMinutes(minutesLeft(absMin)), isLive: false}
  })
}

// Header uses timetable-only times (no live override, shows departure schedule)
function getHeaderTimes(): string[] {
  return baseDepartureTimes.value.map(base => formatMinutes(minutesLeft(base)))
}

// ─── Full timetable ──────────────────────────────────────────────────────────
type TimetableTab = 'weekdays' | 'saturday' | 'sunday'

const todayTab = computed((): TimetableTab => {
  const day = (userTime.value || new Date()).getDay()
  if (day === 0) return 'sunday'
  if (day === 6) return 'saturday'
  return 'weekdays'
})

const selectedTimetableTab = ref<TimetableTab>(todayTab.value)

const availableTabs = computed(() => {
  const tt = timetable.value
  if (!tt) return []
  const tabs: Array<{key: TimetableTab; label: string}> = []
  if (tt.weekdays?.entries?.length) tabs.push({key: 'weekdays', label: t('weekdays')})
  if (tt.saturday?.entries?.length) tabs.push({key: 'saturday', label: t('saturday')})
  if (tt.sunday?.entries?.length)   tabs.push({key: 'sunday',   label: t('sunday')})
  return tabs
})

type TimetableChip = {time: string; isPast: boolean; isSuspended: boolean}

const timetableEntries = computed((): TimetableChip[] => {
  const tt = timetable.value
  if (!tt) return []
  const sched =
    selectedTimetableTab.value === 'sunday'   ? tt.sunday :
    selectedTimetableTab.value === 'saturday' ? tt.saturday :
    tt.weekdays
  if (!sched?.entries?.length) return []

  // Only grey out past times when viewing today's actual schedule
  const isToday = selectedTimetableTab.value === todayTab.value
  const now = currentMinutes.value

  return sched.entries
    .map((entry): TimetableChip | null => {
      const raw = isOutgoing.value ? entry.departure_in : entry.departure_out
      const timeStr = raw?.trim()
      if (!timeStr) return null
      const absMin = timeStringToMinutes(timeStr)
      // Non-time strings (e.g. "Suspendat") are kept and rendered with a
      // distinct chip — dropping them silently leaves the day looking empty.
      if (absMin === null) return {time: timeStr, isPast: false, isSuspended: true}
      return {time: timeStr, isPast: isToday && absMin < now, isSuspended: false}
    })
    .filter((e): e is TimetableChip => e !== null)
})

const allEntriesSuspended = computed(
  () => timetableEntries.value.length > 0 && timetableEntries.value.every((e) => e.isSuspended),
)

// ─── Map integration ─────────────────────────────────────────────────────────
function buildDisplayShape() {
  return {
    trip_id: currentTripId.value,
    route_short_name: shapeInfo.value!.route_short_name,
    route_long_name: routeDisplayName.value,
    route_color: shapeInfo.value!.route_color,
    route_type: shapeInfo.value!.route_type,
  }
}

/** Updates shape + vehicles on the map for the current direction.
 *  Synchronous when the direction's shape is already loaded — avoids the
 *  network/clear/redraw flicker that an async `setShapesToDisplay` causes. */
function updateMap() {
  if (!shapeInfo.value) return
  const dirShape = currentDirectionShape.value
  const meta = buildDisplayShape()
  if (dirShape) {
    mapStore.setLoadedShapes([[meta, dirShape.shape]])
  } else {
    // First paint, before loadAllDirections finishes — fall back to network.
    void mapStore.setShapesToDisplay([meta])
  }
  mapStore.setVehiclesToDisplay(currentDirectionVehicles.value)
  zoomOut.value = true
}

// ─── Highlighted stops on map: selected green, nearest purple, fav red, rest gray ───
// watchEffect auto-tracks every reactive value it reads, including
// favoriteStopIds array contents (via .includes), fromStopId, nearestStopIdx,
// and stopsForDirection — so any of those changing re-runs this automatically.
const {favoriteStopIds} = storeToRefs(favoritesStore)

watchEffect(() => {
  const highlights: Array<{stopId: string; color: 'green' | 'purple' | 'red' | 'gray'}> = []
  stopsForDirection.value.forEach((stop, idx) => {
    const stopId = String(stop.stop_id)
    if (stopId === fromStopId.value) {
      highlights.push({stopId, color: 'green'})
    } else if (idx === nearestStopIdx.value) {
      highlights.push({stopId, color: 'purple'})
    } else if (favoriteStopIds.value.includes(stop.stop_id)) {
      highlights.push({stopId, color: 'red'})
    } else {
      highlights.push({stopId, color: 'gray'})
    }
  })
  mapStore.setHighlightedStops(highlights)
})

// ─── Direction loading + vehicle refresh ─────────────────────────────────────
let vehicleInterval: ReturnType<typeof setInterval> | null = null

/**
 * Fetches one direction's shape, builds its index, and computes the
 * stop_id → shapeIdx lookup table once. Returns null if there's no shape
 * (route doesn't run that direction).
 */
async function loadDirectionShape(dir: '0' | '1'): Promise<DirectionShape | null> {
  if (!shapeInfo.value) return null
  const tripId = `${props.routeId}${dir === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`
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
    const rawStops: StopTime[] = (shapeInfo.value as any).stop_time ?? (shapeInfo.value as any).stop_times ?? []
    const tripStops = rawStops.filter((st) => st.trip_id === tripId)

    const stopShapeIdxByStopId = new Map<number, number>()
    for (const st of tripStops) {
      if (!st.stop_lat || !st.stop_lon) continue
      stopShapeIdxByStopId.set(st.stop_id, findClosestShapeIdx(st.stop_lat, st.stop_lon, shape))
    }

    return {shape, shapeIndex, stopShapeIdxByStopId}
  } catch (e) {
    console.warn(`Failed to load direction ${dir} shape:`, e)
    return null
  }
}

async function loadAllDirections() {
  // Both directions in parallel — switching after this is purely visual.
  const [d0, d1] = await Promise.all([loadDirectionShape('0'), loadDirectionShape('1')])
  direction0Shape.value = d0
  direction1Shape.value = d1
}

async function refreshVehicles() {
  if (!shapeInfo.value) return
  const routeShortName = shapeInfo.value.route_short_name
  const outgoingTripId = `${props.routeId}${OUTGOING_SUFFIX}`
  const incomingTripId = `${props.routeId}${INCOMING_SUFFIX}`

  const tripIds: string[] = []
  if (direction0Shape.value) tripIds.push(outgoingTripId)
  if (direction1Shape.value) tripIds.push(incomingTripId)

  let vehiclesByTrip = new Map<string, import('@/types/tranzy.ts').Vehicle[]>()
  try {
    vehiclesByTrip = await fetchVehiclesForTrips(tripIds)
  } catch (e) {
    console.warn('Failed to bulk-fetch vehicles:', e)
  }

  const tasks: Promise<void>[] = []

  if (direction0Shape.value) {
    tasks.push((async () => {
      try {
        direction0Vehicles.value = await getIndexedVehicles(
          outgoingTripId,
          routeShortName,
          direction0Shape.value!.shapeIndex,
          userTime.value,
          vehiclesByTrip.get(outgoingTripId) ?? [],
        )
      } catch (e) {
        console.warn('Failed to fetch outgoing vehicles:', e)
      }
    })())
  }
  if (direction1Shape.value) {
    tasks.push((async () => {
      try {
        direction1Vehicles.value = await getIndexedVehicles(
          incomingTripId,
          routeShortName,
          direction1Shape.value!.shapeIndex,
          userTime.value,
          vehiclesByTrip.get(incomingTripId) ?? [],
        )
      } catch (e) {
        console.warn('Failed to fetch incoming vehicles:', e)
      }
    })())
  }

  await Promise.all(tasks)
  mapStore.setVehiclesToDisplay(currentDirectionVehicles.value)
}

// Direction switch is now ~free: data for both directions is already in
// memory, so we only need to repaint the map and re-show the matching
// vehicles. No fetches, no recomputation.
watch(currentDirection, () => {
  updateMap()
})

// ─── Scroll to from-stop ─────────────────────────────────────────────────────
const fromStopEl = ref<HTMLElement | null>(null)

async function scrollToFromStop() {
  await nextTick()
  fromStopEl.value?.scrollIntoView({behavior: 'smooth', block: 'center'})
}

const isInitialLoading = ref(false)

const goBack = () => {
  if (window.history.state && window.history.state.back) {
    router.back()
  } else {
    router.push({name: 'home'})
  }
}

async function loadShapeInfoFromApi(): Promise<boolean> {
  const {routes, fetchRoutes} = useRoutesApi()
  const {fetchShapeInfo} = useRouteShapeInfoApi()
  await fetchRoutes()
  const route = routes.value.find((r) => r.route_id === Number(props.routeId))
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
  if (!shapeInfo.value) {
    isInitialLoading.value = true
    const ok = await loadShapeInfoFromApi()
    isInitialLoading.value = false
    if (!ok) return
  }
  // First paint: shapes not loaded yet — `updateMap` will fall back to a
  // network fetch via `setShapesToDisplay`. The in-flight dedupe in
  // `apiRequest` ensures this fetch is shared with `loadAllDirections`'s
  // request for the same direction (no duplicate hit).
  updateMap()
  scrollToFromStop()
  // Load both directions' shapes in parallel; once they land, re-paint the
  // map from the cached data (synchronous, no flicker) and start polling
  // vehicles for both directions so direction switches are instant.
  void loadAllDirections().then(() => {
    updateMap()
    void refreshVehicles()
  })
  vehicleInterval = setInterval(() => void refreshVehicles(), 10_000)
})

onUnmounted(() => {
  if (vehicleInterval !== null) clearInterval(vehicleInterval)
  mapStore.setHighlightedStops([])
  mapStore.setShapesToDisplay([])
  mapStore.setVehiclesToDisplay([])
  centerOnUser.value = true
})
</script>

<template>
  <div v-if="isInitialLoading" class="route-view-container flex items-center justify-center">
    <svg class="w-6 h-6 text-slate-400 animate-spin" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"/>
    </svg>
  </div>

  <div v-else-if="!shapeInfo" class="route-view-container flex items-center justify-center">
    <p class="text-slate-500 dark:text-slate-400 text-sm">{{ t('noRouteData') }} <button @click="goBack" class="underline">{{ t('goBack') }}</button></p>
  </div>

  <div v-else class="route-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100">

    <!-- ─── Back button ─── -->
    <div class="flex items-center mb-4!">
      <button
        @click="goBack"
        class="flex items-center gap-1.5 px-2 py-1.5 rounded-xl text-sm font-semibold text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200 transition-colors duration-150"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7"/>
        </svg>
        {{ t('back') }}
      </button>
    </div>

    <!-- ─── Route identity header ─── -->
    <header class="flex items-start gap-4 pb-5">
      <div
        class="shrink-0 min-w-[3.5rem] h-14 px-3 rounded-2xl flex items-center justify-center mt-0.5"
        :style="{ backgroundColor: formatGtfsColor(shapeInfo.route_color), boxShadow: `0 8px 24px -4px ${formatGtfsColor(shapeInfo.route_color)}66` }"
      >
        <span class="text-2xl font-black text-white leading-none">{{ shapeInfo.route_short_name }}</span>
      </div>
      <div class="flex-1 min-w-0">
        <div class="text-[10px] font-bold text-slate-400 dark:text-slate-500 uppercase tracking-[0.18em] mb-0.5">{{ t('route') }}</div>
        <h1 class="text-2xl font-black tracking-tight text-slate-900 dark:text-white leading-tight">
          {{ timetable?.route_long_name || shapeInfo.route_short_name }}
        </h1>
        <p v-if="fromStopName" class="flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400 font-medium mt-1.5">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0"></span>
          {{ t('from', { name: fromStopName }) }}
        </p>
      </div>
      <button
        type="button"
        class="fav-btn mt-1 shrink-0"
        :class="{ 'is-fav': isFavorite }"
        :title="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-label="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-pressed="isFavorite"
        @click="favoritesStore.toggleRouteFavorite(routeIdNum)"
      >
        <svg v-if="isFavorite" class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
        </svg>
        <svg v-else class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/>
        </svg>
      </button>
    </header>

    <!-- ─── Direction toggle ─── -->
    <div class="direction-toggle-wrap">
      <button
        :disabled="!hasOutgoing"
        @click="currentDirection = '0'"
        :class="['dir-btn', currentDirection === '0' ? 'dir-btn-active' : 'dir-btn-inactive']"
      >
        <svg class="w-3 h-3 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
        <span class="truncate">{{ timetable?.out_stop_name }}</span>
      </button>
      <button
        :disabled="!hasIncoming"
        @click="currentDirection = '1'"
        :class="['dir-btn', currentDirection === '1' ? 'dir-btn-active' : 'dir-btn-inactive']"
      >
        <svg class="w-3 h-3 shrink-0 rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
        <span class="truncate">{{ timetable?.in_stop_name }}</span>
      </button>
    </div>

    <!-- ─── No trips message ─── -->
    <div v-if="!stopsForDirection.length" class="mt-8 text-center text-slate-400 dark:text-slate-500 text-sm">
      {{ t('noSchedule') }}
    </div>

    <div v-else>

      <!-- ─── Stops section header ─── -->
      <div class="stops-header">
        <span class="section-label-text">{{ t('stops') }}</span>
        <div class="flex-1 h-px bg-slate-100 dark:bg-slate-800 mx-2"></div>
        <div class="times-cols">
          <span v-for="(t, i) in getHeaderTimes()" :key="i" class="time-cell text-slate-400 dark:text-slate-500">{{ t }}</span>
        </div>
      </div>

      <!-- ─── Transit timeline ─── -->
      <div class="relative">
        <!-- Vertical track line -->
        <div class="absolute left-[10px] top-3 bottom-3 w-0.5 bg-slate-200 dark:bg-slate-700"></div>

        <div
          v-for="(stop, idx) in stopsForDirection"
          :key="stop.stop_id + '-' + idx"
          :ref="(el) => { if (String(stop.stop_id) === fromStopId) fromStopEl = el as HTMLElement }"
          :class="['stop-row',
            String(stop.stop_id) === fromStopId ? 'stop-row-selected' :
            idx === nearestStopIdx ? 'stop-row-nearest' :
            favoritesStore.isStopFavorite(stop.stop_id) ? 'stop-row-fav' : ''
          ]"
        >
          <!-- Track dot -->
          <div class="relative z-10 w-5 shrink-0 flex items-center justify-center">
            <div v-if="String(stop.stop_id) === fromStopId"
                 class="w-3 h-3 rounded-full bg-emerald-500 border-2 border-white dark:border-slate-900 shadow-sm"></div>
            <div v-else-if="idx === nearestStopIdx"
                 class="w-3 h-3 rounded-full bg-purple-500 border-2 border-white dark:border-slate-900 shadow-sm"></div>
            <div v-else-if="idx === 0 || idx === stopsForDirection.length - 1"
                 class="w-3 h-3 rounded-full bg-slate-400 dark:bg-slate-500 border-2 border-white dark:border-slate-900"></div>
            <div v-else
                 class="w-2 h-2 rounded-full bg-white dark:bg-slate-900 border-2 border-slate-300 dark:border-slate-600"></div>
          </div>

          <!-- Stop label + status icon -->
          <div class="flex-1 min-w-0 flex items-center gap-1.5">
            <span :class="[
              'text-sm leading-tight truncate',
              String(stop.stop_id) === fromStopId
                ? 'font-bold text-emerald-500 dark:text-emerald-400'
                : idx === nearestStopIdx
                  ? 'font-semibold text-purple-500 dark:text-purple-400'
                  : idx === 0 || idx === stopsForDirection.length - 1
                    ? 'font-semibold text-slate-700 dark:text-slate-200'
                    : 'font-medium text-slate-500 dark:text-slate-400'
            ]">
              {{
                idx === 0 ? (isOutgoing ? timetable?.in_stop_name : timetable?.out_stop_name) :
                idx === stopsForDirection.length - 1 ? (isOutgoing ? timetable?.out_stop_name : timetable?.in_stop_name) :
                stop.stop_headsign || `Stop ${idx + 1}`
              }}
            </span>
            <!-- Selected: location pin -->
            <svg v-if="String(stop.stop_id) === fromStopId"
                 class="w-3.5 h-3.5 text-emerald-500 shrink-0" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
            </svg>
            <!-- Nearest: person -->
            <svg v-else-if="idx === nearestStopIdx"
                 class="w-3.5 h-3.5 text-purple-500 shrink-0" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
            </svg>
            <!-- Favorite stop: heart -->
            <svg v-if="favoritesStore.isStopFavorite(stop.stop_id)"
                 class="w-3 h-3 text-rose-400 shrink-0" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
            </svg>
          </div>

          <!-- Arrival times -->
          <div class="times-cols shrink-0">
            <span
              v-for="(t, i) in getStopTimesDisplay(stop)"
              :key="i"
              :class="[
                'time-cell',
                t.isLive
                  ? 'time-cell-live'
                  : String(stop.stop_id) === fromStopId
                    ? 'text-emerald-600 dark:text-emerald-400'
                    : idx === nearestStopIdx
                      ? 'text-purple-600 dark:text-purple-400'
                      : 'text-slate-500 dark:text-slate-400'
              ]"
            >{{ t.label }}</span>
          </div>
        </div>
      </div>

      <!-- ─── Full Timetable ─── -->
      <div v-if="availableTabs.length" class="mt-8">
        <div class="flex items-center gap-2 my-3!">
          <span class="section-label-text">{{ t('timetable') }}</span>
          <div class="flex-1 h-px bg-slate-100 dark:bg-slate-800"></div>
        </div>

        <!-- Day selector tabs -->
        <div class="flex gap-1.5 mb-4!">
          <button
            v-for="tab in availableTabs"
            :key="tab.key"
            @click="selectedTimetableTab = tab.key"
            :class="['tt-tab', selectedTimetableTab === tab.key ? 'tt-tab-active' : 'tt-tab-inactive']"
          >
            {{ tab.label }}
            <span v-if="tab.key === todayTab" class="tt-today-dot"></span>
          </button>
        </div>

        <div v-if="allEntriesSuspended" class="suspended-banner">
          {{ t('serviceSuspended') }}
        </div>

        <div class="timetable-grid">
          <span
            v-for="(entry, i) in timetableEntries"
            :key="i"
            :class="[
              'timetable-chip',
              entry.isSuspended ? 'timetable-chip-suspended'
                : entry.isPast    ? 'timetable-chip-past'
                                  : 'timetable-chip-future',
            ]"
          >{{ entry.time }}</span>
        </div>
      </div>

    </div>
  </div>
</template>

<style scoped>
.route-view-container {
  padding: 1.25rem 1.5rem 2rem;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

/* ─── Section label (matches StopView's .section-label) ─── */
.section-label-text {
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.15em;
  color: #64748b;
  white-space: nowrap;
}
@media (prefers-color-scheme: dark) {
  .section-label-text { color: #94a3b8; }
}

/* ─── Direction toggle ─── */
.direction-toggle-wrap {
  display: flex;
  gap: 0.5rem;
  margin: 1rem 0 1.5rem;
}

.dir-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  padding: 0.5rem 0.75rem;
  border-radius: 0.75rem;
  font-size: 0.75rem;
  font-weight: 700;
  border: 1.5px solid transparent;
  transition: all 0.15s;
  overflow: hidden;
  cursor: pointer;
}
.dir-btn:disabled { opacity: 0.35; cursor: not-allowed; }

.dir-btn-active   { background: #0f172a; color: white; border-color: #0f172a; }
.dir-btn-inactive { background: transparent; color: #64748b; border-color: #e2e8f0; }
.dir-btn-inactive:hover:not(:disabled) { background: #f8fafc; color: #334155; border-color: #cbd5e1; }

@media (prefers-color-scheme: dark) {
  .dir-btn-active   { background: #f1f5f9; color: #0f172a; border-color: #f1f5f9; }
  .dir-btn-inactive { color: #94a3b8; border-color: #334155; }
  .dir-btn-inactive:hover:not(:disabled) { background: rgb(30 41 59 / 0.6); color: #e2e8f0; border-color: #475569; }
}

/* ─── Stops section header ─── */
.stops-header {
  display: flex;
  align-items: center;
  margin-bottom: 0.875rem;
}

/* ─── Time columns ─── */
.times-cols { display: flex; gap: 0; }

.time-cell {
  font-size: 0.7rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  width: 2.75rem;
  text-align: right;
  letter-spacing: -0.01em;
}

.time-cell-live { color: #10b981; }

@media (prefers-color-scheme: dark) {
  .time-cell-live { color: #34d399; }
}

/* ─── Stop rows ─── */
.stop-row {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.55rem 0.5rem;
  border-radius: 0.75rem;
  margin: 0 -0.5rem;
  transition: background 0.15s;
}

.stop-row-selected {
  background: #ecfdf5;
  padding: 0.625rem 0.5rem;
}

.stop-row-nearest {
  background: #faf5ff;
  padding: 0.625rem 0.5rem;
}

.stop-row-fav {
  background: #fff1f2;
  padding: 0.625rem 0.5rem;
}

@media (prefers-color-scheme: dark) {
  .stop-row-selected { background: rgb(16 185 129 / 0.08); }
  .stop-row-nearest  { background: rgb(168 85 247 / 0.08); }
  .stop-row-fav      { background: rgb(244 63 94 / 0.06); }
}

/* ─── Timetable day tabs ─── */
.tt-tab {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.35rem 0.75rem;
  border-radius: 0.625rem;
  font-size: 0.72rem;
  font-weight: 700;
  border: 1.5px solid transparent;
  cursor: pointer;
  transition: all 0.15s;
}

.tt-tab-active   { background: #0f172a; color: white; border-color: #0f172a; }
.tt-tab-inactive { background: transparent; color: #64748b; border-color: #e2e8f0; }
.tt-tab-inactive:hover { background: #f8fafc; color: #334155; border-color: #cbd5e1; }

.tt-today-dot {
  display: inline-block;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #10b981;
  flex-shrink: 0;
}
.tt-tab-active .tt-today-dot { background: #6ee7b7; }

@media (prefers-color-scheme: dark) {
  .tt-tab-active   { background: #f1f5f9; color: #0f172a; border-color: #f1f5f9; }
  .tt-tab-inactive { color: #94a3b8; border-color: #334155; }
  .tt-tab-inactive:hover { background: rgb(30 41 59 / 0.6); color: #e2e8f0; border-color: #475569; }
  .tt-tab-active .tt-today-dot { background: #065f46; }
}

/* ─── Timetable grid ─── */
.timetable-grid { display: flex; flex-wrap: wrap; gap: 0.375rem; }

.timetable-chip {
  font-size: 0.7rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  padding: 0.3rem 0.55rem;
  border-radius: 0.5rem;
  letter-spacing: 0.01em;
}
.timetable-chip-future    { background: #f1f5f9; color: #334155; }
.timetable-chip-past      { background: transparent; color: #94a3b8; }
.timetable-chip-suspended {
  background: #fef2f2;
  color: #b91c1c;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

@media (prefers-color-scheme: dark) {
  .timetable-chip-future    { background: #1e293b; color: #e2e8f0; }
  .timetable-chip-past      { background: transparent; color: #475569; }
  .timetable-chip-suspended { background: rgb(244 63 94 / 0.12); color: #fda4af; }
}

/* ─── Suspended-day banner ─── */
.suspended-banner {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.55rem 0.75rem;
  margin-bottom: 0.625rem;
  border-radius: 0.625rem;
  background: #fef2f2;
  color: #b91c1c;
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}
@media (prefers-color-scheme: dark) {
  .suspended-banner { background: rgb(244 63 94 / 0.12); color: #fda4af; }
}

/* ─── Favorite toggle button ─── */
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

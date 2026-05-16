<script setup lang="ts">
import {computed, nextTick, onMounted, onUnmounted, ref, watch, watchEffect} from 'vue'
import {useHead} from '@unhead/vue'
import HeaderNavigation from "@/components/HeaderNavigation.vue"
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import {useRouteStore} from '@/stores/route.ts'
import {useUserStore} from '@/stores/user.ts'
import {useMapStore} from '@/stores/map.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type Shape, type StopTime} from '@/types/tranzy.ts'
import {
  formatMinutesFromNow,
  getMinutesFromDate,
  getTimetableDayKey,
  getTimetableForDay,
  timeStringToMinutes
} from '@/utils/time.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {getShapeStopTimes} from '@/utils/trips.ts'
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
import LoadingIndicator from '@/components/LoadingIndicator.vue'
import IconHeartFilled from '@/components/icons/IconHeartFilled.vue'
import IconHeartOutline from '@/components/icons/IconHeartOutline.vue'
import {useSettingsStore} from '@/stores/settings.ts'
import ShareButton from '@/components/ShareButton.vue'
import RoutePong from '@/components/RoutePong.vue'
import {useRouter} from "vue-router";

const props = defineProps<{ routeId: string; direction: string }>()

const {t} = useI18n()
const routeStore = useRouteStore()
const userStore = useUserStore()
const mapStore = useMapStore()
const favoritesStore = useFavoritesStore()
const settings = useSettingsStore()
const router = useRouter()
const {userTime, userLocation} = storeToRefs(userStore)
const {zoomOut} = storeToRefs(mapStore)
const {favoriteStopIds} = storeToRefs(favoritesStore)

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
  const tt = timetable.value
  if (!tt) return shapeInfo.value?.route_short_name || ''
  return isOutgoing.value ? tt.route_long_name : `${tt.out_stop_name} - ${tt.in_stop_name}`
})

useHead(() => {
  const shortName = shapeInfo.value?.route_short_name ?? ''
  // Don't override server-injected meta until route data has loaded — otherwise
  // a crawler snapshot taken pre-fetch sees a stale "Conexiuni Cluj" title.
  if (!shortName) return {}
  const longName = timetable.value?.route_long_name ?? ''
  const title = t('headRouteTitle', {shortName, longName: longName ? ` — ${longName}` : ''})
  const description = t('headRouteDesc', {shortName})
  const url = `https://bus.bmarian.online/route/${props.routeId}/${props.direction}`
  return {
    title,
    meta: [
      {name: 'description', content: description},
      {property: 'og:title', content: title},
      {property: 'og:description', content: description},
      {property: 'og:url', content: url},
      {name: 'twitter:title', content: title},
      {name: 'twitter:description', content: description},
    ],
    link: [{rel: 'canonical', href: url}],
  }
})

type DirectionShape = {
  shape: Shape[]
  shapeIndex: ShapeIndex
  stopShapeIdxByStopId: Map<number, number>
}

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

type IndexedStop = StopTime & { timeOffsetFromStart: number }

const rawStops = computed((): StopTime[] => getShapeStopTimes(shapeInfo.value))

const stopsForDirection = computed((): IndexedStop[] => {
  const filtered = rawStops.value
    .filter((st) => st.trip_id === currentTripId.value)
    .sort((a, b) => a.stop_sequence - b.stop_sequence)
  let cumulativeSec = 0
  return filtered.map((st) => {
    const offset = Math.ceil(cumulativeSec / 60)
    cumulativeSec += st.offset_arrival_time
    return {...st, timeOffsetFromStart: offset}
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
    outgoing: byTrip(`${props.routeId}${OUTGOING_SUFFIX}`),
    incoming: byTrip(`${props.routeId}${INCOMING_SUFFIX}`),
  }
})

const hasOutgoing = computed(() =>
  rawStops.value.some((st) => st.trip_id === `${props.routeId}${OUTGOING_SUFFIX}`)
)
const hasIncoming = computed(() =>
  rawStops.value.some((st) => st.trip_id === `${props.routeId}${INCOMING_SUFFIX}`)
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

function minutesLeft(absMinutes: number): number {
  return ((absMinutes - currentMinutes.value) + 1440) % 1440
}

function formatAbsoluteMinutes(absMin: number): string {
  const h = Math.floor(absMin / 60) % 24
  const m = absMin % 60
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`
}

const baseDepartureTimes = computed((): number[] => {
  const tt = timetable.value
  if (!tt) return []
  const sched = getTimetableForDay(tt, userTime.value || new Date())
  if (!sched?.entries?.length) return []
  return sched.entries
    .map((e) => timeStringToMinutes(isOutgoing.value ? e.departure_in : e.departure_out))
    .filter((v): v is number => v !== null)
    .map((absMin) => ({absMin, delta: ((absMin - currentMinutes.value) + 1440) % 1440}))
    .filter((v) => v.delta < 480)
    .sort((a, b) => a.delta - b.delta)
    .slice(0, 3)
    .map((v) => v.absMin)
})

interface StopTimeDisplay {
  label: string;
  isLive: boolean
}

function getStopTimesDisplay(stop: IndexedStop): StopTimeDisplay[] {
  const times = baseDepartureTimes.value.map((base) => base + stop.timeOffsetFromStart)
  if (!times.length) return []
  let liveMinutes: number | null = null
  const dirShape = currentDirectionShape.value
  if (dirShape && currentDirectionVehicles.value.length) {
    const stopIdx = dirShape.stopShapeIdxByStopId.get(stop.stop_id)
    if (stopIdx !== undefined && stopIdx >= 0) {
      const eta = etaForStop(stopIdx, currentDirectionVehicles.value, dirShape.shapeIndex, {
        tripStops: stopsForDirection.value,
        targetStopId: stop.stop_id,
        referenceTime: userTime.value,
      })
      if (eta && eta.etaMinutes > 0) liveMinutes = eta.etaMinutes
    }
  }
  return times.map((absMin, i) => {
    if (i === 0 && liveMinutes !== null) return {label: formatMinutes(liveMinutes), isLive: true}
    return {label: formatMinutes(minutesLeft(absMin)), isLive: false}
  })
}

function getHeaderTimes(): string[] {
  return baseDepartureTimes.value.map((base) => formatMinutes(minutesLeft(base)))
}

function getStopLabel(idx: number, stop: IndexedStop): string {
  return stop.stop_headsign || `Stop ${idx + 1}`
}

type TimetableTab = 'weekdays' | 'saturday' | 'sunday'

const todayTab = computed((): TimetableTab => getTimetableDayKey(userTime.value || new Date()))

const selectedTimetableTab = ref<TimetableTab>(todayTab.value)

const timetableTabOrder: Record<TimetableTab, number> = {
  weekdays: 0,
  saturday: 1,
  sunday: 2,
}

const isPastTab = (tab: TimetableTab): boolean =>
  timetableTabOrder[tab] < timetableTabOrder[todayTab.value]

const availableTabs = computed(() => {
  const tt = timetable.value
  if (!tt) return []
  const tabs: Array<{ key: TimetableTab; label: string }> = []
  if (tt.weekdays?.entries?.length) tabs.push({key: 'weekdays', label: t('weekdays')})
  if (tt.saturday?.entries?.length) tabs.push({key: 'saturday', label: t('saturday')})
  if (tt.sunday?.entries?.length) tabs.push({key: 'sunday', label: t('sunday')})
  return tabs
})

type TimetableChip = { time: string; isPast: boolean; isSuspended: boolean }

const timetableEntries = computed((): TimetableChip[] => {
  const tt = timetable.value
  if (!tt) return []
  const sched =
    selectedTimetableTab.value === 'sunday' ? tt.sunday :
      selectedTimetableTab.value === 'saturday' ? tt.saturday :
        tt.weekdays
  if (!sched?.entries?.length) return []
  const isToday = selectedTimetableTab.value === todayTab.value
  const selectedTabIsPast = isPastTab(selectedTimetableTab.value)
  const now = currentMinutes.value
  return sched.entries
    .map((entry): TimetableChip | null => {
      const raw = (isOutgoing.value ? entry.departure_in : entry.departure_out)?.trim()
      if (!raw) return null
      const absMin = timeStringToMinutes(raw)
      if (absMin === null) return {time: raw, isPast: false, isSuspended: true}
      return {
        time: raw,
        isPast: selectedTabIsPast || (isToday && absMin < now),
        isSuspended: false,
      }
    })
    .filter((e): e is TimetableChip => e !== null)
})

const allEntriesSuspended = computed(
  () => timetableEntries.value.length > 0 && timetableEntries.value.every((e) => e.isSuspended)
)

type HourGroup = { hour: string; chips: TimetableChip[]; isNextDay: boolean }

const timetableByHour = computed((): HourGroup[] => {
  const groups = new Map<string, TimetableChip[]>()
  for (const entry of timetableEntries.value) {
    if (entry.isSuspended) continue
    const rawHour = entry.time.slice(0, 2)
    if (!groups.has(rawHour)) groups.set(rawHour, [])
    groups.get(rawHour)!.push(entry)
  }
  return Array.from(groups.entries()).map(([rawHour, chips]) => {
    const h = parseInt(rawHour, 10)
    const isNextDay = h >= 24
    const hour = isNextDay ? String(h - 24).padStart(2, '0') : rawHour
    return {hour, chips, isNextDay}
  })
})

const selectedDepartureTimeDisplay = computed(() => {
  if (!selectedDepartureTime.value) return null
  const m = timeStringToMinutes(selectedDepartureTime.value)
  return m !== null ? formatAbsoluteMinutes(m) : selectedDepartureTime.value
})

const selectedDepartureTime = ref<string | null>(null)
const tripViewRef = ref<HTMLElement | null>(null)

function selectDeparture(entry: TimetableChip) {
  if (entry.isSuspended) return
  selectedDepartureTime.value = selectedDepartureTime.value === entry.time ? null : entry.time
}

watch(selectedDepartureTime, async (val) => {
  if (!val) return
  await nextTick()
  tripViewRef.value?.scrollIntoView({behavior: 'smooth', block: 'nearest'})
})

watch(currentDirection, () => {
  selectedDepartureTime.value = null
})
watch(selectedTimetableTab, () => {
  selectedDepartureTime.value = null
})

type TripStop = IndexedStop & { arrivalTimeStr: string }

const selectedDepartureStops = computed((): TripStop[] => {
  if (!selectedDepartureTime.value) return []
  const depMin = timeStringToMinutes(selectedDepartureTime.value)
  if (depMin === null) return []
  return stopsForDirection.value.map((stop) => ({
    ...stop,
    arrivalTimeStr: formatAbsoluteMinutes(depMin + stop.timeOffsetFromStart),
  }))
})

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
  if (direction0Shape.value) ids.push(`${props.routeId}${OUTGOING_SUFFIX}`)
  if (direction1Shape.value) ids.push(`${props.routeId}${INCOMING_SUFFIX}`)
  return ids
})
const {vehiclesByTrip} = useVehicleStream(streamTripIds)

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
    const tid = `${props.routeId}${direction === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`
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

const isInitialLoading = ref(!shapeInfo.value || shapeInfo.value.route_id !== Number(props.routeId))

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

// Pong secret: spam the direction toggle 5× within 2 s to activate
const pongActive = ref(false)
const dirTapTimes: number[] = []

function onDirClick(dir: '0' | '1') {
  router.replace({name: 'route', params: {routeId: props.routeId, direction: dir}})

  currentDirection.value = dir
  const now = Date.now()
  while (dirTapTimes.length && now - dirTapTimes[0]! > 2000) dirTapTimes.shift()
  dirTapTimes.push(now)
  if (dirTapTimes.length >= 5) {
    pongActive.value = true
    dirTapTimes.length = 0
  }
}

// Secret: chomp animation for the route badge
const mouthOpen = ref(true)
let chompTimer: ReturnType<typeof setInterval> | null = null

function startChomp() {
  if (chompTimer) return
  chompTimer = setInterval(() => {
    mouthOpen.value = !mouthOpen.value
  }, 320)
}

function stopChomp() {
  if (chompTimer) {
    clearInterval(chompTimer);
    chompTimer = null
  }
  mouthOpen.value = true
}

watch(() => settings.arcadeActive, (active) => {
  if (active) startChomp()
  else stopChomp()
}, {immediate: true})

function ghostFill(stop: IndexedStop, idx: number): string {
  if (String(stop.stop_id) === fromStopId.value) return '#10b981'
  if (idx === nearestStopIdx.value) return '#a855f7'
  if (favoritesStore.isStopFavorite(stop.stop_id)) return '#f43f5e'
  if (idx === 0 || idx === stopsForDirection.value.length - 1) return settings.isDark ? '#64748b' : '#94a3b8'
  return settings.isDark ? '#334155' : '#b0b8c4'
}

onMounted(async () => {
  mapStore.directionArrowAtStart = true
  const storeRouteId = shapeInfo.value?.route_id
  if (!shapeInfo.value || storeRouteId !== Number(props.routeId)) {
    isInitialLoading.value = true
    const ok = await loadShapeInfoFromApi()
    isInitialLoading.value = false
    if (!ok) return
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
  stopChomp()
})
</script>

<template>
  <div v-if="isInitialLoading"
       class="route-view-container bg-white dark:bg-[#0f172a] animate-pulse flex flex-col gap-6">
    <div class="h-6 w-16 bg-slate-200 dark:bg-slate-800 rounded-lg"></div>
    <header class="flex items-start gap-4 pb-1">
      <div class="w-14 h-14 rounded-2xl bg-slate-200 dark:bg-slate-800 shrink-0"></div>
      <div class="flex-1 flex flex-col gap-2">
        <div class="h-3 w-14 bg-slate-200 dark:bg-slate-800 rounded"></div>
        <div class="h-6 w-56 bg-slate-200 dark:bg-slate-800 rounded-lg"></div>
      </div>
      <div class="w-9 h-9 rounded-full bg-slate-200 dark:bg-slate-800 shrink-0"></div>
    </header>
    <div class="flex gap-2">
      <div class="flex-1 h-9 rounded-xl bg-slate-200 dark:bg-slate-800"></div>
      <div class="flex-1 h-9 rounded-xl bg-slate-200 dark:bg-slate-800"></div>
    </div>
    <section class="flex flex-col gap-2.5">
      <div v-for="i in 7" :key="i" class="flex items-center gap-3 py-1.5">
        <div class="w-3 h-3 rounded-full bg-slate-200 dark:bg-slate-800 shrink-0"></div>
        <div class="h-3.5 flex-1 bg-slate-200 dark:bg-slate-800 rounded"></div>
        <div class="h-3.5 w-24 bg-slate-200 dark:bg-slate-800 rounded"></div>
      </div>
    </section>
  </div>

  <div v-else-if="!shapeInfo" class="route-view-container flex flex-col">
    <LoadingIndicator :text="t('loadingRoute')"/>
  </div>

  <div v-else
       class="route-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100">

    <div class="flex items-center mb-4!">
      <HeaderNavigation/>
    </div>

    <header class="flex items-start gap-4 pb-5">
      <div
        class="shrink-0 min-w-[3.5rem] h-14 px-3 rounded-2xl flex items-center justify-center mt-0.5"
        :style="{ backgroundColor: shapeInfo.route_color, boxShadow: `0 8px 24px -4px ${shapeInfo.route_color}66` }"
      >
        <span class="text-2xl font-black text-white leading-none">{{
            shapeInfo.route_short_name
          }}</span>
      </div>
      <div class="flex-1 min-w-0">
        <div
          class="route-header-label text-[10px] font-semibold text-slate-400 dark:text-slate-500 tracking-wide mb-0.5">
          {{ t('route') }}
        </div>
        <h1 class="text-2xl font-black tracking-tight text-slate-900 dark:text-white leading-tight">
          {{ timetable?.route_long_name || shapeInfo.route_short_name }}
        </h1>
        <p v-if="fromStopName"
           class="flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400 font-medium mt-1.5">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0"></span>
          {{ t('from', {name: fromStopName}) }}
        </p>
      </div>
      <ShareButton class="mt-1"/>
      <button
        type="button"
        class="fav-btn mt-1 shrink-0"
        :class="{ 'is-fav': isFavorite }"
        :title="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-label="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-pressed="isFavorite"
        @click="favoritesStore.toggleRouteFavorite(routeIdNum)"
      >
        <IconHeartFilled v-if="isFavorite" class="w-5 h-5"/>
        <IconHeartOutline v-else class="w-5 h-5"/>
      </button>
    </header>

    <RoutePong
      v-if="pongActive"
      :route-short-name="shapeInfo.route_short_name"
      :route-color="shapeInfo.route_color"
      @exit="pongActive = false"
    />
    <div v-else class="direction-toggle-wrap">
      <button :disabled="!hasOutgoing" @click="onDirClick('0')"
              :class="['dir-btn', currentDirection === '0' ? 'dir-btn-active' : 'dir-btn-inactive']">
        <svg class="w-3 h-3 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"
             stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
        <span class="truncate">{{ directionTerminals.outgoing.last || timetable?.out_stop_name }}</span>
      </button>
      <button :disabled="!hasIncoming" @click="onDirClick('1')"
              :class="['dir-btn', currentDirection === '1' ? 'dir-btn-active' : 'dir-btn-inactive']">
        <span class="truncate">{{ directionTerminals.incoming.last || timetable?.in_stop_name }}</span>
        <svg class="w-3 h-3 shrink-0 rotate-180" fill="none" viewBox="0 0 24 24"
             stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
      </button>
    </div>

    <div v-if="!stopsForDirection.length"
         class="mt-8 text-center text-slate-400 dark:text-slate-500 text-sm">
      {{ t('noSchedule') }}
    </div>

    <div v-else>

      <div class="stops-header">
        <span class="section-label-text">{{ t('stops') }}</span>
        <div class="flex-1 h-px bg-slate-100 dark:bg-slate-800 mx-2"></div>
        <div class="flex flex-col items-end gap-0.5">
          <span
            class="text-[9px] font-bold uppercase tracking-widest text-slate-400 dark:text-slate-500">{{
              t('nextDepartures')
            }}</span>
          <div class="times-cols">
            <span v-for="(time, i) in getHeaderTimes()" :key="i"
                  class="time-cell text-slate-400 dark:text-slate-500">{{ time }}</span>
          </div>
        </div>
      </div>

      <div class="relative">
        <div class="absolute left-[10px] top-3 bottom-3 w-0.5 bg-slate-200 dark:bg-slate-700"></div>

        <div v-if="settings.arcadeActive" class="arcade-eater" aria-hidden="true">
          <div class="arcade-chomp"
               style="width:16px;height:16px;background:#FACC15;border-radius:50%;border:1.5px solid #D97706;transform:rotate(90deg);"></div>
        </div>

        <div
          v-for="(stop, idx) in stopsForDirection"
          :key="stop.stop_id + '-' + idx"
          :class="['stop-row',
            String(stop.stop_id) === fromStopId ? 'stop-row-selected' :
            idx === nearestStopIdx ? 'stop-row-nearest' :
            favoritesStore.isStopFavorite(stop.stop_id) ? 'stop-row-fav' : ''
          ]"
        >
          <div class="relative z-10 w-5 shrink-0 flex items-center justify-center">
            <template v-if="settings.arcadeActive">
              <svg viewBox="0 0 12 16"
                   :width="(idx === 0 || idx === stopsForDirection.length - 1 || String(stop.stop_id) === fromStopId || idx === nearestStopIdx || favoritesStore.isStopFavorite(stop.stop_id)) ? 13 : 10"
                   :height="(idx === 0 || idx === stopsForDirection.length - 1 || String(stop.stop_id) === fromStopId || idx === nearestStopIdx || favoritesStore.isStopFavorite(stop.stop_id)) ? 17 : 13"
                   aria-hidden="true">
                <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z"
                      :fill="ghostFill(stop, idx)"/>
                <circle cx="4" cy="8" r="1.4" fill="white"/>
                <circle cx="8" cy="8" r="1.4" fill="white"/>
                <circle cx="4.4" cy="8.5" r="0.75" fill="rgba(0,0,0,0.65)"/>
                <circle cx="8.4" cy="8.5" r="0.75" fill="rgba(0,0,0,0.65)"/>
              </svg>
            </template>
            <template v-else-if="settings.legacyBlueActive">
              <span v-if="String(stop.stop_id) === fromStopId" class="route-stop-emoji">📍</span>
              <span v-else-if="idx === nearestStopIdx" class="route-stop-emoji">🙎‍♂️</span>
              <span v-else-if="favoritesStore.isStopFavorite(stop.stop_id)"
                    class="route-stop-emoji">❤️</span>
              <span v-else-if="idx === 0 || idx === stopsForDirection.length - 1"
                    class="route-stop-bullet route-stop-bullet-end"></span>
              <span v-else class="route-stop-bullet"></span>
            </template>
            <template v-else>
              <div v-if="String(stop.stop_id) === fromStopId"
                   class="w-3 h-3 rounded-full bg-emerald-500 border-2 border-white dark:border-slate-900 shadow-sm"></div>
              <div v-else-if="idx === nearestStopIdx"
                   class="w-3 h-3 rounded-full bg-purple-500 border-2 border-white dark:border-slate-900 shadow-sm"></div>
              <div v-else-if="favoritesStore.isStopFavorite(stop.stop_id)"
                   class="w-3 h-3 rounded-full bg-rose-500 border-2 border-white dark:border-slate-900 shadow-sm"></div>
              <div v-else-if="idx === 0 || idx === stopsForDirection.length - 1"
                   class="w-3 h-3 rounded-full bg-slate-400 dark:bg-slate-500 border-2 border-white dark:border-slate-900"></div>
              <div v-else
                   class="w-2 h-2 rounded-full bg-white dark:bg-slate-900 border-2 border-slate-300 dark:border-slate-600"></div>
            </template>
          </div>

          <div class="flex-1 min-w-0 flex items-center gap-1.5">
            <router-link :class="[
              'text-sm leading-tight truncate cursor-pointer',
              String(stop.stop_id) === fromStopId ? 'font-bold text-emerald-500 dark:text-emerald-400' :
              idx === nearestStopIdx ? 'font-semibold text-purple-500 dark:text-purple-400' :
              idx === 0 || idx === stopsForDirection.length - 1 ? 'font-semibold text-slate-700 dark:text-slate-200' :
              'font-medium text-slate-500 dark:text-slate-400'
            ]" :to="`/stop/${stop.stop_id}`">{{ getStopLabel(idx, stop) }}
            </router-link>
            <template v-if="!settings.legacyBlueActive">
              <svg v-if="String(stop.stop_id) === fromStopId"
                   class="w-3.5 h-3.5 text-emerald-500 shrink-0" viewBox="0 0 24 24"
                   fill="currentColor">
                <path
                  d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
              </svg>
              <svg v-else-if="idx === nearestStopIdx" class="w-3.5 h-3.5 text-purple-500 shrink-0"
                   viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
              </svg>
              <svg v-if="favoritesStore.isStopFavorite(stop.stop_id)"
                   class="w-3 h-3 text-rose-400 shrink-0" viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
              </svg>
            </template>
          </div>

          <div class="times-cols shrink-0">
            <span
              v-for="(stopTime, i) in getStopTimesDisplay(stop)"
              :key="i"
              :class="[
                'time-cell',
                stopTime.isLive ? 'time-cell-live' :
                'text-slate-500 dark:text-slate-400'
              ]"
            >{{ stopTime.label }}</span>
          </div>
        </div>
      </div>

      <div v-if="availableTabs.length" class="mt-8">
        <div class="flex items-center gap-2 my-3!">
          <span class="section-label-text">{{ t('timetable') }}</span>
          <div class="flex-1 h-px bg-slate-100 dark:bg-slate-800"></div>
          <span class="text-[10px] text-slate-400 dark:text-slate-500">{{
              t('timetableClickHint')
            }}</span>
        </div>

        <div class="flex gap-1.5 mb-4!">
          <button
            v-for="tab in availableTabs"
            :key="tab.key"
            @click="selectedTimetableTab = tab.key"
            :class="[
              'tt-tab',
              selectedTimetableTab === tab.key ? 'tt-tab-active' : 'tt-tab-inactive',
              isPastTab(tab.key) ? 'tt-tab-past' : '',
            ]"
          >
            {{ tab.label }}
            <span v-if="tab.key === todayTab" class="tt-today-dot"></span>
          </button>
        </div>

        <div v-if="allEntriesSuspended" class="suspended-banner">{{ t('serviceSuspended') }}</div>

        <div class="tt-table">
          <div v-for="group in timetableByHour" :key="group.hour" class="tt-row">
            <span class="tt-hour" :class="group.isNextDay ? 'tt-hour-next-day' : ''">{{
                group.hour
              }}</span>
            <div class="tt-mins">
              <span
                v-for="chip in group.chips"
                :key="chip.time"
                @click="selectDeparture(chip)"
                :class="[
                  'tt-min',
                  selectedDepartureTime === chip.time ? 'tt-min-selected' :
                  chip.isPast ? 'tt-min-past' : 'tt-min-future'
                ]"
              >{{ chip.time.slice(3) }}</span>
            </div>
          </div>
        </div>

        <div v-if="selectedDepartureTime && selectedDepartureStops.length" ref="tripViewRef"
             class="trip-view">
          <div class="flex items-center gap-2 mb-3">
            <span class="section-label-text">{{
                t('tripAt', {time: selectedDepartureTimeDisplay})
              }}</span>
            <div class="flex-1"></div>
            <button
              @click="selectedDepartureTime = null"
              class="flex items-center justify-center w-5 h-5 rounded-full bg-slate-200 dark:bg-slate-700 text-slate-500 dark:text-slate-400 hover:bg-slate-300 dark:hover:bg-slate-600 transition-colors text-xs font-bold"
              :aria-label="t('closeTripView')"
            >×
            </button>
          </div>

          <div class="relative">
            <div
              class="absolute left-[10px] top-3 bottom-3 w-0.5 bg-slate-200 dark:bg-slate-700"></div>
            <div
              v-for="(stop, idx) in selectedDepartureStops"
              :key="stop.stop_id + '-trip'"
              :class="['trip-stop-row',
                String(stop.stop_id) === fromStopId ? 'stop-row-selected' :
                idx === nearestStopIdx ? 'stop-row-nearest' :
                favoritesStore.isStopFavorite(stop.stop_id) ? 'stop-row-fav' : ''
              ]"
            >
              <div class="relative z-10 w-5 shrink-0 flex items-center justify-center">
                <template v-if="settings.arcadeActive">
                  <svg viewBox="0 0 12 16"
                       :width="(idx === 0 || idx === selectedDepartureStops.length - 1 || String(stop.stop_id) === fromStopId || idx === nearestStopIdx || favoritesStore.isStopFavorite(stop.stop_id)) ? 13 : 10"
                       :height="(idx === 0 || idx === selectedDepartureStops.length - 1 || String(stop.stop_id) === fromStopId || idx === nearestStopIdx || favoritesStore.isStopFavorite(stop.stop_id)) ? 17 : 13"
                       aria-hidden="true">
                    <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z"
                          :fill="ghostFill(stop, idx)"/>
                    <circle cx="4" cy="8" r="1.4" fill="white"/>
                    <circle cx="8" cy="8" r="1.4" fill="white"/>
                    <circle cx="4.4" cy="8.5" r="0.75" fill="rgba(0,0,0,0.65)"/>
                    <circle cx="8.4" cy="8.5" r="0.75" fill="rgba(0,0,0,0.65)"/>
                  </svg>
                </template>
                <template v-else-if="settings.legacyBlueActive">
                  <span v-if="String(stop.stop_id) === fromStopId" class="route-stop-emoji">📍</span>
                  <span v-else-if="idx === nearestStopIdx" class="route-stop-emoji">🙎‍♂️</span>
                  <span v-else-if="favoritesStore.isStopFavorite(stop.stop_id)"
                        class="route-stop-emoji">❤️</span>
                  <span v-else-if="idx === 0 || idx === selectedDepartureStops.length - 1"
                        class="route-stop-bullet route-stop-bullet-end"></span>
                  <span v-else class="route-stop-bullet"></span>
                </template>
                <template v-else>
                  <div v-if="String(stop.stop_id) === fromStopId"
                       class="w-3 h-3 rounded-full bg-emerald-500 border-2 border-white dark:border-slate-900 shadow-sm"></div>
                  <div v-else-if="idx === nearestStopIdx"
                       class="w-3 h-3 rounded-full bg-purple-500 border-2 border-white dark:border-slate-900 shadow-sm"></div>
                  <div v-else-if="favoritesStore.isStopFavorite(stop.stop_id)"
                       class="w-3 h-3 rounded-full bg-rose-500 border-2 border-white dark:border-slate-900 shadow-sm"></div>
                  <div v-else-if="idx === 0 || idx === selectedDepartureStops.length - 1"
                       class="w-3 h-3 rounded-full bg-slate-400 dark:bg-slate-500 border-2 border-white dark:border-slate-900"></div>
                  <div v-else
                       class="w-2 h-2 rounded-full bg-white dark:bg-slate-900 border-2 border-slate-300 dark:border-slate-600"></div>
                </template>
              </div>

              <div class="flex-1 min-w-0 flex items-center gap-1.5">
                <router-link :class="[
                  'text-sm leading-tight truncate cursor-pointer',
                  String(stop.stop_id) === fromStopId ? 'font-bold text-emerald-500 dark:text-emerald-400' :
                  idx === nearestStopIdx ? 'font-semibold text-purple-500 dark:text-purple-400' :
                  idx === 0 || idx === selectedDepartureStops.length - 1 ? 'font-semibold text-slate-700 dark:text-slate-200' :
                  'font-medium text-slate-500 dark:text-slate-400'
                ]" :to="`/stop/${stop.stop_id}`">{{ getStopLabel(idx, stop) }}</router-link>
                <template v-if="!settings.legacyBlueActive">
                  <svg v-if="String(stop.stop_id) === fromStopId"
                       class="w-3.5 h-3.5 text-emerald-500 shrink-0" viewBox="0 0 24 24"
                       fill="currentColor">
                    <path
                      d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
                  </svg>
                  <svg v-else-if="idx === nearestStopIdx"
                       class="w-3.5 h-3.5 text-purple-500 shrink-0" viewBox="0 0 24 24"
                       fill="currentColor">
                    <path
                      d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
                  </svg>
                  <svg v-if="favoritesStore.isStopFavorite(stop.stop_id)"
                       class="w-3 h-3 text-rose-400 shrink-0" viewBox="0 0 24 24"
                       fill="currentColor">
                    <path
                      d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
                  </svg>
                </template>
              </div>

              <span :class="[
                'text-sm font-bold tabular-nums shrink-0',
                idx === 0 ? 'text-blue-600 dark:text-blue-400' : 'text-slate-600 dark:text-slate-300'
              ]">{{ stop.arrivalTimeStr }}</span>
            </div>
          </div>
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

.section-label-text {
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  white-space: nowrap;
}

.direction-toggle-wrap {
  display: flex;
  gap: 0;
  margin: 1rem 0 1.5rem;
  background: #f1f5f9;
  border-radius: 0.875rem;
  padding: 0.25rem;
}

.dir-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  padding: 0.45rem 0.75rem;
  border-radius: 0.625rem;
  font-size: 0.75rem;
  font-weight: 700;
  border: none;
  overflow: hidden;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, box-shadow 0.15s;
}

.dir-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.dir-btn-active {
  background: #ffffff;
  color: #0f172a;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.1), 0 1px 2px rgba(0, 0, 0, 0.06);
}

.dir-btn-inactive {
  background: transparent;
  color: #64748b;
}

.dir-btn-inactive:hover:not(:disabled) {
  color: #334155;
}

.stops-header {
  display: flex;
  align-items: center;
  margin-bottom: 0.875rem;
}

.times-cols {
  display: flex;
  gap: 0;
}

.time-cell {
  font-size: 0.7rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  width: 2.75rem;
  text-align: right;
  letter-spacing: -0.01em;
}

.time-cell-live {
  color: #10b981;
}

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

.tt-tab-active {
  background: #0f172a;
  color: white;
  border-color: #0f172a;
}

.tt-tab-inactive {
  background: transparent;
  color: #64748b;
  border-color: #e2e8f0;
}

.tt-tab-inactive:hover {
  background: #f8fafc;
  color: #334155;
  border-color: #cbd5e1;
}

.tt-tab-past {
  opacity: 0.55;
}

.tt-today-dot {
  display: inline-block;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #10b981;
  flex-shrink: 0;
}

.tt-tab-active .tt-today-dot {
  background: #6ee7b7;
}

.tt-table {
  width: 100%;
}

.tt-row {
  display: flex;
  align-items: center;
  border-bottom: 1px solid #f1f5f9;
}

.tt-row:last-child {
  border-bottom: none;
}

.tt-hour {
  width: 2.5rem;
  align-self: stretch;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding-right: 0.625rem;
  font-size: 0.9375rem;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: #94a3b8;
  flex-shrink: 0;
  border-right: 2px solid #e2e8f0;
  line-height: 1;
}

.tt-mins {
  display: flex;
  flex-wrap: wrap;
  gap: 0.125rem;
  padding: 0.25rem 0 0.25rem 0.375rem;
  flex: 1;
}

.tt-min {
  font-size: 0.9375rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  min-width: 2.5rem;
  height: 2.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
  user-select: none;
}

.tt-min-future {
  color: #1e293b;
}

.tt-min-future:hover {
  background: #f1f5f9;
}

.tt-min-past {
  color: #cbd5e1;
  cursor: default;
}

.tt-min-selected {
  background: #1e40af;
  color: white !important;
}

.suspended-banner {
  display: flex;
  align-items: center;
  padding: 0.55rem 0.75rem;
  margin-bottom: 0.75rem;
  border-radius: 0.625rem;
  background: #fef2f2;
  color: #b91c1c;
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.trip-view {
  margin-top: 1.25rem;
}

.trip-stop-row {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.45rem 0.5rem;
  border-radius: 0.625rem;
  margin: 0 -0.5rem;
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

.tt-hour-next-day {
  color: #7dd3fc !important;
}

</style>

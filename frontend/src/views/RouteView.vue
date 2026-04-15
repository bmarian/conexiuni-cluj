<script setup lang="ts">
import {computed, nextTick, onMounted, onUnmounted, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useRouteStore} from '@/stores/route.ts'
import {useUserStore} from '@/stores/user.ts'
import {useMapStore} from '@/stores/map.ts'
import {OUTGOING_SUFFIX, INCOMING_SUFFIX, type StopTime} from '@/types/tranzy.ts'
import {getMinutesFromDate, timeStringToMinutes} from '@/utils/time.ts'
import {haversineMeters} from '@/utils/geo.ts'

const props = defineProps<{ routeId: string; direction: string }>()

const router = useRouter()
const routeStore = useRouteStore()
const userStore = useUserStore()
const mapStore = useMapStore()
const {userTime, userLocation} = storeToRefs(userStore)
const {zoomOut, centerOnUser} = storeToRefs(mapStore)

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

// ─── Stops for current direction ────────────────────────────────────────────
const stopsForDirection = computed(() => {
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

// ─── Next 3 base departure times from first stop ─────────────────────────────
const currentMinutes = computed(() => getMinutesFromDate(userTime.value || new Date()))

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

function formatAbsMinutes(absMinutes: number): string {
  const h = Math.floor(absMinutes / 60) % 24
  const m = absMinutes % 60
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`
}

function getArrivalTimesAtStop(offsetFromStart: number): string[] {
  return baseDepartureTimes.value.map((base) => formatAbsMinutes(base + offsetFromStart))
}

// ─── Full timetable ──────────────────────────────────────────────────────────
const dayLabel = computed(() => {
  const day = (userTime.value || new Date()).getDay()
  if (day === 0) return 'Sunday'
  if (day === 6) return 'Saturday'
  return 'Weekdays'
})

const timetableEntries = computed(() => {
  const t = timetable.value
  if (!t) return []
  const day = (userTime.value || new Date()).getDay()
  const sched = day === 0 ? t.sunday : day === 6 ? t.saturday : t.weekdays
  if (!sched?.entries?.length) return []
  const now = currentMinutes.value

  return sched.entries
    .map((entry) => {
      const timeStr = isOutgoing.value ? entry.departure_in : entry.departure_out
      if (!timeStr) return null
      const absMin = timeStringToMinutes(timeStr)
      if (absMin === null) return null
      return {time: timeStr, isPast: absMin < now}
    })
    .filter((e): e is {time: string; isPast: boolean} => e !== null)
})

// ─── Map integration ─────────────────────────────────────────────────────────
function updateMap() {
  if (!shapeInfo.value) return
  mapStore.setShapesToDisplay([{
    trip_id: currentTripId.value,
    route_short_name: shapeInfo.value.route_short_name,
    route_long_name: routeDisplayName.value,
    route_color: shapeInfo.value.route_color,
    route_type: shapeInfo.value.route_type,
  }])
  zoomOut.value = true
}

watch(currentDirection, () => updateMap())

// ─── Scroll to from-stop ─────────────────────────────────────────────────────
const fromStopEl = ref<HTMLElement | null>(null)

async function scrollToFromStop() {
  await nextTick()
  fromStopEl.value?.scrollIntoView({behavior: 'smooth', block: 'center'})
}

onMounted(() => {
  if (!shapeInfo.value) {
    router.back()
    return
  }
  updateMap()
  scrollToFromStop()
})

onUnmounted(() => {
  mapStore.setShapesToDisplay([])
  mapStore.setVehiclesToDisplay([])
  centerOnUser.value = true
})
</script>

<template>
  <div v-if="!shapeInfo" class="route-view-container flex items-center justify-center">
    <p class="text-slate-500 dark:text-slate-400 text-sm">No route data. <button @click="router.back()" class="underline">Go back</button></p>
  </div>

  <div v-else class="route-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100">

    <!-- ─── Back button ─── -->
    <div class="flex items-center pt-2 pb-1 -mx-1">
      <button
        @click="router.back()"
        class="flex items-center gap-1.5 px-2 py-1.5 rounded-xl text-sm font-semibold text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200 transition-colors"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7"/>
        </svg>
        Back
      </button>
    </div>

    <!-- ─── Route identity header ─── -->
    <header class="flex items-start gap-4 pt-1 pb-5">
      <div
        class="shrink-0 min-w-[3.5rem] h-14 px-3 rounded-2xl flex items-center justify-center mt-0.5"
        :style="{ backgroundColor: formatGtfsColor(shapeInfo.route_color), boxShadow: `0 8px 24px -4px ${formatGtfsColor(shapeInfo.route_color)}66` }"
      >
        <span class="text-2xl font-black text-white leading-none">{{ shapeInfo.route_short_name }}</span>
      </div>
      <div class="flex-1 min-w-0">
        <div class="text-[10px] font-bold text-slate-400 dark:text-slate-500 uppercase tracking-[0.18em] mb-0.5">Route</div>
        <h1 class="text-2xl font-black tracking-tight text-slate-900 dark:text-white leading-tight">
          {{ timetable?.route_long_name || shapeInfo.route_short_name }}
        </h1>
        <p v-if="fromStopName" class="flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400 font-medium mt-1.5">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0"></span>
          From {{ fromStopName }}
        </p>
      </div>
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
      No schedule data for this direction.
    </div>

    <div v-else>

      <!-- ─── Stops section header ─── -->
      <div class="stops-header">
        <span class="text-xs font-bold text-slate-400 dark:text-slate-500 uppercase tracking-widest">Stops</span>
        <div class="flex-1 h-px bg-slate-100 dark:bg-slate-800 mx-2"></div>
        <div class="times-cols">
          <span v-for="(time, i) in getArrivalTimesAtStop(0)" :key="i" class="time-cell">{{ time }}</span>
        </div>
      </div>

      <!-- ─── Transit timeline ─── -->
      <div class="relative">
        <!-- Vertical track line -->
        <div class="absolute left-[9px] top-3 bottom-3 w-0.5 bg-slate-200 dark:bg-slate-700"></div>

        <div
          v-for="(stop, idx) in stopsForDirection"
          :key="stop.stop_id + '-' + idx"
          :ref="(el) => { if (String(stop.stop_id) === fromStopId) fromStopEl = el as HTMLElement }"
          :class="['stop-row', String(stop.stop_id) === fromStopId ? 'stop-row-selected' : idx === nearestStopIdx ? 'stop-row-nearest' : '']"
        >
          <!-- Track dot (left) — simple, uniform size, just colored -->
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
          <div class="flex-1 min-w-0 flex items-center gap-1.5 min-w-0">
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
          </div>

          <!-- Arrival times -->
          <div class="times-cols shrink-0">
            <span
              v-for="(time, i) in getArrivalTimesAtStop(stop.timeOffsetFromStart)"
              :key="i"
              :class="[
                'time-cell',
                String(stop.stop_id) === fromStopId
                  ? 'text-emerald-600 dark:text-emerald-400'
                  : idx === nearestStopIdx
                    ? 'text-purple-600 dark:text-purple-400'
                    : 'text-slate-500 dark:text-slate-400'
              ]"
            >{{ time }}</span>
          </div>
        </div>
      </div>

      <!-- ─── Full Timetable ─── -->
      <div v-if="timetableEntries.length" class="mt-8">
        <div class="flex items-center gap-2 mb-4">
          <span class="text-xs font-bold text-slate-400 dark:text-slate-500 uppercase tracking-widest">Timetable</span>
          <div class="flex-1 h-px bg-slate-100 dark:bg-slate-800"></div>
          <span class="text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400">
            {{ dayLabel }}
          </span>
        </div>

        <div class="timetable-grid">
          <span
            v-for="(entry, i) in timetableEntries"
            :key="i"
            :class="['timetable-chip', entry.isPast ? 'timetable-chip-past' : 'timetable-chip-future']"
          >{{ entry.time }}</span>
        </div>
      </div>

    </div>

  </div>
</template>

<style scoped>
.route-view-container {
  padding: 0.5rem 1.25rem 2rem;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

/* ─── Direction toggle ─── */
.direction-toggle-wrap {
  display: flex;
  gap: 0.5rem;
  margin: 1rem 0 1.25rem;
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

.dir-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.dir-btn-active {
  background: #0f172a;
  color: white;
  border-color: #0f172a;
}

.dir-btn-inactive {
  background: transparent;
  color: #64748b;
  border-color: #e2e8f0;
}

.dir-btn-inactive:hover:not(:disabled) {
  background: #f8fafc;
  color: #334155;
  border-color: #cbd5e1;
}

@media (prefers-color-scheme: dark) {
  .dir-btn-active {
    background: #f1f5f9;
    color: #0f172a;
    border-color: #f1f5f9;
  }
  .dir-btn-inactive {
    color: #94a3b8;
    border-color: #334155;
  }
  .dir-btn-inactive:hover:not(:disabled) {
    background: rgb(30 41 59 / 0.6);
    color: #e2e8f0;
    border-color: #475569;
  }
}

/* ─── Shared time columns ─── */
/* Header row and every stop row share the same times-cols / time-cell classes
   so the columns are guaranteed to line up. */
.stops-header {
  display: flex;
  align-items: center;
  margin-bottom: 0.75rem;
}

.times-cols {
  display: flex;
  gap: 0;
}

.time-cell {
  font-size: 0.7rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  width: 2.75rem;       /* 44px — fixed for every column */
  text-align: right;
  letter-spacing: -0.01em;
}

/* ─── Stop rows ─── */
.stop-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.25rem;
  border-radius: 0.75rem;
  margin: 0 -0.25rem;
  transition: background 0.1s;
}

.stop-row-selected {
  background: #ecfdf5;
  margin: 0.125rem -0.25rem;
  padding: 0.5rem 0.25rem;
}

.stop-row-nearest {
  background: #faf5ff;
  margin: 0.125rem -0.25rem;
  padding: 0.5rem 0.25rem;
}

@media (prefers-color-scheme: dark) {
  .stop-row-selected {
    background: rgb(16 185 129 / 0.08);
  }
  .stop-row-nearest {
    background: rgb(168 85 247 / 0.08);
  }
}

/* ─── Timetable grid ─── */
.timetable-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.timetable-chip {
  font-size: 0.7rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  padding: 0.3rem 0.55rem;
  border-radius: 0.5rem;
  letter-spacing: 0.01em;
}

.timetable-chip-future {
  background: #f1f5f9;
  color: #334155;
}

.timetable-chip-past {
  background: transparent;
  color: #94a3b8;
}

@media (prefers-color-scheme: dark) {
  .timetable-chip-future {
    background: #1e293b;
    color: #e2e8f0;
  }
  .timetable-chip-past {
    background: transparent;
    color: #475569;
  }
}
</style>

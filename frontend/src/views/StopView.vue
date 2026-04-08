<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts"
import {storeToRefs} from "pinia"
import {computed, ref, watch} from "vue"
import {useStopInfoApi} from "@/composables/useStopInfoApi.ts"
import StopIcon from "../../public/stop.svg"
import type {Timetable} from "@/types/ctp.ts";
import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type ShapeInfo, type StopTime} from "@/types/tranzy.ts";
import {getMinutesFromDate, timeStringToMinutes} from "@/utils/time.ts";
import {type DisplayShape, useMapStore} from "@/stores/map.ts";

const props = defineProps<{
  stopId: string
}>()
const userStore = useUserStore()
const mapStore = useMapStore()
const {userTime} = storeToRefs(userStore)
const {stopInfo, fetchStopData} = useStopInfoApi()
const stopName = computed(() => stopInfo.value?.stop_name)
const isLoading = ref(false)

function formatGtfsColor(colorString?: string) {
  if (!colorString) return '#3b82f6';
  return colorString.startsWith('#') ? colorString : `#${colorString}`;
}

function getRouteTypeLabel(type: number) {
  switch (type) {
    case 0:
      return '🚋';
    case 2:
      return '🚆';
    case 3:
      return '🚌';
    case 11:
      return '🚎';
    default:
      return '🚌';
  }
}

const hasTimetable = (timetable: Timetable): boolean => {
  const hasWeekday = timetable?.weekdays?.entries?.length
  const hasSaturday = timetable?.saturday?.entries?.length
  const hasSunday = timetable?.sunday?.entries?.length
  return !!(hasWeekday || hasSaturday || hasSunday)
}

const isAvailableToday = (today: number, timetable: Timetable): boolean => {
  const hasWeekday = timetable?.weekdays?.entries?.length
  const hasSaturday = timetable?.saturday?.entries?.length
  const hasSunday = timetable?.sunday?.entries?.length
  return !!(today === 0 ? hasSunday : today === 6 ? hasSaturday : hasWeekday)
}

const busesWithAvailableTimetables = computed(() => {
  return stopInfo.value?.shapes_info.filter((shape: ShapeInfo) => hasTimetable(shape.timetable))
})

const getTimeOffsetToStop = (stopTime:StopTime[], tripId: string): number => {
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

const comingNext = computed(() => {
  if (!stopInfo.value) return []

  const today = userTime.value?.getDay() || new Date().getDay()
  const currentTimeInMinutes = getMinutesFromDate(userTime.value || new Date())
  const {outgoing_trip_ids, incoming_trip_ids, shapes_info} = stopInfo.value

  const results = []
  for (let i = 0; i < shapes_info.length; i++) {
    const {route_short_name, route_type, route_color, route_id, stop_time, timetable} = shapes_info[i]
    if (!hasTimetable(timetable) || !isAvailableToday(today, timetable)) continue

    const tripId = getTripId(outgoing_trip_ids, incoming_trip_ids, route_id)
    if (!tripId) continue

    const timeOffsetToStop = getTimeOffsetToStop(stop_time, tripId)
    const isOutgoing = tripId.endsWith(OUTGOING_SUFFIX)

    const todayTimetable = (today === 0 ? timetable.sunday : today === 6 ? timetable.saturday : timetable.weekdays)
    const todayTimes = todayTimetable
      .entries
      .map((entry: { departure_in: string; departure_out: string }) => {
        const timetableTime = timeStringToMinutes(isOutgoing ? entry.departure_in : entry.departure_out)
        return timetableTime === null ? timetableTime : timetableTime + timeOffsetToStop
      }).reduce((acc: number[], curr: number) => {
        const minutesLeft = ((curr - currentTimeInMinutes) + 1440) % 1440
        if (minutesLeft <= 30) acc.push(minutesLeft)
        return acc
      }, [])
    if (!todayTimes.length) continue

    results.push({
      minutes_left: Math.min(...todayTimes),
      route_short_name,
      route_type,
      route_color,
      route_long_name: isOutgoing
        ? `${timetable.route_long_name}`
        : `${reverseRouteLongName(timetable.route_long_name)}`,
      direction: isOutgoing ? 'outgoing' : 'incoming',
    })
  }
  return results.sort((a, b) => a.minutes_left - b.minutes_left)
})

watch(() => props.stopId, async (newValue) => {
  isLoading.value = true
  await fetchStopData(newValue)
  isLoading.value = false
}, {immediate: true})

watch(busesWithAvailableTimetables, (newVal) => {
  if (!stopInfo.value || !Array.isArray(newVal) || newVal.length === 0) return

  const routeIdsWithAvailableTimetables = newVal.map((shape: ShapeInfo) => shape.route_id)

  const {outgoing_trip_ids, incoming_trip_ids} = stopInfo.value
  const displayShapes: DisplayShape[] = [...(outgoing_trip_ids || []), ...(incoming_trip_ids || [])]
    .filter((trip_id) => routeIdsWithAvailableTimetables.some((route_id: number) => trip_id.startsWith(route_id.toString())))
    .reduce((acc: DisplayShape[], trip_id: string) => {
      const routeId = Number(trip_id.replace(OUTGOING_SUFFIX, '').replace(INCOMING_SUFFIX, ''))
      const shape = newVal.find((shape: ShapeInfo) => shape.route_id === routeId)
      if (shape) {
        acc.push({
          trip_id,
          route_short_name: shape.route_short_name,
          route_long_name: shape.route_long_name,
          route_color: shape.route_color,
          route_type: shape.route_type,
        })
      }
      return acc
    }, [])
  mapStore.setShapesToDisplay(displayShapes)
})
</script>

<template>
  <div v-if="isLoading"
       class="stop-view-container bg-white dark:bg-[#0f172a] p-5 h-full flex flex-col gap-10 animate-pulse">

    <header class="flex items-center gap-4">
      <div class="w-12 h-12 shrink-0 rounded-full bg-slate-200 dark:bg-slate-800"></div>
      <div class="h-8 w-48 bg-slate-200 dark:bg-slate-800 rounded-lg"></div>
    </header>

    <section>
      <div class="h-3.5 w-36 bg-slate-200 dark:bg-slate-800 rounded mb-5"></div>

      <div class="flex flex-col gap-4">
        <div v-for="i in 3" :key="'skeleton-next-'+i"
             class="bg-slate-50 dark:bg-slate-800/40 rounded-2xl p-4 pr-5 flex items-center gap-4 border border-slate-100 dark:border-slate-800/50">
          <div class="shrink-0 w-14 h-12 rounded-xl bg-slate-200 dark:bg-slate-700/50"></div>

          <div class="flex-1 flex flex-col gap-2 justify-center py-1">
            <div class="h-2.5 w-24 bg-slate-200 dark:bg-slate-700/50 rounded"></div>
            <div class="h-4 w-40 bg-slate-200 dark:bg-slate-700/50 rounded"></div>
          </div>

          <div class="shrink-0 w-10 h-8 bg-slate-200 dark:bg-slate-700/50 rounded-lg mr-1"></div>
        </div>
      </div>
    </section>

    <section>
      <div class="h-3.5 w-48 bg-slate-200 dark:bg-slate-800 rounded mb-5"></div>

      <div class="grid grid-cols-1 xl:grid-cols-2 gap-x-4 gap-y-3">
        <div v-for="i in 6" :key="'skeleton-all-'+i" class="flex items-center gap-3 p-2 -mx-2">
          <div class="shrink-0 w-10 h-8 rounded-md bg-slate-200 dark:bg-slate-800"></div>
          <div class="h-3.5 w-32 bg-slate-200 dark:bg-slate-800 rounded"></div>
        </div>
      </div>
    </section>

  </div>
  <div v-else
       class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 h-full overflow-y-auto flex flex-col gap-10 font-sans shadow-2xl transition-colors duration-300 relative">

    <header class="flex items-center gap-4">
      <div
        class="w-12 h-12 shrink-0 rounded-full bg-emerald-100 dark:bg-emerald-500/20 flex items-center justify-center text-emerald-600 dark:text-emerald-400 shadow-sm border border-emerald-200 dark:border-emerald-500/30 transition-colors">
        <StopIcon class="w-6 h-6"/>
      </div>
      <h1
        class="text-2xl sm:text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white leading-tight transition-colors">
        {{ stopName }}
      </h1>
    </header>

    <section>
      <h2
        class="text-sm font-bold text-slate-500 dark:text-slate-400 uppercase tracking-[0.2em] mb-5 flex items-center gap-3 transition-colors">
        <span
          class="w-2.5 h-2.5 rounded-full bg-emerald-500 animate-pulse shadow-[0_0_8px_rgba(16,185,129,0.5)]"></span>
        Next Departures
      </h2>

      <div class="flex flex-col gap-4">
        <div v-for="shape in comingNext" :key="shape.route_short_name"
             class="bg-slate-50 hover:bg-slate-100 dark:bg-slate-800/60 dark:hover:bg-slate-800/80 transition-all rounded-2xl p-4 pr-5 flex items-center gap-4 border border-slate-200 dark:border-slate-700/50 shadow-sm dark:shadow-md relative overflow-hidden group">

          <div
            class="flex items-center justify-center shrink-0 w-14 h-12 rounded-xl font-black text-lg text-white shadow-sm z-10"
            :style="{ backgroundColor: formatGtfsColor(shape.route_color) }">
            {{ shape.route_short_name }}
          </div>

          <div class="flex-1 min-w-0 flex flex-col justify-center z-10">
            <span
              class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-0.5 flex items-center gap-1.5 transition-colors">
              {{ getRouteTypeLabel(shape.route_type) }} Scheduled
            </span>
            <span
              class="text-base font-bold text-slate-800 dark:text-slate-200 truncate group-hover:text-slate-900 dark:group-hover:text-white transition-colors">
              {{ shape.route_long_name }}
            </span>
          </div>

          <div class="flex flex-col items-end justify-center shrink-0 min-w-16 z-10 mr-3!">
            <div class="flex items-baseline gap-1">
              <span v-if="shape.minutes_left > 0"
                    class="text-slate-400 dark:text-slate-500 font-semibold text-xl transition-colors">~</span>
              <span
                class="text-3xl font-black tracking-tighter transition-colors"
                :class="shape.minutes_left <= 5 ? 'text-rose-500 dark:text-rose-400 animate-pulse' : 'text-emerald-600 dark:text-emerald-400'">
                {{ shape.minutes_left === 0 ? 'NOW' : shape.minutes_left }}
              </span>
            </div>
            <span v-if="shape.minutes_left > 0"
                  class="text-[10px] text-slate-500 dark:text-slate-400 font-bold uppercase tracking-widest mt-0.5 transition-colors">
              min
            </span>
          </div>

        </div>
      </div>
    </section>

    <section>
      <h2
        class="text-sm font-bold text-slate-500 dark:text-slate-400 uppercase tracking-[0.2em] mb-5 border-b border-slate-200 dark:border-slate-700/50 pb-3 transition-colors">
        All Routes at this Stop
      </h2>

      <div class="grid grid-cols-1 xl:grid-cols-2 gap-x-4 gap-y-3">
        <div v-for="shape in busesWithAvailableTimetables" :key="shape.timetable.route_short_name"
             class="flex items-center gap-3 group cursor-default p-2 -mx-2 rounded-lg hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors border border-transparent hover:border-slate-200 dark:hover:border-slate-700/50">

          <div
            class="flex items-center justify-center shrink-0 w-10 h-8 rounded-md text-xs font-black text-white shadow-sm opacity-90 group-hover:opacity-100 transition-opacity"
            :style="{ backgroundColor: formatGtfsColor(shape.route_color) }">
            {{ shape.timetable.route_short_name }}
          </div>

          <span
            class="text-sm font-semibold text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
              {{ shape.timetable.route_long_name }}
            </span>
        </div>
      </div>
    </section>

  </div>
</template>

<style scoped>
.stop-view-container {
  padding-left: 2rem;
  padding-right: 2rem;
  padding-top: 0.5rem;
}
</style>

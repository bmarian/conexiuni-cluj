<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts"
import {storeToRefs} from "pinia"
import {computed, watch} from "vue"
import {useStopInfoApi} from "@/composables/useStopInfoApi.ts"
import StopIcon from "../../public/stop.svg"
import type {Timetable} from "@/types/ctp.ts";
import {INCOMING_SUFFIX, OUTGOING_SUFFIX} from "@/types/tranzy.ts";
import {getMinutesFromDate, timeStringToMinutes} from "@/utils/time.ts";

const props = defineProps<{
  stopId: string
}>()
const userStore = useUserStore()
const {userTime} = storeToRefs(userStore)
const {stopInfo, fetchStopData} = useStopInfoApi()
const stopName = computed(() => stopInfo.value?.stop_name)

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

const comingNext = computed(() => {
  if (!stopInfo.value) return []

  const today = userTime.value?.getDay() || new Date().getDay()
  const currentTimeInMinutes = getMinutesFromDate(userTime.value || new Date())

  const results: unknown[] = []
  const {outgoing_trip_ids, incoming_trip_ids, shapes_info} = stopInfo.value
  for (let i = 0; i < shapes_info.length; i++) {
    const {route_short_name, stop_time, timetable} = shapes_info[i]
    if (!hasTimetable(timetable) || !isAvailableToday(today, timetable)) continue

    // TODO: Possible issue to have the same bus in both directions in the same stop
    // e.g 45 with the trip_ids 113_0 and 113_1
    // if that happens, just ignore it or something 🤷‍♂️
    const routeId = stop_time[0].trip_id.replace(OUTGOING_SUFFIX, '').replace(INCOMING_SUFFIX, '')
    const tripId = [...outgoing_trip_ids, ...incoming_trip_ids].find((id) => id.startsWith(routeId))
    if (!tripId) continue

    let timeOffsetToStop = 0
    for (let j = 0; j < stop_time.length; j++) {
      const {trip_id, stop_id, offset_arrival_time} = stop_time[j]
      if (trip_id !== tripId) continue
      if (stop_id !== props.stopId) break

      timeOffsetToStop += Math.ceil(offset_arrival_time/60)
    }

    const todayTimetable = (today === 0 ? timetable.sunday : today === 6 ? timetable.saturday : timetable.weekdays)
      .entries
      .map((entry: { departure_in: string; departure_out: string }) => {
        const timetableTime = timeStringToMinutes(tripId.endsWith(OUTGOING_SUFFIX) ? entry.departure_in : entry.departure_out)
        return timetableTime === null ? timetableTime : timetableTime + timeOffsetToStop
      })
    console.log(todayTimetable)
  }


  return results
})

watch(() => props.stopId, async (newValue) => {
  await fetchStopData(newValue)
}, {immediate: true})
</script>

<template>
  <h1>
    <StopIcon/>
    <span>{{ stopName }}</span></h1>
  <div>
    <h2>Next Buses</h2>
    <ul>
      <li v-for="shape in comingNext"></li>
    </ul>
  </div>
  <div>
    <h2>Available Buses</h2>
    <ul>
      <li v-for="shape in stopInfo?.shapes_info" :key="shape.route_short_name">
        {{ shape.timetable.route_short_name }} {{ shape.timetable.route_long_name }}
      </li>
    </ul>
  </div>

</template>

<style scoped lang="scss">

</style>

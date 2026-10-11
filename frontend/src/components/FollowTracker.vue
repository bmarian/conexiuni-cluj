<script setup lang="ts">
import {onMounted, onUnmounted, shallowRef, watch} from 'vue'
import {storeToRefs} from 'pinia'
import type {Shape, Vehicle} from '@/types/tranzy.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'
import {useFollowsStore} from '@/stores/follows.ts'
import {useMapStore} from '@/stores/map.ts'
import {useUserStore} from '@/stores/user.ts'
import {targetReading, walkMinutes, type FollowReading, type Followed} from '@/utils/follow.ts'
import {tripDepartures} from '@/utils/legDeparture.ts'

const props = defineProps<{ followed: Followed, vehicles: Map<string, Vehicle[]> }>()

const STOP_REFRESH_MS = 10 * 60_000

const follows = useFollowsStore()
const mapStore = useMapStore()
const {userTime, userLocation} = storeToRefs(useUserStore())
const {stopInfo, fetchStopData} = useStopInfoApi()
const shapes = shallowRef(new Map<string, Shape[]>())
let refresh: ReturnType<typeof setInterval> | null = null
let sequence = 0

async function loadStop() {
  await fetchStopData(String(props.followed.stopId))
  if (!stopInfo.value || shapes.value.has(props.followed.tripId)) return
  try {
    const [entry] = await mapStore.requestShapes([{
      trip_id: props.followed.tripId,
      route_short_name: props.followed.line,
      route_long_name: '',
      route_color: props.followed.color,
      route_type: 3,
    }])
    if (entry?.[1].length) shapes.value = new Map([[props.followed.tripId, entry[1]]])
  } catch {}
}

async function read() {
  const stop = stopInfo.value
  if (!stop) return
  const run = ++sequence
  const nowMs = Date.now()
  const followed = props.followed
  const [departure] = await tripDepartures(stop, [followed.tripId], shapes.value, props.vehicles, new Date(nowMs))
  if (run !== sequence) return
  let reading: FollowReading | null
  if (followed.targetAt !== undefined) {
    reading = targetReading(departure?.scheduled ?? [], departure?.liveMinutes ?? null, followed.targetAt, nowMs)
  } else {
    reading = departure ? {minutes: departure.minutes, live: departure.live, later: departure.later} : null
  }
  const here = userLocation.value
  const walk = here ? walkMinutes(here.latitude, here.longitude, followed.stopLat, followed.stopLon) : null
  follows.onReading(followed.id, reading, walk, nowMs)
}

onMounted(() => {
  void loadStop()
  refresh = setInterval(() => void loadStop(), STOP_REFRESH_MS)
})
onUnmounted(() => {
  if (refresh) clearInterval(refresh)
  sequence++
})

watch([stopInfo, shapes, () => props.vehicles, userTime], () => void read())
</script>

<template>
  <span hidden></span>
</template>

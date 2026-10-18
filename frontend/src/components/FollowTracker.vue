<script setup lang="ts">
import {onMounted, onUnmounted, watch} from 'vue'
import {storeToRefs} from 'pinia'
import type {LiveRow} from '@/types/tranzy.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'
import {useFollowsStore} from '@/stores/follows.ts'
import {useUserStore} from '@/stores/user.ts'
import {targetReading, type FollowReading, type Followed} from '@/utils/follow.ts'
import {tripDepartures} from '@/utils/legDeparture.ts'

const props = defineProps<{ followed: Followed, rows: Map<string, LiveRow>, asOfMs: number | null }>()

const STOP_REFRESH_MS = 10 * 60_000

const follows = useFollowsStore()
const {userTime} = storeToRefs(useUserStore())
const {stopInfo, fetchStopData} = useStopInfoApi()
let refresh: ReturnType<typeof setInterval> | null = null

async function loadStop() {
  await fetchStopData(String(props.followed.stopId))
}

function read() {
  const stop = stopInfo.value
  if (!stop) return
  const nowMs = Date.now()
  const followed = props.followed
  const [departure] = tripDepartures(stop, [followed.tripId], props.rows, props.asOfMs, new Date(nowMs))
  let reading: FollowReading | null
  if (followed.targetAt !== undefined) {
    reading = targetReading(departure?.scheduled ?? [], departure?.liveMinutes ?? null, followed.targetAt, nowMs, stop.late_grace_min ?? 1)
  } else {
    reading = departure ? {minutes: departure.minutes, live: departure.live, later: departure.later} : null
  }
  follows.onReading(followed.id, reading, nowMs)
}

onMounted(() => {
  void loadStop()
  refresh = setInterval(() => void loadStop(), STOP_REFRESH_MS)
})
onUnmounted(() => {
  if (refresh) clearInterval(refresh)
})

watch([stopInfo, () => props.rows, () => props.asOfMs, userTime], () => read())
</script>

<template>
  <span hidden></span>
</template>

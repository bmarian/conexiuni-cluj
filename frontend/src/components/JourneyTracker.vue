<script setup lang="ts">
import {onMounted, onUnmounted, ref, shallowRef, watch} from 'vue'
import type {LiveRow} from '@/types/tranzy.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'
import {useJourneyStore} from '@/stores/journey.ts'
import {tripDepartures} from '@/utils/legDeparture.ts'
import type {Fix, LegDeparture} from '@/utils/journey.ts'

const props = defineProps<{ rows: Map<string, LiveRow>, asOfMs: number | null }>()

const TICK_MS = 5_000

const store = useJourneyStore()
const {stopInfo, fetchStopData} = useStopInfoApi()
const fix = shallowRef<Fix | null>(null)
const departure = shallowRef<LegDeparture | null>(null)
const departureLeg = ref(-1)
const clock = ref(Date.now())
let handledSkips = store.skips
let watchId: number | null = null
let ticker: ReturnType<typeof setInterval> | null = null

function startLocation() {
  if (!('geolocation' in navigator)) return
  watchId = navigator.geolocation.watchPosition(
    (p) => {
      fix.value = {lat: p.coords.latitude, lon: p.coords.longitude, accuracy: p.coords.accuracy, speed: p.coords.speed ?? null}
    },
    () => {},
    {enableHighAccuracy: true, maximumAge: 2_000, timeout: 20_000},
  )
}

async function loadLeg(index: number) {
  departure.value = null
  departureLeg.value = index
  const leg = store.journey?.legs[index]
  if (!leg) return
  await fetchStopData(String(leg.stops[0]!.id))
}

function readDeparture() {
  const leg = store.journey?.legs[departureLeg.value]
  const stop = stopInfo.value
  if (!leg || !stop || stop.stop_id !== leg.stops[0]!.id) return
  const [first] = tripDepartures(stop, leg.tripIds, props.rows, props.asOfMs, new Date())
  departure.value = first ? {minutes: first.minutes, live: first.live, line: first.line, color: first.color} : null
}

function tick() {
  const skip = store.skips !== handledSkips
  handledSkips = store.skips
  const legDeparture = departureLeg.value === store.legIndex ? departure.value : null
  store.onTick(fix.value, legDeparture, Date.now(), skip)
}

watch(() => store.legIndex, (index) => {
  if (index >= 0 && index !== departureLeg.value) void loadLeg(index)
}, {immediate: true})
watch([stopInfo, () => props.rows, () => props.asOfMs, clock], () => readDeparture())
watch([fix, departure, clock, () => store.skips], tick)

onMounted(() => {
  startLocation()
  ticker = setInterval(() => { clock.value = Date.now() }, TICK_MS)
  tick()
})
onUnmounted(() => {
  if (watchId !== null) navigator.geolocation.clearWatch(watchId)
  if (ticker) clearInterval(ticker)
})
</script>

<template>
  <span hidden></span>
</template>

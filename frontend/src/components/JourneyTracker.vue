<script setup lang="ts">
import {onMounted, onUnmounted, ref, shallowRef, watch} from 'vue'
import type {Shape, Vehicle} from '@/types/tranzy.ts'
import {useStopInfoApi} from '@/composables/useStopInfoApi.ts'
import {useJourneyStore} from '@/stores/journey.ts'
import {useMapStore} from '@/stores/map.ts'
import {tripDepartures} from '@/utils/legDeparture.ts'
import type {Fix, LegDeparture} from '@/utils/journey.ts'

const props = defineProps<{ vehicles: Map<string, Vehicle[]> }>()

const TICK_MS = 5_000

const store = useJourneyStore()
const mapStore = useMapStore()
const {stopInfo, fetchStopData} = useStopInfoApi()
const shapes = shallowRef(new Map<string, Shape[]>())
const fix = shallowRef<Fix | null>(null)
const departure = shallowRef<LegDeparture | null>(null)
const departureLeg = ref(-1)
const clock = ref(Date.now())
let handledSkips = store.skips
let watchId: number | null = null
let ticker: ReturnType<typeof setInterval> | null = null
let sequence = 0

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
  try {
    const entries = await mapStore.requestShapes(leg.tripIds.map((tripId, i) => ({
      trip_id: tripId,
      route_short_name: leg.lines[i] ?? leg.lines[0] ?? '',
      route_long_name: '',
      route_color: leg.colors[i] ?? leg.colors[0] ?? '#64748b',
      route_type: 3,
    })))
    if (departureLeg.value === index) shapes.value = new Map(entries.map(([d, points]) => [d.trip_id, points]))
  } catch {}
}

async function readDeparture() {
  const leg = store.journey?.legs[departureLeg.value]
  const stop = stopInfo.value
  if (!leg || !stop || stop.stop_id !== leg.stops[0]!.id) return
  const run = ++sequence
  const [first] = await tripDepartures(stop, leg.tripIds, shapes.value, props.vehicles, new Date())
  if (run !== sequence) return
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
watch([stopInfo, shapes, () => props.vehicles, clock], () => void readDeparture())
watch([fix, departure, clock, () => store.skips], tick)

onMounted(() => {
  startLocation()
  ticker = setInterval(() => { clock.value = Date.now() }, TICK_MS)
  tick()
})
onUnmounted(() => {
  if (watchId !== null) navigator.geolocation.clearWatch(watchId)
  if (ticker) clearInterval(ticker)
  sequence++
})
</script>

<template>
  <span hidden></span>
</template>

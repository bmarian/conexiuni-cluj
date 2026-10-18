<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {useHead} from '@unhead/vue'
import {storeToRefs} from 'pinia'
import HeaderNavigation from '@/components/HeaderNavigation.vue'
import {useJourneyStore} from '@/stores/journey.ts'
import {useMapStore, type DisplayShape, type HighlightedStop} from '@/stores/map.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useUserStore} from '@/stores/user.ts'
import {findClosestShapeIdx} from '@/utils/geo.ts'
import {endsAtStop, lineLabel, type Journey, type JourneyStep, type StepKind} from '@/utils/journey.ts'
import {arriveText, stepDetail, stepTitle, thenText} from '@/utils/journeyText.ts'
import type {Shape} from '@/types/tranzy.ts'
import {cityPath} from '@/utils/city.ts'

const {t} = useI18n()
const router = useRouter()
const store = useJourneyStore()
const mapStore = useMapStore()
const settings = useSettingsStore()
const {userLocation, userTime} = storeToRefs(useUserStore())

const FOLLOW_EVERY_MS = 8_000

const journey = computed(() => store.journey)

useHead(() => ({
  title: journey.value ? t('headPlanToTitle', {dest: journey.value.destination.name}) : t('headPlanTitle'),
  meta: [{name: 'robots', content: 'noindex'}],
}))
const step = computed(() => store.step)
const now = computed(() => (userTime.value ?? new Date()).getTime())
const title = computed(() => (step.value ? stepTitle(step.value) : t('followWaiting')))
const detail = computed(() => (step.value ? stepDetail(step.value, now.value) : null))
const then = computed(() => (journey.value && step.value ? thenText(journey.value, step.value) : null))
const minutesLeft = computed(() => (step.value ? Math.max(0, Math.round((step.value.etaAt - now.value) / 60_000)) : null))
const following = ref(true)

interface Row {
  key: string
  kind: StepKind
  leg: number
  text: string
  lines: {line: string, color: string}[]
}

const rows = computed<Row[]>(() => {
  const j = journey.value
  if (!j) return []
  const out: Row[] = []
  j.legs.forEach((leg, i) => {
    const boarding = leg.stops[0]!
    const alighting = leg.stops[leg.stops.length - 1]!
    const lines = leg.lines.map((line, k) => ({line, color: leg.colors[k] ?? leg.colors[0] ?? '#64748b'}))
    if (leg.walkMinutes >= 0.5) out.push({key: `walk-${i}`, kind: 'walk', leg: i, text: t('journeyWalkTo', {name: boarding.name}), lines: []})
    out.push({key: `wait-${i}`, kind: 'wait', leg: i, text: t(i === 0 ? 'navGetOnAt' : 'navChangeAt', {name: boarding.name}), lines})
    out.push({key: `ride-${i}`, kind: 'ride', leg: i, text: t('journeyGetOffAt', {name: alighting.name}), lines: []})
  })
  if (!endsAtStop(j)) out.push({key: 'walkEnd', kind: 'walkEnd', leg: -1, text: t('journeyWalkTo', {name: j.destination.name}), lines: []})
  return out
})

function isCurrent(row: Row, s: JourneyStep | null) {
  if (!s) return false
  if (row.kind === 'walkEnd') return s.kind === 'walkEnd'
  return row.leg === s.legIndex && row.kind === s.kind
}

async function showOnMap(j: Journey) {
  const entries: Array<[DisplayShape, Shape[]]> = []
  for (const leg of j.legs) {
    const tripId = leg.tripIds[0]
    if (!tripId) continue
    try {
      const [loaded] = await mapStore.requestShapes([{
        trip_id: tripId, route_short_name: leg.lines[0] ?? '', route_long_name: '', route_color: leg.colors[0] ?? '#64748b', route_type: 3,
      }])
      const points = loaded?.[1] ?? []
      if (!loaded || !points.length) continue
      const a = findClosestShapeIdx(leg.stops[0]!.lat, leg.stops[0]!.lon, points)
      const b = findClosestShapeIdx(leg.stops[leg.stops.length - 1]!.lat, leg.stops[leg.stops.length - 1]!.lon, points)
      entries.push([loaded[0], points.slice(Math.min(a, b), Math.max(a, b) + 1)])
    } catch {}
  }
  if (store.journey?.id !== j.id) return
  mapStore.setLoadedShapes(entries)
  const highlighted: HighlightedStop[] = []
  j.legs.forEach((leg, i) => {
    highlighted.push({stopId: String(leg.stops[0]!.id), color: i === 0 ? 'green' : 'purple'})
    if (i === j.legs.length - 1) highlighted.push({stopId: String(leg.stops[leg.stops.length - 1]!.id), color: 'red'})
  })
  mapStore.setHighlightedStops(highlighted)
  mapStore.fitWalkingPolylines = !following.value
  mapStore.setWalkingPolylines(j.walks.map((w) => [...w]))
}

let lastFollow = 0
function followMe(force = false) {
  const here = userLocation.value
  if (!here || !following.value) return
  if (!force && Date.now() - lastFollow < FOLLOW_EVERY_MS) return
  lastFollow = Date.now()
  mapStore.setFlyToLocation(here.latitude, here.longitude)
}

function overview() {
  following.value = false
  if (journey.value) {
    mapStore.fitWalkingPolylines = true
    mapStore.setWalkingPolylines(journey.value.walks.map((w) => [...w]))
  }
}

function recenter() {
  following.value = true
  followMe(true)
}

let lock: {release: () => Promise<void>} | null = null
async function keepAwake() {
  try {
    lock = await (navigator as Navigator & {wakeLock?: {request: (type: 'screen') => Promise<{release: () => Promise<void>}>}}).wakeLock?.request('screen') ?? null
  } catch {}
}
function onVisibility() {
  if (document.visibilityState === 'visible' && store.journey) void keepAwake()
}

function end() {
  const back = journey.value?.planPath
  store.stop()
  void router.replace(back ?? '/')
}

watch(journey, (j, before) => {
  if (j) {
    void showOnMap(j)
    return
  }
  if (before) settings.showToast(t('journeyArrived'), {body: before.destination.name, icon: 'bell'})
  void router.replace(cityPath('/'))
}, {immediate: true})
watch(userLocation, () => followMe())
// On board, the traveller is the bus.
watch(step, (s) => {
  const leg = s?.kind === 'ride' ? journey.value?.legs[s.legIndex] : undefined
  mapStore.rideAs = leg ? {line: s?.line ?? leg.lines[0] ?? '', color: s?.color ?? leg.colors[0] ?? '#64748b'} : null
}, {immediate: true})

onMounted(() => {
  if (!store.journey) return
  mapStore.preciseLocation = true
  void keepAwake()
  document.addEventListener('visibilitychange', onVisibility)
  followMe(true)
})
onUnmounted(() => {
  document.removeEventListener('visibilitychange', onVisibility)
  void lock?.release().catch(() => {})
  mapStore.rideAs = null
  mapStore.preciseLocation = false
  mapStore.setLoadedShapes([])
  mapStore.clearWalkingPolylines()
  mapStore.setHighlightedStops([])
})
</script>

<template>
  <div v-if="journey" class="nav-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-4">
    <div class="flex items-center -mb-2">
      <HeaderNavigation/>
    </div>

    <section class="nav-banner" :class="`is-${step?.kind ?? 'walk'}`" aria-live="polite">
      <svg v-if="step?.kind === 'ride'" class="w-9 h-9 shrink-0" viewBox="0 0 24 24" fill="currentColor">
        <path d="M4 16c0 .9.4 1.7 1 2.2V20a1 1 0 0 0 1 1h1a1 1 0 0 0 1-1v-1h8v1a1 1 0 0 0 1 1h1a1 1 0 0 0 1-1v-1.8c.6-.5 1-1.3 1-2.2V6c0-3.5-3.6-4-8-4S4 2.5 4 6v10zm3.5 1a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3zm9 0a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3zm1.5-6H6V6h12v5z"/>
      </svg>
      <svg v-else-if="step?.kind === 'wait'" class="w-9 h-9 shrink-0" viewBox="0 0 24 24" fill="currentColor">
        <path d="M6 2v6l4 4-4 4v6h12v-6l-4-4 4-4V2H6zm10 14.5V20H8v-3.5l4-4 4 4z"/>
      </svg>
      <svg v-else-if="step?.kind === 'arrived'" class="w-9 h-9 shrink-0" viewBox="0 0 24 24" fill="currentColor">
        <path d="M14.4 6L14 4H5v17h2v-7h5.6l.4 2h7V6z"/>
      </svg>
      <svg v-else class="w-9 h-9 shrink-0" viewBox="0 0 24 24" fill="currentColor">
        <path d="M13.5 5.5a2 2 0 1 0 0-4 2 2 0 0 0 0 4zM9.8 8.9L7 23h2.1l1.8-8 2.1 2v6h2v-7.5l-2.1-2 .6-3A7.3 7.3 0 0 0 19 13v-2a5 5 0 0 1-4.3-2.4l-1-1.6a2 2 0 0 0-1.7-1c-.3 0-.5.1-.8.1L6 8.3V13h2V9.6l1.8-.7"/>
      </svg>
      <div class="min-w-0 flex-1">
        <p class="nav-title">{{ title }}</p>
        <p v-if="detail" class="nav-detail">{{ detail }}</p>
      </div>
      <span v-if="step?.line" class="nav-badge" :style="{ backgroundColor: step.color ?? '#64748b' }">{{ step.line }}</span>
    </section>

    <p v-if="then" class="nav-then">
      <svg class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.4">
        <path stroke-linecap="round" stroke-linejoin="round" d="M5 4v7a3 3 0 0 0 3 3h11m-4-4l4 4-4 4"/>
      </svg>
      {{ then }}
    </p>

    <div class="nav-eta">
      <div class="min-w-0">
        <p v-if="minutesLeft !== null" class="nav-eta-minutes">{{ t('etaMinutes', {n: minutesLeft}) }}</p>
        <p v-if="step" class="nav-eta-at">{{ arriveText(step.etaAt) }} · {{ journey.destination.name }}</p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <button type="button" class="nav-round" :aria-label="following ? t('navOverview') : t('navRecenter')"
                :title="following ? t('navOverview') : t('navRecenter')" @click="following ? overview() : recenter()">
          <svg v-if="following" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 20l-5.4-2.7A1 1 0 0 1 3 16.4V5.6a1 1 0 0 1 1.4-.9L9 7m0 13l6-3m-6 3V7m6 10l4.6 2.3a1 1 0 0 0 1.4-.9V7.6a1 1 0 0 0-.6-.9L15 4m0 13V4m0 0L9 7"/>
          </svg>
          <svg v-else class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2L4.5 20.3l.7.7L12 18l6.8 3 .7-.7z"/></svg>
        </button>
        <button type="button" class="nav-next" @click="store.next()">{{ t('journeyNext') }}</button>
        <button type="button" class="nav-end" @click="end">{{ t('journeyEnd') }}</button>
      </div>
    </div>
    <div class="nav-progress" role="progressbar" :aria-valuenow="Math.round((step?.progress ?? 0) * 100)" aria-valuemin="0" aria-valuemax="100">
      <span :style="{ width: `${(step?.progress ?? 0) * 100}%` }"></span>
    </div>

    <ol class="nav-steps">
      <li v-for="row in rows" :key="row.key" class="nav-step" :class="{ 'is-current': isCurrent(row, step) }">
        <span class="nav-step-dot"></span>
        <span class="min-w-0 flex-1">{{ row.text }}</span>
        <span v-if="row.lines.length" class="nav-step-lines">
          <span v-for="l in row.lines" :key="l.line" class="nav-step-badge" :style="{ backgroundColor: l.color }">{{ l.line }}</span>
        </span>
      </li>
      <li class="nav-step" :class="{ 'is-current': step?.kind === 'arrived' }">
        <span class="nav-step-dot is-end"></span>
        <span class="min-w-0 flex-1">{{ t('journeyArrived') }} · {{ journey.destination.name }}</span>
      </li>
    </ol>
    <p class="sr-only">{{ journey.legs.map(lineLabel).join(', ') }}</p>
  </div>
</template>

<style scoped>
.nav-view-container {
  padding: 1.25rem 1.5rem 1.5rem;
  height: 100%;
  overflow-y: auto;
}

.nav-banner {
  display: flex;
  align-items: center;
  gap: 0.875rem;
  padding: 1rem;
  border-radius: 1.25rem;
  background: var(--color-sky-600, #0284c7);
  color: var(--color-white, #fff);
  box-shadow: 0 6px 18px rgba(2, 132, 199, 0.3);
}

.nav-banner.is-ride {
  background: var(--color-emerald-600, #059669);
  box-shadow: 0 6px 18px rgba(5, 150, 105, 0.3);
}

.nav-banner.is-arrived {
  background: var(--color-violet-600, #7c3aed);
}

.nav-title {
  font-size: 1.125rem;
  font-weight: 800;
  line-height: 1.25;
}

.nav-detail {
  font-size: 0.9375rem;
  opacity: 0.92;
}

.nav-badge {
  min-width: 2.75rem;
  height: 2rem;
  padding: 0 0.5rem;
  border-radius: 0.5rem;
  border: 2px solid rgba(255, 255, 255, 0.85);
  color: var(--color-white, #fff);
  font-weight: 900;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.nav-then {
  align-self: flex-start;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: -0.25rem 0 0 0.75rem;
  padding: 0.4rem 0.8rem;
  border-radius: 0.875rem;
  font-size: 0.8125rem;
  font-weight: 600;
  background: var(--color-sky-100, #e0f2fe);
  color: var(--color-sky-800, #075985);
}

.nav-eta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.nav-eta-minutes {
  font-size: 1.5rem;
  font-weight: 800;
  color: var(--color-emerald-600, #059669);
  line-height: 1.1;
}

.nav-eta-at {
  font-size: 0.8125rem;
  color: var(--color-slate-500, #64748b);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-round {
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-slate-100, #f1f5f9);
  color: var(--color-sky-600, #0284c7);
}

.nav-next {
  padding: 0.5rem 0.9rem;
  border-radius: 9999px;
  font-size: 0.8125rem;
  font-weight: 700;
  background: var(--color-sky-100, #e0f2fe);
  color: var(--color-sky-800, #075985);
}

.nav-end {
  padding: 0.5rem 0.9rem;
  border-radius: 9999px;
  font-size: 0.8125rem;
  font-weight: 700;
  background: var(--color-red-600, #dc2626);
  color: var(--color-white, #fff);
}

.nav-progress {
  height: 0.375rem;
  border-radius: 9999px;
  background: var(--color-slate-200, #e2e8f0);
  overflow: hidden;
}

.nav-progress span {
  display: block;
  height: 100%;
  background: var(--color-emerald-500, #10b981);
  transition: width 0.6s ease;
}

.nav-steps {
  display: flex;
  flex-direction: column;
}

.nav-step {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 0;
  font-size: 0.875rem;
  color: var(--color-slate-500, #64748b);
}

.nav-step.is-current {
  color: var(--color-slate-900, #0f172a);
  font-weight: 700;
}

.nav-step-dot {
  width: 0.625rem;
  height: 0.625rem;
  border-radius: 9999px;
  border: 2px solid var(--color-slate-300, #cbd5e1);
  flex-shrink: 0;
}

.nav-step.is-current .nav-step-dot {
  border-color: var(--color-sky-600, #0284c7);
  background: var(--color-sky-600, #0284c7);
}

.nav-step-dot.is-end {
  border-color: var(--color-rose-500, #f43f5e);
}

.nav-step-lines {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.25rem;
  max-width: 50%;
}

.nav-step-badge {
  min-width: 2rem;
  height: 1.5rem;
  padding: 0 0.35rem;
  border-radius: 0.4rem;
  color: var(--color-white, #fff);
  font-size: 0.75rem;
  font-weight: 900;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

:root.dark .nav-then {
  background: rgba(2, 132, 199, 0.2);
  color: var(--color-sky-200, #bae6fd);
}

:root.dark .nav-round,
:root.dark .nav-next {
  background: var(--color-slate-800, #1e293b);
  color: var(--color-sky-300, #7dd3fc);
}

:root.dark .nav-progress {
  background: var(--color-slate-800, #1e293b);
}

:root.dark .nav-eta-minutes {
  color: var(--color-emerald-400, #34d399);
}

:root.dark .nav-step.is-current {
  color: var(--color-slate-100, #f1f5f9);
}

:root.dark .nav-step-dot {
  border-color: var(--color-slate-600, #475569);
}
</style>

<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {useHead} from '@unhead/vue'
import {storeToRefs} from 'pinia'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import {
  CloseIcon,
  DirectionsBusIcon,
  DirectionsWalkIcon,
  FlagIcon,
  HourglassTopIcon,
  NavigationIcon,
  RouteIcon,
  SubdirectoryArrowRightIcon,
} from '@/utils/icons.ts'
import {useJourneyStore} from '@/stores/journey.ts'
import {useMapStore, type DisplayShape, type HighlightedStop} from '@/stores/map.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useUserStore} from '@/stores/user.ts'
import {findClosestShapeIdx} from '@/utils/geo.ts'
import {endsAtStop, lineLabel, type Journey, type JourneyStep, type StepKind} from '@/utils/journey.ts'
import {arriveText, stepDetail, stepTitle, thenText} from '@/utils/journeyText.ts'
import type {Shape} from '@/types/tranzy.ts'
import {cityParams, cityPath} from '@/utils/city.ts'

const {t} = useI18n()
const router = useRouter()
const store = useJourneyStore()
const mapStore = useMapStore()
const settings = useSettingsStore()
const {userLocation, userTime} = storeToRefs(useUserStore())

const FOLLOW_EVERY_MS = 8_000

const journey = computed(() => store.journey)

useHead(() => ({
  title: journey.value ? t('headPlanToTitle', {...cityParams(), dest: journey.value.destination.name}) : t('headPlanTitle'),
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

const stepIcon = computed(() => ({wait: HourglassTopIcon, ride: DirectionsBusIcon, arrived: FlagIcon})[step.value?.kind as 'wait' | 'ride' | 'arrived'] ?? DirectionsWalkIcon)
</script>
<template>
  <div class="nav-view">
    <SheetTopBar/>
    <template v-if="journey">
      <section class="banner" aria-live="polite">
        <Transition name="step" mode="out-in">
          <div :key="`${title}|${detail}`" class="flex items-center gap-3.5">
            <component :is="stepIcon" class="m-icon size-10"/>
            <div class="flex-1 min-w-0">
              <p class="text-title-large font-bold line-clamp-2">{{ title }}</p>
              <p v-if="detail" class="text-body-large line-clamp-2">{{ detail }}</p>
            </div>
            <LineBadge v-if="step?.line" :name="step.line" :color="step.color ?? '#64748b'" :height="32" :min-width="44" :corner="8"/>
          </div>
        </Transition>
      </section>

      <Transition name="fade">
        <p v-if="then" class="then">
          <SubdirectoryArrowRightIcon class="m-icon size-[18px]"/>
          <span class="truncate">{{ then }}</span>
        </p>
      </Transition>

      <section class="panel">
        <div class="flex items-center gap-3">
          <button type="button" class="end squish" :aria-label="t('journeyEnd')" :title="t('journeyEnd')" @click="end">
            <CloseIcon class="m-icon"/>
          </button>
          <div class="flex-1 min-w-0 flex flex-col items-center">
            <p class="text-headline-medium font-extrabold text-live">{{ minutesLeft !== null ? t('etaMinutes', {n: minutesLeft}) : '—' }}</p>
            <p v-if="step" class="text-body-medium text-on-surface-variant truncate max-w-full">{{ arriveText(step.etaAt) }} · {{ journey.destination.name }}</p>
          </div>
          <button type="button" class="btn btn-tonal" @click="store.next()">{{ t('journeyNext') }}</button>
        </div>
        <div class="flex items-center gap-3">
          <div class="progress" role="progressbar" :aria-valuenow="Math.round((step?.progress ?? 0) * 100)" aria-valuemin="0" aria-valuemax="100">
            <span :style="{ width: `${(step?.progress ?? 0) * 100}%` }"></span>
          </div>
          <button
            type="button"
            class="icon-btn"
            :aria-label="following ? t('navOverview') : t('navRecenter')"
            :title="following ? t('navOverview') : t('navRecenter')"
            @click="following ? overview() : recenter()"
          >
            <RouteIcon v-if="following" class="m-icon"/>
            <NavigationIcon v-else class="m-icon text-primary"/>
          </button>
        </div>
      </section>

      <ol class="steps">
        <li v-for="row in rows" :key="row.key" class="step" :class="{ 'is-current': isCurrent(row, step) }">
          <span class="dot"></span>
          <span class="flex-1 min-w-0 text-body-medium">{{ row.text }}</span>
          <span v-if="row.lines.length" class="flex flex-wrap gap-1 shrink-0">
            <LineBadge v-for="l in row.lines" :key="l.line" :name="l.line" :color="l.color" :height="22" :min-width="30" :corner="5"/>
          </span>
        </li>
        <li class="step" :class="{ 'is-current': step?.kind === 'arrived' }">
          <span class="dot is-end"></span>
          <span class="flex-1 min-w-0 text-body-medium">{{ t('journeyArrived') }} · {{ journey.destination.name }}</span>
        </li>
      </ol>
      <p class="sr-only">{{ journey.legs.map(lineLabel).join(', ') }}</p>
      <div class="h-6"></div>
    </template>
  </div>
</template>

<style scoped>
.banner {
  margin: 0 8px;
  padding: 14px 16px;
  border-radius: 24px;
  background: var(--md-primary);
  color: var(--md-on-primary);
  box-shadow: var(--md-elevation-2);
}

.then {
  display: flex;
  align-items: center;
  gap: 8px;
  width: fit-content;
  max-width: calc(100% - 28px);
  margin: 6px 8px 0 20px;
  padding: 8px 12px;
  border-radius: 16px;
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
  box-shadow: var(--md-elevation-1);
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 12px 8px 0;
  padding: 12px 16px;
  border-radius: 28px;
  background: var(--md-surface-container-high);
}

.end {
  --squish: 0.9;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 48px;
  height: 48px;
  border: 0;
  border-radius: 24px;
  background: var(--md-error-container);
  color: var(--md-on-error-container);
  cursor: pointer;
}

.progress {
  position: relative;
  flex: 1;
  height: 4px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--md-secondary-container);
}

.progress > span {
  position: absolute;
  inset: 0 auto 0 0;
  border-radius: 2px;
  background: var(--md-primary);
  transition: width 400ms ease;
}

.steps {
  display: flex;
  flex-direction: column;
  padding: 12px 0 0;
}

.step {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  padding: 6px 16px 6px 46px;
  color: var(--md-on-surface-variant);
}

.step::before {
  content: "";
  position: absolute;
  left: 25px;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--md-outline-variant);
}

.step:first-child::before {
  top: 50%;
}

.step:last-child::before {
  bottom: 50%;
}

.dot {
  position: absolute;
  left: 21px;
  top: 50%;
  width: 10px;
  height: 10px;
  margin-top: -5px;
  border-radius: 50%;
  background: var(--md-surface-container-low);
  box-shadow: inset 0 0 0 2px var(--md-outline-variant);
}

.dot.is-end {
  background: var(--md-outline);
  box-shadow: none;
}

.step.is-current {
  background: color-mix(in srgb, var(--md-primary) 8%, transparent);
  color: var(--md-on-surface);
  font-weight: 600;
}

.step.is-current .dot {
  width: 14px;
  height: 14px;
  left: 19px;
  margin-top: -7px;
  background: var(--md-primary);
  box-shadow: 0 0 0 2px var(--md-surface-container-low);
}

.step-enter-active,
.step-leave-active,
.fade-enter-active,
.fade-leave-active {
  transition: opacity 180ms linear;
}

.step-enter-from,
.step-leave-to,
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>

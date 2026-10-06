<script setup lang="ts">
import {computed, toRef, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useUserStore} from '@/stores/user.ts'
import {useRouteArrivals} from '@/composables/useRouteArrivals.ts'
import WatchMap, {type WatchMapFocus, type WatchMapLabel} from '@/components/watch/WatchMap.vue'
import IconHeartFilled from '@/components/icons/IconHeartFilled.vue'
import IconHeartOutline from '@/components/icons/IconHeartOutline.vue'
import {getTimetableForDay, timeStringToMinutes} from '@/utils/time.ts'

const FROM_STOP_ZOOM = 15

const props = defineProps<{ routeId: string; direction: string }>()

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const favoritesStore = useFavoritesStore()
const {userTime} = storeToRefs(useUserStore())
const {
  shapeInfo,
  fromStopId,
  currentDirection,
  isOutgoing,
  timetable,
  stopsForDirection,
  hasOutgoing,
  hasIncoming,
  currentMinutes,
  stopTimesByStop,
  isInitialLoading,
} = useRouteArrivals(toRef(props, 'routeId'), toRef(props, 'direction'))

const timetableOpen = computed(() => route.query.timetable !== undefined)
const isFavorite = computed(() => favoritesStore.isRouteFavorite(Number(props.routeId), currentDirection.value))
const canSwap = computed(() => currentDirection.value === '0' ? hasIncoming.value : hasOutgoing.value)

function swapDirection() {
  const direction = currentDirection.value === '0' ? '1' : '0'
  currentDirection.value = direction
  void router.replace({name: 'route', params: {routeId: props.routeId, direction}, query: route.query})
}

// The map and the timetable are two history entries, so going back after a swap lands
// on one that still has the old direction. The swap wins.
watch(() => props.direction, (direction) => {
  if (direction === currentDirection.value) return
  void router.replace({name: 'route', params: {routeId: props.routeId, direction: currentDirection.value}, query: route.query})
})

function toggleTimetable() {
  if (!timetableOpen.value) {
    void router.push({query: {timetable: '1'}})
  } else if (typeof window.history.state?.back === 'string' && window.history.state.back.startsWith(`/route/${props.routeId}/`)) {
    router.back()
  } else {
    void router.replace({query: {}})
  }
}

// Named like the favorites on the home screen, so a direction reads the same everywhere.
const terminal = computed(() => {
  const name = shapeInfo.value?.route_long_name ?? ''
  const i = name.lastIndexOf(' - ')
  if (i < 0) return name
  return isOutgoing.value ? name.slice(i + 3) : name.slice(0, i)
})

const labels = computed((): WatchMapLabel[] => stopsForDirection.value.flatMap((stop, idx) => {
  const next = stopTimesByStop.value[idx]?.[0]
  if (!next || !stop.stop_lat || !stop.stop_lon) return []
  const isFrom = String(stop.stop_id) === fromStopId.value
  return [{
    id: `${stop.stop_id}-${idx}`,
    lat: stop.stop_lat,
    lng: stop.stop_lon,
    text: next.label,
    live: next.isLive,
    rank: isFrom ? Number.MIN_SAFE_INTEGER : next.minutes,
  }]
}))

const focus = computed((): WatchMapFocus | null => {
  const stop = stopsForDirection.value.find((s) => String(s.stop_id) === fromStopId.value)
  return stop?.stop_lat && stop.stop_lon ? {lat: stop.stop_lat, lng: stop.stop_lon, zoom: FROM_STOP_ZOOM} : null
})

const daySchedule = computed(() => timetable.value ? getTimetableForDay(timetable.value, userTime.value || new Date()) : null)

const frequency = computed(() =>
  (isOutgoing.value ? daySchedule.value?.in_frequency : daySchedule.value?.out_frequency) ?? null)

const frequencyLabel = computed(() => {
  const f = frequency.value
  if (!f) return ''
  if (f.min_minutes && f.max_minutes && f.min_minutes !== f.max_minutes) {
    return t('frequencyEveryRange', {min: f.min_minutes, max: f.max_minutes})
  }
  return t('frequencyEvery', {min: f.min_minutes || f.max_minutes})
})

const departures = computed(() => (daySchedule.value?.entries ?? [])
  .map((entry) => timeStringToMinutes((isOutgoing.value ? entry.departure_in : entry.departure_out) ?? ''))
  .filter((m): m is number => m !== null)
  .sort((a, b) => a - b))

// From the current hour on: on a watch only what is still to come matters.
const hours = computed(() => {
  const now = currentMinutes.value
  const byHour = new Map<number, Array<{ text: string; past: boolean }>>()
  for (const m of departures.value) {
    const hour = Math.floor(m / 60)
    if (hour < Math.floor(now / 60)) continue
    if (!byHour.has(hour)) byHour.set(hour, [])
    byHour.get(hour)!.push({text: String(m % 60).padStart(2, '0'), past: m < now})
  }
  return [...byHour].map(([hour, minutes]) => ({hour: String(hour % 24).padStart(2, '0'), minutes}))
})
</script>

<template>
  <WatchMap :active="!timetableOpen" :labels="labels" :focus="focus"/>

  <template v-if="!timetableOpen">
    <div class="wt-map-title" aria-hidden="true">
      <span
        v-if="shapeInfo"
        class="wt-badge wt-badge-sm"
        :style="{ backgroundColor: shapeInfo.route_color }"
      >{{ shapeInfo.route_short_name }}</span>
      <span v-if="terminal" class="wt-map-title-text">→ {{ terminal }}</span>
    </div>
    <p v-if="isInitialLoading" class="wt-map-note">{{ t('loadingRoute') }}</p>
    <p v-else-if="!stopsForDirection.length" class="wt-map-note">{{ t('noSchedule') }}</p>
  </template>

  <div v-else class="wt-list">
    <header class="wt-head">
      <span
        v-if="shapeInfo"
        class="wt-badge"
        :style="{ backgroundColor: shapeInfo.route_color }"
      >{{ shapeInfo.route_short_name }}</span>
      <h1 v-if="terminal" class="wt-heading">→ {{ terminal }}</h1>
    </header>

    <button
      type="button"
      class="wt-row wt-row-action wt-fish"
      :aria-pressed="isFavorite"
      @click="favoritesStore.toggleRouteFavorite(Number(routeId), currentDirection)"
    >
      <IconHeartFilled v-if="isFavorite" class="wt-heart"/>
      <IconHeartOutline v-else/>
      {{ isFavorite ? t('removeFromFavorites') : t('addToFavorites') }}
    </button>

    <p v-if="frequencyLabel" class="wt-note">{{ frequencyLabel }}</p>
    <p v-else-if="!departures.length" class="wt-note">{{ t('noSchedule') }}</p>
    <p v-else-if="!hours.length" class="wt-note">{{ t('watchNoMoreToday') }}</p>
    <div v-for="row in hours" :key="row.hour" class="wt-tt-row wt-fish">
      <span class="wt-tt-hour">{{ row.hour }}</span>
      <span class="wt-tt-minutes">
        <span
          v-for="(minute, i) in row.minutes"
          :key="i"
          :class="{ 'is-past': minute.past }"
        >{{ minute.text }}</span>
      </span>
    </div>
  </div>

  <div class="wt-float-bar">
    <button
      v-if="canSwap"
      type="button"
      class="wt-float wt-float-icon"
      :aria-label="t('kbdDirection')"
      @click="swapDirection"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.25" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round" d="M7 16H20M7 16l3.5-3.5M7 16l3.5 3.5M17 8H4m13 0-3.5-3.5M17 8l-3.5 3.5"/>
      </svg>
    </button>
    <button type="button" class="wt-float" @click="toggleTimetable">
      {{ timetableOpen ? t('watchMap') : t('timetable') }}
    </button>
  </div>
</template>

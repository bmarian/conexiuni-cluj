<script setup lang="ts">
import {computed, inject, nextTick, ref, type Ref, toRef, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {type IndexedStop, useRouteArrivals} from '@/composables/useRouteArrivals.ts'
import IconHeartFilled from '@/components/icons/IconHeartFilled.vue'
import IconHeartOutline from '@/components/icons/IconHeartOutline.vue'

const props = defineProps<{ routeId: string; direction: string }>()

const {t} = useI18n()
const router = useRouter()
const favoritesStore = useFavoritesStore()
const restoring = inject<Ref<boolean>>('watchRestoring', ref(false))
const {
  shapeInfo,
  fromStopId,
  currentDirection,
  timetable,
  stopsForDirection,
  directionTerminals,
  hasOutgoing,
  hasIncoming,
  nearestStopIdx,
  stopTimesByStop,
  isInitialLoading,
} = useRouteArrivals(toRef(props, 'routeId'), toRef(props, 'direction'))

const isFavorite = computed(() => favoritesStore.isRouteFavorite(Number(props.routeId), currentDirection.value))

function terminal(dir: '0' | '1'): string {
  return dir === '0'
    ? directionTerminals.value.outgoing.last || timetable.value?.out_stop_name || ''
    : directionTerminals.value.incoming.last || timetable.value?.in_stop_name || ''
}

const otherDirection = computed(() => currentDirection.value === '0' ? '1' : '0')
const canSwitch = computed(() => otherDirection.value === '0' ? hasOutgoing.value : hasIncoming.value)

function switchDirection() {
  const dir = otherDirection.value
  void router.replace({name: 'route', params: {routeId: props.routeId, direction: dir}})
  currentDirection.value = dir
}

function isFrom(stop: IndexedStop): boolean {
  return String(stop.stop_id) === fromStopId.value
}

function dotClass(stop: IndexedStop, idx: number): string {
  if (isFrom(stop)) return 'is-from'
  if (idx === nearestStopIdx.value) return 'is-near'
  if (favoritesStore.isStopFavorite(stop.stop_id)) return 'is-fav'
  if (idx === 0 || idx === stopsForDirection.value.length - 1) return 'is-end'
  return ''
}

const rowEls: HTMLElement[] = []
let centeredFor = ''

// Open on the stop the user came from, or the one nearest to them once the position arrives.
watch([stopsForDirection, isInitialLoading, nearestStopIdx], async () => {
  const key = `${props.routeId}-${currentDirection.value}`
  if (isInitialLoading.value || !stopsForDirection.value.length || centeredFor === key) return
  const from = stopsForDirection.value.findIndex(isFrom)
  const anchor = from >= 0 ? from : nearestStopIdx.value
  if (anchor < 0 && !restoring.value && window.scrollY === 0) return
  centeredFor = key
  if (restoring.value || window.scrollY > 0) return
  await nextTick()
  rowEls[anchor]?.scrollIntoView({block: 'center'})
}, {immediate: true})
</script>

<template>
  <div class="wt-list">
    <header class="wt-head">
      <span
        v-if="shapeInfo"
        class="wt-badge"
        :style="{ backgroundColor: shapeInfo.route_color }"
      >{{ shapeInfo.route_short_name }}</span>
      <h1 v-if="terminal(currentDirection)" class="wt-heading">→ {{ terminal(currentDirection) }}</h1>
    </header>

    <p v-if="isInitialLoading" class="wt-note">{{ t('loadingRoute') }}</p>
    <p v-else-if="!stopsForDirection.length" class="wt-note">{{ t('noSchedule') }}</p>
    <div v-else class="wt-stops">
      <button
        v-for="(stop, idx) in stopsForDirection"
        :key="stop.stop_id + '-' + idx"
        :ref="(el) => { if (el) rowEls[idx] = el as HTMLElement }"
        type="button"
        class="wt-stop wt-fish"
        :class="dotClass(stop, idx)"
        @click="router.push({name: 'stop', params: {stopId: String(stop.stop_id)}})"
      >
        <span class="wt-stop-dot" aria-hidden="true"></span>
        <span class="wt-stop-name">{{ stop.stop_headsign || stop.stop_id }}</span>
        <span
          v-if="stopTimesByStop[idx]?.[0]"
          class="wt-stop-time"
          :class="{ 'is-live': stopTimesByStop[idx]![0]!.isLive }"
        >{{ stopTimesByStop[idx]![0]!.label }}</span>
      </button>
    </div>

    <button
      v-if="canSwitch && terminal(otherDirection)"
      type="button"
      class="wt-row wt-row-action wt-fish"
      @click="switchDirection"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M12 5l7 7-7 7"/>
      </svg>
      {{ t('watchTowards', {name: terminal(otherDirection)}) }}
    </button>
    <button type="button" class="wt-row wt-row-action wt-fish" @click="router.push({query: {map: '1'}})">
      {{ t('watchMap') }}
    </button>
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
  </div>
</template>

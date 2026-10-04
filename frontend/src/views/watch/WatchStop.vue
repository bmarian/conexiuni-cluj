<script setup lang="ts">
import {computed, toRef} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useRouteStore} from '@/stores/route.ts'
import {useUserStore} from '@/stores/user.ts'
import {useStopDepartures} from '@/composables/useStopDepartures.ts'
import IconHeartFilled from '@/components/icons/IconHeartFilled.vue'
import IconHeartOutline from '@/components/icons/IconHeartOutline.vue'
import type {ShapeInfo, VehiclesInStop} from '@/types/tranzy.ts'
import {formatMinutesFromNow} from '@/utils/time.ts'
import {getDirectionFromTripId} from '@/utils/trips.ts'

const props = defineProps<{ stopId: string }>()

const {t} = useI18n()
const router = useRouter()
const routeStore = useRouteStore()
const favoritesStore = useFavoritesStore()
const {userTime} = storeToRefs(useUserStore())
const {
  stopInfo,
  stopName,
  isLoading,
  loadError,
  isComputingDepartures,
  departures,
  isDepartureFavorite,
} = useStopDepartures(toRef(props, 'stopId'))

const isFavorite = computed(() => favoritesStore.isStopFavorite(Number(props.stopId)))

const departuresSorted = computed(() => [...departures.value].sort((a, b) => {
  const aFav = isDepartureFavorite(a) ? 0 : 1
  const bFav = isDepartureFavorite(b) ? 0 : 1
  if (aFav !== bFav) return aFav - bFav
  return a.minutes_left - b.minutes_left
}))

function formatMinutes(minutes: number): string {
  return formatMinutesFromNow(minutes, userTime.value || new Date(), t('now'))
}

function routeDestination(name: string): string {
  const i = name.lastIndexOf(' - ')
  return i >= 0 ? name.slice(i + 3) : name
}

function laterTimes(shape: VehiclesInStop): string {
  return (shape.next_times ?? []).slice(1).map((time) => formatMinutes(time.minutes)).join(' · ')
}

function openRoute(shape: VehiclesInStop) {
  const si = stopInfo.value?.shapes_info?.find((s: ShapeInfo) => s.route_id === shape.route_id)
  if (!si) return
  routeStore.setSelectedRoute(si, shape.trip_id, props.stopId, stopName.value || '')
  void router.push({
    name: 'route',
    params: {routeId: shape.route_id, direction: getDirectionFromTripId(shape.trip_id)},
  })
}
</script>

<template>
  <div class="wt-list">
    <header class="wt-head">
      <span class="wt-kicker">{{ t('busStop') }}</span>
      <h1 class="wt-heading">{{ stopName || '…' }}</h1>
    </header>

    <p v-if="loadError" class="wt-note">{{ t('watchLoadFailed') }}</p>
    <p v-else-if="isLoading || isComputingDepartures" class="wt-note">{{ t('loadingStop') }}</p>
    <p v-else-if="!departuresSorted.length" class="wt-note">{{ t('noSchedule') }}</p>
    <template v-else>
      <button
        v-for="shape in departuresSorted"
        :key="shape.route_short_name"
        type="button"
        class="wt-row wt-fish"
        @click="openRoute(shape)"
      >
        <span class="wt-badge" :style="{ backgroundColor: shape.route_color }">{{ shape.route_short_name }}</span>
        <span class="wt-row-text">
          <span class="wt-row-title">{{ routeDestination(shape.route_long_name) }}</span>
          <span v-if="laterTimes(shape)" class="wt-row-sub">{{ laterTimes(shape) }}</span>
        </span>
        <span class="wt-time" :class="{ 'is-live': !shape.static_time_approximation }">
          {{ shape.static_time_approximation ? '~' : '' }}{{ formatMinutes(shape.next_times?.[0]?.minutes ?? shape.minutes_left) }}
        </span>
      </button>
    </template>

    <button type="button" class="wt-row wt-row-action wt-fish" @click="router.push({query: {map: '1'}})">
      {{ t('watchMap') }}
    </button>
    <button
      type="button"
      class="wt-row wt-row-action wt-fish"
      :aria-pressed="isFavorite"
      @click="favoritesStore.toggleStopFavorite(Number(stopId))"
    >
      <IconHeartFilled v-if="isFavorite" class="wt-heart"/>
      <IconHeartOutline v-else/>
      {{ isFavorite ? t('removeFromFavorites') : t('addToFavorites') }}
    </button>
  </div>
</template>

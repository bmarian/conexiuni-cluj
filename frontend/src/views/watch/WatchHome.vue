<script setup lang="ts">
import {computed, inject, ref, type Ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {type FavoriteRoute, useFavoritesStore} from '@/stores/favorites.ts'
import {useRouteStore} from '@/stores/route.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useUserStore} from '@/stores/user.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import type {WatchLocationStatus} from '@/composables/useWatchLocation.ts'
import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type Route, type Stop} from '@/types/tranzy.ts'
import {formatMeters, haversineMeters, sortByDistance} from '@/utils/geo.ts'

const NEARBY_METERS = 700
const NEARBY_SHOWN = 5

const {t} = useI18n()
const router = useRouter()
const settings = useSettingsStore()
const routeStore = useRouteStore()
const favoritesStore = useFavoritesStore()
const {favoriteRoutes, favoriteStopIds} = storeToRefs(favoritesStore)
const {userLocation} = storeToRefs(useUserStore())
const {stops} = useStopsApi()
const {routes} = useRoutesApi()
const {fetchShapeInfo} = useRouteShapeInfoApi()
const locationStatus = inject<Ref<WatchLocationStatus>>('watchLocation', ref('unavailable'))

const stopsById = computed(() => new Map(stops.value.map((s) => [s.stop_id, s])))
const routesById = computed(() => new Map(routes.value.map((r) => [r.route_id, r])))

const favoriteStops = computed(() =>
  favoriteStopIds.value.map((id) => stopsById.value.get(id)).filter((s): s is Stop => !!s))

const favoriteRouteRows = computed(() => favoriteRoutes.value.flatMap((fav: FavoriteRoute) => {
  const route = routesById.value.get(fav.routeId)
  if (!route) return []
  const i = route.route_long_name.lastIndexOf(' - ')
  const origin = i >= 0 ? route.route_long_name.slice(0, i) : ''
  const end = i >= 0 ? route.route_long_name.slice(i + 3) : route.route_long_name
  return [{route, direction: fav.direction, destination: fav.direction === '1' && origin ? origin : end}]
}))

const nearby = computed(() => {
  const loc = userLocation.value
  if (!loc) return []
  return sortByDistance(stops.value, loc.latitude, loc.longitude, (s) => s.stop_lat, (s) => s.stop_lon, NEARBY_METERS)
    .slice(0, NEARBY_SHOWN)
    .map((stop) => ({stop, dist: formatMeters(haversineMeters(loc.latitude, loc.longitude, stop.stop_lat, stop.stop_lon))}))
})

const locationNote = computed(() => {
  if (userLocation.value) return nearby.value.length ? '' : t('watchNoNearby')
  return locationStatus.value === 'locating' ? t('watchLocating') : t('watchNoLocation')
})

function openStop(stop: Stop) {
  void router.push({name: 'stop', params: {stopId: String(stop.stop_id)}})
}

const openingRouteId = ref<number | null>(null)

async function openRoute(route: Route, direction: FavoriteRoute['direction']) {
  if (openingRouteId.value !== null) return
  openingRouteId.value = route.route_id
  try {
    const shapeInfo = await fetchShapeInfo(route)
    routeStore.setSelectedRoute(shapeInfo, `${route.route_id}${direction === '1' ? INCOMING_SUFFIX : OUTGOING_SUFFIX}`, '', '')
    await router.push({name: 'route', params: {routeId: String(route.route_id), direction}})
  } catch (e) {
    console.error('Failed to load route:', e)
  } finally {
    openingRouteId.value = null
  }
}
</script>

<template>
  <div class="wt-list">
    <template v-if="favoriteStops.length || favoriteRouteRows.length">
      <h2 v-if="favoriteStops.length" class="wt-label">{{ t('favoriteStops') }}</h2>
      <button
        v-for="stop in favoriteStops"
        :key="`fs-${stop.stop_id}`"
        type="button"
        class="wt-row wt-fish"
        @click="openStop(stop)"
      >
        <span class="wt-stop-icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
          </svg>
        </span>
        <span class="wt-row-text">
          <span class="wt-row-title">{{ stop.stop_name }}</span>
        </span>
      </button>

      <h2 v-if="favoriteRouteRows.length" class="wt-label">{{ t('favoriteRoutes') }}</h2>
      <button
        v-for="row in favoriteRouteRows"
        :key="`fr-${row.route.route_id}-${row.direction}`"
        type="button"
        class="wt-row wt-fish"
        :class="{ 'is-busy': openingRouteId === row.route.route_id }"
        @click="openRoute(row.route, row.direction)"
      >
        <span class="wt-badge" :style="{ backgroundColor: row.route.route_color }">{{ row.route.route_short_name }}</span>
        <span class="wt-row-text">
          <span class="wt-row-title">→ {{ row.destination }}</span>
        </span>
      </button>
    </template>
    <p v-else class="wt-note">{{ t('noFavorites') }}</p>

    <h2 class="wt-label">{{ t('planNearbyStops') }}</h2>
    <p v-if="locationNote" class="wt-note">{{ locationNote }}</p>
    <button
      v-for="item in nearby"
      :key="`nb-${item.stop.stop_id}`"
      type="button"
      class="wt-row wt-fish"
      @click="openStop(item.stop)"
    >
      <span class="wt-stop-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24" fill="currentColor">
          <path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
        </svg>
      </span>
      <span class="wt-row-text">
        <span class="wt-row-title">{{ item.stop.stop_name }}</span>
        <span class="wt-row-sub">{{ item.dist }}</span>
      </span>
    </button>

    <button type="button" class="wt-row wt-row-action wt-fish" @click="router.push({query: {map: '1'}})">
      {{ t('watchMap') }}
    </button>
    <button type="button" class="wt-row wt-row-action wt-fish" @click="settings.setSimplified(false)">
      {{ t('simplifiedTurnOff') }}
    </button>

    <p class="wt-credits">© OpenStreetMap, CARTO · tranzy.ai · CTP Cluj-Napoca</p>
  </div>
</template>

<script setup lang="ts">
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {type FavoriteRoute, useFavoritesStore} from '@/stores/favorites.ts'
import {useRouteStore} from '@/stores/route.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type Route, type Stop} from '@/types/tranzy.ts'

const {t} = useI18n()
const router = useRouter()
const settings = useSettingsStore()
const routeStore = useRouteStore()
const favoritesStore = useFavoritesStore()
const {favoriteRoutes, favoriteStopIds} = storeToRefs(favoritesStore)
const {stops} = useStopsApi()
const {routes} = useRoutesApi()
const {fetchShapeInfo} = useRouteShapeInfoApi()

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

    <div class="wt-gap" aria-hidden="true"></div>
    <button type="button" class="wt-row wt-fish" @click="router.push({query: {browse: 'routes'}})">
      <span class="wt-browse-icon" aria-hidden="true">
        <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
        </svg>
      </span>
      <span class="wt-row-text">
        <span class="wt-row-title">{{ t('allRoutes') }}</span>
      </span>
    </button>
    <button type="button" class="wt-row wt-fish" @click="router.push({query: {browse: 'stops'}})">
      <span class="wt-stop-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24" fill="currentColor">
          <path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
        </svg>
      </span>
      <span class="wt-row-text">
        <span class="wt-row-title">{{ t('allStops') }}</span>
      </span>
    </button>
    <button type="button" class="wt-row wt-fish" @click="router.push({query: {pair: '1'}})">
      <span class="wt-browse-icon" aria-hidden="true">
        <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="3" y="3" width="7" height="7" rx="1"/>
          <rect x="14" y="3" width="7" height="7" rx="1"/>
          <rect x="3" y="14" width="7" height="7" rx="1"/>
          <path d="M14 14h3v3h-3zM20 14v.01M14 20v.01M17 20h4v-3"/>
        </svg>
      </span>
      <span class="wt-row-text">
        <span class="wt-row-title">{{ t('watchCopyFavorites') }}</span>
      </span>
    </button>

    <button type="button" class="wt-row wt-row-action wt-fish" @click="settings.setSimplified(false)">
      {{ t('simplifiedTurnOff') }}
    </button>

    <p class="wt-credits">© OpenStreetMap, CARTO · tranzy.ai · CTP Cluj-Napoca</p>
  </div>
</template>

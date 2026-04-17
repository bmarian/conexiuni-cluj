<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useRouteStore} from '@/stores/route.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {OUTGOING_SUFFIX, type Route, type Stop} from '@/types/tranzy.ts'

const {t} = useI18n()
const router = useRouter()
const favoritesStore = useFavoritesStore()
const routeStore = useRouteStore()
const {favoriteRouteIds, favoriteStopIds} = storeToRefs(favoritesStore)

const {routes, isLoading: routesLoading, fetchRoutes} = useRoutesApi()
const {stops, fetchStops} = useStopsApi()
const {fetchShapeInfo} = useRouteShapeInfoApi()

const search = ref('')
const navigatingRouteId = ref<number | null>(null)

onMounted(() => {
  void fetchRoutes()
  void fetchStops()
})

function formatGtfsColor(c?: string) {
  if (!c) return '#3b82f6'
  return c.startsWith('#') ? c : `#${c}`
}

const routesById = computed(() => {
  const map = new Map<number, Route>()
  for (const r of routes.value) map.set(r.route_id, r)
  return map
})

const stopsById = computed(() => {
  const map = new Map<number, Stop>()
  for (const s of stops.value) map.set(s.stop_id, s)
  return map
})

const favoriteRoutes = computed<Route[]>(() => {
  return favoriteRouteIds.value
    .map((id) => routesById.value.get(id))
    .filter((r): r is Route => !!r)
})

const favoriteStops = computed<Stop[]>(() => {
  return favoriteStopIds.value
    .map((id) => stopsById.value.get(id))
    .filter((s): s is Stop => !!s)
})

const hasFavorites = computed(() => favoriteRouteIds.value.length > 0 || favoriteStopIds.value.length > 0)

const sortedRoutes = computed<Route[]>(() => {
  return [...routes.value].sort((a, b) =>
    a.route_short_name.localeCompare(b.route_short_name, undefined, {numeric: true}),
  )
})

const filteredRoutes = computed<Route[]>(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return sortedRoutes.value
  return sortedRoutes.value.filter(
    (r) =>
      r.route_short_name.toLowerCase().includes(q) ||
      r.route_long_name.toLowerCase().includes(q),
  )
})

async function navigateToRoute(route: Route) {
  if (navigatingRouteId.value === route.route_id) return
  navigatingRouteId.value = route.route_id
  try {
    const shapeInfo = await fetchShapeInfo(route)
    const tripId = `${route.route_id}${OUTGOING_SUFFIX}`
    routeStore.setSelectedRoute(shapeInfo, tripId, '', '')
    await router.push({name: 'route', params: {routeId: String(route.route_id), direction: '0'}})
  } catch (e) {
    console.error('Failed to load route:', e)
  } finally {
    navigatingRouteId.value = null
  }
}

function navigateToStop(stop: Stop) {
  void router.push({name: 'stop', params: {stopId: String(stop.stop_id)}})
}

function removeFavoriteRoute(route: Route, ev: Event) {
  ev.stopPropagation()
  favoritesStore.toggleRouteFavorite(route.route_id)
}

function removeFavoriteStop(stop: Stop, ev: Event) {
  ev.stopPropagation()
  favoritesStore.toggleStopFavorite(stop.stop_id)
}
</script>

<template>
  <div class="home-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-7">

    <!-- ─── Favorites ─── -->
    <section v-if="hasFavorites" class="flex flex-col gap-5">
      <h2 class="section-label">
        <svg class="w-3.5 h-3.5 text-rose-500 shrink-0" fill="currentColor" viewBox="0 0 24 24">
          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
        </svg>
        {{ t('favorites') }}
      </h2>

      <!-- Favorite routes -->
      <div v-if="favoriteRoutes.length" class="flex flex-col gap-2">
        <h3 class="sub-label">{{ t('favoriteRoutes') }}</h3>
        <div class="flex flex-wrap gap-2">
          <div
            v-for="route in favoriteRoutes"
            :key="route.route_id"
            @click="navigateToRoute(route)"
            @keydown.enter.space.prevent="navigateToRoute(route)"
            role="button"
            tabindex="0"
            class="fav-route-chip group"
            :class="{ 'opacity-60 pointer-events-none': navigatingRouteId === route.route_id }"
          >
            <span
              class="flex items-center justify-center shrink-0 w-10 h-7 rounded-md text-xs font-black text-white shadow-sm"
              :style="{ backgroundColor: formatGtfsColor(route.route_color) }"
            >{{ route.route_short_name }}</span>
            <span class="text-xs font-semibold text-slate-700 dark:text-slate-200 truncate max-w-[10rem]">
              {{ route.route_long_name }}
            </span>
            <button
              type="button"
              class="fav-remove"
              :title="t('removeFromFavorites')"
              :aria-label="t('removeFromFavorites')"
              @click="removeFavoriteRoute(route, $event)"
            >
              <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
          </div>
        </div>
      </div>

      <!-- Favorite stops -->
      <div v-if="favoriteStops.length" class="flex flex-col gap-2">
        <h3 class="sub-label">{{ t('favoriteStops') }}</h3>
        <div class="flex flex-col divide-y divide-slate-100 dark:divide-slate-800/60">
          <div
            v-for="stop in favoriteStops"
            :key="stop.stop_id"
            @click="navigateToStop(stop)"
            class="fav-stop-row group"
          >
            <div class="w-7 h-7 shrink-0 rounded-full bg-emerald-100 dark:bg-emerald-500/15 flex items-center justify-center">
              <svg class="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
              </svg>
            </div>
            <span class="flex-1 text-sm font-medium text-slate-700 dark:text-slate-200 group-hover:text-slate-900 dark:group-hover:text-white truncate">
              {{ stop.stop_name }}
            </span>
            <button
              type="button"
              class="fav-stop-remove"
              :title="t('removeFromFavorites')"
              :aria-label="t('removeFromFavorites')"
              @click="removeFavoriteStop(stop, $event)"
            >
              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
            <svg class="w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </div>
        </div>
      </div>
    </section>

    <!-- ─── All routes ─── -->
    <section class="flex flex-col gap-3 pb-6">
      <h2 class="section-label">
        <svg class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
        </svg>
        {{ t('allRoutes') }}
      </h2>

      <!-- Search -->
      <div class="search-wrap">
        <svg class="w-4 h-4 text-slate-400 dark:text-slate-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-4.35-4.35M11 19a8 8 0 100-16 8 8 0 000 16z"/>
        </svg>
        <input
          v-model="search"
          type="search"
          :placeholder="t('searchRoutes')"
          class="search-input"
        />
      </div>

      <!-- Empty hint when no favorites and search empty -->
      <p v-if="!hasFavorites && !search" class="text-xs text-slate-400 dark:text-slate-500 leading-relaxed -mt-1 mb-1">
        {{ t('noFavorites') }}
      </p>

      <!-- Loading -->
      <div v-if="routesLoading && !routes.length" class="flex flex-col gap-1 animate-pulse">
        <div v-for="i in 8" :key="i" class="flex items-center gap-3 py-2.5">
          <div class="w-10 h-7 rounded-md bg-slate-200 dark:bg-slate-800 shrink-0"></div>
          <div class="h-3.5 flex-1 bg-slate-200 dark:bg-slate-800 rounded"></div>
        </div>
      </div>

      <!-- Results -->
      <div v-else-if="filteredRoutes.length" class="flex flex-col divide-y divide-slate-100 dark:divide-slate-800/60">
        <div
          v-for="route in filteredRoutes"
          :key="route.route_id"
          @click="navigateToRoute(route)"
          class="all-route-row group"
          :class="{ 'opacity-60 pointer-events-none': navigatingRouteId === route.route_id }"
        >
          <div
            class="flex items-center justify-center shrink-0 w-10 h-7 rounded-md text-xs font-black text-white shadow-sm opacity-90 group-hover:opacity-100 transition-opacity"
            :style="{ backgroundColor: formatGtfsColor(route.route_color) }"
          >{{ route.route_short_name }}</div>

          <span class="flex-1 text-sm font-medium text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
            {{ route.route_long_name }}
          </span>

          <svg
            v-if="navigatingRouteId === route.route_id"
            class="w-3.5 h-3.5 text-slate-400 shrink-0 animate-spin"
            fill="none" viewBox="0 0 24 24"
          >
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"/>
          </svg>
          <svg
            v-else
            class="w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
          </svg>
        </div>
      </div>

      <p v-else class="text-sm text-slate-400 dark:text-slate-500 py-4 text-center">
        {{ t('noResults') }}
      </p>
    </section>

  </div>
</template>

<style scoped>
.home-view-container {
  padding: 1.25rem 1.5rem 0;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

.section-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.15em;
  color: #64748b;
}

.sub-label {
  font-size: 0.6rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.13em;
  color: #94a3b8;
}

@media (prefers-color-scheme: dark) {
  .section-label { color: #94a3b8; }
  .sub-label { color: #64748b; }
}

/* ─── Favorite route chips ─── */
.fav-route-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.375rem 0.5rem 0.375rem 0.375rem;
  border-radius: 0.75rem;
  border: 1px solid #f1f5f9;
  background: #f8fafc;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
  text-align: left;
}
.fav-route-chip:hover {
  background: white;
  border-color: #e2e8f0;
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}
.fav-route-chip:disabled { opacity: 0.5; cursor: wait; }

@media (prefers-color-scheme: dark) {
  .fav-route-chip {
    border-color: rgb(51 65 85 / 0.5);
    background: rgb(30 41 59 / 0.6);
  }
  .fav-route-chip:hover {
    background: rgb(30 41 59 / 0.9);
    border-color: rgb(51 65 85 / 0.8);
  }
}

.fav-remove {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  border-radius: 9999px;
  color: #94a3b8;
  background: transparent;
  transition: background 0.15s, color 0.15s;
}
.fav-remove:hover {
  background: #fee2e2;
  color: #dc2626;
}
@media (prefers-color-scheme: dark) {
  .fav-remove { color: #64748b; }
  .fav-remove:hover {
    background: rgb(220 38 38 / 0.15);
    color: #f87171;
  }
}

/* ─── Favorite stop rows ─── */
.fav-stop-row {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.625rem 0.25rem;
  cursor: pointer;
  transition: background 0.15s;
  border-radius: 0.5rem;
  margin: 0 -0.25rem;
}
.fav-stop-row:hover { background: #f8fafc; }
@media (prefers-color-scheme: dark) {
  .fav-stop-row:hover { background: rgb(30 41 59 / 0.5); }
}

.fav-stop-remove {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 9999px;
  color: #94a3b8;
  background: transparent;
  transition: background 0.15s, color 0.15s;
  opacity: 0;
}
.fav-stop-row:hover .fav-stop-remove { opacity: 1; }
.fav-stop-remove:hover {
  background: #fee2e2;
  color: #dc2626;
}
@media (prefers-color-scheme: dark) {
  .fav-stop-remove { color: #64748b; }
  .fav-stop-remove:hover {
    background: rgb(220 38 38 / 0.15);
    color: #f87171;
  }
}
/* Always-visible remove button on touch devices */
@media (hover: none) {
  .fav-stop-remove { opacity: 1; }
}

/* ─── Search input ─── */
.search-wrap {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.55rem 0.75rem;
  border-radius: 0.75rem;
  border: 1.5px solid #e2e8f0;
  background: #f8fafc;
  transition: border-color 0.15s, background 0.15s;
}
.search-wrap:focus-within {
  border-color: #94a3b8;
  background: white;
}
.search-input {
  flex: 1;
  background: transparent;
  border: 0;
  outline: 0;
  font-size: 0.875rem;
  color: #0f172a;
  font-weight: 500;
}
.search-input::placeholder {
  color: #94a3b8;
  font-weight: 400;
}
.search-input::-webkit-search-cancel-button {
  -webkit-appearance: none;
  appearance: none;
}

@media (prefers-color-scheme: dark) {
  .search-wrap {
    background: rgb(30 41 59 / 0.6);
    border-color: rgb(51 65 85 / 0.7);
  }
  .search-wrap:focus-within {
    background: rgb(30 41 59 / 0.9);
    border-color: #475569;
  }
  .search-input { color: #f1f5f9; }
  .search-input::placeholder { color: #64748b; }
}

/* ─── All-routes row (matches StopView's .all-route-row) ─── */
.all-route-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 0.25rem;
  cursor: pointer;
  transition: background 0.15s;
  border-radius: 0.5rem;
  margin: 0 -0.25rem;
}
.all-route-row:hover { background: #f8fafc; }
@media (prefers-color-scheme: dark) {
  .all-route-row:hover { background: rgb(30 41 59 / 0.5); }
}
</style>

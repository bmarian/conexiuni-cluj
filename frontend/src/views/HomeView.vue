<script setup lang="ts">
import {computed, onMounted, ref, watchPostEffect} from 'vue'
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import Draggable from 'vuedraggable'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useRouteStore} from '@/stores/route.ts'
import {useMapStore} from '@/stores/map.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {OUTGOING_SUFFIX, type Route, type Stop} from '@/types/tranzy.ts'
import MetroEasterEgg from '@/components/MetroEasterEgg.vue'

const {t} = useI18n()
const router = useRouter()
const favoritesStore = useFavoritesStore()
const routeStore = useRouteStore()
const mapStore = useMapStore()
const settings = useSettingsStore()
const {favoriteRouteIds, favoriteStopIds, isHydrated} = storeToRefs(favoritesStore)

const {routes, isLoading: routesLoading, fetchRoutes} = useRoutesApi()
const {stops, fetchStops} = useStopsApi()
const {fetchShapeInfo} = useRouteShapeInfoApi()

const search = ref('')
const navigatingRouteId = ref<number | null>(null)

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

watchPostEffect(() => {
  mapStore.setHighlightedStops(
    favoriteStopIds.value.map(id => ({stopId: String(id), color: 'red' as const}))
  )
})

onMounted(() => {
  void fetchRoutes()
  void fetchStops()
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

const routeFavoritesModel = computed<number[]>({
  get: () => favoriteRouteIds.value,
  set: (newIds) => favoritesStore.reorderRouteIds([...newIds]),
})

const stopFavoritesModel = computed<number[]>({
  get: () => favoriteStopIds.value,
  set: (newIds) => favoritesStore.reorderStopIds([...newIds]),
})
</script>

<template>
  <div
    class="home-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-7">
    <div v-if="navigatingRouteId" class="nav-loading-bar" aria-hidden="true"></div>

    <section v-if="isHydrated && hasFavorites" class="flex flex-col gap-5">
      <h2 class="section-label">
        <span v-if="settings.traditionalActive" class="emoji-icon" aria-hidden="true">❤️</span>
        <svg v-else class="w-3.5 h-3.5 text-rose-500 shrink-0" fill="currentColor"
             viewBox="0 0 24 24">
          <path
            d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
        </svg>
        {{ t('favorites') }}
      </h2>

      <div v-if="favoriteRoutes.length" class="flex flex-col gap-2">
        <h3 class="sub-label">{{ t('favoriteRoutes') }}</h3>
        <Draggable
          v-model="routeFavoritesModel"
          :item-key="(id: number) => id"
          class="favorite-routes-grid"
          tag="div"
          handle=".drag-handle"
          ghost-class="drag-ghost"
          chosen-class="drag-chosen"
          :animation="180"
        >
          <template #item="{element: routeId}">
            <div
              v-if="routesById.get(routeId)"
              class="fav-route-chip group"
              :class="{ 'opacity-60 pointer-events-none': navigatingRouteId === routeId }"
              :style="{ '--chip-color': routesById.get(routeId)?.route_color }"
              @click="navigateToRoute(routesById.get(routeId)!)"
              @keydown.enter.space.prevent="navigateToRoute(routesById.get(routeId)!)"
              role="button"
              tabindex="0"
            >
              <svg
                class="drag-handle w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 cursor-grab"
                viewBox="0 0 24 24" fill="currentColor">
                <circle cx="9" cy="5" r="1.5"/>
                <circle cx="15" cy="5" r="1.5"/>
                <circle cx="9" cy="12" r="1.5"/>
                <circle cx="15" cy="12" r="1.5"/>
                <circle cx="9" cy="19" r="1.5"/>
                <circle cx="15" cy="19" r="1.5"/>
              </svg>
              <span
                class="fav-route-badge"
                :style="{ backgroundColor: routesById.get(routeId)?.route_color }"
                :title="routesById.get(routeId)?.route_long_name"
              >{{ routesById.get(routeId)?.route_short_name }}</span>
              <span
                class="fav-route-name"
                :title="routesById.get(routeId)?.route_long_name"
              >{{ routesById.get(routeId)?.route_long_name }}</span>
              <button
                type="button"
                class="fav-remove"
                :title="t('removeFromFavorites')"
                :aria-label="t('removeFromFavorites')"
                @click.stop="removeFavoriteRoute(routesById.get(routeId)!, $event)"
              >
                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"
                     stroke-width="2.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
          </template>
        </Draggable>
      </div>

      <div v-if="favoriteStops.length" class="flex flex-col gap-2">
        <h3 class="sub-label">{{ t('favoriteStops') }}</h3>
        <Draggable
          v-model="stopFavoritesModel"
          :item-key="(id: number) => id"
          class="flex flex-col divide-y divide-slate-100 dark:divide-slate-800/60"
          tag="div"
          handle=".drag-handle"
          ghost-class="drag-ghost"
          chosen-class="drag-chosen"
          :animation="180"
        >
          <template #item="{element: stopId}">
            <div
              v-if="stopsById.get(stopId)"
              class="fav-stop-row group"
              @click="navigateToStop(stopsById.get(stopId)!)"
            >
              <svg
                class="drag-handle w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 cursor-grab"
                viewBox="0 0 24 24" fill="currentColor">
                <circle cx="9" cy="5" r="1.5"/>
                <circle cx="15" cy="5" r="1.5"/>
                <circle cx="9" cy="12" r="1.5"/>
                <circle cx="15" cy="12" r="1.5"/>
                <circle cx="9" cy="19" r="1.5"/>
                <circle cx="15" cy="19" r="1.5"/>
              </svg>
              <div
                class="w-7 h-7 shrink-0 rounded-full bg-emerald-100 dark:bg-emerald-500/15 flex items-center justify-center">
                <span v-if="settings.traditionalActive" class="emoji-icon-md"
                      aria-hidden="true">📍</span>
                <svg v-else class="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400"
                     viewBox="0 0 24 24" fill="currentColor">
                  <path
                    d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
                </svg>
              </div>
              <span
                class="flex-1 text-sm font-medium text-slate-700 dark:text-slate-200 group-hover:text-slate-900 dark:group-hover:text-white truncate">
                {{ stopsById.get(stopId)?.stop_name }}
              </span>
              <button
                type="button"
                class="fav-stop-remove"
                :title="t('removeFromFavorites')"
                :aria-label="t('removeFromFavorites')"
                @click.stop="removeFavoriteStop(stopsById.get(stopId)!, $event)"
              >
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"
                     stroke-width="2.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
              <svg
                class="w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors"
                fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
              </svg>
            </div>
          </template>
        </Draggable>
      </div>
    </section>

    <section class="flex flex-col gap-3 pb-6">
      <h2 class="section-label">
        <span v-if="settings.traditionalActive" class="emoji-icon" aria-hidden="true">🗺️</span>
        <svg v-else class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500 shrink-0" fill="none"
             viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round"
                d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
        </svg>
        {{ t('allRoutes') }}
      </h2>

      <div class="search-wrap">
        <span v-if="settings.traditionalActive" class="emoji-icon" aria-hidden="true">🔍</span>
        <svg v-else class="w-4 h-4 text-slate-400 dark:text-slate-500 shrink-0" fill="none"
             viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round"
                d="M21 21l-4.35-4.35M11 19a8 8 0 100-16 8 8 0 000 16z"/>
        </svg>
        <input
          v-model="search"
          type="search"
          :placeholder="t('searchRoutes')"
          class="search-input"
        />
      </div>

      <p v-if="isHydrated && !hasFavorites && !search"
         class="text-xs text-slate-400 dark:text-slate-500 leading-relaxed -mt-1 mb-1">
        {{ t('noFavorites') }}
      </p>

      <div v-if="routesLoading && !routes.length" class="flex flex-col gap-1 animate-pulse">
        <div v-for="i in 8" :key="i" class="flex items-center gap-3 py-2.5">
          <div class="w-10 h-7 rounded-md bg-slate-200 dark:bg-slate-800 shrink-0"></div>
          <div class="h-3.5 flex-1 bg-slate-200 dark:bg-slate-800 rounded"></div>
        </div>
      </div>

      <div v-else-if="filteredRoutes.length"
           class="flex flex-col divide-y divide-slate-100 dark:divide-slate-800/60">
        <div
          v-for="route in filteredRoutes"
          :key="route.route_id"
          @click="navigateToRoute(route)"
          class="all-route-row group"
          :class="{ 'opacity-60 pointer-events-none': navigatingRouteId === route.route_id }"
        >
          <div
            class="flex items-center justify-center shrink-0 w-10 h-7 rounded-md text-xs font-black text-white shadow-sm opacity-90 group-hover:opacity-100 transition-opacity"
            :style="{ backgroundColor: route.route_color }"
          >{{ route.route_short_name }}
          </div>

          <span
            class="flex-1 text-sm font-medium text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
            {{ route.route_long_name }}
          </span>

          <svg
            v-if="navigatingRouteId === route.route_id"
            class="w-3.5 h-3.5 text-slate-400 shrink-0 animate-spin"
            fill="none" viewBox="0 0 24 24"
          >
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor"
                    stroke-width="4"/>
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

      <MetroEasterEgg v-else :search="search"/>
    </section>

  </div>
</template>

<style scoped>
.home-view-container {
  position: relative;
  padding: 1.25rem 1.5rem 0;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

.nav-loading-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: linear-gradient(90deg, #10b981, #06b6d4, #10b981);
  background-size: 200% 100%;
  animation: nav-sweep 1.2s linear infinite;
}

@keyframes nav-sweep {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}

.section-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
}

.sub-label {
  font-size: 0.7rem;
  font-weight: 600;
  color: #94a3b8;
}

.favorite-routes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(7rem, 1fr));
  gap: 0.5rem;
}

.fav-route-chip {
  container-type: inline-size;
  container-name: fav-chip;
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
  min-height: 2.5rem;
  padding: 0.375rem 0.5rem;
  border-radius: 0.75rem;
  border: 1px solid #f1f5f9;
  background: #f8fafc;
  cursor: grab;
  transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
  text-align: left;
  user-select: none;
}

.fav-route-chip::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 0.75rem;
  background: var(--chip-color, transparent);
  opacity: 0.09;
  pointer-events: none;
  transition: opacity 0.15s;
}

.fav-route-chip:active {
  cursor: grabbing;
}

.fav-route-chip:hover {
  background: white;
  border-color: #e2e8f0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.fav-route-chip:hover::before {
  opacity: 0.13;
}

.fav-route-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 1.75rem;
  flex-shrink: 0;
  border-radius: 0.5rem;
  font-size: 0.75rem;
  font-weight: 900;
  color: #fff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.25);
}

.fav-route-name {
  flex: 1;
  min-width: 0;
  font-size: 0.75rem;
  font-weight: 600;
  color: #475569;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@container fav-chip (max-width: 9rem) {
  .fav-route-name {
    display: none;
  }

  .fav-route-badge {
    margin-right: auto;
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
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.fav-remove:hover {
  background: #fee2e2;
  color: #dc2626;
}

.fav-stop-row {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.625rem 0.25rem;
  cursor: pointer;
  transition: background 0.15s;
  border-radius: 0.5rem;
  margin: 0 -0.25rem;
  user-select: none;
}

.fav-stop-row:hover {
  background: #f8fafc;
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
  cursor: pointer;
}

.fav-stop-row:hover .fav-stop-remove {
  opacity: 1;
}

.fav-stop-remove:hover {
  background: #fee2e2;
  color: #dc2626;
}

@media (hover: none) {
  .fav-stop-remove {
    opacity: 1;
  }
}

.drag-handle {
  touch-action: none;
}

.drag-ghost {
  opacity: 0.55;
}

.drag-chosen {
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.14);
}

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

.all-route-row:hover {
  background: #f8fafc;
}
</style>

<script setup lang="ts">
import {computed, onUnmounted, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {useMapStore} from '@/stores/map.ts'
import {useRouteStore} from '@/stores/route.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useUserStore} from '@/stores/user.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {OUTGOING_SUFFIX, type Route, type Stop} from '@/types/tranzy.ts'
import {formatMeters, haversineMeters, sortByDistance} from '@/utils/geo.ts'
import MetroEasterEgg from '@/components/MetroEasterEgg.vue'

interface GeoResult {
  place_id: number
  display_name: string
  lat: string
  lon: string
}

interface EnrichedGeoResult extends GeoResult {
  parsedLat: number
  parsedLon: number
  dist: string | null
  main: string
  sub: string
}

interface StopWithDist {
  stop: Stop
  dist: string | null
}

const props = defineProps<{
  routes: Route[]
  stops: Stop[]
}>()

const emit = defineEmits<{
  'update:active': [value: boolean]
}>()

const {t, locale} = useI18n()
const router = useRouter()
const mapStore = useMapStore()
const routeStore = useRouteStore()
const settings = useSettingsStore()
const userStore = useUserStore()
const {fetchShapeInfo} = useRouteShapeInfoApi()

const search = ref('')
const navigatingRouteId = ref<number | null>(null)
const geoResults = ref<GeoResult[]>([])
const geoLoading = ref(false)

let geoDebounceTimer: ReturnType<typeof setTimeout> | null = null

const isSearchMode = computed(() => search.value.trim().length > 0)

watch(isSearchMode, (val) => emit('update:active', val), {immediate: true})

function norm(s: string): string {
  return s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()
}

const searchRouteResults = computed<Route[]>(() => {
  const q = norm(search.value.trim())
  if (!q) return []
  return props.routes
    .filter(r =>
      norm(r.route_short_name).includes(q) ||
      norm(r.route_long_name).includes(q)
    )
    .slice(0, 3)
})

const metroEggVisible = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (q.length < 2) return false
  return 'metrou'.startsWith(q) || q === 'm1'
})

const searchStopResults = computed<Stop[]>(() => {
  const q = norm(search.value.trim())
  if (!q) return []
  const filtered = props.stops.filter(s => norm(s.stop_name).includes(q))
  const loc = userStore.userLocation
  const sorted = loc
    ? sortByDistance(filtered, loc.latitude, loc.longitude, s => s.stop_lat, s => s.stop_lon)
    : filtered
  return sorted.slice(0, 3)
})

async function fetchGeo(q: string) {
  geoLoading.value = true
  try {
    const params = new URLSearchParams({
      q,
      format: 'json',
      countrycodes: 'ro',
      viewbox: '22.75,47.50,24.27,46.38',
      bounded: '1',
      limit: '3',
      'accept-language': locale.value,
    })
    const resp = await fetch(`https://nominatim.openstreetmap.org/search?${params}`)
    const results: GeoResult[] = resp.ok ? await resp.json() : []
    const loc = userStore.userLocation
    geoResults.value = loc && results.length > 1
      ? sortByDistance(results, loc.latitude, loc.longitude, r => parseFloat(r.lat), r => parseFloat(r.lon))
      : results
  } catch {
    geoResults.value = []
  } finally {
    geoLoading.value = false
  }
}

watch(search, (q) => {
  if (geoDebounceTimer) clearTimeout(geoDebounceTimer)
  geoResults.value = []
  const trimmed = q.trim()
  if (trimmed.length < 3) {
    geoResults.value = []
    geoLoading.value = false
    return
  }
  geoLoading.value = true
  geoDebounceTimer = setTimeout(() => fetchGeo(trimmed), 350)
})

onUnmounted(() => {
  if (geoDebounceTimer) clearTimeout(geoDebounceTimer)
})

function distanceFrom(lat: number, lon: number): string | null {
  const loc = userStore.userLocation
  if (!loc) return null
  return formatMeters(haversineMeters(loc.latitude, loc.longitude, lat, lon))
}

const enrichedGeoResults = computed<EnrichedGeoResult[]>(() =>
  geoResults.value.map(r => {
    const parsedLat = parseFloat(r.lat)
    const parsedLon = parseFloat(r.lon)
    const parts = r.display_name.split(',').map(p => p.trim())
    return {
      ...r,
      parsedLat,
      parsedLon,
      dist: distanceFrom(parsedLat, parsedLon),
      main: parts[0] ?? r.display_name,
      sub: parts.slice(1, 3).join(', '),
    }
  })
)

const stopResultsWithDist = computed<StopWithDist[]>(() =>
  searchStopResults.value.map(stop => ({
    stop,
    dist: distanceFrom(stop.stop_lat, stop.stop_lon),
  }))
)

function navigateToPlan(result: GeoResult) {
  search.value = ''

  void router.push({
    name: 'plan',
    query: {
      lat: result.lat,
      lon: result.lon,
      name: result.display_name,
    },
  })
}

async function navigateToRoute(route: Route) {
  if (navigatingRouteId.value === route.route_id) return
  navigatingRouteId.value = route.route_id
  mapStore.clearPinnedLocation()
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
  mapStore.clearPinnedLocation()
  mapStore.setFlyToLocation(stop.stop_lat, stop.stop_lon)
  void router.push({name: 'stop', params: {stopId: String(stop.stop_id)}})
}

</script>

<template>
  <div class="universal-search">
    <!-- Absolute-positioned into home-view-container which has position:relative. -->
    <div v-if="navigatingRouteId" class="nav-loading-bar" aria-hidden="true"></div>

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
        :placeholder="t('searchPlaceholder')"
        class="search-input"
        autocomplete="off"
        @keydown.enter="($event.target as HTMLInputElement).blur()"
      />
      <button
        v-if="search"
        type="button"
        class="search-clear"
        aria-label="Clear search"
        @click="search = ''; mapStore.clearPinnedLocation()"
      >
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"
             stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>

    <div v-if="isSearchMode" class="search-results">

      <div v-if="geoLoading" class="geo-loading" aria-label="Loading places">
        <span class="geo-loading-dot"></span>
        <span class="geo-loading-dot"></span>
        <span class="geo-loading-dot"></span>
      </div>

      <div v-if="enrichedGeoResults.length" class="result-group">
        <h3 class="sub-label">{{ t('searchResultsPlaces') }}</h3>
        <div class="divide-y divide-slate-100 dark:divide-slate-800/60">
          <div
            v-for="result in enrichedGeoResults"
            :key="result.place_id"
            class="geo-result-row group"
            role="button"
            tabindex="0"
            @click="navigateToPlan(result)"
            @keydown.enter.space.prevent="navigateToPlan(result)"
          >
            <div
              class="w-8 h-8 shrink-0 rounded-full bg-sky-100 dark:bg-sky-500/15 flex items-center justify-center">
              <span v-if="settings.traditionalActive" class="emoji-icon-md"
                    aria-hidden="true">📍</span>
              <svg v-else class="w-4 h-4 text-sky-500 dark:text-sky-400" viewBox="0 0 24 24"
                   fill="currentColor">
                <path
                  d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
              </svg>
            </div>
            <div class="flex flex-col flex-1 min-w-0 gap-0.5">
              <span
                class="text-sm font-medium text-slate-700 dark:text-slate-200 group-hover:text-slate-900 dark:group-hover:text-white truncate">
                {{ result.main }}
              </span>
              <span
                v-if="result.sub"
                class="text-xs text-slate-400 dark:text-slate-500 truncate">
                {{ result.sub }}
              </span>
            </div>
            <span v-if="result.dist" class="dist-badge">{{ result.dist }}</span>
            <svg
              class="w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors"
              fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </div>
        </div>
      </div>

      <div v-if="searchRouteResults.length" class="result-group">
        <h3 class="sub-label">{{ t('searchResultsRoutes') }}</h3>
        <div class="divide-y divide-slate-100 dark:divide-slate-800/60">
          <div
            v-for="route in searchRouteResults"
            :key="route.route_id"
            class="all-route-row group"
            :class="{'opacity-60 pointer-events-none': navigatingRouteId === route.route_id}"
            @click="navigateToRoute(route)"
          >
            <div
              class="flex items-center justify-center shrink-0 w-10 h-7 rounded-md text-xs font-black text-white shadow-sm opacity-90 group-hover:opacity-100 transition-opacity"
              :style="{backgroundColor: route.route_color}"
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
              <path class="opacity-75" fill="currentColor"
                    d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"/>
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
      </div>

      <div v-if="stopResultsWithDist.length" class="result-group">
        <h3 class="sub-label">{{ t('searchResultsStops') }}</h3>
        <div class="divide-y divide-slate-100 dark:divide-slate-800/60">
          <div
            v-for="entry in stopResultsWithDist"
            :key="entry.stop.stop_id"
            class="all-route-row group"
            @click="navigateToStop(entry.stop)"
          >
            <div
              class="w-8 h-8 shrink-0 rounded-full bg-emerald-100 dark:bg-emerald-500/15 flex items-center justify-center">
              <span v-if="settings.traditionalActive" class="emoji-icon-md"
                    aria-hidden="true">🚏</span>
              <svg v-else class="w-4 h-4 text-emerald-600 dark:text-emerald-400"
                   viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
              </svg>
            </div>
            <span
              class="flex-1 text-sm font-medium text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
              {{ entry.stop.stop_name }}
            </span>
            <span v-if="entry.dist" class="dist-badge">{{ entry.dist }}</span>
            <svg
              class="w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors"
              fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
            </svg>
          </div>
        </div>
      </div>

      <MetroEasterEgg :search="search"/>

      <p
        v-if="!metroEggVisible && !geoLoading && !enrichedGeoResults.length && !searchRouteResults.length && !stopResultsWithDist.length"
        class="no-results"
      >{{ t('noResults') }}</p>

    </div>
  </div>
</template>

<style scoped>
.universal-search {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
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
  font-size: 1rem;
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

.search-clear {
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
  flex-shrink: 0;
}

.search-clear:hover {
  background: #f1f5f9;
  color: #64748b;
}

.search-results {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding-bottom: 1.5rem;
}

.result-group {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.sub-label {
  font-size: 0.7rem;
  font-weight: 600;
  color: #94a3b8;
}

.geo-result-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 0.25rem;
  cursor: pointer;
  transition: background 0.15s;
  border-radius: 0.5rem;
  margin: 0 -0.25rem;
}

.geo-result-row:hover {
  background: #f8fafc;
}

.geo-result-row:focus-visible {
  outline: 2px solid #94a3b8;
  outline-offset: 2px;
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

.no-results {
  font-size: 0.875rem;
  color: #94a3b8;
  padding: 1rem 0;
  text-align: center;
}

.dist-badge {
  flex-shrink: 0;
  font-size: 0.65rem;
  font-weight: 600;
  color: #94a3b8;
  background: #f1f5f9;
  border-radius: 0.25rem;
  padding: 0.1rem 0.35rem;
  white-space: nowrap;
}

.geo-loading {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.5rem 0.25rem;
}

.geo-loading-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #94a3b8;
  animation: geo-pulse 1.1s ease-in-out infinite;
}

.geo-loading-dot:nth-child(2) { animation-delay: 0.18s; }
.geo-loading-dot:nth-child(3) { animation-delay: 0.36s; }

@keyframes geo-pulse {
  0%, 80%, 100% { opacity: 0.25; transform: scale(0.8); }
  40% { opacity: 1; transform: scale(1); }
}
</style>

<style>
/* Unscoped so it covers HomeView's full-list rows too. */
html.dark .geo-result-row:hover,
html.dark .all-route-row:hover,
html.dark .fav-stop-row:hover {
  background: #1e293b;
}
</style>

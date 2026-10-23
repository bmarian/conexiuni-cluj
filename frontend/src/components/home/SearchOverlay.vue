<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { storeToRefs } from "pinia";
import { useMapStore } from "@/stores/map.ts";
import { useUserStore } from "@/stores/user.ts";
import { useFavoritesStore, type FavoritePlan } from "@/stores/favorites.ts";
import { useRoutesApi } from "@/composables/useRoutesApi.ts";
import { useStopsApi } from "@/composables/useStopsApi.ts";
import { useOnline } from "@/composables/useOnline.ts";
import { useSearch } from "@/composables/useSearch.ts";
import { useOpenRoute } from "@/composables/useOpenRoute.ts";
import { useKbdLayer } from "@/composables/useKeyboardNav.ts";
import type { Route, Stop } from "@/types/tranzy.ts";
import { formatMeters, haversineMeters, sortByDistance } from "@/utils/geo.ts";
import { searchPlaces, type Place } from "@/utils/places.ts";
import { RESULTS_PER_KIND, stopMatchRank } from "@/utils/stopSearch.ts";
import { fold } from "@/utils/text.ts";
import { network } from "@/utils/network.ts";
import { cityCenter } from "@/utils/city.ts";
import { planQuery } from "@/utils/planQuery.ts";
import {
  ArrowBackIcon,
  CloseIcon,
  DirectionsBusIcon,
  KeyboardArrowRightIcon,
  MapIcon,
  PlaceIcon,
  PushPinIcon,
  ScheduleIcon,
} from "@/utils/icons.ts";
import SubLabel from "@/components/ui/SubLabel.vue";
import RowIcon from "@/components/ui/RowIcon.vue";
import ListRow from "@/components/ui/ListRow.vue";
import RouteRow from "@/components/rows/RouteRow.vue";
import StationRow from "@/components/rows/StationRow.vue";
import LoadMore from "@/components/LoadMore.vue";

const PLACE_DEBOUNCE_MS = 350;
const PLACE_MIN_CHARS = 3;
const RECENTS_SHOWN = 8;
const PAGE_SIZE = 30;

const { t } = useI18n();
const router = useRouter();
const mapStore = useMapStore();
const userStore = useUserStore();
const favorites = useFavoritesStore();
const { favoritePlans, recentPlans } = storeToRefs(favorites);
const { routes, fetchRoutes } = useRoutesApi();
const { stops, fetchStops } = useStopsApi();
const { isOnline } = useOnline();
const { open, query } = useSearch();
const { openingRouteId, openRoute } = useOpenRoute();

const panel = ref<HTMLElement | null>(null);
const scroller = ref<HTMLElement | null>(null);
const input = ref<HTMLInputElement | null>(null);

function close(clear: boolean) {
  open.value = false;
  if (clear) query.value = "";
}

useKbdLayer(open, { el: () => panel.value, close: () => close(true) });

watch(open, async (isOpen) => {
  if (!isOpen) return;
  void fetchRoutes();
  void fetchStops();
  await nextTick();
  input.value?.focus({ preventScroll: true });
});

const searching = computed(() => query.value.trim().length > 0);

const byName = new Intl.Collator(undefined, { numeric: true });
const sortedRoutes = computed(() =>
  [...routes.value].sort((a, b) => byName.compare(a.route_short_name, b.route_short_name)),
);
const sortedStops = computed(() => {
  const sorted = [...stops.value].sort((a, b) => byName.compare(a.stop_name, b.stop_name));
  return network.value?.oneStopPerStation(sorted) ?? sorted;
});

const routeResults = computed<Route[]>(() => {
  const q = fold(query.value.trim());
  if (!q) return [];
  return sortedRoutes.value
    .filter((r) => fold(r.route_short_name).includes(q) || fold(r.route_long_name).includes(q))
    .slice(0, RESULTS_PER_KIND);
});

const stopResults = computed<Stop[]>(() => {
  const q = fold(query.value.trim());
  if (!q) return [];
  const matches = stops.value
    .map((stop) => ({ stop, rank: stopMatchRank(fold(stop.stop_name), q) }))
    .filter((m) => m.rank >= 0);
  const loc = userStore.userLocation;
  const sorted = loc
    ? sortByDistance(
        matches,
        loc.latitude,
        loc.longitude,
        (m) => m.stop.stop_lat,
        (m) => m.stop.stop_lon,
      )
    : matches;
  const ranked = sorted.sort((a, b) => a.rank - b.rank).map((m) => m.stop);
  return (network.value?.oneStopPerStation(ranked) ?? ranked).slice(0, RESULTS_PER_KIND);
});

const places = ref<Place[]>([]);
const placesLoading = ref(false);
let placeTimer: ReturnType<typeof setTimeout> | null = null;
let placeRequest: AbortController | null = null;

async function fetchPlaces(q: string) {
  const request = new AbortController();
  placeRequest = request;
  try {
    const found = await searchPlaces(q, RESULTS_PER_KIND, request.signal);
    if (request.signal.aborted) return;
    const loc = userStore.userLocation;
    places.value =
      loc && found.length > 1
        ? sortByDistance(
            found,
            loc.latitude,
            loc.longitude,
            (r) => parseFloat(r.lat),
            (r) => parseFloat(r.lon),
          )
        : found;
  } catch {
    if (!request.signal.aborted) places.value = [];
  } finally {
    if (placeRequest === request) {
      placeRequest = null;
      placesLoading.value = false;
    }
  }
}

watch(query, (q) => {
  if (placeTimer) clearTimeout(placeTimer);
  placeRequest?.abort();
  placeRequest = null;
  places.value = [];
  const trimmed = q.trim();
  placesLoading.value = isOnline.value && trimmed.length >= PLACE_MIN_CHARS;
  if (placesLoading.value) placeTimer = setTimeout(() => fetchPlaces(trimmed), PLACE_DEBOUNCE_MS);
});

onUnmounted(() => {
  if (placeTimer) clearTimeout(placeTimer);
  placeRequest?.abort();
});

function distanceTo(lat: number, lon: number): string | null {
  const loc = userStore.userLocation;
  return loc ? formatMeters(haversineMeters(loc.latitude, loc.longitude, lat, lon)) : null;
}

const noResults = computed(
  () =>
    !placesLoading.value &&
    !places.value.length &&
    !routeResults.value.length &&
    !stopResults.value.length,
);

const recents = computed(() =>
  recentPlans.value
    .filter((p) => !favorites.isPlanFavorite(p.lat, p.lon, p.originLat, p.originLon))
    .slice(0, RECENTS_SHOWN),
);

function persistedToggle(key: string) {
  const on = ref(localStorage.getItem(key) === "true");
  watch(on, (v) => localStorage.setItem(key, String(v)));
  return on;
}

const showAllRoutes = persistedToggle("home.showAllRoutes");
const showAllStops = persistedToggle("home.showAllStops");
const routesShown = ref(PAGE_SIZE);
const stopsShown = ref(PAGE_SIZE);
watch(showAllRoutes, () => (routesShown.value = PAGE_SIZE));
watch(showAllStops, () => (stopsShown.value = PAGE_SIZE));

function go(action: () => void) {
  input.value?.blur();
  close(false);
  action();
}

function openPlace(place: Place) {
  const lat = parseFloat(place.lat);
  const lon = parseFloat(place.lon);
  go(() => {
    if (!isNaN(lat) && !isNaN(lon)) favorites.addRecentPlan({ name: place.label, lat, lon });
    void router.push({
      name: "plan",
      query: { lat: place.lat, lon: place.lon, name: place.label },
    });
  });
}

function openPlan(plan: FavoritePlan) {
  go(() => void router.push({ name: "plan", query: planQuery(plan) }));
}

function openStop(stop: Stop) {
  go(() => {
    mapStore.clearPinnedLocation();
    mapStore.setFlyToLocation(stop.stop_lat, stop.stop_lon);
    void router.push({ name: "stop", params: { stopId: String(stop.stop_id) } });
  });
}

function pickRoute(route: Route) {
  input.value?.blur();
  void openRoute(route).then(() => close(false));
}

function dropPin() {
  go(
    () =>
      void router.push({
        name: "plan",
        query: {
          lat: String(cityCenter().lat),
          lon: String(cityCenter().lon),
          name: t("droppedPin"),
        },
      }),
  );
}
</script>

<template>
  <div v-if="open" class="search-backdrop" @pointerdown="close(false)"></div>
  <Transition name="search">
    <div
      v-if="open"
      ref="panel"
      class="search-overlay"
      role="dialog"
      :aria-label="t('searchPlaceholder')"
    >
      <div class="search-bar" data-kbd-section="search" data-kbd-axis="x">
        <button
          type="button"
          class="icon-btn"
          data-kbd-item="search-back"
          :aria-label="t('back')"
          :title="t('back')"
          @click="close(true)"
        >
          <ArrowBackIcon class="m-icon" />
        </button>
        <input
          ref="input"
          v-model="query"
          type="search"
          class="search-input text-body-large"
          data-kbd-item="search"
          :placeholder="t('searchPlaceholder')"
          autocomplete="off"
          enterkeyhint="search"
          @keydown.enter="input?.blur()"
        />
        <button
          v-if="query"
          type="button"
          class="icon-btn"
          :aria-label="t('clearSearch')"
          :title="t('clearSearch')"
          @click="query = ''"
        >
          <CloseIcon class="m-icon" />
        </button>
      </div>
      <div ref="scroller" class="search-scroll">
        <template v-if="searching">
          <div v-if="placesLoading" class="progress" aria-hidden="true"><span></span></div>
          <section v-if="isOnline && places.length" data-kbd-section="search-results">
            <SubLabel :text="t('searchResultsPlaces')" class="pt-2" />
            <ListRow
              v-for="place in places"
              :key="place.id"
              :title="place.label"
              :trailing="distanceTo(parseFloat(place.lat), parseFloat(place.lon)) ?? undefined"
              chevron
              :data-kbd-item="`place-${place.id}`"
              @open="openPlace(place)"
            >
              <template #icon><RowIcon :icon="PlaceIcon" tone="tertiary" /></template>
            </ListRow>
          </section>
          <section v-if="routeResults.length" data-kbd-section="search-results">
            <SubLabel :text="t('searchResultsRoutes')" class="pt-2" />
            <RouteRow
              v-for="route in routeResults"
              :key="route.route_id"
              :name="route.route_short_name"
              :color="route.route_color"
              :title="route.route_long_name"
              :busy="openingRouteId === route.route_id"
              :data-kbd-item="`search-route-${route.route_id}`"
              @open="pickRoute(route)"
            />
          </section>
          <section v-if="stopResults.length" data-kbd-section="search-results">
            <SubLabel :text="t('searchResultsStops')" class="pt-2" />
            <StationRow
              v-for="stop in stopResults"
              :key="stop.stop_id"
              :stop="stop"
              :distance="distanceTo(stop.stop_lat, stop.stop_lon)"
              :data-kbd-item="`search-stop-${stop.stop_id}`"
              @open="openStop(stop)"
            />
          </section>
          <template v-if="noResults">
            <ListRow
              v-if="isOnline"
              :title="t('noResults')"
              :subtitle="t('dropPin')"
              data-kbd-section="search-results"
              data-kbd-item="drop-pin"
              @open="dropPin"
            >
              <template #icon><RowIcon :icon="PushPinIcon" /></template>
            </ListRow>
            <p v-else class="text-body-medium text-on-surface-variant p-4">{{ t("noResults") }}</p>
          </template>
        </template>

        <template v-else>
          <section v-if="isOnline && recents.length" data-kbd-section="recent-plans">
            <SubLabel :text="t('recentDestinations')" class="pt-2" />
            <ListRow
              v-for="plan in recents"
              :key="`recent-${plan.lat}-${plan.lon}-${plan.originLat ?? ''}`"
              :title="plan.name"
              :subtitle="plan.originName ? `${t('planFrom')}: ${plan.originName}` : undefined"
              chevron
              :data-kbd-item="`recent-${plan.lat}_${plan.lon}`"
              @open="openPlan(plan)"
            >
              <template #icon><RowIcon :icon="ScheduleIcon" /></template>
            </ListRow>
          </section>
          <section v-if="isOnline && favoritePlans.length" data-kbd-section="fav-plans">
            <SubLabel :text="t('favoritePlans')" class="pt-2" />
            <ListRow
              v-for="plan in favoritePlans"
              :key="`saved-${plan.lat}-${plan.lon}-${plan.originLat ?? ''}`"
              :title="plan.label ?? plan.name"
              chevron
              :data-kbd-item="`saved-${plan.lat}_${plan.lon}`"
              @open="openPlan(plan)"
            >
              <template #icon><RowIcon :icon="PlaceIcon" tone="tertiary" /></template>
            </ListRow>
          </section>

          <section data-kbd-section="all-routes" class="pt-3">
            <button
              type="button"
              class="collapse-row state squish"
              :aria-expanded="showAllRoutes"
              data-kbd-item="toggle-all-routes"
              @click="showAllRoutes = !showAllRoutes"
            >
              <RowIcon :icon="MapIcon" :size="32" />
              <span class="flex-1 text-left text-title-small">{{ t("allRoutes") }}</span>
              <span v-if="sortedRoutes.length" class="count">{{ sortedRoutes.length }}</span>
              <KeyboardArrowRightIcon
                class="m-icon size-[18px] text-outline chevron"
                :class="{ 'is-open': showAllRoutes }"
              />
            </button>
            <template v-if="showAllRoutes">
              <div v-if="!routes.length" class="progress" aria-hidden="true"><span></span></div>
              <RouteRow
                v-for="route in sortedRoutes.slice(0, routesShown)"
                :key="route.route_id"
                :name="route.route_short_name"
                :color="route.route_color"
                :title="route.route_long_name"
                :busy="openingRouteId === route.route_id"
                :data-kbd-item="`route-${route.route_id}`"
                @open="pickRoute(route)"
              />
              <LoadMore
                v-if="routesShown < sortedRoutes.length"
                :root="scroller"
                @more="routesShown += PAGE_SIZE"
              />
            </template>
          </section>
          <section data-kbd-section="all-stops">
            <button
              type="button"
              class="collapse-row state squish"
              :aria-expanded="showAllStops"
              data-kbd-item="toggle-all-stops"
              @click="showAllStops = !showAllStops"
            >
              <RowIcon :icon="DirectionsBusIcon" :size="32" tone="secondary" />
              <span class="flex-1 text-left text-title-small">{{ t("allStops") }}</span>
              <span v-if="sortedStops.length" class="count">{{ sortedStops.length }}</span>
              <KeyboardArrowRightIcon
                class="m-icon size-[18px] text-outline chevron"
                :class="{ 'is-open': showAllStops }"
              />
            </button>
            <template v-if="showAllStops">
              <div v-if="!stops.length" class="progress" aria-hidden="true"><span></span></div>
              <StationRow
                v-for="stop in sortedStops.slice(0, stopsShown)"
                :key="stop.stop_id"
                :stop="stop"
                :icon-size="32"
                :data-kbd-item="`stop-${stop.stop_id}`"
                @open="openStop(stop)"
              />
              <LoadMore
                v-if="stopsShown < sortedStops.length"
                :root="scroller"
                @more="stopsShown += PAGE_SIZE"
              />
            </template>
          </section>
        </template>
        <div class="h-6"></div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.search-overlay {
  position: fixed;
  inset: 0;
  z-index: 6000;
  display: flex;
  flex-direction: column;
  background: var(--md-surface);
  color: var(--md-on-surface);
  padding-top: var(--safe-top);
}

.search-bar {
  display: flex;
  align-items: center;
  gap: 4px;
  min-height: 64px;
  padding: 4px;
  border-bottom: 1px solid var(--md-outline-variant);
}

.search-input {
  flex: 1;
  min-width: 0;
  height: 56px;
  padding: 0 12px;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--md-on-surface);
  caret-color: var(--md-primary);
}

.search-input::placeholder {
  color: var(--md-on-surface-variant);
}

.search-input::-webkit-search-cancel-button {
  appearance: none;
}

.search-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
  padding-bottom: var(--safe-bottom);
}

.collapse-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 10px 16px;
  border: 0;
  background: transparent;
  color: var(--md-on-surface);
  cursor: pointer;
}

.count {
  padding: 2px 8px;
  border-radius: 9999px;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface-variant);
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
}

.chevron {
  transition: rotate 200ms ease;
}

.chevron.is-open {
  rotate: 90deg;
}

.progress {
  position: relative;
  height: 4px;
  margin: 8px 16px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--md-secondary-container);
}

.progress > span {
  position: absolute;
  inset: 0 auto 0 0;
  width: 40%;
  border-radius: 2px;
  background: var(--md-primary);
  animation: progress 1.4s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}

@keyframes progress {
  from {
    translate: -100% 0;
  }
  to {
    translate: 250% 0;
  }
}

.search-enter-active {
  transition:
    opacity 160ms linear,
    translate 220ms cubic-bezier(0.2, 0, 0, 1);
}

.search-leave-active {
  transition:
    opacity 120ms linear,
    translate 160ms cubic-bezier(0.3, 0, 1, 1);
}

.search-enter-from,
.search-leave-to {
  opacity: 0;
  translate: 0 -6.25%;
}

.search-backdrop {
  display: none;
}

@media (max-width: 1023px) and (orientation: landscape), (min-width: 1024px) {
  .search-backdrop {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 5999;
  }

  .search-overlay {
    inset: auto;
    top: calc(8px + var(--safe-top));
    left: calc(var(--panel-shown) + 16px);
    width: calc(min(520px, 100vw - var(--panel-shown)) - 32px);
    max-height: max(240px, calc((100dvh - var(--safe-top)) * 2 / 3));
    padding-top: 0;
    overflow: hidden;
    border-radius: 28px;
    background: var(--md-surface-container-high);
    box-shadow: var(--md-elevation-3);
  }

  .search-bar {
    min-height: 56px;
    padding: 0 4px;
  }

  .search-scroll {
    flex: 0 1 auto;
    padding-bottom: 8px;
  }
}
</style>

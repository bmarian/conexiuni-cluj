<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import {useMapStore} from '@/stores/map.ts'
import {useUserStore} from '@/stores/user.ts'
import {matchingPlans, useFavoritesStore, type FavoritePlan} from '@/stores/favorites.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import {useOnline} from '@/composables/useOnline.ts'
import {useSearch} from '@/composables/useSearch.ts'
import {TIMETABLE_HASH, useOpenRoute} from '@/composables/useOpenRoute.ts'
import {useRecents} from '@/composables/useRecents.ts'
import {useKbdLayer} from '@/composables/useKeyboardNav.ts'
import {useOpenOnEnter} from '@/composables/useOpenOnEnter.ts'
import {INCOMING_SUFFIX, type Route, type Stop} from '@/types/tranzy.ts'
import {formatMeters, haversineMeters, sortByDistance} from '@/utils/geo.ts'
import {placeDetail, type Place} from '@/utils/places.ts'
import type {SearchBus, SearchOtherLine} from '@/utils/search.ts'
import {useSearchAnswer} from '@/composables/useSearchAnswer.ts'
import {matching, RESULTS_PER_KIND} from '@/utils/stopSearch.ts'
import {fold} from '@/utils/text.ts'
import {network} from '@/utils/network.ts'
import {cityCenter, cityParams, currentCity} from '@/utils/city.ts'
import {type DestinationSummary, fetchDestinations} from '@/utils/destinationPages.ts'
import {searchExamples} from '@/utils/searchExamples.ts'
import {planQuery} from '@/utils/planQuery.ts'
import {groupLocation, stopLocation} from '@/utils/pagePaths.ts'
import type {GroupLegs} from '@/utils/lineGroups.ts'
import {ArrowBackIcon, CloseIcon, DirectionsBusIcon, KeyboardArrowRightIcon, MapIcon, PlaceIcon, PushPinIcon, ScheduleIcon} from '@/utils/icons.ts'
import SubLabel from '@/components/ui/SubLabel.vue'
import RowIcon from '@/components/ui/RowIcon.vue'
import ListRow from '@/components/ui/ListRow.vue'
import RecentRow from '@/components/rows/RecentRow.vue'
import RouteRow from '@/components/rows/RouteRow.vue'
import StationRow from '@/components/rows/StationRow.vue'
import LoadMore from '@/components/LoadMore.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import StationIcon from '@/components/StationIcon.vue'
import SearchTop from '@/components/home/SearchTop.vue'

const RECENTS_SHOWN = 8
const PAGE_SIZE = 30

const {t} = useI18n()
const router = useRouter()
const mapStore = useMapStore()
const userStore = useUserStore()
const favorites = useFavoritesStore()
const {favoritePlans} = storeToRefs(favorites)
const {routes, fetchRoutes} = useRoutesApi()
const {stops, fetchStops} = useStopsApi()
const {isOnline} = useOnline()
const {open, query} = useSearch()
const {openingRouteId, openRoute} = useOpenRoute()

const panel = ref<HTMLElement | null>(null)
const scroller = ref<HTMLElement | null>(null)
const input = ref<HTMLInputElement | null>(null)

function close(clear: boolean) {
  open.value = false
  if (clear) query.value = ''
}

useKbdLayer(open, {el: () => panel.value, close: () => close(true)})

const areasByCity = new Map<string, DestinationSummary[]>()
const areas = ref<DestinationSummary[]>([])

async function loadAreas() {
  const city = currentCity.value
  const known = areasByCity.get(city)
  if (known) {
    areas.value = known
    return
  }
  try {
    const list = await fetchDestinations()
    areasByCity.set(city, list)
    if (currentCity.value === city) areas.value = list
  } catch {
    areas.value = []
  }
}

watch(open, async (isOpen) => {
  if (!isOpen) return
  void fetchRoutes()
  void fetchStops()
  void loadAreas()
  await nextTick()
  input.value?.focus({preventScroll: true})
})

const searching = computed(() => query.value.trim().length > 0)

const byName = new Intl.Collator(undefined, {numeric: true})
const sortedRoutes = computed(() => [...routes.value].sort((a, b) => byName.compare(a.route_short_name, b.route_short_name)))
const sortedStops = computed(() => {
  const sorted = [...stops.value].sort((a, b) => byName.compare(a.stop_name, b.stop_name))
  return network.value?.oneStopPerStation(sorted) ?? sorted
})

const savedResults = computed(() => matchingPlans(favoritePlans.value, query.value))

const {answer, loading: placesLoading} = useSearchAnswer(query, isOnline)
const best = computed(() => answer.value?.best)

const routeById = computed(() => new Map(routes.value.map((r) => [r.route_id, r])))
const stopById = computed(() => new Map(stops.value.map((s) => [s.stop_id, s])))

const routeResults = computed<Route[]>(() => {
  const found = answer.value
  if (found) {
    const ids = best.value === 'line' ? found.lines.slice(1) : found.lines
    return ids.flatMap((id) => routeById.value.get(id) ?? []).slice(0, RESULTS_PER_KIND)
  }
  const q = fold(query.value.trim())
  if (!q) return []
  return matching(sortedRoutes.value, (r) => fold(`${r.route_short_name} ${r.route_long_name}`), q)
    .slice(0, RESULTS_PER_KIND)
    .map((m) => m.item)
})

const stopResults = computed<Stop[]>(() => {
  const q = fold(query.value.trim())
  if (!q) return []
  const found = answer.value
  const matches = found
    ? (best.value === 'stop' ? found.stops.slice(1) : found.stops).flatMap((s) => {
        const item = stopById.value.get(s.id)
        return item ? [{item, rank: s.rank}] : []
      })
    : matching(stops.value, (stop) => fold(stop.stop_name), q)
  const loc = userStore.userLocation
  const sorted = loc
    ? sortByDistance(
        matches,
        loc.latitude,
        loc.longitude,
        (m) => m.item.stop_lat,
        (m) => m.item.stop_lon,
      )
    : matches
  const ranked = sorted.sort((a, b) => a.rank - b.rank).map((m) => m.item)
  return (network.value?.oneStopPerStation(ranked) ?? ranked).slice(0, RESULTS_PER_KIND)
})

const places = computed<Place[]>(() => {
  const found = answer.value?.places ?? []
  return best.value === 'place' ? found.slice(1) : found
})
const placesOpen = ref(false)
watch(query, () => (placesOpen.value = false))

function nearestFirst(found: Place[]): Place[] {
  const loc = userStore.userLocation
  if (!loc) return found
  return sortByDistance(
    found,
    loc.latitude,
    loc.longitude,
    (r) => parseFloat(r.lat),
    (r) => parseFloat(r.lon),
  )
}

// So nothing moves when the rest open.
const shownPlaces = computed(() => [
  ...nearestFirst(places.value.slice(0, RESULTS_PER_KIND)),
  ...(placesOpen.value ? nearestFirst(places.value.slice(RESULTS_PER_KIND)) : []),
])

function distanceTo(lat: number, lon: number): string | null {
  const loc = userStore.userLocation
  return loc ? formatMeters(haversineMeters(loc.latitude, loc.longitude, lat, lon)) : null
}

const noResults = computed(
  () =>
    !placesLoading.value &&
    !best.value &&
    !answer.value?.notice &&
    !places.value.length &&
    !savedResults.value.length &&
    !routeResults.value.length &&
    !stopResults.value.length,
)

const recents = useRecents(RECENTS_SHOWN)

const examples = computed(() => (network.value ? searchExamples(network.value, isOnline.value ? areas.value : []) : {}))
const hasExamples = computed(() => !!(examples.value.line || examples.value.stop || examples.value.place))

function tryExample(text: string) {
  query.value = text
  input.value?.focus()
}

function persistedToggle(key: string) {
  const on = ref(localStorage.getItem(key) === 'true')
  watch(on, (v) => localStorage.setItem(key, String(v)))
  return on
}

const showAllRoutes = persistedToggle('home.showAllRoutes')
const showAllStops = persistedToggle('home.showAllStops')
const routesShown = ref(PAGE_SIZE)
const stopsShown = ref(PAGE_SIZE)
watch(showAllRoutes, () => (routesShown.value = PAGE_SIZE))
watch(showAllStops, () => (stopsShown.value = PAGE_SIZE))

function go(action: () => void) {
  input.value?.blur()
  close(false)
  action()
}

function openPlace(place: Place) {
  const lat = parseFloat(place.lat)
  const lon = parseFloat(place.lon)
  go(() => {
    if (!isNaN(lat) && !isNaN(lon)) favorites.addRecent({name: place.label, lat, lon})
    void router.push({
      name: 'plan',
      query: {lat: place.lat, lon: place.lon, name: place.label},
    })
  })
}

function openPlan(plan: FavoritePlan) {
  go(() => void router.push({name: 'plan', query: planQuery(plan)}))
}

function openStop(stop: Stop) {
  go(() => {
    favorites.addRecent({stopId: stop.stop_id})
    mapStore.clearPinnedLocation()
    mapStore.setFlyToLocation(stop.stop_lat, stop.stop_lon)
    void router.push(stopLocation(stop.stop_id))
  })
}

function openGroup(legs: GroupLegs) {
  const index = network.value
  if (index) go(() => void router.push(groupLocation(index, legs)))
}

function pickRoute(route: Route, timetable = false) {
  input.value?.blur()
  void openRoute(route, undefined, timetable ? TIMETABLE_HASH : undefined).then(() => {
    favorites.addRecent({routeId: route.route_id})
    close(false)
  })
}

function pickRouteId(routeId: number, timetable: boolean) {
  const route = routeById.value.get(routeId)
  if (route) pickRoute(route, timetable)
}

function openBus(bus: SearchBus) {
  const route = routeById.value.get(bus.route_id)
  if (!route) return
  input.value?.blur()
  void openRoute(route, bus.trip_id.endsWith(INCOMING_SUFFIX) ? '1' : '0').then(() => {
    mapStore.setFlyToLocation(bus.lat, bus.lon)
    close(false)
  })
}

function openOtherLine(line: SearchOtherLine) {
  go(() => void router.push(line.slug ? `/${line.city}/linia/${line.slug}/auto` : `/${line.city}/route/${line.route_id}/auto`))
}

function openCity(slug: string) {
  go(() => void router.push(`/${slug}/`))
}

function openTrip() {
  const trip = answer.value?.trip
  if (!trip) return
  const {to, from} = trip
  openPlan(from ? {...to, originName: from.name, originLat: from.lat, originLon: from.lon} : to)
}

function openLink(url: string) {
  window.open(url, '_blank', 'noopener')
}

function openBest(): boolean {
  const found = answer.value
  const kind = best.value
  if (!found || !kind) return false
  const place = found.places[0]
  const stop = found.stops[0] && stopById.value.get(found.stops[0].id)
  if (kind === 'line') pickRouteId(found.lines[0]!, found.show === 'timetable')
  else if (kind === 'stop' && stop) openStop(stop)
  else if ((kind === 'place' || kind === 'far') && place) openPlace(place)
  else if (kind === 'bus') openBus(found.buses[0]!)
  else if (kind === 'other') openOtherLine(found.other[0]!)
  else if (kind === 'city') openCity(found.cities[0]!.slug)
  else if (kind === 'trip') openTrip()
  return true
}

function openTopResult(): boolean {
  if (openBest()) return true
  const plan = isOnline.value ? savedResults.value[0] : undefined
  const route = routeResults.value[0]
  const stop = stopResults.value[0]
  const place = isOnline.value ? shownPlaces.value[0] : undefined
  if (plan) openPlan(plan)
  else if (route) pickRoute(route)
  else if (stop) openStop(stop)
  else if (place) openPlace(place)
  return !!(plan || route || stop || place)
}

const onEnter = useOpenOnEnter(openTopResult, placesLoading, query, () => input.value?.blur())

function dropPin() {
  go(
    () =>
      void router.push({
        name: 'plan',
        query: {
          lat: String(cityCenter().lat),
          lon: String(cityCenter().lon),
          name: t('droppedPin'),
        },
      }),
  )
}
</script>

<template>
  <div v-if="open" class="search-backdrop" @pointerdown="close(false)"></div>
  <Transition name="search">
    <div v-if="open" ref="panel" class="search-overlay" role="dialog" :aria-label="t('searchPlaceholder')">
      <div class="search-bar" data-kbd-section="search" data-kbd-axis="x">
        <button type="button" class="icon-btn" data-kbd-item="search-back" :aria-label="t('back')" :title="t('back')" @click="close(true)">
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
          @keydown.enter="onEnter"
        />
        <button v-if="query" type="button" class="icon-btn" :aria-label="t('clearSearch')" :title="t('clearSearch')" @click="query = ''">
          <CloseIcon class="m-icon" />
        </button>
      </div>
      <div ref="scroller" class="search-scroll">
        <template v-if="searching">
          <div v-if="placesLoading" class="progress" aria-hidden="true"><span></span></div>
          <SearchTop
            v-if="isOnline && answer"
            :answer="answer"
            :distance-to="distanceTo"
            @line="pickRouteId"
            @stop="openStop"
            @place="openPlace"
            @bus="openBus"
            @other="openOtherLine"
            @city="openCity"
            @trip="openTrip"
            @link="openLink"
          />
          <section v-if="isOnline && savedResults.length" data-kbd-section="search-results">
            <SubLabel :text="t('favoritePlans')" class="pt-2" />
            <ListRow
              v-for="plan in savedResults"
              :key="`saved-result-${plan.lat}-${plan.lon}-${plan.originLat ?? ''}`"
              :title="plan.label ?? plan.name"
              :subtitle="plan.label ? plan.name : undefined"
              chevron
              :data-kbd-item="`saved-result-${plan.lat}_${plan.lon}`"
              @open="openPlan(plan)"
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
          <!-- Last: places arrive after the typing stops and would push the rows above them down. -->
          <section v-if="isOnline && places.length" data-kbd-section="search-results">
            <SubLabel :text="t('searchResultsPlaces')" class="pt-2" />
            <ListRow
              v-for="place in shownPlaces"
              :key="place.id"
              :title="place.name"
              :subtitle="placeDetail(place) || undefined"
              :trailing="distanceTo(parseFloat(place.lat), parseFloat(place.lon)) ?? undefined"
              chevron
              :data-kbd-item="`place-${place.id}`"
              @open="openPlace(place)"
            >
              <template #icon><RowIcon :icon="PlaceIcon" tone="tertiary" /></template>
            </ListRow>
            <ListRow
              v-if="!placesOpen && places.length > RESULTS_PER_KIND"
              :title="t('routeChangeShowMore')"
              data-kbd-item="more-places"
              @open="placesOpen = true"
            >
              <template #icon><RowIcon :icon="KeyboardArrowRightIcon" class="rotate-90" /></template>
            </ListRow>
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
            <p v-else class="text-body-medium text-on-surface-variant p-4">{{ t('noResults') }}</p>
          </template>
        </template>

        <template v-else>
          <section v-if="!recents.length && hasExamples" data-kbd-section="search-examples">
            <SubLabel :text="t('searchExamples')" class="pt-2" />
            <ListRow
              v-if="examples.line"
              :title="examples.line.route_short_name"
              :subtitle="t('searchExampleLine')"
              data-kbd-item="example-line"
              @open="tryExample(examples.line!.route_short_name)"
            >
              <template #icon>
                <span class="example-icon">
                  <LineBadge :name="examples.line.route_short_name" :color="examples.line.route_color" :height="26" :min-width="36" :corner="6" />
                </span>
              </template>
            </ListRow>
            <ListRow
              v-if="examples.stop"
              :title="examples.stop.stop_name"
              :subtitle="t('searchExampleStop')"
              data-kbd-item="example-stop"
              @open="tryExample(examples.stop!.stop_name)"
            >
              <template #icon><StationIcon :look="network?.look(examples.stop.stop_id)" class="size-9" /></template>
            </ListRow>
            <ListRow
              v-if="examples.place"
              :title="examples.place"
              :subtitle="t('searchExamplePlace')"
              data-kbd-item="example-place"
              @open="tryExample(examples.place!)"
            >
              <template #icon><RowIcon :icon="PlaceIcon" tone="tertiary" /></template>
            </ListRow>
          </section>
          <section v-if="recents.length" data-kbd-section="recent-plans">
            <SubLabel :text="t('recentDestinations')" class="pt-2" />
            <template v-for="item in recents" :key="item.key">
              <ListRow
                v-if="item.plan"
                :title="item.plan.name"
                :subtitle="item.plan.originName ? `${t('planFrom')}: ${item.plan.originName}` : undefined"
                :remove-label="t('removeFromRecents')"
                :data-kbd-item="`recent-${item.key}`"
                @open="openPlan(item.plan!)"
                @remove="favorites.removeRecent(item.recent)"
              >
                <template #icon><RowIcon :icon="ScheduleIcon" /></template>
              </ListRow>
              <RecentRow
                v-else
                :stop="item.stop"
                :route="item.route"
                :group="item.group"
                :remove-label="t('removeFromRecents')"
                :busy="!!item.route && openingRouteId === item.route.route_id"
                :data-kbd-item="`recent-${item.key}`"
                @open="item.stop ? openStop(item.stop) : item.route ? pickRoute(item.route) : openGroup(item.group!)"
                @remove="favorites.removeRecent(item.recent)"
              />
            </template>
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
              <span class="flex-1 text-left text-title-small">{{ t('linesIn', cityParams()) }}</span>
              <span v-if="sortedRoutes.length" class="count">{{ sortedRoutes.length }}</span>
              <KeyboardArrowRightIcon class="m-icon size-[18px] text-outline chevron" :class="{'is-open': showAllRoutes}" />
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
              <LoadMore v-if="routesShown < sortedRoutes.length" :root="scroller" @more="routesShown += PAGE_SIZE" />
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
              <span class="flex-1 text-left text-title-small">{{ t('stopsIn', cityParams()) }}</span>
              <span v-if="sortedStops.length" class="count">{{ sortedStops.length }}</span>
              <KeyboardArrowRightIcon class="m-icon size-[18px] text-outline chevron" :class="{'is-open': showAllStops}" />
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
              <LoadMore v-if="stopsShown < sortedStops.length" :root="scroller" @more="stopsShown += PAGE_SIZE" />
            </template>
          </section>
        </template>
        <div class="h-6"></div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.example-icon {
  display: inline-flex;
  justify-content: center;
  flex-shrink: 0;
  width: 36px;
}

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
  transition: rotate 175ms ease;
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
    opacity 140ms linear,
    translate 190ms cubic-bezier(0.2, 0, 0, 1);
}

.search-leave-active {
  transition:
    opacity 105ms linear,
    translate 140ms cubic-bezier(0.3, 0, 1, 1);
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

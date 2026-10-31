<script setup lang="ts">
import {useUserStore} from '@/stores/user.ts'
import {storeToRefs} from 'pinia'
import {computed, defineAsyncComponent, toRef, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import StationIcon from '@/components/StationIcon.vue'
import {network} from '@/utils/network.ts'
import {OUTGOING_SUFFIX, type ShapeInfo, type VehiclesInStop} from '@/types/tranzy.ts'
import {formatMinutesFromNow} from '@/utils/time.ts'
import {useStopDepartures} from '@/composables/useStopDepartures.ts'
import {useRouteStore} from '@/stores/route.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useRoute, useRouter} from 'vue-router'
import {getDirectionFromTripId, getTripIdForRouteAtStop} from '@/utils/trips.ts'
import {useMapStore} from '@/stores/map.ts'
import {useKbdShortcuts} from '@/composables/useKeyboardNav.ts'
import {canonicalUrl} from '@/utils/site.ts'
import {cityParams} from '@/utils/city.ts'
import {linePath, stopPath} from '@/utils/pagePaths.ts'
import {servedName} from '@/utils/servedName.ts'
import {groupByMode, mainMode} from '@/utils/modes.ts'
import {useShare} from '@/composables/useShare.ts'
import {adminAuthed, stopControlsOn} from '@/composables/useAdmin.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ActionChips from '@/components/ui/ActionChips.vue'
import SectionLabel from '@/components/ui/SectionLabel.vue'
import SubLabel from '@/components/ui/SubLabel.vue'
import DepartureCard from '@/components/rows/DepartureCard.vue'
import LiveDot from '@/components/ui/LiveDot.vue'
import {provideLiveBeat} from '@/composables/useLiveBeat.ts'
import {useDueArrivals} from '@/composables/useDueArrivals.ts'
import {displayColor} from '@/utils/lineColor.ts'
import RouteRow from '@/components/rows/RouteRow.vue'
import type {ChipAction, MenuAction} from '@/components/ui/types.ts'
import {DirectionsIcon, FavoriteBorderIcon, FavoriteIcon, MapIcon, RefreshIcon, ShareIcon} from '@/utils/icons.ts'

const props = defineProps<{stopId: string}>()

const AdminHideStop = defineAsyncComponent(() => import('@/components/admin/AdminHideStop.vue'))
const showStopControls = adminAuthed() && stopControlsOn()

const {t} = useI18n()
const userStore = useUserStore()
const routeStore = useRouteStore()
const favoritesStore = useFavoritesStore()
const mapStore = useMapStore()
const router = useRouter()
const currentRoute = useRoute()
const servedAs = computed(() => servedName(currentRoute.path))
const stopIdNum = computed(() => Number(props.stopId))
const {userTime} = storeToRefs(userStore)
const {
  stopInfo,
  platformOf,
  isFavorite,
  toggleFavorite,
  stopName,
  isLoading,
  loadError,
  isComputingDepartures,
  departures: shapesComingToTheStopBasedOnVehiclePositions,
  asOfMs,
  busesWithAvailableTimetables,
  isDepartureFavorite,
  reload,
} = useStopDepartures(toRef(props, 'stopId'))

provideLiveBeat(asOfMs)

useDueArrivals(shapesComingToTheStopBasedOnVehiclePositions, (shape) => {
  if (isDepartureFavorite(shape)) mapStore.signalArrival(displayColor(shape.route_short_name, shape.route_color), shape.next_times[0]?.vehicle_id)
})

useKbdShortcuts({f: toggleFavorite})

const platformAt = (shape: VehiclesInStop) => shape.stop_id ?? stopIdNum.value

function planHere() {
  const info = stopInfo.value
  if (!info) return
  void router.push({name: 'plan', query: {lat: String(info.stop_lat), lon: String(info.stop_lon), name: info.stop_name}})
}

watch(
  () => stopInfo.value?.stop_id,
  () => {
    const info = stopInfo.value
    if (!info) return
    mapStore.setFlyToLocation(info.stop_lat, info.stop_lon)
  },
  {immediate: true},
)

useHead(() => {
  // Keep the server-injected meta for crawlers until the stop loads.
  if (!stopName.value) return {}
  const title = t('headStopTitle', {...cityParams(), stopName: stopName.value})
  const description = t('headStopDesc', {...cityParams(), stopName: stopName.value})
  const url = canonicalUrl(stopPath(props.stopId))
  return {
    title,
    meta: [
      {name: 'description', content: description},
      {property: 'og:title', content: title},
      {property: 'og:description', content: description},
      {property: 'og:url', content: url},
      {name: 'twitter:title', content: title},
      {name: 'twitter:description', content: description},
    ],
    link: [{rel: 'canonical', href: url}],
  }
})

function formatMinutes(minutes: number): string {
  return formatMinutesFromNow(minutes, userTime.value || new Date(), t('now'))
}

const lineGroups = computed(() =>
  groupByMode(busesWithAvailableTimetables.value || []).map((group) => ({
    ...group,
    lines: [...group.lines].sort(
      (a: ShapeInfo, b: ShapeInfo) => (favoritesStore.isRouteFavorite(a.route_id) ? 0 : 1) - (favoritesStore.isRouteFavorite(b.route_id) ? 0 : 1),
    ),
  })),
)

const look = computed(() => network.value?.look(stopIdNum.value))
const stationMode = computed(() => look.value?.mode ?? mainMode((busesWithAvailableTimetables.value || []).map((s: ShapeInfo) => s.route_type)))

const departuresSorted = computed(() => {
  return [...shapesComingToTheStopBasedOnVehiclePositions.value].sort((a, b) => {
    const aFav = isDepartureFavorite(a) ? 0 : 1
    const bFav = isDepartureFavorite(b) ? 0 : 1
    if (aFav !== bFav) return aFav - bFav
    return a.minutes_left - b.minutes_left
  })
})

const navigateToRoute = (shape: VehiclesInStop) => {
  const si = platformOf(shape.route_id)?.shapes_info.find((s: ShapeInfo) => s.route_id === shape.route_id)
  if (!si) return
  routeStore.setSelectedRoute(si, shape.trip_id, String(platformAt(shape)), stopName.value || '')
  router.push(linePath(shape.route_id, getDirectionFromTripId(shape.trip_id)))
}

const tripAtStop = (shape: ShapeInfo) => {
  const platform = platformOf(shape.route_id) ?? stopInfo.value
  return getTripIdForRouteAtStop(platform?.outgoing_trip_ids || [], platform?.incoming_trip_ids || [], shape.route_id) || `${shape.route_id}${OUTGOING_SUFFIX}`
}

const allRouteHref = (shape: ShapeInfo) => linePath(shape.route_id, getDirectionFromTripId(tripAtStop(shape)))

const navigateToAllRoute = (shape: ShapeInfo) => {
  const platform = platformOf(shape.route_id) ?? stopInfo.value
  const tripId = tripAtStop(shape)
  routeStore.setSelectedRoute(shape, tripId, String(platform?.stop_id ?? props.stopId), stopName.value || '')
  router.push(linePath(shape.route_id, getDirectionFromTripId(tripId)))
}

const {share} = useShare()

const topMenu = computed<MenuAction[]>(() => [{label: t('refresh'), icon: RefreshIcon, onClick: reload}])

const chips = computed<ChipAction[]>(() => [
  ...(stopInfo.value ? [{label: t('directions'), icon: DirectionsIcon, primary: true, kbd: 'directions', onClick: planHere}] : []),
  {
    label: isFavorite.value ? t('saved') : t('save'),
    icon: isFavorite.value ? FavoriteIcon : FavoriteBorderIcon,
    selected: isFavorite.value,
    tint: isFavorite.value ? 'var(--md-rose)' : undefined,
    kbd: 'fav',
    onClick: toggleFavorite,
  },
  {label: t('share'), icon: ShareIcon, kbd: 'share', onClick: () => void share()},
])
</script>

<template>
  <div class="stop-view">
    <SheetTopBar :title="stopName" :menu="topMenu" />

    <template v-if="isLoading">
      <div class="header">
        <span class="skeleton size-[52px] rounded-2xl"></span>
        <div class="flex-1 flex flex-col gap-2 pt-1">
          <span class="skeleton h-3 w-24 rounded"></span>
          <h1 v-if="servedAs" class="text-title-large font-bold">{{ servedAs }}</h1>
          <span v-else class="skeleton h-6 w-44 rounded-lg"></span>
        </div>
      </div>
      <div v-for="i in 3" :key="i" class="skeleton card-skeleton"></div>
    </template>

    <div v-else-if="loadError" class="flex flex-col items-start gap-3 p-4">
      <p class="text-body-medium text-on-surface-variant">{{ t('loadFailed') }}</p>
      <button type="button" class="btn btn-tonal" @click="reload">{{ t('retry') }}</button>
    </div>

    <template v-else>
      <header class="header">
        <StationIcon :look="look" :corner="16" :favorite="isFavorite" class="size-[52px]" />
        <div class="flex-1 min-w-0 pt-0.5">
          <div class="flex gap-2 text-label-medium">
            <span class="text-primary">{{ t(`stopKind_${stationMode}`) }}</span>
            <span v-if="stopInfo?.stop_code" class="text-outline">#{{ stopInfo.stop_code }}</span>
          </div>
          <h1 class="text-title-large font-bold">{{ stopName }}</h1>
        </div>
      </header>
      <ActionChips :actions="chips" class="pb-2" />

      <AdminHideStop v-if="showStopControls" :stop-id="stopIdNum" />

      <section class="flex flex-col" data-kbd-section="departures" data-kbd-entry="1">
        <div class="departures-label">
          <LiveDot />
          <h2 class="text-title-small text-on-surface-variant">{{ t('nextDepartures') }}</h2>
        </div>
        <template v-if="isComputingDepartures">
          <div v-for="i in 3" :key="i" class="skeleton card-skeleton"></div>
        </template>
        <p v-else-if="!departuresSorted.length" class="text-body-medium text-on-surface-variant px-4 py-3">{{ t('noSchedule') }}</p>
        <template v-else>
          <DepartureCard
            v-for="(shape, i) in departuresSorted"
            :key="shape.trip_id"
            :class="{cascade: i < 12}"
            :style="{'--i': i}"
            :shape="shape"
            :label="formatMinutes"
            :favorite="isDepartureFavorite(shape)"
            :data-kbd-item="`dep-${shape.trip_id}`"
            @open="navigateToRoute(shape)"
          />
        </template>
      </section>

      <section v-if="lineGroups.length" class="pt-4" data-kbd-section="stop-routes" data-kbd-entry="2">
        <SectionLabel :text="t('allRoutesAtStop')" :icon="MapIcon" />
        <template v-for="group in lineGroups" :key="group.mode">
          <SubLabel v-if="lineGroups.length > 1" :text="t(`modeLines_${group.mode}`)" class="pt-2" />
          <RouteRow
            v-for="shape in group.lines"
            :key="shape.route_short_name"
            :name="shape.route_short_name"
            :color="shape.route_color"
            :title="shape.timetable.route_long_name"
            :favorite="favoritesStore.isRouteFavorite(shape.route_id)"
            :data-kbd-item="`route-${shape.route_short_name}`"
            :href="allRouteHref(shape)"
            @open="navigateToAllRoute(shape)"
          />
        </template>
      </section>
      <div class="h-6"></div>
    </template>
  </div>
</template>

<style scoped>
.header {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 4px 8px 8px 16px;
}

.departures-label {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
}

.skeleton {
  display: block;
  background: var(--md-surface-container);
}

.card-skeleton {
  height: 64px;
  margin: 4px 12px;
  border-radius: 12px;
}
</style>

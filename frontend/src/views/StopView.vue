<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts"
import {storeToRefs} from "pinia"
import {computed, ref, toRef, watch} from "vue"
import {useHead} from '@unhead/vue'
import {useI18n} from "vue-i18n"
import StationIcon from "@/components/StationIcon.vue"
import {network} from "@/utils/network.ts";
import {
  OUTGOING_SUFFIX,
  type ShapeInfo,
  type VehiclesInStop
} from "@/types/tranzy.ts";
import {formatMinutesFromNow, routeDestination} from "@/utils/time.ts";
import {useStopDepartures} from "@/composables/useStopDepartures.ts";
import {useRouteStore} from "@/stores/route.ts";
import {useFavoritesStore} from "@/stores/favorites.ts";
import {useRoute, useRouter} from "vue-router";
import BusMenu, {type BusMenuItem} from "@/components/BusMenu.vue";
import {useLongPress} from "@/composables/useLongPress.ts";
import {useFollowsStore} from "@/stores/follows.ts";
import {nextBusReminder, useRemindersStore, type ReminderTarget} from "@/stores/reminders.ts";
import {justNotify, walkFrom} from "@/utils/follow.ts";
import {MAX_REMINDERS, nextOf} from "@/utils/reminders.ts";
import {reminderWhen} from "@/utils/reminderText.ts";
import {askNotifications} from "@/utils/notify.ts";
import {pushSupported} from "@/utils/pushSubscription.ts";
import {getRouteIdFromTripId} from "@/utils/trips.ts";
import {getDirectionFromTripId, getTripIdForRouteAtStop} from "@/utils/trips.ts";
import {useSettingsStore} from "@/stores/settings.ts"
import {useMapStore} from "@/stores/map.ts"
import {useKbdShortcuts} from "@/composables/useKeyboardNav.ts";
import {siteUrl} from "@/utils/site.ts";
import {cityParams, cityPath} from "@/utils/city.ts";
import {groupByMode, mainMode} from "@/utils/modes.ts";
import {useShare} from "@/composables/useShare.ts";
import SheetTopBar from "@/components/ui/SheetTopBar.vue";
import ActionChips from "@/components/ui/ActionChips.vue";
import SectionLabel from "@/components/ui/SectionLabel.vue";
import SubLabel from "@/components/ui/SubLabel.vue";
import DepartureCard from "@/components/rows/DepartureCard.vue";
import RouteRow from "@/components/rows/RouteRow.vue";
import type {ChipAction, MenuAction} from "@/components/ui/types.ts";
import {DirectionsIcon, FavoriteBorderIcon, FavoriteIcon, MapIcon, RefreshIcon, ShareIcon} from "@/utils/icons.ts";

const props = defineProps<{ stopId: string }>()

const {t, locale} = useI18n()
const userStore = useUserStore()
const settings = useSettingsStore()
const routeStore = useRouteStore()
const favoritesStore = useFavoritesStore()
const router = useRouter()
const route = useRoute()
const follows = useFollowsStore()
const remindersStore = useRemindersStore()
const stopIdNum = computed(() => Number(props.stopId))
const {userTime, userLocation} = storeToRefs(userStore)
const {
  stopInfo,
  stopInfos,
  platformOf,
  isFavorite,
  toggleFavorite,
  stopName,
  isLoading,
  loadError,
  isComputingDepartures,
  departures: shapesComingToTheStopBasedOnVehiclePositions,
  busesWithAvailableTimetables,
  isDepartureFavorite,
  reload,
} = useStopDepartures(toRef(props, 'stopId'))

useKbdShortcuts({f: toggleFavorite})

const pointerHint = typeof window !== 'undefined' && window.matchMedia?.('(hover: hover) and (pointer: fine)').matches

const platformAt = (shape: VehiclesInStop) => shape.stop_id ?? stopIdNum.value

function busAt(shape: VehiclesInStop): ReminderTarget & {stopLat: number, stopLon: number} {
  const platform = stopInfos.value.find((info) => info.stop_id === platformAt(shape)) ?? stopInfo.value
  return {
    stopId: platformAt(shape),
    stopName: stopName.value || '',
    stopLat: platform?.stop_lat ?? 0,
    stopLon: platform?.stop_lon ?? 0,
    routeId: shape.route_id,
    tripId: shape.trip_id,
    line: shape.route_short_name,
    color: shape.route_color,
    destination: routeDestination(shape.route_long_name),
  }
}

const following = (shape: VehiclesInStop) => follows.find(platformAt(shape), shape.route_id, shape.trip_id)
const busReminders = (shape: VehiclesInStop) => remindersStore.reminders.filter((r) =>
  r.stopId === platformAt(shape) && r.routeId === shape.route_id && r.tripId === shape.trip_id)
const isReminded = (shape: VehiclesInStop) => busReminders(shape).some((r) => !r.date)
const isRemindOn = (shape: VehiclesInStop) => !!following(shape) || busReminders(shape).some((r) => r.date)

// At the stop, or too late to walk there, it's told when the bus comes; otherwise reminded when to leave for it.
async function remind(shape: VehiclesInStop) {
  const target = busAt(shape)
  const walk = walkFrom(userLocation.value, target.stopLat, target.stopLon)
  const reminder = justNotify(walk, shape.next_times?.[0]?.minutes ?? shape.minutes_left) || !pushSupported()
    ? null
    : nextBusReminder(target, walk)
  if (!reminder) return follows.start(target)
  if (remindersStore.full()) return settings.showToast(t('limitReached', {n: MAX_REMINDERS}), {icon: 'bell-off'})
  if ((await askNotifications()) !== 'granted') return settings.showToast(t('notificationsBlocked'), {icon: 'bell-off'})
  const next = nextOf(reminder, Date.now())
  if (await remindersStore.save(reminder)) {
    settings.showToast(t('remindSaved'), {body: next === null ? undefined : reminderWhen(next, locale.value), icon: 'bell'})
  } else {
    settings.showToast(t('remindFailed'), {icon: 'bell-off'})
  }
}

function toggleRemind(shape: VehiclesInStop) {
  if (!isRemindOn(shape)) return void remind(shape)
  const followed = following(shape)
  if (followed) follows.remove(followed.id)
  for (const r of busReminders(shape)) if (r.date) void remindersStore.remove(r.id)
}

const menu = ref<{ x: number, y: number, items: BusMenuItem[] } | null>(null)
const press = useLongPress<VehiclesInStop>((shape, x, y) => {
  const followed = following(shape)
  const items: BusMenuItem[] = []
  if (followed) items.push({key: 'notify', label: t('notifyMeOff'), icon: 'bell-off', action: () => follows.remove(followed.id)})
  items.push({key: 'remind', label: t('remindMenu'), icon: 'alarm', action: () => remindersStore.openSheet(busAt(shape))})
  menu.value = {x, y, items}
})

function planHere() {
  const info = stopInfo.value
  if (!info) return
  void router.push({name: 'plan', query: {lat: String(info.stop_lat), lon: String(info.stop_lon), name: info.stop_name}})
}

const mapStore = useMapStore()
watch(() => stopInfo.value?.stop_id, () => {
  const info = stopInfo.value
  if (info) mapStore.setFlyToLocation(info.stop_lat, info.stop_lon)
}, {immediate: true})

watch([stopInfo, () => route.query.follow], ([info, tripId]) => {
  const at = Number(route.query.at)
  if (!info || typeof tripId !== 'string' || !Number.isFinite(at)) return
  if (!settings.busAlerts) return void router.replace({query: {}})
  const routeId = getRouteIdFromTripId(tripId)
  const shape = info.shapes_info.find((s: ShapeInfo) => s.route_id === routeId)
  if (routeId !== null && shape && at > Date.now() - 5 * 60_000) {
    const longName = tripId.endsWith('_1') ? shape.timetable.route_long_name.split(' - ').reverse().join(' - ') : shape.timetable.route_long_name
    void follows.start({
      stopId: stopIdNum.value, stopName: info.stop_name, stopLat: info.stop_lat, stopLon: info.stop_lon,
      routeId, tripId, line: shape.route_short_name, color: shape.route_color,
      destination: routeDestination(longName), targetAt: at,
    })
  }
  void router.replace({query: {}})
}, {immediate: true})

useHead(() => {
  // Keep the server-injected meta for crawlers until the stop loads.
  if (!stopName.value) return {}
  const title = t('headStopTitle', {...cityParams(), stopName: stopName.value})
  const description = t('headStopDesc', {...cityParams(), stopName: stopName.value})
  const url = siteUrl(cityPath(`/stop/${props.stopId}`))
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

const lineGroups = computed(() => groupByMode(busesWithAvailableTimetables.value || []).map((group) => ({
  ...group,
  lines: [...group.lines].sort((a: ShapeInfo, b: ShapeInfo) =>
    (favoritesStore.isRouteFavorite(a.route_id) ? 0 : 1) - (favoritesStore.isRouteFavorite(b.route_id) ? 0 : 1)),
})))

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
  router.push({
    name: 'route',
    params: {
      routeId: shape.route_id,
      direction: getDirectionFromTripId(shape.trip_id)
    }
  })
}

const navigateToAllRoute = (shape: ShapeInfo) => {
  const platform = platformOf(shape.route_id) ?? stopInfo.value
  const tripId = getTripIdForRouteAtStop(platform?.outgoing_trip_ids || [], platform?.incoming_trip_ids || [], shape.route_id) || `${shape.route_id}${OUTGOING_SUFFIX}`
  routeStore.setSelectedRoute(shape, tripId, String(platform?.stop_id ?? props.stopId), stopName.value || '')
  router.push({
    name: 'route',
    params: {routeId: shape.route_id, direction: getDirectionFromTripId(tripId)}
  })
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
    <SheetTopBar :title="stopName" :menu="topMenu"/>

    <template v-if="isLoading">
      <div class="header">
        <span class="skeleton size-[52px] rounded-2xl"></span>
        <div class="flex-1 flex flex-col gap-2 pt-1">
          <span class="skeleton h-3 w-24 rounded"></span>
          <span class="skeleton h-6 w-44 rounded-lg"></span>
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
        <StationIcon :look="look" :corner="16" class="size-[52px]"/>
        <div class="flex-1 min-w-0 pt-0.5">
          <div class="flex gap-2 text-label-medium">
            <span class="text-primary">{{ t(`stopKind_${stationMode}`) }}</span>
            <span v-if="stopInfo?.stop_code" class="text-outline">#{{ stopInfo.stop_code }}</span>
          </div>
          <h1 class="text-title-large font-bold">{{ stopName }}</h1>
        </div>
      </header>
      <ActionChips :actions="chips" class="pb-2"/>

      <section data-kbd-section="departures" data-kbd-entry="1">
        <div class="departures-label">
          <span class="live-dot"></span>
          <h2 class="text-title-small text-on-surface-variant">{{ t('nextDepartures') }}</h2>
          <span class="flex-1"></span>
          <span v-if="settings.busAlerts" class="text-label-small text-outline">{{ pointerHint ? t('departuresClickHint') : t('departuresHoldHint') }}</span>
        </div>
        <template v-if="isComputingDepartures">
          <div v-for="i in 3" :key="i" class="skeleton card-skeleton"></div>
        </template>
        <p v-else-if="!departuresSorted.length" class="text-body-medium text-on-surface-variant px-4 py-3">{{ t('noSchedule') }}</p>
        <template v-else>
        <DepartureCard
          v-for="(shape, i) in departuresSorted"
          :key="shape.trip_id"
          v-bind="settings.busAlerts ? press.bind(shape) : {}"
          :class="{ cascade: i < 12 }"
          :style="{ '--i': i }"
          :shape="shape"
          :label="formatMinutes"
          :favorite="isDepartureFavorite(shape)"
          :remindable="settings.busAlerts"
          :remind-on="isRemindOn(shape)"
          :reminded="settings.busAlerts && isReminded(shape)"
          :data-kbd-item="`dep-${shape.trip_id}`"
          @open="press.click(() => navigateToRoute(shape))"
          @remind="toggleRemind(shape)"
        />
        </template>
      </section>

      <section v-if="lineGroups.length" class="pt-4" data-kbd-section="stop-routes" data-kbd-entry="2">
        <SectionLabel :text="t('allRoutesAtStop')" :icon="MapIcon"/>
        <template v-for="group in lineGroups" :key="group.mode">
          <SubLabel v-if="lineGroups.length > 1" :text="t(`modeLines_${group.mode}`)" class="pt-2"/>
          <RouteRow
            v-for="shape in group.lines"
            :key="shape.route_short_name"
            :name="shape.route_short_name"
            :color="shape.route_color"
            :title="shape.timetable.route_long_name"
            :favorite="favoritesStore.isRouteFavorite(shape.route_id)"
            :data-kbd-item="`route-${shape.route_short_name}`"
            @open="navigateToAllRoute(shape)"
          />
        </template>
      </section>
      <div class="h-6"></div>
    </template>

    <BusMenu v-if="menu" :x="menu.x" :y="menu.y" :items="menu.items" @close="menu = null"/>
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

.live-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--md-live);
  animation: live-pulse 1s ease-in-out infinite alternate;
}

@keyframes live-pulse {
  to {
    opacity: 0.35;
  }
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

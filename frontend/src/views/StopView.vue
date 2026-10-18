<script setup lang="ts">
import {useUserStore} from "@/stores/user.ts"
import {storeToRefs} from "pinia"
import {computed, ref, toRef, watch} from "vue"
import {useHead} from '@unhead/vue'
import {useI18n} from "vue-i18n"
import StopIcon from "@/assets/stop.svg"
import IconHeartFilled from "@/components/icons/IconHeartFilled.vue"
import IconHeartOutline from "@/components/icons/IconHeartOutline.vue"
import {
  OUTGOING_SUFFIX,
  type ShapeInfo,
  type VehiclesInStop
} from "@/types/tranzy.ts";
import {formatMinutesFromNow} from "@/utils/time.ts";
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
import HeaderNavigation from "@/components/HeaderNavigation.vue"
import {getDirectionFromTripId, getTripIdForRouteAtStop} from "@/utils/trips.ts";
import {useSettingsStore} from "@/stores/settings.ts"
import {useMapStore} from "@/stores/map.ts"
import ShareButton from "@/components/ShareButton.vue";
import {useKbdShortcuts} from "@/composables/useKeyboardNav.ts";
import {keepHyphenatedWords} from "@/utils/text.ts";
import {siteUrl} from "@/utils/site.ts";
import {cityPath} from '@/utils/city.ts'

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
const isFavorite = computed(() => favoritesStore.isStopFavorite(stopIdNum.value))

useKbdShortcuts({f: () => favoritesStore.toggleStopFavorite(stopIdNum.value)})
const {userTime, userLocation} = storeToRefs(userStore)
const {
  stopInfo,
  stopName,
  isLoading,
  loadError,
  isComputingDepartures,
  departures: shapesComingToTheStopBasedOnVehiclePositions,
  busesWithAvailableTimetables,
  isDepartureFavorite,
  reload,
} = useStopDepartures(toRef(props, 'stopId'))

const pointerHint = typeof window !== 'undefined' && window.matchMedia?.('(hover: hover) and (pointer: fine)').matches

function busAt(shape: VehiclesInStop): ReminderTarget & {stopLat: number, stopLon: number} {
  return {
    stopId: stopIdNum.value,
    stopName: stopName.value || '',
    stopLat: stopInfo.value?.stop_lat ?? 0,
    stopLon: stopInfo.value?.stop_lon ?? 0,
    routeId: shape.route_id,
    tripId: shape.trip_id,
    line: shape.route_short_name,
    color: shape.route_color,
    destination: routeDestination(shape.route_long_name),
  }
}

const following = (shape: VehiclesInStop) => follows.find(stopIdNum.value, shape.route_id, shape.trip_id)
const busReminders = (shape: VehiclesInStop) => remindersStore.reminders.filter((r) =>
  r.stopId === stopIdNum.value && r.routeId === shape.route_id && r.tripId === shape.trip_id)
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
  if (info && !settings.watchActive) mapStore.setFlyToLocation(info.stop_lat, info.stop_lon)
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
  const title = t('headStopTitle', {stopName: stopName.value})
  const description = t('headStopDesc', {stopName: stopName.value})
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

const busesWithAvailableTimetablesSorted = computed(() => {
  return [...(busesWithAvailableTimetables.value || [])].sort((a: ShapeInfo, b: ShapeInfo) => {
    const aFav = favoritesStore.isRouteFavorite(a.route_id) ? 0 : 1
    const bFav = favoritesStore.isRouteFavorite(b.route_id) ? 0 : 1
    if (aFav !== bFav) return aFav - bFav
    return a.route_short_name.localeCompare(b.route_short_name, undefined, {numeric: true})
  })
})

const departuresSorted = computed(() => {
  return [...shapesComingToTheStopBasedOnVehiclePositions.value].sort((a, b) => {
    const aFav = isDepartureFavorite(a) ? 0 : 1
    const bFav = isDepartureFavorite(b) ? 0 : 1
    if (aFav !== bFav) return aFav - bFav
    return a.minutes_left - b.minutes_left
  })
})

function routeDestination(name: string): string {
  const i = name.lastIndexOf(' - ')
  return i >= 0 ? name.slice(i + 3) : name
}

function routeOrigin(name: string): string {
  const i = name.lastIndexOf(' - ')
  return i >= 0 ? name.slice(0, i) : ''
}

const navigateToRoute = (shape: VehiclesInStop) => {
  const si = stopInfo.value?.shapes_info?.find((s: ShapeInfo) => s.route_id === shape.route_id)
  if (!si) return
  routeStore.setSelectedRoute(si, shape.trip_id, props.stopId, stopName.value || '')
  router.push({
    name: 'route',
    params: {
      routeId: shape.route_id,
      direction: getDirectionFromTripId(shape.trip_id)
    }
  })
}

const navigateToAllRoute = (shape: ShapeInfo) => {
  const tripId = getTripIdForRouteAtStop(stopInfo.value?.outgoing_trip_ids || [], stopInfo.value?.incoming_trip_ids || [], shape.route_id) || `${shape.route_id}${OUTGOING_SUFFIX}`
  routeStore.setSelectedRoute(shape, tripId, props.stopId, stopName.value || '')
  router.push({
    name: 'route',
    params: {routeId: shape.route_id, direction: getDirectionFromTripId(tripId)}
  })
}
</script>

<template>
  <div v-if="isLoading"
       class="stop-view-container bg-white dark:bg-[#0f172a] animate-pulse flex flex-col gap-8">
    <header class="flex items-center gap-4">
      <div class="w-11 h-11 rounded-full bg-slate-200 dark:bg-slate-800 shrink-0"></div>
      <div class="h-7 w-44 bg-slate-200 dark:bg-slate-800 rounded-lg"></div>
    </header>
    <section class="flex flex-col gap-3">
      <div class="h-3 w-32 bg-slate-200 dark:bg-slate-800 rounded mb-2"></div>
      <div v-for="i in 4" :key="i"
           class="flex items-center gap-3 rounded-2xl p-3 border border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/30">
        <div class="w-11 h-9 rounded-xl bg-slate-200 dark:bg-slate-700 shrink-0"></div>
        <div class="flex-1 flex flex-col gap-1.5">
          <div class="h-2.5 w-16 bg-slate-200 dark:bg-slate-700 rounded"></div>
          <div class="h-4 w-36 bg-slate-200 dark:bg-slate-700 rounded"></div>
        </div>
        <div class="flex gap-1.5">
          <div v-for="j in 3" :key="j"
               class="w-9 h-6 rounded-lg bg-slate-200 dark:bg-slate-700"></div>
        </div>
      </div>
    </section>
    <section class="flex flex-col gap-2">
      <div class="h-3 w-40 bg-slate-200 dark:bg-slate-800 rounded mb-2"></div>
      <div v-for="i in 5" :key="i" class="flex items-center gap-3 py-2">
        <div class="w-10 h-7 rounded-md bg-slate-200 dark:bg-slate-800 shrink-0"></div>
        <div class="h-3.5 w-40 bg-slate-200 dark:bg-slate-800 rounded"></div>
      </div>
    </section>
  </div>

  <div v-else-if="loadError"
       class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-4">
    <div class="flex items-center -mb-2">
      <HeaderNavigation />
    </div>
    <p class="text-sm text-slate-500 dark:text-slate-400">{{ t('loadFailed') }}</p>
    <button type="button" class="retry-btn self-start" @click="reload">{{ t('retry') }}</button>
  </div>

  <div v-else
       class="stop-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-8">

    <template v-if="settings.paperActive">
    <div class="flex items-center -mb-4">
      <HeaderNavigation />
    </div>

    <header class="pp-masthead" data-kbd-section="actions" data-kbd-axis="x">
      <p class="pp-kicker">
        {{ t('busStop') }}<template v-if="stopInfo?.stop_code"> · Nº {{ stopInfo.stop_code }}</template>
      </p>
      <h1 class="pp-title">{{ keepHyphenatedWords(stopName || '') }}</h1>
      <div class="pp-actions">
        <button
          type="button"
          class="pp-action"
          :aria-pressed="isFavorite"
          :aria-label="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
          data-kbd-item="fav"
          @click="favoritesStore.toggleStopFavorite(stopIdNum)"
        >{{ isFavorite ? '♥' : '♡' }} {{ t('paperFavoriteStamp') }}</button>
        <button type="button" class="pp-action" data-kbd-item="directions" @click="planHere">{{ t('directions') }}</button>
        <ShareButton/>
      </div>
    </header>

    <section data-kbd-section="departures" data-kbd-entry="1">
      <h2 class="pp-heading">{{ t('nextDepartures') }}</h2>
      <div class="pp-table" role="table">
        <div class="pp-thead" role="row">
          <span role="columnheader">{{ t('paperColLine') }}</span>
          <span role="columnheader">{{ t('paperColTowards') }}</span>
          <span role="columnheader">{{ t('paperColDue') }}</span>
        </div>
        <p v-if="isComputingDepartures" class="pp-empty">{{ t('paperComputing') }}</p>
        <p v-else-if="!shapesComingToTheStopBasedOnVehiclePositions.length" class="pp-empty">
          {{ t('noSchedule') }}
        </p>
        <div
          v-for="shape in isComputingDepartures ? [] : departuresSorted"
          :key="shape.route_short_name"
          class="pp-row"
          role="row"
          :data-kbd-item="`dep-${shape.route_short_name}`"
          v-bind="settings.busAlerts ? press.bind(shape) : {}"
          @click="press.click(() => navigateToRoute(shape))"
        >
          <span class="pp-cell pp-cell-line">
            <span class="pp-line" :style="{ '--line': shape.route_color }">{{ shape.route_short_name }}</span>
          </span>
          <span class="pp-cell pp-cell-dest">
            <span class="pp-dest">
              {{ routeDestination(shape.route_long_name) }}
              <span v-if="isDepartureFavorite(shape)" class="pp-fav-mark">♥</span>
              <span v-if="settings.busAlerts && isReminded(shape)" class="pp-fav-mark" :title="t('remindSaved')">⏰</span>
              <button v-if="settings.busAlerts" type="button" class="pp-follow" :aria-pressed="isRemindOn(shape)"
                      :aria-label="isRemindOn(shape) ? t('notifyMeOff') : t('remindSet')"
                      @click.stop="toggleRemind(shape)">⏰</button>
            </span>
            <span class="pp-from">{{ t('paperFrom') }} {{ routeOrigin(shape.route_long_name) }}</span>
          </span>
          <span class="pp-cell pp-cell-time">
            <span class="pp-time" :class="{ 'is-live': !shape.static_time_approximation }">
              {{ shape.static_time_approximation ? '~' : '' }}{{ formatMinutes(shape.next_times?.[0]?.minutes ?? shape.minutes_left) }}<sup v-if="!shape.static_time_approximation">*</sup>
            </span>
            <span v-if="(shape.next_times?.length ?? 0) > 1" class="pp-time-more">
              {{ shape.next_times!.slice(1).map((time) => formatMinutes(time.minutes)).join(' · ') }}
            </span>
          </span>
        </div>
      </div>
    </section>

    <section class="pb-6" data-kbd-section="stop-routes" data-kbd-entry="2">
      <h2 class="pp-heading">{{ t('allRoutesAtStop') }}</h2>
      <div class="pp-index">
        <div
          v-for="shape in busesWithAvailableTimetablesSorted"
          :key="shape.route_short_name"
          class="pp-index-row"
          :data-kbd-item="`route-${shape.route_short_name}`"
          @click="navigateToAllRoute(shape)"
        >
          <span class="pp-line pp-line-sm" :style="{ '--line': shape.route_color }">{{ shape.route_short_name }}</span>
          <span class="pp-index-name">
            {{ shape.timetable.route_long_name }}
            <span v-if="favoritesStore.isRouteFavorite(shape.route_id)" class="pp-fav-mark">♥</span>
          </span>
          <span class="pp-leader" aria-hidden="true"></span>
          <span class="pp-index-ref">{{ t('timetable') }}</span>
        </div>
      </div>
    </section>
    </template>

    <template v-else>
    <div class="flex items-center -mb-4">
      <HeaderNavigation />
    </div>

    <header class="flex items-start gap-4" data-kbd-section="actions" data-kbd-axis="x">
      <div
        class="w-14 h-14 shrink-0 rounded-2xl bg-gradient-to-br from-emerald-400 to-teal-600 flex items-center justify-center shadow-lg shadow-emerald-500/20 mt-0.5">
        <span v-if="settings.legacyBlueActive" class="emoji-icon-xl" aria-hidden="true">🚏</span>
        <StopIcon v-else class="w-7 h-7 text-white"/>
      </div>
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2 mb-0.5">
          <span
            class="text-[10px] font-semibold text-emerald-600 dark:text-emerald-400 tracking-wide">{{
              t('busStop')
            }}</span>
          <span v-if="stopInfo?.stop_code"
                class="text-[10px] font-semibold text-slate-400 dark:text-slate-500 font-mono tracking-wide">#{{
              stopInfo.stop_code
            }}</span>
        </div>
        <h1 class="text-lg font-bold text-slate-900 dark:text-white leading-snug">
          {{ stopName }}
        </h1>
      </div>
      <button
        type="button"
        class="directions-btn mt-1 shrink-0"
        :title="t('directions')"
        :aria-label="t('directions')"
        data-kbd-item="directions"
        @click="planHere"
      >
        <span v-if="settings.legacyBlueActive" class="emoji-icon" aria-hidden="true">🧭</span>
        <svg v-else class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
          <path d="M21.7 11.3l-9-9a1 1 0 0 0-1.4 0l-9 9a1 1 0 0 0 0 1.4l9 9a1 1 0 0 0 1.4 0l9-9a1 1 0 0 0 0-1.4zM14 14.5V12h-4v3H8v-4a1 1 0 0 1 1-1h5V7.5l3.5 3.5-3.5 3.5z"/>
        </svg>
      </button>
      <ShareButton class="mt-1"/>
      <button
        type="button"
        class="fav-btn mt-1 shrink-0"
        :class="{ 'is-fav': isFavorite }"
        :title="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-label="isFavorite ? t('removeFromFavorites') : t('addToFavorites')"
        :aria-pressed="isFavorite"
        data-kbd-item="fav"
        @click="favoritesStore.toggleStopFavorite(stopIdNum)"
      >
        <IconHeartFilled v-if="isFavorite" class="w-5 h-5"/>
        <IconHeartOutline v-else class="w-5 h-5"/>
      </button>
    </header>

    <section data-kbd-section="departures" data-kbd-entry="1">
      <h2 class="section-label">
        <span
          class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse shadow-[0_0_6px_rgba(16,185,129,0.6)] shrink-0"></span>
        {{ t('nextDepartures') }}
        <span v-if="settings.busAlerts" class="dep-hint">{{ pointerHint ? t('departuresClickHint') : t('departuresHoldHint') }}</span>
      </h2>

      <div v-if="isComputingDepartures" class="flex flex-col gap-2.5 animate-pulse">
        <div v-for="i in 3" :key="i"
             class="flex items-center gap-2.5 rounded-2xl p-3 border border-slate-100 dark:border-slate-800/50 bg-slate-50 dark:bg-slate-800/30">
          <div class="w-1 self-stretch rounded-full bg-slate-200 dark:bg-slate-700 shrink-0"></div>
          <div class="w-11 h-9 rounded-xl bg-slate-200 dark:bg-slate-700 shrink-0"></div>
          <div class="flex-1 flex flex-col gap-1.5">
            <div class="h-2.5 w-12 bg-slate-200 dark:bg-slate-700 rounded"></div>
            <div class="h-4 w-32 bg-slate-200 dark:bg-slate-700 rounded"></div>
          </div>
          <div class="flex gap-1">
            <div v-for="j in 3" :key="j"
                 class="w-10 h-6 rounded-lg bg-slate-200 dark:bg-slate-700"></div>
          </div>
          <div class="w-4 h-4 rounded bg-slate-100 dark:bg-slate-800 shrink-0"></div>
        </div>
      </div>
      <template v-else>
        <p v-if="!shapesComingToTheStopBasedOnVehiclePositions.length"
           class="text-sm text-slate-400 dark:text-slate-500 py-2">
          {{ t('noSchedule') }}
        </p>
        <div class="flex flex-col gap-2.5">
          <div
            v-for="shape in departuresSorted"
            :key="shape.route_short_name"
            v-bind="settings.busAlerts ? press.bind(shape) : {}"
            @click="press.click(() => navigateToRoute(shape))"
            class="departure-card group"
            :data-kbd-item="`dep-${shape.route_short_name}`"
            :class="{ 'departure-card-fav': isDepartureFavorite(shape) }"
          >
            <div
              :class="['w-1 self-stretch rounded-full shrink-0', !shape.static_time_approximation ? 'bg-emerald-500' : 'bg-transparent']"></div>

            <div
              class="flex items-center justify-center shrink-0 w-11 h-9 rounded-xl font-black text-sm text-white shadow-sm"
              :style="{ backgroundColor: shape.route_color }"
            >{{ shape.route_short_name }}
            </div>

            <div class="flex-1 min-w-0 flex flex-col justify-center">
              <div class="flex items-center gap-1.5 mb-0.5">
                <span v-if="!shape.static_time_approximation" class="live-badge">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
                  {{ t('live') }}
                </span>
                <span class="card-dest">→ {{ routeDestination(shape.route_long_name) }}</span>
                <svg v-if="settings.busAlerts && isReminded(shape)" class="w-3.5 h-3.5 shrink-0 text-emerald-500" fill="none" viewBox="0 0 24 24"
                     stroke="currentColor" stroke-width="2.2" :aria-label="t('remindSaved')">
                  <circle cx="12" cy="13" r="8"/><path stroke-linecap="round" d="M12 9v4l2 2M5 3L2 6m17-3l3 3"/>
                </svg>
              </div>
              <span class="card-origin">{{ routeOrigin(shape.route_long_name) }}</span>
            </div>

            <div class="flex items-center gap-1 shrink-0">
              <span
                v-for="(t, i) in shape.next_times"
                :key="i"
                :class="[
                  'time-pill',
                  i === 0 && !shape.static_time_approximation
                    ? 'time-pill-live'
                    : 'time-pill-sched',
                  i > 0 ? 'time-pill-extra' : '',
                  i > 1 ? 'time-pill-third' : ''
                ]"
              >{{
                  i === 0 && shape.static_time_approximation ? '~\u202f' : ''
                }}{{ formatMinutes(t.minutes) }}</span>
            </div>

            <button
              v-if="settings.busAlerts"
              type="button"
              class="dep-follow"
              :class="{ 'is-on': isRemindOn(shape) }"
              :aria-pressed="isRemindOn(shape)"
              :title="isRemindOn(shape) ? t('notifyMeOff') : t('remindSet')"
              :aria-label="isRemindOn(shape) ? t('notifyMeOff') : t('remindSet')"
              @click.stop="toggleRemind(shape)"
              @pointerdown.stop
            >
              <span v-if="settings.legacyBlueActive" aria-hidden="true">⏰</span>
              <svg v-else class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
                   stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="13" r="8"/>
                <path d="M5 3 2 6m17-3 3 3"/>
                <path v-if="isRemindOn(shape)" d="m9 13 2 2 4-4"/>
                <path v-else d="M12 10v6m-3-3h6"/>
              </svg>
            </button>
          </div>
        </div>
      </template>
    </section>

    <section class="pb-6" data-kbd-section="stop-routes" data-kbd-entry="2">
      <h2 class="section-label">
        <span v-if="settings.legacyBlueActive" class="emoji-icon" aria-hidden="true">🗺️</span>
        <svg v-else class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500 shrink-0" fill="none"
             viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round"
                d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
        </svg>
        {{ t('allRoutesAtStop') }}
      </h2>

      <div class="flex flex-col divide-y divide-slate-100 dark:divide-slate-800/60">
        <div
          v-for="shape in busesWithAvailableTimetablesSorted"
          :key="shape.route_short_name"
          @click="navigateToAllRoute(shape)"
          class="all-route-row group"
          :data-kbd-item="`route-${shape.route_short_name}`"
          :class="{ 'all-route-row-fav': favoritesStore.isRouteFavorite(shape.route_id) }"
        >
          <div
            class="flex items-center justify-center shrink-0 w-10 h-7 rounded-md text-xs font-black text-white shadow-sm opacity-90 group-hover:opacity-100 transition-opacity"
            :style="{ backgroundColor: shape.route_color }"
          >{{ shape.route_short_name }}
          </div>

          <span
            class="flex-1 text-sm font-medium text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
            {{ shape.timetable.route_long_name }}
          </span>

          <svg
            class="w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
          </svg>
        </div>
      </div>
    </section>
    </template>

    <BusMenu v-if="menu" :x="menu.x" :y="menu.y" :items="menu.items" @close="menu = null"/>
  </div>
</template>

<style scoped>
.stop-view-container {
  padding: 1.25rem 1.5rem 0;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

.section-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  margin-bottom: 0.875rem;
}

.departure-card {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.75rem 0.625rem 0.75rem 0.5rem;
  border-radius: 1rem;
  border: 1px solid #f1f5f9;
  background: #f8fafc;
  container-type: inline-size;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
}

.departure-card:hover {
  background: white;
  border-color: #e2e8f0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.live-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.625rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: #059669;
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  padding: 0.125rem 0.375rem;
  border-radius: 9999px;
}

.time-pill {
  font-size: 0.7rem;
  font-weight: 700;
  padding: 0.2rem 0.45rem;
  border-radius: 0.5rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.time-pill-live {
  background: #10b981;
  color: white;
}

.time-pill-sched {
  background: #f1f5f9;
  color: #475569;
}

/* Hide 2nd + 3rd pill when card is narrow */
@container (max-width: 380px) {
  .time-pill-third,
  .live-badge {
    display: none;
  }
}

@container (max-width: 300px) {
  .time-pill-extra {
    display: none;
  }
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

.departure-card-fav {
  background: #fff1f2;
  border-color: #fecdd3;
}

.departure-card-fav:hover {
  background: #ffe4e6;
  border-color: #fda4af;
}

.all-route-row-fav {
  background: #fff1f2;
}

.all-route-row-fav:hover {
  background: #ffe4e6 !important;
}

.card-dest {
  font-size: 0.8125rem;
  font-weight: 700;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.card-origin {
  font-size: 0.6875rem;
  font-weight: 500;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dep-hint {
  margin-left: auto;
  font-size: 0.625rem;
  font-weight: 500;
  color: var(--color-slate-400, #94a3b8);
}

.directions-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 9999px;
  color: var(--color-emerald-600, #059669);
  background: transparent;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, transform 0.15s;
}

.directions-btn:hover {
  background: var(--color-emerald-50, #ecfdf5);
}

.directions-btn:active {
  transform: scale(0.92);
}

:root.dark .directions-btn {
  color: var(--color-emerald-400, #34d399);
}

:root.dark .directions-btn:hover {
  background: rgb(16 185 129 / 0.15);
}

.retry-btn {
  padding: 0.5rem 1rem;
  border-radius: 9999px;
  font-size: 0.875rem;
  font-weight: 700;
  background: var(--color-emerald-100, #d1fae5);
  color: var(--color-emerald-800, #065f46);
}

.dep-follow {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  margin: -0.25rem -0.25rem -0.25rem 0;
  border-radius: 9999px;
  color: var(--color-slate-400, #94a3b8);
  flex-shrink: 0;
  transition: background 0.15s, color 0.15s, transform 0.15s;
}

.dep-follow:hover {
  background: var(--color-emerald-50, #ecfdf5);
  color: var(--color-emerald-600, #059669);
}

.dep-follow:active {
  transform: scale(0.9);
}

.dep-follow.is-on {
  color: var(--color-emerald-600, #059669);
  background: var(--color-emerald-100, #d1fae5);
}

:root.dark .dep-follow:hover {
  background: rgba(16, 185, 129, 0.15);
  color: var(--color-emerald-400, #34d399);
}

:root.dark .dep-follow.is-on {
  color: var(--color-emerald-400, #34d399);
  background: rgba(16, 185, 129, 0.2);
}

:root.dark .retry-btn {
  background: rgba(16, 185, 129, 0.2);
  color: var(--color-emerald-300, #6ee7b7);
}

.pp-follow {
  margin-left: 0.25rem;
  font-size: 0.85em;
}

.pp-follow[aria-pressed="false"] {
  opacity: 0.4;
}

.fav-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 9999px;
  color: #94a3b8;
  background: transparent;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, transform 0.15s;
}

.fav-btn:hover {
  background: #fef2f2;
  color: #f43f5e;
}

.fav-btn:active {
  transform: scale(0.92);
}

.fav-btn.is-fav {
  color: #f43f5e;
}

.fav-btn.is-fav:hover {
  background: #fee2e2;
}

</style>

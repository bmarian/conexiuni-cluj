<script setup lang="ts">
import {computed, defineAsyncComponent, nextTick, ref, toRef, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import type {HourlyStopOffsets} from '@/types/tranzy.ts'
import type {DaySchedule, Frequency} from '@/types/ctp.ts'
import {directedRouteName, getTimetableDayKey, plainTimetableDayKey, specialDayOn, timeStringToMinutes} from '@/utils/time.ts'
import {frequencyLabel} from '@/utils/frequencyText.ts'
import {type IndexedStop, useRouteArrivals} from '@/composables/useRouteArrivals.ts'
import {fetchHourlyStopOffsets} from '@/composables/useRouteShapeInfoApi.ts'
import LoadingIndicator from '@/components/LoadingIndicator.vue'
import {useSettingsStore} from '@/stores/settings.ts'
import {MAX_FOLLOWED_LINES, type RouteChange, useRouteUpdatesStore} from '@/stores/routeUpdates.ts'
import RouteChangeBanner from '@/components/RouteChangeBanner.vue'
import {useRoute, useRouter} from 'vue-router'
import {servedName} from '@/utils/servedName.ts'
import {useKbdEscape, useKbdShortcuts, useKeyboardNav} from '@/composables/useKeyboardNav.ts'
import {canonicalUrl} from '@/utils/site.ts'
import {cityParams} from '@/utils/city.ts'
import {lineCanonicalPath, linePath, stopLocation, stopPath, type Direction} from '@/utils/pagePaths.ts'
import {useShare} from '@/composables/useShare.ts'
import {adminAuthed, lineStatsOn} from '@/composables/useAdmin.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ActionChips from '@/components/ui/ActionChips.vue'
import ChoiceRow from '@/components/ui/ChoiceRow.vue'
import DirectionArrow from '@/components/ui/DirectionArrow.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import StopLine, {type StopMark} from '@/components/rows/StopLine.vue'
import StopListBuses from '@/components/rows/StopListBuses.vue'
import {useListFlip} from '@/composables/useListFlip.ts'
import {directionKeys} from '@/utils/stopPairs.ts'
import {network} from '@/utils/network.ts'
import {displayColor} from '@/utils/lineColor.ts'
import {modeOf} from '@/utils/modes.ts'
import type {ChipAction, Choice, MenuAction} from '@/components/ui/types.ts'
import {
  ArrowBackIcon,
  ArrowForwardIcon,
  CloseIcon,
  FavoriteBorderIcon,
  FavoriteIcon,
  NotificationsIcon,
  NotificationsNoneIcon,
  ShareIcon,
} from '@/utils/icons.ts'

const props = defineProps<{routeId: string; direction: string}>()

const AdminLineStats = defineAsyncComponent(() => import('@/components/admin/AdminLineStats.vue'))
const showLineStats = adminAuthed() && lineStatsOn()

const {t} = useI18n()
const favoritesStore = useFavoritesStore()
const settings = useSettingsStore()
const routeUpdates = useRouteUpdatesStore()
const router = useRouter()
const currentRoute = useRoute()
const servedAs = computed(() => servedName(currentRoute.path))
const {userTime} = storeToRefs(useUserStore())
const {
  shapeInfo,
  fromStopId,
  fromStopName,
  currentDirection,
  isOutgoing,
  currentTripId,
  timetable,
  stopsForDirection,
  stopsByDirection,
  directionTerminals,
  directionHeadings,
  hasOutgoing,
  hasIncoming,
  nearestStopIdx,
  currentMinutes,
  stopTimesByStop,
  directionPath,
  stopsOnPath,
  listVehicles,
  getHeaderTimes,
  isInitialLoading,
} = useRouteArrivals(toRef(props, 'routeId'), toRef(props, 'direction'))

const routeIdNum = computed(() => Number(props.routeId))

const isFavorite = computed(() => favoritesStore.isRouteFavorite(routeIdNum.value, currentDirection.value))
const isFollowing = computed(() => !!shapeInfo.value && routeUpdates.isFollowing(shapeInfo.value.route_short_name))

function toggleFollow() {
  const name = shapeInfo.value?.route_short_name
  if (!name) return
  const following = routeUpdates.toggleFollow(name)
  if (following === null) {
    settings.showToast(t('limitReached', {n: MAX_FOLLOWED_LINES}), {icon: 'bell-off'})
    return
  }
  settings.showToast(t(following ? 'followLineToast' : 'unfollowLineToast', {route: name}), {
    body: t(following ? 'followLineToastBody' : 'unfollowLineToastBody'),
    icon: following ? 'bell' : 'bell-off',
  })
}

const activeChange = ref<RouteChange | null>(null)

// Direction switches replace the URL without the query, so only a new id replaces the banner.
watch(
  () => currentRoute.query.change,
  async (raw) => {
    const id = Number(raw)
    if (!Number.isInteger(id) || id <= 0) return
    const change = await routeUpdates.loadChange(id)
    if (change) {
      activeChange.value = change
      routeUpdates.markSeen([change.id])
    }
  },
  {immediate: true},
)

function dismissChange() {
  activeChange.value = null
  if (currentRoute.query.change) {
    void router.replace(linePath(props.routeId, currentDirection.value))
  }
}

useHead(() => {
  const shortName = shapeInfo.value?.route_short_name ?? ''
  // Keep the server-injected meta for crawlers until the route loads.
  if (!shortName) return {}
  const longName = timetable.value?.route_long_name ?? ''
  const title = t('headRouteTitle', {...cityParams(), shortName, longName: longName ? ` — ${directedRouteName(longName, currentDirection.value)}` : ''})
  const description = t('headRouteDesc', {...cityParams(), shortName})
  const url = canonicalUrl(lineCanonicalPath(props.routeId, props.direction as Direction))
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

function formatAbsoluteMinutes(absMin: number): string {
  const dayMin = ((Math.round(absMin) % 1440) + 1440) % 1440
  const h = Math.floor(dayMin / 60)
  const m = dayMin % 60
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`
}

function getStopLabel(idx: number, stop: IndexedStop): string {
  return stop.stop_headsign || `Stop ${idx + 1}`
}

type TimetableTab = 'weekdays' | 'saturday' | 'sunday'

const todayTab = computed((): TimetableTab => getTimetableDayKey(userTime.value || new Date()))

const pickedTimetableTab = ref<TimetableTab>(todayTab.value)

watch(todayTab, (tab, previous) => {
  if (pickedTimetableTab.value === previous) pickedTimetableTab.value = tab
})

const plainTodayTab = computed((): TimetableTab => plainTimetableDayKey(userTime.value || new Date()))

const todayNotice = computed(() => {
  const special = specialDayOn(userTime.value || new Date())
  if (!special) return ''
  const timetable = t(`timetableName_${special.schedule}`)
  const holiday = special.note?.holiday
  return holiday ? t('specialDayHoliday', {holiday: t(`holiday_${holiday}`), timetable}) : t('specialDayChanged', {timetable})
})

const timetableTabOrder: Record<TimetableTab, number> = {
  weekdays: 0,
  saturday: 1,
  sunday: 2,
}

const isPastTab = (tab: TimetableTab): boolean => timetableTabOrder[tab] < timetableTabOrder[plainTodayTab.value]

const dayHasContent = (d?: DaySchedule | null): boolean => !!(d?.entries?.length || d?.in_frequency || d?.out_frequency)

const availableTabs = computed(() => {
  const tt = timetable.value
  if (!tt) return []
  const tabs: Array<{key: TimetableTab; label: string}> = []
  if (dayHasContent(tt.weekdays)) tabs.push({key: 'weekdays', label: t('weekdays')})
  if (dayHasContent(tt.saturday)) tabs.push({key: 'saturday', label: t('saturday')})
  if (dayHasContent(tt.sunday)) tabs.push({key: 'sunday', label: t('sunday')})
  return tabs
})

// A line with no service on the picked day shows the next day that has some.
const selectedTimetableTab = computed<TimetableTab>({
  get: () => {
    const picked = pickedTimetableTab.value
    const keys = availableTabs.value.map((tab) => tab.key)
    if (!keys.length || keys.includes(picked)) return picked
    return keys.find((key) => timetableTabOrder[key] > timetableTabOrder[picked]) ?? keys[0]!
  },
  set: (tab) => {
    pickedTimetableTab.value = tab
  },
})

// Headway directions (e.g. M26) list no departures, so show the frequency card.
const activeFrequency = computed((): Frequency | null => {
  const tt = timetable.value
  if (!tt) return null
  const sched = selectedTimetableTab.value === 'sunday' ? tt.sunday : selectedTimetableTab.value === 'saturday' ? tt.saturday : tt.weekdays
  return (isOutgoing.value ? sched?.in_frequency : sched?.out_frequency) ?? null
})

const hourlyOffsets = ref<Record<string, HourlyStopOffsets>>({})

watch(
  [() => shapeInfo.value?.route_short_name, selectedTimetableTab],
  async ([name, tab]) => {
    if (!name) return
    const data = await fetchHourlyStopOffsets(name, tab === 'weekdays' ? 'weekday' : tab)
    if (name === shapeInfo.value?.route_short_name && tab === selectedTimetableTab.value) hourlyOffsets.value = data
  },
  {immediate: true},
)

const hourlyTripOffsets = computed((): Record<string, number[]> | null => {
  const trip = hourlyOffsets.value[currentTripId.value]
  const stops = stopsForDirection.value
  if (!trip || trip.stop_ids.length !== stops.length) return null
  return trip.stop_ids.every((id, i) => id === stops[i]!.stop_id) ? trip.hourly_offset_seconds : null
})

// Shared by the chips and the trip view so both show the same minute.
function tripOffsetsAt(departureMin: number): number[] {
  const seconds = hourlyTripOffsets.value?.[Math.floor(departureMin / 60) % 24]
  return (seconds ?? stopsForDirection.value.map((stop) => stop.secondsFromStart)).map((s) => s / 60)
}

// time is CTP's printed departure and identifies the run; startMin adds the first stop's learned delay.
type TimetableChip = {time: string; startMin: number | null; isPast: boolean}
type GridChip = TimetableChip & {startMin: number}

const timetableEntries = computed((): TimetableChip[] => {
  const tt = timetable.value
  const day = selectedTimetableTab.value
  const sched = day === 'sunday' ? tt?.sunday : day === 'saturday' ? tt?.saturday : tt?.weekdays
  if (!sched?.entries?.length) return []
  const isToday = day === todayTab.value
  const dayIsPast = isPastTab(day)
  const now = currentMinutes.value
  return sched.entries
    .map((entry): TimetableChip | null => {
      const raw = (isOutgoing.value ? entry.departure_in : entry.departure_out)?.trim()
      if (!raw) return null
      const departureMin = timeStringToMinutes(raw)
      if (departureMin === null) return {time: raw, startMin: null, isPast: false}
      const startMin = Math.round(departureMin + (tripOffsetsAt(departureMin)[0] ?? 0))
      return {
        time: raw,
        startMin,
        isPast: dayIsPast || (isToday && startMin < now),
      }
    })
    .filter((e): e is TimetableChip => e !== null)
})

const allEntriesSuspended = computed(() => timetableEntries.value.length > 0 && timetableEntries.value.every((e) => e.startMin === null))

type HourGroup = {hour: string; chips: GridChip[]; isNextDay: boolean}

const timetableByHour = computed((): HourGroup[] => {
  const groups = new Map<number, GridChip[]>()
  const chips = timetableEntries.value.filter((chip): chip is GridChip => chip.startMin !== null).sort((a, b) => a.startMin - b.startMin)
  for (const chip of chips) {
    const h = Math.floor(chip.startMin / 60)
    if (!groups.has(h)) groups.set(h, [])
    groups.get(h)!.push(chip)
  }
  return Array.from(groups.entries()).map(([h, chips]) => ({
    hour: formatAbsoluteMinutes(h * 60).slice(0, 2),
    chips,
    isNextDay: h >= 24,
  }))
})

const selectedDepartureTime = ref<string | null>(null)
const tripViewRef = ref<HTMLElement | null>(null)

function selectDeparture(entry: TimetableChip) {
  selectedDepartureTime.value = selectedDepartureTime.value === entry.time ? null : entry.time
}

watch(selectedDepartureTime, async (val) => {
  if (!val) return
  await nextTick()
  tripViewRef.value?.scrollIntoView({behavior: 'smooth', block: 'nearest'})
})

watch([currentDirection, selectedTimetableTab], () => {
  selectedDepartureTime.value = null
})

type TripStop = IndexedStop & {arrivalTimeStr: string}

const selectedDepartureStops = computed((): TripStop[] => {
  if (!selectedDepartureTime.value) return []
  const departureMin = timeStringToMinutes(selectedDepartureTime.value)
  if (departureMin === null) return []
  const offsets = tripOffsetsAt(departureMin)
  return stopsForDirection.value.map((stop, idx) => {
    const offset = offsets[idx] ?? stop.secondsFromStart / 60
    return {...stop, arrivalTimeStr: formatAbsoluteMinutes(departureMin + offset)}
  })
})

const selectedDepartureTimeDisplay = computed(() => selectedDepartureStops.value[0]?.arrivalTimeStr ?? null)

function onDirClick(dir: '0' | '1') {
  router.replace(linePath(props.routeId, dir))
  currentDirection.value = dir
}

const kbd = useKeyboardNav()

function switchDirection() {
  const next = currentDirection.value === '0' ? '1' : '0'
  if (next === '0' ? !hasOutgoing.value : !hasIncoming.value) return
  const key = kbd.focusedKey()
  onDirClick(next)
  void nextTick(() => {
    const onStop = key?.startsWith('stop-')
    if (onStop && kbd.focusKey(key!)) return
    if (onStop || !kbd.focusedKey()) kbd.focusEntry()
  })
}

const nextChipTime = computed(() => {
  for (const group of timetableByHour.value) {
    const chip = group.chips.find((c) => !c.isPast)
    if (chip) return chip.time
  }
  return null
})

watch(selectedDepartureTime, (time, prev) => {
  if (!kbd.keyboardMode.value) return
  const seq = kbd.navSeq()
  void nextTick(() => {
    if (kbd.navSeq() !== seq) return
    if (time) kbd.focusSection('trip')
    else if (prev && !kbd.focusedKey()) kbd.focusKey(`min-${prev}`)
  })
})

useKbdEscape(
  () => selectedDepartureTime.value !== null,
  () => {
    selectedDepartureTime.value = null
  },
)

useKbdShortcuts({
  f: () => favoritesStore.toggleRouteFavorite(routeIdNum.value, currentDirection.value),
  b: toggleFollow,
  d: switchDirection,
  t: () => kbd.focusSection('tt-minutes') || kbd.focusSection('tt-tabs'),
})

const {share} = useShare()

const barTitle = computed(() =>
  shapeInfo.value ? `${shapeInfo.value.route_short_name} · ${timetable.value?.route_long_name || shapeInfo.value.route_short_name}` : null,
)

const topMenu = computed<MenuAction[]>(() =>
  shapeInfo.value
    ? [
        {
          label: t(isFollowing.value ? 'followingChanges' : 'followChanges'),
          icon: isFollowing.value ? NotificationsIcon : NotificationsNoneIcon,
          onClick: toggleFollow,
        },
      ]
    : [],
)

const chips = computed<ChipAction[]>(() => [
  {
    label: t(isFavorite.value ? 'saved' : 'save'),
    icon: isFavorite.value ? FavoriteIcon : FavoriteBorderIcon,
    selected: isFavorite.value,
    tint: isFavorite.value ? 'var(--md-rose)' : undefined,
    kbd: 'fav',
    onClick: () => favoritesStore.toggleRouteFavorite(routeIdNum.value, currentDirection.value),
  },
  {label: t('share'), icon: ShareIcon, kbd: 'share', onClick: () => void share()},
])

const directionChoices = computed<Choice<'0' | '1'>[]>(() => [
  {value: '0', label: directionTerminals.value.outgoing.last || timetable.value?.out_stop_name || '', disabled: !hasOutgoing.value},
  {value: '1', label: directionTerminals.value.incoming.last || timetable.value?.in_stop_name || '', disabled: !hasIncoming.value},
])

const tabChoices = computed<Choice<TimetableTab>[]>(() => availableTabs.value.map((tab) => ({value: tab.key, label: tab.label})))

function stopMark(idx: number, stop: IndexedStop, count: number): StopMark {
  if (String(stop.stop_id) === fromStopId.value) return 'from'
  if (idx === nearestStopIdx.value) return 'nearest'
  if (favoritesStore.isStopFavorite(stop.stop_id)) return 'favorite'
  if (idx === 0 || idx === count - 1) return 'terminal'
  return 'plain'
}

const keysByDirection = computed(() =>
  directionKeys(stopsByDirection.value['0'], stopsByDirection.value['1'], (stop) => network.value?.stationId(stop.stop_id) ?? stop.stop_id),
)
const stopKeys = computed(() => keysByDirection.value[currentDirection.value === '0' ? 0 : 1])

const stopsList = ref<HTMLElement>()
const flip = useListFlip(stopsList, stopsForDirection)

const busTarget = computed(() => {
  const from = stopsForDirection.value.findIndex((stop) => String(stop.stop_id) === fromStopId.value)
  return from === -1 ? nearestStopIdx.value : from
})

const lineColor = computed(() => (shapeInfo.value ? displayColor(shapeInfo.value.route_short_name, shapeInfo.value.route_color) : ''))

const stopHref = (stop: IndexedStop) => stopPath(stop.stop_id)
const openStop = (stop: IndexedStop) => router.push(stopLocation(stop.stop_id))
</script>

<template>
  <div class="route-view">
    <SheetTopBar :title="barTitle" :menu="topMenu" />

    <LoadingIndicator v-if="isInitialLoading || !shapeInfo" :text="servedAs ? t('loadingRouteNamed', {name: servedAs}) : t('loadingRoute')" />

    <template v-else>
      <RouteChangeBanner v-if="activeChange" :change="activeChange" @dismiss="dismissChange" />

      <header class="header">
        <LineBadge :name="shapeInfo.route_short_name" :color="shapeInfo.route_color" :height="52" :min-width="52" :corner="16" />
        <div class="flex-1 min-w-0">
          <h1>
            <span class="block text-label-medium text-on-surface-variant">{{ t('routeNamed', {name: shapeInfo.route_short_name}) }}</span>
            <span class="block text-title-large font-bold">{{ timetable?.route_long_name || shapeInfo.route_short_name }}</span>
          </h1>
          <p v-if="fromStopName" class="flex items-center gap-1.5 pt-1 text-body-small text-on-surface-variant">
            <span class="size-1.5 rounded-full bg-[#10b981] shrink-0"></span>
            <span class="min-w-0">{{ t('from', {name: fromStopName}) }}</span>
          </p>
        </div>
      </header>
      <ActionChips :actions="chips" class="pb-3" />

      <ChoiceRow :options="directionChoices" :model-value="currentDirection" kbd="dir" class="mx-4" data-kbd-entry="3" @update:model-value="onDirClick">
        <template #label="{option, checked}">
          <DirectionArrow v-if="option.value === '0'" :icon="ArrowForwardIcon" :rest="90" :heading="directionHeadings['0']" :on="checked" />
          <span class="truncate">{{ option.label }}</span>
          <DirectionArrow v-if="option.value === '1'" :icon="ArrowBackIcon" :rest="270" :heading="directionHeadings['1']" :on="checked" />
        </template>
      </ChoiceRow>

      <AdminLineStats v-if="showLineStats" :route-id="routeId" :direction="currentDirection">
        <template #head="{day}">
          <div class="tt-head">
            <span class="text-title-small text-on-surface-variant">Line stats</span>
            <span class="divider"></span>
            <span class="text-label-small text-outline">{{ day }}</span>
          </div>
        </template>
      </AdminLineStats>

      <p v-if="!stopsForDirection.length" class="text-body-medium text-on-surface-variant px-4 pt-4">{{ t('noSchedule') }}</p>

      <template v-else>
        <div class="stops-head">
          <span class="flex flex-1 items-center min-w-0">
            <span class="text-title-small text-on-surface-variant">{{ t('stops') }}</span>
            <span class="divider"></span>
          </span>
          <span class="flex flex-col items-end">
            <span class="next-head">{{ t('nextDepartures') }}</span>
            <span class="flex gap-1">
              <span v-for="(time, i) in getHeaderTimes()" :key="i" class="head-time">{{ time }}</span>
            </span>
          </span>
        </div>

        <div ref="stopsList" class="stops" :style="{'--paint': lineColor}" data-kbd-section="stops">
          <TransitionGroup name="flip" enter-active-class="cascade" appear @before-leave="flip.pin">
            <StopLine
              v-for="(stop, idx) in stopsForDirection"
              :key="stopKeys[idx]"
              :style="{'--i': idx}"
              :name="getStopLabel(idx, stop)"
              :mark="stopMark(idx, stop, stopsForDirection.length)"
              :favorite="favoritesStore.isStopFavorite(stop.stop_id)"
              :first="idx === 0"
              :last="idx === stopsForDirection.length - 1"
              :times="stopTimesByStop[idx]"
              :data-kbd-item="`stop-${stop.stop_id}`"
              :data-kbd-entry="String(stop.stop_id) === fromStopId ? 1 : idx === nearestStopIdx ? 2 : undefined"
              :href="stopHref(stop)"
              paint
              @open="openStop(stop)"
            />
          </TransitionGroup>
          <StopListBuses
            :path="directionPath"
            :stops-at="stopsOnPath"
            :vehicles="listVehicles"
            :target="busTarget"
            :color="lineColor"
            :mode="modeOf(shapeInfo.route_type)"
          />
        </div>

        <template v-if="availableTabs.length">
          <div class="tt-head">
            <span class="text-title-small text-on-surface-variant">{{ t('timetable') }}</span>
            <span class="divider"></span>
            <span class="text-label-small text-outline">{{ t('timetableClickHint') }}</span>
          </div>

          <ChoiceRow v-model="selectedTimetableTab" :options="tabChoices" kbd="tab" class="mx-4" data-kbd-section="tt-tabs">
            <template #label="{option, checked}">
              <span class="truncate" :class="{'text-outline': isPastTab(option.value) && !checked}">{{ option.label }}</span>
              <span v-if="option.value === todayTab" class="today-dot"></span>
            </template>
          </ChoiceRow>

          <p v-if="todayNotice" class="flex items-center gap-1.5 px-4 pt-2 text-label-small text-on-surface-variant">
            <span class="today-dot ml-0!"></span>{{ todayNotice }}
          </p>

          <div v-if="allEntriesSuspended" class="suspended">{{ t('serviceSuspended') }}</div>

          <div v-if="activeFrequency" class="frequency">
            <span class="w-8 text-title-large text-primary">↻</span>
            <span class="flex flex-col">
              <span v-if="activeFrequency.min_minutes || activeFrequency.max_minutes" class="text-title-small">{{ frequencyLabel(activeFrequency) }}</span>
              <span v-if="activeFrequency.start" class="text-body-small text-on-surface-variant">
                {{ t('frequencyWindow', {start: activeFrequency.start, end: activeFrequency.end}) }}
              </span>
            </span>
          </div>

          <div v-if="timetableByHour.length" class="pt-2" data-kbd-section="tt-minutes" data-kbd-axis="grid">
            <div v-for="group in timetableByHour" :key="group.hour" class="hour-row">
              <span class="hour" :class="{'is-gone': group.isNextDay || group.chips.every((c) => c.isPast)}">{{ group.hour }}</span>
              <span class="minutes">
                <button
                  v-for="chip in group.chips"
                  :key="chip.time"
                  type="button"
                  class="minute squish"
                  :class="{
                    'is-selected': selectedDepartureTime === chip.time,
                    'is-past': chip.isPast,
                    'is-next': !selectedDepartureTime && nextChipTime === chip.time,
                  }"
                  :data-kbd-item="`min-${chip.time}`"
                  :data-kbd-active="selectedDepartureTime ? selectedDepartureTime === chip.time : nextChipTime === chip.time"
                  @click="selectDeparture(chip)"
                >
                  {{ formatAbsoluteMinutes(chip.startMin).slice(3) }}
                </button>
              </span>
            </div>
          </div>

          <div v-if="selectedDepartureTime && selectedDepartureStops.length" ref="tripViewRef" class="pt-5">
            <div class="trip-head" data-kbd-section="trip-head" data-kbd-axis="x">
              <span class="flex-1 text-title-small text-on-surface-variant">{{ t('tripAt', {time: selectedDepartureTimeDisplay}) }}</span>
              <button
                type="button"
                class="icon-btn"
                :aria-label="t('closeTripView')"
                :title="t('closeTripView')"
                data-kbd-item="trip-close"
                @click="selectedDepartureTime = null"
              >
                <CloseIcon class="m-icon" />
              </button>
            </div>
            <div data-kbd-section="trip">
              <StopLine
                v-for="(stop, idx) in selectedDepartureStops"
                :key="`${stop.stop_id}-trip`"
                :name="getStopLabel(idx, stop)"
                :mark="stopMark(idx, stop, selectedDepartureStops.length)"
                :favorite="favoritesStore.isStopFavorite(stop.stop_id)"
                :first="idx === 0"
                :last="idx === selectedDepartureStops.length - 1"
                :trailing="stop.arrivalTimeStr"
                :trailing-highlight="idx === 0"
                :data-kbd-item="`trip-stop-${stop.stop_id}`"
                :href="stopHref(stop)"
                @open="openStop(stop)"
              />
            </div>
          </div>
        </template>
      </template>
      <div class="h-8"></div>
    </template>
  </div>
</template>

<style scoped>
.header {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 4px 4px 12px 16px;
}

.stops-head {
  display: flex;
  align-items: flex-end;
  padding: 20px 16px 6px;
}

.stops {
  position: relative;
}

.flip-move {
  transition: transform var(--spring-slide-ms) var(--spring-slide);
}

.flip-leave-active {
  position: absolute;
  left: 0;
  right: 0;
  transition: opacity 140ms linear;
}

.flip-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .flip-move,
  .flip-leave-active {
    transition: none;
  }
}

.next-head {
  font-size: 9px;
  line-height: 14px;
  font-weight: 500;
  letter-spacing: 1px;
  text-transform: uppercase;
  color: var(--md-on-surface-variant);
}

.head-time {
  width: 44px;
  font-size: 13px;
  line-height: 20px;
  font-weight: 600;
  text-align: right;
  color: var(--md-outline);
  white-space: nowrap;
}

.today-dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  margin-left: 6px;
  border-radius: 50%;
  background: currentColor;
}

.suspended {
  margin: 16px;
  padding: 12px;
  border-radius: 12px;
  background: var(--md-error-container);
  color: var(--md-on-error-container);
}

.frequency {
  display: flex;
  gap: 12px;
  padding: 8px 16px;
}

.hour-row {
  display: flex;
  align-items: stretch;
  padding: 0 16px;
}

.hour {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-shrink: 0;
  width: 36px;
  padding-right: 10px;
  border-right: 2px solid var(--md-outline-variant);
  font-size: 16px;
  line-height: 24px;
  font-weight: 700;
  color: var(--md-primary);
}

.hour.is-gone {
  color: var(--md-outline-variant);
}

/* 48px touch target */
.minutes {
  display: grid;
  flex: 1;
  grid-template-columns: repeat(auto-fill, 48px);
  grid-auto-rows: 48px;
  place-items: center;
  gap: 6px;
  min-width: 0;
  padding: 4px 0 4px 10px;
}

.minute {
  --squish: 0.88;
  padding: 6px 10px;
  border: 0;
  border-radius: 8px;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface);
  font-size: 14px;
  line-height: 20px;
  font-weight: 700;
  letter-spacing: 0.1px;
  cursor: pointer;
  transition:
    border-radius var(--spring-fast-ms) var(--spring-fast),
    scale var(--spring-press-ms) var(--spring-press),
    background-color 130ms linear;
}

.minute.is-past {
  background: transparent;
  color: var(--md-outline-variant);
  font-weight: 400;
}

.minute.is-next {
  box-shadow: inset 0 0 0 1.5px var(--md-primary);
}

.minute.is-selected {
  border-radius: 16px;
  background: var(--md-primary);
  color: var(--md-on-primary);
  font-weight: 700;
}

.trip-head {
  display: flex;
  align-items: center;
  padding: 0 8px 0 16px;
}
</style>

<script setup lang="ts">
import {computed, nextTick, onMounted} from 'vue'
import {useHead} from '@unhead/vue'
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import Draggable from 'vuedraggable'
import {useFavoritesStore} from '@/stores/favorites.ts'
import type {FavoritePlan, FavoriteRoute} from '@/stores/favorites.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import {useOnline} from '@/composables/useOnline.ts'
import {useOpenRoute} from '@/composables/useOpenRoute.ts'
import type {Route, RouteDirection, Stop} from '@/types/tranzy.ts'
import {network} from '@/utils/network.ts'
import FollowCard from '@/components/FollowCard.vue'
import JourneyCard from '@/components/JourneyCard.vue'
import StationIcon from '@/components/StationIcon.vue'
import StationLines from '@/components/rows/StationLines.vue'
import SubLabel from '@/components/ui/SubLabel.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import RowIcon from '@/components/ui/RowIcon.vue'
import ListRow from '@/components/ui/ListRow.vue'
import {CloseIcon, MapIcon, ScheduleIcon} from '@/utils/icons.ts'
import {useFollowsStore} from '@/stores/follows.ts'
import {useJourneyStore} from '@/stores/journey.ts'
import {useRemindersStore} from '@/stores/reminders.ts'
import {useUserStore} from '@/stores/user.ts'
import {nextOf, type Reminder} from '@/utils/reminders.ts'
import {lineLabel} from '@/utils/journey.ts'
import {reminderRepeat, reminderWhen} from '@/utils/reminderText.ts'
import {planQuery} from '@/utils/planQuery.ts'
import {useKbdShortcuts, useKeyboardNav} from '@/composables/useKeyboardNav.ts'
import {siteUrl} from '@/utils/site.ts'
import {cityParams, cityPath} from '@/utils/city.ts'

const RECENTS_SHOWN = 5
const LONG_PRESS_MS = 350

const {t, locale} = useI18n()
const router = useRouter()
const followsStore = useFollowsStore()
const journeyStore = useJourneyStore()
const remindersStore = useRemindersStore()
const {userTime} = storeToRefs(useUserStore())

const reminderRows = computed(() => {
  const nowMs = (userTime.value ?? new Date()).getTime()
  return remindersStore.reminders.map((reminder) => {
    const next = nextOf(reminder, nowMs)
    return {
      reminder,
      title: `${next !== null ? reminderWhen(next, locale.value, nowMs) : t('remindNoBus')} → ${reminder.destination}`,
      kicker: [
        reminder.journey?.legs.map(lineLabel).join(' › '),
        reminder.time === null ? t('remindLastBus') : null,
        reminderRepeat(reminder.days, locale.value),
        reminder.stopName,
      ].filter(Boolean).join(' · '),
    }
  })
})

useHead(() => ({
  title: t('headHomeTitle', cityParams()),
  meta: [
    {name: 'description', content: t('headHomeDesc', cityParams())},
    {property: 'og:title', content: t('headHomeTitle', cityParams())},
    {property: 'og:description', content: t('headHomeDesc', cityParams())},
    {property: 'og:url', content: siteUrl(cityPath('/'))},
    {name: 'twitter:title', content: t('headHomeTitle', cityParams())},
    {name: 'twitter:description', content: t('headHomeDesc', cityParams())},
  ],
  link: [{rel: 'canonical', href: siteUrl(cityPath('/'))}],
}))
const favoritesStore = useFavoritesStore()
const settings = useSettingsStore()
const {favoriteRoutes, favoriteStopIds, favoritePlans, recentPlans, isHydrated} = storeToRefs(favoritesStore)

const {routes, fetchRoutes} = useRoutesApi()
const {stops, fetchStops} = useStopsApi()
const {isOnline} = useOnline()
const {openingRouteId, openRoute} = useOpenRoute()

const routesById = computed(() => new Map(routes.value.map((r) => [r.route_id, r])))
const stopsById = computed(() => new Map(stops.value.map((s) => [s.stop_id, s])))

onMounted(() => {
  void fetchRoutes()
  void fetchStops()
})

type FavoriteRouteGroup = {
  routeId: number
  favorites: FavoriteRoute[]
}

type FavoriteRouteChip = {
  route: Route
  label: string
  name: string
  direction?: RouteDirection
}

const favoriteRouteGroups = computed<FavoriteRouteGroup[]>(() => {
  const groups = new Map<number, FavoriteRouteGroup>()
  for (const fav of favoriteRoutes.value) {
    const group = groups.get(fav.routeId)
    if (group) group.favorites.push(fav)
    else groups.set(fav.routeId, {routeId: fav.routeId, favorites: [fav]})
  }
  return [...groups.values()]
})

// Both directions saved show as one chip that opens the direction nearest the user.
const favoriteRouteChips = computed(() => {
  const chips = new Map<number, FavoriteRouteChip>()
  for (const group of favoriteRouteGroups.value) {
    const route = routesById.value.get(group.routeId)
    if (!route) continue
    const longName = route.route_long_name
    const i = longName.lastIndexOf(' - ')
    const origin = i >= 0 ? longName.slice(0, i) : ''
    const end = i >= 0 ? longName.slice(i + 3) : longName
    const single = group.favorites.length === 1 ? group.favorites[0]! : null
    let chip: FavoriteRouteChip
    if (!single) chip = {route, label: origin ? `${origin} ⇄ ${end}` : longName, name: longName}
    else if (single.direction === '1' && origin) chip = {route, label: `→ ${origin}`, name: `${end} - ${origin}`, direction: '1'}
    else chip = {route, label: `→ ${end}`, name: longName, direction: single.direction}
    chips.set(group.routeId, chip)
  }
  return chips
})

const favoriteStops = computed<Stop[]>(() => favoriteStopIds.value
  .map((id) => stopsById.value.get(id))
  .filter((s): s is Stop => !!s))

const hasFavorites = computed(() =>
  favoriteRoutes.value.length > 0 ||
  favoriteStopIds.value.length > 0 ||
  (isOnline.value && favoritePlans.value.length > 0)
)

const recentNonFavoritePlans = computed<FavoritePlan[]>(() => recentPlans.value
  .filter((p) => !favoritesStore.isPlanFavorite(p.lat, p.lon, p.originLat, p.originLon))
  .slice(0, RECENTS_SHOWN))

const lookOf = (stopId: number) => network.value?.look(stopId)

function openReminder(reminder: Reminder) {
  if (reminder.journey?.planPath) void router.push(reminder.journey.planPath)
  else remindersStore.openSheet(reminder)
}

function openStop(stop: Stop) {
  void router.push({name: 'stop', params: {stopId: String(stop.stop_id)}})
}

function openPlan(plan: FavoritePlan) {
  if (isOnline.value) void router.push({name: 'plan', query: planQuery(plan)})
}

function removeRouteGroup(group: FavoriteRouteGroup) {
  for (const fav of group.favorites) favoritesStore.toggleRouteFavorite(fav.routeId, fav.direction)
}

const routeFavoritesModel = computed<FavoriteRouteGroup[]>({
  get: () => favoriteRouteGroups.value,
  set: (groups) => favoritesStore.reorderRoutes(groups.flatMap((group) => group.favorites)),
})

const stopFavoritesModel = computed<number[]>({
  get: () => favoriteStopIds.value,
  set: (newIds) => favoritesStore.reorderStopIds([...newIds]),
})

const planFavoritesModel = computed<FavoritePlan[]>({
  get: () => favoritePlans.value,
  set: (newPlans) => favoritesStore.reorderPlans([...newPlans]),
})

const kbd = useKeyboardNav()

const favRouteItemKey = (group: FavoriteRouteGroup) => `fav-route-${group.routeId}`
const favStopItemKey = (id: number) => `fav-stop-${id}`
const planKey = (p: FavoritePlan) => `${p.lat}_${p.lon}_${p.originLat ?? ''}_${p.originLon ?? ''}`
const favPlanItemKey = (p: FavoritePlan) => `fav-plan-${planKey(p)}`
const recentItemKey = (p: FavoritePlan) => `recent-${planKey(p)}`

function removeFocused(): boolean {
  const key = kbd.focusedKey()
  if (!key) return false
  const neighbor = kbd.neighborKey()
  const group = favoriteRouteGroups.value.find(g => favRouteItemKey(g) === key)
  const stopId = favoriteStopIds.value.find(id => favStopItemKey(id) === key)
  const plan = favoritePlans.value.find(p => favPlanItemKey(p) === key)
  const recent = recentNonFavoritePlans.value.find(p => recentItemKey(p) === key)
  if (group) removeRouteGroup(group)
  else if (stopId !== undefined) favoritesStore.toggleStopFavorite(stopId)
  else if (plan && isOnline.value) favoritesStore.togglePlanFavorite(plan)
  else if (recent && isOnline.value) favoritesStore.removeRecentPlan(recent)
  else return false
  void nextTick(() => {
    if (!neighbor || !kbd.focusKey(neighbor)) kbd.focusEntry()
  })
  return true
}

function swapInList<T>(list: T[], keyOf: (item: T) => string, key: string, delta: number, apply: (next: T[]) => void): boolean {
  const i = list.findIndex(item => keyOf(item) === key)
  if (i < 0) return false
  const j = i + delta
  if (j < 0 || j >= list.length) return true
  const next = [...list]
  ;[next[i], next[j]] = [next[j]!, next[i]!]
  apply(next)
  return true
}

function moveFocused(delta: 1 | -1): boolean {
  const key = kbd.focusedKey()
  if (!key) return false
  const moved = swapInList(favoriteRouteGroups.value, favRouteItemKey, key, delta, l => favoritesStore.reorderRoutes(l.flatMap((g) => g.favorites)))
    || swapInList(favoriteStopIds.value, favStopItemKey, key, delta, l => favoritesStore.reorderStopIds(l))
    || swapInList(favoritePlans.value, favPlanItemKey, key, delta, l => favoritesStore.reorderPlans(l))
  if (moved) void nextTick(() => kbd.focusKey(key))
  return moved
}

useKbdShortcuts({
  x: removeFocused,
  Delete: removeFocused,
  J: () => moveFocused(1),
  K: () => moveFocused(-1),
})

const dragOptions = {
  delay: LONG_PRESS_MS,
  delayOnTouchOnly: true,
  animation: 180,
  ghostClass: 'drag-ghost',
  chosenClass: 'drag-chosen',
}
</script>

<template>
  <div class="home-view">
    <section v-if="journeyStore.journey || (settings.busAlerts && followsStore.follows.length)" class="live-cards"
             data-kbd-section="live" data-kbd-entry="1">
      <JourneyCard v-if="journeyStore.journey"/>
      <template v-if="settings.busAlerts">
        <FollowCard v-for="f in followsStore.follows" :key="f.id" :followed="f"/>
      </template>
    </section>

    <section v-if="settings.busAlerts && remindersStore.reminders.length" class="pt-3" data-kbd-section="reminders" data-kbd-entry="1">
      <SubLabel :text="t('reminders')"/>
      <ListRow
        v-for="row in reminderRows"
        :key="row.reminder.id"
        :title="row.title"
        :kicker="row.kicker"
        :remove-label="t('remindDelete')"
        :data-kbd-item="`reminder-${row.reminder.id}`"
        @open="openReminder(row.reminder)"
        @remove="remindersStore.remove(row.reminder.id)"
      >
        <template #icon>
          <LineBadge :name="row.reminder.line" :color="row.reminder.color" :height="30" :min-width="38" :corner="8"/>
        </template>
      </ListRow>
    </section>

    <template v-if="isHydrated">
      <section v-if="favoriteRouteChips.size" class="pt-3" data-kbd-section="fav-routes" data-kbd-axis="grid" data-kbd-entry="1">
        <SubLabel :text="t('favoriteRoutes')"/>
        <Draggable v-model="routeFavoritesModel" item-key="routeId" class="route-chips" tag="div" v-bind="dragOptions">
          <template #item="{element: group}">
            <div
              v-if="favoriteRouteChips.get(group.routeId)"
              class="route-chip state squish"
              :class="{ 'is-busy': openingRouteId === group.routeId }"
              role="button"
              tabindex="0"
              :title="favoriteRouteChips.get(group.routeId)!.name"
              :data-kbd-item="favRouteItemKey(group)"
              @click="openRoute(favoriteRouteChips.get(group.routeId)!.route, favoriteRouteChips.get(group.routeId)!.direction)"
              @keydown.enter.prevent="openRoute(favoriteRouteChips.get(group.routeId)!.route, favoriteRouteChips.get(group.routeId)!.direction)"
            >
              <LineBadge
                :name="favoriteRouteChips.get(group.routeId)!.route.route_short_name"
                :color="favoriteRouteChips.get(group.routeId)!.route.route_color"
                :height="28" :min-width="36" :corner="8"
              />
              <span class="chip-label">{{ favoriteRouteChips.get(group.routeId)!.label }}</span>
              <button
                type="button"
                class="icon-btn chip-remove"
                :title="t('removeFromFavorites')"
                :aria-label="t('removeFromFavorites')"
                @click.stop="removeRouteGroup(group)"
              >
                <CloseIcon class="m-icon size-4"/>
              </button>
            </div>
          </template>
        </Draggable>
      </section>

      <section v-if="favoriteStops.length" :class="favoriteRouteChips.size ? 'pt-2' : 'pt-3'" data-kbd-section="fav-stops" data-kbd-entry="1">
        <SubLabel :text="t('favoriteStops')"/>
        <Draggable v-model="stopFavoritesModel" :item-key="(id: number) => id" tag="div" v-bind="dragOptions">
          <template #item="{element: stopId}">
            <ListRow
              v-if="stopsById.get(stopId)"
              :title="stopsById.get(stopId)!.stop_name"
              :remove-label="t('removeFromFavorites')"
              :data-kbd-item="favStopItemKey(stopId)"
              @open="openStop(stopsById.get(stopId)!)"
              @remove="favoritesStore.toggleStopFavorite(stopId)"
            >
              <template #icon>
                <StationIcon :look="lookOf(stopId)" class="size-[30px]"/>
              </template>
              <template #lines>
                <StationLines :routes="lookOf(stopId)?.routes ?? []"/>
              </template>
            </ListRow>
          </template>
        </Draggable>
      </section>

      <section v-if="isOnline && favoritePlans.length" class="pt-2" data-kbd-section="fav-plans" data-kbd-entry="1">
        <SubLabel :text="t('favoritePlans')"/>
        <Draggable v-model="planFavoritesModel" :item-key="planKey" tag="div" v-bind="dragOptions">
          <template #item="{element: plan}">
            <ListRow
              :title="plan.label ?? plan.name"
              :title-prefix="!plan.label && plan.originName ? `${t('planTo')}: ` : undefined"
              :kicker="!plan.label && plan.originName ? `${t('planFrom')}: ${plan.originName}` : undefined"
              :remove-label="t('removeFromFavorites')"
              :data-kbd-item="favPlanItemKey(plan)"
              @open="openPlan(plan)"
              @remove="favoritesStore.togglePlanFavorite(plan)"
            >
              <template #icon><RowIcon :icon="MapIcon" tone="tertiary" :size="30"/></template>
            </ListRow>
          </template>
        </Draggable>
      </section>

      <section v-if="isOnline && recentNonFavoritePlans.length" :class="hasFavorites ? 'pt-2' : 'pt-3'" data-kbd-section="recent-plans" data-kbd-entry="1">
        <SubLabel :text="t('recentDestinations')"/>
        <ListRow
          v-for="plan in recentNonFavoritePlans"
          :key="planKey(plan)"
          :title="plan.name"
          :title-prefix="plan.originName ? `${t('planTo')}: ` : undefined"
          :kicker="plan.originName ? `${t('planFrom')}: ${plan.originName}` : undefined"
          :remove-label="t('removeFromRecents')"
          :data-kbd-item="recentItemKey(plan)"
          @open="openPlan(plan)"
          @remove="favoritesStore.removeRecentPlan(plan)"
        >
          <template #icon><RowIcon :icon="ScheduleIcon" :size="30"/></template>
        </ListRow>
      </section>

      <p v-if="!hasFavorites && !(isOnline && recentNonFavoritePlans.length)" class="empty">
        {{ t('noFavorites') }}
      </p>
    </template>
    <div class="h-6"></div>
  </div>
</template>

<style scoped>
.live-cards {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 8px 16px;
}

.route-chips {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  padding: 4px 12px;
}

.route-chip {
  --squish: 0.93;
  display: flex;
  align-items: center;
  min-width: 0;
  padding: 6px 0 6px 10px;
  border-radius: 14px;
  background: var(--md-surface-container);
  color: var(--md-on-surface);
  cursor: pointer;
}

.route-chip.is-busy {
  opacity: 0.6;
  pointer-events: none;
}

.chip-label {
  flex: 1;
  min-width: 0;
  padding-left: 8px;
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  letter-spacing: 0.25px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.chip-remove {
  width: 32px;
  height: 32px;
  color: var(--md-outline);
}

.empty {
  padding: 12px 16px;
  font-size: 12px;
  line-height: 16px;
  letter-spacing: 0.4px;
  color: var(--md-on-surface-variant);
}

.drag-ghost {
  opacity: 0.3;
}

.drag-chosen {
  scale: 1.03;
  box-shadow: var(--md-elevation-3);
  background: var(--md-surface-container-high);
}
</style>

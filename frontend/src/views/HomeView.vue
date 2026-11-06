<script setup lang="ts">
import {computed, nextTick, onMounted, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {storeToRefs} from 'pinia'
import Draggable from 'vuedraggable'
import {useFavoritesStore} from '@/stores/favorites.ts'
import type {FavoriteGroup, FavoritePlan, FavoriteRoute} from '@/stores/favorites.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import {useOnline} from '@/composables/useOnline.ts'
import {useOpenRoute} from '@/composables/useOpenRoute.ts'
import {type RecentItem, useRecents} from '@/composables/useRecents.ts'
import type {Route, RouteDirection, Stop} from '@/types/tranzy.ts'
import {network} from '@/utils/network.ts'
import StationIcon from '@/components/StationIcon.vue'
import StationLines from '@/components/rows/StationLines.vue'
import RecentRow from '@/components/rows/RecentRow.vue'
import HomeSection from '@/components/home/HomeSection.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import RowIcon from '@/components/ui/RowIcon.vue'
import ListRow from '@/components/ui/ListRow.vue'
import AllLines from '@/components/home/AllLines.vue'
import PlaceChips from '@/components/home/PlaceChips.vue'
import {useFavoriteDrag} from '@/composables/useFavoriteDrag.ts'
import {groupKey, type GroupLine, groupRoutes} from '@/utils/lineGroups.ts'
import {CheckIcon, MapIcon, SwapHorizIcon} from '@/utils/icons.ts'
import {planQuery} from '@/utils/planQuery.ts'
import {groupLocation, stopLocation} from '@/utils/pagePaths.ts'
import {useKbdShortcuts, useKeyboardNav} from '@/composables/useKeyboardNav.ts'
import {canonicalUrl} from '@/utils/site.ts'
import {cityParams, cityPath} from '@/utils/city.ts'
import {type HomeSectionId, withShownOrder} from '@/utils/homeSections.ts'
import {useHomeSections} from '@/composables/useHomeSections.ts'

const RECENTS_SHOWN = 5
const LONG_PRESS_MS = 350

const {t} = useI18n()
const router = useRouter()

useHead(() => ({
  title: t('headHomeTitle', cityParams()),
  meta: [
    {name: 'description', content: t('headHomeDesc', cityParams())},
    {property: 'og:title', content: t('headHomeTitle', cityParams())},
    {property: 'og:description', content: t('headHomeDesc', cityParams())},
    {property: 'og:url', content: canonicalUrl(cityPath('/'))},
    {name: 'twitter:title', content: t('headHomeTitle', cityParams())},
    {name: 'twitter:description', content: t('headHomeDesc', cityParams())},
  ],
  link: [{rel: 'canonical', href: canonicalUrl(cityPath('/'))}],
}))
const favoritesStore = useFavoritesStore()
const {favoriteRoutes, favoriteStopIds, favoritePlans, favoriteGroups, isHydrated} = storeToRefs(favoritesStore)

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
  parts: string[]
  both?: boolean
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
    if (!single) chip = {route, parts: origin ? [origin, end] : [longName], both: true, name: longName}
    else if (single.direction === '1' && origin) chip = {route, parts: [`→ ${origin}`], name: `${end} - ${origin}`, direction: '1'}
    else chip = {route, parts: [`→ ${end}`], name: longName, direction: single.direction}
    chips.set(group.routeId, chip)
  }
  return chips
})

const favoriteStops = computed<Stop[]>(() => favoriteStopIds.value.map((id) => stopsById.value.get(id)).filter((s): s is Stop => !!s))

// Saved ones count until lines and stops load.
const hasFavorites = computed(() => {
  const shown =
    routes.value.length && stops.value.length
      ? favoriteRouteChips.value.size + favoriteStops.value.length
      : favoriteRoutes.value.length + favoriteStopIds.value.length
  return shown > 0 || favoriteGroups.value.length > 0 || (isOnline.value && favoritePlans.value.length > 0)
})

const groupLines = computed(() => {
  const index = network.value
  return index ? favoriteGroups.value.map((g) => groupRoutes(index, g.lines)) : []
})

function openGroup(lines: GroupLine[]) {
  if (network.value) void router.push(groupLocation(network.value, lines))
}

const recents = useRecents(RECENTS_SHOWN)

const lookOf = (stopId: number) => network.value?.look(stopId)

function openStop(stop: Stop) {
  void router.push(stopLocation(stop.stop_id))
}

function openPlan(plan: FavoritePlan) {
  if (isOnline.value) void router.push({name: 'plan', query: planQuery(plan)})
}

function openRecent(item: RecentItem) {
  favoritesStore.addRecent(item.recent)
  if (item.stop) openStop(item.stop)
  else if (item.route) void openRoute(item.route)
  else if (item.group) openGroup(item.group)
}

function removeRouteGroup(group: FavoriteRouteGroup) {
  for (const fav of group.favorites) favoritesStore.toggleRouteFavorite(fav.routeId, fav.direction)
}

const {holdStart, holdEnd, removeWithUndo} = useFavoriteDrag()

const routeFavoritesModel = computed<FavoriteRouteGroup[]>({
  get: () => favoriteRouteGroups.value,
  set: (groups) => favoritesStore.reorderRoutes(groups.flatMap((group) => group.favorites)),
})

const stopFavoritesModel = computed<number[]>({
  get: () => favoriteStopIds.value,
  set: (newIds) => favoritesStore.reorderStopIds([...newIds]),
})

const groupFavoritesModel = computed<FavoriteGroup[]>({
  get: () => favoriteGroups.value,
  set: (groups) => favoritesStore.reorderGroups([...groups]),
})

const planFavoritesModel = computed<FavoritePlan[]>({
  get: () => favoritePlans.value,
  set: (newPlans) => favoritesStore.reorderPlans([...newPlans]),
})

const {order: sectionOrder, setOrder: setSectionOrder, toggle: toggleCollapsed, isCollapsed} = useHomeSections()

const shownSections = computed<HomeSectionId[]>(() =>
  sectionOrder.value.filter((id) => {
    if (id === 'lines') return favoriteRouteChips.value.size > 0
    if (id === 'groups') return favoriteGroups.value.length > 0
    if (id === 'stops') return favoriteStops.value.length > 0
    if (id === 'places') return isOnline.value && favoritePlans.value.length > 0
    if (id === 'recents') return recents.value.length > 0
    return false
  }),
)

const sectionsModel = computed<HomeSectionId[]>({
  get: () => shownSections.value,
  set: (shown) => setSectionOrder(withShownOrder(sectionOrder.value, shown)),
})

const SECTION_TITLES: Partial<Record<HomeSectionId, string>> = {
  lines: 'favoriteRoutes',
  groups: 'lineGroups',
  stops: 'favoriteStops',
  places: 'favoritePlans',
  recents: 'recentDestinations',
}
const sectionTitle = (id: HomeSectionId) => t(SECTION_TITLES[id] ?? id)

const arranging = ref(false)

const kbd = useKeyboardNav()

const favRouteItemKey = (group: FavoriteRouteGroup) => `fav-route-${group.routeId}`
const favGroupItemKey = (group: FavoriteGroup) => `fav-group-${groupKey(group.lines)}`
const favStopItemKey = (id: number) => `fav-stop-${id}`
const planKey = (p: FavoritePlan) => `${p.lat}_${p.lon}_${p.originLat ?? ''}_${p.originLon ?? ''}`
const favPlanItemKey = (p: FavoritePlan) => `fav-plan-${planKey(p)}`
const recentItemKey = (item: RecentItem) => `recent-${item.key}`
const sectionItemKey = (id: HomeSectionId) => `section-${id}`

function removeFocused(): boolean {
  const key = kbd.focusedKey()
  if (!key) return false
  const neighbor = kbd.neighborKey()
  const group = favoriteRouteGroups.value.find((g) => favRouteItemKey(g) === key)
  const stopId = favoriteStopIds.value.find((id) => favStopItemKey(id) === key)
  const plan = favoritePlans.value.find((p) => favPlanItemKey(p) === key)
  const recent = recents.value.find((item) => recentItemKey(item) === key)
  const lineGroup = favoriteGroups.value.find((g) => favGroupItemKey(g) === key)
  if (lineGroup) removeWithUndo(() => favoritesStore.removeGroup(lineGroup.lines))
  else if (group) removeWithUndo(() => removeRouteGroup(group))
  else if (stopId !== undefined) removeWithUndo(() => favoritesStore.toggleStopFavorite(stopId))
  else if (plan && isOnline.value) removeWithUndo(() => favoritesStore.togglePlanFavorite(plan))
  else if (recent) favoritesStore.removeRecent(recent.recent)
  else return false
  void nextTick(() => {
    if (!neighbor || !kbd.focusKey(neighbor)) kbd.focusEntry()
  })
  return true
}

function swapInList<T>(list: T[], keyOf: (item: T) => string, key: string, delta: number, apply: (next: T[]) => void): boolean {
  const i = list.findIndex((item) => keyOf(item) === key)
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
  const moved =
    swapInList(shownSections.value, sectionItemKey, key, delta, (l) => (sectionsModel.value = l)) ||
    swapInList(favoriteGroups.value, favGroupItemKey, key, delta, (l) => favoritesStore.reorderGroups(l)) ||
    swapInList(favoriteRouteGroups.value, favRouteItemKey, key, delta, (l) => favoritesStore.reorderRoutes(l.flatMap((g) => g.favorites))) ||
    swapInList(favoriteStopIds.value, favStopItemKey, key, delta, (l) => favoritesStore.reorderStopIds(l)) ||
    swapInList(favoritePlans.value, favPlanItemKey, key, delta, (l) => favoritesStore.reorderPlans(l))
  if (moved) void nextTick(() => kbd.focusKey(key))
  return moved
}

useKbdShortcuts({
  Escape: () => {
    if (!arranging.value) return false
    arranging.value = false
    return true
  },
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

const sectionDragOptions = computed(() => ({
  disabled: !arranging.value,
  animation: dragOptions.animation,
  ghostClass: dragOptions.ghostClass,
  chosenClass: dragOptions.chosenClass,
  handle: '.section-head',
  // The native drag image lags behind the list.
  forceFallback: true,
  fallbackTolerance: 3,
  // The sheet's transform offsets a nested clone.
  fallbackOnBody: true,
}))
</script>

<template>
  <div class="home-view">
    <PlaceChips />
    <template v-if="isHydrated">
      <div v-if="arranging" class="arrange-bar">
        <span class="text-body-medium text-on-surface-variant">{{ t('sectionsArrangeHint') }}</span>
        <button type="button" class="btn btn-tonal" @click="arranging = false"><CheckIcon class="m-icon" />{{ t('save') }}</button>
      </div>
      <Draggable v-model="sectionsModel" :item-key="(id: HomeSectionId) => id" tag="div" v-bind="sectionDragOptions">
        <template #item="{element: id}">
          <HomeSection
            :title="sectionTitle(id)"
            :collapsed="isCollapsed(id)"
            :arranging="arranging"
            :head-key="sectionItemKey(id)"
            @toggle="toggleCollapsed(id)"
            @hold="arranging = true"
          >
            <div v-if="id === 'lines'" data-kbd-section="fav-routes" data-kbd-axis="grid" data-kbd-entry="1">
              <Draggable
                v-model="routeFavoritesModel"
                item-key="routeId"
                class="route-chips"
                tag="div"
                v-bind="dragOptions"
                @start="holdStart"
                @end="holdEnd('routes', $event)"
              >
                <template #item="{element: group}">
                  <div v-if="favoriteRouteChips.get(group.routeId)" class="chip-cell">
                    <div
                      class="route-chip state squish"
                      :class="{'is-busy': openingRouteId === group.routeId}"
                      role="button"
                      tabindex="0"
                      :title="favoriteRouteChips.get(group.routeId)!.name"
                      :data-kbd-item="favRouteItemKey(group)"
                      @click="openRoute(favoriteRouteChips.get(group.routeId)!.route, favoriteRouteChips.get(group.routeId)!.direction)"
                      @keydown.enter.prevent="openRoute(favoriteRouteChips.get(group.routeId)!.route, favoriteRouteChips.get(group.routeId)!.direction)"
                    >
                      <span class="chip-badge">
                        <LineBadge
                          :name="favoriteRouteChips.get(group.routeId)!.route.route_short_name"
                          :color="favoriteRouteChips.get(group.routeId)!.route.route_color"
                          :height="28"
                          :min-width="36"
                          :corner="8"
                        />
                        <SwapHorizIcon v-if="favoriteRouteChips.get(group.routeId)!.both" class="m-icon size-4 text-on-surface-variant" />
                      </span>
                      <span class="chip-label" :class="{'is-both': favoriteRouteChips.get(group.routeId)!.both}">
                        <span v-for="(part, i) in favoriteRouteChips.get(group.routeId)!.parts" :key="i">{{ part }}</span>
                      </span>
                    </div>
                  </div>
                </template>
              </Draggable>
            </div>

            <div v-else-if="id === 'groups'" data-kbd-section="fav-groups" data-kbd-entry="1">
              <Draggable
                v-model="groupFavoritesModel"
                :item-key="(g: FavoriteGroup) => groupKey(g.lines)"
                class="fav-list"
                tag="div"
                v-bind="dragOptions"
                @start="holdStart"
                @end="holdEnd('groups', $event)"
              >
                <template #item="{element: group, index: i}">
                  <ListRow
                    :title="group.name"
                    :data-kbd-item="favGroupItemKey(group)"
                    @open="openGroup(group.lines)"
                  >
                    <template #lines>
                      <StationLines :routes="groupLines[i] ?? []" />
                    </template>
                  </ListRow>
                </template>
              </Draggable>
            </div>

            <div v-else-if="id === 'stops'" data-kbd-section="fav-stops" data-kbd-entry="1">
              <Draggable
                v-model="stopFavoritesModel"
                :item-key="(id: number) => id"
                class="fav-list"
                tag="div"
                v-bind="dragOptions"
                @start="holdStart"
                @end="holdEnd('stops', $event)"
              >
                <template #item="{element: stopId}">
                  <ListRow
                    v-if="stopsById.get(stopId)"
                    :title="stopsById.get(stopId)!.stop_name"
                    :data-kbd-item="favStopItemKey(stopId)"
                    @open="openStop(stopsById.get(stopId)!)"
                  >
                    <template #icon>
                      <StationIcon :look="lookOf(stopId)" class="size-[30px]" />
                    </template>
                    <template #lines>
                      <StationLines :routes="lookOf(stopId)?.routes ?? []" />
                    </template>
                  </ListRow>
                </template>
              </Draggable>
            </div>

            <div v-else-if="id === 'places'" data-kbd-section="fav-plans" data-kbd-entry="1">
              <Draggable
                v-model="planFavoritesModel"
                :item-key="planKey"
                class="fav-list"
                tag="div"
                v-bind="dragOptions"
                @start="holdStart"
                @end="holdEnd('plans', $event)"
              >
                <template #item="{element: plan}">
                  <ListRow
                    :title="plan.label ?? plan.name"
                    :title-prefix="!plan.label && plan.originName ? `${t('planTo')}: ` : undefined"
                    :kicker="!plan.label && plan.originName ? `${t('planFrom')}: ${plan.originName}` : undefined"
                    :data-kbd-item="favPlanItemKey(plan)"
                    @open="openPlan(plan)"
                  >
                    <template #icon><RowIcon :icon="MapIcon" tone="tertiary" :size="30" /></template>
                  </ListRow>
                </template>
              </Draggable>
            </div>

            <div v-else-if="id === 'recents'" data-kbd-section="recent-plans" data-kbd-entry="1">
              <template v-for="item in recents" :key="item.key">
                <ListRow
                  v-if="item.plan"
                  :title="item.plan.name"
                  :subtitle="item.plan.originName ? `${t('planFrom')}: ${item.plan.originName}` : t('planTo')"
                  :remove-label="t('removeFromRecents')"
                  :data-kbd-item="recentItemKey(item)"
                  @open="openPlan(item.plan!)"
                  @remove="favoritesStore.removeRecent(item.recent)"
                >
                  <template #icon><RowIcon :icon="MapIcon" :size="30" /></template>
                </ListRow>
                <RecentRow
                  v-else
                  :stop="item.stop"
                  :route="item.route"
                  :group="item.group"
                  :icon-size="30"
                  by-kind
                  :remove-label="t('removeFromRecents')"
                  :busy="!!item.route && openingRouteId === item.route.route_id"
                  :data-kbd-item="recentItemKey(item)"
                  @open="openRecent(item)"
                  @remove="favoritesStore.removeRecent(item.recent)"
                />
              </template>
            </div>
          </HomeSection>
        </template>
      </Draggable>

      <p v-if="!hasFavorites && !recents.length" class="empty">
        {{ t('noFavorites') }}
      </p>
      <AllLines v-if="!hasFavorites" />
    </template>
    <div class="h-6"></div>
  </div>
</template>

<style scoped>
.route-chips {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: start;
  gap: 8px;
  padding: 0 12px 4px;
}

/* 48px touch target */
.chip-cell {
  display: flex;
  align-items: center;
  min-width: 0;
  min-height: 48px;
}

.route-chip {
  --squish: 0.93;
  display: flex;
  flex: 1;
  align-items: center;
  min-width: 0;
  padding: 6px 12px 6px 10px;
  border-radius: 14px;
  background: var(--md-surface-container-high);
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

.chip-label.is-both {
  display: flex;
  flex-direction: column;
  font-size: 13px;
  letter-spacing: 0;
}

.chip-label.is-both > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chip-badge {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.route-chips,
.fav-list {
  user-select: none;
  -webkit-user-select: none;
  -webkit-touch-callout: none;
}

.empty {
  padding: 12px 16px;
  font-size: 12px;
  line-height: 16px;
  letter-spacing: 0.4px;
  color: var(--md-on-surface-variant);
}

.arrange-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 4px 16px 0 24px;
}

.drag-ghost {
  opacity: 0.3;
}

.drag-chosen {
  scale: 1.03;
}

.drag-chosen:not(.chip-cell) {
  border-radius: 16px;
  box-shadow: var(--md-elevation-3);
  background: var(--md-surface-container-high);
}

.drag-chosen > .route-chip {
  box-shadow: var(--md-elevation-3);
  background: var(--md-surface-container-highest);
}
</style>

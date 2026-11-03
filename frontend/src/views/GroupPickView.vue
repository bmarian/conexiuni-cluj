<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {atCap, useFavoritesStore} from '@/stores/favorites.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import SubLabel from '@/components/ui/SubLabel.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import PickRow from '@/components/rows/PickRow.vue'
import type {RouteDirection} from '@/types/tranzy.ts'
import {network, type NetworkRoute} from '@/utils/network.ts'
import {useLegsNear} from '@/composables/useLegsNear.ts'
import {type GroupLine, meetingLines, readLines} from '@/utils/lineGroups.ts'
import {groupLocation} from '@/utils/pagePaths.ts'
import {compareUse, lineUse, readUsage, totalUses, whenOf} from '@/utils/lineUsage.ts'
import {displayColor} from '@/utils/lineColor.ts'
import {byModeThenNumber, groupByMode} from '@/utils/modes.ts'
import {matching, RESULTS_PER_KIND} from '@/utils/stopSearch.ts'
import {fold} from '@/utils/text.ts'
import {GROUP_LINES_CAP} from '@/utils/sync/records.ts'
import {directedDestination, hasTimetableEntries} from '@/utils/time.ts'
import {CloseIcon, SearchIcon, SwapHorizIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const favorites = useFavoritesStore()

useHead(() => ({title: `${t('lineGroups')} | Ce autobuz iau?`, meta: [{name: 'robots', content: 'noindex'}]}))

const chosen = ref<GroupLine[]>([])
const query = ref('')

watch(
  network,
  (index, before) => {
    if (index && !before) chosen.value = readLines(index, String(route.query.linii ?? ''))
  },
  {immediate: true},
)

const routes = computed(() => (network.value?.bundle.routes ?? []).filter((r) => r.timetable && hasTimetableEntries(r.timetable)))
const routeOf = (routeId: number) => network.value?.routeById.get(routeId)
const isChosen = (routeId: number) => chosen.value.some((l) => l.routeId === routeId)

const legs = useLegsNear(chosen)

// Read once so the lists don't reorder under the finger.
const lineUsage = readUsage()
const now = whenOf(Date.now())

const usageOf = (routeId: number) => lineUse(lineUsage, routeId)

const isFavorite = (routeId: number) => favorites.isRouteFavorite(routeId)
const recentIds = new Set(favorites.recents.flatMap((r) => ('routeId' in r ? [r.routeId] : [])))
const used = (routeId: number) => isFavorite(routeId) || recentIds.has(routeId) || totalUses(usageOf(routeId)) > 0
const byUse = (a: number, b: number) => compareUse(usageOf(a), usageOf(b), now) || Number(isFavorite(b)) - Number(isFavorite(a))

const first = computed(() => legs.value[0]?.lines[0])
const servable = computed(() => new Set(routes.value.map((r) => r.route_id)))

const meeting = computed(() => (network.value && first.value ? meetingLines(network.value, first.value).filter((m) => servable.value.has(m.routeId)) : []))

const connections = computed(() =>
  [...meeting.value].sort((a, b) => byUse(a.routeId, b.routeId)).filter((m, i) => used(m.routeId) || isChosen(m.routeId) || i < RESULTS_PER_KIND),
)

const often = computed(() => {
  const shown = new Set(connections.value.map((m) => m.routeId))
  const ids = new Set<number>([
    ...Object.keys(lineUsage).map((key) => Number(key.split('/')[0])),
    ...favorites.favoriteRoutes.map((r) => r.routeId),
    ...recentIds,
  ])
  return [...ids]
    .filter((id) => servable.value.has(id) && !shown.has(id))
    .map((id) => routeOf(id)!)
    .sort((a, b) => byUse(a.route_id, b.route_id) || byModeThenNumber(a, b))
})

const allByMode = computed(() => groupByMode(routes.value))

const found = computed(() => {
  const q = fold(query.value.trim())
  return q
    ? matching(routes.value, (r) => fold(`${r.route_short_name} ${r.route_long_name}`), q)
        .sort((a, b) => a.rank - b.rank)
        .map((m) => m.item)
    : []
})

function directionFor(routeId: number): RouteDirection {
  const meet = meeting.value.find((m) => m.routeId === routeId)
  if (meet) return meet.direction
  const [u0, u1] = [totalUses(lineUsage[`${routeId}/0`]), totalUses(lineUsage[`${routeId}/1`])]
  if (u0 !== u1) return u1 > u0 ? '1' : '0'
  return favorites.favoriteRoutes.find((r) => r.routeId === routeId)?.direction ?? '0'
}

function toggle(routeId: number, direction?: RouteDirection) {
  if (isChosen(routeId)) chosen.value = chosen.value.filter((l) => l.routeId !== routeId)
  else if (!atCap(chosen.value, GROUP_LINES_CAP)) chosen.value = [...chosen.value, {routeId, direction: direction ?? directionFor(routeId)}]
}

function flip(routeId: number) {
  chosen.value = chosen.value.map((l) => (l.routeId === routeId ? {...l, direction: l.direction === '0' ? '1' : '0'} : l))
}

const badgeColor = (r: NetworkRoute) => displayColor(r.route_short_name, r.route_color)

const chosenRows = computed(() =>
  chosen.value.flatMap((line) => {
    const r = routeOf(line.routeId)
    if (!r) return []
    const board = legs.value.flatMap((leg) => leg.lines).find((l) => l.routeId === line.routeId)
    return [
      {
        line,
        route: r,
        destination: directedDestination(r.route_long_name, line.direction),
        from: board ? stopName(board.stopId) : '',
      },
    ]
  }),
)

const stopName = (stopId: number) => network.value?.stopById.get(stopId)?.stop_name ?? ''

function show() {
  const index = network.value
  if (!index || !chosen.value.length) return
  const editing = route.query.schimba
  if (editing) favorites.replaceGroupLines(readLines(index, String(editing)), chosen.value)
  void router.replace(groupLocation(index, chosen.value))
}
</script>

<template>
  <div class="group-pick-view">
    <SheetTopBar :title="t('lineGroups')" />
    <h1 class="text-title-large font-bold px-4 pb-2">{{ t('lineGroups') }}</h1>

    <label class="search-pill mx-4 mb-2">
      <SearchIcon class="m-icon text-on-surface" />
      <input v-model="query" class="search-input text-body-large" type="search" :placeholder="t('groupSearch')" :aria-label="t('groupSearch')" />
    </label>

    <section v-if="chosenRows.length" class="pt-1" data-kbd-section="chosen">
      <SubLabel :text="t('groupChosen')" />
      <div v-for="row in chosenRows" :key="row.line.routeId" class="chosen-row">
        <LineBadge :name="row.route.route_short_name" :color="badgeColor(row.route)" plain :height="32" :min-width="40" :corner="10" />
        <span class="flex-1 min-w-0 flex flex-col">
          <span class="text-title-small truncate">→ {{ row.destination }}</span>
          <span v-if="row.from" class="text-body-small text-on-surface-variant truncate">{{ t('from', {name: row.from}) }}</span>
        </span>
        <button type="button" class="icon-btn text-on-surface-variant" :aria-label="t('groupFlip')" :title="t('groupFlip')" @click="flip(row.line.routeId)">
          <SwapHorizIcon class="m-icon" />
        </button>
        <button type="button" class="icon-btn text-outline" :aria-label="t('groupRemoveLine')" :title="t('groupRemoveLine')" @click="toggle(row.line.routeId)">
          <CloseIcon class="m-icon size-5" />
        </button>
      </div>
    </section>

    <template v-if="!query.trim()">
      <section v-if="connections.length" class="pt-2" data-kbd-section="connections">
        <SubLabel :text="t('groupConnects', {line: routeOf(first!.routeId)?.route_short_name ?? ''})" />
        <PickRow
          v-for="m in connections"
          :key="m.routeId"
          :name="routeOf(m.routeId)!.route_short_name"
          :color="badgeColor(routeOf(m.routeId)!)"
          :title="routeOf(m.routeId)!.route_long_name"
          :subtitle="t('groupMeetsAt', {name: stopName(m.stopId)})"
          :checked="isChosen(m.routeId)"
          @toggle="toggle(m.routeId, m.direction)"
        />
      </section>

      <section v-if="often.length" class="pt-2" data-kbd-section="often">
        <SubLabel :text="t('groupOften')" />
        <PickRow
          v-for="r in often"
          :key="r.route_id"
          :name="r.route_short_name"
          :color="badgeColor(r)"
          :title="r.route_long_name"
          :checked="isChosen(r.route_id)"
          @toggle="toggle(r.route_id)"
        />
      </section>

      <section class="pt-2" data-kbd-section="all-lines">
        <template v-for="group in allByMode" :key="group.mode">
          <SubLabel :text="t(`modeLines_${group.mode}`)" class="pt-2" />
          <PickRow
            v-for="r in group.lines"
            :key="r.route_id"
            :name="r.route_short_name"
            :color="badgeColor(r)"
            :title="r.route_long_name"
            :checked="isChosen(r.route_id)"
            @toggle="toggle(r.route_id)"
          />
        </template>
      </section>
    </template>

    <section v-else class="pt-2" data-kbd-section="found">
      <PickRow
        v-for="r in found"
        :key="r.route_id"
        :name="r.route_short_name"
        :color="badgeColor(r)"
        :title="r.route_long_name"
        :checked="isChosen(r.route_id)"
        @toggle="toggle(r.route_id)"
      />
      <p v-if="!found.length" class="text-body-medium text-on-surface-variant px-4 py-3">{{ t('noResults') }}</p>
    </section>

    <div class="pick-bar">
      <button type="button" class="btn btn-filled w-full" :disabled="!chosen.length" @click="show">{{ t('groupShow') }}</button>
    </div>
  </div>
</template>

<style scoped>
.chosen-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 56px;
  padding: 0 4px 0 16px;
}

.pick-bar {
  position: sticky;
  bottom: 0;
  z-index: 1;
  padding: 12px 16px 16px;
  background: var(--md-surface-container-low);
  box-shadow: 0 -1px 0 var(--md-outline-variant);
}
</style>

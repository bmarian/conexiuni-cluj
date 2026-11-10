<script setup lang="ts">
import {computed, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {atCap, useFavoritesStore} from '@/stores/favorites.ts'
import {useScrollStart} from '@/composables/useDrawerScroll.ts'
import {useGroupBuilder} from '@/composables/useGroupBuilder.ts'
import {useGroupShapes} from '@/composables/useGroupShapes.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import SubLabel from '@/components/ui/SubLabel.vue'
import ListRow from '@/components/ui/ListRow.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import type {RouteDirection} from '@/types/tranzy.ts'
import {network, type NetworkRoute} from '@/utils/network.ts'
import {type GroupLine, lineStops, opposite, withStops} from '@/utils/lineGroups.ts'
import {type Call, callsByStation, viaStops} from '@/utils/lineCalls.ts'
import {compareUse, lineUse, readUsage, totalUses, whenOf} from '@/utils/lineUsage.ts'
import {byModeThenNumber, groupByMode} from '@/utils/modes.ts'
import {matching, RESULTS_PER_KIND} from '@/utils/stopSearch.ts'
import {fold} from '@/utils/text.ts'
import {GROUP_LINES_CAP} from '@/utils/sync/records.ts'
import {directedDestination, hasTimetableEntries} from '@/utils/time.ts'
import {CloseIcon, SearchIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const favorites = useFavoritesStore()
const {lines, openStops} = useGroupBuilder()
useGroupShapes(lines)

useHead(() => ({title: `${t('groupAddLine')} | Ce autobuz iau?`, meta: [{name: 'robots', content: 'noindex'}]}))

const view = ref<HTMLElement>()
useScrollStart(view)

const query = ref('')
const asked = computed(() => fold(query.value.trim()))

const routes = computed(() => (network.value?.bundle.routes ?? []).filter((r) => r.timetable && hasTimetableEntries(r.timetable)))
const routeOf = (routeId: number) => network.value?.routeById.get(routeId)
const calls = computed(() => (network.value ? callsByStation(network.value, routes.value) : new Map<number, Call[]>()))

// Read once so the lists don't reorder under the finger.
const lineUsage = readUsage()
const now = whenOf(Date.now())

const isFavorite = (routeId: number) => favorites.isRouteFavorite(routeId)
const recentIds = new Set(favorites.recents.flatMap((r) => ('routeId' in r ? [r.routeId] : [])))
const byUse = (a: number, b: number) =>
  compareUse(lineUse(lineUsage, a), lineUse(lineUsage, b), now) || Number(isFavorite(b)) - Number(isFavorite(a))

const yours = computed(() => {
  const used = Object.keys(lineUsage).map((key) => Number(key.split('/')[0]))
  const ids = new Set<number>([...used, ...favorites.favoriteRoutes.map((r) => r.routeId), ...recentIds])
  const servable = new Set(routes.value.map((r) => r.route_id))
  return [...ids]
    .filter((id) => servable.has(id))
    .map((id) => routeOf(id)!)
    .sort((a, b) => byUse(a.route_id, b.route_id) || byModeThenNumber(a, b))
})

const byRank = (a: {rank: number}, b: {rank: number}) => a.rank - b.rank

const found = computed(() => (asked.value ? matching(routes.value, (r) => fold(`${r.route_short_name} ${r.route_long_name}`), asked.value).sort(byRank) : []))

const through = computed(() => {
  const index = network.value
  if (!index || !asked.value) return []
  const ranked = matching([...index.stopById.values()], (s) => fold(s.stop_name), asked.value).sort(byRank)
  const rankOf = new Map(ranked.map((m) => [m.item.stop_id, m.rank]))
  return index
    .oneStopPerStation(ranked.map((m) => m.item))
    .flatMap((stop) => {
      const lines = calls.value.get(index.stationId(stop.stop_id)) ?? []
      return lines.length ? [{stop, lines, rank: rankOf.get(stop.stop_id)!}] : []
    })
    .slice(0, RESULTS_PER_KIND)
})

function directionFor(routeId: number): RouteDirection {
  const [u0, u1] = [totalUses(lineUsage[`${routeId}/0`]), totalUses(lineUsage[`${routeId}/1`])]
  const liked = u0 !== u1 ? (u1 > u0 ? '1' : '0') : (favorites.favoriteRoutes.find((r) => r.routeId === routeId)?.direction ?? '0')
  const runs = (direction: RouteDirection) => !!network.value && lineStops(network.value, {routeId, direction}).length > 0
  return runs(liked) || !runs(opposite(liked)) ? liked : opposite(liked)
}

const added = (routeId: number): GroupLine | undefined => lines.value.find((l) => l.routeId === routeId)

function pick(routeId: number, direction?: RouteDirection, from?: number) {
  const inGroup = added(routeId)
  if (inGroup) return openStops(inGroup)
  if (atCap(lines.value, GROUP_LINES_CAP)) return
  openStops(withStops({routeId, direction: direction ?? directionFor(routeId)}, from, undefined))
}

const vias = computed(() => {
  const index = network.value
  return new Map(index ? routes.value.map((r) => [r.route_id, viaStops(index, r, calls.value)]) : [])
})

const via = (r: NetworkRoute) => {
  const names = vias.value.get(r.route_id) ?? []
  return names.length ? t('groupVia', {stops: names.join(', ')}) : undefined
}

type Row = {key: string; route: NetworkRoute; title: string; subtitle?: string; pick: () => void}
type Section = {key: string; label: string; rows: Row[]}

const lineRow = (r: NetworkRoute): Row => ({key: String(r.route_id), route: r, title: r.route_long_name, subtitle: via(r), pick: () => pick(r.route_id)})

const sections = computed<Section[]>(() => {
  if (!asked.value) {
    const all = groupByMode(routes.value).map((group) => ({key: group.mode, label: t(`modeLines_${group.mode}`), rows: group.lines.map(lineRow)}))
    return [...(yours.value.length ? [{key: 'yours', label: t('groupYours'), rows: yours.value.map(lineRow)}] : []), ...all]
  }
  const lines = found.value.length ? [{key: 'found', label: t('groupLines'), rows: found.value.map((m) => lineRow(m.item))}] : []
  const stations = through.value.map((station) => ({
    key: `station-${station.stop.stop_id}`,
    label: t('groupThrough', {name: station.stop.stop_name}),
    rows: station.lines.map((call) => ({
      key: `${call.route.route_id}-${call.direction}`,
      route: call.route,
      title: `→ ${directedDestination(call.route.route_long_name, call.direction)}`,
      subtitle: t('from', {name: network.value?.stopName(call.stopId)}),
      pick: () => pick(call.route.route_id, call.direction, call.stopId),
    })),
  }))
  // A station named outright leads lines that only have the letters somewhere.
  const stationsLead = !!through.value.length && (!found.value.length || through.value[0]!.rank < found.value[0]!.rank)
  return stationsLead ? [...stations, ...lines] : [...lines, ...stations]
})
</script>

<template>
  <div ref="view" class="group-add-view">
    <SheetTopBar :title="t('groupAddLine')" />
    <h1 class="text-title-large font-bold px-4 pb-2">{{ t('groupAddLine') }}</h1>

    <label class="search-pill mx-4 mb-2">
      <SearchIcon class="m-icon text-on-surface" />
      <input v-model="query" class="search-input text-body-large" type="search" :placeholder="t('groupSearch')" :aria-label="t('groupSearch')" />
      <button v-if="query" type="button" class="icon-btn" :aria-label="t('clearSearch')" :title="t('clearSearch')" @click="query = ''">
        <CloseIcon class="m-icon" />
      </button>
    </label>

    <section v-for="(section, at) in sections" :key="section.key" class="pt-1" :data-kbd-section="section.key" :data-kbd-entry="at === 0 ? 1 : undefined">
      <SubLabel :text="section.label" class="pt-1" />
      <ListRow
        v-for="row in section.rows"
        :key="row.key"
        :title="row.title"
        :subtitle="row.subtitle"
        :trailing="added(row.route.route_id) ? t('groupInGroup') : undefined"
        class="pr-4"
        :data-kbd-item="`${section.key}-${row.key}`"
        @open="row.pick()"
      >
        <template #icon>
          <LineBadge :name="row.route.route_short_name" :color="row.route.route_color" :height="32" :min-width="40" :corner="10" />
        </template>
      </ListRow>
    </section>
    <p v-if="!sections.length" class="text-body-medium text-on-surface-variant px-4 py-3">{{ t('noResults') }}</p>
    <div class="h-6"></div>
  </div>
</template>

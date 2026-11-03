<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useRouteStore} from '@/stores/route.ts'
import {useLineGroup} from '@/composables/useLineGroup.ts'
import {provideLiveBeat} from '@/composables/useLiveBeat.ts'
import {useShare} from '@/composables/useShare.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ActionChips from '@/components/ui/ActionChips.vue'
import DeparturesLabel from '@/components/ui/DeparturesLabel.vue'
import DepartureTimes from '@/components/ui/DepartureTimes.vue'
import BusCard from '@/components/rows/BusCard.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import GroupNameDialog from '@/components/GroupNameDialog.vue'
import type {ChipAction, MenuAction} from '@/components/ui/types.ts'
import {buildStopInfo, network} from '@/utils/network.ts'
import {type GroupLine, groupKey, groupLabel, groupRoutes, readLines, tripOf, writeLines} from '@/utils/lineGroups.ts'
import {directedDestination, formatMinutesFromNow} from '@/utils/time.ts'
import {groupLocation, linePath} from '@/utils/pagePaths.ts'
import {EditIcon, FavoriteBorderIcon, FavoriteIcon, ShareIcon, SwapHorizIcon, VisibilityIcon, VisibilityOffIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const favorites = useFavoritesStore()
const routeStore = useRouteStore()
const {userTime} = storeToRefs(useUserStore())

const raw = computed(() => String(route.query.linii ?? ''))
const lines = computed(() => (network.value ? readLines(network.value, raw.value) : []))
const key = computed(() => groupKey(lines.value))
const saved = computed(() => favorites.findGroup(lines.value))
const routes = computed(() => (network.value ? groupRoutes(network.value, lines.value) : []))

const {hidden, toggleHidden, boards, departures, asOfMs} = useLineGroup(lines)
provideLiveBeat(asOfMs)

// The shared drawer scroller keeps the picker's offset.
const view = ref<HTMLElement>()
onMounted(() => {
  const scroller = view.value?.closest('.drawer-view')
  if (scroller) scroller.scrollTop = 0
})

watch(
  key,
  (k) => {
    if (k) favorites.addRecent({lines: lines.value})
  },
  {immediate: true},
)

const title = computed(() => saved.value?.name || groupLabel(routes.value))

useHead(() => ({
  title: [title.value, 'Ce autobuz iau?'].filter(Boolean).join(' | '),
  meta: [{name: 'robots', content: 'noindex'}],
}))

const stopName = (stopId: number | undefined) => (stopId === undefined ? '' : (network.value?.stopById.get(stopId)?.stop_name ?? ''))

const rows = computed(() =>
  lines.value.flatMap((line, i) => {
    const r = network.value?.routeById.get(line.routeId)
    if (!r) return []
    const stopId = boards.value[i]
    return [
      {
        line,
        route: r,
        stopId,
        from: stopName(stopId),
        departure: departures.value[i] ?? null,
        destination: directedDestination(r.route_long_name, line.direction),
      },
    ]
  }),
)

const label = (minutes: number) => formatMinutesFromNow(minutes, userTime.value || new Date(), t('now'))

function flip(routeId: number) {
  const index = network.value
  if (!index) return
  const next = lines.value.map((l): GroupLine => (l.routeId === routeId ? {...l, direction: l.direction === '0' ? '1' : '0'} : l))
  if (saved.value) favorites.replaceGroupLines(lines.value, next)
  favorites.removeRecent({lines: lines.value})
  void router.replace(groupLocation(index, next))
}

function openLine(line: GroupLine, stopId: number | undefined) {
  const index = network.value
  const info = index && stopId !== undefined ? buildStopInfo(index, stopId)?.shapes_info.find((s) => s.route_id === line.routeId) : undefined
  if (info) routeStore.setSelectedRoute(info, tripOf(line), String(stopId), stopName(stopId))
  void router.push(linePath(line.routeId, line.direction))
}

const naming = ref(false)

function saveNamed(name: string) {
  favorites.saveGroup(lines.value, name || undefined)
}

function toggleSaved() {
  if (saved.value) favorites.removeGroup(lines.value)
  else naming.value = true
}

function editLines() {
  const index = network.value
  if (!index) return
  const linii = writeLines(index, lines.value)
  void router.replace({name: 'group-pick', query: saved.value ? {linii, schimba: linii} : {linii}})
}

const {share} = useShare()

const chips = computed<ChipAction[]>(() => {
  const edit: ChipAction = {label: t('groupLines'), icon: EditIcon, kbd: 'lines', onClick: editLines}
  if (network.value && !lines.value.length) return [edit]
  return [
    {
      label: t(saved.value ? 'saved' : 'save'),
      icon: saved.value ? FavoriteIcon : FavoriteBorderIcon,
      selected: !!saved.value,
      tint: saved.value ? 'var(--md-rose)' : undefined,
      kbd: 'fav',
      onClick: toggleSaved,
    },
    edit,
    {label: t('share'), icon: ShareIcon, kbd: 'share', onClick: () => void share()},
  ]
})

const topMenu = computed<MenuAction[]>(() => (saved.value ? [{label: t('groupRename'), icon: EditIcon, onClick: () => (naming.value = true)}] : []))
</script>

<template>
  <div ref="view" class="group-view">
    <SheetTopBar :title="title" :menu="topMenu" />

    <header class="header">
      <h1 class="flex flex-col gap-1.5">
        <span class="text-label-medium text-primary">{{ t('lineGroups') }}</span>
        <span v-if="saved?.name" class="text-headline-small font-bold">{{ saved.name }}</span>
        <span class="flex flex-wrap gap-1.5">
          <LineBadge v-for="r in routes" :key="r.route_id" :name="r.route_short_name" :color="r.route_color" :height="36" :min-width="44" :corner="12" />
        </span>
      </h1>
    </header>
    <ActionChips :actions="chips" class="pb-2" />

    <section class="flex flex-col" data-kbd-section="lines" data-kbd-axis="grid" data-kbd-entry="1">
      <DeparturesLabel :text="t('nextDepartures')" />
      <template v-if="!network">
        <div v-for="i in 3" :key="i" class="skeleton card-skeleton"></div>
      </template>
      <p v-else-if="!rows.length" class="text-body-medium text-on-surface-variant px-4 py-3">{{ t('noResults') }}</p>
      <BusCard
        v-for="(row, i) in rows"
        :key="row.line.routeId"
        class="cascade"
        :class="{'is-hidden': hidden.has(row.line.routeId)}"
        :style="{'--i': i}"
        :live="!!row.departure && !row.departure.static_time_approximation"
        :name="row.route.route_short_name"
        :color="row.route.route_color"
        :destination="row.destination"
        :subtitle="row.from ? t('from', {name: row.from}) : undefined"
        :kbd="`line-${row.line.routeId}`"
        @open="openLine(row.line, row.stopId)"
      >
        <DepartureTimes v-if="row.departure" :shape="row.departure" :label="label" />
        <span v-else class="text-body-small text-on-surface-variant pt-1">{{ t('groupNoMore') }}</span>
        <template #trailing>
          <button
            type="button"
            class="icon-btn text-on-surface-variant"
            :aria-label="t('groupFlip')"
            :title="t('groupFlip')"
            :data-kbd-item="`flip-${row.line.routeId}`"
            @click="flip(row.line.routeId)"
          >
            <SwapHorizIcon class="m-icon" />
          </button>
          <button
            type="button"
            class="icon-btn text-on-surface-variant"
            :aria-label="t(hidden.has(row.line.routeId) ? 'groupShowLine' : 'groupHideLine')"
            :title="t(hidden.has(row.line.routeId) ? 'groupShowLine' : 'groupHideLine')"
            :data-kbd-item="`hide-${row.line.routeId}`"
            @click="toggleHidden(row.line.routeId)"
          >
            <VisibilityOffIcon v-if="hidden.has(row.line.routeId)" class="m-icon" />
            <VisibilityIcon v-else class="m-icon" />
          </button>
        </template>
      </BusCard>
    </section>
    <div class="h-6"></div>

    <GroupNameDialog v-model:open="naming" :routes="routes" :name="saved?.name" @save="saveNamed" />
  </div>
</template>

<style scoped>
.header {
  padding: 4px 16px 8px;
}

.is-hidden {
  opacity: 0.55;
}
</style>

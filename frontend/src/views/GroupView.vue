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
import LineChain from '@/components/ui/LineChain.vue'
import DeparturesLabel from '@/components/ui/DeparturesLabel.vue'
import LegCard from '@/components/rows/LegCard.vue'
import GroupNameDialog from '@/components/GroupNameDialog.vue'
import type {ChipAction, MenuAction} from '@/components/ui/types.ts'
import {buildStopInfo, network} from '@/utils/network.ts'
import {chainOf, chainText, groupKey, type Leg, readLines, tripOf, writeLines} from '@/utils/lineGroups.ts'
import {recordUse} from '@/utils/lineUsage.ts'
import {displayColor} from '@/utils/lineColor.ts'
import {cityClock} from '@/utils/cityClock.ts'
import {directedDestination, formatMinutesFromNow} from '@/utils/time.ts'
import {linePath, stopLocation} from '@/utils/pagePaths.ts'
import {DirectionsIcon, EditIcon, FavoriteBorderIcon, FavoriteIcon, ShareIcon} from '@/utils/icons.ts'

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

const {legs, times, asOfMs} = useLineGroup(lines)
provideLiveBeat(asOfMs)

const now = computed(() => (userTime.value ?? new Date()).getTime())

// The shared drawer scroller keeps the picker's offset.
const view = ref<HTMLElement>()
onMounted(() => {
  const scroller = view.value?.closest('.drawer-view')
  if (scroller) scroller.scrollTop = 0
})

watch(
  key,
  (k) => {
    if (k) recordUse('groups', k)
  },
  {immediate: true},
)

const chain = computed(() => (network.value ? chainOf(network.value, legs.value, (name, color) => displayColor(name, color, true)) : []))
const chainLabel = computed(() => chainText(chain.value))
const title = computed(() => saved.value?.name || chainLabel.value)

useHead(() => ({
  title: [title.value, 'Ce autobuz iau?'].filter(Boolean).join(' | '),
  meta: [{name: 'robots', content: 'noindex'}],
}))

const stopName = (stopId: number) => network.value?.stopById.get(stopId)?.stop_name ?? ''

function cardOf(leg: Leg) {
  const index = network.value
  const cardLines = leg.lines.flatMap((l) => {
    const r = index?.routeById.get(l.routeId)
    return r ? [{routeId: l.routeId, name: r.route_short_name, color: displayColor(r.route_short_name, r.route_color, true), routeType: r.route_type}] : []
  })
  const destinations = leg.lines.map((l) => directedDestination(index?.routeById.get(l.routeId)?.route_long_name ?? '', l.direction))
  return {lines: cardLines, destination: [...new Set(destinations)].join(' / '), from: stopName(leg.lines[0]!.stopId)}
}

const label = (minutes: number) => formatMinutesFromNow(minutes, userTime.value || new Date(), t('now'))

function openLeg(leg: Leg) {
  const index = network.value
  const first = leg.lines[0]!
  if (leg.lines.length > 1 || !index) {
    void router.push(stopLocation(first.stopId))
    return
  }
  const info = buildStopInfo(index, first.stopId)?.shapes_info.find((s) => s.route_id === first.routeId)
  if (info) routeStore.setSelectedRoute(info, tripOf(first), String(first.stopId), stopName(first.stopId))
  void router.push(linePath(first.routeId, first.direction))
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

function directions() {
  const index = network.value
  if (!index) return
  const to = saved.value?.destination
  void router.push({name: 'plan', query: {linii: writeLines(index, lines.value), ...(to ? {lat: String(to.lat), lon: String(to.lon), name: to.name} : {})}})
}

const chips = computed<ChipAction[]>(() => [
  {label: t('directions'), icon: DirectionsIcon, primary: true, kbd: 'directions', onClick: directions},
  {
    label: t(saved.value ? 'saved' : 'save'),
    icon: saved.value ? FavoriteIcon : FavoriteBorderIcon,
    selected: !!saved.value,
    tint: saved.value ? 'var(--md-rose)' : undefined,
    kbd: 'fav',
    onClick: toggleSaved,
  },
  {label: t('groupLines'), icon: EditIcon, kbd: 'lines', onClick: editLines},
  {label: t('share'), icon: ShareIcon, kbd: 'share', onClick: () => void share()},
])

const topMenu = computed<MenuAction[]>(() => (saved.value ? [{label: t('groupRename'), icon: EditIcon, onClick: () => (naming.value = true)}] : []))
</script>

<template>
  <div ref="view" class="group-view">
    <SheetTopBar :title="title" :menu="topMenu" />

    <header class="header">
      <h1 class="flex flex-col gap-1.5">
        <span class="text-label-medium text-primary">{{ t('lineGroups') }}</span>
        <span v-if="saved?.name" class="text-headline-small font-bold">{{ saved.name }}</span>
        <LineChain :legs="chain" />
      </h1>
    </header>
    <ActionChips :actions="chips" class="pb-2" />

    <section class="flex flex-col" data-kbd-section="departures" data-kbd-entry="1">
      <DeparturesLabel :text="t('nextDepartures')" />
      <template v-if="!network">
        <div v-for="i in 2" :key="i" class="skeleton card-skeleton"></div>
      </template>
      <p v-else-if="!legs.length" class="text-body-medium text-on-surface-variant px-4 py-3">{{ t('noResults') }}</p>
      <template v-for="(leg, i) in legs" v-else :key="`${i}-${leg.lines[0]!.routeId}`">
        <div v-if="leg.alight" class="change">
          <span class="change-rule"><span></span></span>
          <span class="flex items-center gap-2 py-1">
            <span class="text-title-small">{{ t('groupChange', {name: stopName(leg.lines[0]!.stopId)}) }}</span>
            <span v-if="times[i]?.arrival" class="text-body-small text-on-surface-variant">{{ cityClock(times[i]!.arrival!) }}</span>
          </span>
        </div>
        <LegCard
          v-if="times[i]"
          v-bind="cardOf(leg)"
          class="cascade"
          :style="{'--i': i}"
          :times="times[i]!"
          :now="now"
          :label="label"
          :data-kbd-item="`leg-${i}`"
          @open="openLeg(leg)"
        />
      </template>
    </section>
    <div class="h-6"></div>

    <GroupNameDialog v-model:open="naming" :chain="chain" :name="saved?.name" @save="saveNamed" />
  </div>
</template>

<style scoped>
.header {
  padding: 4px 16px 8px;
}

.change {
  display: flex;
  align-items: stretch;
  min-height: 44px;
  margin: 0 12px;
}

.change-rule {
  display: flex;
  justify-content: center;
  flex-shrink: 0;
  width: 72px;
}

.change-rule > span {
  border-left: 2px dotted var(--md-outline);
}
</style>

<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRoute} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useUserStore} from '@/stores/user.ts'
import {useMapStore} from '@/stores/map.ts'
import {scrollBelow} from '@/composables/useDrawerScroll.ts'
import {useGroupBuilder} from '@/composables/useGroupBuilder.ts'
import {usePhoneSheet} from '@/composables/usePhoneSheet.ts'
import {useStretchMap} from '@/composables/useStretchMap.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ChoiceRow from '@/components/ui/ChoiceRow.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import StopLine, {type StopMark} from '@/components/rows/StopLine.vue'
import type {Choice} from '@/components/ui/types.ts'
import {network} from '@/utils/network.ts'
import {
  boardingStop,
  flipLine,
  type GroupLine,
  lineStops,
  opposite,
  pickStop,
  readLines,
  stopRows,
  type StopRole,
  type StretchEnd,
  stretchNames,
  stretchOf,
  withStops,
} from '@/utils/lineGroups.ts'
import {directedDestination} from '@/utils/time.ts'
import {FlagIcon, PlaceIcon, SwapHorizIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const route = useRoute()
const phone = usePhoneSheet()
const mapStore = useMapStore()
const {userLocation} = storeToRefs(useUserStore())
const {lines, backToGroup} = useGroupBuilder()

const line = ref<GroupLine | null>(null)
const end = ref<StretchEnd>('from')

watch(
  network,
  (index) => {
    if (!index || line.value) return
    line.value = readLines(index, String(route.query.linie ?? ''))[0] ?? null
    if (line.value?.from !== undefined && line.value.to === undefined) end.value = 'to'
  },
  {immediate: true},
)

const info = computed(() => (line.value ? network.value?.routeById.get(line.value.routeId) : undefined))
const destination = computed(() => (info.value && line.value ? directedDestination(info.value.route_long_name, line.value.direction) : ''))
const others = computed(() => lines.value.filter((l) => l.routeId !== line.value?.routeId))
const isNew = computed(() => others.value.length === lines.value.length)
const stretch = computed(() => (network.value && line.value ? stretchOf(network.value, line.value) : {stops: [], from: -1, to: -1}))
const chosen = computed(() => !!line.value && (line.value.from !== undefined || line.value.to !== undefined))

const nearest = computed(() =>
  network.value && line.value && stretch.value.from < 0 && userLocation.value ? boardingStop(network.value, line.value, userLocation.value) : undefined,
)

const canFlip = computed(
  () => !!network.value && !!line.value && lineStops(network.value, {routeId: line.value.routeId, direction: opposite(line.value.direction)}).length > 0,
)

useHead(() => ({
  title: [info.value?.route_short_name, t('lineGroups'), 'Ce autobuz iau?'].filter(Boolean).join(' | '),
  meta: [{name: 'robots', content: 'noindex'}],
}))

function tap(stopId: number) {
  if (!network.value || !line.value) return
  const next = pickStop(network.value, line.value, end.value, stopId)
  line.value = next.line
  end.value = next.next
}

useStretchMap(line, others, nearest, tap)

function flip() {
  if (!network.value || !line.value) return
  line.value = flipLine(network.value, line.value)
  end.value = 'from'
}

function wholeLine() {
  if (!line.value) return
  line.value = withStops(line.value, undefined, undefined)
  end.value = 'from'
}

function done() {
  const edited = line.value
  if (!edited) return
  void backToGroup(isNew.value ? [...lines.value, edited] : lines.value.map((l) => (l.routeId === edited.routeId ? edited : l)))
}

const ends = computed<Choice<StretchEnd>[]>(() => {
  const {from, to} = network.value && line.value ? stretchNames(network.value, line.value) : {}
  return [
    {value: 'from', icon: PlaceIcon, label: from ?? t('groupBoardAt')},
    {value: 'to', icon: FlagIcon, label: to ?? t('groupLeaveAt')},
  ]
})

const MARKS: Record<StopRole, StopMark> = {board: 'from', near: 'nearest', leave: 'terminal', pass: 'plain'}

const rows = computed(() => {
  const index = network.value
  if (!index || !line.value) return []
  const tags: Partial<Record<StopRole, string>> = {board: t('groupBoardAt'), near: t('groupNearYou'), leave: t('groupLeaveAt')}
  const all = stopRows(index, line.value, nearest.value)
  return all.map((row, at) => ({
    ...row,
    name: index.stopName(row.stopId),
    mark: row.role === 'pass' && at === all.length - 1 ? 'terminal' : MARKS[row.role],
    tag: tags[row.role],
  }))
})

const view = ref<HTMLElement>()
const list = ref<HTMLElement>()
const pinned = ref<HTMLElement>()
let placed = false
watch(
  rows,
  async (shown) => {
    if (placed || !shown.length) return
    placed = true
    await nextTick()
    const boards = shown.findIndex((row) => !row.dimmed)
    const row = list.value?.children[boards] as HTMLElement | undefined
    if (boards > 0 && row && view.value && pinned.value) scrollBelow(view.value, pinned.value, row)
  },
  {immediate: true},
)

const lineColor = computed(() => mapStore.lineColors.get(info.value?.route_short_name ?? '') ?? info.value?.route_color)
</script>

<template>
  <div ref="view" class="has-bottom-bar">
    <SheetTopBar :title="info ? `${info.route_short_name} → ${destination}` : null">
      <template v-if="phone && info" #lead>
        <LineBadge :name="info.route_short_name" :color="info.route_color" :height="32" :min-width="40" :corner="10" class="ml-1" />
        <h1 class="lead-title text-title-medium">→ {{ destination }}</h1>
      </template>
      <button v-if="phone && canFlip" type="button" class="icon-btn" :aria-label="t('groupFlip')" :title="t('groupFlip')" data-kbd-item="flip" @click="flip">
        <SwapHorizIcon class="m-icon" />
      </button>
    </SheetTopBar>

    <header v-if="!phone && info" class="head">
      <LineBadge :name="info.route_short_name" :color="info.route_color" :height="36" :min-width="44" :corner="12" />
      <h1 class="flex-1 min-w-0 text-title-large font-bold truncate">→ {{ destination }}</h1>
      <button v-if="canFlip" type="button" class="icon-btn" :aria-label="t('groupFlip')" :title="t('groupFlip')" data-kbd-item="flip" @click="flip">
        <SwapHorizIcon class="m-icon" />
      </button>
    </header>

    <div ref="pinned" class="ends" data-kbd-section="ends" data-kbd-axis="x">
      <ChoiceRow v-model="end" :options="ends" kbd="end" class="mx-4" />
    </div>

    <div ref="list" class="stops" :style="{'--paint': lineColor}" data-kbd-section="stops" data-kbd-entry="1">
      <StopLine
        v-for="(row, at) in rows"
        :key="`${at}-${row.stopId}`"
        :name="row.name"
        :mark="row.mark"
        :first="at === 0"
        :last="at === rows.length - 1"
        :trailing="row.tag"
        :painted="row.painted"
        :dimmed="row.dimmed"
        :data-kbd-item="`stop-${at}`"
        @open="tap(row.stopId)"
      />
    </div>
    <div class="h-4"></div>

    <div class="bottom-bar" data-kbd-section="done" data-kbd-axis="x">
      <button v-if="chosen" type="button" class="btn btn-text" data-kbd-item="whole" @click="wholeLine">{{ t('groupWholeLine') }}</button>
      <button type="button" class="btn btn-filled flex-1" :disabled="!line" data-kbd-item="done" @click="done">
        {{ t(isNew ? 'groupAddToGroup' : 'done') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.lead-title {
  flex: 1;
  min-width: 0;
  padding: 0 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 4px 4px 16px;
}

.ends {
  position: sticky;
  top: 48px;
  z-index: 10;
  padding: 8px 0 4px;
  background: var(--md-surface-container-low);
}

.stops {
  position: relative;
}
</style>

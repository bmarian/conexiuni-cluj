<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useLineGroup} from '@/composables/useLineGroup.ts'
import {useShare} from '@/composables/useShare.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ActionChips from '@/components/ui/ActionChips.vue'
import SubLabel from '@/components/ui/SubLabel.vue'
import ListRow from '@/components/ui/ListRow.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import GroupNameDialog from '@/components/GroupNameDialog.vue'
import type {ChipAction, MenuAction} from '@/components/ui/types.ts'
import {network} from '@/utils/network.ts'
import {type GroupLine, groupKey, groupLabel, groupRoutes, readLines, writeLines} from '@/utils/lineGroups.ts'
import {directedDestination} from '@/utils/time.ts'
import {groupLocation, linePath} from '@/utils/pagePaths.ts'
import {EditIcon, FavoriteBorderIcon, FavoriteIcon, ShareIcon, SwapHorizIcon, VisibilityIcon, VisibilityOffIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const favorites = useFavoritesStore()

const raw = computed(() => String(route.query.linii ?? ''))
const lines = computed(() => (network.value ? readLines(network.value, raw.value) : []))
const key = computed(() => groupKey(lines.value))
const saved = computed(() => favorites.findGroup(lines.value))
const routes = computed(() => (network.value ? groupRoutes(network.value, lines.value) : []))

const {hidden, toggleHidden} = useLineGroup(lines)

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

const rows = computed(() =>
  lines.value.flatMap((line) => {
    const r = network.value?.routeById.get(line.routeId)
    return r ? [{line, route: r, destination: directedDestination(r.route_long_name, line.direction)}] : []
  }),
)

function flip(routeId: number) {
  const index = network.value
  if (!index) return
  const next = lines.value.map((l): GroupLine => (l.routeId === routeId ? {...l, direction: l.direction === '0' ? '1' : '0'} : l))
  if (saved.value) favorites.replaceGroupLines(lines.value, next)
  favorites.removeRecent({lines: lines.value})
  void router.replace(groupLocation(index, next))
}

const openLine = (line: GroupLine) => void router.push(linePath(line.routeId, line.direction))

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

const chips = computed<ChipAction[]>(() => [
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
        <span class="flex flex-wrap gap-1.5">
          <LineBadge v-for="r in routes" :key="r.route_id" :name="r.route_short_name" :color="r.route_color" :height="36" :min-width="44" :corner="12" />
        </span>
      </h1>
    </header>
    <ActionChips :actions="chips" class="pb-2" />

    <section class="flex flex-col" data-kbd-section="lines" data-kbd-entry="1">
      <SubLabel :text="t('groupOnMap')" />
      <ListRow
        v-for="row in rows"
        :key="row.line.routeId"
        :title="`→ ${row.destination}`"
        :class="{'is-hidden': hidden.has(row.line.routeId)}"
        :data-kbd-item="`line-${row.line.routeId}`"
        @open="openLine(row.line)"
      >
        <template #icon>
          <LineBadge :name="row.route.route_short_name" :color="row.route.route_color" :height="28" :min-width="40" :corner="6" />
        </template>
        <template #trailing>
          <button type="button" class="icon-btn text-on-surface-variant" :aria-label="t('groupFlip')" :title="t('groupFlip')" @click.stop="flip(row.line.routeId)">
            <SwapHorizIcon class="m-icon" />
          </button>
          <button
            type="button"
            class="icon-btn text-on-surface-variant mr-1"
            :aria-label="t(hidden.has(row.line.routeId) ? 'groupShowLine' : 'groupHideLine')"
            :title="t(hidden.has(row.line.routeId) ? 'groupShowLine' : 'groupHideLine')"
            @click.stop="toggleHidden(row.line.routeId)"
          >
            <VisibilityOffIcon v-if="hidden.has(row.line.routeId)" class="m-icon" />
            <VisibilityIcon v-else class="m-icon" />
          </button>
        </template>
      </ListRow>
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

<script setup lang="ts">
import {computed, ref} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useScrollStart} from '@/composables/useDrawerScroll.ts'
import {useGroupBuilder} from '@/composables/useGroupBuilder.ts'
import {useGroupShapes} from '@/composables/useGroupShapes.ts'
import {usePhoneSheet} from '@/composables/usePhoneSheet.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import ListRow from '@/components/ui/ListRow.vue'
import RowIcon from '@/components/ui/RowIcon.vue'
import BusCard from '@/components/rows/BusCard.vue'
import {network} from '@/utils/network.ts'
import {readLines} from '@/utils/lineGroups.ts'
import {groupLocation} from '@/utils/pagePaths.ts'
import {directedDestination} from '@/utils/time.ts'
import {AddIcon, CloseIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const router = useRouter()
const favorites = useFavoritesStore()
const phone = usePhoneSheet()
const {lines, editing, openAdd, openStops, showGroup, stretchLabel} = useGroupBuilder()
useGroupShapes(lines)

useHead(() => ({title: `${t('lineGroups')} | Ce autobuz iau?`, meta: [{name: 'robots', content: 'noindex'}]}))

const view = ref<HTMLElement>()
useScrollStart(view)

const rows = computed(() =>
  lines.value.flatMap((line) => {
    const r = network.value?.routeById.get(line.routeId)
    return r ? [{line, route: r, destination: directedDestination(r.route_long_name, line.direction), stretch: stretchLabel(line)}] : []
  }),
)

function remove(routeId: number) {
  showGroup(lines.value.filter((l) => l.routeId !== routeId))
}

function show() {
  const index = network.value
  if (!index || !lines.value.length) return
  if (editing.value) favorites.replaceGroupLines(readLines(index, editing.value), lines.value)
  void router.replace(groupLocation(index, lines.value))
}
</script>

<template>
  <div ref="view" class="has-bottom-bar">
    <SheetTopBar :title="t('lineGroups')" />
    <h1 class="text-title-large font-bold px-4 pb-2">{{ t('lineGroups') }}</h1>

    <section class="flex flex-col" data-kbd-section="lines" data-kbd-axis="grid" data-kbd-entry="1">
      <BusCard
        v-for="row in rows"
        :key="row.line.routeId"
        :live="false"
        :name="row.route.route_short_name"
        :color="row.route.route_color"
        :destination="row.destination"
        :subtitle="row.stretch"
        :kbd="`line-${row.line.routeId}`"
        @open="openStops(row.line)"
      >
        <template #trailing>
          <button
            type="button"
            class="icon-btn text-outline"
            :aria-label="t('groupRemoveLine')"
            :title="t('groupRemoveLine')"
            :data-kbd-item="`remove-${row.line.routeId}`"
            @click="remove(row.line.routeId)"
          >
            <CloseIcon class="m-icon size-5" />
          </button>
        </template>
      </BusCard>
      <ListRow v-if="!phone" :title="t('groupAddLine')" data-kbd-item="add" @open="openAdd">
        <template #icon>
          <RowIcon :icon="AddIcon" tone="primary" />
        </template>
      </ListRow>
    </section>
    <div class="h-4"></div>

    <div class="bottom-bar" data-kbd-section="show" data-kbd-axis="x">
      <button v-if="phone" type="button" class="btn btn-tonal flex-1" data-kbd-item="add" @click="openAdd">
        <AddIcon class="m-icon" />
        {{ t('groupAddLine') }}
      </button>
      <button type="button" class="btn btn-filled flex-1" :disabled="!lines.length" data-kbd-item="show" @click="show">{{ t('groupShow') }}</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useHead} from '@unhead/vue'
import {useI18n} from 'vue-i18n'
import {network} from '@/utils/network.ts'
import {stationDestinations} from '@/utils/destinations.ts'
import {useMapPicks} from '@/composables/useMapPicks.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import type {Stop} from '@/types/tranzy.ts'
import SheetTopBar from '@/components/ui/SheetTopBar.vue'
import StationRow from '@/components/rows/StationRow.vue'
import StationLines from '@/components/rows/StationLines.vue'

const props = defineProps<{stopIds: string}>()

const {t} = useI18n()
const {openStop} = useMapPicks()
const favorites = useFavoritesStore()

const stations = computed(() => {
  const index = network.value
  if (!index) return []
  return props.stopIds
    .split(',')
    .map((id) => index.stopById.get(Number(id)))
    .filter((stop): stop is Stop => !!stop)
    .map((stop) => ({
      stop,
      destinations: stationDestinations(index, stop.stop_id),
      favorite: index.platforms(stop.stop_id).some((p) => favorites.isStopFavorite(p.stop_id)),
    }))
})

useHead(() => ({
  title: [...stations.value.map(({stop}) => stop.stop_name), 'Ce autobuz iau?'].join(' | '),
  meta: [{name: 'robots', content: 'noindex'}],
}))
</script>

<template>
  <div class="stops-view">
    <SheetTopBar />
    <section data-kbd-section="stations" data-kbd-entry="1">
      <StationRow
        v-for="{stop, destinations, favorite} in stations"
        :key="stop.stop_id"
        :stop="stop"
        :favorite="favorite"
        class="items-start!"
        :data-kbd-item="`stop-${stop.stop_id}`"
        @open="openStop(String(stop.stop_id))"
      >
        <template #lines>
          <div class="flex flex-col gap-1">
            <div v-for="to in destinations" :key="to.name ?? ''" class="flex items-start gap-2 min-w-0">
              <StationLines :routes="to.routes" class="shrink-0 max-w-[60%]" />
              <span class="text-label-medium text-on-surface-variant truncate">{{ to.name ? `→ ${to.name}` : t('lineEndsHere') }}</span>
            </div>
          </div>
        </template>
      </StationRow>
    </section>
    <div class="h-6"></div>
  </div>
</template>

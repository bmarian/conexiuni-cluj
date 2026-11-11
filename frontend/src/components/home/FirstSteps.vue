<script setup lang="ts">
import {computed, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {storeToRefs} from 'pinia'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {useUserStore} from '@/stores/user.ts'
import {useSearch} from '@/composables/useSearch.ts'
import {hideFirstSteps, markFirstStep, startFirstSteps, useFirstSteps} from '@/composables/useFirstSteps.ts'
import {network} from '@/utils/network.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {stopLocation} from '@/utils/pagePaths.ts'
import type {Stop} from '@/types/tranzy.ts'
import ListRow from '@/components/ui/ListRow.vue'
import RowIcon from '@/components/ui/RowIcon.vue'
import StationIcon from '@/components/StationIcon.vue'
import StationLines from '@/components/rows/StationLines.vue'
import {CloseIcon, DirectionsBusIcon, MapIcon, PlaceIcon} from '@/utils/icons.ts'

const {t} = useI18n()
const router = useRouter()
const search = useSearch()
const favorites = useFavoritesStore()
const {favoriteRoutes, favoriteStopIds, favoriteGroups, favoritePlans, recents, isHydrated} = storeToRefs(favorites)
const userStore = useUserStore()
const {left, done, total} = useFirstSteps()

watch(
  isHydrated,
  (hydrated) => {
    if (!hydrated) return
    const saved = favoriteRoutes.value.length + favoriteStopIds.value.length + favoriteGroups.value.length + favoritePlans.value.length
    startFirstSteps(saved === 0 && recents.value.length === 0)
  },
  {immediate: true},
)

watch(
  [favoriteStopIds, favoriteRoutes, favoriteGroups],
  ([stops, routes, groups]) => {
    if (stops.length) markFirstStep('stop')
    if (routes.length || groups.length) markFirstStep('line')
  },
  {immediate: true, deep: true},
)

const nearest = computed<Stop | null>(() => {
  const loc = userStore.userLocation
  const index = network.value
  if (!loc || !index) return null
  let best: Stop | null = null
  let bestMeters = Infinity
  for (const stop of index.bundle.stops) {
    const meters = haversineMeters(loc.latitude, loc.longitude, stop.stop_lat, stop.stop_lon)
    if (meters < bestMeters && index.look(stop.stop_id)?.routes.length) {
      best = stop
      bestMeters = meters
    }
  }
  return best
})

function scrollToLines() {
  document.querySelector('[data-kbd-section="all-lines"]')?.scrollIntoView({behavior: 'smooth', block: 'start'})
}
</script>

<template>
  <section v-if="left.length" class="first-steps" data-kbd-section="first-steps" data-kbd-entry="1">
    <div class="head">
      <h2 class="text-title-medium truncate flex-1">{{ t('firstSteps') }}</h2>
      <span class="text-body-medium text-on-surface-variant">{{ t('firstStepsCount', {done, total}) }}</span>
      <button type="button" class="icon-btn text-on-surface-variant" :aria-label="t('firstStepsHide')" :title="t('firstStepsHide')" @click="hideFirstSteps">
        <CloseIcon class="m-icon size-5" />
      </button>
    </div>
    <template v-for="step in left" :key="step">
      <ListRow v-if="step === 'plan'" :title="t('planTitle')" chevron data-kbd-item="first-plan" @open="search.open.value = true">
        <template #icon><RowIcon :icon="MapIcon" tone="tertiary" :size="30" /></template>
      </ListRow>
      <ListRow
        v-else-if="step === 'stop' && nearest"
        :kicker="t('nearestStop')"
        :title="nearest.stop_name"
        data-kbd-item="first-stop"
        @open="router.push(stopLocation(nearest.stop_id))"
      >
        <template #icon><StationIcon :look="network?.look(nearest.stop_id)" class="size-[30px]" /></template>
        <template #lines><StationLines :routes="network?.look(nearest.stop_id)?.routes ?? []" /></template>
        <template #trailing>
          <button type="button" class="btn btn-text shrink-0" @click.stop="favorites.toggleStationFavorite(nearest.stop_id)">{{ t('save') }}</button>
        </template>
      </ListRow>
      <ListRow v-else-if="step === 'stop'" :title="t('firstStepStop')" chevron data-kbd-item="first-stop" @open="search.open.value = true">
        <template #icon><RowIcon :icon="PlaceIcon" tone="tertiary" :size="30" /></template>
      </ListRow>
      <ListRow v-else :title="t('firstStepLine')" chevron data-kbd-item="first-line" @open="scrollToLines">
        <template #icon><RowIcon :icon="DirectionsBusIcon" tone="tertiary" :size="30" /></template>
      </ListRow>
    </template>
  </section>
</template>

<style scoped>
.first-steps {
  margin: 8px 12px 0;
  padding-bottom: 6px;
  border-radius: 20px;
  background: var(--md-surface-container);
}

.head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 52px;
  padding: 0 4px 0 16px;
  color: var(--md-on-surface);
}
</style>

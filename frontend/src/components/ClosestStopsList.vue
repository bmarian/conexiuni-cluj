<script setup lang="ts">
import {computed, onUnmounted, watch} from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useMapStore } from '@/stores/map.ts'
import type { Stop } from '@/types/tranzy.ts'
import { formatMeters, haversineMeters, sortByDistance } from '@/utils/geo.ts'
import { network } from '@/utils/network.ts'
import StationLabel from '@/components/StationLabel.vue'

const props = defineProps<{
  stops: Stop[]
  centerLat: number
  centerLon: number
}>()

const { t } = useI18n()
const router = useRouter()
const mapStore = useMapStore()

const closestStops = computed(() => {
  const sorted = sortByDistance(
    props.stops,
    props.centerLat,
    props.centerLon,
    s => s.stop_lat,
    s => s.stop_lon,
    700
  )
  const stations = network.value?.oneStopPerStation(sorted) ?? sorted
  return stations.slice(0, 6).map(stop => ({
    stop,
    dist: formatMeters(haversineMeters(props.centerLat, props.centerLon, stop.stop_lat, stop.stop_lon))
  }))
})

function navigateToStop(stop: Stop) {
  mapStore.clearPinnedLocation()
  mapStore.setFlyToLocation(stop.stop_lat, stop.stop_lon)
  void router.push({ name: 'stop', params: { stopId: String(stop.stop_id) } })
}

watch(closestStops, (stops) => {
  const highlightStops = stops.map(stop => ({
    stopId: String(stop.stop.stop_id),
    color: 'green' as const,
  }))
  mapStore.setHighlightedStops(highlightStops)
})

onUnmounted(() => {
  mapStore.setHighlightedStops([])
})
</script>

<template>
  <div class="result-group">
    <h3 class="sub-label">{{ t('planNearbyStops') }}</h3>
    <div class="divide-y divide-slate-100 dark:divide-slate-800/60">
      <div
        v-for="item in closestStops"
        :key="item.stop.stop_id"
        class="stop-result-row group"
        role="button"
        tabindex="0"
        @click="navigateToStop(item.stop)"
        @keydown.enter.space.prevent="navigateToStop(item.stop)"
      >
        <StationLabel :stop="item.stop"/>
        <span v-if="item.dist" class="dist-badge">{{ item.dist }}</span>
        <svg
          class="w-3.5 h-3.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors"
          fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
        </svg>
      </div>
    </div>
  </div>
</template>

<style scoped>
.result-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.sub-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  margin-bottom: 0.25rem;
}

.stop-result-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 0.5rem;
  cursor: pointer;
  border-radius: 0.75rem;
  transition: background-color 0.15s ease;
}

.stop-result-row:hover {
  background-color: #f8fafc;
}

.dark .stop-result-row:hover {
  background-color: rgba(30, 41, 59, 0.5);
}

.dist-badge {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.125rem 0.375rem;
  background: #f1f5f9;
  color: #475569;
  border-radius: 0.375rem;
  white-space: nowrap;
}

.dark .dist-badge {
  background: #1e293b;
  color: #94a3b8;
}

.emoji-icon-md {
  font-size: 1.25rem;
}

</style>

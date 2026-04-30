<script setup lang="ts">
import {computed, onUnmounted, watch} from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings.ts'
import { useMapStore } from '@/stores/map.ts'
import type { Stop } from '@/types/tranzy.ts'
import { formatMeters, haversineMeters, sortByDistance } from '@/utils/geo.ts'

const props = defineProps<{
  stops: Stop[]
  centerLat: number
  centerLon: number
}>()

const { t } = useI18n()
const router = useRouter()
const settings = useSettingsStore()
const mapStore = useMapStore()

const closestStops = computed(() => {
  const sorted = sortByDistance(
    props.stops,
    props.centerLat,
    props.centerLon,
    s => s.stop_lat,
    s => s.stop_lon
  )
  return sorted.slice(0, 6).map(stop => ({
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
        <div
          class="w-8 h-8 shrink-0 rounded-full bg-emerald-100 dark:bg-emerald-500/15 flex items-center justify-center">
          <span v-if="settings.traditionalActive" class="emoji-icon-md"
                aria-hidden="true">🚏</span>
          <svg v-else class="w-4 h-4 text-emerald-600 dark:text-emerald-400"
               viewBox="0 0 24 24" fill="currentColor">
            <path
              d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
          </svg>
        </div>
        <span
          class="flex-1 text-sm font-medium text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
          {{ item.stop.stop_name }}
        </span>
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

/* Specific theme overrides if needed, though most are handled by global CSS */
</style>

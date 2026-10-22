<script setup lang="ts">
import {computed} from 'vue'
import {useHead} from '@unhead/vue'
import HeaderNavigation from '@/components/HeaderNavigation.vue'
import StationLabel from '@/components/StationLabel.vue'
import {network} from '@/utils/network.ts'
import {stationDestinations} from '@/utils/destinations.ts'
import {useMapPicks} from '@/composables/useMapPicks.ts'
import type {Stop} from '@/types/tranzy.ts'

const props = defineProps<{ stopIds: string }>()

const {openStop} = useMapPicks()

const stations = computed(() => {
  const index = network.value
  if (!index) return []
  return props.stopIds.split(',').map((id) => index.stopById.get(Number(id)))
    .filter((stop): stop is Stop => !!stop)
    .map((stop) => ({stop, destinations: stationDestinations(index, stop.stop_id)}))
})

useHead(() => ({
  title: [...stations.value.map(({stop}) => stop.stop_name), 'Ce autobuz iau?'].join(' | '),
  meta: [{name: 'robots', content: 'noindex'}],
}))

const open = (stop: Stop) => openStop(String(stop.stop_id))
</script>

<template>
  <div class="stops-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-6">
    <div class="flex items-center -mb-2">
      <HeaderNavigation/>
    </div>

    <section class="pb-6" data-kbd-section="stations" data-kbd-entry="1">
      <div class="flex flex-col divide-y divide-slate-100 dark:divide-slate-800/60">
        <div
          v-for="{stop, destinations} in stations"
          :key="stop.stop_id"
          class="all-route-row group"
          role="button"
          tabindex="0"
          :data-kbd-item="`stop-${stop.stop_id}`"
          @click="open(stop)"
          @keydown.enter.prevent="open(stop)"
        >
          <StationLabel :stop="stop" :destinations="destinations"/>
          <svg
            class="w-3.5 h-3.5 mt-2.5 text-slate-300 dark:text-slate-600 shrink-0 group-hover:text-slate-500 dark:group-hover:text-slate-400 transition-colors"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
          </svg>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.stops-view-container {
  padding: 1.25rem 1.5rem 0;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

.all-route-row {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.75rem 0.25rem;
  margin: 0 -0.25rem;
  border-radius: 0.5rem;
  cursor: pointer;
  transition: background 0.15s;
}

.all-route-row:hover {
  background: #f8fafc;
}
</style>

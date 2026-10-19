<script setup lang="ts">
import {computed} from 'vue'
import {useSettingsStore} from '@/stores/settings.ts'
import type {Stop} from '@/types/tranzy.ts'
import {network} from '@/utils/network.ts'
import StationIcon from '@/components/StationIcon.vue'

const props = defineProps<{ stop: Stop }>()

const settings = useSettingsStore()
const look = computed(() => network.value?.look(props.stop.stop_id))
</script>

<template>
  <span v-if="settings.legacyBlueActive"
        class="w-8 h-8 shrink-0 rounded-full bg-emerald-100 dark:bg-emerald-500/15 flex items-center justify-center">
    <span class="emoji-icon-md" aria-hidden="true">🚏</span>
  </span>
  <StationIcon v-else :look="look" class="w-8 h-8 rounded-full"/>
  <div class="flex-1 min-w-0 flex flex-col gap-1">
    <span
      class="text-sm font-medium text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
      {{ stop.stop_name }}
    </span>
    <span v-if="look?.routes.length" class="station-lines">
      <span v-for="line in look.routes" :key="line.route_id" class="station-line" :style="{ backgroundColor: line.route_color }">
        {{ line.route_short_name }}
      </span>
    </span>
  </div>
</template>

<style scoped>
.station-lines {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.station-line {
  min-width: 1.75rem;
  padding: 0 0.3rem;
  border-radius: 0.3rem;
  font-size: 0.6875rem;
  line-height: 1.125rem;
  font-weight: 900;
  text-align: center;
  color: white;
}
</style>

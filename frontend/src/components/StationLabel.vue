<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings.ts'
import type {Stop} from '@/types/tranzy.ts'
import {network} from '@/utils/network.ts'
import type {Destination} from '@/utils/destinations.ts'
import StationIcon from '@/components/StationIcon.vue'
import {MODE_EMOJI} from '@/utils/modes.ts'

const props = defineProps<{ stop: Stop, destinations?: Destination[] }>()

const {t} = useI18n()
const settings = useSettingsStore()
const look = computed(() => network.value?.look(props.stop.stop_id))
</script>

<template>
  <span v-if="settings.legacyBlueActive"
        class="w-8 h-8 shrink-0 rounded-full bg-emerald-100 dark:bg-emerald-500/15 flex items-center justify-center">
    <span class="emoji-icon-md" aria-hidden="true">{{ look ? MODE_EMOJI[look.mode] : '🚏' }}</span>
  </span>
  <StationIcon v-else :look="look" class="w-8 h-8 rounded-full"/>
  <div class="flex-1 min-w-0 flex flex-col gap-1">
    <span
      class="text-sm font-medium text-slate-600 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors truncate">
      {{ stop.stop_name }}
    </span>
    <template v-if="destinations">
      <span v-for="to in destinations" :key="to.name ?? ''" class="station-towards">
        <span class="station-lines">
          <span v-for="line in to.routes" :key="line.route_id" class="station-line" :style="{ backgroundColor: line.route_color }">
            {{ line.route_short_name }}
          </span>
        </span>
        <span class="station-towards-name">{{ to.name ? `→ ${to.name}` : t('lineEndsHere') }}</span>
      </span>
    </template>
    <span v-else-if="look?.routes.length" class="station-lines">
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

.station-towards {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
}

.station-towards .station-lines {
  flex-shrink: 0;
  max-width: 60%;
}

.station-towards-name {
  min-width: 0;
  font-size: 0.75rem;
  line-height: 1.125rem;
  font-weight: 600;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:root.dark .station-towards-name {
  color: #94a3b8;
}
</style>

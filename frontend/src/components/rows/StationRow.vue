<script setup lang="ts">
import {computed} from 'vue'
import type {Stop} from '@/types/tranzy.ts'
import {network} from '@/utils/network.ts'
import StationIcon from '@/components/StationIcon.vue'
import StationLines from '@/components/rows/StationLines.vue'
import {KeyboardArrowRightIcon} from '@/utils/icons.ts'

const {iconSize = 36, stop} = defineProps<{ stop: Stop, distance?: string | null, iconSize?: number }>()
defineEmits<{ open: [] }>()

const look = computed(() => network.value?.look(stop.stop_id))
</script>

<template>
  <div class="station-row state squish" role="button" tabindex="0" @click="$emit('open')" @keydown.enter.prevent="$emit('open')">
    <StationIcon :look="look" :style="{ width: `${iconSize}px`, height: `${iconSize}px` }"/>
    <div class="flex-1 min-w-0 flex flex-col gap-1">
      <span class="text-body-large truncate">{{ stop.stop_name }}</span>
      <slot name="lines">
        <StationLines :routes="look?.routes ?? []"/>
      </slot>
    </div>
    <span v-if="distance" class="text-label-medium text-on-surface-variant shrink-0">{{ distance }}</span>
    <KeyboardArrowRightIcon class="m-icon size-[18px] text-outline"/>
  </div>
</template>

<style scoped>
.station-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  color: var(--md-on-surface);
  cursor: pointer;
}
</style>

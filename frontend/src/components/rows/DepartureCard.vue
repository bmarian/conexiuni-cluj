<script setup lang="ts">
import type {VehiclesInStop} from '@/types/tranzy.ts'
import {routeDestination, routeOrigin} from '@/utils/time.ts'
import {modeOf} from '@/utils/modes.ts'
import BusCard from '@/components/rows/BusCard.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import TimePill from '@/components/ui/TimePill.vue'

defineProps<{
  shape: VehiclesInStop
  label: (minutes: number) => string
  favorite?: boolean
}>()
const emit = defineEmits<{open: []}>()
</script>

<template>
  <BusCard
    :live="!shape.static_time_approximation"
    :destination="routeDestination(shape.route_long_name)"
    :subtitle="routeOrigin(shape.route_long_name)"
    :favorite="favorite"
    @open="emit('open')"
  >
    <template #badge>
      <LineBadge :name="shape.route_short_name" :color="shape.route_color" :height="36" :min-width="44" :corner="12" />
    </template>
    <div class="flex flex-wrap items-center gap-1 pt-1">
      <TimePill
        v-for="(time, i) in shape.next_times"
        :key="i"
        :text="i === 0 && shape.static_time_approximation ? `~ ${label(time.minutes)}` : label(time.minutes)"
        :live="i === 0 && !shape.static_time_approximation"
        :extra="i > 0"
        :due="i === 0 && !shape.static_time_approximation && time.minutes === 0"
        :mode="modeOf(shape.route_type)"
      />
    </div>
  </BusCard>
</template>

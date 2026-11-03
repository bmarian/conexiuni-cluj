<script setup lang="ts">
import type {VehiclesInStop} from '@/types/tranzy.ts'
import {routeDestination, routeOrigin} from '@/utils/time.ts'
import BusCard from '@/components/rows/BusCard.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import DepartureTimes from '@/components/ui/DepartureTimes.vue'

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
    <DepartureTimes :shape="shape" :label="label" />
  </BusCard>
</template>

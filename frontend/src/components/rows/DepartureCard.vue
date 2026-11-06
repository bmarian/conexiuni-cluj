<script setup lang="ts">
import type {VehiclesInStop} from '@/types/tranzy.ts'
import {routeDestination, routeOrigin} from '@/utils/time.ts'
import BusCard from '@/components/rows/BusCard.vue'
import DepartureTimes from '@/components/ui/DepartureTimes.vue'
import TrafficNote from '@/components/ui/TrafficNote.vue'
import type {Jam} from '@/utils/traffic.ts'

defineProps<{
  shape: VehiclesInStop
  label: (minutes: number) => string
  favorite?: boolean
  kbd?: string
  jam?: Jam
}>()
const emit = defineEmits<{open: []}>()
</script>

<template>
  <BusCard
    :live="!shape.static_time_approximation"
    :name="shape.route_short_name"
    :color="shape.route_color"
    :destination="routeDestination(shape.route_long_name)"
    :subtitle="routeOrigin(shape.route_long_name)"
    :favorite="favorite"
    :kbd="kbd"
    @open="emit('open')"
  >
    <DepartureTimes :shape="shape" :label="label" />
    <TrafficNote v-if="jam" :jam="jam" class="pt-1 text-body-small" />
  </BusCard>
</template>

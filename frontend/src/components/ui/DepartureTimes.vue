<script setup lang="ts">
import type {VehiclesInStop} from '@/types/tranzy.ts'
import {modeOf} from '@/utils/modes.ts'
import TimePill from '@/components/ui/TimePill.vue'

defineProps<{shape: VehiclesInStop; label: (minutes: number) => string}>()
</script>

<template>
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
</template>

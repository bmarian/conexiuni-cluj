<script setup lang="ts">
import {computed} from 'vue'
import type {Route, Stop} from '@/types/tranzy.ts'
import {network} from '@/utils/network.ts'
import StationIcon from '@/components/StationIcon.vue'
import StationLines from '@/components/rows/StationLines.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import ListRow from '@/components/ui/ListRow.vue'

const {iconSize = 36, stop} = defineProps<{route?: Route; stop?: Stop; iconSize?: number; removeLabel?: string; busy?: boolean}>()
defineEmits<{open: []; remove: []}>()

const look = computed(() => (stop ? network.value?.look(stop.stop_id) : undefined))
</script>

<template>
  <ListRow
    :title="stop?.stop_name ?? route?.route_long_name ?? ''"
    :remove-label="removeLabel"
    :chevron="!removeLabel"
    :busy="busy"
    @open="$emit('open')"
    @remove="$emit('remove')"
  >
    <template #icon>
      <LineBadge v-if="route" :name="route.route_short_name" :color="route.route_color" :height="28" :min-width="40" :corner="6" />
      <StationIcon v-else :look="look" :style="{width: `${iconSize}px`, height: `${iconSize}px`}" />
    </template>
    <template v-if="stop" #lines>
      <StationLines :routes="look?.routes ?? []" />
    </template>
  </ListRow>
</template>

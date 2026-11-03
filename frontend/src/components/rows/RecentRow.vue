<script setup lang="ts">
import {computed} from 'vue'
import type {Route, Stop} from '@/types/tranzy.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {network} from '@/utils/network.ts'
import {type GroupLine, groupRoutes} from '@/utils/lineGroups.ts'
import StationIcon from '@/components/StationIcon.vue'
import StationLines from '@/components/rows/StationLines.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import ListRow from '@/components/ui/ListRow.vue'

const {
  iconSize = 36,
  stop,
  route,
  group,
} = defineProps<{route?: Route; stop?: Stop; group?: GroupLine[]; iconSize?: number; removeLabel?: string; busy?: boolean}>()
defineEmits<{open: []; remove: []}>()

const favorites = useFavoritesStore()

const look = computed(() => (stop ? network.value?.look(stop.stop_id) : undefined))
const groupLines = computed(() => (group && network.value ? groupRoutes(network.value, group) : []))
const title = computed(() => stop?.stop_name ?? route?.route_long_name ?? (group && favorites.findGroup(group)?.name))
</script>

<template>
  <ListRow :title="title" :remove-label="removeLabel" :chevron="!removeLabel" :busy="busy" @open="$emit('open')" @remove="$emit('remove')">
    <template v-if="!group" #icon>
      <LineBadge v-if="route" :name="route.route_short_name" :color="route.route_color" :height="28" :min-width="40" :corner="6" />
      <StationIcon v-else :look="look" :style="{width: `${iconSize}px`, height: `${iconSize}px`}" />
    </template>
    <template v-if="stop" #lines>
      <StationLines :routes="look?.routes ?? []" />
    </template>
    <template v-else-if="group" #lines>
      <StationLines :routes="groupLines" />
    </template>
  </ListRow>
</template>

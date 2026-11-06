<script setup lang="ts">
import {computed} from 'vue'
import type {Route, Stop} from '@/types/tranzy.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {network} from '@/utils/network.ts'
import {type GroupLine, groupLabel, groupRoutes} from '@/utils/lineGroups.ts'
import StationIcon from '@/components/StationIcon.vue'
import StationLines from '@/components/rows/StationLines.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import ListRow from '@/components/ui/ListRow.vue'
import RowIcon from '@/components/ui/RowIcon.vue'
import {HistoryIcon} from '@/utils/icons.ts'
import {useI18n} from 'vue-i18n'

const {
  iconSize = 36,
  stop,
  route,
  group,
  byKind = false,
} = defineProps<{route?: Route; stop?: Stop; group?: GroupLine[]; iconSize?: number; removeLabel?: string; busy?: boolean; byKind?: boolean}>()
defineEmits<{open: []; remove: []}>()

const favorites = useFavoritesStore()
const {t} = useI18n()
const kind = computed(() => (!byKind ? undefined : stop ? t('recentKindStop') : route ? t('route') : t('lineGroups')))

const look = computed(() => (stop ? network.value?.look(stop.stop_id) : undefined))
const groupLines = computed(() => (group && network.value ? groupRoutes(network.value, group) : []))
const title = computed(() => {
  const name = stop?.stop_name ?? (route && (byKind ? route.route_short_name : route.route_long_name)) ?? (group && favorites.findGroup(group)?.name)
  return name ?? (byKind && group ? groupLabel(groupLines.value) : undefined)
})
</script>

<template>
  <ListRow :title="title" :subtitle="kind" :remove-label="removeLabel" :chevron="!removeLabel" :busy="busy" @open="$emit('open')" @remove="$emit('remove')">
    <template v-if="byKind" #icon>
      <RowIcon :icon="HistoryIcon" :size="iconSize" />
    </template>
    <template v-else-if="!group" #icon>
      <LineBadge v-if="route" :name="route.route_short_name" :color="route.route_color" :height="28" :min-width="40" :corner="6" />
      <StationIcon v-else :look="look" :style="{width: `${iconSize}px`, height: `${iconSize}px`}" />
    </template>
    <template v-if="!byKind && stop" #lines>
      <StationLines :routes="look?.routes ?? []" />
    </template>
    <template v-else-if="!byKind && group" #lines>
      <StationLines :routes="groupLines" />
    </template>
  </ListRow>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {formatMeters} from '@/utils/geo.ts'
import type {PlanBeyond} from '@/utils/plan.ts'
import {InfoIcon} from '@/utils/icons.ts'
import ListRow from '@/components/ui/ListRow.vue'
import RowIcon from '@/components/ui/RowIcon.vue'

const {beyond} = defineProps<{beyond: PlanBeyond}>()
const {t} = useI18n()
const end = computed(() => beyond.to ?? beyond.from)
</script>

<template>
  <ListRow v-if="end" :title="t(beyond.to ? 'planBeyondTo' : 'planBeyondFrom')" :subtitle="t('planBeyondStop', {distance: formatMeters(end.meters)})">
    <template #icon><RowIcon :icon="InfoIcon" /></template>
  </ListRow>
</template>

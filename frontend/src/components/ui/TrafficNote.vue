<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {network} from '@/utils/network.ts'
import {type Jam, jamMinutes} from '@/utils/traffic.ts'
import {TrafficIcon} from '@/utils/icons.ts'

const props = defineProps<{jam: Jam; between?: boolean}>()
const {t} = useI18n()

const name = (stopId: number) => network.value?.stopName(stopId) ?? ''
const text = computed(() =>
  props.between
    ? t('trafficBetween', {from: name(props.jam.from_stop_id), to: name(props.jam.to_stop_id), min: jamMinutes(props.jam)})
    : t('trafficAfter', {from: name(props.jam.from_stop_id), min: jamMinutes(props.jam)}),
)
</script>

<template>
  <p class="traffic-note">
    <TrafficIcon class="m-icon size-4 shrink-0" />
    <span class="min-w-0">{{ text }}</span>
  </p>
</template>

<style scoped>
.traffic-note {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--md-error);
  font-weight: 500;
}
</style>

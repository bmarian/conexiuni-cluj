<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import BusCard from '@/components/rows/BusCard.vue'
import LineBadge from '@/components/ui/LineBadge.vue'
import TimePill from '@/components/ui/TimePill.vue'
import type {LegTimes} from '@/utils/lineGroups.ts'
import {modeOf} from '@/utils/modes.ts'
import type {RouteType} from '@/types/tranzy.ts'

type CardLine = {routeId: number; name: string; color: string; routeType: RouteType}

const props = defineProps<{lines: CardLine[]; destination: string; from: string; times: LegTimes; now: number; label: (minutes: number) => string}>()
const emit = defineEmits<{open: []}>()

const {t} = useI18n()

const minutesTo = (at: number) => Math.max(0, Math.round((at - props.now) / 60_000))
const lineOf = (routeId: number) => props.lines.find((l) => l.routeId === routeId)

const pills = computed(() => {
  const mixed = props.lines.length > 1
  const pill = (routeId: number) => {
    const line = lineOf(routeId)
    return {line: mixed && line ? {name: line.name, color: line.color} : undefined, mode: modeOf(line?.routeType)}
  }
  const missed = props.times.missed
  return [
    ...(missed
      ? [{...pill(missed.routeId), key: `m${missed.at}`, text: props.label(minutesTo(missed.at)), missed: true, live: false, extra: false, due: false}]
      : []),
    ...props.times.shown.map((d, i) => {
      const text = props.label(minutesTo(d.at))
      return {
        ...pill(d.routeId),
        key: `${d.routeId}@${d.at}`,
        text: i === 0 && !d.live ? `~ ${text}` : text,
        missed: false,
        live: i === 0 && d.live,
        extra: i > 0,
        due: i === 0 && d.live && minutesTo(d.at) === 0,
      }
    }),
  ]
})
</script>

<template>
  <BusCard :live="!!times.shown[0]?.live" :destination="destination" :subtitle="t('from', {name: from})" @open="emit('open')">
    <template #badge>
      <span class="badges">
        <LineBadge
          v-for="line in lines"
          :key="line.routeId"
          :name="line.name"
          :color="line.color"
          plain
          :height="lines.length > 1 ? 28 : 36"
          :min-width="44"
          :corner="lines.length > 1 ? 9 : 12"
        />
      </span>
    </template>
    <div v-if="pills.length" class="flex flex-wrap items-center gap-1 pt-1">
      <TimePill v-for="p in pills" :key="p.key" :text="p.text" :live="p.live" :extra="p.extra" :due="p.due" :mode="p.mode" :line="p.line" :missed="p.missed" />
    </div>
    <span v-else class="text-body-small text-on-surface-variant pt-1">{{ t('groupNoMore') }}</span>
  </BusCard>
</template>

<style scoped>
.badges {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex-shrink: 0;
}
</style>

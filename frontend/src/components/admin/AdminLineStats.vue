<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import AdminDayBars, {type DayBar} from '@/components/admin/AdminDayBars.vue'
import type {LineStats} from '@/components/admin/types.ts'
import '@/components/admin/admin.css'
import {getJSON, SignedOut, writeAuthFlag} from '@/composables/useAdmin.ts'
import {came, dur, minSec, windowLabels} from '@/utils/adminFormat.ts'
import {currentCity} from '@/utils/city.ts'
import {cityMinutes} from '@/utils/cityClock.ts'

const props = defineProps<{routeId: string; direction: string}>()

const stats = ref<LineStats | null>(null)
const loadError = ref('')

watch(
  () => props.routeId,
  async (id) => {
    stats.value = null
    try {
      const next = await getJSON<LineStats>(`/api/admin/line?city=${currentCity.value}&route=${id}`)
      if (id === props.routeId) stats.value = next
      loadError.value = ''
    } catch (err) {
      if (err instanceof SignedOut) writeAuthFlag(false)
      else loadError.value = (err as Error).message
    }
  },
  {immediate: true},
)

const line = computed(() => stats.value?.directions[Number(props.direction)])
const day = computed(() => (stats.value ? (windowLabels[stats.value.day_type] ?? stats.value.day_type) : ''))

const hourLabel = (hour: number) => `${String(hour).padStart(2, '0')}:00`
const tripTime = (sec?: number) => (sec != null ? dur(Math.round(sec / 60) * 60) : '–')

const hours = computed<DayBar[]>(() =>
  (line.value?.hours ?? []).map((h) => ({
    date: String(h.hour),
    value: h.trip_sec ?? 0,
    caption: [hourLabel(h.hour), h.trip_sec != null ? tripTime(h.trip_sec) : '', h.delay_sec != null ? came(h.delay_sec) : '']
      .filter(Boolean)
      .join(' · '),
  })),
)

const currentHour = computed(() => {
  const hour = Math.floor(cityMinutes(Date.now()) / 60)
  const at = hours.value.findIndex((h) => Number(h.date) >= hour)
  return at === -1 ? hours.value.length - 1 : at
})
</script>

<template>
  <template v-if="line">
    <slot name="head" :day="day" />
    <dl class="admin-rows">
      <div>
        <dt>Start to end</dt>
        <dd>{{ tripTime(line.trip_sec) }}</dd>
      </div>
      <div>
        <dt>Buses came</dt>
        <dd>{{ line.delay_sec != null ? came(line.delay_sec) : '–' }}</dd>
      </div>
      <div v-if="line.slowest">
        <dt class="name">{{ line.slowest.from }} → {{ line.slowest.to }}</dt>
        <dd>+{{ minSec(line.slowest.extra_sec) }} at {{ hourLabel(line.slowest.hour) }}</dd>
      </div>
    </dl>
    <AdminDayBars v-if="hours.some((h) => h.value)" :key="direction" :days="hours" :current="currentHour" />
  </template>
  <p v-else-if="loadError" class="admin-note admin-bad">Could not load ({{ loadError }})</p>
</template>

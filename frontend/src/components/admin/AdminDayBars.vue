<script setup lang="ts">
import {computed, ref, watch} from 'vue'

export type DayBar = {
  date: string
  value: number
  caption: string
}

const props = defineProps<{
  days: DayBar[]
  max?: number
}>()

const selected = ref(props.days.length - 1)
watch(() => props.days.length, (n) => { selected.value = n - 1 })

const top = computed(() => props.max ?? Math.max(1, ...props.days.map(d => d.value)))
const caption = computed(() => props.days[selected.value]?.caption ?? '')
</script>

<template>
  <div class="day-bars">
    <div class="bars" role="list">
      <button
        v-for="(day, i) in days"
        :key="day.date"
        type="button"
        role="listitem"
        class="bar"
        :class="{ 'is-today': i === days.length - 1, 'is-selected': i === selected }"
        :aria-label="day.caption"
        @click="selected = i"
        @pointerenter="selected = i"
      >
        <span :style="{ height: `${Math.max(0, Math.min(1, day.value / top)) * 100}%` }"></span>
      </button>
    </div>
    <p class="caption">{{ caption }}</p>
  </div>
</template>

<style scoped>
.bars {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 3.5rem;
  border-bottom: 1px solid var(--a-rule);
}

.bar {
  flex: 1;
  height: 100%;
  display: flex;
  align-items: flex-end;
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
}

.bar span {
  display: block;
  width: 100%;
  min-height: 1px;
  border-radius: 2px 2px 0 0;
  background: var(--a-bar);
}

.bar.is-today span {
  background: var(--a-bar-strong);
}

.bar.is-selected span {
  background: var(--a-text);
}

.caption {
  margin: 0.375rem 0 0;
  font-size: 0.75rem;
  color: var(--a-muted);
  font-variant-numeric: tabular-nums;
}
</style>

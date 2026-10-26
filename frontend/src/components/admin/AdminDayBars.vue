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
watch(
  () => props.days.length,
  (n) => {
    selected.value = n - 1
  },
)

const top = computed(() => props.max ?? Math.max(1, ...props.days.map((d) => d.value)))
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
        :class="{'is-today': i === days.length - 1, 'is-selected': i === selected}"
        :aria-label="day.caption"
        @click="selected = i"
        @pointerenter="selected = i"
      >
        <span :style="{height: `${Math.max(0, Math.min(1, day.value / top)) * 100}%`}"></span>
      </button>
    </div>
    <p class="caption">{{ caption }}</p>
  </div>
</template>

<style scoped>
.day-bars {
  padding: 8px 16px 4px;
}

.bars {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 64px;
  border-bottom: 1px solid var(--md-outline-variant);
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
  border-radius: 3px 3px 0 0;
  background: color-mix(in srgb, var(--md-primary) 30%, transparent);
  transition: background-color 130ms linear;
}

.bar.is-today span {
  background: color-mix(in srgb, var(--md-primary) 60%, transparent);
}

.bar.is-selected span {
  background: var(--md-primary);
}

.caption {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  color: var(--md-on-surface-variant);
  font-variant-numeric: tabular-nums;
}
</style>

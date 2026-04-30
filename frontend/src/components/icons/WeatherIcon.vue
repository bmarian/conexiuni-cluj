<script setup lang="ts">
import { computed } from 'vue'

interface WeatherIconProps {
  slug: string
  size?: number | string
}

const props = withDefaults(defineProps<WeatherIconProps>(), {
  size: 64
})

// Specifically, include only used icons to avoid bloating the bundle.
const icons = import.meta.glob(
  '../../../node_modules/@meteocons/svg/fill/{clear-day,clear-night,partly-cloudy-day,partly-cloudy-night,cloudy,overcast-night,fog-day,fog-night,partly-cloudy-day-drizzle,partly-cloudy-night-drizzle,drizzle,partly-cloudy-day-sleet,partly-cloudy-night-sleet,sleet,partly-cloudy-day-rain,partly-cloudy-night-rain,rain,partly-cloudy-day-snow,partly-cloudy-night-snow,snow,snowflake,thunderstorms-day-rain,thunderstorms-night-rain,not-available}.svg',
  { query: '?url', import: 'default', eager: true }
) as Record<string, string>

const src = computed(() => {
  if (!props.slug) return ''
  const path = `../../../node_modules/@meteocons/svg/fill/${props.slug}.svg`
  return icons[path] || ''
})
</script>

<template>
  <img
    v-if="src"
    v-bind="$attrs"
    :src="src"
    :alt="slug"
    :style="{
      width: typeof size === 'number' ? `${size}px` : size,
      height: typeof size === 'number' ? `${size}px` : size
    }"
  />
  <div
    v-else
    v-bind="$attrs"
    :style="{
      width: typeof size === 'number' ? `${size}px` : size,
      height: typeof size === 'number' ? `${size}px` : size
    }"
  />
</template>

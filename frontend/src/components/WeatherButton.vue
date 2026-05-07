<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {useSettingsStore} from '@/stores/settings'
import WeatherIcon from './icons/WeatherIcon.vue'

const props = withDefaults(defineProps<{ topOffset?: string }>(), {topOffset: '3.5rem'})

const settings = useSettingsStore()
const isDark = computed(() => settings.isDark)

const topValue = computed(() => props.topOffset)

const temp = ref<number | null>(null)
const code = ref<number | null>(null)
const isDay = ref(true)

const WMO: Record<number, [string, string]> = {
  0: ['clear-day', 'clear-night'],
  1: ['clear-day', 'clear-night'],
  2: ['partly-cloudy-day', 'partly-cloudy-night'],
  3: ['cloudy', 'overcast-night'],
  45: ['fog-day', 'fog-night'],
  48: ['fog-day', 'fog-night'],
  51: ['partly-cloudy-day-drizzle', 'partly-cloudy-night-drizzle'],
  53: ['partly-cloudy-day-drizzle', 'partly-cloudy-night-drizzle'],
  55: ['drizzle', 'partly-cloudy-night-drizzle'],
  56: ['partly-cloudy-day-sleet', 'partly-cloudy-night-sleet'],
  57: ['sleet', 'partly-cloudy-night-sleet'],
  61: ['partly-cloudy-day-rain', 'partly-cloudy-night-rain'],
  63: ['rain', 'partly-cloudy-night-rain'],
  65: ['rain', 'partly-cloudy-night-rain'],
  66: ['partly-cloudy-day-sleet', 'partly-cloudy-night-sleet'],
  67: ['sleet', 'partly-cloudy-night-sleet'],
  71: ['partly-cloudy-day-snow', 'partly-cloudy-night-snow'],
  73: ['snow', 'partly-cloudy-night-snow'],
  75: ['snow', 'partly-cloudy-night-snow'],
  77: ['snowflake', 'snowflake'],
  80: ['partly-cloudy-day-rain', 'partly-cloudy-night-rain'],
  81: ['rain', 'partly-cloudy-night-rain'],
  82: ['rain', 'partly-cloudy-night-rain'],
  85: ['partly-cloudy-day-snow', 'partly-cloudy-night-snow'],
  86: ['snow', 'partly-cloudy-night-snow'],
  95: ['thunderstorms-day-rain', 'thunderstorms-night-rain'],
  96: ['thunderstorms-day-rain', 'thunderstorms-night-rain'],
  99: ['thunderstorms-day-rain', 'thunderstorms-night-rain'],
}

const iconSlug = computed(() => {
  if (code.value == null) return null
  const pair = WMO[code.value]
  if (!pair) return 'not-available'
  return isDay.value ? pair[0] : pair[1]
})

async function fetch_weather() {
  try {
    const res = await fetch(
      'https://api.open-meteo.com/v1/forecast?latitude=46.77&longitude=23.59&current=temperature_2m,weathercode,is_day&timezone=Europe%2FBucharest'
    )
    const data = await res.json()
    temp.value = Math.round(data.current.temperature_2m)
    code.value = data.current.weathercode
    isDay.value = data.current.is_day === 1
    scheduleNext(data.current.time, data.current.interval)
  } catch {
    // fail silently, retry in 10 min
    timer = setTimeout(fetch_weather, 10 * 60 * 1000)
  }
}

function scheduleNext(isoTime: string, intervalSec: number) {
  const dataTime = new Date(isoTime).getTime()
  const nextUpdate = dataTime + intervalSec * 1000
  const delay = Math.max(nextUpdate - Date.now() + 5_000, 30_000)
  timer = setTimeout(fetch_weather, delay)
}

let timer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  fetch_weather()
})

onUnmounted(() => {
  if (timer !== null) clearTimeout(timer)
})
</script>

<template>
  <div v-if="temp !== null" class="weather-root" :class="{ 'is-dark': isDark }">
    <div class="weather-pill">
      <WeatherIcon v-if="iconSlug" :slug="iconSlug" size="1.5rem" class="weather-icon" />
      <span class="weather-temp">{{ temp }}°</span>
    </div>
  </div>
</template>

<style scoped>
.weather-root {
  position: fixed;
  top: calc(v-bind(topValue) + env(safe-area-inset-top));
  right: calc(0.75rem + env(safe-area-inset-right));
  z-index: 3000;
  transition: right 250ms cubic-bezier(0.32, 0.72, 0, 1);
}

@media (max-width: 1023px) and (orientation: landscape) {
  .weather-root.landscape-open {
    right: calc(var(--landscape-drawer-width) + 0.75rem + env(safe-area-inset-right));
  }
}

@media (min-width: 1024px) {
  .weather-root {
    right: calc(30vw + 0.75rem + env(safe-area-inset-right));
  }
}

.weather-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0 0.6rem;
  height: 2.25rem;
  border: 0;
  border-radius: 0.875rem;
  background: #ffffff;
  color: #334155;
  box-shadow: 0 2px 10px -1px rgba(0, 0, 0, 0.14), 0 1px 3px rgba(0, 0, 0, 0.08);
  font-size: 0.8rem;
  font-weight: 600;
  white-space: nowrap;
  user-select: none;
}

.weather-root.is-dark .weather-pill {
  background: #0f172a;
  color: #f1f5f9;
  box-shadow: 0 4px 16px -2px rgba(0, 0, 0, 0.4), 0 1px 4px rgba(0, 0, 0, 0.24);
}

.weather-icon {
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
}

.weather-temp {
  font-variant-numeric: tabular-nums;
}
</style>

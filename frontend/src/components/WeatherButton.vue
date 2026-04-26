<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'

const settings = useSettingsStore()
const isDark = computed(() => settings.isDark)

const temp = ref<number | null>(null)
const code = ref<number | null>(null)
const isDay = ref(true)

const WMO: Record<number, string> = {
  0: '☀️',
  1: '🌤️', 2: '⛅', 3: '☁️',
  45: '🌫️', 48: '🌫️',
  51: '🌦️', 53: '🌦️', 55: '🌦️',
  56: '🌧️', 57: '🌧️',
  61: '🌧️', 63: '🌧️', 65: '🌧️',
  66: '🌧️', 67: '🌧️',
  71: '🌨️', 73: '🌨️', 75: '❄️', 77: '❄️',
  80: '🌦️', 81: '🌦️', 82: '🌦️',
  85: '🌨️', 86: '🌨️',
  95: '⛈️', 96: '⛈️', 99: '⛈️',
}

const WMO_NIGHT: Partial<Record<number, string>> = {
  0: '🌙',
  1: '🌙',
  2: '🌛',
}

const emoji = computed(() => {
  if (code.value == null) return null
  if (!isDay.value && code.value in WMO_NIGHT) return WMO_NIGHT[code.value]!
  return WMO[code.value] ?? '🌡️'
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
  } catch {
    // fail silently — widget just stays hidden
  }
}

let timer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  fetch_weather()
  timer = setInterval(fetch_weather, 10 * 60 * 1000)
})

onUnmounted(() => {
  if (timer !== null) clearInterval(timer)
})
</script>

<template>
  <div v-if="temp !== null" class="weather-root" :class="{ 'is-dark': isDark }">
    <div class="weather-pill">
      <span class="weather-emoji" aria-hidden="true">{{ emoji }}</span>
      <span class="weather-temp">{{ temp }}°</span>
    </div>
  </div>
</template>

<style scoped>
.weather-root {
  position: fixed;
  top: calc(3.5rem + env(safe-area-inset-top));
  right: calc(0.75rem + env(safe-area-inset-right));
  z-index: 3000;
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

.weather-emoji {
  font-size: 0.95rem;
  line-height: 1;
}

.weather-temp {
  font-variant-numeric: tabular-nums;
}
</style>

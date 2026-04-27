<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {useSettingsStore} from '@/stores/settings'

import iClearDay from '@meteocons/svg/fill/clear-day.svg?url'
import iClearNight from '@meteocons/svg/fill/clear-night.svg?url'
import iPartlyCloudyDay from '@meteocons/svg/fill/partly-cloudy-day.svg?url'
import iPartlyCloudyNight from '@meteocons/svg/fill/partly-cloudy-night.svg?url'
import iCloudy from '@meteocons/svg/fill/cloudy.svg?url'
import iOvercastNight from '@meteocons/svg/fill/overcast-night.svg?url'
import iFogDay from '@meteocons/svg/fill/fog-day.svg?url'
import iFogNight from '@meteocons/svg/fill/fog-night.svg?url'
import iDrizzle from '@meteocons/svg/fill/drizzle.svg?url'
import iPCDayDrizzle from '@meteocons/svg/fill/partly-cloudy-day-drizzle.svg?url'
import iPCNightDrizzle from '@meteocons/svg/fill/partly-cloudy-night-drizzle.svg?url'
import iSleet from '@meteocons/svg/fill/sleet.svg?url'
import iPCDaySleet from '@meteocons/svg/fill/partly-cloudy-day-sleet.svg?url'
import iPCNightSleet from '@meteocons/svg/fill/partly-cloudy-night-sleet.svg?url'
import iRain from '@meteocons/svg/fill/rain.svg?url'
import iPCDayRain from '@meteocons/svg/fill/partly-cloudy-day-rain.svg?url'
import iPCNightRain from '@meteocons/svg/fill/partly-cloudy-night-rain.svg?url'
import iSnow from '@meteocons/svg/fill/snow.svg?url'
import iSnowflake from '@meteocons/svg/fill/snowflake.svg?url'
import iPCDaySnow from '@meteocons/svg/fill/partly-cloudy-day-snow.svg?url'
import iPCNightSnow from '@meteocons/svg/fill/partly-cloudy-night-snow.svg?url'
import iThunderstormsDayRain from '@meteocons/svg/fill/thunderstorms-day-rain.svg?url'
import iThunderstormsNightRain from '@meteocons/svg/fill/thunderstorms-night-rain.svg?url'
import iNotAvailable from '@meteocons/svg/fill/not-available.svg?url'

const settings = useSettingsStore()
const isDark = computed(() => settings.isDark)

const temp = ref<number | null>(null)
const code = ref<number | null>(null)
const isDay = ref(true)

// WMO code → [dayIcon, nightIcon]
const WMO: Record<number, [string, string]> = {
  0: [iClearDay, iClearNight],
  1: [iClearDay, iClearNight],
  2: [iPartlyCloudyDay, iPartlyCloudyNight],
  3: [iCloudy, iOvercastNight],
  45: [iFogDay, iFogNight],
  48: [iFogDay, iFogNight],
  51: [iPCDayDrizzle, iPCNightDrizzle],
  53: [iPCDayDrizzle, iPCNightDrizzle],
  55: [iDrizzle, iPCNightDrizzle],
  56: [iPCDaySleet, iPCNightSleet],
  57: [iSleet, iPCNightSleet],
  61: [iPCDayRain, iPCNightRain],
  63: [iRain, iPCNightRain],
  65: [iRain, iPCNightRain],
  66: [iPCDaySleet, iPCNightSleet],
  67: [iSleet, iPCNightSleet],
  71: [iPCDaySnow, iPCNightSnow],
  73: [iSnow, iPCNightSnow],
  75: [iSnow, iPCNightSnow],
  77: [iSnowflake, iSnowflake],
  80: [iPCDayRain, iPCNightRain],
  81: [iRain, iPCNightRain],
  82: [iRain, iPCNightRain],
  85: [iPCDaySnow, iPCNightSnow],
  86: [iSnow, iPCNightSnow],
  95: [iThunderstormsDayRain, iThunderstormsNightRain],
  96: [iThunderstormsDayRain, iThunderstormsNightRain],
  99: [iThunderstormsDayRain, iThunderstormsNightRain],
}

const iconSrc = computed(() => {
  if (code.value == null) return null
  const pair = WMO[code.value]
  if (!pair) return iNotAvailable
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
      <img v-if="iconSrc" :src="iconSrc" class="weather-icon" alt=""/>
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

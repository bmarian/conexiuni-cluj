<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'
import WeatherIcon from './icons/WeatherIcon.vue'

const props = withDefaults(defineProps<{ topOffset?: string }>(), {topOffset: '3.5rem'})

const {locale, t} = useI18n()
const settings = useSettingsStore()
const isDark = computed(() => settings.isDark)

const topValue = computed(() => props.topOffset)

const temp = ref<number | null>(null)
const code = ref<number | null>(null)
const isDay = ref(true)
const apparentTemp = ref<number | null>(null)
const humidity = ref<number | null>(null)
const windSpeed = ref<number | null>(null)
const highTemp = ref<number | null>(null)
const lowTemp = ref<number | null>(null)
const precipitationChance = ref<number | null>(null)
const sunrise = ref<string | null>(null)
const sunset = ref<string | null>(null)
const isOpen = ref(false)
const rootRef = ref<HTMLElement | null>(null)

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

const WMO_LABELS: Record<number, { en: string, ro: string }> = {
  0: {en: 'Clear sky', ro: 'Cer senin'},
  1: {en: 'Mostly clear', ro: 'Mai mult senin'},
  2: {en: 'Partly cloudy', ro: 'Parțial noros'},
  3: {en: 'Overcast', ro: 'Noros'},
  45: {en: 'Fog', ro: 'Ceață'},
  48: {en: 'Rime fog', ro: 'Ceață cu depuneri'},
  51: {en: 'Light drizzle', ro: 'Burniță slabă'},
  53: {en: 'Drizzle', ro: 'Burniță'},
  55: {en: 'Dense drizzle', ro: 'Burniță densă'},
  56: {en: 'Freezing drizzle', ro: 'Burniță înghețată'},
  57: {en: 'Dense freezing drizzle', ro: 'Burniță înghețată densă'},
  61: {en: 'Light rain', ro: 'Ploaie slabă'},
  63: {en: 'Rain', ro: 'Ploaie'},
  65: {en: 'Heavy rain', ro: 'Ploaie puternică'},
  66: {en: 'Freezing rain', ro: 'Ploaie înghețată'},
  67: {en: 'Heavy freezing rain', ro: 'Ploaie înghețată puternică'},
  71: {en: 'Light snow', ro: 'Ninsoare slabă'},
  73: {en: 'Snow', ro: 'Ninsoare'},
  75: {en: 'Heavy snow', ro: 'Ninsoare abundentă'},
  77: {en: 'Snow grains', ro: 'Măzăriche'},
  80: {en: 'Light showers', ro: 'Averse slabe'},
  81: {en: 'Showers', ro: 'Averse'},
  82: {en: 'Heavy showers', ro: 'Averse puternice'},
  85: {en: 'Snow showers', ro: 'Averse de ninsoare'},
  86: {en: 'Heavy snow showers', ro: 'Averse puternice de ninsoare'},
  95: {en: 'Thunderstorm', ro: 'Furtună'},
  96: {en: 'Thunderstorm with hail', ro: 'Furtună cu grindină'},
  99: {en: 'Strong thunderstorm with hail', ro: 'Furtună puternică cu grindină'},
}

const iconSlug = computed(() => {
  if (code.value == null) return null
  const pair = WMO[code.value]
  if (!pair) return 'not-available'
  return isDay.value ? pair[0] : pair[1]
})

const conditionLabel = computed(() => {
  if (code.value == null) return t('weatherUnknown')
  const labels = WMO_LABELS[code.value]
  if (!labels) return t('weatherUnknown')
  return locale.value === 'en' ? labels.en : labels.ro
})

const formattedSunrise = computed(() => formatTime(sunrise.value))
const formattedSunset = computed(() => formatTime(sunset.value))

async function fetch_weather() {
  try {
    const res = await fetch(
      'https://api.open-meteo.com/v1/forecast?latitude=46.77&longitude=23.59&current=temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,is_day,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max,sunrise,sunset&timezone=Europe%2FBucharest&forecast_days=1'
    )
    if (!res.ok) throw new Error()
    const data = await res.json()
    temp.value = roundOrNull(data.current?.temperature_2m)
    apparentTemp.value = roundOrNull(data.current?.apparent_temperature)
    humidity.value = roundOrNull(data.current?.relative_humidity_2m)
    windSpeed.value = roundOrNull(data.current?.wind_speed_10m)
    code.value = data.current?.weather_code ?? data.current?.weathercode ?? null
    isDay.value = data.current?.is_day === 1
    highTemp.value = roundOrNull(data.daily?.temperature_2m_max?.[0])
    lowTemp.value = roundOrNull(data.daily?.temperature_2m_min?.[0])
    precipitationChance.value = roundOrNull(data.daily?.precipitation_probability_max?.[0])
    sunrise.value = data.daily?.sunrise?.[0] ?? null
    sunset.value = data.daily?.sunset?.[0] ?? null
    scheduleNext(data.current?.time, data.current?.interval)
  } catch {
    timer = setTimeout(fetch_weather, 10 * 60 * 1000)
  }
}

function roundOrNull(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? Math.round(value) : null
}

function formatTime(value: string | null): string {
  if (!value) return '—'
  return new Intl.DateTimeFormat(locale.value === 'en' ? 'en-US' : 'ro-RO', {
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function scheduleNext(isoTime?: string, intervalSec?: number) {
  if (!isoTime || !intervalSec) {
    timer = setTimeout(fetch_weather, 10 * 60 * 1000)
    return
  }
  const dataTime = new Date(isoTime).getTime()
  const nextUpdate = dataTime + intervalSec * 1000
  const delay = Math.max(nextUpdate - Date.now() + 5_000, 30_000)
  timer = setTimeout(fetch_weather, delay)
}

function toggle() {
  isOpen.value = !isOpen.value
}

function onDocumentPointerDown(e: PointerEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

let timer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  document.addEventListener('pointerdown', onDocumentPointerDown)
  fetch_weather()
})

onUnmounted(() => {
  document.removeEventListener('pointerdown', onDocumentPointerDown)
  if (timer !== null) clearTimeout(timer)
})
</script>

<template>
  <div
    v-if="temp !== null"
    ref="rootRef"
    class="weather-root"
    :class="{ 'is-dark': isDark }"
    :style="isOpen ? { zIndex: 9999 } : {}"
  >
    <button
      type="button"
      class="weather-pill"
      :title="t('weather')"
      :aria-label="t('weather')"
      :aria-expanded="isOpen"
      @click="toggle"
    >
      <WeatherIcon v-if="iconSlug" :slug="iconSlug" size="1.5rem" class="weather-icon" />
      <span class="weather-temp">{{ temp }}°</span>
    </button>

    <div v-if="isOpen" class="weather-popover" role="dialog" :aria-label="t('weatherToday')">
      <div class="weather-popover-head">
        <WeatherIcon v-if="iconSlug" :slug="iconSlug" size="2.75rem" class="weather-popover-icon" />
        <div class="weather-popover-main">
          <p class="weather-popover-title">{{ t('weatherToday') }}</p>
          <p class="weather-condition">{{ conditionLabel }}</p>
        </div>
        <span class="weather-popover-temp">{{ temp }}°</span>
      </div>

      <div class="weather-range">
        <span>{{ t('weatherHigh') }} {{ highTemp ?? '—' }}°</span>
        <span>{{ t('weatherLow') }} {{ lowTemp ?? '—' }}°</span>
      </div>

      <div class="weather-grid">
        <div class="weather-stat">
          <span class="weather-stat-label">{{ t('weatherFeelsLike') }}</span>
          <span class="weather-stat-value">{{ apparentTemp ?? '—' }}°</span>
        </div>
        <div class="weather-stat">
          <span class="weather-stat-label">{{ t('weatherHumidity') }}</span>
          <span class="weather-stat-value">{{ humidity ?? '—' }}%</span>
        </div>
        <div class="weather-stat">
          <span class="weather-stat-label">{{ t('weatherWind') }}</span>
          <span class="weather-stat-value">{{ windSpeed ?? '—' }} km/h</span>
        </div>
        <div class="weather-stat">
          <span class="weather-stat-label">{{ t('weatherRain') }}</span>
          <span class="weather-stat-value">{{ precipitationChance ?? '—' }}%</span>
        </div>
      </div>

      <div class="weather-sun">
        <span>{{ t('weatherSunrise') }} {{ formattedSunrise }}</span>
        <span>{{ t('weatherSunset') }} {{ formattedSunset }}</span>
      </div>
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
  cursor: pointer;
  font-size: 0.8rem;
  font-weight: 600;
  white-space: nowrap;
  user-select: none;
  transition: background 150ms ease, color 150ms ease;
}

.weather-pill:hover {
  background: #f1f5f9;
  color: #0f172a;
}

.weather-root.is-dark .weather-pill {
  background: #0f172a;
  color: #f1f5f9;
  box-shadow: 0 4px 16px -2px rgba(0, 0, 0, 0.4), 0 1px 4px rgba(0, 0, 0, 0.24);
}

.weather-root.is-dark .weather-pill:hover {
  background: #1e293b;
  color: #f8fafc;
}

.weather-icon {
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
}

.weather-temp {
  font-variant-numeric: tabular-nums;
}

.weather-popover {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  width: min(19rem, calc(100vw - 1.5rem - env(safe-area-inset-left) - env(safe-area-inset-right)));
  max-height: calc(
    100dvh -
    (
      v-bind(topValue) +
      env(safe-area-inset-top) +
      2.25rem +
      0.5rem +
      env(safe-area-inset-bottom) +
      0.75rem
    )
  );
  overflow-y: auto;
  background: #ffffff;
  border-radius: 0.875rem;
  box-shadow: 0 10px 30px -4px rgba(0, 0, 0, 0.18), 0 1px 6px rgba(0, 0, 0, 0.08);
  padding: 0.875rem;
  color: #0f172a;
}

.weather-root.is-dark .weather-popover {
  background: #1e293b;
  color: #f8fafc;
  box-shadow: 0 10px 30px -4px rgba(0, 0, 0, 0.5), 0 1px 6px rgba(0, 0, 0, 0.24);
}

.weather-popover-head {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.weather-popover-icon {
  width: 2.75rem;
  height: 2.75rem;
  flex-shrink: 0;
}

.weather-popover-main {
  min-width: 0;
  flex: 1;
}

.weather-popover-title {
  margin: 0;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  color: #94a3b8;
  text-transform: uppercase;
}

.weather-root.is-dark .weather-popover-title {
  color: #94a3b8;
}

.weather-condition {
  margin: 0.125rem 0 0;
  color: #334155;
  font-size: 0.9rem;
  font-weight: 700;
  line-height: 1.2;
}

.weather-root.is-dark .weather-condition {
  color: #e2e8f0;
}

.weather-popover-temp {
  flex-shrink: 0;
  color: #0f172a;
  font-size: 1.75rem;
  font-weight: 800;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.weather-root.is-dark .weather-popover-temp {
  color: #f8fafc;
}

.weather-range {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  margin-top: 0.75rem;
  padding: 0.55rem 0.65rem;
  border-radius: 0.625rem;
  background: #f8fafc;
  color: #475569;
  font-size: 0.75rem;
  font-weight: 700;
}

.weather-root.is-dark .weather-range {
  background: #0f172a;
  color: #cbd5e1;
}

.weather-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
  margin-top: 0.75rem;
}

.weather-stat {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.125rem;
  padding: 0.55rem 0.65rem;
  border: 1px solid #e2e8f0;
  border-radius: 0.625rem;
  background: #ffffff;
}

.weather-root.is-dark .weather-stat {
  border-color: #334155;
  background: #0f172a;
}

.weather-stat-label {
  color: #94a3b8;
  font-size: 0.65rem;
  font-weight: 600;
  letter-spacing: 0.03em;
  text-transform: uppercase;
}

.weather-root.is-dark .weather-stat-label {
  color: #64748b;
}

.weather-stat-value {
  color: #1e293b;
  font-size: 0.85rem;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}

.weather-root.is-dark .weather-stat-value {
  color: #f1f5f9;
}

.weather-sun {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  margin-top: 0.75rem;
  color: #64748b;
  font-size: 0.72rem;
  font-weight: 600;
}

.weather-root.is-dark .weather-sun {
  color: #94a3b8;
}
</style>

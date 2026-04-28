<script setup lang="ts">
import {computed, onMounted, onUnmounted, watch} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {useMapStore} from '@/stores/map.ts'
import {useUserStore} from '@/stores/user.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import IconBack from '@/components/icons/IconBack.vue'
import {closestStop} from "@/utils/geo.ts";
import {apiRequest} from "@/utils/request_cache.ts";
import type {Stop, StopInfo} from "@/types/tranzy.ts";
import {storeToRefs} from "pinia";

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const mapStore = useMapStore()
const userStore = useUserStore()
const settings = useSettingsStore()
const {userLocation} = storeToRefs(userStore)

const destName = computed(() => (route.query.name as string | undefined) ?? '')
const destLat = computed(() => parseFloat((route.query.lat as string) ?? 'NaN'))
const destLon = computed(() => parseFloat((route.query.lot as string) ?? 'NaN'))

const hasValidDest = computed(() => destName.value.length > 0)
const hasValidCoords = computed(() => !isNaN(destLat.value) && !isNaN(destLon.value))

const originLabel = computed(() => {
  if (userStore.userLocation) return t('planOriginCurrentLocation')
  return t('planOriginUnknown')
})

onMounted(() => {
  mapStore.setHighlightedStops([])
  mapStore.setVehiclesToDisplay([])
  mapStore.setShapesToDisplay([])
  if (hasValidCoords.value) {
    mapStore.setFlyToLocation(destLat.value, destLon.value)
    mapStore.setPinnedLocation(destLat.value, destLon.value, destName.value)
  }
})

onUnmounted(() => {
  mapStore.clearPinnedLocation()
  mapStore.setVehiclesToDisplay([])
  mapStore.setShapesToDisplay([])
})

function goBack() {
  router.replace({name: 'home'})
}

const stopWatcher = watch([destLat, destLon, userLocation], async ([lat, lon, ul]) => {
  if (Number.isNaN(lat) || Number.isNaN(lon) || !ul) return

  const stops = await apiRequest('stops') as Stop[]
  if (!Array.isArray(stops) || !stops.length) return

  const closestStopToDestination = closestStop(lat, lon, stops) as Stop
  const closestStopToUser = closestStop(ul.latitude, ul.longitude, stops) as Stop

  if (!closestStopToDestination || !closestStopToUser) return
  const destinationStop = await apiRequest(`stop_info?stop_id=${closestStopToDestination.stop_id}`) as Promise<StopInfo>
  const startStop = await apiRequest(`stop_info?stop_id=${closestStopToUser.stop_id}`) as Promise<StopInfo>

  console.log('destinationStop', destinationStop)
  console.log('startStop', startStop)

  stopWatcher()
}, {immediate: true})

// using openrouteservice 2000 requests/day 40/min directionsV2 API - to go to this stop

</script>

<template>
  <div
    class="plan-view-container bg-white dark:bg-[#0f172a] text-slate-800 dark:text-slate-100 flex flex-col gap-6">

    <div class="flex items-center -mb-2">
      <button
        @click="goBack"
        class="flex items-center gap-1.5 px-2 py-1.5 rounded-xl text-sm font-semibold text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200 transition-colors duration-150"
      >
        <IconBack class="w-4 h-4"/>
        {{ t('back') }}
      </button>
    </div>

    <header class="flex items-center gap-3">
      <div
        class="w-12 h-12 shrink-0 rounded-2xl bg-gradient-to-br from-sky-400 to-sky-600 flex items-center justify-center shadow-lg shadow-sky-500/20">
        <span v-if="settings.traditionalActive" class="emoji-icon-xl" aria-hidden="true">🗺️</span>
        <svg v-else class="w-6 h-6 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round"
                d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <span
          class="text-[10px] font-semibold text-sky-600 dark:text-sky-400 tracking-wide uppercase">{{
            t('planTitle')
          }}</span>
        <h1
          class="text-xl font-black tracking-tight text-slate-900 dark:text-white leading-tight truncate">
          {{ hasValidDest ? destName : t('planTitleGeneric') }}
        </h1>
      </div>
    </header>

    <section class="route-legs-card">
      <div class="leg-row">
        <div class="leg-icon-col">
          <div class="leg-dot leg-dot-origin">
            <svg class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
              <circle cx="12" cy="12" r="6"/>
            </svg>
          </div>
          <div class="leg-line"></div>
        </div>
        <div class="leg-label-col">
          <span class="leg-type-badge">{{ t('planFrom') }}</span>
          <span class="leg-name">{{ originLabel }}</span>
        </div>
      </div>

      <div class="leg-row">
        <div class="leg-icon-col">
          <div class="leg-dot leg-dot-dest">
            <svg class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="currentColor">
              <path
                d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
            </svg>
          </div>
        </div>
        <div class="leg-label-col">
          <span class="leg-type-badge leg-type-badge-dest">{{ t('planTo') }}</span>
          <span class="leg-name">{{ hasValidDest ? destName : '—' }}</span>
        </div>
      </div>
    </section>

    <section class="flex flex-col gap-3 pb-8">
      <h2 class="section-label">
        <span class="w-2 h-2 rounded-full bg-sky-500 shrink-0"></span>
        {{ t('planRoutesLabel') }}
      </h2>

      <div class="plan-placeholder">
        <svg class="w-10 h-10 text-slate-300 dark:text-slate-700 mb-3" viewBox="0 0 24 24"
             fill="none" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round"
                d="M8.25 18.75a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 01-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 00-3.213-9.193 2.056 2.056 0 00-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 00-10.026 0 1.106 1.106 0 00-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12"/>
        </svg>
        <p class="text-sm font-medium text-slate-400 dark:text-slate-500 text-center">
          {{ t('planComingSoon') }}</p>
        <p class="text-xs text-slate-300 dark:text-slate-600 text-center mt-1">
          {{ t('planComingSoonDesc') }}</p>
      </div>
    </section>

  </div>
</template>

<style scoped>
.plan-view-container {
  padding: 1.25rem 1.5rem 0;
  height: 100%;
  overflow-y: auto;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}

.section-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: #64748b;
  margin-bottom: 0.25rem;
}

.route-legs-card {
  display: flex;
  flex-direction: column;
  background: #f8fafc;
  border: 1.5px solid #e2e8f0;
  border-radius: 1rem;
  padding: 0.875rem 1rem;
  gap: 0;
}

.leg-row {
  display: flex;
  align-items: stretch;
  gap: 0.875rem;
  min-height: 2.5rem;
}

.leg-icon-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 1.5rem;
  flex-shrink: 0;
}

.leg-dot {
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 0.125rem;
}

.leg-dot-origin {
  background: #0ea5e9;
  box-shadow: 0 0 0 3px #e0f2fe;
}

.leg-dot-dest {
  background: #0284c7;
  box-shadow: 0 0 0 3px #bae6fd;
}

.leg-line {
  flex: 1;
  width: 2px;
  background: repeating-linear-gradient(to bottom, #7dd3fc 0, #7dd3fc 4px, transparent 4px, transparent 8px);
  margin: 0.25rem 0;
  min-height: 0.75rem;
}

.leg-label-col {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0.125rem;
  flex: 1;
  min-width: 0;
  padding-bottom: 0.625rem;
}

.leg-row:last-child .leg-label-col {
  padding-bottom: 0;
}

.leg-type-badge {
  font-size: 0.625rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #0ea5e9;
}

.leg-type-badge-dest {
  color: #0284c7;
}

.leg-name {
  font-size: 0.9375rem;
  font-weight: 600;
  color: #0f172a;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plan-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2.5rem 1rem;
  border: 1.5px dashed #e2e8f0;
  border-radius: 1rem;
  background: #fafafa;
}
</style>

import {computed, ref} from 'vue'
import {defineStore} from 'pinia'
import type {UserLocation} from "@/types/tranzy.ts"
import {useSettingsStore} from './settings'

export const useUserStore = defineStore('user', () => {
  const userLocation = ref<UserLocation | null>(null)
  const hasLocationPermission = ref(true)
  const positionWatchId = ref<number | null>(null)
  const setUserLocation = (lat: number, lon: number) => {
    userLocation.value = {
      latitude: lat,
      longitude: lon,
    }
  }
  const setHasLocationPermission = (permission: boolean) => {
    hasLocationPermission.value = permission
  }
  const clearUserLocation = () => {
    userLocation.value = null
  }

  const userTime = ref<Date | null>(null)
  const timerIntervalId = ref<number | null>(null)
  const startTimeTracker = () => {
    userTime.value = new Date()
    timerIntervalId.value = setInterval(() => {
      userTime.value = new Date()
    }, 10000)
  }

  const isDarkMode = computed(() => useSettingsStore().isDark)

  return {
    userLocation,
    hasLocationPermission,
    userTime,
    isDarkMode,

    setUserLocation,
    clearUserLocation,
    setHasLocationPermission,
    startTimeTracker,

    positionWatchId,
    timerIntervalId,
  }
})

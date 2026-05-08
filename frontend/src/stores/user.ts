import {computed, ref} from 'vue'
import {defineStore} from 'pinia'
import type {UserLocation} from "@/types/tranzy.ts"
import {useSettingsStore} from './settings'

export const useUserStore = defineStore('user', () => {
  const userLocation = ref<UserLocation | null>(null)
  const hasLocationPermission = ref(false)
  const isLocating = ref(false)
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
  const setIsLocating = (locating: boolean) => {
    isLocating.value = locating
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
    isLocating,
    userTime,
    isDarkMode,

    setUserLocation,
    clearUserLocation,
    setHasLocationPermission,
    setIsLocating,
    startTimeTracker,

    positionWatchId,
    timerIntervalId,
  }
})

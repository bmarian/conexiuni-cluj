import {ref} from 'vue'
import {defineStore} from 'pinia'
import type {UserLocation} from "@/types/tranzy.ts"

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

  const userTime = ref<Date | null>(null)
  const timerIntervalId = ref<number | null>(null)
  const startTimeTracker = () => {
    userTime.value = new Date()
    timerIntervalId.value = setInterval(() => {
      userTime.value = new Date()
    }, 10000)
  }

  const darkModeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  const isDarkMode = ref(darkModeMediaQuery.matches)

  const startSchemeWatcher = () => {
    darkModeMediaQuery.addEventListener('change', (event) => {
      isDarkMode.value = event.matches
    })
  }
  return {
    userLocation,
    hasLocationPermission,
    userTime,
    isDarkMode,

    setUserLocation,
    setHasLocationPermission,
    startTimeTracker,
    startSchemeWatcher,

    positionWatchId,
    timerIntervalId,
  }
})

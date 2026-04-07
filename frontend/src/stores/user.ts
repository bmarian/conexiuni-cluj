import { ref } from 'vue'
import { defineStore } from 'pinia'
import type {UserLocation} from "@/types/tranzy.ts"

export const useUserStore = defineStore('user', () => {
  const userLocation = ref<UserLocation | null>(null)
  const userTime = ref<Date | null>(null)
  const hasLocationPermission = ref(true)

  const positionWatchId = ref<number | null>(null)
  const timerIntervalId = ref<number | null>(null)

  const startLocationTracker = () => {
    if (!navigator.geolocation) {
      hasLocationPermission.value = false
    }
    if (!hasLocationPermission.value) {
      return
    }

    const success = (position: GeolocationPosition) => {
      userLocation.value = {
        latitude: position.coords.latitude,
        longitude: position.coords.longitude,
      }
    }
    const error = (error: GeolocationPositionError) => {
      hasLocationPermission.value = false
    }
    navigator.geolocation.getCurrentPosition(success, error)
    positionWatchId.value = navigator.geolocation.watchPosition(success, error)
  }

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
    userTime,
    isDarkMode,

    startLocationTracker,
    startTimeTracker,
    startSchemeWatcher,

    positionWatchId,
    timerIntervalId,
  }
})

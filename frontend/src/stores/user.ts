import { ref } from 'vue'
import { defineStore } from 'pinia'
import type {UserLocation} from "@/types/tranzy.ts"

export const useUserStore = defineStore('user', () => {
  const currentLocation = ref<UserLocation | null>(null)
  const isLocationPermissionDenied = ref(false)
  let locationInterval: ReturnType<typeof setInterval> | null = null

  const updateCurrentLocation = () => {
    if (!navigator.geolocation || isLocationPermissionDenied.value) {
      return
    }

    navigator.geolocation.getCurrentPosition(
      (position) => {
        currentLocation.value = {
          latitude: position.coords.latitude,
          longitude: position.coords.longitude,
        }
      },
      (error) => {
        if (error.code === error.PERMISSION_DENIED) {
          isLocationPermissionDenied.value = true
          stopLocationTracking()
        }
      },
    )
  }

  const stopLocationTracking = () => {
    if (!locationInterval) {
      return
    }

    clearInterval(locationInterval)
    locationInterval = null
  }

  const getUserLocationConsent = () => {
    if (!navigator.geolocation || isLocationPermissionDenied.value) {
      return
    }

    navigator.geolocation.getCurrentPosition(
      () => {
        updateCurrentLocation()
        locationInterval = setInterval(updateCurrentLocation, 1000)
      },
      (error) => {
        if (error.code === error.PERMISSION_DENIED) {
          isLocationPermissionDenied.value = true
          stopLocationTracking()
        }
      },
    )
  }

  return {
    currentLocation,
    isLocationPermissionDenied,
    getUserLocationConsent,
  }
})

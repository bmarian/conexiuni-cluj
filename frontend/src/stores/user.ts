import { ref } from 'vue'
import { defineStore } from 'pinia'
import type {UserLocation} from "@/types/tranzy.ts"

export const useUserStore = defineStore('user', () => {
  const currentLocation = ref<UserLocation | null>(null)
  const isLocationPermissionDenied = ref(false)

  const updateUserCurrentLocation = () => {
    if (!navigator.geolocation || isLocationPermissionDenied.value) {
      return
    }

    navigator.geolocation.getCurrentPosition(
      (position) => {
        currentLocation.value = {
          latitude: position.coords.latitude,
          longitude: position.coords.longitude,
        }
        updateUserCurrentLocation()
      },
      (error) => {
        if (error.code === error.PERMISSION_DENIED) {
          isLocationPermissionDenied.value = true
        }
      },
    )
  }

  return {
    currentLocation,
    isLocationPermissionDenied,
    updateUserCurrentLocation,
  }
})

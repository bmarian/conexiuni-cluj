import {onMounted, onUnmounted, ref} from 'vue'
import {useUserStore} from '@/stores/user.ts'

export type WatchLocationStatus = 'locating' | 'found' | 'denied' | 'unavailable'

// The watch has no map running in the background, so the shell asks for the position itself.
export function useWatchLocation() {
  const userStore = useUserStore()
  const status = ref<WatchLocationStatus>(userStore.userLocation ? 'found' : 'locating')
  let watchId: number | null = null

  function start() {
    if (watchId !== null) return
    if (!navigator.geolocation || !window.isSecureContext) {
      status.value = 'unavailable'
      return
    }
    watchId = navigator.geolocation.watchPosition(
      (position) => {
        status.value = 'found'
        userStore.setHasLocationPermission(true)
        userStore.setUserLocation(position.coords.latitude, position.coords.longitude)
      },
      (error) => {
        if (error.code === error.PERMISSION_DENIED) {
          status.value = 'denied'
          userStore.setHasLocationPermission(false)
          stop()
        } else if (!userStore.userLocation) {
          status.value = 'unavailable'
        }
      },
      {enableHighAccuracy: false, maximumAge: 30000, timeout: 20000},
    )
  }

  function stop() {
    if (watchId === null) return
    navigator.geolocation.clearWatch(watchId)
    watchId = null
  }

  function onVisibility() {
    if (document.hidden) stop()
    else if (status.value !== 'denied') start()
  }

  onMounted(() => {
    start()
    document.addEventListener('visibilitychange', onVisibility)
  })

  onUnmounted(() => {
    stop()
    document.removeEventListener('visibilitychange', onVisibility)
  })

  return {status}
}

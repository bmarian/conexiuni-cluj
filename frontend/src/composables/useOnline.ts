import { onMounted, onUnmounted, ref } from 'vue'

const isOnline = ref(typeof navigator === 'undefined' ? true : navigator.onLine)

let listenersAttached = false
let refCount = 0

function handleOnline() {
  isOnline.value = true
}
function handleOffline() {
  isOnline.value = false
}

export function useOnline() {
  onMounted(() => {
    refCount++
    if (!listenersAttached && typeof window !== 'undefined') {
      window.addEventListener('online', handleOnline)
      window.addEventListener('offline', handleOffline)
      listenersAttached = true
    }
  })

  onUnmounted(() => {
    refCount--
    if (refCount <= 0 && listenersAttached && typeof window !== 'undefined') {
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
      listenersAttached = false
      refCount = 0
    }
  })

  return { isOnline }
}

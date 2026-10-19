import {onMounted, onUnmounted, ref, type Ref} from 'vue'
import {standaloneBottomCutoffPx} from '@/utils/ios.ts'

export function useSheetMetrics(sheet: Ref<HTMLElement | null>) {
  const height = ref(window.innerHeight)
  const safeTop = ref(0)
  const safeBottom = ref(0)
  let observer: ResizeObserver | null = null

  function measure() {
    if (sheet.value) height.value = sheet.value.offsetHeight
    const root = getComputedStyle(document.documentElement)
    safeTop.value = parseFloat(root.getPropertyValue('--safe-top')) || 0
    safeBottom.value = Math.max(0, (parseFloat(root.getPropertyValue('--safe-bottom')) || 0) - standaloneBottomCutoffPx())
  }

  onMounted(() => {
    measure()
    if (sheet.value) {
      observer = new ResizeObserver(measure)
      observer.observe(sheet.value)
    }
    window.addEventListener('resize', measure, {passive: true})
  })

  onUnmounted(() => {
    observer?.disconnect()
    window.removeEventListener('resize', measure)
  })

  return {height, safeTop, safeBottom}
}

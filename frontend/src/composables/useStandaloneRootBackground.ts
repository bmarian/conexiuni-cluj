import {onMounted, onUnmounted, watch, type WatchSource} from 'vue'
import {isIOSStandalone} from '@/utils/ios.ts'

// The strip below a cut-short Home Screen viewport only ever shows the root background.
export function useStandaloneRootBackground(surface: () => Element | null | undefined, sources: WatchSource[]) {
  if (!isIOSStandalone) return

  function sync() {
    let color = ''
    for (let el = surface(); el && el !== document.documentElement; el = el.parentElement) {
      const bg = getComputedStyle(el).backgroundColor
      if (bg !== 'rgba(0, 0, 0, 0)' && bg !== 'transparent') {
        color = bg
        break
      }
    }
    document.documentElement.style.backgroundColor = color
  }

  const themeObserver = new MutationObserver((records) => {
    if (records.some((r) => r.attributeName !== 'style')) sync()
  })

  onMounted(() => {
    sync()
    themeObserver.observe(document.documentElement, {attributes: true})
  })

  onUnmounted(() => themeObserver.disconnect())

  watch(sources, sync, {flush: 'post'})
}

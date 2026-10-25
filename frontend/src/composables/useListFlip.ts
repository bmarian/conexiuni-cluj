import {watch, type Ref, type WatchSource} from 'vue'
import {reducedMotion, spring} from '@/utils/motion.ts'

// Leaving rows are absolute in the TransitionGroup, so each is pinned where it stood.
export function useListFlip(list: Ref<HTMLElement | undefined>, source: WatchSource) {
  const tops = new WeakMap<Element, number>()
  let height = 0

  watch(
    source,
    () => {
      const el = list.value
      if (!el) return
      height = el.offsetHeight
      for (const row of el.children) tops.set(row, (row as HTMLElement).offsetTop)
    },
    {flush: 'pre'},
  )

  watch(
    source,
    () => {
      const el = list.value
      if (!el || !height || reducedMotion()) return
      const to = el.offsetHeight
      if (to !== height) el.animate([{height: `${height}px`}, {height: `${to}px`}], spring('sheet'))
    },
    {flush: 'post'},
  )

  const pin = (el: Element) => {
    const row = el as HTMLElement
    row.style.top = `${tops.get(el) ?? row.offsetTop}px`
  }

  return {pin}
}

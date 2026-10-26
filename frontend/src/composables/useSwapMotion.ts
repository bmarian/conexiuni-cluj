import {nextTick, type Ref} from 'vue'
import {EMPHASIZED, reducedMotion, spring} from '@/utils/motion.ts'

const TRAVEL_MS = 520

function travel(el: HTMLElement, to: DOMRect, aside: number, under: boolean): Animation {
  const from = el.getBoundingClientRect()
  const dx = to.left - from.left
  const dy = to.top - from.top
  return el.animate(
    [
      {translate: '0 0'},
      {translate: `${dx / 2 + aside}px ${dy / 2}px`, scale: under ? 0.94 : 1.04, opacity: under ? 0.5 : 1, offset: 0.5},
      {translate: `${dx}px ${dy}px`},
    ],
    {duration: TRAVEL_MS, easing: EMPHASIZED, fill: 'forwards'},
  )
}

export function useSwapMotion(icon: Ref<HTMLElement | null | undefined>, from: Ref<HTMLElement | null | undefined>, to: Ref<HTMLElement | null | undefined>) {
  function spin() {
    if (!reducedMotion()) icon.value?.querySelector('svg')?.animate([{rotate: '0deg'}, {rotate: '180deg'}], spring('pop'))
  }

  async function swap(apply: () => Promise<unknown>) {
    const a = from.value
    const b = to.value
    if (reducedMotion() || !a || !b) return apply()
    const aside = a.getBoundingClientRect().height / 2
    const moves = [travel(a, b.getBoundingClientRect(), aside, false), travel(b, a.getBoundingClientRect(), -aside, true)]
    await Promise.all(moves.map((m) => m.finished.catch(() => {})))
    await apply()
    await nextTick()
    for (const m of moves) m.cancel()
  }

  return {spin, swap}
}

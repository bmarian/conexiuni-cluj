import {easeInOut, SWEEP_MS} from '@/utils/lineSweep.ts'
import {clamp} from '@/utils/geo.ts'
import {pop, reducedMotion, spring} from '@/utils/motion.ts'

export function drawTimeline(card: HTMLElement): () => void {
  if (reducedMotion()) return () => {}
  const top = card.getBoundingClientRect().top
  const rows = [...card.querySelectorAll<HTMLElement>('.timeline-row')].map((row) => {
    const box = row.getBoundingClientRect()
    const y = box.top - top
    const rail = row.querySelector<HTMLElement>('.rail')
    return {
      mid: y + box.height / 2,
      dot: rail?.querySelector<HTMLElement>('.dot') ?? null,
      content: [...row.children].filter((el): el is HTMLElement => el !== rail),
      halves: [...(rail?.querySelectorAll<HTMLElement>('.half') ?? [])].map((el) => ({el, from: el.classList.contains('is-top') ? y : y + box.height / 2, span: box.height / 2})),
      shown: false,
    }
  })
  if (!rows.length) return () => {}

  const slide = spring('slide')
  const start = rows[0]!.mid
  const end = rows.at(-1)!.mid
  let frame = 0

  const settle = () => {
    for (const row of rows) {
      for (const {el} of row.halves) el.style.clipPath = ''
      if (row.dot) row.dot.style.scale = ''
      for (const el of row.content) el.style.opacity = ''
    }
  }

  const paint = (reach: number) => {
    for (const row of rows) {
      for (const {el, from, span} of row.halves) el.style.clipPath = `inset(0 0 ${(1 - clamp((reach - from) / span, 0, 1)) * 100}% 0)`
      if (row.shown || reach < row.mid) continue
      row.shown = true
      if (row.dot) {
        row.dot.style.scale = ''
        pop(row.dot, 0)
      }
      for (const el of row.content) {
        el.style.opacity = ''
        el.animate([{opacity: 0, translate: '-6px 0'}, {opacity: 1, translate: '0 0'}], slide)
      }
    }
  }

  for (const row of rows) {
    if (row.dot) row.dot.style.scale = '0'
    for (const el of row.content) el.style.opacity = '0'
  }
  paint(start - 1)

  const began = performance.now()
  const step = (now: number) => {
    const t = Math.min(1, (now - began) / SWEEP_MS)
    paint(start + easeInOut(t) * (end - start))
    if (t < 1) frame = requestAnimationFrame(step)
    else settle()
  }
  frame = requestAnimationFrame(step)

  return () => {
    cancelAnimationFrame(frame)
    settle()
  }
}

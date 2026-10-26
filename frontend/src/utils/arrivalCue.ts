import {pop, reducedMotion, ripple} from '@/utils/motion.ts'

const RIPPLE_GAP_MS = 210

export function rippleFrom(body: HTMLElement, color: string) {
  if (reducedMotion()) return
  for (const delay of [0, RIPPLE_GAP_MS]) {
    const ring = document.createElement('span')
    ring.className = 'arrival-ring'
    ring.style.borderColor = color
    body.prepend(ring)
    const done = () => ring.remove()
    ripple(ring, {delay})?.finished.then(done, done)
  }
  pop(body, 1.2)
}

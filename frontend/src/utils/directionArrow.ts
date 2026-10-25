import {easeOutCubic} from '@/utils/markerMotion.ts'

const TAPS = 2
const TAP_MS = 240
const TAP_PX = 5
const ORBIT_MS = 1000
const ORBIT_PX = 6
const FRAME_MS = 16
export const ARROW_MS = TAPS * TAP_MS + ORBIT_MS
export const ARROW_BACK_MS = 320
// Waits out the page's slide-in.
export const ARROW_OPEN_DELAY_MS = 500

const rad = Math.PI / 180
const smooth = (k: number) => (1 - Math.cos(Math.PI * k)) / 2

/** [x, y, heading] at [ms]: two taps along compass [rest], then a spiral landing on [heading]. */
export function arrowPose(rest: number, heading: number, ms: number): [number, number, number] {
  if (ms < TAPS * TAP_MS) {
    const k = (ms % TAP_MS) / TAP_MS
    const d = TAP_PX * (k < 0.3 ? easeOutCubic(k / 0.3) : 1 - smooth((k - 0.3) / 0.7))
    return [d * Math.sin(rest * rad), -d * Math.cos(rest * rad), rest]
  }
  const turn = Math.sin(rest * rad) > 0 ? -1 : 1
  // Leaving radially along rest and arriving radially from the far side ends on heading.
  const base = (((turn * (heading + 180 - rest)) % 360) + 360) % 360
  const sweep = turn * (base < 270 ? base + 360 : base) * rad
  const v = smooth(Math.min(1, (ms - TAPS * TAP_MS) / ORBIT_MS))
  const phi = rest * rad + sweep * v
  const r = ORBIT_PX * Math.sin(Math.PI * v)
  const facing = phi + Math.atan2(Math.sin(Math.PI * v) * sweep, Math.PI * Math.cos(Math.PI * v))
  return [r * Math.sin(phi), -r * Math.cos(phi), facing / rad]
}

export function arrowKeyframes(rest: number, heading: number): Keyframe[] {
  return Array.from({length: Math.ceil(ARROW_MS / FRAME_MS) + 1}, (_, i) => {
    const ms = Math.min(ARROW_MS, i * FRAME_MS)
    const [x, y, a] = arrowPose(rest, heading, ms)
    return {offset: ms / ARROW_MS, transform: `translate(${x}px, ${y}px) rotate(${a - rest}deg)`}
  })
}

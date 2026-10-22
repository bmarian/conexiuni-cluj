export type SpiderLeg = {x: number, y: number, side: 'left' | 'right'}

const CIRCLE_MAX = 8
const FOOT_PX = 48
const MIN_RADIUS_PX = 44
const SPIRAL_START_PX = 28
const SPIRAL_FOOT_PX = 48
const SPIRAL_GROWTH_PX = 10

const leg = (r: number, angle: number): SpiderLeg => ({
  x: Math.round(r * Math.cos(angle)),
  y: Math.round(r * Math.sin(angle)),
  side: Math.cos(angle) < -1e-6 ? 'left' : 'right',
})

const circleRadius = (count: number) => Math.max(MIN_RADIUS_PX, count * FOOT_PX / (2 * Math.PI))

export function spiderLegs(count: number): SpiderLeg[] {
  if (count <= CIRCLE_MAX) {
    const r = circleRadius(count)
    return Array.from({length: count}, (_, i) => leg(r, 2 * Math.PI * i / count))
  }
  const legs: SpiderLeg[] = []
  let r = SPIRAL_START_PX
  let angle = 0
  for (let i = 0; i < count; i++) {
    angle += SPIRAL_FOOT_PX / r + i * 0.0005
    legs.push(leg(r, angle))
    r += 2 * Math.PI * SPIRAL_GROWTH_PX / angle
  }
  return legs
}

// angles: each station's real direction in screen radians; the circle turns to match them best.
export function spiderLegsToward(angles: number[]): SpiderLeg[] {
  const count = angles.length
  if (count > CIRCLE_MAX) return spiderLegs(count)
  const order = angles.map((_, i) => i).sort((a, b) => angles[a]! - angles[b]!)
  const step = 2 * Math.PI / count
  let x = 0
  let y = 0
  order.forEach((i, k) => {
    x += Math.cos(angles[i]! - k * step)
    y += Math.sin(angles[i]! - k * step)
  })
  const turn = Math.atan2(y, x)
  const r = circleRadius(count)
  const slot = new Map(order.map((i, k) => [i, k]))
  return angles.map((_, i) => leg(r, turn + slot.get(i)! * step))
}

export const spiderRadius = (legs: SpiderLeg[]): number => Math.ceil(Math.max(0, ...legs.map((l) => Math.hypot(l.x, l.y))))

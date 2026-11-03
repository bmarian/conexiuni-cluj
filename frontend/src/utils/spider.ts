import {boxAt, type ClusterBox, type Rect} from '@/utils/stopClusters.ts'

export type SpiderLeg = {x: number; y: number; side: 'left' | 'right'}

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

const circleRadius = (count: number) => Math.max(MIN_RADIUS_PX, (count * FOOT_PX) / (2 * Math.PI))

export function spiderLegs(count: number): SpiderLeg[] {
  if (count <= CIRCLE_MAX) {
    const r = circleRadius(count)
    return Array.from({length: count}, (_, i) => leg(r, (2 * Math.PI * i) / count))
  }
  const legs: SpiderLeg[] = []
  let r = SPIRAL_START_PX
  let angle = 0
  for (let i = 0; i < count; i++) {
    angle += SPIRAL_FOOT_PX / r + i * 0.0005
    legs.push(leg(r, angle))
    r += (2 * Math.PI * SPIRAL_GROWTH_PX) / angle
  }
  return legs
}

export function spiderLegsToward(screenAngles: number[]): SpiderLeg[] {
  const count = screenAngles.length
  if (count > CIRCLE_MAX) return spiderLegs(count)
  const order = screenAngles.map((_, i) => i).sort((a, b) => screenAngles[a]! - screenAngles[b]!)
  const step = (2 * Math.PI) / count
  let x = 0
  let y = 0
  order.forEach((i, k) => {
    x += Math.cos(screenAngles[i]! - k * step)
    y += Math.sin(screenAngles[i]! - k * step)
  })
  const turn = Math.atan2(y, x)
  const r = circleRadius(count)
  const slot = new Map(order.map((i, k) => [i, k]))
  return screenAngles.map((_, i) => leg(r, turn + slot.get(i)! * step))
}

export type SpiderItem = {station: {x: number; y: number}; icon: ClusterBox; labelWidth: number}

export type LabelSide = SpiderLeg['side'] | 'below'

// Same as .stop-spider-item's gap and .stop-cluster-label's line-height and margin in map.css.
const LABEL_GAP_PX = 4
const LABEL_HEIGHT_PX = 13
const LABEL_UNDER_PX = 2
const RINGS = 3

export function labelBox({x, y}: {x: number; y: number}, icon: ClusterBox, labelWidth: number, side: LabelSide): Rect {
  const box = boxAt(x, y, icon)
  if (side === 'below') {
    const left = (box.left + box.right - labelWidth) / 2
    return {left, top: box.bottom + LABEL_UNDER_PX, right: left + labelWidth, bottom: box.bottom + LABEL_UNDER_PX + LABEL_HEIGHT_PX}
  }
  const middle = (box.top + box.bottom) / 2
  const start = side === 'right' ? box.right + LABEL_GAP_PX : box.left - LABEL_GAP_PX - labelWidth
  return {left: start, top: middle - LABEL_HEIGHT_PX / 2, right: start + labelWidth, bottom: middle + LABEL_HEIGHT_PX / 2}
}

const itemBoxes = ({icon, labelWidth}: SpiderItem, {x, y, side}: SpiderLeg): Rect[] => [boxAt(x, y, icon), labelBox({x, y}, icon, labelWidth, side)]

const overlap = (a: Rect, b: Rect) =>
  Math.max(0, Math.min(a.right, b.right) - Math.max(a.left, b.left)) * Math.max(0, Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top))

const coveredBy = (boxes: Rect[], taken: Rect[]) => boxes.reduce((sum, box) => taken.reduce((s, t) => s + overlap(box, t), sum), 0)

// Away from the other stations first, then the other side, then under the icon; else the least covered.
export function labelSidesClear(items: SpiderItem[], obstacles: Rect[], area: Rect): LabelSide[] {
  const taken = [...obstacles]
  const middle = items.reduce((sum, item) => sum + item.station.x, 0) / items.length
  return items.map(({station, icon, labelWidth}) => {
    const out: LabelSide = station.x < middle ? 'left' : 'right'
    let best: {side: LabelSide; box: Rect; covered: number} | null = null
    for (const side of [out, out === 'left' ? 'right' : 'left', 'below'] as const) {
      const box = labelBox(station, icon, labelWidth, side)
      const covered = coveredBy([box], taken) + (box.right - box.left) * (box.bottom - box.top) - overlap(box, area)
      if (!best || covered < best.covered) best = {side, box, covered}
      if (!covered) break
    }
    taken.push(best!.box)
    return best!.side
  })
}

// The free spot nearest each station, else the least covered one.
export function spiderLegsClear(items: SpiderItem[], obstacles: Rect[]): SpiderLeg[] {
  const taken = [...obstacles]
  return items.map((item) => {
    const toward = Math.atan2(item.station.y, item.station.x)
    let best: {leg: SpiderLeg; covered: number; far: number} | null = null
    for (let ring = 0; ring < RINGS; ring++) {
      const r = MIN_RADIUS_PX + ring * FOOT_PX
      const step = item.icon.size[0] / r
      for (let k = 0; k * step <= Math.PI; k++) {
        for (const turn of k ? [k * step, -k * step] : [0]) {
          const l = leg(r, toward + turn)
          const covered = coveredBy(itemBoxes(item, l), taken)
          const far = Math.hypot(l.x - item.station.x, l.y - item.station.y)
          if (!best || covered < best.covered || (covered === best.covered && far < best.far)) best = {leg: l, covered, far}
        }
      }
    }
    taken.push(...itemBoxes(item, best!.leg))
    return best!.leg
  })
}

export const spiderRadius = (legs: SpiderLeg[]): number => Math.ceil(Math.max(0, ...legs.map((l) => Math.hypot(l.x, l.y))))

import {haversineMeters} from '@/utils/geo.ts'

/** Where a point falls on a line: metres along it and metres off it. */
export interface Spot {
  along: number
  off: number
}

interface Point {
  lat: number
  lon: number
}

/** A line measured along its length, to follow someone along it. Points are [lat, lon]. */
export class RouteLine {
  private readonly at: number[] = [0]

  constructor(readonly points: [number, number][]) {
    for (let i = 1; i < points.length; i++) {
      const [aLat, aLon] = points[i - 1]!
      const [bLat, bLon] = points[i]!
      this.at.push(this.at[i - 1]! + haversineMeters(aLat, aLon, bLat, bLon))
    }
  }

  get length(): number {
    return this.at[this.at.length - 1] ?? 0
  }

  /** The nearest spot between `from` and `to` metres along. */
  locate(lat: number, lon: number, from = 0, to = Infinity): Spot | null {
    if (this.points.length < 2) return null
    const kx = 111_320 * Math.cos((lat * Math.PI) / 180)
    const ky = 110_540
    let best: Spot | null = null
    for (let i = 1; i < this.points.length; i++) {
      const start = this.at[i - 1]!
      const end = this.at[i]!
      if (end < from) continue
      if (start > to) break
      const [aLat, aLon] = this.points[i - 1]!
      const [bLat, bLon] = this.points[i]!
      const ax = (aLon - lon) * kx
      const ay = (aLat - lat) * ky
      const dx = (bLon - aLon) * kx
      const dy = (bLat - aLat) * ky
      const len = dx * dx + dy * dy
      const span = end - start
      const tMin = from > start && span > 0 ? (from - start) / span : 0
      const tMax = to < end && span > 0 ? Math.max((to - start) / span, tMin) : 1
      const t = len === 0 ? tMin : Math.min(Math.max(-(ax * dx + ay * dy) / len, tMin), tMax)
      const off = Math.hypot(ax + t * dx, ay + t * dy)
      if (best && off >= best.off) continue
      best = {along: start + t * span, off}
    }
    return best
  }

  /** The line from `from` to `to` metres along. */
  between(from: number, to: number): [number, number][] {
    if (this.points.length < 2 || to <= from) return []
    const inside = this.points.filter((_, i) => this.at[i]! > from && this.at[i]! < to)
    return [this.pointAt(from), ...inside, this.pointAt(to)]
  }

  private pointAt(along: number): [number, number] {
    let i = this.at.findIndex((a) => a >= along)
    if (i < 1) i = along <= 0 ? 1 : this.points.length - 1
    const span = this.at[i]! - this.at[i - 1]!
    const t = span > 0 ? Math.min(Math.max((along - this.at[i - 1]!) / span, 0), 1) : 0
    const [aLat, aLon] = this.points[i - 1]!
    const [bLat, bLon] = this.points[i]!
    return [aLat + t * (bLat - aLat), aLon + t * (bLon - aLon)]
  }
}

/** Each point's metres along the line, taken in order so a line passing near itself keeps them apart. */
export function alongEach(route: RouteLine, points: Point[]): number[] {
  let from = 0
  return points.map((p) => (from = route.locate(p.lat, p.lon, from)?.along ?? from))
}

/** The part of a line's shape between two of its stops, given all its stops in order. */
export function ridePath(shape: [number, number][], stops: Point[], boarding: number, alighting: number): [number, number][] {
  if (shape.length < 2 || boarding < 0 || alighting >= stops.length || boarding >= alighting) return []
  const route = new RouteLine(shape)
  const along = alongEach(route, stops.slice(0, alighting + 1))
  return route.between(along[boarding]!, along[alighting]!)
}

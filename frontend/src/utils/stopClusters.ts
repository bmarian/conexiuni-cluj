import type {MarkerIcon} from '@/utils/mapIcons.ts'
import {type Mode, MODES, modeOf} from '@/utils/modes.ts'
import type {RouteType} from '@/types/tranzy.ts'

export type StopPoint = {id: string, lat: number, lng: number, station: number, modes: number, rank: number, favorite: boolean}

export type StopGroup = {
  key: string
  leader: number
  points: number[]
  stations: Set<number>
  modes: number
  favorite: boolean
}

export type ClusterBox = Pick<MarkerIcon, 'size' | 'anchor'>

type Rect = {left: number, top: number, right: number, bottom: number}

// MapLibre's world is 512 px wide at zoom 0.
const TILE_PX = 512

const mercX = (lng: number) => (lng + 180) / 360
const mercY = (lat: number) => {
  const sin = Math.sin(lat * Math.PI / 180)
  return 0.5 - Math.log((1 + sin) / (1 - sin)) / (4 * Math.PI)
}

const overlaps = (a: Rect, b: Rect) => a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom

export const modeMask = (routeTypes: Iterable<RouteType | number>): number => {
  let mask = 0
  for (const type of routeTypes) mask |= 1 << MODES.indexOf(modeOf(type))
  return mask
}

export const modesIn = (mask: number): Mode[] => MODES.filter((_, i) => mask & (1 << i))

// Bigger modes first, then the station with more lines.
export const stationRank = (routeTypes: Array<RouteType | number>): number => {
  const best = routeTypes.reduce<number>((min, type) => Math.min(min, MODES.indexOf(modeOf(type))), MODES.length - 1)
  return (MODES.length - best) * 10_000 + routeTypes.length
}

// Stops share a marker wherever their drawn markers would overlap; a group sits on its highest ranked stop.
export class StopClusters {
  private readonly levels: StopGroup[][] = []
  private readonly owners: Int32Array[] = []
  private readonly xs: Float64Array
  private readonly ys: Float64Array
  private seq = 0
  readonly indexOf = new Map<string, number>()

  constructor(
    readonly points: StopPoint[],
    private readonly boxOf: (group: StopGroup) => ClusterBox,
    private readonly maxZoom: number,
    private readonly step: number,
  ) {
    this.xs = Float64Array.from(points, (p) => mercX(p.lng))
    this.ys = Float64Array.from(points, (p) => mercY(p.lat))
    points.forEach((p, i) => this.indexOf.set(p.id, i))
  }

  at(zoom: number): StopGroup[] {
    const level = this.levelOf(zoom)
    while (this.levels.length <= level) this.build()
    return this.levels[level]!
  }

  lngLat(group: StopGroup) {
    const {lat, lng} = this.points[group.leader]!
    return {lat, lng}
  }

  bounds(group: StopGroup): [[number, number], [number, number]] {
    const lats = group.points.map((i) => this.points[i]!.lat)
    const lngs = group.points.map((i) => this.points[i]!.lng)
    return [[Math.min(...lngs), Math.min(...lats)], [Math.max(...lngs), Math.max(...lats)]]
  }

  // The zooms at which the group first breaks up, and at which each of its stations stands alone.
  expansion(group: StopGroup, zoom: number): {split: number, apart: number} {
    let split: number | null = null
    for (let level = this.levelOf(zoom) - 1; level >= 0; level--) {
      const owner = this.ownerAt(level)
      const groups = this.levels[level]!
      const first = owner[group.points[0]!]
      if (split === null && group.points.some((i) => owner[i] !== first)) split = this.zoomOf(level)
      if (group.points.every((i) => groups[owner[i]!]!.stations.size === 1)) return {split: split ?? this.zoomOf(level), apart: this.zoomOf(level)}
    }
    return {split: split ?? this.maxZoom, apart: this.maxZoom}
  }

  private levelOf(zoom: number): number {
    return Math.max(0, Math.ceil((this.maxZoom - zoom) / this.step - 1e-9))
  }

  private zoomOf(level: number): number {
    return this.maxZoom - level * this.step
  }

  private ownerAt(level: number): Int32Array {
    let owner = this.owners[level]
    if (!owner) {
      const out = new Int32Array(this.points.length)
      this.levels[level]!.forEach((group, g) => group.points.forEach((i) => (out[i] = g)))
      owner = this.owners[level] = out
    }
    return owner
  }

  private rect(group: StopGroup, px: number): Rect {
    const {size, anchor} = this.boxOf(group)
    const left = this.xs[group.leader]! * px - anchor[0]
    const top = this.ys[group.leader]! * px - anchor[1]
    return {left, top, right: left + size[0], bottom: top + size[1]}
  }

  private build() {
    const level = this.levels.length
    const px = TILE_PX * 2 ** this.zoomOf(level)
    let groups = level ? this.levels[level - 1]! : this.points.map((p, i): StopGroup => ({
      key: `p${i}`,
      leader: i,
      points: [i],
      stations: new Set([p.station]),
      modes: p.modes,
      favorite: p.favorite,
    }))
    for (;;) {
      const merged = this.pass(groups, px)
      if (merged.length === groups.length) break
      groups = merged
    }
    this.levels.push(groups)
  }

  private pass(groups: StopGroup[], px: number): StopGroup[] {
    const rects = groups.map((g) => this.rect(g, px))
    const cell = Math.max(...rects.map((r) => Math.max(r.right - r.left, r.bottom - r.top)))
    const cellKey = (cx: number, cy: number) => cx * 0x4000000 + cy
    const grid = new Map<number, number[]>()
    groups.forEach((g, i) => {
      const key = cellKey(Math.floor(this.xs[g.leader]! * px / cell), Math.floor(this.ys[g.leader]! * px / cell))
      const list = grid.get(key)
      if (list) list.push(i)
      else grid.set(key, [i])
    })
    const rank = (g: StopGroup) => this.points[g.leader]!.rank
    const order = groups.map((_, i) => i).sort((a, b) =>
      rank(groups[b]!) - rank(groups[a]!) || groups[b]!.points.length - groups[a]!.points.length || a - b)
    const taken = new Uint8Array(groups.length)
    const out: StopGroup[] = []
    for (const i of order) {
      if (taken[i]) continue
      taken[i] = 1
      let group = groups[i]!
      let fresh = false
      for (let grew = true; grew;) {
        grew = false
        const r = this.rect(group, px)
        for (let cx = Math.floor((r.left - cell) / cell); cx <= Math.floor((r.right + cell) / cell); cx++) {
          for (let cy = Math.floor((r.top - cell) / cell); cy <= Math.floor((r.bottom + cell) / cell); cy++) {
            for (const j of grid.get(cellKey(cx, cy)) ?? []) {
              if (taken[j] || !overlaps(r, rects[j]!)) continue
              taken[j] = 1
              if (!fresh) {
                group = {...group, key: `g${++this.seq}`, points: [...group.points], stations: new Set(group.stations)}
                fresh = true
              }
              const other = groups[j]!
              group.points.push(...other.points)
              for (const s of other.stations) group.stations.add(s)
              group.modes |= other.modes
              group.favorite ||= other.favorite
              grew = true
            }
          }
        }
      }
      out.push(group)
    }
    return out
  }
}

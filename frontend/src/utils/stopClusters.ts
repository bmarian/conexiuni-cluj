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
  // The towns or districts the group stands for, the one it is named after first.
  areas?: number[]
}

export type ClusterBox = Pick<MarkerIcon, 'size' | 'anchor'>

export type Rect = {left: number, top: number, right: number, bottom: number}

// MapLibre's world is 512 px wide at zoom 0.
export const TILE_PX = 512

export const lngPerPixel = (zoom: number) => 360 / (TILE_PX * 2 ** zoom)

export const mercX = (lng: number) => (lng + 180) / 360
export const mercY = (lat: number) => {
  const sin = Math.sin(lat * Math.PI / 180)
  return 0.5 - Math.log((1 + sin) / (1 - sin)) / (4 * Math.PI)
}

const cellKey = (cx: number, cy: number) => cx * 0x4000000 + cy

export const overlaps = (a: Rect, b: Rect) => a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom

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

const collator = new Intl.Collator(undefined, {sensitivity: 'base'})

const sameWord = (a: string, b: string) => collator.compare(a, b) === 0

const plainWords = (name: string) => name.replace(/\s*\([^)]*\)/g, '').split(' ').filter(Boolean)

// The start of the leader's name that the most other stations share, when it is longer than what any of them adds to it;
// bracketed qualifiers like "(M1/3)" don't count.
export function groupName(leader: string, others: string[]): {name: string, covered: number} {
  const words = plainWords(leader)
  const split = others.map(plainWords)
  let best = {name: leader, covered: 0}
  for (let k = 1; k <= words.length; k++) {
    const root = words.slice(0, k).join(' ')
    if (root.length <= words.slice(k).join(' ').length) continue
    const covered = split.filter((w) => w.length >= k
      && words.slice(0, k).every((word, i) => sameWord(word, w[i]!))
      && root.length > w.slice(k).join(' ').length).length
    if (covered && covered >= best.covered) best = {name: root, covered}
  }
  return best
}

// Markers drop out, lowest ranked first, where they would overlap; each stop then goes to its nearest remaining marker.
export class StopClusters {
  private readonly levels: StopGroup[][] = []
  private readonly owners: Int32Array[] = []
  private readonly boxes = new Map<string, ClusterBox>()
  private pending: StopGroup[] | null = null
  private readonly xs: Float64Array
  private readonly ys: Float64Array
  private readonly span: number
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
    const extent = (values: Float64Array) => values.length ? Math.max(...values) - Math.min(...values) : 0
    this.span = Math.max(extent(this.xs), extent(this.ys)) ** 2
  }

  at(zoom: number): StopGroup[] {
    const level = this.levelOf(zoom)
    while (this.levels.length <= level) this.build()
    return this.levels[level]!
  }

  ready(zoom: number): boolean {
    return this.levels.length > this.levelOf(zoom)
  }

  get depth(): number {
    return this.levels.length
  }

  // One sweep of the next level and the regrouping it calls for.
  advance() {
    const level = this.levels.length
    const groups = this.pending ?? (level ? this.levels[level - 1]! : this.points.map((_, i) => this.group(i, [i])))
    const kept = this.sweep(groups, TILE_PX * 2 ** this.zoomOf(level))
    this.pending = kept.length === groups.length ? null : this.assign(groups, kept)
    if (!this.pending) this.levels.push(groups)
  }

  // The groups whose drawn boxes reach into the view.
  within(zoom: number, west: number, south: number, east: number, north: number): StopGroup[] {
    const px = TILE_PX * 2 ** zoom
    const view: Rect = {left: mercX(west) * px, top: mercY(north) * px, right: mercX(east) * px, bottom: mercY(south) * px}
    return this.at(zoom).filter((group) => overlaps(this.rect(group, px), view))
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

  // The zooms at which the group first breaks up, and at which each of its stations stands alone; null when no zoom breaks it up.
  expansion(group: StopGroup, zoom: number): {split: number, apart: number} | null {
    let split: number | null = null
    for (let level = this.levelOf(zoom) - 1; level >= 0; level--) {
      const owner = this.ownerAt(level)
      const groups = this.levels[level]!
      const first = owner[group.points[0]!]
      if (split === null && group.points.some((i) => owner[i] !== first)) split = this.zoomOf(level)
      if (group.points.every((i) => groups[owner[i]!]!.stations.size === 1)) return {split: split ?? this.zoomOf(level), apart: this.zoomOf(level)}
    }
    return split === null ? null : {split, apart: this.maxZoom}
  }

  levelOf(zoom: number): number {
    return Math.max(0, Math.ceil((this.maxZoom - zoom) / this.step - 1e-9))
  }

  zoomOf(level: number): number {
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
    let box = this.boxes.get(group.key)
    if (!box) this.boxes.set(group.key, box = this.boxOf(group))
    const {size, anchor} = box
    const left = this.xs[group.leader]! * px - anchor[0]
    const top = this.ys[group.leader]! * px - anchor[1]
    return {left, top, right: left + size[0], bottom: top + size[1]}
  }

  private build() {
    const level = this.levels.length
    while (this.levels.length === level) this.advance()
  }

  // The leaders whose markers fit, highest ranked first.
  private sweep(groups: StopGroup[], px: number): number[] {
    const rank = (g: StopGroup) => this.points[g.leader]!.rank
    const order = [...groups].sort((a, b) => rank(b) - rank(a) || b.points.length - a.points.length || a.leader - b.leader)
    const rects = order.map((g) => this.rect(g, px))
    const cell = Math.max(...rects.map((r) => Math.max(r.right - r.left, r.bottom - r.top)))
    const grid = new Map<number, Rect[]>()
    const kept: number[] = []
    order.forEach((group, i) => {
      const r = rects[i]!
      const x0 = Math.floor(r.left / cell)
      const y0 = Math.floor(r.top / cell)
      for (let cx = x0 - 1; cx <= Math.floor(r.right / cell); cx++) {
        for (let cy = y0 - 1; cy <= Math.floor(r.bottom / cell); cy++) {
          if (grid.get(cellKey(cx, cy))?.some((other) => overlaps(r, other))) return
        }
      }
      kept.push(group.leader)
      const key = cellKey(x0, y0)
      const list = grid.get(key)
      if (list) list.push(r)
      else grid.set(key, [r])
    })
    return kept
  }

  // Leaders only ever drop out, so only the stops of dropped groups need their nearest remaining leader.
  private assign(groups: StopGroup[], kept: number[]): StopGroup[] {
    const cell = Math.sqrt(this.span / kept.length) || 1
    const grid = new Map<number, number[]>()
    for (const l of kept) {
      const key = cellKey(Math.floor(this.xs[l]! / cell), Math.floor(this.ys[l]! / cell))
      const list = grid.get(key)
      if (list) list.push(l)
      else grid.set(key, [l])
    }
    const alive = new Set(kept)
    const gained = new Map<number, number[]>()
    for (const group of groups) {
      if (alive.has(group.leader)) continue
      for (const i of group.points) {
        const l = this.nearest(i, grid, cell)
        const list = gained.get(l)
        if (list) list.push(i)
        else gained.set(l, [i])
      }
    }
    const byLeader = new Map(groups.map((g) => [g.leader, g]))
    return kept.map((l) => {
      const group = byLeader.get(l)!
      const extra = gained.get(l)
      return extra ? this.group(l, [...group.points, ...extra].sort((a, b) => a - b)) : group
    })
  }

  private nearest(i: number, grid: Map<number, number[]>, cell: number): number {
    const x = this.xs[i]!
    const y = this.ys[i]!
    const cx = Math.floor(x / cell)
    const cy = Math.floor(y / cell)
    let best = -1
    let bestD = Infinity
    for (let ring = 0; best < 0 || (ring - 1) * cell < Math.sqrt(bestD); ring++) {
      for (let dx = -ring; dx <= ring; dx++) {
        for (let dy = -ring; dy <= ring; dy++) {
          if (Math.max(Math.abs(dx), Math.abs(dy)) !== ring) continue
          for (const l of grid.get(cellKey(cx + dx, cy + dy)) ?? []) {
            const d = (this.xs[l]! - x) ** 2 + (this.ys[l]! - y) ** 2
            if (d < bestD) {
              bestD = d
              best = l
            }
          }
        }
      }
    }
    return best
  }

  private group(leader: number, points: number[]): StopGroup {
    return groupOf(this.points, leader, points, `${leader}`)
  }
}

export function groupOf(all: StopPoint[], leader: number, points: number[], key: string): StopGroup {
  const group: StopGroup = {key: '', leader, points, stations: new Set(), modes: 0, favorite: false}
  for (const i of points) {
    const p = all[i]!
    group.stations.add(p.station)
    group.modes |= p.modes
    group.favorite ||= p.favorite
  }
  group.key = `${key}:${points.length}:${points.reduce((h, i) => (h * 31 + i) | 0, 0)}`
  return group
}

import {
  type ClusterBox,
  groupOf,
  mercX,
  mercY,
  overlaps,
  type Rect,
  type StopClusters,
  type StopGroup,
  type StopPoint,
  TILE_PX,
} from '@/utils/stopClusters.ts'

export type Area = {name: string, lat: number, lon: number, place: string, town: string}

export type AreaLabel = {name: string, more: number, names: string[]}

// OSM's places, biggest first; the first three are towns.
const PLACES = ['city', 'town', 'village', 'suburb', 'quarter', 'neighbourhood']
const isTown = (area: Area) => PLACES.indexOf(area.place) < PLACES.indexOf('suburb')

const nearestOf = (x: number, y: number, among: number[], xs: ArrayLike<number>, ys: ArrayLike<number>) => {
  let best = -1
  let bestD = Infinity
  for (const k of among) {
    const d = (xs[k]! - x) ** 2 + (ys[k]! - y) ** 2
    if (d < bestD) {
      bestD = d
      best = k
    }
  }
  return best
}

// Like StopClusters, led by towns and districts; a dropped one's stops go to the nearest left in their town, else the nearest left.
export class AreaClusters {
  readonly areas: Area[]
  private readonly levels: StopGroup[][] = []
  private readonly boxes = new Map<string, ClusterBox>()
  private readonly ax: number[]
  private readonly ay: number[]
  private readonly sx: Float64Array
  private readonly sy: Float64Array
  private readonly home: Int32Array
  private readonly rank: number[]

  constructor(
    readonly points: StopPoint[],
    areas: Area[],
    private readonly boxOf: (group: StopGroup) => ClusterBox,
    private readonly maxZoom: number,
    private readonly step: number,
  ) {
    this.sx = Float64Array.from(points, (p) => mercX(p.lng))
    this.sy = Float64Array.from(points, (p) => mercY(p.lat))
    const known = areas.filter((a) => PLACES.includes(a.place))
    const kx = known.map((a) => mercX(a.lon))
    const ky = known.map((a) => mercY(a.lat))
    const all = known.map((_, k) => k)
    const nearest = points.map((_, i) => nearestOf(this.sx[i]!, this.sy[i]!, all, kx, ky))
    const owned = known.map(() => 0)
    for (const k of nearest) if (k >= 0) owned[k]!++
    const kept = all.filter((k) => owned[k]! > 0)
    const renumber = new Map(kept.map((k, i) => [k, i]))
    this.areas = kept.map((k) => known[k]!)
    this.ax = kept.map((k) => kx[k]!)
    this.ay = kept.map((k) => ky[k]!)
    this.home = Int32Array.from(nearest, (k) => renumber.get(k) ?? -1)
    this.rank = kept.map((k) => (PLACES.length - PLACES.indexOf(known[k]!.place)) * points.length + owned[k]!)
  }

  at(zoom: number): StopGroup[] {
    const level = this.levelOf(zoom)
    while (this.levels.length <= level) this.advance()
    return this.levels[level]!
  }

  ready(zoom: number): boolean {
    return this.levels.length > this.levelOf(zoom)
  }

  advance() {
    const level = this.levels.length
    let groups = level ? this.levels[level - 1]! : this.initial()
    const px = TILE_PX * 2 ** (this.maxZoom - level * this.step)
    for (let kept = this.sweep(groups, px); kept.length < groups.length; kept = this.sweep(groups, px)) {
      groups = this.merge(groups, kept)
    }
    this.levels.push(groups)
  }

  within(zoom: number, west: number, south: number, east: number, north: number): StopGroup[] {
    const px = TILE_PX * 2 ** zoom
    const view: Rect = {left: mercX(west) * px, top: mercY(north) * px, right: mercX(east) * px, bottom: mercY(south) * px}
    return this.at(zoom).filter((group) => overlaps(this.rect(group, px), view))
  }

  lngLat(group: StopGroup) {
    const {lat, lon} = this.areas[group.areas![0]!]!
    return {lat, lng: lon}
  }

  // A town is named for the towns in it; a district for the districts.
  label(group: StopGroup): AreaLabel {
    const [lead, ...rest] = group.areas!.map((k) => this.areas[k]!)
    if (isTown(lead!)) {
      const towns = [...new Set([lead!.town, ...rest.map((a) => a.town)])]
      return {name: lead!.name, more: towns.length - 1, names: towns}
    }
    return {name: lead!.name, more: rest.length, names: [lead!.name, ...rest.map((a) => a.name)]}
  }

  private levelOf(zoom: number): number {
    return Math.max(0, Math.ceil((this.maxZoom - zoom) / this.step - 1e-9))
  }

  private initial(): StopGroup[] {
    const members = this.areas.map((): number[] => [])
    this.home.forEach((k, i) => k >= 0 && members[k]!.push(i))
    return this.named(members.map((points, k) => this.group(k, points)))
  }

  // A group stands for the areas whose stops are mostly in it.
  private named(groups: StopGroup[]): StopGroup[] {
    const counts = groups.map((group) => {
      const count = new Map<number, number>()
      for (const i of group.points) count.set(this.home[i]!, (count.get(this.home[i]!) ?? 0) + 1)
      return count
    })
    const most = new Map<number, {g: number, n: number}>()
    counts.forEach((count, g) => count.forEach((n, k) => {
      if ((most.get(k)?.n ?? 0) < n) most.set(k, {g, n})
    }))
    groups.forEach((group, g) => {
      const lead = group.areas![0]!
      const others = [...counts[g]!].filter(([k]) => k !== lead && most.get(k)!.g === g).sort((a, b) => b[1] - a[1])
      group.areas = [lead, ...others.map(([k]) => k)]
      group.key = `${group.key.split('|')[0]}|${group.areas.join('.')}`
    })
    return groups
  }

  box(group: StopGroup): ClusterBox {
    let box = this.boxes.get(group.key)
    if (!box) this.boxes.set(group.key, box = this.boxOf(group))
    return box
  }

  private rect(group: StopGroup, px: number): Rect {
    const box = this.box(group)
    const k = group.areas![0]!
    const left = this.ax[k]! * px - box.anchor[0]
    const top = this.ay[k]! * px - box.anchor[1]
    return {left, top, right: left + box.size[0], bottom: top + box.size[1]}
  }

  private sweep(groups: StopGroup[], px: number): StopGroup[] {
    const order = [...groups].sort((a, b) => this.rank[b.areas![0]!]! - this.rank[a.areas![0]!]!)
    const kept: StopGroup[] = []
    const rects: Rect[] = []
    for (const group of order) {
      const r = this.rect(group, px)
      if (rects.some((other) => overlaps(r, other))) continue
      kept.push(group)
      rects.push(r)
    }
    return kept
  }

  private merge(groups: StopGroup[], kept: StopGroup[]): StopGroup[] {
    const alive = kept.map((g) => g.areas![0]!)
    const byTown = new Map<string, number[]>()
    for (const k of alive) {
      const town = this.areas[k]!.town
      byTown.set(town, [...byTown.get(town) ?? [], k])
    }
    const members = new Map(kept.map((g) => [g.areas![0]!, [...g.points]]))
    const keptKeys = new Set(kept.map((g) => g.key))
    for (const group of groups) {
      if (keptKeys.has(group.key)) continue
      for (const i of group.points) {
        const town = this.areas[this.home[i]!]!.town
        const k = nearestOf(this.sx[i]!, this.sy[i]!, byTown.get(town) ?? alive, this.ax, this.ay)
        members.get(k)!.push(i)
      }
    }
    return this.named([...members].map(([k, points]) => this.group(k, points.sort((a, b) => a - b))))
  }

  private group(k: number, points: number[]): StopGroup {
    const leader = points.reduce((best, i) => this.points[i]!.rank > this.points[best]!.rank ? i : best, points[0]!)
    const group = groupOf(this.points, leader, points, `a${k}`)
    group.areas = [k]
    return group
  }
}

// Station groups up close; areas once most station groups hold more stations than a group shows one by one.
export class MapClusters {
  private checked = 0
  private crossing: number | null = null

  constructor(readonly stations: StopClusters, private readonly areas: AreaClusters | null, private readonly detail: number) {}

  get points() {
    return this.stations.points
  }

  at(zoom: number): StopGroup[] {
    if (this.far(zoom)) return this.areas!.at(zoom)
    const groups = this.stations.at(zoom)
    return this.far(zoom) ? this.areas!.at(zoom) : groups
  }

  ready(zoom: number): boolean {
    return this.far(zoom) ? this.areas!.ready(zoom) : this.stations.ready(zoom)
  }

  advance() {
    this.settle()
    if (this.crossing === null) this.stations.advance()
    else this.areas!.advance()
  }

  within(zoom: number, west: number, south: number, east: number, north: number): StopGroup[] {
    return this.far(zoom) ? this.areas!.within(zoom, west, south, east, north) : this.stations.within(zoom, west, south, east, north)
  }

  lngLat(group: StopGroup) {
    return group.areas ? this.areas!.lngLat(group) : this.stations.lngLat(group)
  }

  bounds(group: StopGroup) {
    return this.stations.bounds(group)
  }

  box(group: StopGroup) {
    return group.areas ? this.areas!.box(group) : this.stations.box(group)
  }

  // The zooms the level shown at zoom covers.
  span(zoom: number): [number, number] {
    const level = this.stations.levelOf(zoom)
    return [this.stations.zoomOf(level), this.stations.zoomOf(Math.max(0, level - 1))]
  }

  expansion(group: StopGroup, zoom: number) {
    return group.areas ? null : this.stations.expansion(group, zoom)
  }

  label(group: StopGroup): AreaLabel | null {
    return group.areas ? this.areas!.label(group) : null
  }

  // The lowest zoom that still shows stations.
  stationsFrom(): number | null {
    return this.crossing === null ? null : this.stations.zoomOf(this.crossing - 1)
  }

  private far(zoom: number): boolean {
    this.settle()
    return this.crossing !== null && this.stations.levelOf(zoom) >= this.crossing
  }

  private settle() {
    if (!this.areas) return
    for (; this.crossing === null && this.checked < this.stations.depth; this.checked++) {
      const stations = this.stations.at(this.stations.zoomOf(this.checked))
      const big = stations.filter((g) => g.stations.size > this.detail).length
      if (2 * big > stations.length) this.crossing = this.checked
    }
  }
}

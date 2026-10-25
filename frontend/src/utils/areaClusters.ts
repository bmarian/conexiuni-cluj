import {boxAt, type ClusterBox, groupOf, mercX, mercY, overlaps, type Rect, type StopClusters, type StopGroup, TILE_PX, viewRect} from '@/utils/stopClusters.ts'

export type Area = {name: string; lat: number; lon: number; place: string}

const PLACES = ['city', 'town', 'village', 'suburb', 'quarter', 'neighbourhood']

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
    private readonly stations: StopClusters,
    areas: Area[],
    private readonly boxOf: (group: StopGroup) => ClusterBox,
  ) {
    const points = stations.points
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
    const level = this.stations.levelOf(zoom)
    while (this.levels.length <= level) this.advance()
    return this.levels[level]!
  }

  ready(zoom: number): boolean {
    return this.levels.length > this.stations.levelOf(zoom)
  }

  advance() {
    const level = this.levels.length
    let groups = level ? this.levels[level - 1]! : this.initial()
    const px = TILE_PX * 2 ** this.stations.zoomOf(level)
    for (let kept = this.sweep(groups, px); kept.length < groups.length; kept = this.sweep(groups, px)) {
      groups = this.merge(groups, kept)
    }
    this.levels.push(groups)
  }

  within(zoom: number, west: number, south: number, east: number, north: number): StopGroup[] {
    const px = TILE_PX * 2 ** zoom
    const view = viewRect(px, west, south, east, north)
    return this.at(zoom).filter((group) => overlaps(this.rect(group, px), view))
  }

  lngLat(group: StopGroup) {
    const {lat, lon} = this.areas[group.area!]!
    return {lat, lng: lon}
  }

  box(group: StopGroup): ClusterBox {
    let box = this.boxes.get(group.key)
    if (!box) this.boxes.set(group.key, (box = this.boxOf(group)))
    return box
  }

  name(group: StopGroup): string {
    return this.areas[group.area!]!.name
  }

  private initial(): StopGroup[] {
    const members = this.areas.map((): number[] => [])
    this.home.forEach((k, i) => k >= 0 && members[k]!.push(i))
    return members.map((points, k) => this.group(k, points))
  }

  private rect(group: StopGroup, px: number): Rect {
    return boxAt(this.ax[group.area!]! * px, this.ay[group.area!]! * px, this.box(group))
  }

  private sweep(groups: StopGroup[], px: number): StopGroup[] {
    const order = [...groups].sort((a, b) => this.rank[b.area!]! - this.rank[a.area!]!)
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

  // A dropped area's stops go to the nearest area left that ranks at least as high, so a small place never names a big one.
  private merge(groups: StopGroup[], kept: StopGroup[]): StopGroup[] {
    const alive = kept.map((g) => g.area!)
    const members = new Map(kept.map((g) => [g.area!, [...g.points]]))
    const keptKeys = new Set(kept.map((g) => g.key))
    for (const group of groups) {
      if (keptKeys.has(group.key)) continue
      const above = alive.filter((k) => this.rank[k]! >= this.rank[group.area!]!)
      for (const i of group.points) members.get(nearestOf(this.sx[i]!, this.sy[i]!, above, this.ax, this.ay))!.push(i)
    }
    return [...members].map(([k, points]) =>
      this.group(
        k,
        points.sort((a, b) => a - b),
      ),
    )
  }

  private group(k: number, points: number[]): StopGroup {
    const all = this.stations.points
    const leader = points.reduce((best, i) => (all[i]!.rank > all[best]!.rank ? i : best), points[0]!)
    const group = groupOf(all, leader, points, `a${k}`)
    group.area = k
    return group
  }
}

// Areas take over once most station groups are too big to show each station.
export class MapClusters {
  private checked = 0
  private crossing: number | null = null

  constructor(
    readonly stations: StopClusters,
    private readonly areas: AreaClusters | null,
    private readonly detail: number,
  ) {}

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
    return group.area === undefined ? this.stations.lngLat(group) : this.areas!.lngLat(group)
  }

  bounds(group: StopGroup) {
    return this.stations.bounds(group)
  }

  box(group: StopGroup) {
    return group.area === undefined ? this.stations.box(group) : this.areas!.box(group)
  }

  expansion(group: StopGroup, zoom: number) {
    return group.area === undefined ? this.stations.expansion(group, zoom) : null
  }

  areaName(group: StopGroup): string | null {
    return group.area === undefined ? null : this.areas!.name(group)
  }

  lowestStationZoom(): number | null {
    return this.crossing === null ? null : this.stations.zoomOf(this.crossing - 1)
  }

  // Pixels from the group's spot; the other markers as they span the level's zoom range.
  surroundings(group: StopGroup, zoom: number) {
    const level = this.stations.levelOf(zoom)
    const ends = [this.stations.zoomOf(level), this.stations.zoomOf(Math.max(0, level - 1))]
    const centre = this.lngLat(group)
    const offset = (lat: number, lng: number, z = ends[0]!) => {
      const px = TILE_PX * 2 ** z
      return {x: (mercX(lng) - mercX(centre.lng)) * px, y: (mercY(lat) - mercY(centre.lat)) * px}
    }
    const obstacles = this.at(zoom)
      .filter((g) => g !== group)
      .map((g): Rect => {
        const {lat, lng} = this.lngLat(g)
        const [a, b] = ends.map((z) => {
          const {x, y} = offset(lat, lng, z)
          return boxAt(x, y, this.box(g))
        })
        return {left: Math.min(a!.left, b!.left), top: Math.min(a!.top, b!.top), right: Math.max(a!.right, b!.right), bottom: Math.max(a!.bottom, b!.bottom)}
      })
    obstacles.push(boxAt(0, 0, this.box(group)))
    return {obstacles, offset: (lat: number, lng: number) => offset(lat, lng)}
  }

  private far(zoom: number): boolean {
    this.settle()
    return this.crossing !== null && this.stations.levelOf(zoom) >= this.crossing
  }

  private settle() {
    if (!this.areas) return
    for (; this.crossing === null && this.checked < this.stations.depth; this.checked++) {
      const stations = this.stations.at(this.stations.zoomOf(this.checked))
      if (2 * stations.filter((g) => g.stations.size > this.detail).length > stations.length) this.crossing = this.checked
    }
  }
}
